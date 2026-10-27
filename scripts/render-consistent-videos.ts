/** 文件作用：复用已验收的广告与教程画面，使用每条一次生成的连续Serena旁白重新配时；整条固定增益，只在真实句间静音插零，不逐句合成或归一化。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'

/** 连续音轨的独立识别对齐边界。 */
interface Segment { index: number; text: string; start: number; end: number; coverage: number }
/** 声学边界与同一段连续采样的映射。 */
interface Span { source_start: number; source_end: number; output_start: number; output_end: number; inserted_samples: number; boundary_rms: number }
/** 交付片段只引用备份画面；教程动作长度保持原速。 */
interface Clip { id: string; kind: 'ad' | 'tutorial'; texts: string[]; oldStarts: number[]; oldFrames: number[]; operationFrames: number[]; previewNames: string[]; original: Record<string, unknown> }
/** 独立对齐与一次生成的真实音源信息。 */
interface Voice { segments: Segment[]; samples: Int16Array; sourceHash: string; sourceDuration: number; gain: number; tempo: number; metadata: Record<string, unknown>; alignment: Record<string, unknown> }
/** tmp媒体与其可复核的时间映射。 */
interface Result { clip: Clip; directory: string; mp4: string; wav: string; m4a: string; aac: string; srt: string; metadata: Record<string, unknown>; spans: Span[]; frames: number[]; probe: Record<string, unknown> }

const root = process.cwd(), work = path.join(root, '.run/consistent-render'), sourceRoot = path.join(root, '.run/consistent-voice')
const ads = path.join(root, 'output/playwright/marketing-20261026'), tutorials = path.join(root, 'output/playwright/tutorials-20261026')
const backup = path.join(root, '.run/consistent-originals'), ffmpeg = path.join(root, '.run/media-tools/package/ffmpeg'), ffprobe = '/Applications/TRAE SOLO CN.app/Contents/Resources/app/bin/ffprobe'
const rate = 48000, fps = 30, ids = ['01-platform-admin', '02-club-admin', '03-game-enthusiast', '04-player', '01-platform-tutorial', '02-club-tutorial', '03-order-tutorial']
const oldBoundaries = [0, 3, 7.5, 12, 16.5, 21, 25.5, 30]

