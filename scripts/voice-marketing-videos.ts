/** 文件作用：把七镜头真实配音拼成30秒旁白，统一音量并无损合流到四条宣传MP4，附完整SRT和可复用音频；不合成默认系统配音。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'

/** 一条角色视频的声音名称与七段完整配音文字。 */
interface VoicePlan { id: string; voice: string; segments: string[] }
/** PCM分析结果按10毫秒窗口判断有效声音，避免把整条静音当成配音。 */
interface Levels { seconds: number; peak: number; peak_dbfs: number; active_rms: number; active_seconds: number; first_active_seconds: number; last_active_seconds: number; longest_silence_seconds: number; clipped_samples: number }
/** 每镜头的实际时长、增益和完整播放边界。 */
interface SegmentReport { index: number; text: string; source: string; original_seconds: number; trimmed_seconds: number; slot_seconds: number; start_seconds: number; end_seconds: number; gain_db: number; levels: Levels }
/** ffprobe仅使用本次验收所需的编码和时长字段。 */
interface Probe { format?: { duration?: string }; streams?: Array<{ codec_type?: string; codec_name?: string; width?: number; height?: number; avg_frame_rate?: string; nb_frames?: string; duration?: string; sample_rate?: string; channels?: number }> }

const root = process.cwd(), output = path.resolve(root, process.argv.slice(2).find(value => !value.startsWith('--')) || 'output/playwright/marketing-20261026')
const videos = path.join(output, 'videos'), voiceRoot = path.join(root, '.run/marketing-voice'), temporary = path.join(root, '.run/marketing-voice-mix')
const scriptFile = process.env.MARKETING_VOICE_SCRIPT || path.join(voiceRoot, 'script.json')
const ids = ['01-platform-admin', '02-club-admin', '03-game-enthusiast', '04-player']
const starts = [0, 3, 7.5, 12, 16.5, 21, 25.5], ends = [...starts.slice(1), 30]
const sampleRate = 48000, totalSamples = sampleRate * 30, peakCeiling = 10 ** (-2 / 20), targetRms = 10 ** (-20 / 20)

/** 校验JSON对象边界，不把任意外部值直接当作配音配置。 */
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }

/** 选择本机已有的编码工具，所有外部命令均通过参数数组执行。 */
function tools(): { ffmpeg: string; ffprobe: string; afconvert: string } {
  const candidates = [process.env.MARKETING_FFMPEG, '/opt/homebrew/bin/ffmpeg', '/usr/local/bin/ffmpeg', '/Applications/TRAE SOLO CN.app/Contents/Resources/app/bin/ffmpeg', '/Applications/Trae CN.app/Contents/Resources/app/bin/ffmpeg'].filter((value): value is string => !!value)
  const ffmpeg = candidates.find(file => fs.existsSync(file) && spawnSync(file, ['-version'], { stdio: 'ignore' }).status === 0)
  assert.ok(ffmpeg, '缺少可用ffmpeg，请用MARKETING_FFMPEG指定现有工具')
  const ffprobe = process.env.MARKETING_FFPROBE || path.join(path.dirname(ffmpeg), 'ffprobe'), afconvert = '/usr/bin/afconvert'
  assert.ok(fs.existsSync(ffprobe) && fs.existsSync(afconvert), '缺少ffprobe或macOS afconvert')
  return { ffmpeg, ffprobe, afconvert }
}

/** 执行本机工具，失败时保留清楚的诊断，禁止静默输出未完成文件。 */
function command(file: string, args: string[]): void {
  const result = spawnSync(file, args, { encoding: 'utf8', timeout: 120000, maxBuffer: 1024 * 1024 })
  assert.equal(result.status, 0, path.basename(file) + '执行失败：' + (result.stderr || result.error?.message || '退出码' + result.status))
}

/** 用系统转换器解码成统一48kHz、单声道16位PCM，兼容WAV、AIFF、MP3等音源。 */
function convertPCM(source: string, destination: string, tool: ReturnType<typeof tools>): Int16Array {
  assert.notEqual(path.resolve(source), path.resolve(destination), '不能覆盖原始配音')
  if (fs.existsSync(destination)) fs.unlinkSync(destination)
  command(tool.afconvert, [source, destination, '-f', 'WAVE', '-d', 'LEI16@48000', '-c', '1'])
  return readWav(fs.readFileSync(destination))
}

