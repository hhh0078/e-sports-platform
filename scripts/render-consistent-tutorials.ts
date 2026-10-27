/** 文件作用：复用三条已验收教程的原速操作与烧录字幕，用每条一次生成的连续Serena音源重新配时，只整体增益及重采样一次，仅在真实静音插零，输出待交付的tmp媒体及可复核映射。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'

/** 独立语音识别复核后的完整原稿句子及真实时间。 */
interface Segment { index: number; text: string; start: number; end: number; coverage: number }
/** 已验收教程中的一个原速操作窗口。 */
interface Step { id: string; title: string; narration: string; start: number; end: number; frames: number; operation_seconds: number }
/** 句间采样切点及其10毫秒实测能量。 */
interface Boundary { sample: number; rms: number; asr_midpoint: number }
/** 连续源采样到新增静音后的输出采样映射；末尾插零不覆盖原自然尾音。 */
interface Span { source_start: number; source_end: number; output_start: number; output_end: number; inserted_samples: number; boundary_rms: number }
/** 供最终管线读取的单条教程tmp结果；此脚本没有交付替换职责。 */
export interface TutorialResult { id: string; directory: string; mp4: string; wav: string; srt: string; metadata: Record<string, unknown>; frames: number[]; spans: Span[]; probe: Record<string, unknown> }

const root = process.cwd(), work = path.join(root, '.run/consistent-render'), voiceRoot = path.join(root, '.run/consistent-voice')
const backup = path.join(root, '.run/consistent-originals/tutorials/videos'), ffmpeg = path.join(root, '.run/media-tools/package/ffmpeg')
const ffprobe = '/Applications/TRAE SOLO CN.app/Contents/Resources/app/bin/ffprobe', rate = 48000, fps = 30, samplesPerFrame = rate / fps
const tutorialIds = ['01-platform-tutorial', '02-club-tutorial', '03-order-tutorial']

/** 收窄未知JSON对象；数组不按对象接收。 */
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
/** 读取外部JSON，调用方负责逐项验证数据边界。 */
function json(file: string): unknown { return JSON.parse(fs.readFileSync(file, 'utf8')) }
/** 计算文件SHA-256，追溯冻结音源与原已验收画面。 */
function hash(file: string): string { return createHash('sha256').update(fs.readFileSync(file)).digest('hex') }

/** 使用参数数组执行媒体工具，不通过Shell解释文件路径或文案。 */
async function command(binary: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { stdio: ['ignore', 'pipe', 'pipe'] }); let stdout = '', stderr = ''
    child.stdout.on('data', (bytes: Buffer) => { stdout = (stdout + bytes.toString()).slice(-200000) })
    child.stderr.on('data', (bytes: Buffer) => { stderr = (stderr + bytes.toString()).slice(-30000) })
    child.on('error', reject); child.on('close', code => code === 0 ? resolve(stdout || stderr) : reject(new Error(path.basename(binary) + '失败：' + stderr)))
  })
}

/** 读取48kHz单声道16位PCM，逐块解析WAV，允许工具附加元数据块。 */
function readWav(file: string): Int16Array {
  const bytes = fs.readFileSync(file); assert.equal(bytes.toString('ascii', 0, 4), 'RIFF'); assert.equal(bytes.toString('ascii', 8, 12), 'WAVE')
  let samples: Int16Array | undefined, formatValid = false
  for (let offset = 12; offset + 8 <= bytes.length;) {
    const size = bytes.readUInt32LE(offset + 4), end = offset + 8 + size; assert.ok(end <= bytes.length, 'WAV块超出文件')
    if (bytes.toString('ascii', offset, offset + 4) === 'fmt ') {
      assert.equal(bytes.readUInt16LE(offset + 8), 1); assert.equal(bytes.readUInt16LE(offset + 10), 1)
      assert.equal(bytes.readUInt32LE(offset + 12), rate); assert.equal(bytes.readUInt16LE(offset + 22), 16); formatValid = true
    }
    if (bytes.toString('ascii', offset, offset + 4) === 'data') {
      assert.equal(size % 2, 0); samples = new Int16Array(size / 2)
      for (let i = 0; i < samples.length; i++) samples[i] = bytes.readInt16LE(offset + 8 + i * 2)
    }
    offset = end + size % 2
  }
  assert.ok(formatValid && samples && samples.length, 'WAV缺少PCM格式或声音'); return samples
}