/** 严格检查外部JSON的对象类型。 */
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
/** 读取JSON并保留未知边界，后续逐项校验。 */
function json(file: string): unknown { return JSON.parse(fs.readFileSync(file, 'utf8')) }
/** 计算文件摘要，用于原片备份自检和连续声音追溯。 */
function hash(file: string): string { return createHash('sha256').update(fs.readFileSync(file)).digest('hex') }
/** 使用参数数组执行媒体命令，异步编码，不运行Shell插值。 */
async function command(binary: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { stdio: ['ignore', 'pipe', 'pipe'] }); let stdout = '', stderr = ''
    child.stdout.on('data', (bytes: Buffer) => { stdout = (stdout + bytes.toString()).slice(-200000) })
    child.stderr.on('data', (bytes: Buffer) => { stderr = (stderr + bytes.toString()).slice(-20000) })
    child.on('error', reject); child.on('close', code => code === 0 ? resolve(stdout || stderr) : reject(new Error(path.basename(binary) + '失败：' + stderr)))
  })
}
/** 保存原始成片；重跑仅与备份存储的摘要比较，不能与已替换的交付成片比较。 */
function originals(): void {
  const indexFile = path.join(backup, 'index.json')
  if (fs.existsSync(indexFile)) {
    const data = json(indexFile); assert.ok(record(data) && Array.isArray(data.files))
    for (const entry of data.files as unknown[]) { assert.ok(record(entry) && typeof entry.file === 'string' && typeof entry.sha256 === 'string'); assert.equal(hash(path.join(backup, entry.file)), entry.sha256, '原始备份损坏：' + entry.file) }
    return
  }
  const files: Array<{ file: string; sha256: string }> = []
  for (const [name, directory] of [['ads', ads], ['tutorials', tutorials]]) {
    const wanted = fs.readdirSync(path.join(directory, 'videos')).filter(file => file.endsWith('.mp4') || file.endsWith('report.json') || file.endsWith('.srt'))
    for (const file of wanted) {
      const source = path.join(directory, 'videos', file), relative = path.join(name, 'videos', file), target = path.join(backup, relative)
      fs.mkdirSync(path.dirname(target), { recursive: true }); if (!fs.existsSync(target)) fs.copyFileSync(source, target)
      files.push({ file: relative, sha256: hash(target) })
    }
  }
  fs.writeFileSync(indexFile, JSON.stringify({ created_at: new Date().toISOString(), files }, null, 2) + '\n')
}
/** 读取由完整音轨转换的48kHz单声道16位PCM，不假定固定44字节WAV头。 */
function readWav(file: string): Int16Array {
  const bytes = fs.readFileSync(file); assert.equal(bytes.toString('ascii', 0, 4), 'RIFF'); assert.equal(bytes.toString('ascii', 8, 12), 'WAVE')
  let samples: Int16Array | undefined
  for (let offset = 12; offset + 8 <= bytes.length;) {
    const size = bytes.readUInt32LE(offset + 4), end = offset + 8 + size; assert.ok(end <= bytes.length)
    if (bytes.toString('ascii', offset, offset + 4) === 'fmt ') { assert.equal(bytes.readUInt16LE(offset + 8), 1); assert.equal(bytes.readUInt16LE(offset + 10), 1); assert.equal(bytes.readUInt32LE(offset + 12), rate); assert.equal(bytes.readUInt16LE(offset + 22), 16) }
    if (bytes.toString('ascii', offset, offset + 4) === 'data') { samples = new Int16Array(size / 2); for (let i = 0; i < samples.length; i++) samples[i] = bytes.readInt16LE(offset + 8 + i * 2) }
    offset = end + size % 2
  }
  assert.ok(samples && samples.length); return samples
}
/** 写入精确采样数的标准PCM WAV。 */
function writeWav(file: string, samples: Int16Array): void {
  const bytes = Buffer.alloc(44 + samples.length * 2); bytes.write('RIFF'); bytes.writeUInt32LE(bytes.length - 8, 4); bytes.write('WAVE', 8); bytes.write('fmt ', 12); bytes.writeUInt32LE(16, 16)
  bytes.writeUInt16LE(1, 20); bytes.writeUInt16LE(1, 22); bytes.writeUInt32LE(rate, 24); bytes.writeUInt32LE(rate * 2, 28); bytes.writeUInt16LE(2, 32); bytes.writeUInt16LE(16, 34); bytes.write('data', 36); bytes.writeUInt32LE(samples.length * 2, 40)
  for (let i = 0; i < samples.length; i++) bytes.writeInt16LE(samples[i], 44 + i * 2); fs.writeFileSync(file, bytes)
}
/** 在ASR句界附近寻找真实低能量静音，保留全部声音采样，插零时不会剪掉一个字。 */
function boundary(samples: Int16Array, left: Segment, right: Segment): { sample: number; rms: number } {
  const center = Math.round((left.end + right.start) / 2 * rate), radius = Math.round(.22 * rate), half = 240
  const begin = Math.max(half, center - radius), end = Math.min(samples.length - half, center + radius)
  let selected = center, best = Infinity, rms = Infinity
  for (let point = begin; point <= end; point += 48) {
    let square = 0; for (let i = point - half; i < point + half; i++) square += samples[i] ** 2
    const value = Math.sqrt(square / (half * 2)) / 32768, score = value < .008 ? Math.abs(point - center) / rate : 10 + value
    if (score < best) { best = score; selected = point; rms = value }
  }
  let crossing = selected, smallest = Infinity
  for (let point = Math.max(1, selected - half); point < Math.min(samples.length, selected + half); point++) {
    const value = Math.abs(samples[point - 1]) + Math.abs(samples[point]) + Math.abs(point - selected) * .001
    if (value < smallest) { smallest = value; crossing = point }
  }
  return { sample: crossing, rms }
}
/** 一次计算真实静音界，后续画面和音轨共用同一份采样坐标。 */
function audioBoundaries(source: Int16Array, segments: Segment[]): Array<{ sample: number; rms: number }> { return [{ sample: 0, rms: 0 }, ...segments.slice(1).map((segment, i) => boundary(source, segments[i], segment)), { sample: source.length, rms: 0 }] }
/** 在连续源采样中插入静音，逐片核对所有原声音样本原序保留，禁止逐句gain或变调。 */
function insertSilence(source: Int16Array, segments: Segment[], targetFrames: number[], boundaries = audioBoundaries(source, segments)): { audio: Int16Array; spans: Span[] } {
  const spans: Span[] = []; let output = 0
  for (let i = 0; i < segments.length; i++) {
    const start = boundaries[i].sample, end = boundaries[i + 1].sample, needed = targetFrames[i] * (rate / fps), extra = needed - (end - start)
    assert.ok(end > start && extra >= 0, '整句声音超过镜头窗口，需调整完整音轨速度或镜头边界：' + i)
    if (extra > 0 && i < segments.length - 1) assert.ok(boundaries[i + 1].rms < .008, '句界没有真实静音，不能在说话中插入等待：' + i)
    spans.push({ source_start: start, source_end: end, output_start: output, output_end: output + needed, inserted_samples: extra, boundary_rms: boundaries[i + 1].rms }); output += needed
  }
  const audio = new Int16Array(output)
  for (const span of spans) { audio.set(source.subarray(span.source_start, span.source_end), span.output_start); assert.ok(audio.subarray(span.output_start, span.output_start + span.source_end - span.source_start).every((sample, i) => sample === source[span.source_start + i]), '连续说话采样被更改') }
  return { audio, spans }
}
/** 将精确秒数写成SRT时间。 */
function time(value: number): string { const ms = Math.round(value * 1000); return `${String(Math.floor(ms / 3600000)).padStart(2, '0')}:${String(Math.floor(ms / 60000) % 60).padStart(2, '0')}:${String(Math.floor(ms / 1000) % 60).padStart(2, '0')},${String(ms % 1000).padStart(3, '0')}` }

