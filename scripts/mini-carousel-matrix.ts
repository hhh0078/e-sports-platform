/** 文件作用：以本地微信官方SDK检查三图、六条最长文字与九种位置的首页轮播；只临时修改运营轮播及公告开关，结束恢复并撤销独立会话。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {createRequire} from 'node:module'
import {randomUUID} from 'node:crypto'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {SwiperElement} from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {AdminPublicContent,ContentSlide,PublicContent,PublicContentPatch} from '../frontend/src/types/content'
import type {Game,User} from '../frontend/src/types/models'
import {POSITIONS} from '../frontend/src/lib/cover'

/** evaluate只读取官方窗口信息，禁止读取页面data、存储、令牌或Vue实例。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number}}
/** 官方页面和组件查询的公开边界。 */
type Scope=Page|Element
/** 所有坐标均为官方SDK测得的实际逻辑像素。 */
interface Rect{left:number;top:number;width:number;height:number}
/** 单条可见文字的真实内容、尺寸、字号和层级样式。 */
interface LabelMeasurement{index:number;text:string;rect:Rect;font_size:number;pointer_events:string;visibility:string;opacity:string}
/** 每张轮播保存同屏真实矩形，不用声明尺寸代替实际布局。 */
interface Measurement{position:string;slide:number;hero:Rect;brand:Rect;publish:Rect;dots:Rect;games:Rect;labels:LabelMeasurement[];sample_ms:number;screenshot:string}
const root=process.cwd(),api='http://127.0.0.1:18091/api',width=Number(process.env.UI_EXPECT_WIDTH||390)
const project=path.resolve(process.env.UI_TEST_PROJECT||'frontend/dist/local/mp-weixin'),endpoint=process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9423'
const output=path.join(root,'output/carousel-matrix/mini-'+width),marker='carousel-sdk-'+randomUUID().slice(0,8)
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
const phrases=['游戏陪练任务完成一起开黑','WWWWWWWWWWW1','认真沟通准时上线放心开玩','MMMMMMMMMMM2','中文文案极限长度十二个字','WWWWMMMMWWM3']
const aliases:Record<string,string>={'player-view':'pages/main/player/player','content-carousel':'components/ContentCarousel'}
const measurements:Measurement[]=[],checks:string[]=[],exceptions:string[]=[],warnings:string[]=[],cleanupErrors:string[]=[]
const packageStamp=fs.statSync(path.join(project,'api/client.js')).mtimeMs
let mini:MiniProgram|undefined,token='',original:AdminPublicContent|undefined,changed=false,failure:unknown,stage='本地守卫',complete=false