/** 写入精确采样数的标准PCM WAV，避免音段分别AAC编码产生接缝。 */
function writeWav(file: string, samples: Int16Array): void {
  const bytes = Buffer.alloc(44 + samples.length * 2); bytes.write('RIFF'); bytes.writeUInt32LE(bytes.length - 8, 4); bytes.write('WAVE', 8); bytes.write('fmt ', 12); bytes.writeUInt32LE(16, 16)
  bytes.writeUInt16LE(1, 20); bytes.writeUInt16LE(1, 22); bytes.writeUInt32LE(rate, 24); bytes.writeUInt32LE(rate * 2, 28); bytes.writeUInt16LE(2, 32); bytes.writeUInt16LE(16, 34); bytes.write('data', 36); bytes.writeUInt32LE(samples.length * 2, 40)
  for (let i = 0; i < samples.length; i++) bytes.writeInt16LE(samples[i], 44 + i * 2)
  fs.writeFileSync(file, bytes)
}

/** 读取冻结原片报告中的步骤，核对每步操作长度不会越过下一个烧录字幕。 */
function original(id: string): { row: Record<string, unknown>; steps: Step[]; previewNames: string[] } {
  const report = json(path.join(backup, 'tutorial-render-report.json')); assert.ok(record(report) && Array.isArray(report.results))
  const row = report.results.find((value: unknown) => record(value) && value.id === id); assert.ok(record(row) && Array.isArray(row.steps) && row.steps.length)
  const steps = row.steps.map((value: unknown): Step => {
    assert.ok(record(value) && typeof value.id === 'string' && typeof value.title === 'string' && typeof value.narration === 'string')
    assert.ok(typeof value.start === 'number' && Number.isFinite(value.start) && value.start >= 0 && typeof value.end === 'number')
    assert.ok(typeof value.frames === 'number' && Number.isInteger(value.frames) && value.frames > 0)
    assert.ok(typeof value.operation_seconds === 'number' && Number.isFinite(value.operation_seconds) && value.operation_seconds > 0 && value.operation_seconds <= value.end - value.start + .001)
    return { id: value.id, title: value.title, narration: value.narration, start: value.start, end: value.end, frames: value.frames, operation_seconds: value.operation_seconds }
  })
  assert.ok(Array.isArray(row.previews) && row.previews.length === steps.length && row.previews.every(value => typeof value === 'string'))
  return { row, steps, previewNames: row.previews.map(value => path.basename(String(value))) }
}