/** 解析WAV块而不假设固定44字节头；仅接受已转换的单声道16位PCM。 */
function readWav(bytes: Buffer): Int16Array {
  assert.ok(bytes.length >= 44 && bytes.toString('ascii', 0, 4) === 'RIFF' && bytes.toString('ascii', 8, 12) === 'WAVE', '配音不是有效RIFF/WAVE')
  let data: Buffer | undefined, format: Buffer | undefined
  for (let offset = 12; offset + 8 <= bytes.length;) {
    const name = bytes.toString('ascii', offset, offset + 4), length = bytes.readUInt32LE(offset + 4), end = offset + 8 + length
    assert.ok(end <= bytes.length, 'WAV块超出文件边界')
    if (name === 'fmt ') format = bytes.subarray(offset + 8, end)
    if (name === 'data') data = bytes.subarray(offset + 8, end)
    offset = end + length % 2
  }
  assert.ok(format && format.length >= 16 && data && data.length > 0 && data.length % 2 === 0, 'WAV缺少有效格式或采样数据')
  // CoreAudio解码AAC时可能写WAVE_EXTENSIBLE；只接受GUID明确声明的整数PCM子格式。
  const formatTag = format.readUInt16LE(0), extensiblePCM = formatTag === 65534 && format.length >= 40 && format.subarray(24, 40).equals(Buffer.from('0100000000001000800000aa00389b71', 'hex'))
  assert.ok(formatTag === 1 || extensiblePCM, 'WAV必须为整数PCM或PCM扩展格式'); assert.equal(format.readUInt16LE(2), 1, 'WAV必须为单声道')
  assert.equal(format.readUInt32LE(4), sampleRate, 'WAV采样率必须为48000'); assert.equal(format.readUInt16LE(14), 16, 'WAV必须为16位')
  const samples = new Int16Array(data.length / 2)
  for (let index = 0; index < samples.length; index++) samples[index] = data.readInt16LE(index * 2)
  return samples
}

/** 将统一PCM写成标准WAV，保证精确采样数对应精确时长。 */
function wav(samples: Int16Array): Buffer {
  const bytes = Buffer.alloc(44 + samples.length * 2)
  bytes.write('RIFF'); bytes.writeUInt32LE(bytes.length - 8, 4); bytes.write('WAVE', 8); bytes.write('fmt ', 12)
  bytes.writeUInt32LE(16, 16); bytes.writeUInt16LE(1, 20); bytes.writeUInt16LE(1, 22); bytes.writeUInt32LE(sampleRate, 24)
  bytes.writeUInt32LE(sampleRate * 2, 28); bytes.writeUInt16LE(2, 32); bytes.writeUInt16LE(16, 34); bytes.write('data', 36); bytes.writeUInt32LE(samples.length * 2, 40)
  for (let index = 0; index < samples.length; index++) bytes.writeInt16LE(samples[index], 44 + index * 2)
  return bytes
}

/** 统计峰值、有效语音电平和静音区间；阈值约-50dBFS，不靠文件存在冒充声音有效。 */
function levels(samples: Int16Array): Levels {
  const window = 480, threshold = 10 ** (-50 / 20) * 32768
  let peak = 0, activeSquare = 0, activeSamples = 0, first = -1, last = -1, silence = 0, longest = 0, clipped = 0
  for (let offset = 0; offset < samples.length; offset += window) {
    const end = Math.min(samples.length, offset + window)
    let square = 0
    for (let index = offset; index < end; index++) { const absolute = Math.abs(samples[index]); peak = Math.max(peak, absolute); square += samples[index] ** 2; if (absolute >= 32767) clipped++ }
    if (Math.sqrt(square / (end - offset)) >= threshold) { if (first < 0) first = offset; last = end; activeSquare += square; activeSamples += end - offset; silence = 0 }
    else { silence += end - offset; longest = Math.max(longest, silence) }
  }
  const normalizedPeak = peak / 32768
  return { seconds: samples.length / sampleRate, peak: normalizedPeak, peak_dbfs: normalizedPeak ? 20 * Math.log10(normalizedPeak) : -120, active_rms: activeSamples ? Math.sqrt(activeSquare / activeSamples) / 32768 : 0, active_seconds: activeSamples / sampleRate, first_active_seconds: first / sampleRate, last_active_seconds: last / sampleRate, longest_silence_seconds: longest / sampleRate, clipped_samples: clipped }
}

