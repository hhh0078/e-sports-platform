/** 文件作用：将功能截图按清单裁成圆角、叠加获准商用的 iPhone 边框，并输出单机图、三屏组合图及总览；不生成或修改业务界面。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createRequire } from 'node:module'
import type { OverlayOptions } from 'sharp' with { 'resolution-mode': 'require' }

/** sharp 的 CommonJS 工厂类型，与 createRequire 的加载方式保持一致。 */
type SharpFactory = typeof import('sharp', { with: { 'resolution-mode': 'require' } }).default

/** 截图清单中的真实页面及其简短展示名称。 */
interface Screen { id: string; label: string; file: string }
/** 一组功能对应的三张截图和标题。 */
interface Group { id: string; title: string; subtitle: string; screens: Screen[] }

const root = process.cwd()
const output = path.resolve(root, process.argv[2] || 'output/playwright/marketing-20261026')
const frameFile = path.join(root, '.run/marketing-assets/iphone16-black.webp')
const frameUrl = 'https://tryhoverify.com/images/devices/iphone_16/black.webp'
const licenseUrl = 'https://tryhoverify.com/tools/device-frames/license/'
const frameSize = { width: 1329, height: 2700 }
const screenSize = { width: 1179, height: 2556, left: 75, top: 72, radius: 165 }
const contentSize = { width: 1179, height: 2277, statusHeight: 177, homeHeight: 102 }
const canvas = { width: 2400, height: 1800 }
const sharp = loadSharp()

/** 加载现有 sharp，优先本项目或显式目录，其次使用 Codex 已安装运行时，不安装依赖。 */
function loadSharp(): SharpFactory {
  const require = createRequire(path.join(root, 'frontend/package.json'))
  const candidates = ['sharp', ...(process.env.MARKETING_NODE_MODULES ? [path.join(process.env.MARKETING_NODE_MODULES, 'sharp')] : []), path.join(os.homedir(), '.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/sharp')]
  for (const candidate of candidates) {
    try { return require(candidate) as SharpFactory } catch (error) {
      if (!(error instanceof Error) || !('code' in error) || error.code !== 'MODULE_NOT_FOUND') throw error
    }
  }
  throw new Error('未找到已安装的 sharp，请用 MARKETING_NODE_MODULES 指定包含 sharp 的 node_modules 目录。')
}

/** 验证外部 JSON 的对象边界，避免缺字段时静默生成残缺素材。 */
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

/** 校验清单内的文字字段与安全文件名，返回已缩窄的字符串。 */
function text(value: unknown, label: string, identifier = false): string {
  assert.ok(typeof value === 'string' && value.trim(), label + '不能为空')
  if (identifier) assert.match(value, /^[a-zA-Z0-9_-]+$/, label + '只能含英文字母、数字、下划线或连字符')
  return value.trim()
}

/** 读取三屏功能分组，并验证重复名称、相对路径和输入文件是否存在。 */
function readManifest(): Group[] {
  const manifest: unknown = JSON.parse(fs.readFileSync(path.join(output, 'manifest.json'), 'utf8'))
  assert.ok(isRecord(manifest) && Array.isArray(manifest.groups) && manifest.groups.length > 0, 'manifest.json 必须包含非空 groups')
  const groupIds = new Set<string>(), screens = new Map<string, Screen>()
  return manifest.groups.map((value: unknown) => {
    assert.ok(isRecord(value) && Array.isArray(value.screens) && value.screens.length === 3, '每组必须恰好包含三张真实截图')
    const id = text(value.id, '分组 id', true)
    assert.ok(!groupIds.has(id), '重复分组 id：' + id)
    groupIds.add(id)
    const group: Group = { id, title: text(value.title, id + ' 标题'), subtitle: text(value.subtitle, id + ' 副标题'), screens: [] }
    group.screens = value.screens.map((item: unknown) => {
      assert.ok(isRecord(item), id + ' 截图信息无效')
      const screen = { id: text(item.id, '截图 id', true), label: text(item.label, '截图标签'), file: text(item.file, '截图文件') }
      assert.ok(!path.isAbsolute(screen.file) && !path.relative(output, path.resolve(output, screen.file)).startsWith('..'), '截图必须位于输出目录内')
      assert.ok(fs.statSync(path.join(output, screen.file)).isFile(), '截图不存在：' + screen.file)
      const previous = screens.get(screen.id)
      assert.ok(!previous || previous.file === screen.file, '同一截图 id 不能指向不同文件：' + screen.id)
      screens.set(screen.id, screen)
      return screen
    })
    assert.equal(new Set(group.screens.map(screen => screen.id)).size, 3, id + ' 不能重复使用同一截图')
    return group
  })
}

