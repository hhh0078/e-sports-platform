/** 文件作用：在隔离本地开发环境真实操作三类教程，先预检再录屏；不使用宣传假接口，也不访问生产API。 */
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import {createRequire} from 'node:module'
import type {Page,Locator,BrowserContext,Browser} from 'playwright'
import type sharpType from 'sharp'

const root=process.cwd(),base='http://127.0.0.1:5174',api='http://127.0.0.1:18091/api'
const output=path.join(root,'output/playwright/tutorials-20261026'),preflight=process.env.TUTORIAL_MODE==='preflight'
const modules=path.join(os.homedir(),'.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules')
const runtime=createRequire(import.meta.url),{chromium}=runtime(path.join(modules,'playwright')) as typeof import('playwright')
const sharp=runtime(path.join(modules,'sharp')) as typeof sharpType
/** 单个真实操作步骤及其在原始录屏中的秒数范围。 */
interface Step{id:string;title:string;narration:string;start:number;end:number}
/** 教程展示信息和已测量的真实页面区域，裁掉录屏工具添加的空白。 */
interface Tutorial{id:string;title:string;audience:string;raw:string;content_crop:{x:number;y:number;width:number;height:number};steps:Step[]}
/** 仅保存在本机忽略目录的真实开发会话及演示对象编号。 */
interface State{accounts:Record<string,{token:string}>;service_id:string;team_order_id:string;moderation_task_id:string;club_id:string}
const tutorials:Tutorial[]=[],errors:string[]=[],requests:Array<{method:string;path:string;status:number}>=[]
let page:Page,context:BrowserContext,browser:Browser,current:Tutorial,t0=0,state:State,captureSize:{width?:number;height?:number}|undefined

