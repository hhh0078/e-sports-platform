/** 文件作用：使用已采集的真实界面手机图及逐镜头口播文案，生成四类受众的30秒竖屏字幕中间视频，核验编码、时长与关键帧，供后续配音合流；不修改业务数据。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import os from 'node:os'
import { createRequire } from 'node:module'
import { spawn, spawnSync } from 'node:child_process'
import { once } from 'node:events'
import { createServer } from 'node:http'
import type { Canvas, Image, SKRSContext2D } from '@napi-rs/canvas'

/** 一个镜头只展示现有手机图及对应的准确功能字幕。 */
interface Scene { phone: string; title: string[]; detail: string; note?: string }
/** 每个角色视频固定为3秒钩子、五段功能介绍及4.5秒收尾。 */
interface VideoPlan { id: string; audience: string; hook: string; scenes: Scene[]; disclaimer?: string }
/** ffprobe返回的验证字段，未知外部数据仅在检查后使用。 */
interface Probe { format?: { duration?: string }; streams?: Array<{ codec_type?: string; codec_name?: string; width?: number; height?: number; avg_frame_rate?: string; nb_frames?: string }> }

const root = process.cwd(), output = path.resolve(root, process.argv[2] || 'output/playwright/marketing-20261026')
const modules = process.env.MARKETING_NODE_MODULES || path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules')
const require = createRequire(import.meta.url)
const canvasModule = require(path.join(modules, '@napi-rs/canvas')) as typeof import('@napi-rs/canvas')
const { createCanvas, loadImage, GlobalFonts } = canvasModule
const width = 1080, height = 1920, fps = 30, frameCount = 900
const videos = path.join(output, 'videos'), temporary = path.join(root, '.run/marketing-video-render')
const previewTimes = [0, 3, 8, 15, 24, 29]
const plans: VideoPlan[] = [
  { id: '01-platform-admin', audience: '平台管理员', hook: '想做自己的游戏接单平台？', scenes: [
    { phone: 'admin-console', title: ['想做自己的', '游戏接单平台？'], detail: '先看后台能不能管好' },
    { phone: 'admin-console', title: ['经营与待办', '一屏看清'], detail: '用户、订单、趋势与待办集中展示' },
    { phone: 'admin-users', title: ['用户与权限', '分开管理'], detail: '用户列表、资料与管理入口清晰可查' },
    { phone: 'admin-orders', title: ['订单与售后', '有处可查'], detail: '按订单查看进度、退款与争议' },
    { phone: 'content-center', title: ['轮播与公告', '按需配置'], detail: '运营图、文字与公告独立管理' },
    { phone: 'content-articles', title: ['图文内容', '持续运营'], detail: '文章列表与发布状态集中管理' },
    { phone: 'admin-console', title: ['了解源码', '搭建自己的平台'], detail: '开机电竞 · 多游戏服务小程序' }
  ] },
  { id: '02-club-admin', audience: '俱乐部管理员', hook: '俱乐部接单，怎么管得更省心？', scenes: [
    { phone: 'club-console', title: ['俱乐部接单', '怎么管得更省心？'], detail: '从主页展示，到团队日常协作' },
    { phone: 'club-home', title: ['俱乐部主页', '展示团队特色'], detail: '介绍、游戏范围与服务统一展示' },
    { phone: 'club-console', title: ['管理工作台', '入口更集中'], detail: '成员、申请与订单，按职责处理' },
    { phone: 'club-funds', title: ['佣金与押金', '分别记账'], detail: '团队资金分区展示，状态清楚可查' },
    { phone: 'order-chat', title: ['订单独立聊天', '沟通围绕同一单'], detail: '交易约定与协作消息保留在订单内' },
    { phone: 'order-list', title: ['订单进度', '随时查看'], detail: '从接单到履约，按订单跟进' },
    { phone: 'club-home', title: ['俱乐部入驻', '或独立部署'], detail: '了解系统，让团队协作更有条理' }
  ] },
  { id: '03-game-enthusiast', audience: '游戏爱好者', hook: '游戏打得好，也能接单？', disclaimer: '收入取决于实际成交 · 须完成资料核验', scenes: [
    { phone: 'service-publish', title: ['游戏打得好', '也能接单？'], detail: '把擅长的游戏，整理成一项服务' },
    { phone: 'service-publish', title: ['接单无需', '平台押金'], detail: '先完善资料，再发布适合自己的服务', note: '须符合平台资料核验及交易规则' },
    { phone: 'personal-center', title: ['个人工作台', '管理自己的服务'], detail: '个人资料、服务入口与订单集中查看' },
    { phone: 'order-list', title: ['有单之后', '按进度履约'], detail: '查看订单状态，确认服务约定' },
    { phone: 'order-chat', title: ['服务怎么做', '订单内聊清楚'], detail: '独立聊天室，让沟通对应具体订单' },
    { phone: 'wallet-overview', title: ['余额与结算', '分别展示'], detail: '收入状态与可提现金额，按规则查看' },
    { phone: 'service-publish', title: ['把爱好变成收入', '从一单服务开始'], detail: '开机电竞 · 多游戏服务小程序' }
  ] },
  { id: '04-player', audience: '玩家', hook: '想找陪玩护航，预算不想拉满？', disclaimer: '演示价格，实际以服务详情为准', scenes: [
    { phone: 'service-home', title: ['想找陪玩护航', '预算不想拉满？'], detail: '低价找陪玩，护航按需选' },
    { phone: 'service-home', title: ['多款游戏', '先挑合适队友'], detail: '看分类、服务范围与展示价格' },
    { phone: 'service-detail', title: ['陪玩演示价', '¥12 / 小时'], detail: '先看服务详情，再确认游戏与范围' },
    { phone: 'order-detail', title: ['约定与进度', '订单里看清'], detail: '服务内容、价格和履约状态分别展示' },
    { phone: 'order-chat', title: ['开局前', '先把需求聊清楚'], detail: '订单独立聊天，沟通有具体上下文' },
    { phone: 'club-home', title: ['想找团队', '也能看俱乐部'], detail: '浏览团队主页，选择适合自己的服务' },
    { phone: 'service-home', title: ['挑选合适队友', '查看服务与价格'], detail: '开机电竞 · 多游戏服务小程序' }
  ] }
]