/** 读取备份中的字幕与原时轴，不能从已替换的新成片反推旧步骤。 */
function clip(id: string): Clip {
  const kind = id.endsWith('-tutorial') ? 'tutorial' : 'ad', report = json(path.join(backup, kind === 'ad' ? 'ads/videos/video-report.json' : 'tutorials/videos/tutorial-render-report.json'))
  assert.ok(record(report) && Array.isArray(report.results)); const original = report.results.find((item: unknown) => record(item) && item.id === id)
  assert.ok(record(original), '备份报告缺少原片：' + id)
  if (kind === 'ad') {
    assert.ok(record(report.narration_subtitles) && record(report.narration_subtitles.segments)); const texts = report.narration_subtitles.segments[id]
    assert.ok(Array.isArray(texts) && texts.every(item => typeof item === 'string'))
    return { id, kind, texts, oldStarts: oldBoundaries.slice(0, -1), oldFrames: oldBoundaries.slice(1).map((end, i) => Math.round((end - oldBoundaries[i]) * fps)), operationFrames: [], previewNames: [0, 3, 8, 15, 24, 29].map(second => String(second).padStart(2, '0') + 's.png'), original }
  }
  assert.ok(Array.isArray(original.steps) && Array.isArray(original.previews))
  const steps = original.steps as Array<Record<string, unknown>>
  for (const step of steps) assert.ok(typeof step.narration === 'string' && typeof step.start === 'number' && typeof step.frames === 'number' && typeof step.operation_seconds === 'number')
  return { id, kind, texts: steps.map(step => String(step.narration)), oldStarts: steps.map(step => Number(step.start)), oldFrames: steps.map(step => Number(step.frames)), operationFrames: steps.map(step => Math.ceil(Number(step.operation_seconds) * fps)), previewNames: original.previews.map(item => path.basename(String(item))), original }
}