/** 验证单次整篇Serena与复核时间；只整体一次增益、一次重采样，不改变语速音调。 */
async function voice(id: string, directory: string, steps: Step[]): Promise<{ source: Int16Array; segments: Segment[]; gain: number; sourceHash: string; metadata: Record<string, unknown> }> {
  const file = path.join(voiceRoot, id, 'narration.wav'), sourceHash = hash(file), metadata = json(path.join(voiceRoot, id, 'metadata.json')), alignment = json(path.join(voiceRoot, id, 'alignment.json'))
  assert.ok(record(metadata) && metadata.voice === 'Serena' && metadata.api_calls_per_video === 1 && metadata.audio_sha256 === sourceHash, '必须是冻结的整篇单次Serena音源')
  assert.ok(record(alignment) && alignment.needs_review === false && alignment.audio_sha256 === sourceHash && Array.isArray(alignment.segments) && typeof alignment.source_duration_seconds === 'number', '语音识别尚未真实复核')
  assert.equal(alignment.segments.length, steps.length)
  const duration = alignment.source_duration_seconds, segments = alignment.segments.map((value: unknown, index: number): Segment => {
    assert.ok(record(value) && value.index === index && value.text === steps[index].narration, '原画面字幕与完整原稿不一致')
    assert.ok(typeof value.start === 'number' && Number.isFinite(value.start) && typeof value.end === 'number' && Number.isFinite(value.end) && value.start >= 0 && value.end > value.start && value.end <= duration + .001 && typeof value.coverage === 'number')
    return { index, text: value.text, start: value.start, end: value.end, coverage: value.coverage }
  })
  for (let i = 1; i < segments.length; i++) assert.ok(segments[i].start >= segments[i - 1].end, '源句界时间重叠')
  assert.equal(metadata.text, segments.map(segment => segment.text).join(''), '完整原稿与一次合成正文不同')
  const meter = await command(ffmpeg, ['-hide_banner', '-i', file, '-af', 'volumedetect', '-f', 'null', '-'])
  const mean = meter.match(/mean_volume:\s*(-?[\d.]+) dB/), peak = meter.match(/max_volume:\s*(-?[\d.]+) dB/); assert.ok(mean && peak)
  const gain = Math.min(8, -20 - Number(mean[1]), -3 - Number(peak[1])), normalized = path.join(directory, 'global.wav')
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', file, '-af', `aresample=${rate},volume=${gain}dB`, '-c:a', 'pcm_s16le', '-ar', String(rate), '-ac', '1', normalized])
  const source = readWav(normalized); assert.ok(Math.abs(source.length / rate - duration) <= 1 / rate, '整体处理改变了源声音时长')
  return { source, segments, gain, sourceHash, metadata }
}

/** 测量以指定采样为中心的10毫秒RMS；边界值也按同一声学窗口复核。 */
function rmsAt(source: Int16Array, point: number): number {
  const half = rate / 200; assert.ok(point >= half && point + half <= source.length)
  let square = 0; for (let i = point - half; i < point + half; i++) square += source[i] ** 2
  return Math.sqrt(square / (half * 2)) / 32768
}

/** 从相邻ASR句界中点附近寻找低能量静音，优先最近切点，找不到则保留失败证据供插零门禁拒绝。 */
function boundary(source: Int16Array, left: Segment, right: Segment): Boundary {
  const midpoint = (left.end + right.start) / 2, center = Math.round(midpoint * rate), radius = Math.round(.22 * rate), half = rate / 200
  const begin = Math.max(half, center - radius), end = Math.min(source.length - half, center + radius)
  let selected = Math.min(end, Math.max(begin, center)), best = Infinity
  for (let point = begin; point <= end; point += 48) {
    const rms = rmsAt(source, point), score = rms < .008 ? Math.abs(point - center) / rate : 10 + rms
    if (score < best) { best = score; selected = point }
  }
  let crossing = selected, smallest = Infinity
  for (let point = Math.max(half, selected - half); point <= Math.min(source.length - half, selected + half); point++) {
    const value = Math.abs(source[point - 1]) + Math.abs(source[point]) + Math.abs(point - selected) * .001
    if (value < smallest && rmsAt(source, point) < .008) { smallest = value; crossing = point }
  }
  return { sample: crossing, rms: rmsAt(source, crossing), asr_midpoint: midpoint }
}

/** 每步时长按操作与连续语音窗口两者最大值向上取整到30fps，只在真实句间静音及整条尾部插零。 */
function schedule(source: Int16Array, segments: Segment[], steps: Step[]): { audio: Int16Array; spans: Span[]; frames: number[]; boundaries: Boundary[] } {
  const boundaries: Boundary[] = [{ sample: 0, rms: 0, asr_midpoint: 0 }, ...segments.slice(1).map((segment, i) => boundary(source, segments[i], segment)), { sample: source.length, rms: 0, asr_midpoint: source.length / rate }]
  const frames: number[] = [], spans: Span[] = []; let output = 0
  for (let i = 0; i < segments.length; i++) {
    const start = boundaries[i].sample, end = boundaries[i + 1].sample; assert.ok(end > start, '音频窗口不单调')
    const count = Math.ceil(Math.max(steps[i].operation_seconds * rate, end - start) / samplesPerFrame), needed = count * samplesPerFrame, extra = needed - (end - start)
    if (extra > 0 && i < segments.length - 1) assert.ok(boundaries[i + 1].rms < .008, '没有真实句间静音，禁止在说话中插零：' + i)
    frames.push(count); spans.push({ source_start: start, source_end: end, output_start: output, output_end: output + needed, inserted_samples: extra, boundary_rms: boundaries[i + 1].rms }); output += needed
  }
  const audio = new Int16Array(output)
  for (const span of spans) {
    audio.set(source.subarray(span.source_start, span.source_end), span.output_start)
    for (let i = span.source_start; i < span.source_end; i++) assert.equal(audio[span.output_start + i - span.source_start], source[i], '原始语音采样改变')
    assert.ok(audio.subarray(span.output_start + span.source_end - span.source_start, span.output_end).every(sample => sample === 0), '插入区不是零采样')
  }
  assert.equal(spans.reduce((sum, span) => sum + span.source_end - span.source_start, 0), source.length, '原自然尾音或其它源采样丢失')
  return { audio, spans, frames, boundaries }
}