/** 仅修剪首尾静音并保留呼吸余量，以固定增益统一响度，保留说话人的情绪动态。 */
function normalize(source: Int16Array): { samples: Int16Array; gainDb: number; original: Levels; normalized: Levels } {
  const original = levels(source)
  assert.ok(original.active_seconds >= .2 && original.peak > .015 && original.active_rms > .003, '音源没有足够有效声音，不能用静音完成配音')
  assert.equal(original.clipped_samples, 0, '原始配音已削波，请更换音源')
  const begin = Math.max(0, Math.floor((original.first_active_seconds - .05) * sampleRate)), end = Math.min(source.length, Math.ceil((original.last_active_seconds + .12) * sampleRate))
  const selected = source.slice(begin, end), current = levels(selected), gain = Math.min(8, targetRms / current.active_rms, peakCeiling / current.peak)
  const fade = Math.round(sampleRate * .005)
  for (let index = 0; index < selected.length; index++) selected[index] = Math.round(selected[index] * gain * Math.min(1, index / fade, (selected.length - 1 - index) / fade))
  const normalized = levels(selected)
  assert.ok(normalized.peak <= peakCeiling + .0001 && normalized.clipped_samples === 0, '归一化后出现削波')
  return { samples: selected, gainDb: 20 * Math.log10(gain), original, normalized }
}

/** 读取完整配音文字，仅接受四个已存在的视频与每条七段非空句子。 */
function readPlans(): VoicePlan[] {
  const input: unknown = JSON.parse(fs.readFileSync(scriptFile, 'utf8'))
  assert.ok(record(input) && Array.isArray(input.videos), '配音文字清单需要videos数组')
  const plans = input.videos.map((value: unknown): VoicePlan => {
    assert.ok(record(value) && typeof value.id === 'string' && ids.includes(value.id) && Array.isArray(value.segments) && value.segments.length === 7, '每条配音需要已知id与七段文字')
    const segments = value.segments.map((item: unknown) => { const content = typeof item === 'string' ? item : record(item) ? item.text : undefined; assert.ok(typeof content === 'string' && content.trim() && content.length <= 500, '每段必须有完整配音文字'); return content.trim() })
    return { id: value.id, voice: typeof value.voice === 'string' && value.voice.trim() ? value.voice.trim() : '用户选定的情感配音', segments }
  })
  assert.equal(plans.length, ids.length, '必须提供四条配音'); assert.equal(new Set(plans.map(plan => plan.id)).size, ids.length, '配音id不能重复')
  return ids.map(id => { const plan = plans.find(item => item.id === id); assert.ok(plan); return plan })
}

/** 寻找实际提供的音源文件，不以系统默认语音补空缺。 */
function sourceFile(id: string, index: number): string {
  const directory = path.join(voiceRoot, id), source = ['wav', 'aiff', 'aif', 'mp3', 'm4a', 'aac'].map(extension => path.join(directory, index + '.' + extension)).find(file => fs.existsSync(file))
  assert.ok(source, `缺少真实配音：${id}/${index}.wav（也接受aiff/mp3/m4a）`)
  assert.ok(fs.statSync(source).size > 44 && fs.statSync(source).size < 100000000, '音源大小异常：' + source)
  return source
}

/** 格式化SRT时间戳，毫秒精度与镜头时间保持一致。 */
function timestamp(seconds: number): string {
  const ms = Math.round(seconds * 1000), hours = Math.floor(ms / 3600000), minutes = Math.floor(ms / 60000) % 60, whole = Math.floor(ms / 1000) % 60
  return [hours, minutes, whole].map(value => String(value).padStart(2, '0')).join(':') + ',' + String(ms % 1000).padStart(3, '0')
}

