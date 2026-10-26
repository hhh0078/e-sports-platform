/** 文件作用：从当前本地H5页面采集宣传截图；演示接口仅在隔离浏览器中替换，禁止向业务服务发送写请求。 */
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import {createRequire} from 'node:module'
import type {Page, Route} from 'playwright'
import {buildMarketingFixtures,resolveMarketingResponse} from './marketing-fixtures'

const root=process.cwd(),output=path.join(root,'output/playwright/marketing-20261026')
const modules=process.env.MARKETING_NODE_MODULES||path.join(os.homedir(),'.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules')
const runtime=createRequire(import.meta.url),{chromium}=runtime(path.join(modules,'playwright')) as typeof import('playwright')
const base='http://127.0.0.1:5173',api='http://127.0.0.1:18091/api'
/** 单屏使用当前产品路由；取景调整仅影响截图位置，不改变产品样式。 */
interface Screen {id:string;label:string;route:string;ready:string;scroll?:string;scrollY?:number}
/** 功能组合图与对应三屏。 */
interface Group {id:string;title:string;subtitle:string;screens:Screen[]}
const groups:Group[]=[
 {id:'01-admin',title:'管理员后台',subtitle:'经营概览、用户管理与订单售后，集中处理平台事务',screens:[
  {id:'admin-console',label:'经营概览与待办',route:'/pages/manage/index',ready:'.metric-money'},
  {id:'admin-users',label:'用户与权限管理',route:'/pages/manage/users',ready:'.user-row'},
  {id:'admin-orders',label:'订单与售后管理',route:'/pages/manage/orders',ready:'.manage-search-input'}]},
 {id:'02-content',title:'内容运营',subtitle:'轮播、公告与文章管理，运营内容独立配置',screens:[
  {id:'content-center',label:'轮播与公告配置',route:'/pages/manage/content/index',ready:'.content-tabs'},
  {id:'content-banner',label:'图片与文字分层',route:'/pages/manage/content/banner?id=marketing-banner-01',ready:'.banner-label-editor',scrollY:40},
  {id:'content-articles',label:'图文文章管理',route:'/pages/manage/articles/index',ready:'.article-list-row'}]},
 {id:'03-services',title:'多游戏服务',subtitle:'展示服务、查看详情与编辑发布，覆盖日常接单场景',screens:[
  {id:'service-home',label:'多游戏服务展示',route:'/pages/main/index?tab=player',ready:'.service-card'},
  {id:'service-detail',label:'服务详情与价格',route:'/pages/services/detail?id=marketing-service-01',ready:'.detail-title'},
  {id:'service-publish',label:'封面与服务配置',route:'/pages/services/publish?id=marketing-service-01',ready:'.label-editor'}]},
 {id:'04-orders',title:'订单履约与沟通',subtitle:'订单进度、交易详情与独立聊天，协作过程清楚可查',screens:[
  {id:'order-list',label:'订单进度一目了然',route:'/pages/main/index?tab=orders',ready:'.order-card'},
  {id:'order-detail',label:'交易约定与履约',route:'/pages/orders/detail?id=marketing-order-01',ready:'.order-heading'},
  {id:'order-chat',label:'订单独立聊天室',route:'/pages/orders/chat?id=marketing-order-01',ready:'.message'}]},
 {id:'05-clubs',title:'俱乐部体系',subtitle:'公开主页、管理工作台与团队资金，组织协作更清晰',screens:[
  {id:'club-home',label:'俱乐部品牌主页',route:'/pages/community/clubs/detail?id=marketing-club-01',ready:'.club-title'},
  {id:'club-console',label:'俱乐部管理工作台',route:'/pages/community/clubs/console?id=marketing-club-01',ready:'.club-console-title'},
  {id:'club-funds',label:'佣金与押金分账',route:'/pages/community/clubs/funds?id=marketing-club-01',ready:'.funds-value'}]},
 {id:'06-wallet',title:'个人中心与钱包',subtitle:'个人入口、余额分区与收支记录，资金状态分别展示',screens:[
  {id:'personal-center',label:'个人工作台',route:'/pages/main/index?tab=me',ready:'.profile-name'},
  {id:'wallet-overview',label:'余额与结算分区',route:'/pages/account/wallet',ready:'.balance-value'},
  {id:'wallet-ledger',label:'押金与收支明细',route:'/pages/account/wallet',ready:'.ledger-row',scroll:'.deposit-section'}]}
]
/** 读取本地公开展示资源，不获取管理员令牌或私密信息。 */
async function publicData(route:string):Promise<unknown>{const response=await fetch(api+route,{redirect:'error'});assert.ok(response.ok,route+'公开资源读取失败');const body=await response.json() as {data:unknown};return body.data}
/** 所有演示API都在浏览器拦截中完成；未声明接口拒绝回源。 */
async function fulfill(route:Route,fixtures:ReturnType<typeof buildMarketingFixtures>,unknown:Set<string>):Promise<void>{
 const request=route.request(),url=new URL(request.url())
 if(!['127.0.0.1','localhost'].includes(url.hostname)){await route.abort();return}
 if(!url.pathname.startsWith('/api/')){await route.continue();return}
 if(request.method()==='OPTIONS'){await route.fulfill({status:204,headers:{'Access-Control-Allow-Origin':'*','Access-Control-Allow-Headers':'*','Access-Control-Allow-Methods':'*'}});return}
 if(request.method()!=='GET'){unknown.add(request.method()+' '+url.pathname);await route.fulfill({status:405,json:{error:'宣传演示禁止写入'},headers:{'Access-Control-Allow-Origin':'*'}});return}
 const value=resolveMarketingResponse(url,fixtures)
 if(value===undefined){unknown.add('GET '+url.pathname);await route.fulfill({status:404,json:{error:'演示接口未配置'},headers:{'Access-Control-Allow-Origin':'*'}});return}
 await route.fulfill({status:200,json:{data:value},headers:{'Access-Control-Allow-Origin':'*'}})
}
/** 等待页面真实渲染和图片加载，失败保存取景信息，不把错误页作为宣传截图。 */
async function ready(page:Page,screen:Screen):Promise<void>{
 await page.locator(screen.ready).first().waitFor({state:'visible',timeout:20000})
 await page.waitForTimeout(650)
 await page.evaluate(async()=>{await document.fonts.ready;await Promise.all(Array.from(document.images).map(image=>image.complete?Promise.resolve():new Promise<void>(resolve=>{image.onload=()=>resolve();image.onerror=()=>resolve();setTimeout(resolve,4000)})))})
 const errors=await page.locator('.error-box:visible,.chat-error:visible,.manage-error:visible,.banner-error:visible').allTextContents()
 assert.equal(errors.length,0,screen.id+'存在错误：'+errors.join('；'))
 if(screen.scroll){await page.locator(screen.scroll).first().evaluate(element=>window.scrollTo(0,element.getBoundingClientRect().top+window.scrollY-58));await page.waitForTimeout(200)}
 if(screen.scrollY){await page.evaluate(value=>window.scrollTo(0,value),screen.scrollY);await page.waitForTimeout(200)}
 const width=await page.evaluate(()=>document.documentElement.scrollWidth);assert.ok(width<=394,screen.id+'横向溢出'+width)
}
/** 每屏使用隔离上下文及虚构令牌，只采集当前产品界面，不保存会话。 */
async function run():Promise<void>{
 assert.ok((await fs.readFile(path.join(root,'frontend/package.json'),'utf8')).includes('jijie-marketplace'))
 await fs.mkdir(path.join(output,'raw'),{recursive:true})
 const [bootstrap,services,clubs]=await Promise.all([publicData('/bootstrap'),publicData('/services?limit=100'),publicData('/clubs?limit=100')])
 assert.ok(Array.isArray(services)&&Array.isArray(clubs))
 const fixtures=buildMarketingFixtures(bootstrap,{users:[],services,clubs,orders:[]}),unknown=new Set<string>(),exceptions:string[]=[]
 const browser=await chromium.launch({headless:true,channel:'chrome'})
 const selected=process.env.MARKETING_GROUPS?.split(',')
 try{for(const group of groups.filter(group=>!selected||selected.includes(group.id))){for(const screen of group.screens){
  const context=await browser.newContext({viewport:{width:393,height:759},deviceScaleFactor:3,isMobile:true,hasTouch:true,locale:'zh-CN',timezoneId:'Asia/Shanghai',colorScheme:'dark'})
  await context.route('**/*',route=>fulfill(route,fixtures,unknown))
  await context.addInitScript(()=>{localStorage.setItem('jijie_token','marketing-demo-token');localStorage.setItem('jijie_game','delta')})
  const page=await context.newPage();page.on('pageerror',error=>exceptions.push(screen.id+': '+error.message))
  await page.goto(base+'/#'+screen.route,{waitUntil:'networkidle'})
  try{await ready(page,screen)}catch(error){await fs.writeFile(path.join(output,screen.id+'-debug.txt'),await page.locator('body').innerText());await page.screenshot({path:path.join(output,screen.id+'-debug.png')});throw error}
  await page.screenshot({path:path.join(output,'raw',screen.id+'.png'),animations:'disabled'})
  console.log('已截图 '+screen.id);await context.close()
 }}}
 finally{await browser.close()}
 const manifest={created_at:'2026-10-26',source:'当前本地H5页面，393×759 @3x；手机状态栏与边框由合成脚本添加',data:'全部私有接口仅返回浏览器内虚构数据；未写入业务数据库',groups:groups.map(group=>({...group,screens:group.screens.map(screen=>({...screen,file:'raw/'+screen.id+'.png'}))}))}
 await fs.writeFile(path.join(output,'manifest.json'),JSON.stringify(manifest,null,2)+'\n')
 await fs.writeFile(path.join(output,'capture-report.json'),JSON.stringify({captured_groups:selected||groups.map(group=>group.id),runtime_exceptions:exceptions,unconfigured_requests:[...unknown],viewport:{width:393,height:759,scale:3}},null,2)+'\n')
 assert.deepEqual(exceptions,[],'存在界面运行异常');assert.deepEqual([...unknown],[],'演示接口存在未配置请求')
}
run().catch(error=>{console.error(error);process.exitCode=1})
