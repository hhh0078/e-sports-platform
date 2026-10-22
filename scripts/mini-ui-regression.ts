/** 文件作用：用微信官方自动化 SDK 验证常驻主导航、二级返回、访客空态与四类身份页面，保存真实模拟器截图。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { CustomElement, InputElement, SliderElement, ScrollViewElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { User, Service, Club } from '../frontend/src/types/models'

const projectRoot=process.cwd(), apiBase=process.env.UI_TEST_API||'http://127.0.0.1:18091/api'
const miniProject=path.resolve(process.env.UI_TEST_PROJECT||path.join(projectRoot,'frontend/dist/build/mp-weixin'))
const outputDir=path.resolve(process.env.UI_TEST_OUTPUT || path.join(projectRoot,'output/frontend-standards/mini-ui'))
const require=createRequire(path.join(projectRoot,'frontend/package.json'))
const automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
const checks: string[]=[],exceptions:string[]=[]
let mini: MiniProgram | undefined
let viewport = { width: 0, height: 0, statusBarHeight: 0, capsuleBottom: 0 }
/** 官方自动化evaluate在微信逻辑层读取同步尺寸，避免SDK旧systemInfo调用产生弃用告警。 */
declare const wx: {getWindowInfo():{windowWidth:number;windowHeight:number;statusBarHeight:number};getMenuButtonBoundingClientRect():{bottom:number};navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void}

/** 限时等待真实页面或可见元素，失败时指出具体界面步骤。 */
async function until<T>(read:()=>Promise<T|null|false|undefined>,description:string,timeout=12000):Promise<T> {
  const start=Date.now()
  while(Date.now()-start<timeout){const value=await read();if(value)return value;await new Promise<void>(resolve=>setTimeout(resolve,150))}
  throw new Error('等待超时：'+description)
}

/** 取得已连接的微信实例，避免可选引用在异步回调中扩散。 */
function instance(): MiniProgram {assert.ok(mini,'尚未连接微信自动化');return mini}

/** 读取本地测试 API，凭证和会话内容不进入报告。 */
async function getData<T>(route:string):Promise<T> {
  const response=await fetch(apiBase+route),body:unknown=await response.json()
  assert.ok(response.ok,route+'请求失败：'+response.status)
  assert.ok(typeof body==='object'&&body!==null&&'data' in body,route+'响应格式不正确')
  return body.data as T
}

/** 兼容 uni-app 自定义组件路径与注册标签，从指定范围寻找实际元素。 */
async function child(scope:Page|Element,selectors:string[]):Promise<Element|null> {
  for(const selector of selectors){const element=await scope.$(selector);if(element)return element}
  return null
}

/** 当前路由必须匹配预期，不以等待时长代替页面断言。 */
async function pageAt(route:string):Promise<Page> {return until(async()=>{const page=await instance().currentPage();return page?.path===route?page:null},route)}