/** 将真实ASR时间按插零点映射，句尾取左极限、句首取右极限，保留实际整句说话时长。 */
function mapped(seconds: number, spans: Span[], ending = false): number {
  const sample = Math.round(seconds * rate), span = spans.find(value => sample < value.source_end || ending && sample === value.source_end) || spans[spans.length - 1]
  return (span.output_start + Math.max(0, Math.min(span.source_end - span.source_start, sample - span.source_start))) / rate
}

/** 格式化SRT毫秒时间，不使用画面定帧时长代替实际说话起止。 */
function time(seconds: number): string {
  const ms = Math.round(seconds * 1000)
  return `${String(Math.floor(ms / 3600000)).padStart(2, '0')}:${String(Math.floor(ms / 60000) % 60).padStart(2, '0')}:${String(Math.floor(ms / 1000) % 60).padStart(2, '0')},${String(ms % 1000).padStart(3, '0')}`
}

/** 从原步骤起点只取真实操作前段，保留原速并定帧至新语音窗口；concat显式声明每段精确帧时长。 */
async function visual(id: string, steps: Step[], frames: number[], directory: string): Promise<string> {
  const source = path.join(backup, id + '.mp4'), files: string[] = [], target = path.join(directory, 'silent.mp4')
  for (let i = 0; i < steps.length; i++) {
    const step = steps[i], file = path.join(directory, `visual-${i}.mp4`)
    await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-ss', String(step.start), '-i', source, '-vf', `trim=duration=${step.operation_seconds},setpts=PTS-STARTPTS,fps=${fps},tpad=stop_mode=clone:stop_duration=${frames[i] / fps},trim=end_frame=${frames[i]}`, '-map', '0:v:0', '-frames:v', String(frames[i]), '-r', String(fps), '-c:v', 'libx264', '-preset', 'fast', '-crf', '18', '-pix_fmt', 'yuv420p', '-an', file]); files.push(file)
  }
  const list = path.join(directory, 'visual-list.txt'); fs.writeFileSync(list, files.map((file, i) => `file '${path.basename(file)}'\nduration ${frames[i] / fps}`).join('\n') + '\n')
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', list, '-map', '0:v:0', '-c:v', 'copy', '-an', target]); return target
}