/** 等待真实界面稳定，只检查可见错误；图片等待不改变产品数据。 */
async function ready(selector:string):Promise<void>{await page.locator(selector).first().waitFor({state:'visible',timeout:25000});await page.waitForTimeout(500);const error=await page.locator('.error-box:visible,.chat-error:visible,.error-panel:visible').allTextContents();assert.equal(error.length,0,error.join('；'))}
/** 用鼠标滚动把真实控件放到视野中，避免剪掉关键操作。 */
async function see(target:Locator):Promise<void>{await target.waitFor({state:'visible'});const box=await target.boundingBox();if(box&&(box.y<60||box.y+box.height>700)){await page.mouse.move(350,420);await page.mouse.wheel(0,box.y-220);await page.waitForTimeout(500)}await target.scrollIntoViewIfNeeded()}
/** 真实点击并保留短暂反馈，录制原速不以全片倍速代替修剪。 */
async function click(target:Locator):Promise<void>{await see(target);await target.click();await page.waitForTimeout(450)}
/** 真实逐字输入；清空原值后以正常手速输入中文演示内容。 */
async function type(target:Locator,text:string):Promise<void>{const child=target.locator('input,textarea').first(),editable=await child.count()?child:target;await see(editable);await editable.click();await editable.fill('');await editable.pressSequentially(text,{delay:65});await page.waitForTimeout(350)}
/** 聊天必须等待服务端确认后的具体消息，已有系统消息不能冒充发送成功。 */
async function sendMessage(text:string):Promise<void>{await type(page.locator('.composer-input'),text);await click(page.locator('.send-button'));await page.locator('.message-text').filter({hasText:text}).last().waitFor({state:'visible',timeout:10000})}
/** 一次步骤只保留动作及结果，步骤之间的准备和换号不进入成片。 */
async function step(title:string,narration:string,action:()=>Promise<void>):Promise<void>{const start=(Date.now()-t0)/1000;await page.waitForTimeout(250);await action();await page.waitForTimeout(650);const end=(Date.now()-t0)/1000;current.steps.push({id:String(current.steps.length+1).padStart(2,'0'),title,narration,start,end});await page.screenshot({path:path.join(output,preflight?'preflight':'qa-source',current.id+'-'+String(current.steps.length).padStart(2,'0')+'.png')});console.log((preflight?'预检':'录制')+' '+current.id+' '+title)}
/** 后台身份只使用本地真实登录返回的会话，切视角准备不伪装前台交互。 */
async function identity(id:string,route='/pages/main/index?tab=me'):Promise<void>{assert.ok(state.accounts[id]?.token);await page.goto(base,{waitUntil:'domcontentloaded'});await page.evaluate(({token})=>{localStorage.clear();localStorage.setItem('jijie_token',token);localStorage.setItem('jijie_game','delta')},{token:state.accounts[id].token});await page.goto(base+'/#'+route,{waitUntil:'networkidle'});await page.reload({waitUntil:'networkidle'});await page.waitForTimeout(500)}
/** 正常返回按钮保持产品页面栈；只在换视角时使用明确的深链。 */
async function back():Promise<void>{await click(page.locator('.uni-navbar__header-btns-left:visible,.floating-back:visible').last())}
/** 发起本地断言请求，令牌不写入输出报告。 */
async function read(id:string,route:string):Promise<Record<string,unknown>>{const response=await fetch(api+route,{headers:{Authorization:'Bearer '+state.accounts[id].token}});assert.ok(response.ok,route+' '+response.status);const result=await response.json() as {data:Record<string,unknown>};return result.data}
/** 新教程建立独立上下文、真实网络保护与高分辨率录屏。 */
async function begin(id:string,title:string,audience:string,account:string):Promise<void>{captureSize=undefined;current={id,title,audience,raw:'raw/'+id+'.webm',content_crop:{x:0,y:0,width:393,height:759},steps:[]};tutorials.push(current);context=await browser.newContext({viewport:{width:393,height:759},deviceScaleFactor:2,isMobile:true,hasTouch:true,locale:'zh-CN',timezoneId:'Asia/Shanghai'});await context.route('**/*',async route=>{const url=new URL(route.request().url());if(url.pathname.startsWith('/api/')&&url.origin!=='http://127.0.0.1:18091'){errors.push('阻止非本地API '+url.origin);await route.abort();return}await route.continue()});if(!preflight)await context.addInitScript(()=>{document.addEventListener('pointerdown',event=>{const ring=document.createElement('div');ring.style.cssText='position:fixed;z-index:2147483647;pointer-events:none;width:34px;height:34px;border:2px solid #79edb3;border-radius:50%;background:#79edb333;left:'+(event.clientX-17)+'px;top:'+(event.clientY-17)+'px';document.documentElement.appendChild(ring);ring.animate([{opacity:1,transform:'scale(.65)'},{opacity:0,transform:'scale(1.25)'}],{duration:550,easing:'ease-out'}).onfinish=()=>ring.remove()})});page=await context.newPage();page.on('pageerror',error=>errors.push(current.id+': '+error.message));page.on('response',response=>{const url=new URL(response.url());if(url.pathname.startsWith('/api/'))requests.push({method:response.request().method(),path:url.pathname,status:response.status()})});await identity(account);if(!preflight){await page.screencast.start({path:path.join(output,current.raw),size:{width:394,height:760},quality:95,onFrame:async({data,timestamp})=>{if(!t0){t0=timestamp;const metadata=await sharp(data).metadata();captureSize={width:metadata.width,height:metadata.height}}}});for(let attempt=0;!captureSize&&attempt<200;attempt++)await page.waitForTimeout(25);assert.equal((captureSize as {width?:number;height?:number}|undefined)?.width,393,'录屏JPEG真实宽度改变，必须重新校准裁切');assert.equal((captureSize as {width?:number;height?:number}|undefined)?.height,759,'录屏JPEG真实高度改变，必须重新校准裁切')}else t0=Date.now();await page.waitForTimeout(300)}
/** 录屏保存完成后销毁会话，不向素材包导出令牌。 */
async function finish():Promise<void>{if(!preflight)await page.screencast.stop();await context.close();t0=0}
/** 完整平台运营路径：真实数据查看、审核决定、公共信息保存与回读。 */
async function admin():Promise<void>{await begin('01-platform-tutorial','平台管理员 · 运营上手','平台管理员','admin');
 await step('进入管理中心','平台日常运营，从我的页面进入平台管理中心。',async()=>{await ready('.menu-item');await click(page.locator('.menu-item').filter({hasText:'平台管理中心'}));await ready('.metric-money')});
 await step('查看经营趋势','切换统计周期，查看真实订单金额和经营趋势。',async()=>{await click(page.locator('.range-tab').filter({hasText:'30天'}));await ready('.period-label')});
 await step('查找用户','打开用户管理，输入用户编号，快速定位资料。',async()=>{await click(page.locator('.module-card').filter({hasText:'用户管理'}));await ready('.manage-search-input');await type(page.locator('.manage-search-input'),'outsider');await click(page.locator('.manage-search-button'));await ready('.user-row')});
 await step('核对用户资料','点开用户详情，核对身份和手机验证状态。',async()=>{await click(page.locator('.user-row').filter({hasText:'outsider'}));await ready('.person-name,.person-nickname,.managed-name,.managed-profile-name,.profile-name')});
 await back();await back();
 await step('打开图文审核','回到管理中心，进入图文审核，筛选待人工内容。',async()=>{await click(page.locator('.module-card').filter({hasText:'图文审核'}));await ready('.moderation-filter');await click(page.locator('.moderation-filter').filter({hasText:'待人工'}));await ready('.moderation-row')});
 await step('查看完整内容','打开任务，先看完整内容，再决定是否通过。',async()=>{await click(page.locator('.moderation-row').filter({hasText:state.service_id}).first());await ready('.detail-content')});
 await step('确认审核结果','确认内容符合要求后，点击通过，结果会保留记录。',async()=>{await click(page.locator('.allow-action'));await click(page.getByText('确认通过',{exact:true}));await ready('.detail-readonly')});
 await click(page.locator('.finance-popup-close'));await back();
 await step('编辑首页提示','公共信息里选择顶部提示，填写玩家首页要展示的内容。',async()=>{await click(page.locator('.module-card').filter({hasText:'公共信息'}));await ready('.content-tabs');await click(page.locator('.content-tabs .tab').filter({hasText:'顶部提示'}));await type(page.locator('.form-section').filter({hasText:'玩家页顶部提示'}).locator('textarea'),'欢迎来到开机电竞，先沟通需求，再安心下单。')});
 await step('保存并查看效果','保存公共信息后，返回玩家首页，查看提示是否生效。',async()=>{await click(page.getByText('保存公共信息',{exact:true}));await page.waitForTimeout(500);const saved=await read('admin','/public-content?game_id=delta');assert.ok(JSON.stringify(saved).includes('先沟通需求'));await back();await back();await click(page.locator('.nav-item[aria-label="玩家"]'));await ready('.service-card');assert.ok((await page.locator('body').innerText()).includes('先沟通需求'))});
 await finish()}
