/** 文件作用：按教程清单剪辑真实浏览器录屏，以原速保留点击、输入和滚动，添加 iPhone 展示框、逐步骤字幕与真人声线旁白，输出教程 MP4、SRT 和核验关键帧；不操作业务页面或数据库。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'
import { spawn } from 'node:child_process'
import type { Canvas, Image, SKRSContext2D } from '@napi-rs/canvas'
import type sharpType from 'sharp'

/** 一段连续的真实操作及其完整解说，时间相对于原始录屏。 */
interface Step { id: string; title: string; narration: string; start: number; end: number; duration?: number }
/** 一条完整教程共用一个原始录屏文件，步骤按清单顺序呈现。 */
interface Tutorial { id: string; title: string; audience: string; raw: string; steps: Step[]; content_crop?: { x: number; y: number; width: number; height: number } }
/** 探测器只读取渲染验证需要的媒体字段。 */
interface Probe { format?: { duration?: string }; streams?: Array<{ codec_type?: string; codec_name?: string; width?: number; height?: number; avg_frame_rate?: string; nb_frames?: string; sample_rate?: string; channels?: number }> }
/** 步骤保留原始时间映射，供核对是否按原速剪辑和字幕对齐。 */
interface RenderedStep { id: string; title: string; narration: string; raw_start: number; raw_end: number; operation_seconds: number; voice_seconds: number; hold_seconds: number; start: number; end: number; frames: number; video: string; audio: string }

const root = process.cwd(), output = path.resolve(root, process.argv[2] || 'output/playwright/tutorials-20261026')
const modules = process.env.MARKETING_NODE_MODULES || path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules')
const require = createRequire(import.meta.url)
const { createCanvas, loadImage, GlobalFonts } = require(path.join(modules, '@napi-rs/canvas')) as typeof import('@napi-rs/canvas')
const sharp = require(path.join(modules, 'sharp')) as typeof sharpType
const ffmpeg = process.env.TUTORIAL_FFMPEG || path.join(root, '.run/media-tools/package/ffmpeg')
const ffprobe = process.env.TUTORIAL_FFPROBE || '/Applications/TRAE SOLO CN.app/Contents/Resources/app/bin/ffprobe'
const voices = path.resolve(root, process.env.TUTORIAL_VOICE_ROOT || '.run/tutorial-voice')
const temporary = path.join(root, '.run/tutorial-render'), videos = path.join(output, 'videos')
const width = 1080, height = 1920, fps = 30, phone = { left: 171, top: 210, width: 738, height: 1500 }
const screen = { left: 213, top: 250, width: 655, height: 1420 }, content = { left: 213, top: 348, width: 655, height: 1265 }
const frameFile = path.join(root, '.run/marketing-assets/iphone16-black.webp')
const smoke = process.env.TUTORIAL_SMOKE === '1'

/** 校验 JSON 的对象边界，不直接信任外部清单的类型。 */
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }

/** 读取非空中文文案，文件标识额外限制为安全的英数字名称。 */
function string(value: unknown, label: string, identifier = false): string {
  assert.ok(typeof value === 'string' && value.trim(), label + '不能为空')
  if (identifier) assert.match(value, /^[a-zA-Z0-9_-]+$/, label + '只能使用英文字母、数字、下划线或连字符')
  return value.trim()
}

/** 读取有效非负秒数，拒绝无穷大、负数和字符串。 */
function seconds(value: unknown, label: string): number { assert.ok(typeof value === 'number' && Number.isFinite(value) && value >= 0, label + '必须是有效非负秒数'); return value }