/** 拼接七个完整镜头句子，每段留出前后余量，拒绝挤压、变调或切掉句尾。 */
function mix(plan: VoicePlan, tool: ReturnType<typeof tools>): { wave: string; reports: SegmentReport[]; levels: Levels } {
  const directory = path.join(temporary, plan.id), destination = path.join(videos, 'audio', plan.id), samples = new Int16Array(totalSamples), reports: SegmentReport[] = []
  fs.mkdirSync(directory, { recursive: true }); fs.mkdirSync(destination, { recursive: true })
  for (let index = 0; index < 7; index++) {
    const source = sourceFile(plan.id, index), pcm = convertPCM(source, path.join(directory, index + '-pcm.wav'), tool), normalized = normalize(pcm)
    const available = ends[index] - starts[index] - .15
    assert.ok(normalized.samples.length / sampleRate <= available, `${plan.id}第${index + 1}镜头配音${(normalized.samples.length / sampleRate).toFixed(3)}秒，需不超过${available.toFixed(2)}秒。请缩短文字或用自然的更快语速重新生成；不会截话尾或改变音高。`)
    const offset = Math.round((starts[index] + .06) * sampleRate)
    samples.set(normalized.samples, offset)
    reports.push({ index, text: plan.segments[index], source: path.relative(root, source), original_seconds: normalized.original.seconds, trimmed_seconds: normalized.samples.length / sampleRate, slot_seconds: ends[index] - starts[index], start_seconds: offset / sampleRate, end_seconds: (offset + normalized.samples.length) / sampleRate, gain_db: normalized.gainDb, levels: normalized.normalized })
  }
  const measured = levels(samples)
  assert.equal(measured.seconds, 30); assert.equal(measured.clipped_samples, 0)
  assert.ok(measured.active_seconds >= 5 && measured.first_active_seconds < 1 && measured.last_active_seconds <= 29.9, '整条旁白静音或片尾没有安全余量')
  const wave = path.join(destination, 'narration.wav'); fs.writeFileSync(wave, wav(samples))
  fs.writeFileSync(path.join(videos, plan.id + '.srt'), plan.segments.map((sentence, index) => `${index + 1}\n${timestamp(starts[index])} --> ${timestamp(ends[index] - .001)}\n${sentence}\n`).join('\n'))
  return { wave, reports, levels: measured }
}

/** 编码一次AAC，再直接复制为可复用ADTS；M4A用于保留编码延迟信息并合流。 */
function encodeAudio(wave: string, tool: ReturnType<typeof tools>): { m4a: string; aac: string } {
  const m4a = path.join(path.dirname(wave), 'narration.m4a'), aac = path.join(path.dirname(wave), 'narration.aac')
  for (const target of [m4a, aac]) if (fs.existsSync(target)) fs.unlinkSync(target)
  command(tool.afconvert, [wave, m4a, '-f', 'm4af', '-d', 'aac', '-b', '128000', '-q', '127'])
  command(tool.afconvert, [m4a, aac, '-f', 'adts', '-d', '0'])
  return { m4a, aac }
}

/** 核验实际MP4保留900帧H264，并新增有效的48kHz单声道AAC音轨。 */
function probe(file: string, tool: ReturnType<typeof tools>): Probe {
  const result = spawnSync(tool.ffprobe, ['-v', 'error', '-show_entries', 'format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,duration,sample_rate,channels', '-of', 'json', file], { encoding: 'utf8' })
  assert.equal(result.status, 0, 'ffprobe读取失败：' + result.stderr)
  const value = JSON.parse(result.stdout) as Probe, video = value.streams?.find(stream => stream.codec_type === 'video'), audio = value.streams?.find(stream => stream.codec_type === 'audio')
  assert.ok(video && audio, '实际MP4缺少视频或声音轨道'); assert.equal(video.codec_name, 'h264'); assert.equal(video.width, 1080); assert.equal(video.height, 1920)
  assert.equal(video.avg_frame_rate, '30/1'); assert.equal(Number(video.nb_frames), 900); assert.ok(Math.abs(Number(value.format?.duration) - 30) < .001)
  assert.equal(audio.codec_name, 'aac'); assert.equal(Number(audio.sample_rate), sampleRate); assert.equal(audio.channels, 1); assert.ok(Math.abs(Number(audio.duration) - 30) < .03)
  return value
}

/** 通过PCM往返与归一化断言自检关键运算，测试音不作为最终配音输出。 */
function selfCheck(): void {
  const samples = new Int16Array(sampleRate)
  for (let index = 4800; index < 38400; index++) samples[index] = Math.round(Math.sin(index / sampleRate * 440 * Math.PI * 2) * 6000)
  const read = readWav(wav(samples)); assert.deepEqual(read, samples)
  const normalized = normalize(read); assert.ok(normalized.normalized.peak <= peakCeiling + .0001); assert.ok(normalized.samples.length <= samples.length)
  assert.equal(timestamp(7.5), '00:00:07,500'); assert.equal(timestamp(29.999), '00:00:29,999')
}