/** 将正文安全嵌入 SVG，保留清单中的真实中文文案。 */
function xml(value: string): string {
  return value.replace(/[&<>"']/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&apos;' })[character] || character)
}

/** 将 SVG 作为文字、圆角遮罩或柔影图层交给 sharp 渲染。 */
function svg(width: number, height: number, content: string): Buffer {
  return Buffer.from(`<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">${content}</svg>`)
}

/** 下载并核验透明边框，只缓存到忽略目录，禁止将裸框加入售卖素材包。 */
async function prepareFrame(): Promise<void> {
  if (!fs.existsSync(frameFile)) {
    const response = await fetch(frameUrl, { signal: AbortSignal.timeout(30000) })
    assert.ok(response.ok, '边框下载失败：HTTP ' + response.status)
    const bytes = Buffer.from(await response.arrayBuffer())
    assert.ok(bytes.length > 0 && bytes.length < 10000000, '边框文件大小异常')
    const metadata = await sharp(bytes).metadata()
    assert.equal(metadata.width, frameSize.width, '下载边框宽度不匹配')
    assert.equal(metadata.height, frameSize.height, '下载边框高度不匹配')
    assert.equal(metadata.hasAlpha, true, '下载边框必须包含透明通道')
    fs.mkdirSync(path.dirname(frameFile), { recursive: true })
    fs.writeFileSync(frameFile, bytes)
  }
  const metadata = await sharp(frameFile).metadata()
  assert.equal(metadata.width, frameSize.width, '缓存边框宽度不匹配')
  assert.equal(metadata.height, frameSize.height, '缓存边框高度不匹配')
  assert.equal(metadata.hasAlpha, true, '缓存边框必须包含透明通道')
}

/** 按屏幕真实开口裁出圆角截图，再将透明手机边框覆盖其上。 */
async function composePhone(screen: Screen): Promise<string> {
  const source = path.join(output, screen.file), target = path.join(output, 'phones', screen.id + '.png')
  const metadata = await sharp(source).metadata()
  assert.equal(metadata.width, screenSize.width, screen.file + ' 必须是 393 宽视口的 3×截图（宽1179）')
  assert.ok(metadata.height === contentSize.height || metadata.height === screenSize.height, screen.file + ' 高度必须为2277（759视口）或2556（852视口）')
  let screenImage: Buffer | string = source
  if (metadata.height === contentSize.height) {
    const chrome = svg(screenSize.width, screenSize.height, `<g fill="white" font-family="Arial, sans-serif"><text x="99" y="113" font-size="46" font-weight="600">9:41</text><rect x="918" y="100" width="9" height="13" rx="2"/><rect x="933" y="92" width="9" height="21" rx="2"/><rect x="948" y="84" width="9" height="29" rx="2"/><rect x="963" y="76" width="9" height="37" rx="2"/><path d="M994 88Q1016 68 1038 88M1001 96Q1016 82 1031 96M1009 104Q1016 97 1023 104" fill="none" stroke="white" stroke-width="6" stroke-linecap="round"/><circle cx="1016" cy="112" r="4"/><rect x="1055" y="79" width="65" height="33" rx="8" fill="none" stroke="white" stroke-width="3" opacity=".65"/><rect x="1061" y="85" width="53" height="21" rx="4"/><rect x="1125" y="89" width="5" height="13" rx="2" opacity=".65"/><rect x="387" y="2520" width="405" height="15" rx="7.5" opacity=".9"/></g>`)
    screenImage = await sharp({ create: { width: screenSize.width, height: screenSize.height, channels: 4, background: '#0a1110' } }).composite([
      { input: source, left: 0, top: contentSize.statusHeight },
      { input: chrome, left: 0, top: 0 }
    ]).png().toBuffer()
  }
  const mask = svg(screenSize.width, screenSize.height, `<rect width="${screenSize.width}" height="${screenSize.height}" rx="${screenSize.radius}" fill="white"/>`)
  const rounded = await sharp(screenImage).ensureAlpha().composite([{ input: mask, blend: 'dest-in' }]).png().toBuffer()
  await sharp({ create: { ...frameSize, channels: 4, background: '#00000000' } }).composite([
    { input: rounded, left: screenSize.left, top: screenSize.top },
    { input: frameFile, left: 0, top: 0 }
  ]).png().toFile(target)
  return target
}

/** 生成完整三机横排图，保留适度错位、柔影、简短标签和演示数据说明。 */
async function composeGroup(group: Group, phones: Map<string, string>): Promise<string> {
  const phoneHeight = 1240, phoneWidth = Math.round(frameSize.width / frameSize.height * phoneHeight)
  const positions = [{ left: 170, top: 320 }, { left: Math.round((canvas.width - phoneWidth) / 2), top: 270 }, { left: canvas.width - 170 - phoneWidth, top: 320 }]
  const shadows = positions.map(position => `<rect x="${position.left + 13}" y="${position.top + 27}" width="${phoneWidth - 22}" height="${phoneHeight - 10}" rx="${phoneWidth * .15}" fill="#132b24" opacity=".17" filter="url(#shadow)"/>`).join('')
  const labels = group.screens.map((screen, index) => `<text x="${positions[index].left + phoneWidth / 2}" y="${positions[index].top + phoneHeight + 75}" text-anchor="middle" font-size="34" font-weight="600" fill="#30483e">${xml(screen.label)}</text>`).join('')
  const decoration = svg(canvas.width, canvas.height, `<defs><filter id="shadow" x="-25%" y="-15%" width="150%" height="140%"><feGaussianBlur stdDeviation="22"/></filter></defs><g font-family="Hiragino Sans GB, Heiti SC, Arial, sans-serif"><rect width="2400" height="1800" fill="#f2f4f1"/><rect x="116" y="94" width="10" height="75" rx="5" fill="#22553f"/><text x="156" y="151" font-size="78" font-weight="700" fill="#173c2e">${xml(group.title)}</text><text x="156" y="216" font-size="32" fill="#617168">${xml(group.subtitle)}</text>${shadows}${labels}<text x="116" y="1741" font-size="27" fill="#617168">开机电竞 · 源码功能展示</text><text x="2284" y="1741" text-anchor="end" font-size="27" fill="#617168">演示数据 · 功能展示</text></g>`)
  const layers: OverlayOptions[] = [{ input: decoration, left: 0, top: 0 }]
  for (const [index, screen] of group.screens.entries()) {
    const phone = phones.get(screen.id)
    assert.ok(phone, '缺少已合成单机：' + screen.id)
    layers.push({ input: await sharp(phone).resize(phoneWidth, phoneHeight).png().toBuffer(), ...positions[index] })
  }
  const target = path.join(output, 'composites', group.id + '.png')
  await sharp({ create: { ...canvas, channels: 3, background: '#f2f4f1' } }).composite(layers).png().toFile(target)
  return target
}

/** 核验真实产物的尺寸，并对透明单机的四个角逐像素检查。 */
async function verifyImage(file: string, width: number, height: number, transparent = false): Promise<void> {
  const metadata = await sharp(file).metadata()
  assert.equal(metadata.width, width, file + ' 宽度不匹配')
  assert.equal(metadata.height, height, file + ' 高度不匹配')
  if (transparent) {
    assert.equal(metadata.hasAlpha, true, file + ' 必须保留透明背景')
    const { data, info } = await sharp(file).ensureAlpha().raw().toBuffer({ resolveWithObject: true })
    assert.equal(info.channels, 4)
    for (const [x, y] of [[0, 0], [width - 1, 0], [0, height - 1], [width - 1, height - 1]]) {
      assert.equal(data[(y * width + x) * 4 + 3], 0, file + ' 手机角外没有透明')
    }
  }
}

/** 输出低分辨率联系表，以便快速检查各组的整体布局与截图状态。 */
async function composeContactSheet(files: string[]): Promise<void> {
  const columns = Math.min(3, files.length), width = 600, height = 450
  const layers: OverlayOptions[] = []
  for (const [index, file] of files.entries()) {
    layers.push({ input: await sharp(file).resize(width, height).jpeg({ quality: 86 }).toBuffer(), left: index % columns * width, top: Math.floor(index / columns) * height })
  }
  const target = path.join(output, 'contact-sheet.jpg')
  await sharp({ create: { width: width * columns, height: height * Math.ceil(files.length / columns), channels: 3, background: '#f2f4f1' } }).composite(layers).jpeg({ quality: 88 }).toFile(target)
  await verifyImage(target, width * columns, height * Math.ceil(files.length / columns))
}

/** 按清单完成素材生成，核验文件总数，记录许可出处和输出规格。 */
async function run(): Promise<void> {
  const groups = readManifest()
  await prepareFrame()
  fs.mkdirSync(path.join(output, 'phones'), { recursive: true })
  fs.mkdirSync(path.join(output, 'composites'), { recursive: true })
  const phones = new Map<string, string>(), composites: string[] = []
  for (const group of groups) {
    for (const screen of group.screens) {
      if (phones.has(screen.id)) continue
      const file = await composePhone(screen)
      await verifyImage(file, frameSize.width, frameSize.height, true)
      phones.set(screen.id, file)
      console.log('单机图已生成：' + screen.id)
    }
    const file = await composeGroup(group, phones)
    await verifyImage(file, canvas.width, canvas.height)
    composites.push(file)
    console.log('三屏组合已生成：' + group.title)
  }
  assert.equal(fs.readdirSync(path.join(output, 'phones')).filter(file => file.endsWith('.png')).length, phones.size, '单机图数量与清单不一致，请检查残留文件')
  assert.equal(fs.readdirSync(path.join(output, 'composites')).filter(file => file.endsWith('.png')).length, groups.length, '组合图数量与清单不一致，请检查残留文件')
  await composeContactSheet(composites)
  fs.writeFileSync(path.join(output, 'composition-report.json'), JSON.stringify({ passed: true, groups: groups.length, phones: phones.size, screenshots: contentSize, screen: screenSize, phone: frameSize, composite: canvas, capture: 'H5真实页面截图加设备展示框；759视口上方补59、下方补34 CSS像素设备展示区，不拉伸原图；非真机截图。', frame: { source: frameUrl, license: licenseUrl, usage: '允许带页面截图的商业宣传成品；裸边框仅留本机忽略目录，不随源码或素材包分发。' }, checks: ['输入截图尺寸正确', '输出数量与清单一致', '单机和组合尺寸正确', '单机四角透明', '组合图统一标注演示数据'] }, null, 2) + '\n')
  console.log(`素材完成：${phones.size} 张透明单机，${groups.length} 张三屏组合及 1 张总览。目录：${output}`)
}

run().catch((error: unknown) => { console.error(error instanceof Error ? error.message : String(error)); process.exitCode = 1 })