/** 加载真实录屏剪辑清单，只接受当前393×759视口及存在的原始视频。 */
function readManifest(): Tutorial[] {
  const raw: unknown = JSON.parse(fs.readFileSync(path.join(output, 'manifest.json'), 'utf8'))
  assert.ok(record(raw) && record(raw.viewport) && Array.isArray(raw.tutorials) && raw.tutorials.length > 0, '教程清单格式错误')
  assert.equal(raw.viewport.width, 393); assert.equal(raw.viewport.height, 759)
  const ids = new Set<string>()
  return raw.tutorials.map((value: unknown) => {
    assert.ok(record(value) && Array.isArray(value.steps) && value.steps.length > 0, '每条教程必须包含真实操作步骤')
    const id = string(value.id, '教程编号', true), source = string(value.raw, '原始录屏')
    assert.ok(!ids.has(id), '教程编号重复：' + id); ids.add(id)
    assert.ok(!path.isAbsolute(source) && !path.relative(output, path.resolve(output, source)).startsWith('..'), '原始录屏必须位于教程目录内')
    assert.ok(fs.statSync(path.join(output, source)).isFile(), '原始录屏不存在：' + source)
    const stepIds = new Set<string>()
    const steps = value.steps.map((item: unknown): Step => {
      assert.ok(record(item), '步骤信息无效')
      const step: Step = { id: string(item.id, '步骤编号', true), title: string(item.title, '步骤标题'), narration: string(item.narration, '完整旁白'), start: seconds(item.start, '步骤开始时间'), end: seconds(item.end, '步骤结束时间') }
      assert.ok(!stepIds.has(step.id), '步骤编号重复：' + step.id); stepIds.add(step.id)
      assert.ok(step.end > step.start, step.id + '结束时间必须晚于开始时间')
      if (item.duration !== undefined) step.duration = seconds(item.duration, '步骤最短展示时间')
      return step
    })
    const tutorial: Tutorial = { id, title: string(value.title, '教程标题'), audience: string(value.audience, '教程受众'), raw: source, steps }
    if (value.content_crop !== undefined) {
      assert.ok(record(value.content_crop), id + '实际画面裁剪字段无效')
      const crop = value.content_crop
      const area = { x: seconds(crop.x, '画面横坐标'), y: seconds(crop.y, '画面纵坐标'), width: seconds(crop.width, '实际画面宽度'), height: seconds(crop.height, '实际画面高度') }
      assert.ok(Object.values(area).every(Number.isInteger), '实际画面裁剪必须使用整数像素')
      assert.equal(area.width, 393); assert.equal(area.height, 759)
      tutorial.content_crop = area
    }
    return tutorial
  })
}

/** 仅按清单明确给出的画面区域裁掉录制器pad，先转RGBA并exact裁剪，保留奇数宽高的最右和最下像素。 */
function cropFilter(tutorial: Tutorial): string {
  const area = tutorial.content_crop
  return area ? `format=rgba,crop=w=${area.width}:h=${area.height}:x=${area.x}:y=${area.y}:exact=1,` : ''
}

/** 异步运行已有媒体工具，限定错误日志长度，避免视频编码阻塞整个代理。 */
async function command(binary: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = spawn(binary, args, { stdio: ['ignore', 'pipe', 'pipe'] })
    let stdout = '', stderr = ''
    child.stdout.on('data', (bytes: Buffer) => { stdout = (stdout + bytes.toString()).slice(-200000) })
    child.stderr.on('data', (bytes: Buffer) => { stderr = (stderr + bytes.toString()).slice(-20000) })
    child.on('error', reject)
    child.on('close', code => code === 0 ? resolve(stdout || stderr) : reject(new Error(path.basename(binary) + '执行失败：' + stderr)))
  })
}

/** 探测媒体真实尺寸、时长、编码和音轨，不能以文件存在代替验收。 */
async function probe(file: string): Promise<Probe> {
  return JSON.parse(await command(ffprobe, ['-v', 'error', '-show_entries', 'format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames,sample_rate,channels', '-of', 'json', file])) as Probe
}

/** 从RIFF块计算PCM或浮点WAV的精确时长，兼容现有精简ffprobe没有WAV解复用器的运行环境。 */
function waveSeconds(file: string): number {
  const bytes = fs.readFileSync(file)
  assert.ok(bytes.length >= 44 && bytes.toString('ascii', 0, 4) === 'RIFF' && bytes.toString('ascii', 8, 12) === 'WAVE', '旁白不是有效WAV：' + file)
  let rate = 0, data = 0
  for (let offset = 12; offset + 8 <= bytes.length;) {
    const id = bytes.toString('ascii', offset, offset + 4), length = bytes.readUInt32LE(offset + 4), end = offset + 8 + length
    assert.ok(end <= bytes.length, 'WAV数据块越界：' + file)
    if (id === 'fmt ') {
      assert.ok(length >= 16, 'WAV格式块太短：' + file)
      const tag = bytes.readUInt16LE(offset + 8), sampleRate = bytes.readUInt32LE(offset + 12), byteRate = bytes.readUInt32LE(offset + 16), block = bytes.readUInt16LE(offset + 20)
      assert.ok([1, 3, 65534].includes(tag) && sampleRate > 0 && block > 0 && byteRate === sampleRate * block, '旁白WAV必须为有效PCM或浮点采样：' + file)
      rate = byteRate
    }
    if (id === 'data') data += length
    offset = end + length % 2
  }
  assert.ok(rate > 0 && data > 0, 'WAV缺少采样内容：' + file)
  return data / rate
}