/** 读取四类受众的口播脚本，逐一校验视频编号、镜头数量和完整字幕，避免错配配音。 */
function readVoiceScripts(): Map<string, string[]> {
  const raw: unknown = JSON.parse(fs.readFileSync(path.join(root, 'scripts/marketing-voice-script.json'), 'utf8'))
  assert.ok(raw && typeof raw === 'object' && 'videos' in raw && Array.isArray(raw.videos), '口播脚本缺少videos列表')
  const result = new Map<string, string[]>()
  for (const item of raw.videos as unknown[]) {
    assert.ok(item && typeof item === 'object' && 'id' in item && typeof item.id === 'string' && 'segments' in item && Array.isArray(item.segments), '口播脚本的视频编号或镜头文案无效')
    const id = item.id, plan = plans.find(value => value.id === id)
    assert.ok(plan, '口播脚本包含未知视频编号：' + id)
    assert.ok(!result.has(id), '口播脚本包含重复视频编号：' + id)
    const segments = item.segments.map((value: unknown) => {
      assert.ok(typeof value === 'string' && value.trim(), id + '存在空白口播字幕')
      return value.trim()
    })
    assert.equal(segments.length, plan.scenes.length, id + '口播段数与镜头数量不一致')
    result.set(id, segments)
  }
  assert.deepEqual([...result.keys()].sort(), plans.map(plan => plan.id).sort(), '口播脚本未覆盖全部四个视频')
  return result
}

