/** 文件作用：在明确本地开发环境用微信官方SDK验收图标、封面卡片、充值余额付款退款和自愿押金，禁止连接生产交易。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { Wallet } from '../frontend/src/types/wallet'
import type { Service,Order,AuthSession } from '../frontend/src/types/models'

const root=process.cwd(),api=process.env.UI_TEST_API||'http://127.0.0.1:18091/api',output=process.env.UI_TEST_OUTPUT||path.join(root,'output/game-ui-wallet/mini-390')
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
const checks:string[]=[],exceptions:string[]=[]
let mini:MiniProgram|undefined
/** evaluate只使用官方微信窗口与导航能力，不写入会话。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number};navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void}
/** 明确界面前置条件，等待超时则指出失败步骤。 */
async function until<T>(read:()=>Promise<T|false|null|undefined>,label:string):Promise<T>{const start=Date.now();while(Date.now()-start<15000){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,150))}throw new Error('等待超时：'+label)}
/** 取得已连接SDK。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 只向本地服务创建测试夹具，敏感会话不输出到证据。 */
async function call<T>(route:string,method='GET',body?:unknown,token=''):Promise<T>{const response=await fetch(api+route,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(body?{body:JSON.stringify(body)}:{})});const result=await response.json() as {data:T;error?:string};assert.ok(response.ok,route+' '+response.status+' '+(result.error||''));return result.data}
/** 从原生路由确认页面实例已经改变，避免 SDK 等待已销毁回调。 */
async function open(url:string,restart=false):Promise<Page>{const old=await sdk().currentPage();await sdk().evaluate((target:string,reset:boolean)=>{if(reset)setTimeout(()=>wx.reLaunch({url:target}),30);else wx.navigateTo({url:target});return true},url,restart);return until(async()=>{const page=await sdk().currentPage();return page?.path===url.split('?')[0].slice(1)&&(!restart||page!==old)?page:null},url)}
/** 兼容uni-app自定义组件路径边界。 */
async function child(scope:Page|Element,names:string[]):Promise<Element|null>{for(const name of names){const item=await scope.$(name);if(item)return item}return null}
/** 通过实际可见按钮文字操作，不调用页面内部方法。 */
async function button(label:string):Promise<Element>{return until(async()=>{const page=await sdk().currentPage();if(!page)return null;for(const item of await page.$$('button'))if((await item.text()).replace(/[\uE000-\uF8FF]/g,'').trim()===label)return item;return null},'按钮 '+label)}
/** 读取真实输入组件。 */
async function input(page:Page,selector:string):Promise<InputElement>{const item=await until(()=>page.$(selector),selector);assert.ok('input'in item);return item as InputElement}
/** 记录断言和真实模拟器截图。 */
async function record(label:string,imageName?:string){checks.push(label);console.log('PASS '+label);if(imageName){await new Promise(resolve=>setTimeout(resolve,450));await sdk().screenshot({path:path.join(output,imageName+'.png')})}}
/** 主动勾选本地测试协议并从体验身份选择器登录。 */
async function login(name:string):Promise<Page>{const page=await open('/pages/account/login',true);await(await until(()=>page.$('.consent-line label'),'协议')).tap();await(await button('开发体验')).tap();await(await until(async()=>{for(const item of await page.$$('button'))if((await item.text()).includes(name))return item;return null},name)).tap();await(await button('先这样，稍后完善')).tap();return until(async()=>{const current=await sdk().currentPage();return current?.path==='pages/main/index'?current:null},'登录回跳')}
/** 获得真正元素矩形。 */
async function rect(item:Element){const[a,b]=await Promise.all([item.offset(),item.size()]);return{left:Number(a.left),top:Number(a.top),width:Number(b.width),height:Number(b.height)}}
/** 充值/缴押金过程中允许使用明确标记的本地手机号验证，绝不触发真实手机号或资金。 */
async function finishMoneyForm(page:Page){await until(async()=>{if(!(await page.$('.wallet-modal')))return true;const shell=await child(page,['app-shell','components\\/AppShell']);const phone=shell&&await child(shell,['phone-verification','components\\/PhoneVerification']);const popup=phone&&await child(phone,['uni-popup','node-modules\\/\\@dcloudio\\/uni-ui\\/lib\\/uni-popup\\/uni-popup']);const control=phone&&(await phone.$('.phone-dev')||popup&&await popup.$('.phone-dev'));if(control){await control.tap();return false}const modal=await page.$('.wallet-modal');return!modal},'真实资金表单提交完成')}
/** 通过钱包界面发起模拟付款并验证服务端真实余额增量。 */
async function topup(label:string,amount:string){const page=await open('/pages/account/wallet');await(await button(label)).tap();await(await input(page,'.amount-input')).input(amount);await(await button('确认'+(label==='自愿缴纳'?'缴纳押金':'充值'))).tap();await finishMoneyForm(page);return page}
/** 检查手柄主题色及首页、俱乐部封面文字实际边界。 */
async function visual(main:Page,width:number){
  const nav=await child(main,['bottom-nav','components\\/BottomNav']);assert.ok(nav);const items=await nav.$$('.nav-item');await items[0].tap();await until(async()=>(await items[0].attribute('class')).includes('selected'),'玩家选中渲染')
  const icon=await child(items[0],['icon','components\\/Icon']);assert.ok(icon);const pad=await icon.$('.gamepad-image');assert.ok(pad)
  const selected=await pad.style('background-color');assert.notEqual(selected,'rgba(0, 0, 0, 0)');await record('玩家手柄使用当前主题色，统一轮廓实际可见','nav-player')
  await items[1].tap();await until(async()=>(await pad.style('background-color'))!==selected,'手柄未选中灰色');const muted=await pad.style('background-color');assert.notEqual(selected,muted);await items[0].tap();await until(async()=>(await pad.style('background-color'))===selected,'手柄恢复选中颜色')
  const player=await child(main,['player-view','pages\\/main\\/player\\/player']);assert.ok(player);const text=await player.$('.hero-text'),publish=await player.$('.hero-publish');assert.ok(text&&publish)
  const a=await rect(text),b=await rect(publish);assert.ok(a.left+a.width<=b.left+1&&b.left+b.width<=width-15);assert.equal(Math.round(b.height),36);await record('首页左文右按钮，无水平碰撞且紧凑按钮36px')
  const picker=await child(player,['game-picker','components\\/GamePicker']);assert.ok(picker);const choices=await picker.$$('.game-choice');const honor=await until(async()=>{for(const choice of choices)if((await choice.text()).includes('王者'))return choice;return null},'王者游戏');await honor.tap();await until(async()=>(await pad.style('background-color'))!==selected,'切换游戏更新玩家图标颜色');await record('切换游戏后手柄自动使用新的主题色','nav-player-honor');const delta=await until(async()=>{for(const choice of choices)if((await choice.text()).includes('三角洲'))return choice;return null},'三角洲游戏');await delta.tap();await until(async()=>(await pad.style('background-color'))===selected,'恢复三角洲主题')
  const clubs=await open('/pages/community/clubs/index');const card=await until(()=>child(clubs,['club-card','components\\/ClubCard']),'俱乐部卡片');const box=await card.$('.club-card');assert.ok(box);const area=await rect(box)
  for(const name of ['.club-name','.club-meta','.club-description','.game-tags','.club-footer']){const item=await card.$(name);assert.ok(item);const r=await rect(item);assert.ok(r.left>=area.left-1&&r.left+r.width<=area.left+area.width+1&&r.top>=area.top-1&&r.top+r.height<=area.top+area.height+1,name+'越出封面')}
  await record('俱乐部名称、简介、标签、准入信息均在完整封面内','club-cards')
}
/** 验收本地充值、余额订单付款退款和可选押金服务标识。 */
async function run(){
  assert.match(api,/^http:\/\/127\.0\.0\.1:(8090|18091)\/api$/);assert.equal((await call<{dev_mode:boolean}>('/bootstrap')).dev_mode,true)
  fs.mkdirSync(output,{recursive:true});mini=await automator.connect({wsEndpoint:process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9420'});mini.on('exception',(e:unknown)=>exceptions.push(String(e)))
  const viewport=await mini.evaluate(()=>wx.getWindowInfo());if(process.env.UI_EXPECT_WIDTH)assert.equal(viewport.windowWidth,Number(process.env.UI_EXPECT_WIDTH))
  let main=await login('北辰');await visual(main,viewport.windowWidth)
  const fixtureBuyer=await call<AuthSession>('/auth/dev','POST',{user_id:'player'});await call('/me/phone/dev','POST',{phone_number:'13300000021'},fixtureBuyer.token)
  const blockingOrder=await call<Order>('/orders','POST',{service_id:'delta-service-1',quantity:1,requirements:'本地押金退还阻塞夹具，测试后取消。'},fixtureBuyer.token)
  const worker=await call<AuthSession>('/auth/dev','POST',{user_id:'worker'}),before=await call<Wallet>('/wallet','GET',undefined,worker.token)
  let walletPage=await topup('自愿缴纳','2.34');const deposited=await call<Wallet>('/wallet','GET',undefined,worker.token);assert.equal(deposited.deposit_cents,before.deposit_cents+234);assert.equal(deposited.available_cents,before.available_cents)
  await record('平台押金独立记账，不增加可提现或消费余额','wallet-deposit')
  const service=await call<Service>('/services/delta-service-1');assert.equal(service.provider_deposit_cents,deposited.deposit_cents);const detail=await open('/pages/services/detail?id='+service.id);await until(async()=>{const section=await detail.$('.provider-deposit');return section&&(await section.text()).includes((deposited.deposit_cents/100).toFixed(2).replace(/\.00$/,'').replace(/(\.\d)0$/,'$1'))?section:null},'有效押金详情')
  await record('服务实时展示有效平台押金及金额','service-deposit')
  walletPage=await open('/pages/account/wallet');await(await button('申请退还押金')).tap();await(await button('确认退还押金')).tap();const blocked=await until(async()=>{const e=await walletPage.$('.form-error');return e&&(await e.text()).includes('订单')?e:null},'履约订单阻塞押金退还');assert.ok(blocked);assert.equal((await call<Wallet>('/wallet','GET',undefined,worker.token)).deposit_cents,deposited.deposit_cents);await record('待付款/履约订单阻止退还押金，错误保留真实金额','deposit-blocked');await call('/orders/'+blockingOrder.id+'/actions','POST',{action:'cancel',note:''},fixtureBuyer.token)
  main=await login('云间旅人');const buyer=await call<AuthSession>('/auth/dev','POST',{user_id:'player'}),original=await call<Wallet>('/wallet','GET',undefined,buyer.token)
  await topup('充值','100.00');const recharged=await call<Wallet>('/wallet','GET',undefined,buyer.token);assert.equal(recharged.recharge_cents,original.recharge_cents+10000);assert.equal(recharged.available_cents,original.available_cents);await record('钱包界面充值100元，只进入充值消费余额','wallet-recharged')
  const order=await call<Order>('/orders','POST',{service_id:service.id,quantity:1,requirements:'钱包模拟验收：完成付款与原分账退款，非真实服务交易。'},buyer.token);const page=await open('/pages/orders/detail?id='+order.id)
  await(await button('支付订单')).tap();const options=await until(async()=>{const rows=await page.$$('.payment-option');return rows.length===2?rows:null},'双渠道');await options[1].tap();await(await button('确认支付订单')).tap();await until(async()=>{const title=await page.$('.status-title');return title&&(await title.text())==='等待接单'},'余额订单已付');assert.equal((await call<Wallet>('/wallet','GET',undefined,buyer.token)).recharge_cents,recharged.recharge_cents-order.amount_cents);await record('订单确认弹层余额渠道真实扣款','balance-payment')
  await(await button('取消订单')).tap();await(await button('确认取消订单')).tap();await until(async()=>{const current=await call<Order>('/orders/'+order.id,'GET',undefined,buyer.token);return current.payment_status==='refunded'},'原分账退款');assert.equal((await call<Wallet>('/wallet','GET',undefined,buyer.token)).recharge_cents,recharged.recharge_cents);await record('余额订单取消精确恢复原充值余额','balance-refunded')
  await open('/pages/manage/wallet');await until(async()=>{const p=await sdk().currentPage();const e=p&&await p.$('.access-note');return e&&(await e.text()).includes('管理员')},'非管理员拒绝');await record('普通玩家无法访问钱包管理')
  main=await login('开机电竞管理员');const adminWallet=await open('/pages/manage/wallet');await until(async()=>{const filters=await adminWallet.$$('.status-filter');return filters.length===6&&await adminWallet.$('.payment-link')},'管理员钱包真实配置已渲染');await record('管理员钱包审核与转账公开配置可正常打开','wallet-admin')
  assert.deepEqual(exceptions,[]);fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed:true,viewport,checks,runtime_exceptions:exceptions,remaining:['真实微信手机号与商户资金未用本地模拟替代']},null,2)+'\n');console.log('通过 '+checks.length+' 项')
}
run().catch(error=>{console.error(error);process.exitCode=1}).finally(()=>mini?.disconnect())