/** 在指定宽度内按真实字宽折行，最多两行，过长时逐级减字号而不截断字幕。 */
function lines(context: SKRSContext2D, value: string, size: number, limit: number): { text: string[]; size: number } {
  for (let actual = size; actual >= 28; actual--) {
    context.font = `500 ${actual}px "Tutorial Chinese", sans-serif`
    const result: string[] = []; let current = ''
    for (const character of value) {
      if (current && context.measureText(current + character).width > limit) { result.push(current); current = character } else current += character
    }
    if (current) result.push(current)
    if (result.length <= 2) {
      if (result.length === 2 && [...result[1]].length <= 3) {
        // 仅修正尾行孤字，优先在标点后平衡分行，完整原文和其它已验收的字幕布局保持一致。
        const characters = [...value]; let best = Infinity
        for (let split = 1; split < characters.length; split++) {
          const first = characters.slice(0, split).join(''), last = characters.slice(split).join('')
          if (/^[，。！？；：、,.!?;:）】》]/u.test(last)) continue
          const firstWidth = context.measureText(first).width, lastWidth = context.measureText(last).width
          if (firstWidth > limit || lastWidth > limit) continue
          const score = Math.abs(firstWidth - lastWidth) - (/[，。！？；：、,;:!?]$/u.test(first) ? limit / 2 : 0)
          if (score < best) { best = score; result[0] = first; result[1] = last }
        }
      }
      return { text: result, size: actual }
    }
  }
  throw new Error('教程字幕超过两行安全区，请缩短完整口播：' + value)
}

/** 绘制圆角面板，统一自然灰绿配色，不修改录屏内部界面。 */
function rounded(context: SKRSContext2D, x: number, y: number, w: number, h: number, radius: number, color: string): void {
  context.fillStyle = color; context.beginPath(); context.roundRect(x, y, w, h, radius); context.fill()
}

/** 写入居中两行文案，返回实际换行供独立SRT保持全文一致。 */
function centered(context: SKRSContext2D, value: string, y: number, size: number, limit: number, color: string): string[] {
  const wrapped = lines(context, value, size, limit)
  context.font = `500 ${wrapped.size}px "Tutorial Chinese", sans-serif`; context.fillStyle = color; context.textAlign = 'center'
  wrapped.text.forEach((line, index) => context.fillText(line, width / 2, y + index * (wrapped.size + 8)))
  return wrapped.text
}

/** 绘制展示用状态栏和手势条，位置按已验证iPhone16屏幕开口缩放。 */
function chrome(context: SKRSContext2D): void {
  context.save(); context.translate(screen.left, screen.top); context.scale(5 / 9, 5 / 9)
  context.fillStyle = '#ffffff'; context.textAlign = 'left'; context.font = '600 46px Arial, sans-serif'; context.fillText('9:41', 99, 113)
  for (let index = 0; index < 4; index++) rounded(context, 918 + index * 15, 100 - index * 8, 9, 13 + index * 8, 2, '#ffffff')
  context.strokeStyle = '#ffffff'; context.lineWidth = 6; context.lineCap = 'round'
  for (let index = 0; index < 3; index++) { context.beginPath(); context.moveTo(994 + index * 7, 88 + index * 8); context.quadraticCurveTo(1016, 68 + index * 15, 1038 - index * 7, 88 + index * 8); context.stroke() }
  context.beginPath(); context.arc(1016, 112, 4, 0, Math.PI * 2); context.fill()
  context.lineWidth = 3; context.strokeRect(1055, 79, 65, 33); rounded(context, 1061, 85, 53, 21, 4, '#ffffff'); rounded(context, 1125, 89, 5, 13, 2, '#ffffff')
  rounded(context, 387, 2520, 405, 15, 7.5, '#ffffff'); context.restore()
}

