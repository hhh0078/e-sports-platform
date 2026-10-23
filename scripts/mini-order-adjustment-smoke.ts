/** 文件作用：仅在本地开发环境通过官方微信SDK验看既有调价订单的四身份权限、费用记录与弹层，不触发支付或修改订单。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {createRequire} from 'node:module'
import {execFileSync} from 'node:child_process'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {InputElement,TextareaElement} from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {User} from '../frontend/src/types/models'

const root=process.cwd(),api='http://127.0.0.1:18091/api',endpoint=process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9423',output=path.resolve(process.env.UI_TEST_OUTPUT||'output/order-adjustment-sharing/order-390'),role=process.env.UI_ORDER_ROLE||'all'
const fixture=JSON.parse(fs.readFileSync(path.join(root,'output/order-adjustment-sharing/api-smoke/report.json'),'utf8')) as {order_id:string;title:string;passed:boolean;cleanup_required:boolean}
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
const checks:string[]=[],exceptions:string[]=[],warnings:string[]=[],rectangles:unknown[]=[]
let mini:MiniProgram|undefined,stage='初始化',width=0
/** 官方同步尺寸接口通过evaluate读取，不能作为依赖success回调的异步API调用。 */
declare const wx:{getWindowInfo():{windowWidth:number}}
/** 等待真实节点就绪，超时记录当前步骤，不重复资金动作。 */
async function until<T>(read:()=>Promise<T|null|false|undefined>,label:string):Promise<T>{const deadline=Date.now()+15000;while(Date.now()<deadline){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,150))}throw new Error('等待超时：'+label)}
/** 只使用已建立的官方SDK实例。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 复用官方路由的WebView接续等待，避免继续操作旧页面。 */
async function open(url:string):Promise<Page>{stage='打开 '+url;console.log(stage);const page=await sdk().reLaunch(url);assert.ok(page);return page}
/** 从稳定范围读取按钮，不访问Vue业务方法或注入缓存。 */
async function button(host:Page|Element,selector:string,label:string):Promise<Element>{return until(async()=>{for(const element of await host.$$(selector) as Element[])if((await element.text()).replace(/[\uE000-\uF8FF]/g,'').trim()===label)return element;return null},label)}
/** 用真实勾选协议和本地体验身份登录，跳过可选资料不修改账号。 */
async function login(user:User):Promise<void>{const page=await open('/pages/account/login');await until(()=>page.$('.consent-checkbox-label'),'登录协议');await(await page.$('.consent-checkbox-label'))!.tap();await(await button(page,'.dev-toggle','开发体验')).tap();const account=await until(async()=>{for(const item of await page.$$('.account-card') as Element[])if((await item.text()).includes(user.nickname))return item;return null},'本地体验身份');await account.tap();await until(()=>page.$('.finish-later'),'可选资料弹层');await(await page.$('.finish-later'))!.tap();await until(async()=>(await sdk().currentPage())?.path==='pages/main/index','登录回跳')}
/** 检查真实矩形在窗口内，返回可读的失败选择器。 */
async function within(host:Page|Element,selector:string):Promise<void>{for(const item of await host.$$(selector) as Element[]){const[o,s]=await Promise.all([item.offset(),item.size()]);if(!Number(s.width)||!Number(s.height))continue;rectangles.push({selector,...o,...s});assert.ok(Number(o.left)>=-1&&Number(o.left)+Number(s.width)<=width+1,selector+'横向溢出')}}
/** 使用官方唯一截图工具保存实际模拟器画面，不保存整个开发者工具或私有配置。 */
function screenshot(name:string):void{execFileSync('/Applications/wechatwebdevtools.app/Contents/MacOS/wechatide',['-c','Codex','simulator_screenshot','--project',path.join(root,'frontend/dist/build/mp-weixin'),'--path',path.join(output,name+'.png'),'--optimize','false'],{stdio:'pipe',timeout:15000})}
/** 验看一类身份的金额、全部协商证据和只读/可操作权限，打开创建弹层后仅取消。 */
async function inspect(user:User):Promise<void>{
  await login(user);const page=await open('/pages/orders/detail?id='+fixture.order_id)
  await until(()=>page.$('.order-title'),'订单金额');assert.equal(await(await page.$('.order-title'))!.text(),fixture.title)
  const component=await until(()=>page.$('components\\/OrderAdjustments'),'费用协商组件')
  await until(async()=>(await component.$$('.adjustment-record') as Element[]).length===3,'三次已完成的调价记录')
  const totals=await(await component.$('.adjustment-totals'))!.text();for(const value of ['1.01','0.71','1.3'])assert.ok(totals.includes(value),'费用汇总缺少'+value)
  for(const row of await component.$$('.adjustment-record') as Element[])assert.ok((await row.text()).includes('已完成'))
  const actions=await component.$$('.adjustment-new-button') as Element[];assert.equal(actions.length,user.id==='worker'?2:user.id==='player'?1:0,'交易双方与管理只读权限')
  const area=await(await component.$('.adjustment-heading'))!.offset();await sdk().pageScrollTo(Math.max(0,Number(area.top)-130));await within(component,'.adjustment-totals');await within(component,'.adjustment-record');screenshot(user.id+'-records')
  if(actions.length){
    await actions[0].tap();await until(()=>component.$('.adjustment-sheet'),'协商弹层')
    const amount=await until(()=>component.$('.adjustment-amount-input'),'差额输入框') as InputElement,reason=await until(()=>component.$('.adjustment-reason-input'),'协商原因输入框') as TextareaElement
    await amount.input('0.01');await reason.input('仅检查长原因排版\n这是第二行说明，不提交也不扣款。'.repeat(5))
    await within(component,'.adjustment-sheet');await within(component,'.adjustment-confirm');screenshot(user.id+'-draft');await(await button(component,'.text-btn','取消')).tap()
  }
  checks.push(user.id+' 费用守恒、三次证据、权限与长原因弹层');console.log('PASS '+checks[checks.length-1])
}
/** 保存真实通过项与具体失败步骤，不能把局部失败改写为整套通过。 */
function report(passed:boolean,error?:unknown):void{fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({passed,stage,width,checks,rectangles,runtime_exceptions:exceptions,canvas_color_warnings:warnings,...(error?{error:error instanceof Error?error.message:String(error)}:{}),boundaries:['仅本地dev订单，只读费用记录与取消表单；补款退款由独立真实本机API流程和资金集成测试验证，未调用真实微信资金。']},null,2)+'\n')}
/** 串行验看四身份，连接与页面销毁均由同一SDK管理。 */
async function main():Promise<void>{assert.match(endpoint,/^ws:\/\/127\.0\.0\.1:942[23]$/);assert.ok(fixture.passed&&fixture.cleanup_required);assert.match(fixture.title,/^调价验收/);const bootstrap=await(await fetch(api+'/bootstrap')).json();assert.equal(bootstrap.data.dev_mode,true);const accounts=(await(await fetch(api+'/dev/accounts')).json()).data as User[];fs.mkdirSync(output,{recursive:true});mini=await automator.connect({wsEndpoint:endpoint});mini.on('exception',(error:unknown)=>exceptions.push(String(error)));mini.on('console',(entry:unknown)=>{if(/不支持颜色|unsupported color/i.test(JSON.stringify(entry)))warnings.push('微信颜色警告')});width=Number((await sdk().evaluate(()=>wx.getWindowInfo()) as {windowWidth:number}).windowWidth);assert.equal(width,Number(process.env.UI_EXPECT_WIDTH||390));for(const id of ['worker','player','admin','clubowner'].filter(id=>role==='all'||role===id)){const user=accounts.find(item=>item.id===id);assert.ok(user);await inspect(user)}assert.deepEqual(exceptions,[]);assert.deepEqual(warnings,[]);report(true)}
void main().catch(error=>{report(false,error);console.error(error);process.exitCode=1}).finally(async()=>{await mini?.disconnect()})
