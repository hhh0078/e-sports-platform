/** 文件作用：通过微信官方SDK验证独立主页面的实际路径、导航和订单筛选缓存，仅连接本轮本地验收API。 */
const assert=require('node:assert/strict');
const path=require('node:path');
const fs=require('node:fs');
const automator=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator');
const MiniProgram=require('/Users/Zhuanz/Desktop/三角洲/frontend/node_modules/miniprogram-automator/out/MiniProgram').default;
/** 读取真实基础库版本，兼容当前IDE返回的非语义版本号。 */
MiniProgram.prototype.checkVersion=async function(){const info=await this.callWxMethod('getSystemInfoSync');assert.ok(info.SDKVersion);console.log('SDK',info.SDKVersion)};
const project=path.resolve('.run/independent-tabs-mini-v2'),out=path.resolve('output/independent-tabs');
/** 等待可观察SDK结果，不注入页面内部状态。 */
async function until(read,label){for(let i=0;i<40;i++){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,150))}throw new Error('等待超时 '+label)}
/** 关闭首次身份提醒，只点本页实际可见按钮。 */
async function closeIdentity(frame){const identity=await frame.$('components\\/IdentityVerification');if(identity){const buttons=await identity.$$('.identity-secondary');for(const button of buttons){const box=await button.size();if(box.height>0){await button.tap();break}}}}
/** 获得当前共享框架，原生路由与页面数据相互独立。 */
async function frameOf(page){return until(()=>page.$('components\\/MainPage'),'共享框架')}
/** 通过真正底栏事件切换，确认微信当前Page.path发生变化。 */
async function switchNav(mini,page,label,want){const frame=await frameOf(page);const nav=await frame.$('components\\/BottomNav');assert.ok(nav);const buttons=await nav.$$('.nav-item');for(const button of buttons)if((await button.text()).trim()===label){await button.tap();break}return until(async()=>{const next=await mini.currentPage();return next?.path===want?next:null},'独立路径 '+want)}
/** 在本地API和官方模拟器中检查页面缓存，不发送支付或退款。 */
async function main(){const health=await fetch('http://127.0.0.1:18096/health').then(res=>res.json());assert.equal(health.data.dev_mode,true);fs.mkdirSync(out,{recursive:true});const mini=process.env.MINI_CONNECT==='1'?await automator.connect({wsEndpoint:'ws://127.0.0.1:9429'}):await automator.launch({projectPath:project,port:9429,trustProject:true,timeout:60000});const exceptions=[];mini.on('exception',error=>exceptions.push(String(error)));const checks=[];try{
 let page=await mini.reLaunch('/pages/main/orders/index');await page.waitFor(1800);assert.equal(page.path,'pages/main/orders/index');let frame=await frameOf(page);await closeIdentity(frame);checks.push('直接打开独立订单页');await mini.screenshot({path:path.join(out,'mini-orders-direct.png')});
 const orders=await page.$('pages\\/main\\/orders\\/orders');assert.ok(orders,'独立订单页必须实际挂载订单视图');const roles=await orders.$$('.role-tab');assert.equal(roles.length,2);await roles[1].tap();const filters=await orders.$$('.filter');for(const filter of filters)if((await filter.text()).trim()==='进行中'){await filter.tap();break};
 page=await switchNav(mini,page,'我的','pages/main/me/index');checks.push('我的独立路径');page=await switchNav(mini,page,'订单','pages/main/orders/index');await page.waitFor(350);const restored=await page.$('pages\\/main\\/orders\\/orders');assert.equal((await(await restored.$('.role-tab.selected')).text()).trim(),'我接的单');assert.equal((await(await restored.$('.filter.selected')).text()).trim(),'进行中');checks.push('订单身份和筛选保留');
 page=await switchNav(mini,page,'打手','pages/main/worker/index');checks.push('打手独立路径');page=await switchNav(mini,page,'玩家','pages/main/index');await page.waitFor(1400);frame=await frameOf(page);const announcement=await frame.$('components\\/HomeAnnouncement');if(announcement){const close=await announcement.$('.announcement-close');if(close&&(await close.size()).height>0){await mini.screenshot({path:path.join(out,'mini-announcement-compressed.png')});await close.tap()}}
 checks.push('玩家首页独立路径');assert.deepEqual(exceptions,[]);const info=await mini.callWxMethod('getSystemInfoSync');const report={passed:true,checks,SDKVersion:info.SDKVersion,windowWidth:info.windowWidth,exceptions,externalPayment:false,realDevice:false,directLocalNetwork:false,scope:'仅实际原生路由、订单筛选保留；模拟器未接通本地API；滚动与图片由H5真实API验收'};fs.writeFileSync(path.join(out,'mini-report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report));
 }finally{await mini.disconnect()}}
main().catch(error=>{console.error(error);process.exitCode=1});