/** 验证完整音源与独立句界，以同一固定gain处理全条；广告只允许一次全条保调变速。 */
async function voiceFor(item: Clip, directory: string): Promise<Voice> {
  assert.equal(item.kind, 'ad', '教程交给专用helper处理')
  const file = path.join(sourceRoot, item.id, 'narration.wav'), metadata = json(path.join(sourceRoot, item.id, 'metadata.json')), alignment = json(path.join(sourceRoot, item.id, 'alignment.json'))
  const digest = hash(file)
  assert.ok(record(metadata) && metadata.voice === 'Serena' && metadata.api_calls_per_video === 1 && metadata.audio_sha256 === digest, '必须是冻结的一次生成完整Serena：' + item.id)
  assert.ok(record(alignment) && alignment.needs_review === false && Array.isArray(alignment.segments) && alignment.audio_sha256 === digest && typeof alignment.source_duration_seconds === 'number', '句界尚未完成真实复核：' + item.id)
  const sourceDuration = alignment.source_duration_seconds, segments = alignment.segments.map((value: unknown, index: number): Segment => {
    assert.ok(record(value) && value.index === index && value.text === item.texts[index] && typeof value.start === 'number' && typeof value.end === 'number' && value.end > value.start && typeof value.coverage === 'number', '完整旁白与原烧录字幕不一致：' + item.id + '/' + index)
    assert.ok(value.start >= 0 && value.end <= sourceDuration + .05)
    return { index, text: value.text, start: value.start, end: value.end, coverage: value.coverage }
  })
  assert.equal(segments.length, item.texts.length)
  for (let i = 1; i < segments.length; i++) assert.ok(segments[i].start >= segments[i - 1].end - .001, '句界不是单调不重叠时轴')
  const tempo = Math.max(1, sourceDuration / 29.5), limit = item.id === '03-game-enthusiast' ? 1.25 : 1.2
  assert.ok(tempo <= limit, '完整广告超出已批准的全条变速上限，不能剪话尾：' + item.id)
  const meter = await command(ffmpeg, ['-hide_banner', '-i', file, '-af', 'volumedetect', '-f', 'null', '-']), mean = meter.match(/mean_volume:\s*(-?[\d.]+) dB/), peak = meter.match(/max_volume:\s*(-?[\d.]+) dB/)
  assert.ok(mean && peak); const gain = Math.min(8, -20 - Number(mean[1]), -3 - Number(peak[1])), normalized = path.join(directory, 'global.wav')
  const filters = [`aresample=${rate}`, `volume=${gain}dB`, ...(tempo > 1 ? [`atempo=${tempo}`] : [])]
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', file, '-af', filters.join(','), '-c:a', 'pcm_s16le', '-ar', String(rate), '-ac', '1', normalized])
  const samples = readWav(normalized), scale = samples.length / rate / sourceDuration
  return { samples, segments: segments.map(segment => ({ ...segment, start: segment.start * scale, end: segment.end * scale })), sourceHash: digest, sourceDuration, gain, tempo, metadata, alignment }
}

/** 优先保留旧广告节奏；长整篇广告按真实句长重新分配900帧，教程始终保留每帧原速动作。 */
function schedule(item: Clip, voice: Voice, boundaries: Array<{ sample: number; rms: number }>): number[] {
  const minimum = item.texts.map((_, i) => Math.ceil((boundaries[i + 1].sample - boundaries[i].sample) / rate * fps))
  assert.ok(minimum.reduce((sum, frames) => sum + frames, 0) <= 900, '广告完整音轨无法放入30秒')
  const result = [...minimum]; let remaining = 900 - result.reduce((sum, frames) => sum + frames, 0)
  while (remaining > 0) {
    let selected = 0, deficit = -Infinity
    for (let i = 0; i < result.length; i++) { const desired = item.oldFrames[i] - result[i]; if (desired > deficit) { selected = i; deficit = desired } }
    result[selected]++; remaining--
  }
  return result
}