/** 官方SDK在部分权限页等待navigateTo回调会超时；直接发起同一原生API，再按真实路由和元素断言。 */
async function openRoute(route:string):Promise<void> {await instance().evaluate((url:string)=>{wx.navigateTo({url});return true},route);await pageAt(route.split('?')[0].replace(/^\//,''))}

/** 重启路由前先结束自动化RPC，避免逻辑层重启使SDK原回调悬挂；后续仍验证真实页面。 */
async function restartRoute(route:string):Promise<void> {const previous=await instance().currentPage();await instance().evaluate((url:string)=>{setTimeout(()=>wx.reLaunch({url}),30);return true},route);await until(async()=>{const page=await instance().currentPage();return page?.path===route.split('?')[0].replace(/^\//,'')&&page!==previous?page:null},'新路由 '+route.split('?')[0])}

/** 查找当前页面用户真正看得到的按钮，保留原生点击流程。 */
async function button(text:string):Promise<Element> {
  return until(async()=>{const page=await instance().currentPage();if(!page)return null;const elements:Element[]=await page.$$('button');for(const element of elements)if((await element.text()).replace(/[\uE000-\uF8FF]/g,'').trim()===text)return element;return null},'按钮 '+text)
}

/** 验证底栏只出现在主页面，并取得其中的四个入口。 */
async function bottomNavigation(page:Page):Promise<{bar:Element;items:Element[]}> {
  const bar=await child(page,['bottom-nav','components\\/BottomNav'])
  assert.ok(bar,'主页面缺少底栏组件')
  const items:Element[]=await bar.$$('.nav-item')
  assert.equal(items.length,4,'底栏必须有四项导航')
  const bars:Element[]=await page.$$('bottom-nav')
  if(bars.length)assert.equal(bars.length,1,'主页面底栏重复挂载')
  return{bar,items}
}

/** 保存真实模拟器首屏截图，不创建或修改设计图。 */
async function screenshot(name:string):Promise<void> {
  await new Promise<void>(resolve=>setTimeout(resolve,450))
  try { await instance().screenshot({path:path.join(outputDir,name+'.png')}) } catch (error) { throw new Error('截图失败：'+name, {cause:error}) }
}

/** 点击实际顶部返回按钮，并验证页面回到保留的主实例。 */
async function returnToMain(page:Page,mainPage:Page,expectedPath='pages/main/index'):Promise<void> {
  assert.equal(await child(page,['bottom-nav','components\\/BottomNav']),null,'二级页面不应有主底栏')
  const shell=await child(page,['app-shell','components\\/AppShell'])
  assert.ok(shell,'二级页面缺少统一 AppShell')
  const floating=await child(shell,['floating-back','components\\/FloatingBack'])
  if(floating){
    const back=await floating.$('.floating-back');assert.ok(back,'展示页缺少圆形返回按钮');await back.tap()
    assert.strictEqual(await pageAt(expectedPath),mainPage,'展示页返回时主页面被重建');return
  }
  const header=await child(shell,['page-header','components\\/PageHeader'])
  assert.ok(header,'二级页面缺少统一顶部导航')
  const navbar=await child(header,['uni-nav-bar','node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-nav-bar\\/uni-nav-bar'])
  assert.ok(navbar,'统一顶部导航未使用官方 uni-nav-bar')
  const back=await child(navbar,['.uni-navbar__header-btns-left','.uni-navbar__header-btns'])
  assert.ok(back,'二级页面缺少左上返回点击区')
  await back.tap()
  const current=await pageAt(expectedPath)
  assert.strictEqual(current,mainPage,'二级页面返回时主页面被重建')
}

/** 通过显式同意协议和本地体验身份选择器登录，不直接注入会话。 */
async function login(name:string):Promise<Page> {
  console.log('CHECK 登录 '+name)
  await restartRoute('/pages/account/login')
  console.log('CHECK 登录页已打开 '+name)
  const page=await pageAt('pages/account/login')
  const consent=await until(()=>page.$('.consent-line label'),'主动勾选协议')
  await consent.tap()
  await until(async()=>{const checkbox=await page.$('checkbox');return checkbox&&String(await checkbox.property('checked'))==='true'},'协议已勾选')
  await(await button('开发体验')).tap()
  console.log('CHECK 体验身份列表 '+name)
  const account=await until(async()=>{const elements:Element[]=await page.$$('button');for(const element of elements)if((await element.text()).includes(name))return element;return null},'体验身份 '+name)
  await account.tap()
  console.log('CHECK 完善资料弹层 '+name)
  await(await button('先这样，稍后完善')).tap()
  const main=await pageAt('pages/main/index')
  checks.push('登录：'+name)
  return main
}

/** 访问二级路由、截图并点击统一返回，不改动其业务资料。 */
async function visit(route:string,name:string,main:Page):Promise<void> {
  console.log('CHECK 页面 '+route)
  await openRoute(route)
  const page=await pageAt(route.split('?')[0].replace(/^\//,''))
  await screenshot(name)
  if(name==='manage-settings-admin'){
    await(await button('微信支付')).tap()
    await until(()=>page.$('.payment-status'),'管理员支付配置状态')
    const secrets:Element[]=await page.$$('.key-content')
    assert.equal(secrets.length,2,'支付公私钥字段缺失')
    for(const secret of secrets)assert.equal(await secret.value(),'','已保存的支付秘密不能回显')
    await screenshot('manage-payment-admin');checks.push('管理员支付配置可打开，已保存秘密不回显，未写入或提交支付设置')
  }
  await returnToMain(page,main)
  checks.push('页面与返回：'+route.split('?')[0]); console.log('PASS 页面与返回 '+route.split('?')[0])
}

/** 检查控制台十二个独立入口与下级游戏编辑器，验证层级返回且不保存配置。 */
async function adminControls(main:Page):Promise<void> {
  await openRoute('/pages/manage/index')
  const consolePage=await pageAt('pages/manage/index')
  const cards:Element[]=await until(async()=>{const result=await consolePage.$$('.module-card');return result.length===12?result:null},'控制台十二个入口')
  await until(async()=>!(await consolePage.$('.chart-loading')),'真实交易曲线读取')
  assert.equal(await consolePage.$('.admin-tabs'),null,'控制台仍残留旧大全页签')
  for(const card of cards){const box=await rectangle(card);assert.ok(box.left>=15&&box.left+box.width<=viewport.width-15,'控制台入口卡片横向溢出')}
  await screenshot('manage-console')
  const platform=await button('平台设置');await instance().pageScrollTo(Math.max(0,(await rectangle(platform)).top-120));await platform.tap()
  const platformPage=await pageAt('pages/manage/platform');await until(()=>platformPage.$('input[maxlength="24"]'),'平台设置字段')
  await returnToMain(platformPage,consolePage,'pages/manage/index')
  await until(async()=>!(await consolePage.$('.chart-loading')),'控制台返回数据读取')
  const games=await button('游戏配置');await instance().pageScrollTo(Math.max(0,(await rectangle(games)).top-120));await games.tap()
  const page=await pageAt('pages/manage/games');await until(()=>page.$('.game-summary'),'独立游戏管理列表')
  await(await button('编辑')).tap()
  const modal=await until(async()=>{const direct=await page.$('.game-sheet');if(direct)return direct;const popup=await child(page,['uni-popup','node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']);return popup?popupNode(popup,'.game-sheet'):null},'游戏配置弹层') as ScrollViewElement
  const area=await rectangle(modal);assert.ok(Number(await modal.scrollWidth())<=area.width+1,'游戏配置弹层横向溢出')
  const tools=await modal.$('.upload-actions');assert.ok(tools);const toolsBox=await rectangle(tools)
  assert.ok(toolsBox.left+toolsBox.width<=viewport.width+1,'游戏图标上传工具超出窄屏')
  await screenshot('manage-game-modal');await(await button('关闭')).tap();await until(async()=>!(await page.$('.game-sheet')),'关闭游戏编辑器')
  await returnToMain(page,consolePage,'pages/manage/index');await returnToMain(consolePage,main)
  checks.push('控制台十二个卡片与真实曲线可见，独立平台/游戏页及两级返回可用，编辑器窄屏无溢出')
}

/** 在专用本地账号暂存长昵称/简介，核验真实主页后立即恢复原资料。 */
async function longProfile(main:Page):Promise<void> {
  const name='窄屏验收玩家昵称甲乙丙丁戊己庚辛壬癸',bio='认真游戏，耐心沟通。'.repeat(25)
  await openRoute('/pages/account/profile');let page=await pageAt('pages/account/profile')
  let nickname=await until(()=>page.$('input[maxlength="24"]'),'昵称输入') as InputElement
  let intro=await page.$('.bio-input') as InputElement|null;assert.ok(intro)
  const original=await until(async()=>{const value=String(await nickname.value());return value||null},'原始资料已载入'),originalBio=String(await intro.value())
  try{
    await nickname.input(name);await intro.input(bio);await(await button('保存资料')).tap()
    await until(async()=>String(await(await button('保存资料')).property('disabled'))==='false','长资料保存结束')
    await returnToMain(page,main);const {items}=await bottomNavigation(main);await items[3].tap()
    const me=await child(main,['me-view','pages\\/main\\/me\\/me']);assert.ok(me)
    const title=await until(async()=>{const value=await me.$('.profile-name');return value&&(await value.text())===name?value:null},'主页长昵称已保存')
    const biography=await me.$('.profile-bio'),identity=await me.$('.identity-row'),stats=await me.$('.stats-row');assert.ok(biography&&identity&&stats)
    const titleBox=await rectangle(title),bioBox=await rectangle(biography),identityBox=await rectangle(identity),statsBox=await rectangle(stats)
    assert.ok(titleBox.left+titleBox.width<=viewport.width-16+1,'长昵称越出屏幕')
    assert.ok(bioBox.height<=37&&statsBox.top>=identityBox.top+identityBox.height+12,'长简介挤压统计或超过两行')
    await screenshot('me-long-content');checks.push('真实保存长昵称/长简介：昵称省略、简介最多两行、统计不重叠；随后恢复原资料')
  }finally{
    if((await instance().currentPage())?.path!=='pages/account/profile')await openRoute('/pages/account/profile')
    page=await pageAt('pages/account/profile');nickname=await until(()=>page.$('input[maxlength="24"]'),'恢复昵称字段') as InputElement;intro=await page.$('.bio-input') as InputElement|null;assert.ok(intro)
    await nickname.input(original);await intro.input(originalBio);await(await button('保存资料')).tap()
    await until(async()=>String(await(await button('保存资料')).property('disabled'))==='false','原资料恢复完成')
    await returnToMain(page,main)
    const me=await child(main,['me-view','pages\\/main\\/me\\/me']);assert.ok(me)
    await until(async()=>{const value=await me.$('.profile-name');return value&&(await value.text())===original},'原昵称恢复到主页')
    await screenshot('me-authenticated')
  }
}

/** 记录已登录接单者的真实服务列表与新版插画工作台。 */
async function workerLayout(main:Page):Promise<void> {
  const {items}=await bottomNavigation(main);await items[1].tap()
  const worker=await child(main,['worker-view','pages\\/main\\/worker\\/worker']);assert.ok(worker)
  await until(()=>child(worker,['service-card','components\\/ServiceCard']),'接单者真实服务列表')
  const controls:Element[]=await worker.$$('.service-controls')
  assert.ok(controls.length,'服务管理缺少统一操作行')
  for(const row of controls.slice(0,4)){const buttons:Element[]=await row.$$('.service-control');const status=await row.$('.service-status');if(buttons.length===2){const a=await rectangle(buttons[0]),b=await rectangle(buttons[1]);assert.ok(Math.abs(a.top-b.top)<=1,'卡片两个操作被插槽包装挤成两行')}else if(buttons.length&&status){const a=await rectangle(buttons[0]),b=await rectangle(status);assert.ok(Math.abs((a.top+a.height/2)-(b.top+b.height/2))<=2,'卡片编辑与状态未在同一行居中')}}
  await screenshot('worker-authenticated');checks.push('已登录打手工作台展示真实服务、收入统计及原创插画')
  await maximumPriceLayout(worker)
}

/** 临时注入合法上限的原生渲染夹具，只检查真实卡片排版，结束恢复且不修改服务或交易数据。 */
async function maximumPriceLayout(worker:Element):Promise<void> {
  const card=await child(worker,['service-card','components\\/ServiceCard']) as CustomElement|null;assert.ok(card)
  const template=fs.readFileSync(path.join(miniProject,'components/ServiceCard.wxml'),'utf8')
  const amountKey=template.match(/class="amount[^\"]*"[^>]*>\{\{(\w+)\}\}/)?.[1],unitKey=template.match(/class="unit[^\"]*"[^>]*>\/\{\{(\w+)\}\}/)?.[1],countKey=template.match(/class="count[^\"]*"[^>]*>成交 \{\{(\w+)\}\}/)?.[1]
  assert.ok(amountKey&&unitKey&&countKey,'未找到当前微信编译卡片的金额渲染字段')
  const previous:Record<string,unknown>=await card.data(),restore={[amountKey]:previous[amountKey],[unitKey]:previous[unitKey],[countKey]:previous[countKey]}
  try{
    await card.setData({[amountKey]:'10000',[unitKey]:'小时',[countKey]:'99999'})
    const amount=await until(async()=>{const value=await card.$('.amount');return value&&(await value.text())==='10000'?value:null},'高金额原生卡片夹具')
    const body=await card.$('.service-body'),price=await card.$('.price-value'),count=await card.$('.count'),unit=await card.$('.unit');assert.ok(body&&price&&count&&unit)
    const area=await rectangle(body)
    for(const item of [amount,price,count,unit]){const box=await rectangle(item);assert.ok(box.left>=area.left-1&&box.left+box.width<=area.left+area.width+1,'上限金额或成交数超出卡片正文')}
    const left=await rectangle(price),right=await rectangle(count)
    const overlapX=Math.min(left.left+left.width,right.left+right.width)-Math.max(left.left,right.left),overlapY=Math.min(left.top+left.height,right.top+right.height)-Math.max(left.top,right.top)
    assert.ok(overlapX<=1||overlapY<=1,'金额与成交数相互重叠')
    await screenshot('service-max-price')
    checks.push('原生卡片夹具：合法上限10000元/小时与长成交数不裁切、不重叠；结束恢复，不写业务数据')
  }finally{await card.setData(restore)}
}

/** 检查四主导航循环切换的页面实例和每个入口，截图访客空态。 */
async function mainTabs(main:Page):Promise<void> {
  assert.equal(await child(main,['page-header','components\\/PageHeader']),null,'主导航页面不应有应用顶部导航')
  const{items}=await bottomNavigation(main),names=['player','worker','orders','me']
  for(let round=0;round<8;round++)for(let index=0;index<items.length;index++){
    await items[index].tap()
    assert.strictEqual(await instance().currentPage(),main,'主导航切换重建了页面实例')
    await until(async()=>String(await items[index].attribute('class')).includes('selected'),'主导航选中 '+names[index])
    if(round===0){
      const scrolls:ScrollViewElement[]=await main.$$('.main-scroll');assert.ok(Number(await scrolls[index].scrollWidth())<=viewport.width+1,'主视图横向溢出：'+names[index])
      if(index===0){const player=await child(main,['player-view','pages\\/main\\/player\\/player']);assert.ok(player);const card=await until(()=>child(player,['service-card','components\\/ServiceCard']),'真实服务卡片');const art=await child(card,['cover-art','components\\/CoverArt']),body=await card.$('.service-body');assert.ok(art&&body);const cover=await rectangle(art),info=await rectangle(body);assert.ok(info.top>=cover.top+cover.height-1,'卡片价格侵入图片');const tags=await card.$('.service-tags'),title=await card.$('.service-title');assert.ok(tags&&title);const tagBox=await rectangle(tags),titleBox=await rectangle(title);assert.ok(tagBox.top>=cover.top&&titleBox.top>=tagBox.top+tagBox.height-1&&titleBox.top+titleBox.height<=cover.top+cover.height+1,'标签与标题未依次悬浮在图片底部');checks.push('服务卡片标签上标题下悬浮图内，价格在图下正常排列')}
      await screenshot('guest-'+names[index])
    }
  }
  checks.push('四个主导航连续切换 8 轮，页面实例保持不变')
  await items[3].tap()
  const me=await child(main,['me-view','pages\\/main\\/me\\/me'])
  assert.ok(me,'个人中心内容视图不存在')
  assert.ok((await me.text()).includes('登录后查看'),'访客个人中心未展示登录提示')
  const hero=await me.$('.profile-hero'),identity=await me.$('.identity-row'),shade=await me.$('.profile-hero-shade');assert.ok(hero&&identity&&shade)
  const heroBox=await rectangle(hero),identityBox=await rectangle(identity)
  assert.ok(heroBox.left>=-1&&heroBox.width<=viewport.width+1,'个人中心游戏背景横向越界')
  assert.ok(identityBox.top>=viewport.capsuleBottom-1,'个人中心身份内容侵入原生状态栏或胶囊')
  await items[2].tap()
  const orders=await child(main,['orders-view','pages\\/main\\/orders\\/orders'])
  assert.ok(orders,'订单内容视图不存在')
  assert.ok((await orders.text()).includes('请登录后查看'),'访客订单缺少小字登录提示')
  checks.push('访客个人中心和订单正常展示登录空态')
}

/** 等待官方游戏弹层完成动画、搜索结果和缓存更新，所有选择均通过可见控件。 */
async function chooseGameViaCatalog(picker:Element,name:string,id:string):Promise<void> {
  const trigger=await picker.$('.catalog-trigger');assert.ok(trigger);await trigger.tap()
  await until(()=>popupNode(picker,'.catalog-game'),'游戏选择弹框')
  await new Promise<void>(resolve=>setTimeout(resolve,400))
  await until(()=>visibleGameSheet(picker),'首页游戏弹框真实可见')
  const search=await popupNode(picker,'.catalog-input') as InputElement|null;assert.ok(search);await search.input(name)
  const option=await until(async()=>{const value=await popupNode(picker,'.catalog-game');return value&&(await value.text()).includes(name)?value:null},'游戏选项 '+name)
  await option.tap()
  await until(async()=>String(await instance().callWxMethod('getStorageSync','jijie_game'))===id,'游戏选择写入本地缓存')
  await new Promise<void>(resolve=>setTimeout(resolve,450))
}

/** 节点存在不等于可见：游戏弹框必须具有真实尺寸且完整位于当前手机窗口内。 */
async function visibleGameSheet(scope:Element):Promise<Element|null>{
  const sheet=await popupNode(scope,'.catalog-sheet');if(!sheet)return null
  const box=await rectangle(sheet)
  return box.width>200&&box.height>100&&box.left>=-1&&box.left+box.width<=viewport.width+1&&box.top>=-1&&box.top+box.height<=viewport.height+1?sheet:null
}

/** 读取官方弹层槽位中的真实节点，兼容微信自定义组件边界。 */
async function popupNode(scope:Element,selector:string):Promise<Element|null> {
  const direct=await scope.$(selector);if(direct)return direct
  const gameSelector=await child(scope,['game-selector','components\\/GameSelector'])
  if(gameSelector){const value=await popupNode(gameSelector,selector);if(value)return value}
  const popup=await child(scope,['uni-popup','node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup'])
  if(!popup)return null
  const content=await popup.$(selector);if(content)return content
  const transitions:Element[]=await popup.$$('node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-transition\\/uni-transition')
  for(const transition of transitions){const value=await transition.$(selector);if(value)return value}
  return null
}

/** 通过实际游戏目录选择并缓存，检查单行布局及跨游戏详情不会改写偏好。 */
async function gameCatalog(main:Page):Promise<void> {
  const player=await child(main,['player-view','pages\\/main\\/player\\/player']);assert.ok(player)
  const picker=await child(player,['game-picker','components\\/GamePicker']);assert.ok(picker)
  const trigger=await picker.$('.catalog-trigger');assert.ok(trigger)
  await chooseGameViaCatalog(picker,'三角洲','delta')
  const selected='delta'
  const choices:Element[]=await picker.$$('.game-choice'),rows:Awaited<ReturnType<typeof rectangle>>[]=[]
  for(const choice of choices)rows.push(await rectangle(choice))
  assert.ok(rows.length>5,'游戏目录数量不足')
  assert.ok(rows.every(row=>Math.abs(row.top-rows[0].top)<=1),'首页游戏目录没有保持单行')
  const width=await rectangle(picker),control=await rectangle(trigger)
  assert.ok(control.left+control.width<=width.left+width.width+1,'全部游戏入口超出容器')
  const services=await getData<Service[]>('/services?limit=100'),other=services.find(item=>item.game_id!==selected)
  assert.ok(other,'测试需要另一游戏的服务')
  await visit('/pages/services/detail?id='+other.id,'services-other-game',main)
  assert.equal(String(await instance().callWxMethod('getStorageSync','jijie_game')),selected,'查看另一游戏详情改写了主游戏偏好')
  const {items}=await bottomNavigation(main)
  await items[3].tap();const me=await child(main,['me-view','pages\\/main\\/me\\/me']);assert.ok(me)
  const image=await until(()=>me.$('.profile-hero-image'),'当前游戏个人中心背景'),deltaSource=await image.property('src')
  await items[0].tap();await chooseGameViaCatalog(picker,'王者荣耀','honor');await items[3].tap()
  await until(async()=>(await image.property('src'))!==deltaSource,'个人中心同步切换游戏背景')
  await items[0].tap();await chooseGameViaCatalog(picker,'三角洲','delta')
  checks.push('游戏目录单行受控横滑、全部弹框可选并缓存、浏览其他游戏不改偏好；个人中心背景随游戏同步更换')
}

/** 测量真实节点矩形，所有数值必须来自微信模拟器。 */
async function rectangle(element: Element): Promise<{left:number;top:number;width:number;height:number}> {
  const size=await element.size(),offset:{left:number|string;top:number|string}=await element.offset()
  const result={left:Number(offset.left),top:Number(offset.top),width:parseFloat(String(size.width)),height:parseFloat(String(size.height))}
  assert.ok(Object.values(result).every(Number.isFinite),'节点矩形不是有效尺寸')
  return result
}

/** 从实际共享网格收集选中游戏ID，避免用组件内部数据替代用户可见状态。 */
async function selectedCatalogGames(selector:Element):Promise<string[]>{
  const grid=await popupNode(selector,'.catalog-grid');assert.ok(grid);const result:string[]=[]
  for(const option of await grid.$$('.catalog-game') as Element[])if(String(await option.attribute('class')).split(/\s+/).includes('selected'))result.push(String(await option.attribute('data-id')))
  return result.sort()
}

/** 表单复用首页弹框，单选不改主偏好，多选取消不回填、确认才修改草稿，不提交业务数据。 */
async function formGamePicker(main:Page,route:string,kind:'service'|'request'|'club-create'|'club-edit'):Promise<void>{
  await openRoute(route);const page=await pageAt(route.split('?')[0].slice(1)),multiple=kind.startsWith('club'),cache=String(await instance().callWxMethod('getStorageSync','jijie_game'))
  let clubName:InputElement|null=null
  if(kind==='club-create'){await(await button('创建')).tap();clubName=await until(()=>page.$('.create-form input'),'俱乐部创建草稿') as InputElement;await clubName.input('本地未提交的俱乐部草稿')}
  const selector=await until(()=>child(page,['game-selector','components\\/GameSelector']),'共享游戏选择器 '+kind)
  const trigger=kind==='club-create'?await page.$('.club-games-trigger'):await selector.$('.selector-trigger');assert.ok(trigger)
  const original=await trigger.text();await trigger.tap();await until(()=>visibleGameSheet(selector),'表单游戏网格真实可见 '+kind)
  if(kind==='club-create'){const overlay=await selector.$('.selector-overlay'),mask=await page.$('.modal-mask');assert.ok(overlay&&mask);assert.ok(Number(await overlay.style('z-index'))>Number(await mask.style('z-index')),'游戏选择被创建俱乐部遮罩覆盖')}
  if(multiple){
    const before=await selectedCatalogGames(selector),grid=await popupNode(selector,'.catalog-grid');assert.ok(grid);let target:Element|undefined,targetID=''
    for(const option of await grid.$$('.catalog-game') as Element[])if(!String(await option.attribute('class')).split(/\s+/).includes('selected')){target=option;targetID=String(await option.attribute('data-id'));break}
    assert.ok(target&&targetID);await target.tap();assert.equal(await trigger.text(),original,'多选未确认就回填表单');await screenshot('game-picker-'+kind+'-popup')
    const cancel=await popupNode(selector,'.selector-cancel');assert.ok(cancel);await cancel.tap();await until(async()=>!await visibleGameSheet(selector),'取消共享游戏选择')
    assert.equal(await trigger.text(),original,'取消游戏选择改变了草稿');await trigger.tap();await until(()=>visibleGameSheet(selector),'重新打开多选真实可见');assert.deepEqual(await selectedCatalogGames(selector),before,'取消后仍残留临时游戏')
    const reopened=await popupNode(selector,'.catalog-grid');assert.ok(reopened);let next:Element|undefined
    for(const option of await reopened.$$('.catalog-game') as Element[])if(String(await option.attribute('data-id'))===targetID)next=option
    assert.ok(next);await next.tap();const confirm=await popupNode(selector,'.selector-confirm');assert.ok(confirm);await confirm.tap();await until(async()=>!await visibleGameSheet(selector),'确认共享游戏选择');assert.notEqual(await trigger.text(),original,'确认多选未回填草稿')
  }else{
    const name=original.includes('王者')?'三角洲行动':'王者荣耀',search=await popupNode(selector,'.catalog-input') as InputElement|null;assert.ok(search);await search.input(name)
    const option=await until(async()=>{const value=await popupNode(selector,'.catalog-game');return value&&(await value.text()).includes(name)?value:null},'表单搜索游戏 '+name);await until(()=>visibleGameSheet(selector),'搜索后弹框保持可见');await screenshot('game-picker-'+kind+'-popup');await option.tap();await until(async()=>(await trigger.text()).includes(name),'单选回填游戏名称')
  }
  assert.equal(String(await instance().callWxMethod('getStorageSync','jijie_game')),cache,'表单游戏选择改写了首页偏好')
  if(clubName){assert.equal(await clubName.value(),'本地未提交的俱乐部草稿','游戏弹层丢失俱乐部名称草稿');await(await button('取消')).tap();await until(async()=>!await page.$('.modal-mask'),'取消创建俱乐部草稿')}
  await instance().pageScrollTo(0);await returnToMain(page,main);checks.push('共享游戏选择：'+kind+' '+(multiple?'取消保留、确认回填且嵌套层级正确':'搜索单选正确')+'，主游戏偏好未改变')
}

/** 使用实际编辑控件添加六条最大字号文字，核对独立图层及卡片下方信息。 */
async function coverDraft(main: Page): Promise<void> {
  await openRoute('/pages/services/publish')
  const page=await pageAt('pages/services/publish')
  await until(()=>page.$('.preview'),'服务草稿预览')
  for(let index=0;index<6;index++){await(await button('添加文字')).tap();await until(async()=>(await page.$$('.label-editor .input')).length===index+1,'新增独立文字 '+(index+1))}
  const inputs:InputElement[]=await page.$$('.label-editor .input')
  assert.equal(inputs.length,6)
  for(const input of inputs)await input.input('超长文案测试甲乙丙丁戊己')
  const sliders:SliderElement[]=await page.$$('slider')
  for(const slider of sliders)await slider.slideTo(24)
  const title=await page.$('input[maxlength="40"]') as InputElement|null
  assert.ok(title);await title.input('三角洲超长服务标题测试甲乙丙丁戊己庚辛壬癸')
  await instance().pageScrollTo(0)
  await new Promise<void>(resolve=>setTimeout(resolve,350))
  const card=await child(page,['service-card','components\\/ServiceCard'])
  assert.ok(card,'发布预览必须使用正式服务卡片')
  const art=await child(card,['cover-art','components\\/CoverArt']);assert.ok(art)
  const labels:Element[]=await art.$$('.cover-label');assert.equal(labels.length,6)
  const cover=await rectangle(art),boxes=[]
  for(const label of labels){const box=await rectangle(label);assert.ok(box.width>0&&box.height>0);assert.ok(box.left>=cover.left-1&&box.top>=cover.top-1&&box.left+box.width<=cover.left+cover.width+1&&box.top+box.height<=cover.top+cover.height+1,'封面文字越界');boxes.push(box)}
  for(let left=0;left<boxes.length;left++)for(let right=left+1;right<boxes.length;right++){const a=boxes[left],b=boxes[right];assert.ok(a.left+a.width<=b.left+1||b.left+b.width<=a.left+1||a.top+a.height<=b.top+1||b.top+b.height<=a.top+1,'独立文案互相重叠')}
  const system=await card.$('.service-overlay');assert.ok(system);const reserved=await rectangle(system)
  assert.ok(boxes.every(box=>box.top+box.height<=reserved.top+1),'用户文案侵入系统标签标题预留区')
  assert.equal(await card.$('.service-body'),null,'编辑器应聚焦封面，避免价格占位挤出文案控件')
  const source=await art.$('.cover-image');assert.ok(source)
  const before=await source.property('src')
  await(await button('换一张底图')).tap()
  await until(async()=>(await source.property('src'))!==before,'换图更新真实图片')
  assert.equal((await art.$$('.cover-label')).length,6,'换底图丢失独立文字')
  await screenshot('services-six-labels')
  await returnToMain(page,main)
  checks.push('六条最大字号文案真实布局不越界、不重叠；编辑器聚焦封面，换图保留独立图层')
}

/** 俱乐部申请中的官方手机号弹层必须可见、位于上层，关闭后仍保留申请草稿。 */
async function phoneOverlay(main: Page, clubId: string): Promise<void> {
  await openRoute('/pages/community/clubs/detail?id='+encodeURIComponent(clubId))
  const page=await pageAt('pages/community/clubs/detail')
  await(await button('申请加入')).tap()
  const draft=await until(()=>page.$('.club-modal textarea'),'加入俱乐部申请草稿渲染') as InputElement
  const content='本地界面验收草稿：关闭手机号层后保留'
  await draft.input(content)
  const consent=await page.$('.rules-consent');if(consent)await consent.tap()
  await(await button('提交加入申请')).tap()
  const shell=await child(page,['app-shell','components\\/AppShell']);assert.ok(shell)
  const phone=await child(shell,['phone-verification','components\\/PhoneVerification']);assert.ok(phone)
  const popup=await until(()=>child(phone,['uni-popup','node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']),'官方手机号弹层')
  const sheet=await until(async()=>{
    const direct=await phone.$('.phone-sheet')||await popup.$('.phone-sheet');if(direct)return direct
    const transitions:Element[]=await popup.$$('node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-transition\\/uni-transition')
    for(const transition of transitions){const value=await transition.$('.phone-sheet');if(value)return value}
    return null
  },'可操作手机号验证内容')
  const layer=await phone.$('.phone-layer'),mask=await page.$('.club-mask');assert.ok(layer&&mask)
  assert.ok(Number(await layer.style('z-index'))>Number(await mask.style('z-index')),'手机号弹层被申请遮罩遮挡')
  assert.ok((await sheet.text()).includes('验证手机号'))
  await screenshot('clubs-phone-overlay')
  const close=await sheet.$('.icon-btn');assert.ok(close);await close.tap()
  await until(async()=>{const submit=await button('提交加入申请');return String(await submit.property('disabled'))==='false'},'关闭手机号层后恢复申请提交')
  assert.equal(await draft.value(),content,'取消手机号验证丢失申请草稿')
  const closeJoin=await page.$('.modal-heading button');assert.ok(closeJoin);await closeJoin.tap()
  await returnToMain(page,main)
  checks.push('俱乐部申请中的手机号官方弹层位于上层，可关闭并保留申请草稿，未发送加入申请')
}

/** 保存当前执行范围的实际断言，不把布局专项报告写成全业务通过。 */
function report(scope: 'layout' | 'full' | 'admin' | 'long' | 'selectors'): void {
  assert.deepEqual(exceptions,[],'小程序存在运行异常')
  const result={passed:true,scope,time:new Date().toISOString(),api:apiBase,viewport,checks,runtime_exceptions:exceptions,remaining:['真实微信授权与商户资金不由本地模拟回归替代','订单聊天与售后由独立交易回归脚本覆盖']}
  fs.writeFileSync(path.join(outputDir,'report.json'),JSON.stringify(result,null,2)+'\n')
  console.log(JSON.stringify(result,null,2))
}

/** 在有明确本地开发标志的环境运行界面回归，保存可复查报告。 */
async function run():Promise<void> {
  assert.ok(/^http:\/\/127\.0\.0\.1:(8090|18091)\/api$/.test(apiBase),'只允许本项目本地测试 API')
  const bootstrap=await getData<{dev_mode:boolean}>('/bootstrap')
  assert.equal(bootstrap.dev_mode,true,'禁止对正式微信账号或交易运行此脚本')
  fs.mkdirSync(outputDir,{recursive:true})
  mini=await automator.connect({wsEndpoint:process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9420'})
  const system:{windowWidth:number;windowHeight:number;statusBarHeight:number;capsuleBottom:number}=await mini.evaluate(()=>{const info=wx.getWindowInfo();return{windowWidth:info.windowWidth,windowHeight:info.windowHeight,statusBarHeight:info.statusBarHeight,capsuleBottom:wx.getMenuButtonBoundingClientRect().bottom}})
  viewport={width:system.windowWidth,height:system.windowHeight,statusBarHeight:system.statusBarHeight,capsuleBottom:system.capsuleBottom};if(process.env.UI_EXPECT_WIDTH)assert.equal(viewport.width,Number(process.env.UI_EXPECT_WIDTH))
  mini.on('exception',(event:unknown)=>exceptions.push(typeof event==='object'&&event!==null&&'message'in event?String(event.message):String(event)))
  await restartRoute('/pages/account/profile')
  const previousProfile=await pageAt('pages/account/profile')
  await new Promise<void>(resolve=>setTimeout(resolve,800))
  const logout=await previousProfile.$('.logout-button')
  if(logout){await logout.tap();await pageAt('pages/main/index')}
  await mini.callWxMethod('removeStorageSync','jijie_token')
  await restartRoute('/pages/main/index?tab=player')
  let main=await pageAt('pages/main/index')
  await new Promise<void>(resolve=>setTimeout(resolve,900))
  if(process.env.UI_TEST_SCOPE==='admin'){main=await login('开机电竞管理员');await adminControls(main);await visit('/pages/manage/settings','manage-settings-admin',main);report('admin');return}
  if(process.env.UI_TEST_SCOPE==='long'){main=await login('云间旅人');await longProfile(main);main=await login('北辰');await workerLayout(main);const {items}=await bottomNavigation(main);await items[0].tap();await gameCatalog(main);report('long');return}
  await gameCatalog(main)
  if(process.env.UI_TEST_SCOPE==='selectors'){main=await login('云间旅人');await formGamePicker(main,'/pages/requests/publish','request');main=await login('北辰');await formGamePicker(main,'/pages/services/publish','service');main=await login('山海主理人');await formGamePicker(main,'/pages/community/clubs/index','club-create');await formGamePicker(main,'/pages/community/clubs/edit?id=shanhai','club-edit');report('selectors');return}
  await mainTabs(main)
  if(process.env.UI_TEST_SCOPE==='layout'){main=await login('北辰');await coverDraft(main);report('layout');return}
  const services=await getData<Service[]>('/services?limit=1'),deltaServices=await getData<Service[]>('/services?game_id=delta&limit=1'),clubs=await getData<Club[]>('/clubs?limit=1'),accounts=await getData<User[]>('/dev/accounts')
  const service=services[0],club=clubs.find(item=>item.id==='shanhai')||clubs[0],worker=accounts.find(item=>item.id==='worker')||accounts[0]
  assert.ok(service&&club&&worker,'本地界面回归需要已初始化的服务、俱乐部和体验账号')
  for(const[route,name]of[
    ['/pages/account/login','account-login'],['/pages/account/profile','account-profile-guest'],['/pages/account/agreements?type=user_agreement','account-agreements'],
    ['/pages/services/detail?id='+service.id,'services-detail'],['/pages/community/clubs/index','clubs-index'],['/pages/community/clubs/detail?id='+club.id,'clubs-detail'],['/pages/community/players/detail?id='+worker.id,'players-detail'],
  ])await visit(route,name,main)
  if(deltaServices[0])await visit('/pages/services/detail?id='+deltaServices[0].id,'services-detail-delta',main)
  main=await login('云间旅人')
  await longProfile(main)
  await visit('/pages/account/profile','account-profile-player',main)
  await formGamePicker(main,'/pages/requests/publish','request')
  await visit('/pages/requests/publish','requests-publish',main)
  await visit('/pages/manage/index','manage-denied-player',main)
  await visit('/pages/manage/settings','settings-denied-player',main)
  main=await login('北辰')
  await workerLayout(main)
  await formGamePicker(main,'/pages/services/publish','service')
  await coverDraft(main)
  await visit('/pages/services/publish','services-publish',main)
  await visit('/pages/community/clubs/edit?id='+club.id,'clubs-edit-denied-worker',main)
  main=await login('自由玩家')
  await phoneOverlay(main,'shanhai')
  main=await login('山海主理人')
  await formGamePicker(main,'/pages/community/clubs/index','club-create')
  await formGamePicker(main,'/pages/community/clubs/edit?id=shanhai','club-edit')
  await visit('/pages/community/clubs/detail?id=shanhai','clubs-detail-owner',main)
  await visit('/pages/community/clubs/edit?id=shanhai','clubs-edit-owner',main)
  main=await login('开机电竞管理员')
  await adminControls(main)
  await visit('/pages/manage/index','manage-index-admin',main)
  await visit('/pages/manage/settings','manage-settings-admin',main)
  await visit('/pages/orders/detail?id=missing-ui-regression','orders-detail-denied',main)
  report('full')
}

run().catch(error=>{fs.mkdirSync(outputDir,{recursive:true});fs.writeFileSync(path.join(outputDir,'report.json'),JSON.stringify({passed:false,api:apiBase,viewport,checks,runtime_exceptions:exceptions,error:error instanceof Error?error.message:String(error)},null,2)+'\n');console.error(error instanceof Error?error.stack:String(error));process.exitCode=1}).finally(async()=>{if(mini)await mini.disconnect()})