/** 隐去会话及身份秘密，不把配置快照或完整接口响应写入报告。 */
function redact(value:unknown):string{return String(value instanceof Error?value.message:value).replace(/Bearer\s+[^\s,}"']+/gi,'Bearer [REDACTED]').replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi,'$1=[REDACTED]').slice(0,1600)}
/** 仅返回本轮唯一官方SDK连接，不自动打开其它工程。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 固定本地origin且禁止重定向，准备会话仅在Node进程内使用。 */
async function local<T>(route:string,method='GET',body?:unknown,authenticated=false):Promise<T>{
  assert.ok(route.startsWith('/')&&!route.startsWith('//'))
  const url=new URL(api+route);assert.equal(url.origin,'http://127.0.0.1:18091')
  const headers:Record<string,string>={'Content-Type':'application/json'}
  if(authenticated){assert.ok(token,'独立准备会话不存在');headers.Authorization='Bearer '+token}
  const response=await fetch(url,{method,headers,redirect:'error',signal:AbortSignal.timeout(15000),...(body===undefined?{}:{body:JSON.stringify(body)})})
  assert.equal(response.status,200,'本地接口 '+method+' '+route)
  return(await response.json() as {data:T}).data
}
/** 等待只读真实条件；写入失败不自动重试，不调用私有业务方法。 */
async function until<T>(read:()=>Promise<T|false|null|undefined>,label:string,timeout=18000):Promise<T>{const start=Date.now();while(Date.now()-start<timeout){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,100))}throw new Error('等待超时：'+label)}
/** 兼容官方SDK的短组件标签与当前编译路径。 */
async function children(scope:Scope,name:string):Promise<Element[]>{for(const alias of [name,aliases[name]?.replaceAll('/','\\/')]){if(!alias)continue;const nodes=await scope.$$(alias) as Element[];if(nodes.length)return nodes}return[]}
/** 查找实际节点，仅穿过本矩阵已知的两个公开组件。 */
async function find(scope:Scope,selector:string,depth=0):Promise<Element|null>{const direct=await scope.$(selector);if(direct)return direct;if(depth===3)return null;for(const name of Object.keys(aliases))for(const child of await children(scope,name)){const node=await find(child,selector,depth+1);if(node)return node}return null}
/** 读取公开实际外框，并防止SDK无效值混入几何检查。 */
async function rect(node:Element):Promise<Rect>{const[position,size]=await Promise.all([node.offset(),node.size()]);const box={left:Number(position.left),top:Number(position.top),width:Number(size.width),height:Number(size.height)};assert.ok(Object.values(box).every(Number.isFinite),'实际矩形必须为有限数值');return box}
/** 允许一像素排版舍入，内容仍须完整位于轮播原图区域。 */
function inside(child:Rect,parent:Rect,label:string):void{assert.ok(child.width>0&&child.height>0,label+'没有实际尺寸');assert.ok(child.left>=parent.left-1&&child.top>=parent.top-1&&child.left+child.width<=parent.left+parent.width+1&&child.top+child.height<=parent.top+parent.height+1,label+'越出轮播图')}
/** 检查实际矩形间没有正面积交叠，保留一像素舍入容差。 */
function separate(a:Rect,b:Rect):boolean{return a.left+a.width<=b.left+1||b.left+b.width<=a.left+1||a.top+a.height<=b.top+1||b.top+b.height<=a.top+1}
/** 保存前读取最新修订，仅提交本矩阵确实修改的运营字段。 */
async function save(patch:Omit<PublicContentPatch,'revision'>):Promise<AdminPublicContent>{const current=await local<AdminPublicContent>('/admin/public-content','GET',undefined,true);return local<AdminPublicContent>('/admin/public-content','PATCH',{revision:current.revision,...patch},true)}
/** 用三个可信内置素材构造完整六文案场景，不上传图片或借用交易证据。 */
function slides(position:string,images:string[]):ContentSlide[]{return images.map((image,index)=>({id:marker+'-'+position+'-'+index,kind:'image',image_url:image,target:null,labels:phrases.map((text,i)=>({id:'label-'+i,text,position,font_size:24,color:i%2?'#F5F0DE':'#FFFFFF',background:i%2?'#253B2E':'#111A15'}))}))}
/** 验证显式local-test产物的实际API和本地健康状态，不按目录名猜环境。 */
async function guard():Promise<string[]>{
  assert.ok(width===320||width===390,'UI_EXPECT_WIDTH必须为真实320或390机型')
  const address=new URL(endpoint);assert.equal(address.protocol,'ws:');assert.equal(address.hostname,'127.0.0.1');assert.equal(address.port,'9423');assert.equal(address.pathname,'/');assert.equal(address.search,'');assert.equal(address.username,'');assert.equal(address.password,'')
  const client=fs.readFileSync(path.join(project,'api/client.js'),'utf8');assert.ok(client.includes(api),'编译客户端必须固定本地18091；正式或旧包禁止执行');assert.ok(!client.includes('https://game.ai.58it.cn/api'),'本地包不得仍引用正式API')
  const config=JSON.parse(fs.readFileSync(path.join(project,'project.config.json'),'utf8')) as {compileType?:string;setting?:{urlCheck?:boolean}}
  assert.equal(config.compileType,'miniprogram');assert.equal(config.setting?.urlCheck,false,'本地HTTP工程必须显式关闭合法域名检查')
  const health=await local<{status:string;database:string;redis:string;dev_mode:boolean}>('/health');assert.equal(health.status,'ok');assert.equal(health.database,'ok');assert.equal(health.redis,'ok');assert.equal(health.dev_mode,true)
  const bootstrap=await local<{dev_mode:boolean;games:Game[]}>('/bootstrap');assert.equal(bootstrap.dev_mode,true,'只允许本地dev_mode API')
  const images=[...new Set(bootstrap.games.filter(game=>game.enabled!==false).flatMap(game=>[game.cover_url,...(game.covers||[])])).values()].filter(image=>/^\/static\/covers\/(official\/)?[a-z0-9_-]+\.(jpg|jpeg|png|webp)$/.test(image)).slice(0,3)
  assert.equal(images.length,3,'至少需要三个现有可信游戏素材，不能临时上传或借图')
  for(const image of images){const response=await fetch('http://127.0.0.1:18091'+image,{redirect:'error',signal:AbortSignal.timeout(15000)});assert.ok(response.ok&&response.headers.get('content-type')?.startsWith('image/'),'本地后端必须正常提供内置游戏图片');assert.ok((await response.arrayBuffer()).byteLength>100,'内置素材实际字节不能为空')}
  assert.equal(POSITIONS.length,9);assert.equal(phrases.length,6);for(const text of phrases)assert.equal(Array.from(text).length,12,'矩阵必须覆盖十二码点最长文字')
  return images
}
/** 只建立Node准备用短会话，真实小程序仍使用原有身份和公开首页。 */
async function prepare():Promise<void>{const session=await local<{token:string;user:User}>('/auth/dev','POST',{user_id:'admin'});assert.ok(session.token);token=session.token;assert.ok(session.user.role==='admin'&&session.user.platform_admin_role!=='deputy','准备身份必须是本地根管理员');original=await local<AdminPublicContent>('/admin/public-content','GET',undefined,true)}
/** 通过正常页面路由重新读取刚保存的公开轮播，不注入任何页面状态。 */
async function homepage(expected:ContentSlide[]):Promise<{page:Page;player:Element;carousel:Element;images:SwiperElement;labels:SwiperElement}>{
  const page=await sdk().reLaunch('/pages/main/index?tab=player');assert.ok(page);assert.equal(page.path,'pages/main/index')
  const player=await until(async()=>(await children(page,'player-view'))[0],'真实玩家视图'),carousel=await until(async()=>(await children(player,'content-carousel'))[0],'独立轮播组件')
  const images=await until(()=>carousel.$('.carousel-images'),'原生图片swiper') as SwiperElement,labels=await until(()=>carousel.$('.carousel-labels'),'独立文字swiper') as SwiperElement
  assert.equal(Number(await images.property('interval')),3000);assert.equal(await labels.style('pointer-events'),'none')
  await until(async()=>{const planes=await carousel.$$('.carousel-label-plane') as Element[];if(planes.length!==3)return false;const texts=await Promise.all(planes.map(async plane=>Promise.all((await plane.$$('.carousel-label') as Element[]).map(node=>node.text()))));return texts.every((items,index)=>items.length===6&&items.every((text,i)=>text.trim()===expected[index].labels[i].text))},'三张图各六条完整实时文字')
  await page.waitFor(800)
  return{page,player,carousel,images,labels}
}
/** 读取当前图原始几何；待采样前后索引确认稳定后再判断，避免自动切图瞬间产生假失败。 */
async function measure(player:Element,carousel:Element,index:number):Promise<Omit<Measurement,'position'|'slide'|'sample_ms'|'screenshot'>>{
  const selectors=['.hero-visual','.hero-brand-row','.hero-publish','.game-section'],nodes=await Promise.all(selectors.map(selector=>find(player,selector))),dots=await carousel.$('.carousel-dots');assert.ok(nodes.every(Boolean)&&dots,'首页原有品牌、发布、目录与指示点必须存在')
  const [hero,brand,publish,games,dotsBox]=await Promise.all([...nodes.map(node=>rect(node!)),rect(dots)])
  const planes=await carousel.$$('.carousel-label-plane') as Element[];assert.equal(planes.length,3);const labels=await planes[index].$$('.carousel-label') as Element[];assert.equal(labels.length,6,'不得隐藏或合并文案')
  const values=await Promise.all(labels.map(async(node,i)=>{const[text,box,size,pointer,visibility,opacity]=await Promise.all([node.text(),rect(node),node.style('font-size'),node.style('pointer-events'),node.style('visibility'),node.style('opacity')]);return{index:i,text:text.trim(),rect:box,font_size:Number.parseFloat(size),pointer_events:pointer,visibility,opacity}}))
  return{hero,brand,publish,dots:dotsBox,games,labels:values}
}
/** 同图稳定采样才执行可读性及碰撞断言，不用隐藏文案或缩小字号换取通过。 */
function validate(value:Omit<Measurement,'position'|'slide'|'sample_ms'|'screenshot'>,expected:ContentSlide):void{
  const{hero,brand,publish,dots,games,labels}=value
  assert.ok(hero.left>=-1&&hero.left+hero.width<=width+1,'轮播图不得水平越界');inside(brand,hero,'品牌/分享');inside(publish,hero,'发布按钮');inside(dots,hero,'轮播指示点');assert.ok(games.top>=hero.top+hero.height-1,'游戏目录不得被扩展轮播覆盖')
  for(const label of labels){assert.equal(label.text,expected.labels[label.index].text,'标签全文必须可见');assert.ok(Number.isFinite(label.font_size)&&label.font_size>=9.99,'文字小于10实际像素');assert.equal(label.pointer_events,'none');assert.notEqual(label.visibility,'hidden');assert.notEqual(label.opacity,'0');inside(label.rect,hero,'文案'+label.index);for(const [name,box] of [['品牌',brand],['发布',publish],['指示点',dots]] as const)assert.ok(separate(label.rect,box),'文案与'+name+'重叠')}
  for(let i=0;i<labels.length;i++)for(let j=i+1;j<labels.length;j++)assert.ok(separate(labels[i].rect,labels[j].rect),'文案'+i+'与'+j+'重叠')
}
/** 原生自动轮播持续运行，仅保留同一图前后索引一致的短采样，不能暂停或改写组件data。 */
async function capture(context:Awaited<ReturnType<typeof homepage>>,position:string,index:number,slide:ContentSlide):Promise<Measurement>{
  const{page,player,carousel,images,labels}=context
  for(let attempt=0;attempt<4;attempt++){
    await images.swipeTo(index)
    await until(async()=>Number(await images.property('current'))===index&&Number(await labels.property('current'))===index,'原生滑动及300毫秒字幕跟随')
    await page.waitFor(420)
    const start=Date.now(),value=await measure(player,carousel,index),file=path.join(output,position+'-slide-'+(index+1)+'.png')
    await sdk().screenshot({path:file})
    const [imageIndex,textIndex]=await Promise.all([images.property('current'),labels.property('current')]),elapsed=Date.now()-start
    if(Number(imageIndex)!==index||Number(textIndex)!==index||elapsed>=2300)continue
    validate(value,slide)
    assert.ok(fs.statSync(file).size>100,'官方SDK原始截图不能为空')
    return{position,slide:index, ...value,sample_ms:elapsed,screenshot:file}
  }
  throw new Error('原生自动轮播期间无法获得同图稳定采样：'+position+' / '+index+'；未伪造暂停或读取内部状态')
}
/** 每个位置真实保存三图组，逐图检查同组稳定高度和下方目录位置。 */
async function scene(position:string,images:string[]):Promise<void>{
  stage='九宫格 '+position;console.log('CHECK '+stage)
  const expected=slides(position,images);changed=true
  assert.ok(original)
  await save({global_slides:expected,announcement:{...original.announcement,enabled:false}})
  const publicValue=await local<PublicContent>('/public-content');assert.deepEqual(publicValue.slides.map(slide=>slide.id),expected.map(slide=>slide.id),'公开配置必须已真实生效')
  const context=await homepage(expected),group:Measurement[]=[]
  for(let index=0;index<3;index++){const value=await capture(context,position,index,expected[index]);group.push(value);measurements.push(value)}
  for(const value of group){assert.ok(Math.abs(value.hero.height-group[0].hero.height)<=1,'同一图组切换时轮播高度跳变');assert.ok(Math.abs(value.games.top-group[0].games.top)<=1,'同一图组切换推动游戏目录')}
  checks.push(position+'：3图×6条十二码点、24号请求；实际字号≥10px，全部文字与品牌/按钮/指示点分离，同组高度稳定')
  console.log('PASS '+position)
}
/** 恢复仅本矩阵改动的字段，保持修订/审计单调；撤销短会话时不注销用户原有客户端身份。 */
async function cleanup():Promise<void>{
  /** 各清理步骤独立执行并明确记录失败，避免恢复失败后遗漏会话撤销。 */
  const attempt=async(label:string,action:()=>Promise<void>):Promise<void>=>{try{await action()}catch(error){cleanupErrors.push(label+'：'+redact(error));process.exitCode=1}}
  if(changed&&original&&token)await attempt('恢复原轮播与公告开关',async()=>{assert.ok(original);const current=await local<AdminPublicContent>('/admin/public-content','GET',undefined,true);const unchanged=JSON.stringify(current.global_slides)===JSON.stringify(original.global_slides);assert.ok(unchanged||current.global_slides.every(slide=>slide.id.startsWith(marker+'-')),'运营内容被其它操作者修改，拒绝覆盖外部更新');const restored=await save({global_slides:original.global_slides,announcement:{...current.announcement,enabled:original.announcement.enabled}});assert.deepEqual(restored.global_slides,original.global_slides);assert.equal(restored.announcement.enabled,original.announcement.enabled)})
  if(mini&&changed)await attempt('重新读取已恢复首页',async()=>{await sdk().reLaunch('/pages/main/index?tab=player')})
  if(token)await attempt('撤销独立本地准备会话',async()=>{await local('/auth/logout','POST',{},true);token=''})
  if(mini)await attempt('断开官方SDK',async()=>{await sdk().disconnect();mini=undefined})
}
/** 所有本地和真实窗口守卫通过后，才允许创建短会话和可恢复运营夹具。 */
async function run():Promise<void>{
  const images=await guard();fs.mkdirSync(output,{recursive:true});mini=await automator.connect({wsEndpoint:endpoint})
  mini.on('exception',(error:unknown)=>exceptions.push(redact(error)))
  mini.on('console',(entry:unknown)=>{const value=JSON.stringify(entry);if(/Some selectors are not allowed|不支持颜色|unsupported color/i.test(value))warnings.push(redact(value))})
  const viewport=await sdk().evaluate(()=>wx.getWindowInfo()) as {windowWidth:number;windowHeight:number};assert.equal(viewport.windowWidth,width,'必须在微信工具实际切换机型，不能模拟窗口');assert.ok(viewport.windowHeight>0)
  await prepare()
  for(const[position]of POSITIONS)await scene(position,images)
  assert.equal(fs.statSync(path.join(project,'api/client.js')).mtimeMs,packageStamp,'矩阵期间编译包被其它任务覆盖');assert.equal(measurements.length,27);assert.deepEqual(exceptions,[],'不应存在运行时异常');assert.deepEqual(warnings,[],'不应存在选择器或颜色兼容告警');complete=true
}