/** 复用原烧录字幕和设备框；广告可调整轻运镜时长，教程只取原速操作并在末帧补齐。 */
async function visual(item: Clip, frames: number[], directory: string): Promise<string> {
  const source = path.join(backup, item.kind === 'ad' ? 'ads/videos' : 'tutorials/videos', item.id + '.mp4'), target = path.join(directory, 'silent.mp4')
  if (item.kind === 'ad' && frames.every((value, i) => value === item.oldFrames[i])) {
    await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', source, '-map', '0:v:0', '-c:v', 'copy', '-an', target]); return target
  }
  const files: string[] = []
  for (let i = 0; i < frames.length; i++) {
    const oldFrames = item.oldFrames[i], file = path.join(directory, `visual-${i}.mp4`)
    const timing = `setpts=(PTS-STARTPTS)*${frames[i]}/${oldFrames}`
    await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-ss', String(item.oldStarts[i]), '-i', source, '-vf', `trim=start_frame=0:end_frame=${oldFrames},${timing},fps=${fps},tpad=stop_mode=clone:stop_duration=${1 / fps}`, '-map', '0:v:0', '-frames:v', String(frames[i]), '-r', String(fps), '-c:v', 'libx264', '-preset', 'fast', '-crf', '20', '-pix_fmt', 'yuv420p', '-an', file]); files.push(file)
  }
  const list = path.join(directory, 'visual-list.txt'); fs.writeFileSync(list, files.map((file, i) => `file '${path.basename(file)}'\nduration ${frames[i] / fps}`).join('\n') + '\n')
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', list, '-c:v', 'copy', '-an', target]); return target
}

/** 将源声音时间映射到只增加静音后的时间；句尾采用左极限，句首采用右极限。 */
function mapped(seconds: number, spans: Span[], ending = false): number {
  const sample = Math.round(seconds * rate), span = spans.find(value => sample < value.source_end || ending && sample === value.source_end) || spans[spans.length - 1]
  return (span.output_start + Math.max(0, Math.min(span.source_end - span.source_start, sample - span.source_start))) / rate
}

/** 解码实际成片，严格核验画布、帧数、时长、AAC和有声电平。 */
async function verify(file: string, frames: number[]): Promise<Record<string, unknown>> {
  const result = JSON.parse(await command(ffprobe, ['-v', 'error', '-show_entries', 'format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,sample_rate', '-of', 'json', file])) as { format: { duration: string }; streams: Array<{ codec_type: string; codec_name: string; width?: number; height?: number; avg_frame_rate?: string; nb_frames?: string; sample_rate?: string }> }
  const video = result.streams.find(stream => stream.codec_type === 'video'), audio = result.streams.find(stream => stream.codec_type === 'audio'), count = frames.reduce((sum, value) => sum + value, 0)
  assert.ok(video && audio); assert.equal(video.width, 1080); assert.equal(video.height, 1920); assert.equal(video.codec_name, 'h264'); assert.equal(video.avg_frame_rate, '30/1'); assert.equal(Number(video.nb_frames), count); assert.equal(audio.codec_name, 'aac'); assert.equal(Number(audio.sample_rate), rate)
  assert.ok(Math.abs(Number(result.format.duration) - count / fps) < .06, '最终音画时轴长度不一致')
  const meter = await command(ffmpeg, ['-hide_banner', '-i', file, '-vn', '-af', 'volumedetect', '-f', 'null', '-']), match = meter.match(/max_volume:\s*(-?[\d.]+) dB/)
  assert.ok(match && Number(match[1]) > -45 && Number(match[1]) < 0, '实际AAC声音无效或超过削波上限')
  return { ...result, max_volume_dbfs: Number(match[1]) }
}