/** 生成步骤标题、完整口播字幕及静态设备外框；真实内容区域留给原视频。 */
async function graphics(tutorial: Tutorial, step: Step, index: number, directory: string, frame: Image): Promise<{ base: string; border: string; mask: string; caption: string[] }> {
  const canvas: Canvas = createCanvas(width, height), context = canvas.getContext('2d')
  context.fillStyle = '#f2f4f1'; context.fillRect(0, 0, width, height)
  context.font = '500 27px "Tutorial Chinese", sans-serif'; context.fillStyle = '#5b7065'; context.textAlign = 'left'; context.fillText('开机电竞 · ' + tutorial.audience, 65, 66)
  context.textAlign = 'right'; context.fillText(`${index + 1} / ${tutorial.steps.length}`, 1015, 66)
  centered(context, step.title, 132, 43, 950, '#173d2d')
  context.save(); context.shadowColor = '#102c2030'; context.shadowBlur = 22; context.shadowOffsetY = 12
  context.drawImage(frame, phone.left, phone.top, phone.width, phone.height); context.restore()
  rounded(context, screen.left, screen.top, screen.width, screen.height, 92, '#0a1110'); chrome(context)
  rounded(context, 55, 1730, 970, 106, 16, '#173d2d')
  const caption = centered(context, step.narration, 1771, 34, 920, '#ffffff')
  context.font = '500 23px "Tutorial Chinese", sans-serif'; context.fillStyle = '#617168'; context.textAlign = 'center'; context.fillText(smoke ? '动态测试图 · 非真实教程 · 仅验证渲染管线' : '真实页面录屏 · 演示数据 · 操作原速播放', width / 2, 1880)
  const base = path.join(directory, `base-${index}.png`), border = path.join(directory, 'device-border.png'), mask = path.join(directory, 'content-mask.png')
  fs.writeFileSync(base, canvas.toBuffer('image/png'))
  if (!fs.existsSync(border)) { context.clearRect(0, 0, width, height); context.drawImage(frame, phone.left, phone.top, phone.width, phone.height); fs.writeFileSync(border, canvas.toBuffer('image/png')) }
  if (!fs.existsSync(mask)) {
    const svg = Buffer.from('<svg xmlns="http://www.w3.org/2000/svg" width="1179" height="2556"><rect width="1179" height="2556" rx="165" fill="white"/></svg>')
    await sharp(svg).extract({ left: 0, top: 177, width: 1179, height: 2277 }).resize(content.width, content.height).flatten({ background: '#000000' }).removeAlpha().png().toFile(mask)
  }
  return { base, border, mask, caption }
}

