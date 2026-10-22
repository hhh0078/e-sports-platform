/** 文件作用：通过微信官方SDK验收管理员AI配置、系统消息读后删除与本地保留、身份隔离和审核失败草稿；仅连接本地开发服务。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement, TextareaElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { AISettings } from '../frontend/src/types/ai'
import type { AuthSession } from '../frontend/src/types/models'
import type { MessageInbox } from '../frontend/src/types/messages'

const root=process.cwd(),api=process.env.UI_TEST_API||'http://127.0.0.1:18091/api',output=process.env.UI_TEST_OUTPUT||path.join(root,'output/game-ui-wallet/ai-messages-390')
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
let mini:MiniProgram|undefined,viewport:{windowWidth:number;windowHeight:number}|undefined,restorePrompt:{token:string;prompt:string}|undefined,modalMocked=false
const checks:string[]=[],exceptions:string[]=[],canvasWarnings:string[]=[]
/** 仅使用官方路由和窗口能力，不写入会话或调用Vue内部方法。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number};navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void}
/** 获取唯一连接，禁止与其它SDK脚本并行执行。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 等待实际页面条件并保留超时步骤。 */
async function until<T>(read:()=>Promise<T|false|null|undefined>,label:string,timeout=15000):Promise<T>{const started=Date.now();while(Date.now()-started<timeout){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,150))}throw new Error('等待超时：'+label)}
/** 去掉图标字体和空白再比较实际按钮标题。 */
function plain(text:string):string{return text.replace(/[\uE000-\uF8FF]/g,'').trim()}
/** 找到实际按钮，不依赖编译后的内部变量。 */
async function button(page:Page,label:string):Promise<Element>{return until(async()=>{for(const item of await page.$$('button') as Element[])if(plain(await item.text())===label)return item;return null},'按钮 '+label)}
/** 使用原生路由切换并等待页面实例改变。 */
async function open(url:string,restart=false):Promise<Page>{console.log('CHECK 页面 '+url);const previous=await sdk().currentPage();await sdk().evaluate((target:string,reset:boolean)=>{if(reset)setTimeout(()=>wx.reLaunch({url:target}),30);else wx.navigateTo({url:target});return true},url,restart);return until(async()=>{const current=await sdk().currentPage();return current?.path===url.split('?')[0].slice(1)&&(!restart||current!==previous)?current:null},url)}
/** 用独立后端会话核对数据，不向小程序注入token或伪造资料。 */
async function call<T>(route:string,token='',method='GET',body?:unknown):Promise<T>{const response=await fetch(api+route,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(body===undefined?{}:{body:JSON.stringify(body)})});const envelope=await response.json() as {data:T;error?:string};assert.ok(response.ok,route+' '+response.status+' '+(envelope.error||''));return envelope.data}
/** 通过可见协议和体验身份按钮登录。 */
async function login(name:string){console.log('CHECK 登录 '+name);const page=await open('/pages/account/login',true);await(await until(()=>page.$('.consent-line label'),'协议勾选')).tap();await(await button(page,'开发体验')).tap();await(await until(async()=>{for(const item of await page.$$('.account-card') as Element[])if((await item.text()).includes(name))return item;return null},'体验账号 '+name)).tap();await until(async()=>{if((await sdk().currentPage())?.path==='pages/main/index')return true;const later=await page.$('.finish-later');if(later){await later.tap();return true}return null},'跳过可选资料');await until(async()=>(await sdk().currentPage())?.path==='pages/main/index','登录完成')}
/** 取得真实输入节点。 */
async function field(page:Page,selector:string):Promise<InputElement|TextareaElement>{const item=await until(()=>page.$(selector),'输入 '+selector);assert.ok('input' in item);return item as InputElement|TextareaElement}
/** 等待页面过渡后保存实际模拟器截图。 */
async function record(label:string,name?:string){checks.push(label);if(name){await new Promise(resolve=>setTimeout(resolve,450));await sdk().screenshot({path:path.join(output,name+'.png')})}console.log('PASS '+label)}
/** 检查真实元素不会横向撑出手机窗口。 */
async function within(page:Page,selector:string){for(const item of await page.$$(selector) as Element[]){const [offset,size]=await Promise.all([item.offset(),item.size()]);assert.ok(Number(offset.left)>=-1&&Number(offset.left)+Number(size.width)<=Number(viewport?.windowWidth)+1,selector+'横向溢出')}}
/** 等待用户看得到的指定消息。 */
async function messageRow(page:Page,title:string){return until(async()=>{for(const item of await page.$$('.message-row') as Element[])if((await item.text()).includes(title))return item;return null},'系统消息 '+title)}
/** 验证完整图文与通知链路，不调用真实AI或商户资金。 */
async function run(){
  assert.match(api,/^http:\/\/(127\.0\.0\.1|localhost):18091\/api$/)
  assert.equal((await call<{dev_mode:boolean}>('/bootstrap')).dev_mode,true)
  const admin=await call<AuthSession>('/auth/dev','','POST',{user_id:'admin'}),player=await call<AuthSession>('/auth/dev','','POST',{user_id:'player'})
  const original=await call<AISettings>('/admin/ai-settings',admin.token);assert.equal(original.has_api_key,false,'本地验收要求无真实AI密钥');assert.equal(original.mode,'local_simulation')
  fs.mkdirSync(output,{recursive:true});mini=await automator.connect({wsEndpoint:process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9420'});mini.on('exception',(entry:unknown)=>exceptions.push(String(entry)));mini.on('console',(entry:unknown)=>{if(/不支持颜色|unsupported color/i.test(JSON.stringify(entry)))canvasWarnings.push('颜色兼容警告')})
  viewport=await mini.evaluate(()=>wx.getWindowInfo()) as {windowWidth:number;windowHeight:number};if(process.env.UI_EXPECT_WIDTH)assert.equal(viewport.windowWidth,Number(process.env.UI_EXPECT_WIDTH))
  await login('集结管理员');let page=await open('/pages/manage/index');await until(async()=>(await page.$$('.module-card')).length===12,'十二个模块')
  await(await button(page,'AI 设置')).tap();page=await until(async()=>{const current=await sdk().currentPage();return current?.path==='pages/manage/ai'?current:null},'AI独立配置')
  const base=await field(page,'.ai-base-url'),model=await field(page,'.ai-model'),key=await field(page,'.ai-key'),prompt=await field(page,'.ai-prompt')
  await until(async()=>(await base.property('value'))===original.base_url,'配置默认地址');assert.equal(await model.property('value'),original.model);assert.equal(await key.property('value'),'');await within(page,'input');await within(page,'textarea');await record('AI独立配置默认地址模型正确，密钥不回显，窄屏输入不溢出','ai-settings')
  const changed=original.prompt+'\n连接验收：继续按照平台规则审核。';restorePrompt={token:admin.token,prompt:original.prompt};await prompt.input(changed);await(await button(page,'保存AI配置')).tap();await until(async()=>(await call<AISettings>('/admin/ai-settings',admin.token)).prompt===changed,'真实保存提示词');assert.equal(await key.property('value'),'');await record('管理员可真实保存提示词，密钥字段保持空白')
  await call('/admin/ai-settings',admin.token,'PATCH',{prompt:original.prompt});restorePrompt=undefined
  const title='消息验收 '+Date.now(),body='这是一条仅用于本地验收的系统通知。\n打开后应在本机保留全文，服务器副本删除。'
  page=await open('/pages/manage/messages');await(await field(page,'.notice-recipient')).input('player');await(await field(page,'.notice-title')).input(title);await(await field(page,'.notice-body')).input(body);await sdk().mockWxMethod('showModal',{confirm:true,cancel:false});modalMocked=true;await(await button(page,'发送消息')).tap();await until(async()=>(await call<MessageInbox>('/messages?limit=100',player.token)).items.some(item=>item.title===title),'消息真实投递');await sdk().restoreWxMethod('showModal');modalMocked=false;await within(page,'input');await record('管理员发送指定账号系统通知成功','admin-message')
  await login('云间旅人');page=await open('/pages/account/messages/index');let row=await messageRow(page,title);assert.ok(await row.$('.unread-dot'));await within(page,'.message-row');const inbox=await call<MessageInbox>('/messages?limit=100',player.token),remote=inbox.items.find(item=>item.title===title);assert.ok(remote);assert.ok(!(await row.text()).includes('这是一条'),'列表只能显示标题');await record('列表仅标题，新消息未读且服务器副本仍在','message-unread')
  await row.tap();page=await until(async()=>{const current=await sdk().currentPage();return current?.path==='pages/account/messages/detail'?current:null},'消息详情');await until(async()=>{const content=await page.$('.message-body');return content&&(await content.text())===body},'消息完整正文');await until(async()=>!(await call<MessageInbox>('/messages?limit=100',player.token)).items.some(item=>item.id===remote.id),'读后删除本人服务器副本');await within(page,'.message-body');await record('打开详情后全文可读，服务器副本已删除','message-detail')
  page=await open('/pages/account/messages/index',true);row=await messageRow(page,title);assert.equal(await row.$('.unread-dot'),null);await row.tap();page=await until(async()=>{const current=await sdk().currentPage();return current?.path==='pages/account/messages/detail'?current:null},'本机历史详情');await until(async()=>{const content=await page.$('.message-body');return content&&(await content.text())===body},'重启页面保留全文');await record('原生重启页面后已读和全文仍保存在本机，不依赖服务器')
  await login('自由玩家');page=await open('/pages/account/messages/index');await until(async()=>!await page.$('.loading-line'),'其它账号读取完成');for(const item of await page.$$('.message-row') as Element[])assert.ok(!(await item.text()).includes(title));await record('切换账号不显示前一账号消息')
  for(const[name,slug]of [['北辰','worker'],['山海主理人','clubowner'],['云间旅人','player']]){await login(name);for(const route of ['ai','messages']){page=await open('/pages/manage/'+route);await until(async()=>{const empty=await page.$('empty-state')||await page.$('components\\/EmptyState');return empty&&(await empty.text()).includes('仅平台管理员')},'非平台管理员权限提示');assert.equal((await page.$$('input')).length,0);await record(slug+'无AI配置或后台发送权限：'+route)}}
  page=await open('/pages/requests/publish');const fields=await page.$$('input') as InputElement[];assert.equal(fields.length,2);await fields[0].input('[审核拒绝测试]三角洲任务陪练');await(await field(page,'textarea')).input('按约定时段完成正常游戏教学，交付后核对服务记录。');await fields[1].input('20');await(await button(page,'确认需求，去支付')).tap();await until(async()=>{const error=await page.$('.error-box');return error&&(await error.text()).includes('审核未通过')},'审核拒绝原因');assert.equal(await fields[0].property('value'),'[审核拒绝测试]三角洲任务陪练');assert.equal(await fields[1].property('value'),'20');assert.equal((await sdk().currentPage())?.path,'pages/requests/publish');await record('真实请求审核拒绝后留在发布页，完整原因与预算草稿仍在','moderation-draft')
  assert.deepEqual(exceptions,[]);assert.deepEqual(canvasWarnings,[])
  fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed:true,viewport,checks,runtime_exceptions:exceptions,canvas_color_warnings:canvasWarnings,native_confirmation:'重复回归使用官方SDK模拟确认；390首次另经原生窗口手动确认',remaining:['真实AI密钥和图像模型能力未配置；本脚本使用明确本地模拟。','真实微信手机号、支付和商家转账另行验收。']},null,2)+'\n')
}
run().catch(error=>{fs.mkdirSync(output,{recursive:true});fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed:false,viewport,checks,error:error instanceof Error?error.message:String(error),runtime_exceptions:exceptions},null,2)+'\n');console.error(error);process.exitCode=1}).finally(async()=>{if(restorePrompt)try{await call('/admin/ai-settings',restorePrompt.token,'PATCH',{prompt:restorePrompt.prompt})}catch{console.error('本地测试提示词恢复失败，请核对AI配置')}if(modalMocked)await mini?.restoreWxMethod('showModal');await mini?.disconnect()})
