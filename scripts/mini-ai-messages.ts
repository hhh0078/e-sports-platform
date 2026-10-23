/** 文件作用：通过微信官方SDK验收管理员AI配置、系统消息读后删除与本地保留、身份隔离和后置人工审核、拒绝修改入口；仅连接本地开发服务。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement, TextareaElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { AISettings } from '../frontend/src/types/ai'
import type { AuthSession } from '../frontend/src/types/models'
import type { MessageInbox } from '../frontend/src/types/messages'
import type { ModerationInbox } from '../frontend/src/types/moderation'

const root=process.cwd(),api=process.env.UI_TEST_API||'http://127.0.0.1:18091/api',output=process.env.UI_TEST_OUTPUT||path.join(root,'output/post-publish-moderation/mini-390')
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
let mini:MiniProgram|undefined,viewport:{windowWidth:number;windowHeight:number}|undefined,restorePrompt:{token:string;prompt:string}|undefined,modalMocked=false
const moderationOnly=process.env.UI_TEST_SCOPE==='moderation'
const checks:string[]=[],exceptions:string[]=[],canvasWarnings:string[]=[],versionChecks:string[]=[]
const runFile=promisify(execFile),project=path.resolve(process.env.UI_TEST_PROJECT||'frontend/dist/local/mp-weixin')
const SDK=(require('miniprogram-automator/out/MiniProgram') as {default:typeof MiniProgram}).default,originalCheck=SDK.prototype.checkVersion
/** 仅修复工具缺少版本元数据的兼容故障，仍核验实际基础库最低版本。 */
function compatibilityCheck(){SDK.prototype.checkVersion=async function(this:MiniProgram){try{await originalCheck.call(this)}catch(error){if(!(error instanceof TypeError)||!/split/.test(error.message))throw error;const info=await this.callWxMethod('getSystemInfoSync') as {SDKVersion?:string};assert.match(String(info.SDKVersion),/^\d+\.\d+\.\d+/);const [major,minor,patch]=String(info.SDKVersion).split('.').map(Number);assert.ok(major>2||major===2&&(minor>7||minor===7&&patch>=3));versionChecks.push('工具元数据缺失，已核验实际基础库'+info.SDKVersion)}}}
/** 仅使用官方路由和窗口能力，不写入会话或调用Vue内部方法。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number};navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void}
/** 获取唯一连接，禁止与其它SDK脚本并行执行。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 等待实际页面条件并保留超时步骤。 */
async function until<T>(read:()=>Promise<T|false|null|undefined>,label:string,timeout=15000):Promise<T>{const started=Date.now();while(Date.now()-started<timeout){let value:T|false|null|undefined;try{value=await read()}catch(error){if(!/rawPath|getPageMetaByWebviewId/.test(String(error)))throw error}if(value)return value;await new Promise(resolve=>setTimeout(resolve,150))}throw new Error('等待超时：'+label)}
/** 去掉图标字体和空白再比较实际按钮标题。 */
function plain(text:string):string{return text.replace(/[\uE000-\uF8FF]/g,'').trim()}
/** 找到实际按钮，不依赖编译后的内部变量。 */
async function button(page:Page,label:string):Promise<Element>{return until(async()=>{for(const item of await page.$$('button') as Element[])if(plain(await item.text())===label)return item;return null},'按钮 '+label)}
/** 使用原生路由切换并等待页面实例改变。 */
async function open(url:string,restart=false):Promise<Page>{console.log('CHECK 页面 '+url);try{const current=restart?await sdk().reLaunch(url):await sdk().navigateTo(url);assert.ok(current);assert.equal(current.path,url.split('?')[0].slice(1));return current}catch(error){if(!/rawPath|getPageMetaByWebviewId/.test(String(error)))throw error;versionChecks.push('导航已下达，仅读取实际页面直到工具元数据就绪，不重复导航');return until(async()=>{const current=await sdk().currentPage();return current?.path===url.split('?')[0].slice(1)?current:null},'导航元数据 '+url)}}
/** 用独立后端会话核对数据，不向小程序注入token或伪造资料。 */
async function call<T>(route:string,token='',method='GET',body?:unknown):Promise<T>{const response=await fetch(api+route,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(body===undefined?{}:{body:JSON.stringify(body)})});const envelope=await response.json() as {data:T;error?:string};assert.ok(response.ok,route+' '+response.status+' '+(envelope.error||''));return envelope.data}
/** 通过可见协议和体验身份按钮登录。 */
async function login(name:string){console.log('CHECK 登录 '+name);const page=await open('/pages/account/login',true);await(await until(()=>page.$('.consent-line label'),'协议勾选')).tap();await(await button(page,'开发体验')).tap();await(await until(async()=>{for(const item of await page.$$('.account-card') as Element[])if((await item.text()).includes(name))return item;return null},'体验账号 '+name)).tap();await until(async()=>{if((await sdk().currentPage())?.path==='pages/main/index')return true;const later=await page.$('.finish-later');if(later){await later.tap();return true}return null},'跳过可选资料');await until(async()=>(await sdk().currentPage())?.path==='pages/main/index','登录完成')}
/** 取得真实输入节点。 */
async function field(page:Page,selector:string):Promise<InputElement|TextareaElement>{const item=await until(()=>page.$(selector),'输入 '+selector);assert.ok('input' in item);return item as InputElement|TextareaElement}
/** 等待页面过渡后保存实际模拟器截图。 */
async function record(label:string,name?:string){checks.push(label);if(name){await new Promise(resolve=>setTimeout(resolve,450));await runFile('/Applications/wechatwebdevtools.app/Contents/MacOS/wechatide',['-c','Codex','simulator_screenshot','--project',project,'--path',path.join(output,name+'.png'),'--optimize','false'],{timeout:25000})}console.log('PASS '+label)}
/** 检查真实元素不会横向撑出手机窗口。 */
async function within(page:Page,selector:string){for(const item of await page.$$(selector) as Element[]){const [offset,size]=await Promise.all([item.offset(),item.size()]);assert.ok(Number(offset.left)>=-1&&Number(offset.left)+Number(size.width)<=Number(viewport?.windowWidth)+1,selector+'横向溢出')}}
/** 等待用户看得到的指定消息。 */
async function messageRow(page:Page,title:string){return until(async()=>{for(const item of await page.$$('.message-row') as Element[])if((await item.text()).includes(title))return item;return null},'系统消息 '+title)}
/** 验证完整图文与通知链路，不调用真实AI或商户资金。 */
async function run(){
  assert.match(api,/^http:\/\/(127\.0\.0\.1|localhost):18091\/api$/)
  const compiled=fs.readFileSync(path.join(project,'api/client.js'),'utf8');assert.ok(compiled.includes(api),'验收包必须使用与脚本相同的本地API');assert.ok(!compiled.includes('https://game.ai.58it.cn/api'),'禁止在正式API包运行写入式UI验收')
  assert.equal((await call<{dev_mode:boolean}>('/bootstrap')).dev_mode,true)
  const admin=await call<AuthSession>('/auth/dev','','POST',{user_id:'admin'}),player=await call<AuthSession>('/auth/dev','','POST',{user_id:'player'})
  const original=await call<AISettings>('/admin/ai-settings',admin.token);assert.equal(original.has_api_key,false,'本地验收要求无真实AI密钥');assert.equal(original.mode,'manual')
  fs.mkdirSync(output,{recursive:true});compatibilityCheck();mini=await automator.connect({wsEndpoint:process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9420'});mini.on('exception',(entry:unknown)=>exceptions.push(String(entry)));mini.on('console',(entry:unknown)=>{if(/不支持颜色|unsupported color/i.test(JSON.stringify(entry)))canvasWarnings.push('颜色兼容警告')})
  viewport=await mini.evaluate(()=>wx.getWindowInfo()) as {windowWidth:number;windowHeight:number};if(process.env.UI_EXPECT_WIDTH)assert.equal(viewport.windowWidth,Number(process.env.UI_EXPECT_WIDTH))
  let page:Page
  if(!moderationOnly){
  await login('开机电竞管理员');page=await open('/pages/manage/index');await until(async()=>(await page.$$('.module-card')).length===14,'十四个模块')
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
  }
  await login('云间旅人')
  const marker='后置人工审核UI '+Date.now(),reason='本地验收：此字段需要修改，不影响其它资料。'
  page=await open('/pages/account/profile');const bio=await field(page,'.bio-input');await until(async()=>(await bio.property('value'))!==undefined,'资料加载');await bio.input(marker);await(await button(page,'保存资料')).tap();await until(async()=>(await call<{bio:string}>('/me',player.token)).bio===marker,'未配置AI仍保存资料成功');await record('资料无需AI配置即可保存，等待人工审核','profile-saved')
  await login('云间旅人');assert.equal((await sdk().currentPage())?.path,'pages/main/index');await record('已有账号再次登录直接回跳，不重复要求头像昵称')
  let inboxTasks=await call<ModerationInbox>('/admin/moderation?status=manual_pending&limit=100',admin.token);const target=inboxTasks.items.find(item=>item.target_type==='users'&&item.target_id==='player'&&item.field_code==='bio'&&item.preview===marker);assert.ok(target,'资料字段缺少人工待审任务')
  await login('开机电竞管理员');page=await open('/pages/manage/moderation');await until(()=>page.$('.moderation-row'),'待审队列');await within(page,'.moderation-row');await within(page,'.moderation-metrics');await record('审核后台展示真实状态和待审列表','moderation-list')
  const taskRow=await until(async()=>{for(const item of await page.$$('.moderation-row') as Element[])if((await item.text()).includes(marker))return item;return null},'资料对应审核行');await taskRow.tap();await until(async()=>{const content=await page.$('.detail-content');return content&&(await content.text())===marker},'读取完整待审原文');await within(page,'.reason-input');await record('点击任务使用真实浮层，全文和拒绝原因输入在窗口内','moderation-detail')
  await(await field(page,'.reason-input')).input(reason);await sdk().mockWxMethod('showModal',{confirm:true,cancel:false});modalMocked=true;await(await button(page,'拒绝')).tap();await until(async()=>(await call<{bio:string}>('/me',player.token)).bio==='审核失败，请查看消息','人工拒绝替换公开字段');await sdk().restoreWxMethod('showModal');modalMocked=false;await record('管理员人工拒绝生效，未阻塞之前的正常发布','moderation-rejected')
  const rejectedInbox=await call<MessageInbox>('/messages?limit=100',player.token),notice=rejectedInbox.items.find(item=>item.body.includes(reason));assert.ok(notice);assert.equal(notice.action_path,'/pages/account/profile');await login('云间旅人');page=await open('/pages/account/messages/index');await(await messageRow(page,notice.title)).tap();page=await until(async()=>{const current=await sdk().currentPage();return current?.path==='pages/account/messages/detail'?current:null},'拒绝通知详情');await until(async()=>{const body=await page.$('.message-body');return body&&(await body.text()).includes(reason)},'完整拒绝原因');await until(async()=>!(await call<MessageInbox>('/messages?limit=100',player.token)).items.some(item=>item.id===notice.id),'拒绝通知读后删除服务器副本');await within(page,'.message-action');await record('拒绝通知有原因、字段及修改入口，本机保留服务器已读即删','moderation-message');await(await until(()=>page.$('.message-action'),'修改资料入口')).tap();await until(async()=>(await sdk().currentPage())?.path==='pages/account/profile','消息修改入口回到资料页');await record('拒绝消息可直接进入对应资料修改页')
  page=await open('/pages/community/clubs/index');await(await button(page,'创建')).tap();const name=await field(page,'.create-form input'),intro=await field(page,'.intro-input');await name.input('本地验收俱乐部');await intro.input('太短');await(await button(page,'保存草稿并继续')).tap();assert.equal(await intro.property('value'),'太短');assert.ok(await page.$('.create-form'));assert.match(await(await until(()=>page.$('.create-form'),'俱乐部表单')).text(),/10—1000/);await within(page,'.modal');await record('创建俱乐部明确显示字段限制，错误输入保留草稿','club-length-hints')
  for(const name of ['北辰','山海主理人','云间旅人']){await login(name);page=await open('/pages/manage/moderation');await until(async()=>{const empty=await page.$('empty-state')||await page.$('components\\/EmptyState');return empty&&(await empty.text()).includes('仅平台管理员')},'审核后台拒绝非管理员');assert.equal((await page.$$('textarea')).length,0);await record(name+'无法查看审核原文或执行人工决定')}
  page=await open('/pages/account/profile');await sdk().pageScrollTo(100000);await(await until(()=>page.$('.logout-button'),'退出本地验收身份')).tap();await until(async()=>(await sdk().currentPage())?.path==='pages/main/index','恢复访客状态');await record('测试后通过真实退出入口恢复访客，不残留管理员会话')
  assert.deepEqual(exceptions,[]);assert.deepEqual(canvasWarnings,[])
  fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed:true,scope:moderationOnly?'moderation':'full',viewport,checks,version_checks:versionChecks,runtime_exceptions:exceptions,canvas_color_warnings:canvasWarnings,native_confirmation:'本轮发送消息与人工拒绝使用官方SDK模拟确认；未执行真实支付',remaining:['真实AI密钥和图像模型能力未配置；本脚本未调用真实模型，使用真实人工待审与处理接口。','真实微信手机号、支付和商家转账另行验收。']},null,2)+'\n')
}
run().catch(error=>{fs.mkdirSync(output,{recursive:true});fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed:false,scope:moderationOnly?'moderation':'full',viewport,checks,error:error instanceof Error?error.message:String(error),runtime_exceptions:exceptions},null,2)+'\n');console.error(error);process.exitCode=1}).finally(async()=>{if(restorePrompt)try{await call('/admin/ai-settings',restorePrompt.token,'PATCH',{prompt:restorePrompt.prompt})}catch{console.error('本地测试提示词恢复失败，请核对AI配置')}if(modalMocked)await mini?.restoreWxMethod('showModal');SDK.prototype.checkVersion=originalCheck;await mini?.disconnect()})