/** 剪出单段原速操作，只在末帧补齐旁白所需时间，并分别写无声H264和精确采样WAV。 */
async function renderStep(tutorial: Tutorial, step: Step, index: number, directory: string, frame: Image, offset: number): Promise<RenderedStep> {
  const source = path.join(output, tutorial.raw), voice = path.join(voices, tutorial.id, index + '.wav')
  assert.ok(fs.existsSync(voice), '缺少对应年轻女声旁白：' + voice)
  const voiceSeconds = waveSeconds(voice)
  assert.ok(Number.isFinite(voiceSeconds) && voiceSeconds >= .2, '旁白文件没有有效音轨或时长：' + voice)
  const operation = step.end - step.start, frames = Math.ceil(Math.max(operation, voiceSeconds + .3, step.duration || 0) * fps), duration = frames / fps
  const layers = await graphics(tutorial, step, index, directory, frame), video = path.join(directory, `step-${index}.mp4`), wav = path.join(directory, `step-${index}.wav`)
  const filter = `[0:v]${cropFilter(tutorial)}trim=start=${step.start}:end=${step.end},setpts=PTS-STARTPTS,fps=${fps},scale=${content.width}:${content.height}:flags=lanczos,format=rgba,tpad=stop_mode=clone:stop_duration=${Math.max(0, duration - operation) + 1 / fps}[clip];[clip][3:v]alphamerge[masked];[1:v][masked]overlay=${content.left}:${content.top}:shortest=1[page];[page][2:v]overlay=0:0:shortest=1,format=yuv420p[outv]`
  // FFmpeg4.4的循环PNG多输出会阻塞尾段音频，音画分别生成再统一拼接，采样时轴仍严格一致。
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', voice, '-af', `aresample=48000,loudnorm=I=-18:TP=-2:LRA=11,apad,atrim=duration=${duration},asetpts=PTS-STARTPTS`, '-c:a', 'pcm_s16le', '-ar', '48000', '-ac', '1', wav])
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', source, '-loop', '1', '-framerate', String(fps), '-t', String(duration), '-i', layers.base, '-loop', '1', '-framerate', String(fps), '-t', String(duration), '-i', layers.border, '-loop', '1', '-framerate', String(fps), '-t', String(duration), '-i', layers.mask, '-filter_complex', filter, '-map', '[outv]', '-frames:v', String(frames), '-r', String(fps), '-t', String(duration), '-c:v', 'libx264', '-preset', 'fast', '-crf', '18', '-pix_fmt', 'yuv420p', '-an', video])
  const rendered = await probe(video), picture = rendered.streams?.find(stream => stream.codec_type === 'video')
  assert.equal(Number(picture?.nb_frames), frames, '步骤录屏帧数与原速时长不一致')
  assert.ok(Math.abs(waveSeconds(wav) - duration) < .002, '步骤WAV未精确填充到相同时间')
  return { id: step.id, title: step.title, narration: step.narration, raw_start: step.start, raw_end: step.end, operation_seconds: operation, voice_seconds: voiceSeconds, hold_seconds: duration - operation, start: offset, end: offset + duration, frames, video, audio: wav }
}

/** 另存无pad的净画面WebM供用户剪辑，VP9无损444支持393×759奇数尺寸，时间轴与原始录屏相同。 */
async function cleanRaw(tutorial: Tutorial, rawDuration: number): Promise<string | undefined> {
  if (!tutorial.content_crop) return undefined
  const directory = path.join(output, 'raw-clean'), target = path.join(directory, tutorial.id + '.webm')
  fs.mkdirSync(directory, { recursive: true })
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', path.join(output, tutorial.raw), '-vf', cropFilter(tutorial) + 'format=yuv444p', '-an', '-vsync', '0', '-c:v', 'libvpx-vp9', '-lossless', '1', '-deadline', 'realtime', '-cpu-used', '4', '-row-mt', '1', target])
  const media = await probe(target), video = media.streams?.find(stream => stream.codec_type === 'video')
  assert.equal(video?.width, 393); assert.equal(video?.height, 759)
  assert.ok(Math.abs(Number(media.format?.duration) - rawDuration) < .05, '净画面录屏与原始录屏时间轴不同')
  return path.relative(output, target)
}

/** 将自然秒数转换成标准SRT时间格式，避免秒数舍入跨分钟。 */
function srtTime(value: number): string {
  const milliseconds = Math.round(value * 1000), hours = Math.floor(milliseconds / 3600000), minutes = Math.floor(milliseconds / 60000) % 60, seconds = Math.floor(milliseconds / 1000) % 60
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')},${String(milliseconds % 1000).padStart(3, '0')}`
}

/** 拼接无声视频与精确WAV，AAC只编码一次，保持剪辑后字幕和旁白时轴一致。 */
async function assemble(tutorial: Tutorial, steps: RenderedStep[], directory: string): Promise<string> {
  const list = path.join(directory, 'video-list.txt'), audioList = path.join(directory, 'audio-list.txt'), target = path.join(videos, tutorial.id + '.mp4')
  // MP4格式时长只保留毫秒；显式给concat每段的帧级时长，避免累积舍入把30fps变成近似分数。
  fs.writeFileSync(list, steps.map(step => `file '${path.basename(step.video)}'\nduration ${step.frames / fps}`).join('\n') + '\n')
  fs.writeFileSync(audioList, steps.map(step => `file '${path.basename(step.audio)}'`).join('\n') + '\n')
  const audioDirectory = path.join(output, 'audio', tutorial.id), narration = path.join(audioDirectory, 'narration.wav')
  fs.mkdirSync(audioDirectory, { recursive: true })
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', audioList, '-c:a', 'pcm_s16le', '-ar', '48000', '-ac', '1', narration])
  assert.ok(Math.abs(waveSeconds(narration) - steps[steps.length - 1].end) < .002, '完整旁白时长与剪辑时轴不一致')
  await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', list, '-i', narration, '-map', '0:v:0', '-map', '1:a:0', '-c:v', 'copy', '-c:a', 'aac', '-b:a', '192k', '-ar', '48000', '-ac', '1', '-movflags', '+faststart', target])
  fs.writeFileSync(path.join(videos, tutorial.id + '.srt'), steps.map((step, index) => `${index + 1}\n${srtTime(step.start)} --> ${srtTime(step.end)}\n${step.narration}\n`).join('\n'))
  return target
}