/** 先准备并核验所有配音与临时视频，全部通过后再替换四条成品，保留原有视频码流。 */
function run(): void {
  selfCheck(); const tool = tools()
  if (process.argv.includes('--check-tools')) { console.log('PCM/SRT自检与本机工具检查通过；未生成默认配音或修改视频。'); return }
  const plans = readPlans(), pending: Array<{ id: string; temp: string; target: string; probe: Probe; voice: string; segments: SegmentReport[]; pcm: Levels; decoded: Levels }> = []
  const existing: unknown = JSON.parse(fs.readFileSync(path.join(videos, 'video-report.json'), 'utf8'))
  assert.ok(record(existing) && Array.isArray(existing.results) && existing.results.length === 4, '既有视频报告格式无效')
  for (const value of existing.results) assert.ok(record(value) && typeof value.id === 'string' && ids.includes(value.id), '既有视频报告包含未知id')
  fs.mkdirSync(temporary, { recursive: true })
  for (const plan of plans) for (let index = 0; index < 7; index++) sourceFile(plan.id, index)
  const prepared = plans.map(plan => { const mixed = mix(plan, tool); return { plan, mixed, audio: encodeAudio(mixed.wave, tool) } })
  for (const { plan, mixed, audio } of prepared) {
    const target = path.join(videos, plan.id + '.mp4'), temp = path.join(videos, '.' + plan.id + '-voiced.tmp.mp4')
    assert.ok(fs.existsSync(target), '缺少原始视频：' + target)
    // AAC尾包可能让容器延长15毫秒；29.98秒截止仍保留900视频帧，只丢弃句尾静音中的最后AAC尾包。
    command(tool.ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', target, '-i', audio.m4a, '-map', '0:v:0', '-map', '1:a:0', '-c', 'copy', '-t', '29.98', '-movflags', '+faststart', temp])
    const metadata = probe(temp, tool), decodedSamples = convertPCM(temp, path.join(temporary, plan.id, 'actual-mp4-audio.wav'), tool), decoded = levels(decodedSamples)
    assert.ok(decoded.active_seconds >= 5 && decoded.peak > .015 && decoded.last_active_seconds <= 29.9 && decoded.clipped_samples === 0, '实际MP4音轨静音、削波或句尾被截断')
    for (let index = 0; index < 7; index++) assert.ok(levels(decodedSamples.slice(Math.round(starts[index] * sampleRate), Math.round(ends[index] * sampleRate))).active_seconds > .1, '镜头配音缺失：' + plan.id + '/' + index)
    pending.push({ id: plan.id, temp, target, probe: metadata, voice: plan.voice, segments: mixed.reports, pcm: mixed.levels, decoded })
    console.log('声音合流与实际解码核验通过：' + plan.id)
  }
  for (const item of pending) fs.renameSync(item.temp, item.target)
  const results = existing.results.map((value: unknown) => { assert.ok(record(value) && typeof value.id === 'string'); const voice = pending.find(item => item.id === value.id); assert.ok(voice); return { ...value, probe: voice.probe } })
  fs.writeFileSync(path.join(videos, 'video-report.json'), JSON.stringify({ ...existing, passed: true, audio: 'AAC配音，48kHz单声道；旁白PCM统一响度并检查实际MP4解码', results, voice_report: 'videos/voice-report.json', subtitles: '已保留成品画面中的烧录字幕，另附七镜头完整配音SRT。' }, null, 2) + '\n')
  fs.writeFileSync(path.join(videos, 'voice-report.json'), JSON.stringify({ passed: true, duration_seconds: 30, sample_rate: sampleRate, subtitles: '每条视频旁同名SRT，完整句子与七镜头同步', method: '真实音源固定增益统一音量，不变调、不挤压语速、不截话尾；AAC只编码一次，原H264视频码流copy合流。', checks: ['全部28段有实际声音', '每段完整放入镜头并保留句尾余量', 'PCM与实际MP4解码无削波', '实际MP4含AAC音轨且每镜头有声', '四条仍为900帧、30fps、1080×1920和30秒'], results: pending.map(({ temp: _temp, target: _target, ...value }) => value) }, null, 2) + '\n')
  console.log('四条情感配音已合流；30秒WAV、AAC、M4A与完整SRT均已导出。')
}

try { run() } catch (error: unknown) { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 }