/** 组织管理路径：成员审核、团队订单沟通与组织资金分区。 */
async function club():Promise<void>{await begin('02-club-tutorial','俱乐部管理员 · 团队协作','俱乐部管理员','clubowner');
 await step('进入俱乐部控制台','在我的页面打开俱乐部中心，进入自己的管理控制台。',async()=>{await click(page.locator('.menu-item').filter({hasText:'俱乐部中心'}));await ready('.manage-entry');await click(page.locator('.manage-entry'));if(await page.locator('.management-club').isVisible())await click(page.locator('.management-club').filter({hasText:'山海电竞'}));await ready('.club-console-title')});
 await step('查看加入规则','加入规则与装修里，可以核对抽成、押金和团队介绍。',async()=>{await click(page.locator('.club-console-link').filter({hasText:'加入规则与装修'}));await ready('.review-note');await page.mouse.wheel(0,430);await page.waitForTimeout(500)});
 await back();
 await step('查找入会申请','打开成员管理，搜索申请人，找到待审核的申请。',async()=>{await click(page.locator('.club-console-link').filter({hasText:'成员管理'}));await ready('.managed-search-input');await type(page.locator('.managed-search-input'),'outsider');await click(page.locator('.managed-member-search').getByText('搜索',{exact:true}));await ready('.managed-member-row')});
 await step('填写审核说明','核对申请人，点击审核，填写本次处理说明。',async()=>{await click(page.locator('.managed-member-row').filter({hasText:'outsider'}).getByText('审核',{exact:true}));await type(page.locator('.club-modal textarea'),'已核对游戏经验，同意加入团队。');const check=page.locator('.club-modal .rules-consent');if(await check.isVisible())await click(check)});
 await step('完成成员认证','确认处理后，申请人会成为俱乐部的认证成员。',async()=>{await click(page.locator('.club-modal').getByText('确认处理',{exact:true}));await page.waitForTimeout(600);assert.ok((await page.locator('.managed-member-row').filter({hasText:'outsider'}).innerText()).includes('认证成员'))});
 await back();
 await step('查看团队订单','进入团队订单，打开订单详情，查看履约进度。',async()=>{await click(page.locator('.club-console-link').filter({hasText:'团队订单'}));await ready('.team-order');await click(page.locator('.team-order').filter({hasText:'教程'}).filter({has:page.getByText('服务中',{exact:true})}).first());await ready('.order-heading');assert.ok(page.url().includes('id='+state.team_order_id))});
 await step('协助订单沟通','订单聊天保存沟通记录，管理员也能协助双方确认进度。',async()=>{await click(page.locator('.chat-entry'));await ready('.composer-input');await sendMessage('俱乐部已跟进，请按约定时间集合。');await ready('.message')});
 await page.goto(base+'/#/pages/community/clubs/console?id='+state.club_id,{waitUntil:'networkidle'});
 await step('查看组织资金','俱乐部资金会分别展示押金、进行中佣金和待结算金额。',async()=>{await click(page.locator('.club-console-link').filter({hasText:'俱乐部资金'}));await ready('.funds-value');await page.mouse.wheel(0,400);await page.waitForTimeout(500)});
 await finish()}