/** 保留当前清单内其它已完成教程的报告，单条重做不清掉旧结果，测试素材和失效文件不得混入正式报告。 */
function preservedResults(tutorials: Tutorial[], selected: Tutorial[]): Record<string, unknown>[] {
  const file = path.join(videos, 'tutorial-render-report.json')
  if (!fs.existsSync(file)) return []
  const previous: unknown = JSON.parse(fs.readFileSync(file, 'utf8'))
  assert.ok(record(previous) && Array.isArray(previous.results), '已有教程报告格式错误，停止覆盖')
  if (!smoke && previous.kind === 'pipeline-smoke') return []
  const allowed = new Set(tutorials.map(tutorial => tutorial.id)), replaced = new Set(selected.map(tutorial => tutorial.id))
  return previous.results.filter((item: unknown): item is Record<string, unknown> => record(item) && typeof item.id === 'string' && allowed.has(item.id) && !replaced.has(item.id) && fs.existsSync(path.join(videos, item.id + '.mp4')))
}

/** 验证最终视频编码、画布、帧数和旁白音轨，并检查音量不是整条静音。 */
async function verify(file: string, steps: RenderedStep[]): Promise<{ media: Probe; max_volume_dbfs: number }> {
  const media = await probe(file), video = media.streams?.find(stream => stream.codec_type === 'video'), audio = media.streams?.find(stream => stream.codec_type === 'audio')
  assert.ok(video && audio, '最终教程必须同时有真实画面与旁白音轨')
  assert.equal(video.width, width); assert.equal(video.height, height); assert.equal(video.codec_name, 'h264'); assert.equal(video.avg_frame_rate, '30/1')
  assert.equal(Number(video.nb_frames), steps.reduce((sum, step) => sum + step.frames, 0)); assert.equal(audio.codec_name, 'aac'); assert.equal(Number(audio.sample_rate), 48000)
  assert.ok(Math.abs(Number(media.format?.duration) - steps[steps.length - 1].end) < .1, '最终时长与原速步骤和旁白时轴不一致')
  const meter = await command(ffmpeg, ['-hide_banner', '-i', file, '-vn', '-af', 'volumedetect', '-f', 'null', '-']), match = meter.match(/max_volume:\s*(-?[\d.]+) dB/)
  assert.ok(match && Number(match[1]) > -45, '最终教程缺少有效有声旁白')
  return { media, max_volume_dbfs: Number(match[1]) }
}