/** 完成一条tmp成片、全部导出音频和精确SRT，正式替换前不触碰交付文件。 */
async function render(item: Clip): Promise<Result> {
  assert.equal(item.kind, 'ad', '教程使用独立helper')
  const directory = path.join(work, item.id); fs.mkdirSync(directory, { recursive: true }); console.log('开始整条Serena配时：' + item.id)
  const voice = await voiceFor(item, directory), boundaries = audioBoundaries(voice.samples, voice.segments), frames = schedule(item, voice, boundaries), scheduled = insertSilence(voice.samples, voice.segments, frames, boundaries)
  const wav = path.join(directory, 'narration.wav'), m4a = path.join(directory, 'narration.m4a'), aac = path.join(directory, 'narration.aac'), mp4 = path.join(directory, item.id + '.mp4'), srt = path.join(directory, item.id + '.srt')
  writeWav(wav, scheduled.audio)
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', wav, '-c:a', 'aac', '-b:a', '192k', '-ar', String(rate), '-ac', '1', m4a])
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', m4a, '-c:a', 'copy', '-f', 'adts', aac])
  const picture = await visual(item, frames, directory)
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', picture, '-i', m4a, '-map', '0:v:0', '-map', '1:a:0', '-c:v', 'copy', '-c:a', 'copy', '-movflags', '+faststart', mp4])
  const probe = await verify(mp4, frames), cues = voice.segments.map(segment => ({ index: segment.index, text: segment.text, start: mapped(segment.start, scheduled.spans), end: mapped(segment.end, scheduled.spans, true), coverage: segment.coverage }))
  fs.writeFileSync(srt, cues.map((cue, i) => `${i + 1}\n${time(cue.start)} --> ${time(cue.end)}\n${cue.text}\n`).join('\n'))
  const metadata = { id: item.id, kind: item.kind, single_generation_pass: true, voice: 'Serena', source_audio_sha256: voice.sourceHash, source_duration_seconds: voice.sourceDuration, alignment_sha256: hash(path.join(sourceRoot, item.id, 'alignment.json')), global_gain_db: voice.gain, global_gain_scope: '同一固定增益应用整条音源，所有句子相同', global_tempo_ratio: voice.tempo, sentence_gain_applied: false, sentence_tts_applied: false, pitch_shift_applied: false, samples_preserved_in_order: true, preserved_sample_scope: '统一resample、固定gain及可选整篇tempo处理后的连续PCM，随后逐样本原序保留，仅在真实句间静音插零', playback_rate: '广告轻运镜按句窗配时', frames: frames.reduce((sum, value) => sum + value, 0), duration_seconds: scheduled.audio.length / rate, spans: scheduled.spans, cues, generation_metadata: voice.metadata, probe }
  fs.writeFileSync(path.join(directory, 'metadata.json'), JSON.stringify(metadata, null, 2) + '\n')
  const qa = path.join(directory, 'qa'); fs.mkdirSync(qa, { recursive: true })
  for (const [i, name] of item.previewNames.entries()) {
    const second = [0, 3, 8, 15, 24, 29][i]
    await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-ss', String(second), '-i', mp4, '-frames:v', '1', path.join(qa, name)])
  }
  console.log('tmp完整核验通过：' + item.id + ' ' + metadata.duration_seconds.toFixed(3) + '秒')
  return { clip: item, directory, mp4, wav, m4a, aac, srt, metadata, spans: scheduled.spans, frames, probe }
}

/** 对最终连续PCM逐样本核验：统一处理后的全部源样本原序保留，新增区必须全部为零。 */
function auditSamples(metadata: Record<string, unknown>, wav: string, normalized: string): Span[] {
  assert.ok(Array.isArray(metadata.spans)); const spans = metadata.spans as Span[], source = readWav(normalized), output = readWav(wav)
  let sourceEnd = 0, outputEnd = 0
  for (const span of spans) {
    assert.equal(span.source_start, sourceEnd); assert.equal(span.output_start, outputEnd)
    assert.ok(span.source_end > span.source_start && span.source_end <= source.length && span.output_end <= output.length)
    const length = span.source_end - span.source_start
    assert.equal(span.output_end - span.output_start, length + span.inserted_samples)
    assert.ok(output.subarray(span.output_start, span.output_start + length).every((sample, i) => sample === source[span.source_start + i]), '统一处理后的连续声音采样发生改变')
    assert.ok(output.subarray(span.output_start + length, span.output_end).every(sample => sample === 0), '新增等待区不是纯静音')
    sourceEnd = span.source_end; outputEnd = span.output_end
  }
  assert.equal(sourceEnd, source.length); assert.equal(outputEnd, output.length); return spans
}