/** 在已有工具中选择可运行的编码器和探测器，不安装软件或依赖。 */
function binaries(): { ffmpeg: string; ffprobe: string; jpegEncoder: string; direct: boolean } {
  const candidates = [process.env.MARKETING_FFMPEG, '/opt/homebrew/bin/ffmpeg', '/usr/local/bin/ffmpeg', '/Applications/TRAE SOLO CN.app/Contents/Resources/app/bin/ffmpeg', '/Applications/Trae CN.app/Contents/Resources/app/bin/ffmpeg'].filter((item): item is string => !!item)
  const ffmpeg = candidates.find(file => fs.existsSync(file) && spawnSync(file, ['-version'], { stdio: 'ignore' }).status === 0)
  assert.ok(ffmpeg, '没有可运行的H264编码器，可用MARKETING_FFMPEG指定现有ffmpeg')
  const ffprobe = process.env.MARKETING_FFPROBE || path.join(path.dirname(ffmpeg), 'ffprobe')
  assert.ok(fs.existsSync(ffprobe), '缺少ffprobe，可用MARKETING_FFPROBE指定现有工具')
  const demuxers = spawnSync(ffmpeg, ['-hide_banner', '-demuxers'], { encoding: 'utf8' }).stdout || ''
  const direct = demuxers.includes('image2pipe')
  const jpegEncoder = direct ? ffmpeg : path.join(os.homedir(), 'Library/Caches/ms-playwright/ffmpeg-1009/ffmpeg-mac')
  assert.ok(fs.existsSync(jpegEncoder), '缺少现有的JPEG帧输入编码器')
  const encoders = spawnSync(ffmpeg, ['-hide_banner', '-encoders'], { encoding: 'utf8' }).stdout || ''
  assert.ok(encoders.includes('libx264'), '输出编码器不支持libx264')
  return { ffmpeg, ffprobe, jpegEncoder, direct }
}

/** 绘制圆角标签和进度线，统一采用自然绿配色。 */
function rounded(context: SKRSContext2D, x: number, y: number, w: number, h: number, radius: number, color: string): void {
  context.fillStyle = color; context.beginPath(); context.roundRect(x, y, w, h, radius); context.fill()
}

/** 在限定宽度中绘制居中单行文案，长文字只等比缩字号，不裁掉字符。 */
function text(context: SKRSContext2D, value: string, y: number, size: number, color: string, weight = 500, limit = 910): void {
  let actual = size
  // 当前Canvas字体解析器只稳定支持整百字重，避免把750误识别为字体尺寸。
  const supportedWeight = Math.round(weight / 100) * 100
  context.font = `${supportedWeight} ${actual}px "Marketing Chinese", sans-serif`
  while (context.measureText(value).width > limit && actual > 24) { actual -= 1; context.font = `${supportedWeight} ${actual}px "Marketing Chinese", sans-serif` }
  assert.ok(context.measureText(value).width <= limit, '字幕超过安全区域：' + value)
  context.fillStyle = color; context.textAlign = 'center'; context.fillText(value, width / 2, y)
}

/** 将现有手机图与准确文案绘制成一个镜头，运镜只轻微放大手机图，不改动界面。 */
function drawScene(canvas: Canvas, plan: VideoPlan, index: number, phone: Image, progress: number, subtitle: string): void {
  const context = canvas.getContext('2d'), scene = plan.scenes[index], hook = index === 0
  const background = hook ? '#123d2d' : '#f2f4f1', ink = hook ? '#ffffff' : '#173d2d', muted = hook ? '#c6dfd0' : '#5b7065'
  context.globalAlpha = 1; context.fillStyle = background; context.fillRect(0, 0, width, height)
  context.fillStyle = hook ? '#204d3b' : '#e4ebe4'; context.beginPath(); context.ellipse(540, 1120, 460, 655, 0, 0, Math.PI * 2); context.fill()
  rounded(context, 100, 91, 9, 39, 4.5, hook ? '#adf1bd' : '#2b7850')
  context.font = '500 29px "Marketing Chinese", sans-serif'; context.fillStyle = muted; context.textAlign = 'left'; context.fillText('开机电竞 · ' + plan.audience, 128, 122)
  context.textAlign = 'right'; context.fillText(index === 0 ? '先看这30秒' : `${String(index).padStart(2, '0')} / 06`, 976, 122)
  text(context, scene.title[0], 240, hook ? 86 : 78, ink, 750)
  text(context, scene.title[1] || '', 343, hook ? 86 : 78, ink, 750)
  text(context, scene.detail, 419, 33, muted, 500)
  const phoneHeight = 1140 * (1 + progress * .022), phoneWidth = phoneHeight * 1329 / 2700
  const left = (width - phoneWidth) / 2, top = 490 - progress * 8
  context.save(); context.shadowColor = hook ? '#001c1466' : '#102c2030'; context.shadowBlur = 35; context.shadowOffsetY = 18
  context.drawImage(phone, left, top, phoneWidth, phoneHeight); context.restore()
  rounded(context, 100, 1670, 880, 50, 14, '#0a3024e6')
  text(context, subtitle, 1706, 34, '#ffffff', 600, 840)
  text(context, '功能演示 · 演示数据', 1758, 28, muted, 500)
  const note = scene.note || plan.disclaimer
  if (note) text(context, note, 1803, 25, muted, 450, 930)
  rounded(context, 100, 1848, 880, 5, 2.5, hook ? '#4e7763' : '#d6e0d6')
}