/** 从真正编码后的教程提取动作完成后的关键帧，避开弹层初始过渡；不以源PNG替代视频验收。 */
async function previews(tutorial: Tutorial, file: string, steps: RenderedStep[]): Promise<string[]> {
  const directory = path.join(videos, 'qa', tutorial.id); fs.mkdirSync(directory, { recursive: true })
  const files: string[] = []
  for (const [index, step] of steps.entries()) {
    const target = path.join(directory, String(index + 1).padStart(2, '0') + '-' + step.id + '.png')
    await command(ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-ss', String(step.start + Math.max(0, step.operation_seconds - .15)), '-i', file, '-frames:v', '1', target]); files.push(path.relative(output, target))
  }
  return files
}

/** 渲染清单内的真实教程，保存原速时间映射、完整字幕、旁白规格和实际视频关键帧。 */
async function run(): Promise<void> {
  assert.ok(fs.existsSync(ffmpeg) && fs.existsSync(ffprobe), '缺少完整ffmpeg或已有ffprobe，禁止用无滤镜编码器冒充剪辑')
  assert.ok(GlobalFonts.registerFromPath('/System/Library/Fonts/Hiragino Sans GB.ttc', 'Tutorial Chinese'), '中文字体加载失败')
  const frameMetadata = await sharp(frameFile).metadata(); assert.equal(frameMetadata.width, 1329); assert.equal(frameMetadata.height, 2700)
  const frame = await loadImage(frameFile), tutorials = readManifest(), selectedIds = process.env.TUTORIAL_IDS?.split(',')
  const selected = tutorials.filter(tutorial => !selectedIds || selectedIds.includes(tutorial.id))
  assert.ok(selected.length > 0 && (!selectedIds || selected.length === new Set(selectedIds).size), 'TUTORIAL_IDS包含未知教程')
  fs.mkdirSync(videos, { recursive: true })
  const results: Record<string, unknown>[] = []
  for (const tutorial of selected) {
    const directory = path.join(temporary, tutorial.id); fs.mkdirSync(directory, { recursive: true })
    const rawMedia = await probe(path.join(output, tutorial.raw)), rawVideo = rawMedia.streams?.find(stream => stream.codec_type === 'video'), rawDuration = Number(rawMedia.format?.duration)
    assert.ok(rawVideo?.width && rawVideo.height && rawVideo.width >= 393 && rawVideo.height >= 759, '原始录屏分辨率不足')
    if (tutorial.content_crop) {
      assert.ok(tutorial.content_crop.x + tutorial.content_crop.width <= rawVideo.width && tutorial.content_crop.y + tutorial.content_crop.height <= rawVideo.height, '明确指定的实际画面裁剪越过原始录屏边界')
    } else assert.ok(Math.abs(rawVideo.width / rawVideo.height - 393 / 759) < .002, '原始录屏比例与393×759视口不一致')
    const cleaned = await cleanRaw(tutorial, rawDuration)
    const steps: RenderedStep[] = []; let offset = 0
    for (const [index, step] of tutorial.steps.entries()) {
      assert.ok(step.end <= rawDuration + .05, step.id + '剪辑超出原始录屏长度')
      console.log(`${tutorial.id}：渲染步骤 ${index + 1}/${tutorial.steps.length} ${step.title}`)
      const rendered = await renderStep(tutorial, step, index, directory, frame, offset); steps.push(rendered); offset = rendered.end
    }
    const file = await assemble(tutorial, steps, directory), verified = await verify(file, steps), frames = await previews(tutorial, file, steps)
    results.push({ id: tutorial.id, title: tutorial.title, audience: tutorial.audience, file: path.relative(output, file), srt: path.relative(output, path.join(videos, tutorial.id + '.srt')), narration_audio: path.relative(output, path.join(output, 'audio', tutorial.id, 'narration.wav')), playback_rate: 1, seconds: offset, raw: tutorial.raw, raw_media: rawMedia, content_crop: tutorial.content_crop, raw_clean: cleaned, steps, verified, previews: frames })
    console.log(`${tutorial.id}：原速教程完成，${offset.toFixed(2)}秒，含完整旁白、烧录字幕与SRT。`)
  }
  const merged = new Map([...preservedResults(tutorials, selected), ...results].map(result => [String(result.id), result]))
  const saved = tutorials.map(tutorial => merged.get(tutorial.id)).filter(record)
  fs.writeFileSync(path.join(videos, 'tutorial-render-report.json'), JSON.stringify({ passed: true, kind: smoke ? 'pipeline-smoke' : 'real-ui-recording', source: smoke ? '动态测试图与测试音，仅验证媒体渲染管线，不是真实页面或真实女声教程。' : '当前H5真实页面连续录屏；数据为隔离环境演示，不代表微信真机或真实资金交易。', playback: '按原录屏时间范围剪辑，操作始终1x；旁白需要额外时间时只延长步骤末帧。', canvas: { width, height }, fps, voice_root: voices, frame_license: 'https://tryhoverify.com/tools/device-frames/license/', rendered_ids: selected.map(tutorial => tutorial.id), results: saved }, null, 2) + '\n')
}

run().catch((error: unknown) => { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 })