/** 验证实际完整成片的1080×1920/H264/30fps、精确帧数、AAC与完整音画时长，并解码排错。 */
async function verify(file: string, frames: number[]): Promise<Record<string, unknown>> {
  const data: unknown = JSON.parse(await command(ffprobe, ['-v', 'error', '-show_entries', 'format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate,r_frame_rate,nb_frames,sample_rate,channels,duration', '-of', 'json', file]))
  assert.ok(record(data) && Array.isArray(data.streams) && record(data.format)); const streams = data.streams.filter(record)
  const video = streams.find(stream => stream.codec_type === 'video'), audio = streams.find(stream => stream.codec_type === 'audio'), count = frames.reduce((sum, value) => sum + value, 0)
  assert.ok(video && audio); assert.equal(video.width, 1080); assert.equal(video.height, 1920); assert.equal(video.codec_name, 'h264'); assert.equal(video.avg_frame_rate, '30/1'); assert.equal(video.r_frame_rate, '30/1'); assert.equal(Number(video.nb_frames), count)
  assert.equal(audio.codec_name, 'aac'); assert.equal(Number(audio.sample_rate), rate); assert.equal(audio.channels, 1)
  assert.ok(Math.abs(Number(video.duration) - count / fps) < 1 / rate && Math.abs(Number(data.format.duration) - count / fps) < .06, '成片音画时长不一致')
  const meter = await command(ffmpeg, ['-hide_banner', '-i', file, '-vn', '-af', 'volumedetect', '-f', 'null', '-']), peak = meter.match(/max_volume:\s*(-?[\d.]+) dB/)
  assert.ok(peak && Number(peak[1]) > -45 && Number(peak[1]) < 0, '成片声音无效或削波')
  await command(ffmpeg, ['-v', 'error', '-i', file, '-f', 'null', '-'])
  return { ...data, max_volume_dbfs: Number(peak[1]), full_decode_passed: true, expected_frames: count, expected_seconds: count / fps }
}

/** 渲染一条教程tmp；不改output交付、不重生成旁白、不逐段编码AAC，返回最终管线所需全部路径及证据。 */
export async function renderTutorial(id: string): Promise<TutorialResult> {
  assert.ok(tutorialIds.includes(id), '未知教程：' + id)
  const directory = path.join(work, id); fs.mkdirSync(directory, { recursive: true }); console.log('开始教程连续音源配时：' + id)
  const item = original(id), input = await voice(id, directory, item.steps), scheduled = schedule(input.source, input.segments, item.steps)
  const wav = path.join(directory, 'narration.wav'), mp4 = path.join(directory, id + '.mp4'), srt = path.join(directory, id + '.srt')
  writeWav(wav, scheduled.audio); assert.deepEqual(readWav(wav), scheduled.audio, '写入WAV改变采样')
  const picture = await visual(id, item.steps, scheduled.frames, directory)
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', picture, '-i', wav, '-map', '0:v:0', '-map', '1:a:0', '-c:v', 'copy', '-c:a', 'aac', '-b:a', '192k', '-ar', String(rate), '-ac', '1', '-movflags', '+faststart', mp4])
  const probe = await verify(mp4, scheduled.frames), cues = input.segments.map(segment => ({ ...segment, start: mapped(segment.start, scheduled.spans), end: mapped(segment.end, scheduled.spans, true) }))
  assert.equal(cues.map(cue => cue.text).join(''), item.steps.map(step => step.narration).join('')); assert.ok(cues.every(cue => cue.end > cue.start && cue.end <= scheduled.audio.length / rate))
  fs.writeFileSync(srt, cues.map((cue, i) => `${i + 1}\n${time(cue.start)} --> ${time(cue.end)}\n${cue.text}\n`).join('\n'))
  const metadata: Record<string, unknown> = {
    id, kind: 'tutorial', voice: 'Serena', single_generation_pass: true, source_audio_sha256: input.sourceHash, single_source_audio_sha256: input.sourceHash,
    source_duration_seconds: input.source.length / rate, original_video_sha256: hash(path.join(backup, id + '.mp4')), old_video_sha256: hash(path.join(backup, id + '.mp4')),
    alignment_sha256: hash(path.join(voiceRoot, id, 'alignment.json')), normalized_pcm_sha256: hash(path.join(directory, 'global.wav')), narration_sha256: hash(wav),
    global_gain_db: input.gain, global_gain_scope: '同一固定增益作用于整篇一次', global_tempo: 1, global_tempo_ratio: 1, resample_passes: 1,
    sentence_gain_applied: false, sentence_tts_applied: false, pitch_shift_applied: false, samples_preserved_in_order: true,
    preserved_sample_scope: '整篇固定gain和一次48k重采样后的全部PCM按原顺序逐样本保留，只插入零值；自然尾音全部保留',
    source_sample_count: input.source.length, preserved_sample_count: scheduled.spans.reduce((sum, span) => sum + span.source_end - span.source_start, 0),
    inserted_samples: scheduled.spans.reduce((sum, span) => sum + span.inserted_samples, 0), sample_rate: rate, playback_rate: 1, fps,
    frames: scheduled.frames.reduce((sum, count) => sum + count, 0), duration_seconds: scheduled.audio.length / rate, spans: scheduled.spans, boundaries: scheduled.boundaries, cues,
    operations: item.steps.map((step, i) => ({ index: i, id: step.id, title: step.title, source_video_start: step.start, source_video_end: step.start + step.operation_seconds, operation_seconds: step.operation_seconds, operation_speed: 1, output_start: scheduled.spans[i].output_start / rate, output_operation_end: scheduled.spans[i].output_start / rate + step.operation_seconds, output_end: scheduled.spans[i].output_end / rate, output_frames: scheduled.frames[i], held_end_frame_seconds: scheduled.frames[i] / fps - step.operation_seconds })),
    generation_metadata: input.metadata, original: item.row, probe,
  }
  fs.writeFileSync(path.join(directory, 'metadata.json'), JSON.stringify(metadata, null, 2) + '\n')
  const qa = path.join(directory, 'qa'); fs.mkdirSync(qa, { recursive: true })
  for (const [i, name] of item.previewNames.entries()) await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-ss', String(scheduled.spans[i].output_start / rate + Math.max(0, item.steps[i].operation_seconds - .15)), '-i', mp4, '-frames:v', '1', path.join(qa, name)])
  console.log('教程tmp核验通过：' + id + ' ' + Number(metadata.duration_seconds).toFixed(3) + '秒')
  return { id, directory, mp4, wav, srt, metadata, frames: scheduled.frames, spans: scheduled.spans, probe }
}

