/** 文件作用：通过微信官方自动化 SDK 验收十三游戏、热门首排和独立文字封面，仅操作本地界面。
 * 执行：node scripts/mini-redesign-smoke.cjs；前提：本地接口 8090、微信自动化 9420 和已导入的最新小程序构建。
 * 可通过 WX_AUTOMATOR_ENDPOINT 覆盖自动化地址，不需要微信正式登录或支付凭据。
 */
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const automator = require('../frontend/node_modules/miniprogram-automator')
const dir = path.resolve(__dirname, '../output/redesign')
const report = { passed: false, time: new Date().toISOString(), steps: [], games: [], sorts: [], positions: [], exceptions: [] }
let mini
/** 等待用户界面出现指定结果，失败时报告步骤。 */
async function until(read, description) {
  const start = Date.now()
  while (Date.now() - start < 12000) {
    const result = await read()
    if (result) return result
    await new Promise(resolve => setTimeout(resolve, 120))
  }
  throw new Error('等待超时：' + description)
}
/** 使用编译后真实组件路径穿过 uni-app 的自定义组件边界。 */
async function component(scope, name) { return scope.$('components\\/' + name) }
/** 读取指定类型的所有真实组件实例。 */
async function components(scope, name) { return scope.$$('components\\/' + name) }
/** 等待路由完成切换后返回真实页面。 */
async function pageAt(route) {
  return until(async () => { const page = await mini.currentPage(); return page?.path === route ? page : null }, route)
}
/** 按指定作用域中的可见文案寻找目标按钮。 */
async function button(scope, text) {
  return until(async () => {
    for (const element of await scope.$$('button')) if ((await element.text()).trim() === text) return element
    return null
  }, '按钮 ' + text)
}
/** 读取数字化的屏幕矩形，避免尺寸字符串参与加法。 */
async function rectangle(element) {
  const [offset, size] = await Promise.all([element.offset(), element.size()])
  const rect = { left: Number(offset.left), top: Number(offset.top), width: Number(size.width), height: Number(size.height) }
  for (const value of Object.values(rect)) assert.ok(Number.isFinite(value), '元素矩形必须有效')
  assert.ok(rect.width > 0 && rect.height > 0, '元素应具有可见尺寸')
  return rect
}
/** 读取独立图片和文字图层，同时记录文字样式与实际矩形。 */
async function layers(page) {
  const preview = await page.$('.preview')
  const art = await component(preview, 'CoverArt')
  const image = await art.$('.cover-image')
  const labels = await art.$$('.cover-label')
  return {
    image: await image.attribute('src'),
    bounds: await rectangle(await art.$('.cover-art')),
    labels: await Promise.all(labels.map(async element => ({ text: await element.text(), style: await element.attribute('style'), rect: await rectangle(element) })))
  }
}
/** 验证每个图层均在封面内，并且所有文案矩形互不重叠。 */
function nonOverlapping(cover) {
  const tolerance = 0.6
  for (const label of cover.labels) {
    const rect = label.rect, bounds = cover.bounds
    assert.ok(rect.left >= bounds.left - tolerance && rect.top >= bounds.top - tolerance && rect.left + rect.width <= bounds.left + bounds.width + tolerance && rect.top + rect.height <= bounds.top + bounds.height + tolerance, '文字超出封面：' + label.text)
  }
  for (let index = 0; index < cover.labels.length; index++) {
    const first = cover.labels[index].rect
    for (const label of cover.labels.slice(index + 1)) {
      const second = label.rect
      assert.ok(first.left + first.width <= second.left + tolerance || second.left + second.width <= first.left + tolerance || first.top + first.height <= second.top + tolerance || second.top + second.height <= first.top + tolerance, '文字发生重叠')
    }
  }
}
/** 保留文案和完整样式，用于证明更换底图不会重绘或丢失文字配置。 */
function labelConfiguration(cover) { return cover.labels.map(label => ({ text: label.text, style: label.style })) }
/** 将页面滚动到目标按钮附近，以真实用户可见的方式完成操作。 */
async function reveal(element) {
  const offset = await element.offset()
  await mini.pageScrollTo(Math.max(0, Number(offset.top) - 160))
}
/** 写入已通过的检查点，便于区分业务问题和自动化定位问题。 */
function passed(description) { report.steps.push(description); console.log('PASS ' + description) }
/** 执行真实小程序操作，不修改组件状态，不调用业务方法。 */
async function run() {
  fs.mkdirSync(dir, { recursive: true })
  const bootstrap = await fetch('http://127.0.0.1:8090/api/bootstrap').then(response => response.json())
  assert.equal(bootstrap.data.dev_mode, true, '禁止对正式环境运行本地验收')
  assert.equal(bootstrap.data.games.length, 13)
  mini = await automator.connect({ wsEndpoint: process.env.WX_AUTOMATOR_ENDPOINT || 'ws://127.0.0.1:9420' })
  mini.on('exception', event => report.exceptions.push(String(event.message || event)))
  await mini.reLaunch('/pages/index/index')
  let page = await pageAt('pages/index/index')
  const picker = await until(() => component(page, 'GamePicker'), '游戏选择组件')
  await until(async () => (await picker.$$('.game-choice')).length === 13, '十三游戏入口')
  const scroll = await picker.$('.games-scroll')
  await scroll.scrollTo(0, 0)
  await (await picker.$('#game-delta')).tap()
  await mini.pageScrollTo(0)
  const screen = await page.size()
  const firstRow = []
  for (const id of ['honor', 'peace', 'delta', 'tft', 'valorant']) {
    const rect = await rectangle(await picker.$('#game-' + id))
    assert.ok(rect.left >= -0.5 && rect.left + rect.width <= Number(screen.width) + 0.5, '热门游戏入口未完整出现在首排：' + id)
    firstRow.push({ id, rect })
  }
  assert.ok(firstRow.every(item => Math.abs(item.rect.top - firstRow[0].rect.top) < 1), '热门游戏必须处于同一行')
  report.first_row = firstRow
  passed('默认三角洲下，王者、和平、三角洲、金铲铲、无畏五个入口完整显示')
  for (let index = 0; index < bootstrap.data.games.length; index++) {
    const game = bootstrap.data.games[index]
    await scroll.scrollTo(Math.max(0, index - 2) * 73, 0)
    await (await picker.$('#game-' + game.id)).tap()
    await until(async () => { const hero = await page.$('.hero-image'); return hero && (await hero.attribute('src')) === game.cover_url }, '主题 ' + game.id)
    const cards = await until(async () => {
      if (await page.$('.loading-line')) return null
      const items = await components(page, 'ServiceCard')
      if (!items.length) return null
      for (const card of items) {
        const art = await component(card, 'CoverArt')
        if (!art || !(await (await art.$('.cover-image')).attribute('src')).includes('/' + game.id + '-')) return null
      }
      return items
    }, '对应游戏服务卡片 ' + game.id)
    report.games.push({ id: game.id, cards: cards.length })
    passed('游戏切换与独立封面：' + game.name)
  }
  await scroll.scrollTo(0, 0)
  await (await picker.$('#game-delta')).tap()
  await until(async () => !(await page.$('.loading-line')), '恢复三角洲')
  await mini.pageScrollTo(0)
  await mini.screenshot({ path: path.join(dir, 'mini-home-redesign.png') })
  for (const [sort, label] of [['price', '价格从低到高'], ['completed', '成交优先'], ['recommended', '综合排序']]) {
    const expectedResponse = await fetch('http://127.0.0.1:8090/api/services?game_id=delta&sort=' + sort + '&limit=24&offset=0').then(response => response.json())
    const expected = expectedResponse.data.map(service => ({ title: service.title, price: service.price_cents, count: service.completed_count || 0 }))
    const control = await until(() => page.$('.sort-control'), '排序按钮')
    await reveal(control)
    await control.tap()
    const menu = await until(() => page.$('.sort-menu'), '排序选项')
    assert.equal((await menu.$$('button')).length, 3)
    if (sort === 'price') await mini.screenshot({ path: path.join(dir, 'mini-sort-menu.png') })
    await (await button(menu, label)).tap()
    await until(async () => (await control.text()).trim() === label && !(await page.$('.sort-menu')), '排序选择 ' + label)
    const actual = await until(async () => {
      if (await page.$('.loading-line')) return null
      const cards = await components(page, 'ServiceCard')
      const values = await Promise.all(cards.map(async card => ({
        title: await (await card.$('.service-title')).text(),
        price: Math.round(Number(await (await card.$('.amount')).text()) * 100),
        count: Number((await (await card.$('.count')).text()).replace(/[^0-9]/g, ''))
      })))
      return JSON.stringify(values) === JSON.stringify(expected) ? values : null
    }, '服务端排序与界面顺序一致 ' + label)
    if (sort === 'price') assert.ok(actual.every((service, index) => index === 0 || service.price >= actual[index - 1].price), '价格必须从低到高')
    if (sort === 'completed') assert.ok(actual.every((service, index) => index === 0 || service.count <= actual[index - 1].count), '成交必须从高到低')
    report.sorts.push({ sort, label, cards: actual })
    passed('排序界面与服务端顺序一致：' + label)
  }
  await mini.pageScrollTo(0)
  await (await button(picker, '全部 13 款')).tap()
  await (await until(() => picker.$('.catalog-search input'), '完整目录搜索')).input('蛋仔')
  await until(async () => (await picker.$$('.catalog-game')).length === 1, '目录搜索过滤')
  const result = await picker.$('.catalog-game')
  assert.ok((await result.text()).includes('蛋仔派对'))
  await mini.screenshot({ path: path.join(dir, 'mini-game-search.png') })
  await result.tap()
  await until(async () => (await (await page.$('.hero-image')).attribute('src')).includes('/eggy-'), '目录搜索选择生效')
  passed('完整十三游戏目录与蛋仔搜索选择')
  await mini.reLaunch('/pages/login/index')
  page = await pageAt('pages/login/index')
  const worker = await until(async () => {
    for (const element of await page.$$('button')) if ((await element.text()).includes('北辰')) return element
    return null
  }, '打手账号')
  await worker.tap()
  await pageAt('pages/me/index')
  await mini.navigateTo('/pages/publish/index?id=9d6fbce4bae12a9e94a3e3d1e1e9aa57')
  page = await pageAt('pages/publish/index')
  await until(async () => {
    const preview = await page.$('.preview')
    const art = preview && await component(preview, 'CoverArt')
    return art && (await art.$$('.cover-label')).length === 6
  }, '六文案回填')
  const before = await layers(page)
  nonOverlapping(before)
  assert.equal((await page.$$('canvas')).length, 0)
  const preview = await page.$('.preview'), art = await component(preview, 'CoverArt')
  assert.equal((await art.$$('canvas')).length, 0)
  await (await button(page, '换一张底图')).tap()
  await until(async () => (await layers(page)).image !== before.image, '随机更换底图')
  const after = await layers(page)
  assert.deepEqual(labelConfiguration(after), labelConfiguration(before), '换图后文字及颜色、背景、位置、字号应完全保留')
  nonOverlapping(after)
  report.independent_layers = 6
  report.no_canvas = true
  report.random_preserved_labels = true
  report.cover_before = before
  report.cover_after = after
  passed('六个独立文字图层、换底图保留完整配置、无 Canvas、文字均不重叠且在封面内')
  const positionButton = await page.$('.position-picker')
  for (const position of ['左上', '上方', '右上', '左侧', '居中', '右侧', '左下', '下方', '右下']) {
    await reveal(positionButton)
    await positionButton.tap()
    const grid = await until(() => page.$('.position-grid'), '九宫格')
    assert.equal((await grid.$$('button')).length, 9)
    await (await button(grid, position)).tap()
    await until(async () => (await positionButton.text()).includes(position), '位置更新 ' + position)
    const current = await layers(page)
    assert.deepEqual(current.labels.map(label => label.text), before.labels.map(label => label.text))
    nonOverlapping(current)
    report.positions.push(position)
  }
  report.position_picker_passed = true
  passed('九宫格全部九种位置可选，六条文字在每种位置均无重叠且不越界')
  await mini.pageScrollTo(0)
  await mini.screenshot({ path: path.join(dir, 'mini-overlay-editor.png') })
  await reveal(positionButton)
  await positionButton.tap()
  await mini.screenshot({ path: path.join(dir, 'mini-position-grid.png') })
  await positionButton.tap()
  assert.deepEqual(report.exceptions, [], '小程序运行异常')
  await mini.reLaunch('/pages/index/index')
  page = await pageAt('pages/index/index')
  const homePicker = await until(() => component(page, 'GamePicker'), '返回首页')
  await (await homePicker.$('.games-scroll')).scrollTo(0, 0)
  await (await homePicker.$('#game-delta')).tap()
  await mini.pageScrollTo(0)
  report.passed = true
}
run().catch(async error => {
  report.error = error.stack
  console.error(error.stack)
  process.exitCode = 1
  if (mini) {
    try { report.failed_page = (await mini.currentPage()).path; await mini.screenshot({ path: path.join(dir, 'mini-redesign-failure.png') }) } catch (captureError) { report.capture_error = captureError.message }
  }
}).finally(async () => {
  fs.mkdirSync(dir, { recursive: true })
  fs.writeFileSync(path.join(dir, 'mini-redesign-smoke.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(JSON.stringify({ passed: report.passed, games: report.games.length, positions: report.positions, steps: report.steps, exceptions: report.exceptions }, null, 2))
  if (mini) await mini.disconnect()
})