/** 复用已完成并绑定冻结源hash的tmp，独立复查媒体和全部PCM；缺失时由专用教程helper渲染。 */
async function cached(item: Clip): Promise<Result | undefined> {
  const directory = path.join(work, item.id), file = path.join(directory, 'metadata.json')
  if (!fs.existsSync(file)) return undefined
  const metadata = json(file)
  if (!record(metadata) || metadata.single_generation_pass !== true || metadata.voice !== 'Serena' || metadata.source_audio_sha256 !== hash(path.join(sourceRoot, item.id, 'narration.wav')) || metadata.alignment_sha256 !== hash(path.join(sourceRoot, item.id, 'alignment.json'))) return undefined
  const wav = path.join(directory, 'narration.wav'), mp4 = path.join(directory, item.id + '.mp4'), srt = path.join(directory, item.id + '.srt')
  if (![wav, mp4, srt, path.join(directory, 'global.wav')].every(fs.existsSync)) return undefined
  if (typeof metadata.narration_sha256 === 'string') assert.equal(hash(wav), metadata.narration_sha256, '缓存连续WAV摘要不一致')
  const spans = auditSamples(metadata, wav, path.join(directory, 'global.wav')), frames = spans.map(span => (span.output_end - span.output_start) / (rate / fps))
  assert.ok(frames.every(Number.isInteger))
  const probe = await verify(mp4, frames)
  return { clip: item, directory, mp4, wav, srt, m4a: path.join(directory, 'narration.m4a'), aac: path.join(directory, 'narration.aac'), metadata, frames, spans, probe }
}

/** 教程只调用独立helper，避免在总入口复制第二份原速录屏配时实现。 */
async function tutorial(item: Clip): Promise<Result> {
  const ready = await cached(item); if (ready) return ready
  const { renderTutorial } = await import('./render-consistent-tutorials')
  await renderTutorial(item.id)
  const result = await cached(item); assert.ok(result, '教程helper没有产出可核验tmp'); return result
}

/** 将tmp文件先复制为同目录临时文件再rename，单个文件替换保持原子性。 */
function replace(source: string, target: string): void { fs.mkdirSync(path.dirname(target), { recursive: true }); const pending = target + '.consistent-tmp'; fs.copyFileSync(source, pending); fs.renameSync(pending, target) }