/** 用可控声音核对帧取整、采样保留、插零映射、自然尾音与无静音拒绝；不冒充真人音色验收。 */
function selfTest(): void {
  const source = new Int16Array(rate * 2); source.fill(2000, 0, rate * .7); source.fill(-3000, rate * 1.2, rate * 1.8); source[source.length - 1] = 7
  const segments: Segment[] = [{ index: 0, text: '第一句。', start: 0, end: .7, coverage: 1 }, { index: 1, text: '第二句。', start: 1.2, end: 1.8, coverage: 1 }]
  const steps: Step[] = segments.map(segment => ({ id: String(segment.index), title: '测试', narration: segment.text, start: 0, end: 3, frames: 90, operation_seconds: 2 }))
  const result = schedule(source, segments, steps); assert.deepEqual(result.frames, [60, 60]); assert.equal(result.audio.length, rate * 4); assert.equal(result.spans.reduce((sum, span) => sum + span.inserted_samples, 0), rate * 2)
  assert.ok(mapped(1.2, result.spans) > 2); const last = result.spans[result.spans.length - 1]; assert.equal(result.audio[last.output_start + source.length - last.source_start - 1], 7)
  const noSilence = new Int16Array(rate * 2).fill(2000); assert.throws(() => schedule(noSilence, segments, steps), /没有真实句间静音/)
  assert.equal(time(61.234), '00:01:01,234'); console.log('自检通过：所有源采样原序保留、只插零、30fps精确配时、自然尾音保留、无静音时拒绝。')
}

/** CLI只生成指定教程tmp，import不会执行；CONSISTENT_IDS可选择一条或多条。 */
async function main(): Promise<void> {
  if (process.argv.includes('--self-test')) { selfTest(); return }
  const selected = (process.env.CONSISTENT_IDS || tutorialIds.join(',')).split(',').filter(Boolean); assert.ok(selected.length && selected.every(id => tutorialIds.includes(id)))
  const results: TutorialResult[] = []; for (const id of selected) results.push(await renderTutorial(id))
  fs.writeFileSync(path.join(work, 'tutorial-results.json'), JSON.stringify({ passed: true, tmp_only: true, results: results.map(result => ({ id: result.id, directory: result.directory, mp4: result.mp4, wav: result.wav, srt: result.srt, ...result.metadata })) }, null, 2) + '\n')
}

if (/^render-consistent-tutorials\.(ts|js)$/.test(path.basename(process.argv[1] || ''))) main().catch((error: unknown) => { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 })