/** 将逐帧JPEG写入编码器，处理反压并限制日志长度，避免缓存整个视频到内存。 */
async function encodeFrames(plan: VideoPlan, phones: Map<string, Image>, tool: ReturnType<typeof binaries>, subtitles: string[]): Promise<string> {
  const destination = path.join(videos, plan.id + '.mp4'), intermediate = path.join(temporary, plan.id + '.webm')
  const args = tool.direct
    ? ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'image2pipe', '-r', String(fps), '-c:v', 'mjpeg', '-i', 'pipe:0', '-frames:v', String(frameCount), '-an', '-c:v', 'libx264', '-preset', 'fast', '-crf', '20', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', destination]
    : ['-y', '-hide_banner', '-loglevel', 'error', '-f', 'image2pipe', '-r', String(fps), '-c:v', 'mjpeg', '-i', 'pipe:0', '-frames:v', String(frameCount), '-an', '-c:v', 'libvpx', '-deadline', 'realtime', '-cpu-used', '8', '-b:v', '4000k', '-pix_fmt', 'yuv420p', intermediate]
  const encoder = spawn(tool.jpegEncoder, args, { stdio: ['pipe', 'ignore', 'pipe'] })
  let stderr = ''
  encoder.stderr.on('data', (bytes: Buffer) => { stderr = (stderr + bytes.toString()).slice(-12000) })
  let pipeError: Error | undefined
  encoder.stdin.on('error', error => { pipeError = error })
  const closed = once(encoder, 'close')
  const layer = createCanvas(width, height), previous = createCanvas(width, height), blended = createCanvas(width, height)
  const starts = [0, 90, 225, 360, 495, 630, 765], ends = [...starts.slice(1), frameCount]
  let lastScene = -1
  try {
    for (let frame = 0; frame < frameCount; frame++) {
      const index = starts.findIndex((start, offset) => frame >= start && frame < ends[offset])
      assert.ok(index >= 0)
      if (index !== lastScene && lastScene >= 0) previous.getContext('2d').drawImage(layer, 0, 0)
      lastScene = index
      const phone = phones.get(plan.scenes[index].phone); assert.ok(phone, '缺少手机图')
      const local = frame - starts[index], progress = local / (ends[index] - starts[index] - 1)
      drawScene(layer, plan, index, phone, progress, subtitles[index])
      const context = blended.getContext('2d')
      if (index > 0 && local < 12) {
        context.globalAlpha = 1; context.drawImage(previous, 0, 0); context.globalAlpha = (local + 1) / 12; context.drawImage(layer, 0, 0); context.globalAlpha = 1
      } else context.drawImage(layer, 0, 0)
      const bytes = blended.toBuffer('image/jpeg', 91)
      if (pipeError) throw pipeError
      if (!encoder.stdin.write(bytes)) await once(encoder.stdin, 'drain')
      if (frame % 150 === 0) console.log(plan.id + '：' + Math.round(frame / frameCount * 100) + '%')
    }
    encoder.stdin.end()
    const [code] = await closed
    assert.equal(code, 0, '帧编码失败：' + stderr)
  } catch (error) { encoder.stdin.destroy(); encoder.kill('SIGKILL'); throw error }
  if (!tool.direct) {
    const converted = spawnSync(tool.ffmpeg, ['-y', '-hide_banner', '-loglevel', 'error', '-i', intermediate, '-vf', 'fps=30', '-frames:v', String(frameCount), '-t', '30', '-an', '-c:v', 'libx264', '-preset', 'fast', '-crf', '20', '-pix_fmt', 'yuv420p', '-movflags', '+faststart', destination], { encoding: 'utf8', maxBuffer: 1024 * 1024, timeout: 300000 })
    assert.equal(converted.status, 0, 'H264转换失败：' + converted.stderr)
    fs.unlinkSync(intermediate)
  }
  return destination
}