/** 所选媒体全部通过才替换交付文件；旧逐句报告移入已有备份，新报告明确一次生成及全局处理。 */
function deliver(results: Result[]): void {
  for (const result of results) {
    const directory = result.clip.kind === 'ad' ? ads : tutorials, audio = path.join(directory, result.clip.kind === 'ad' ? 'videos/audio' : 'audio', result.clip.id)
    const publicMetadata = { ...result.metadata, source_audio_sha256: hash(path.join(sourceRoot, result.clip.id, 'narration.wav')), normalized_pcm_sha256: hash(path.join(result.directory, 'global.wav')), narration_sha256: hash(result.wav), video_sha256: hash(result.mp4), original_video_sha256: hash(path.join(backup, result.clip.kind === 'ad' ? 'ads/videos' : 'tutorials/videos', result.clip.id + '.mp4')), preserved_sample_scope: '统一resample、固定gain及可选整篇tempo处理后的连续PCM，随后逐样本原序保留，仅在真实句间静音插零；不表示24k原始字节未经处理' }
    delete (publicMetadata as Record<string, unknown>).original
    result.metadata = publicMetadata
    const metadataFile = path.join(result.directory, 'delivery-metadata.json'); fs.writeFileSync(metadataFile, JSON.stringify(publicMetadata, null, 2) + '\n')
    replace(result.mp4, path.join(directory, 'videos', result.clip.id + '.mp4')); replace(result.srt, path.join(directory, 'videos', result.clip.id + '.srt')); replace(result.wav, path.join(audio, 'narration.wav')); replace(metadataFile, path.join(audio, 'metadata.json'))
    if (result.clip.kind === 'ad') { replace(result.m4a, path.join(audio, 'narration.m4a')); replace(result.aac, path.join(audio, 'narration.aac')) }
    for (const name of fs.readdirSync(path.join(result.directory, 'qa'))) replace(path.join(result.directory, 'qa', name), path.join(directory, 'videos/qa', result.clip.id, name))
  }
  for (const kind of ['ad', 'tutorial'] as const) {
    const directory = kind === 'ad' ? ads : tutorials, selected = results.filter(result => result.clip.kind === kind)
    if (!selected.length) continue
    const canonical = path.join(directory, 'videos/consistent-voice-report.json'), previous = fs.existsSync(canonical) ? json(canonical) : undefined
    const rows: Record<string, unknown>[] = record(previous) && Array.isArray(previous.results) ? previous.results.filter(record).filter(item => !selected.some(result => result.clip.id === item.id)) : []
    for (const result of selected) {
      const data = { ...result.metadata, audience: result.clip.original.audience, title: result.clip.original.title, hook: result.clip.original.hook, seconds: Number(result.metadata.duration_seconds), raw: result.clip.original.raw, raw_clean: result.clip.original.raw_clean, content_crop: result.clip.original.content_crop, file: 'videos/' + result.clip.id + '.mp4', srt: 'videos/' + result.clip.id + '.srt', narration_audio: (kind === 'ad' ? 'videos/audio/' : 'audio/') + result.clip.id + '/narration.wav', metadata_file: (kind === 'ad' ? 'videos/audio/' : 'audio/') + result.clip.id + '/metadata.json', previews: result.clip.previewNames.map(name => 'videos/qa/' + result.clip.id + '/' + name) }
      rows.push(data)
    }
    const report = { passed: true, method: '每条连续Serena整篇一次生成；统一resample、固定gain及可选整篇tempo处理后的连续PCM随后逐样本原序保留，仅在真实句间静音插零；未经逐句TTS、逐句gain或变调', single_generation_pass: true, voice: 'Serena', sample_rate: rate, fps, canonical_report: 'consistent-voice-report.json', results: rows }
    fs.writeFileSync(canonical, JSON.stringify(report, null, 2) + '\n')
    const names = kind === 'ad' ? ['video-report.json', 'voice-report.json'] : ['tutorial-render-report.json']
    for (const name of names) fs.writeFileSync(path.join(directory, 'videos', name), JSON.stringify(report, null, 2) + '\n')
  }
}

/** 用独立合成样本核对插零和采样原序保留，仅证明时轴算法，不作为真实音色验收。 */
function selfTest(): void {
  const source = new Int16Array(rate * 2); source.fill(2000, 0, rate * .7); source.fill(-3000, rate * 1.2, rate * 1.8)
  const segments: Segment[] = [{ index: 0, text: '测试一', start: 0, end: .7, coverage: 1 }, { index: 1, text: '测试二', start: 1.2, end: 1.8, coverage: 1 }]
  const result = insertSilence(source, segments, [60, 60]); assert.equal(result.audio.length, rate * 4); assert.equal(result.spans.reduce((sum, span) => sum + span.inserted_samples, 0), rate * 2); assert.ok(mapped(1.2, result.spans) > 2)
  console.log('自检通过：完整采样顺序和幅度保持一致，仅在静音插入零值；不代表真实音色验收。')
}

/** 构建指定7条tmp，全部验证后才按显式--apply替换交付文件；默认可先试听而不覆盖。 */
async function run(): Promise<void> {
  if (process.argv.includes('--self-test')) { selfTest(); return }
  originals(); const selected = (process.env.CONSISTENT_IDS || ids.join(',')).split(','); assert.ok(selected.length && selected.every(id => ids.includes(id)))
  const results: Result[] = []
  for (const id of selected) {
    const item = clip(id), result = item.kind === 'tutorial' ? await tutorial(item) : process.argv.includes('--reuse-tmp') ? await cached(item) : undefined
    results.push(result || await render(item))
  }
  if (process.argv.includes('--apply')) { deliver(results); console.log('全部所选tmp验证通过，交付媒体/音频/报告/实际QA已替换。') } else console.log('试听版仅在.run/consistent-render，尚未覆盖交付文件。')
}
run().catch((error: unknown) => { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 })