/** 订单主动作包含明确确认，交付文字作为真实订单记录保存。 */
async function orderAction(label:string,note?:string):Promise<void>{await click(page.locator('.dock-primary').filter({hasText:label}));await ready('.modal-title');if(note)await type(page.locator('.modal-input'),note);await click(page.locator('.modal-actions .btn').filter({hasText:'确认'+label}));await page.waitForTimeout(600)}
/** 交易教程依次切换玩家和接单者视角，走完真实本地模拟支付及履约。 */
async function transaction():Promise<void>{await begin('03-order-tutorial','玩家与接单者 · 完整订单','玩家与游戏爱好者','player');let orderId='';
 await page.goto(base+'/#/pages/services/detail?id='+state.service_id,{waitUntil:'networkidle'});
 await step('选择服务','玩家先查看服务详情，确认价格、介绍和服务方。',async()=>{await ready('.detail-title');await page.mouse.wheel(0,320);await page.waitForTimeout(500)});
 await step('写清需求','填写区服、预约时间和验收标准，双方更容易对齐预期。',async()=>{await type(page.locator('.order-form textarea'),'今晚八点，三角洲双人陪玩一小时，按约定集合。')});
 await step('创建订单','核对金额后确认下单，系统会生成一笔待付款订单。',async()=>{await click(page.locator('.checkout .btn'));await ready('.order-heading');orderId=new URL(page.url()).hash.split('id=')[1];assert.ok(orderId)});
 await step('选择支付方式','订单内选择支付方式。这次演示使用本地测试支付。',async()=>{await click(page.locator('.dock-primary'));await ready('.payment-options');assert.ok((await page.locator('.dev-warning').innerText()).includes('本地测试'));await page.waitForTimeout(500)});
 await step('完成测试支付','确认支付后，订单进入待接单状态，正式交易以微信结果为准。',async()=>{await click(page.locator('.modal-actions .btn').filter({hasText:'确认支付订单'}));await page.waitForTimeout(800);assert.equal((await read('player','/orders/'+orderId)).status,'open')});
 await step('订单内沟通','打开订单聊天，把集合时间和服务需求留在订单里。',async()=>{await click(page.locator('.chat-entry'));await ready('.composer-input');await sendMessage('你好，今晚八点集合，按订单需求开始。')});
 await identity('worker','/pages/orders/detail?id='+orderId);
 await step('接单者接受订单','切到接单者视角，核对需求后，点击接受订单。',async()=>{await ready('.dock-primary');await orderAction('接受订单')});
 await step('开始服务','双方确认后，点击开始服务，订单进度同步更新。',async()=>{await orderAction('开始服务')});
 await step('同步履约进度','服务过程中在订单聊天同步进度，方便双方核对。',async()=>{await click(page.locator('.chat-entry'));await ready('.composer-input');await sendMessage('已确认需求，按约定提供一小时服务。')});
 await back();
 await step('提交交付说明','完成服务后提交交付，写清完成内容，等待玩家验收。',async()=>{await orderAction('提交交付','已按约定完成一小时陪玩，请核对本次服务。')});
 await identity('player','/pages/orders/detail?id='+orderId);
 await step('玩家核对并验收','玩家核对交付说明，确认符合约定后，验收并完成。',async()=>{await ready('.dock-primary');await orderAction('验收并完成');assert.equal((await read('player','/orders/'+orderId)).status,'completed')});
 await identity('worker','/pages/account/wallet');
 await step('查看待结算收入','服务收入先进入结算中，保留七天售后期，再按规则转为可提现。',async()=>{await ready('.balance-value');await page.waitForTimeout(500);const wallet=await read('worker','/wallet');assert.ok(typeof wallet.settling_cents==='number'&&wallet.settling_cents>0)});
 await finish()}

