/** 文件作用：使用微信官方自动化 SDK 验证访客、协议登录、资料弹层、主页和四角色管理入口，保留真实截图。 */
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const automator = require('../frontend/node_modules/miniprogram-automator')
const output = path.resolve(__dirname, '../output/community')
const report = { passed: false, steps: [], exceptions: [], real_wechat_login: null, timestamp: new Date().toISOString() }
let mini
/** 等待可见页面状态，超时给出明确定位，不写内部组件状态。 */
async function until(read, label, timeout = 18000) { const start = Date.now(); while (Date.now()-start<timeout) { const value = await read(); if(value)return value; await new Promise(resolve=>setTimeout(resolve,180)) } throw new Error('等待超时：'+label) }
/** 打开已有页面并等待原生转场结束。 */
async function open(route) { await mini.reLaunch(route); const page=await until(async()=>{const p=await mini.currentPage();return p?.path===route.split('?')[0].replace(/^\//,'')?p:null},route);await page.waitFor(650);return page }
/** 查找页面可见文字按钮。 */
async function button(scope, label) { return until(async()=>{for(const item of await scope.$$('button'))if((await item.text()).trim()===label)return item;return null},label) }
/** 记录真实界面检查点。 */
function pass(label) { report.steps.push(label); console.log('PASS '+label) }
/** 截取微信模拟器实际显示结果。 */
async function shot(name) { await mini.screenshot({path:path.join(output,name+'.png')}) }
/** 原生复选框经 label 点击才触发真实选择，并核对界面已明确勾选。 */
async function agree(page) { const label=await until(()=>page.$('.consent-line label'),'协议勾选标签');await label.tap();await until(async()=>String(await(await page.$('checkbox')).property('checked'))==='true','协议已明确勾选') }
/** 按页面按钮进入明确标识的本地体验身份。 */
async function login(name) {
 const page=await open('/pages/login/index')
 await agree(page)
 await (await button(page,'开发体验')).tap()
 await until(async()=>{for(const item of await page.$$('.account-card'))if((await item.text()).includes(name)){await item.tap();return true}return false},'体验身份 '+name)
 await (await button(page,'先这样，稍后完善')).tap()
 await until(async()=>{const p=await mini.currentPage();return p?.path==='pages/me/index'},'登录后个人中心')
 await (await mini.currentPage()).waitFor(450)
 pass(name+' 通过协议登录，关闭可选资料弹层')
}
/** 执行真实微信与本地四角色验证，真实微信失败记录限制并继续功能回归。 */
async function run() {
 fs.mkdirSync(output,{recursive:true})
 mini=await automator.connect({wsEndpoint:'ws://127.0.0.1:9420'})
 mini.on('exception',error=>report.exceptions.push(String(error.message||error)))
 let page=await open('/pages/profile/index')
 if(await mini.callWxMethod('getStorageSync','jijie_token')){await(await button(page,'退出登录')).tap();await page.waitFor(400)}
 page=await open('/pages/me/index'); assert.ok((await(await page.$('.profile-top')).text()).includes('登录后查看'));assert.equal((await page.$$('.stats-row>view')).length,3);await shot('mini-me-guest');pass('访客个人中心完整布局与零数据')
 page=await open('/pages/orders/index');assert.ok((await(await until(()=>page.$('.guest-note'),'订单登录小字')).text()).includes('请登录后查看'));await shot('mini-orders-guest');pass('访客订单保留身份和状态筛选')
 page=await open('/pages/worker/index');assert.ok(await page.$('.worker-stats'));assert.ok(await page.$('.worker-tabs'));await shot('mini-worker-guest');pass('访客工作台保留统计、发布和接单大厅')
 page=await open('/pages/login/index');await(await button(page,'登录集结')).tap();assert.ok(!(await mini.callWxMethod('getStorageSync','jijie_token')));pass('未勾协议不建立会话')
 await agree(page);await(await button(page,'登录集结')).tap()
 await until(async()=>{const token=await mini.callWxMethod('getStorageSync','jijie_token');if(token){report.real_wechat_login={success:true};return true}const error=await page.$('.login-error');if(error&&(await error.text()).trim()){report.real_wechat_login={success:false,message:(await error.text()).trim()};return true}return false},'微信官方登录结果',30000)
 await shot('mini-wechat-login-result');console.log('WECHAT '+JSON.stringify(report.real_wechat_login))
 if(!report.real_wechat_login.success){report.real_wechat_login.official_diagnostic=await mini.evaluate(()=>new Promise(resolve=>wx.login({timeout:10000,success:result=>resolve({success:!!result.code,errMsg:result.errMsg}),fail:failure=>resolve({success:false,errMsg:failure.errMsg,errno:failure.errno})})));console.log('WECHAT_DIAGNOSTIC '+JSON.stringify(report.real_wechat_login.official_diagnostic))}
 if(report.real_wechat_login.success)await(await button(page,'先这样，稍后完善')).tap()
 await login('云间旅人')
 page=await open('/pages/profile/index');assert.ok(await page.$('.profile-form'));pass('玩家独立资料配置页可用')
 page=await open('/pages/player/index?id=worker');assert.ok(await page.$('.player-content'));await shot('mini-player-home');pass('玩家公开主页可访问')
 await login('北辰');page=await open('/pages/worker/index');assert.ok(await page.$('.worker-tabs'));await shot('mini-worker-user');pass('打手工作台可用')
 await login('山海主理人');page=await open('/pages/club/index?id=shanhai');await shot('mini-club-home');pass('俱乐部管理员可打开俱乐部主页')
 await login('集结管理员');page=await open('/pages/admin-settings/index');assert.ok(await page.$('.settings-tabs'));await shot('mini-admin-settings');pass('管理员可打开协议与支付配置')
 await login('自由玩家');page=await open('/pages/admin-settings/index');assert.ok(!(await page.$('.settings-tabs')));pass('普通用户无协议支付管理权限')
 assert.deepEqual(report.exceptions,[])
 report.passed=true
}
run().catch(error=>{report.error=error.stack;console.error(error.stack);process.exitCode=1}).finally(async()=>{fs.writeFileSync(path.join(output,'mini-community-report.json'),JSON.stringify(report,null,2));if(mini)await mini.disconnect()})