/** 读取实际MP4，严格核验900帧、30fps、1080×1920、H264和30秒。 */
function verifyVideo(file: string, tool: ReturnType<typeof binaries>): Probe {
  const result = spawnSync(tool.ffprobe, ['-v', 'error', '-show_entries', 'format=duration:stream=codec_type,codec_name,width,height,avg_frame_rate,nb_frames', '-of', 'json', file], { encoding: 'utf8' })
  assert.equal(result.status, 0, 'ffprobe读取失败：' + result.stderr)
  const data = JSON.parse(result.stdout) as Probe, stream = data.streams?.find(item => item.codec_type === 'video')
  assert.ok(stream); assert.equal(stream.codec_name, 'h264'); assert.equal(stream.width, width); assert.equal(stream.height, height)
  assert.equal(stream.avg_frame_rate, '30/1'); assert.equal(Number(stream.nb_frames), frameCount)
  assert.ok(Math.abs(Number(data.format?.duration) - 30) < .001, '视频时长不是30秒')
  assert.equal(data.streams?.filter(item => item.codec_type === 'audio').length, 0, '当前渲染阶段应为无音轨中间视频，配音由后续合流加入')
  return data
}

/** 从实际交付MP4解码关键帧，使用本机浏览器读取H264，不以源图替代编码后的画面。 */
async function extractPreviews(files: string[]): Promise<void> {
  const { chromium } = require(path.join(modules, 'playwright')) as typeof import('playwright')
  const available = new Map(files.map(file => [path.basename(file), file]))
  const server = createServer((request, response) => {
    const url = new URL(request.url || '/', 'http://127.0.0.1'), name = url.pathname.slice(1)
    if (!name) { response.setHeader('Content-Type', 'text/html'); response.end('<!doctype html><html><body style="margin:0"><video crossorigin="anonymous" muted preload="auto"></video></body></html>'); return }
    const file = available.get(name)
    if (!file) { response.statusCode = 404; response.end(); return }
    const size = fs.statSync(file).size, range = request.headers.range?.match(/^bytes=(\d+)-(\d*)$/)
    const start = range ? Number(range[1]) : 0, end = range && range[2] ? Math.min(Number(range[2]), size - 1) : size - 1
    if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start < 0 || end < start || end >= size) { response.statusCode = 416; response.setHeader('Content-Range', 'bytes */' + size); response.end(); return }
    response.statusCode = range ? 206 : 200
    response.setHeader('Content-Type', 'video/mp4'); response.setHeader('Accept-Ranges', 'bytes'); response.setHeader('Content-Length', end - start + 1)
    if (range) response.setHeader('Content-Range', `bytes ${start}-${end}/${size}`)
    fs.createReadStream(file, { start, end }).pipe(response)
  })
  server.listen(0, '127.0.0.1'); await once(server, 'listening')
  const address = server.address(); assert.ok(address && typeof address !== 'string')
  const browser = await chromium.launch({ headless: true, channel: 'chrome' })
  try {
    const page = await browser.newPage()
    for (const file of files) {
      const directory = path.join(videos, 'qa', path.basename(file, '.mp4')); fs.mkdirSync(directory, { recursive: true })
      await page.goto('http://127.0.0.1:' + address.port)
      await page.evaluate(async name => {
        const video = document.querySelector('video'); if (!video) throw new Error('没有视频元素')
        video.src = '/' + name
        await new Promise<void>((resolve, reject) => { video.onloadeddata = () => resolve(); video.onerror = () => reject(new Error('H264视频解码失败')) })
      }, path.basename(file))
      for (const time of previewTimes) {
        const data = await page.evaluate(async target => {
          const video = document.querySelector('video'); if (!video) throw new Error('没有视频元素')
          await new Promise<void>(resolve => { video.onseeked = () => resolve(); video.currentTime = target === 0 ? .001 : target })
          const canvas = document.createElement('canvas'); canvas.width = video.videoWidth; canvas.height = video.videoHeight
          const context = canvas.getContext('2d'); if (!context) throw new Error('没有Canvas上下文')
          context.drawImage(video, 0, 0); return canvas.toDataURL('image/png').split(',')[1]
        }, time)
        fs.writeFileSync(path.join(directory, String(time).padStart(2, '0') + 's.png'), Buffer.from(data, 'base64'))
      }
      console.log('已从实际MP4提取关键帧：' + path.basename(file))
    }
  } finally { await browser.close(); await new Promise<void>(resolve => server.close(() => resolve())) }
}