/** 主流程先检验环境和数据，再按选定教程执行并输出可复核证据。 */
async function run():Promise<void>{browser=await chromium.launch({headless:true,channel:'chrome'});const health=await(await fetch('http://127.0.0.1:18091/health')).json() as {data:{dev_mode:boolean}};assert.equal(health.data.dev_mode,true,'禁止在非开发环境制作教程');state=JSON.parse(await fs.readFile(path.join(root,'.run/tutorial-state.json'),'utf8'));for(const dir of ['raw','preflight','qa-source'])await fs.mkdir(path.join(output,dir),{recursive:true});const selected=process.env.TUTORIAL_IDS?.split(',');for(const [id,action] of [['01-platform-tutorial',admin],['02-club-tutorial',club],['03-order-tutorial',transaction]] as const)if(!selected||selected.includes(id))await action();assert.deepEqual(errors,[]);const report={mode:preflight?'preflight':'recording',local_api:api,requests,runtime_errors:errors,real_ui:true,production_writes:false};const reportFile=path.join(output,preflight?'preflight-report.json':'recording-report.json');if(preflight){await fs.writeFile(reportFile,JSON.stringify(report,null,2)+'\n')}else{let runs:Array<{ids:string[]}>=[];try{runs=JSON.parse(await fs.readFile(reportFile,'utf8')).runs||[]}catch{}await fs.writeFile(reportFile,JSON.stringify({mode:'recording',runs:[...runs.filter(run=>!run.ids.some(id=>tutorials.some(item=>item.id===id))),{...report,ids:tutorials.map(item=>item.id)}]},null,2)+'\n')}if(!preflight){let previous:Tutorial[]=[];try{previous=JSON.parse(await fs.readFile(path.join(output,'manifest.json'),'utf8')).tutorials}catch{}await fs.writeFile(path.join(output,'manifest.json'),JSON.stringify({viewport:{width:393,height:759},data:'隔离本地环境真实操作，模拟支付，不产生真实扣费',tutorials:[...previous.filter(item=>!tutorials.some(next=>next.id===item.id)),...tutorials].sort((a,b)=>a.id.localeCompare(b.id))},null,2)+'\n')}}
/** 错误保留实际页面证据并释放浏览器，便于修复后重新预检。 */
async function main():Promise<void>{try{await run()}catch(error){if(page&&!page.isClosed()){await fs.mkdir(path.join(output,'debug'),{recursive:true});await page.screenshot({path:path.join(output,'debug/failure.png')});await fs.writeFile(path.join(output,'debug/failure.txt'),await page.locator('body').innerText())}throw error}finally{await browser?.close()}}
main().catch(error=>{console.error(error);process.exitCode=1})