run().catch(async error=>{failure=error;process.exitCode=1;console.error(redact(error));if(mini)try{await sdk().screenshot({path:path.join(output,'failure.png')})}catch{/* 原始失败保留，不合成替代截图。 */}}).finally(async()=>{await cleanup();fs.mkdirSync(output,{recursive:true});fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({file_purpose:'本地官方SDK首页轮播九宫格矩阵；不等同真机物理手势验收',passed:complete&&!failure&&!cleanupErrors.length,stage,width,project,checks,measurements,exceptions,compatibility_warnings:warnings,cleanup_errors:cleanupErrors,...(failure?{error:redact(failure)}:{}),boundaries:['只修改全局轮播与公告enabled；不改游戏素材池、角色、资金、订单、支付或订阅。','三张内置图各六条十二码点文字、24号请求；字号与矩形为官方SDK实测，未调用页面data、私有业务方法或注入令牌。','swiper.swipeTo为官方公开API；自动轮播保持开启，截图和几何仅保留同一当前图的稳定采样，不等同手机物理滑动。','恢复只提交本矩阵修改字段，配置修订和审计保持单调；独立管理员令牌只在进程内保存并于finally撤销。','每次执行只覆盖当前真实320或390窗口，需分别在微信开发工具切换机型后运行；未操作生产环境。'],completed_at:new Date().toISOString()},null,2)+'\n')})