/** 完成四个明确受众的视频，逐项记录准确字幕、编码验证与实际MP4关键帧。 */
async function run(): Promise<void> {
  fs.mkdirSync(videos, { recursive: true }); fs.mkdirSync(temporary, { recursive: true })
  assert.ok(GlobalFonts.registerFromPath('/System/Library/Fonts/Hiragino Sans GB.ttc', 'Marketing Chinese'), '中文字体加载失败')
  const tool = binaries(), phones = new Map<string, Image>(), requested = process.env.MARKETING_VIDEO_IDS?.split(','), voiceScripts = readVoiceScripts()
  const selected = plans.filter(plan => !requested || requested.includes(plan.id))
  assert.ok(selected.length > 0 && (!requested || selected.length === new Set(requested).size), 'MARKETING_VIDEO_IDS包含未知视频编号')
  for (const plan of plans) {
    assert.equal(plan.scenes.length, 7)
    for (const scene of plan.scenes) assert.ok(fs.existsSync(path.join(output, 'phones', scene.phone + '.png')), '等待手机素材：' + scene.phone)
  }
  const updated: string[] = []
  for (const plan of selected) {
    for (const scene of plan.scenes) if (!phones.has(scene.phone)) {
      const image = await loadImage(path.join(output, 'phones', scene.phone + '.png'))
      assert.equal(image.width, 1329); assert.equal(image.height, 2700); phones.set(scene.phone, image)
    }
    const subtitles = voiceScripts.get(plan.id); assert.ok(subtitles, '缺少对应口播字幕：' + plan.id)
    const file = await encodeFrames(plan, phones, tool, subtitles); updated.push(file)
    verifyVideo(file, tool)
    console.log('视频编码与规格核验通过：' + plan.id)
  }
  await extractPreviews(updated)
  const results = plans.map(plan => {
    const file = path.join(videos, plan.id + '.mp4')
    return { id: plan.id, audience: plan.audience, hook: plan.hook, file: path.relative(output, file), probe: verifyVideo(file, tool) }
  })
  fs.writeFileSync(path.join(videos, 'video-report.json'), JSON.stringify({ passed: true, size: { width, height }, fps, duration_seconds: 30, frames: frameCount, audio: '无音轨中间视频，完整口播字幕已烧录，待配音合流', source: '当前H5真实页面截图套设备展示框；全部私有数据为本地演示，非微信真机录屏。', rendering: '逐帧流式编码，手机轻运镜和镜头淡化；不缓存900帧。', narration_subtitles: { source: 'scripts/marketing-voice-script.json', segments: Object.fromEntries(voiceScripts) }, tools: tool, preview_times_seconds: previewTimes, results, plans }, null, 2) + '\n')
  console.log('4个30秒竖屏中间视频完成，完整口播字幕、实际MP4关键帧与核验报告已保存，待配音合流。')
}

run().catch((error: unknown) => { console.error(error instanceof Error ? error.stack : String(error)); process.exitCode = 1 })
