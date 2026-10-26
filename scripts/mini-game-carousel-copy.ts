/** 文件作用：只读验收13款游戏的真实首页轮播文案，在微信官方SDK实测透明文字、尺寸与安全区；不改运营配置、账户或交易。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import {createRequire} from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {SwiperElement} from '../frontend/node_modules/miniprogram-automator/out/Element'
import type {PublicContent,ContentSlide} from '../frontend/src/types/content'
import type {Game} from '../frontend/src/types/models'

/** evaluate仅读取官方原生窗口与胶囊，不访问页面私有数据。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number;statusBarHeight:number};getMenuButtonBoundingClientRect():Rect}
/** 所有几何数值均为官方SDK实测逻辑像素。 */
interface Rect{left:number;top:number;width:number;height:number}
/** 实际字幕全文、尺寸、颜色及透明度供验收复核。 */
interface LabelEvidence{text:string;rect:Rect;font_size:number;color:string;background_color:string;background_image:string;visibility:string;opacity:string;pointer_events:string}
/** 每个游戏只记录公开图文及实测几何，不记录会话或身份信息。 */
interface Evidence{game_id:string;game_name:string;slide_id:string;slide_index:number;slide_count:number;hero:Rect;native_safe:Rect;dots:Rect|null;games:Rect;services:Rect;labels:LabelEvidence[];sample_ms:number;screenshot:string}
/** 仅沿当前验收使用的公开组件穿透模板。 */
type Scope=Page|Element
const api='http://127.0.0.1:18091/api',width=Number(process.env.UI_EXPECT_WIDTH||390),root=process.cwd()
const project=path.resolve(process.env.UI_TEST_PROJECT||'frontend/dist/dev/mp-weixin'),endpoint=process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9423'
const output=path.join(root,'output/game-carousel-copy/mini-'+width),clientPath=path.join(project,'api/client.js')
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
const copies:Record<string,[string,string]>={
  honor:['峡谷开黑有队友','双排陪练一起练好配合'],peace:['战术组队一起出发','刚枪跑图都能找到搭子'],delta:['撤离路上有人同行','跑图练枪任务一起协作'],
  tft:['双人作战默契上场','阵容运营陪你一起琢磨'],valorant:['配合打好每个回合','练枪战术一起耐心复盘'],lol:['召唤师峡谷找队友','对线团战都有陪练搭子'],
  valorantm:['掌上集结配合开局','地图战术陪你逐局练习'],cfm:['火线集结并肩开黑','枪法地图一起边玩边练'],arena:['暗区探索结伴同行','熟悉地图练好撤离配合'],
  codm:['战术小队随时集结','枪法身法一起练好配合'],identityv:['庄园相约默契配合','追逃练习一起复盘细节'],narakam:['江湖结伴切磋同行','连招身法都有耐心陪练'],eggy:['蛋搭子一起闯关','地图探索欢乐组队同行']
}
const aliases:Record<string,string>={'player-view':'pages/main/player/player','content-carousel':'components/ContentCarousel','home-announcement':'components/HomeAnnouncement','uni-popup':'node-modules/@dcloudio/uni-ui/lib/uni-popup/uni-popup'}
const measurements:Evidence[]=[],checks:string[]=[],exceptions:string[]=[],warnings:string[]=[],cleanupErrors:string[]=[]
let mini:MiniProgram|undefined,stamp=0,nativeContentTop=0,stage='本地守卫',complete=false,failure:unknown

/** 隐去可能来自运行时错误的身份与认证秘密。 */
function redact(value:unknown):string{return String(value instanceof Error?value.message:value).replace(/Bearer\s+[^\s,}"']+/gi,'Bearer [REDACTED]').replace(/(token|secret|openid|authorization)["'\s:=]+[^\s,}"']+/gi,'$1=[REDACTED]').slice(0,1600)}
/** 返回本轮唯一SDK连接，不打开或切换其它工程。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 固定本地公开GET且禁止重定向，不创建或附带任何认证会话。 */
async function local<T>(route:string):Promise<T>{assert.ok(route.startsWith('/')&&!route.startsWith('//'));const url=new URL(api+route);assert.equal(url.origin,'http://127.0.0.1:18091');const response=await fetch(url,{redirect:'error',signal:AbortSignal.timeout(15000)});assert.equal(response.status,200,'本地公开读取 '+route);return(await response.json() as {data:T}).data}
/** 等待真实只读条件，超时后保留明确失败而不改内部状态。 */
async function until<T>(read:()=>Promise<T|false|null|undefined>,label:string,timeout=16000):Promise<T>{const start=Date.now();while(Date.now()-start<timeout){const value=await read();if(value)return value;await new Promise(resolve=>setTimeout(resolve,100))}throw new Error('等待超时：'+label)}
/** 兼容uni-app短组件名称及实际编译路径。 */
async function children(scope:Scope,name:string):Promise<Element[]>{for(const alias of [name,aliases[name]?.replaceAll('/','\\/').replaceAll('@','\\@')]){if(!alias)continue;const nodes=await scope.$$(alias) as Element[];if(nodes.length)return nodes}return[]}
/** 查询实际节点，最多穿过三层已知公开组件。 */
async function find(scope:Scope,selector:string,depth=0):Promise<Element|null>{const direct=await scope.$(selector);if(direct)return direct;if(depth===3)return null;for(const name of Object.keys(aliases))for(const child of await children(scope,name)){const result=await find(child,selector,depth+1);if(result)return result}return null}
/** 读取实测尺寸并拒绝无效几何，不能用CSS声明代替。 */
async function rect(node:Element):Promise<Rect>{const[position,size]=await Promise.all([node.offset(),node.size()]);const value={left:Number(position.left),top:Number(position.top),width:Number(size.width),height:Number(size.height)};assert.ok(Object.values(value).every(Number.isFinite),'SDK矩形必须为有限数值');return value}
/** 检查元素完整处于父图区域内，允许一像素舍入。 */
function inside(child:Rect,parent:Rect,label:string):void{assert.ok(child.width>0&&child.height>0,label+'实际尺寸为空');assert.ok(child.left>=parent.left-1&&child.top>=parent.top-1&&child.left+child.width<=parent.left+parent.width+1&&child.top+child.height<=parent.top+parent.height+1,label+'越出轮播图')}
/** 判断两个矩形没有正面积相交。 */
function separate(a:Rect,b:Rect):boolean{return a.left+a.width<=b.left+1||b.left+b.width<=a.left+1||a.top+a.height<=b.top+1||b.top+b.height<=a.top+1}
/** 将本批六位十六进制配置转换为SDK实际返回的RGB文本，拒绝未覆盖的颜色格式。 */
function rgb(color:string|undefined):string{assert.ok(color&&/^#[0-9a-f]{6}$/i.test(color),'本批预期颜色必须为六位十六进制');return'rgb('+[1,3,5].map(index=>Number.parseInt(color.slice(index,index+2),16)).join(',')+')'}
/** 仅点击真实可见的公告关闭按钮；关闭可能记正常已读，不修改公告内容。 */
async function closeAnnouncement(page:Page):Promise<void>{const node=await find(page,'.announcement-close');if(!node)return;const box=await rect(node);if(box.width<=0||box.height<=0||box.left+box.width<=0||box.top+box.height<=0)return;await node.tap();await page.waitFor(400)}
/** 真实编译包、本地API和当前窗口全部通过守卫后再检查页面。 */
async function guard():Promise<Game[]>{
  assert.ok(width===320||width===390,'只支持真实320/390宽度');const address=new URL(endpoint);assert.equal(address.href,'ws://127.0.0.1:9423/','只允许本地官方SDK')
  const client=fs.readFileSync(clientPath,'utf8');stamp=fs.statSync(clientPath).mtimeMs;assert.ok(client.includes(api)&&!client.includes('https://game.ai.58it.cn/api'),'验收编译包必须固定本地18091')
  const config=JSON.parse(fs.readFileSync(path.join(project,'project.config.json'),'utf8')) as {compileType?:string;setting?:{urlCheck?:boolean}};assert.equal(config.compileType,'miniprogram');assert.equal(config.setting?.urlCheck,false)
  const health=await local<{status:string;database:string;redis:string;dev_mode:boolean}>('/health');assert.equal(health.status,'ok');assert.equal(health.database,'ok');assert.equal(health.redis,'ok');assert.equal(health.dev_mode,true)
  const bootstrap=await local<{dev_mode:boolean;games:Game[]}>('/bootstrap');assert.equal(bootstrap.dev_mode,true);const games=bootstrap.games.filter(game=>game.enabled!==false);assert.equal(games.length,13);assert.deepEqual(games.map(game=>game.id).sort(),Object.keys(copies).sort());checks.push('本地HTTP工程、开发API、13款启用游戏及正式域名隔离守卫通过');return games
}
/** 打开公开游戏深链，确认完整服务端顺序及独立文字已真实渲染。 */
async function homepage(game:Game,expected:PublicContent):Promise<{page:Page;player:Element;carousel:Element;images:SwiperElement;labels:SwiperElement}>{
  assert.equal(fs.statSync(clientPath).mtimeMs,stamp,'验收期间编译包被其它任务覆盖');const page=await sdk().reLaunch('/pages/main/index?tab=player&game_id='+encodeURIComponent(game.id));assert.ok(page);await page.waitFor(400);assert.equal((await sdk().currentPage())?.path,'pages/main/index')
  const player=await until(async()=>(await children(page,'player-view'))[0],'真实玩家视图'),carousel=await until(async()=>(await children(player,'content-carousel'))[0],'真实轮播组件')
  const images=await until(()=>carousel.$('.carousel-images'),'原生图片swiper') as SwiperElement,labels=await until(()=>carousel.$('.carousel-labels'),'独立文字swiper') as SwiperElement
  assert.equal(Number(await images.property('interval')),3000);assert.equal(await labels.style('pointer-events'),'none')
  await until(async()=>{const planes=await carousel.$$('.carousel-label-plane') as Element[];if(planes.length!==expected.slides.length)return false;const values=await Promise.all(planes.map(async plane=>Promise.all((await plane.$$('.carousel-label') as Element[]).map(node=>node.text()))));return values.every((items,index)=>items.length===expected.slides[index].labels.length&&items.every((text,i)=>text.trim()===expected.slides[index].labels[i].text))},'当前游戏和完整后端顺序的全部独立文案')
  await page.waitFor(650);await closeAnnouncement(page);assert.equal(await images.property('autoplay'),expected.slides.length>1,'权威图文就绪后自动轮播必须保持开启');return{page,player,carousel,images,labels}
}
/** 在同图稳定窗口一次读取所有实际尺寸和文字，未预设轮播张数或游戏位置。 */
async function measure(context:Awaited<ReturnType<typeof homepage>>,game:Game,content:PublicContent,index:number):Promise<Omit<Evidence,'sample_ms'|'screenshot'>>{
  const{player,carousel}=context;const nodes=await Promise.all(['.hero-visual','.game-section','.service-toolbar'].map(selector=>find(player,selector)));assert.ok(nodes.every(Boolean),'原轮播、游戏目录、服务区都必须存在')
  const[hero,games,services]=await Promise.all(nodes.map(node=>rect(node!))),dotsNode=await carousel.$('.carousel-dots'),dots=dotsNode?await rect(dotsNode):null
  const planes=await carousel.$$('.carousel-label-plane') as Element[];assert.equal(planes.length,content.slides.length);const labelNodes=await planes[index].$$('.carousel-label') as Element[]
  const labels=await Promise.all(labelNodes.map(async node=>{const[text,box,font,color,bg,image,visibility,opacity,pointer]=await Promise.all([node.text(),rect(node),node.style('font-size'),node.style('color'),node.style('background-color'),node.style('background-image'),node.style('visibility'),node.style('opacity'),node.style('pointer-events')]);return{text:text.trim(),rect:box,font_size:Number.parseFloat(font),color,background_color:bg,background_image:image,visibility,opacity,pointer_events:pointer}}))
  return{game_id:game.id,game_name:game.name,slide_id:content.slides[index].id,slide_index:index,slide_count:content.slides.length,hero,native_safe:{left:hero.left,top:hero.top,width:hero.width,height:nativeContentTop},dots,games,services,labels}
}
/** 校验两个完整纯文字图层、主次字号和全部原生避让，不隐藏文案换取通过。 */
function validate(value:Omit<Evidence,'sample_ms'|'screenshot'>,slide:ContentSlide):void{
  const{hero,games,services,native_safe:nativeSafe,dots,labels}=value;assert.equal(labels.length,2);assert.ok(hero.left>=-1&&hero.left+hero.width<=width+1);inside(nativeSafe,hero,'状态栏和胶囊安全带');if(dots)inside(dots,hero,'指示点');assert.ok(games.top>=hero.top+hero.height-1,'轮播不能压游戏目录');assert.ok(services.top>=games.top+games.height-1,'原服务区不能压游戏目录')
  for(const box of [games,services])assert.ok(box.left>=-1&&box.left+box.width<=width+1,'原分类服务区不得水平溢出')
  for(const[label,index]of labels.map((label,index)=>[label,index] as const)){assert.equal(label.text,slide.labels[index].text);assert.equal(label.color.replace(/\s/g,'').toLowerCase(),rgb(slide.labels[index].color),'实际文字颜色必须匹配当前公开配置');assert.ok(Math.abs(label.font_size-(slide.labels[index].font_size||16)*hero.width/360)<.15,'请求字号应按实际宽度缩放');assert.ok(label.font_size>=9.99);assert.equal(label.pointer_events,'none');assert.notEqual(label.visibility,'hidden');assert.notEqual(label.opacity,'0');assert.ok(['transparent','rgba(0,0,0,0)'].includes(label.background_color.replace(/\s/g,'').toLowerCase()),'字幕背景必须透明');assert.equal(label.background_image,'none');inside(label.rect,hero,'独立文案'+index);assert.ok(separate(label.rect,nativeSafe),'文字不能重叠状态栏或胶囊');if(dots)assert.ok(separate(label.rect,dots),'文字不能重叠指示点')}
  assert.ok(separate(labels[0].rect,labels[1].rect),'两段文字不能互相重叠');assert.ok(labels[0].rect.top<labels[1].rect.top,'主标题应位于副文案上方');assert.ok(labels[0].rect.left<labels[1].rect.left,'主标题左上，副文案右下');assert.ok(labels[0].font_size>labels[1].font_size,'主标题字号应更大')
}
/** 使用官方swiper操作选择目标，自动轮播照常运行，仅接受前后同图且不足2300毫秒的采样。 */
async function capture(context:Awaited<ReturnType<typeof homepage>>,game:Game,content:PublicContent,index:number):Promise<Evidence>{
  const{page,images,labels}=context;for(let attempt=0;attempt<4;attempt++){await images.swipeTo(index);await until(async()=>Number(await images.property('current'))===index&&Number(await labels.property('current'))===index,'图片与300毫秒后的字幕一致');await page.waitFor(420);const start=Date.now(),[beforeImage,beforeText]=await Promise.all([images.property('current'),labels.property('current')]);if(Number(beforeImage)!==index||Number(beforeText)!==index)continue;const value=await measure(context,game,content,index),file=path.join(output,game.id+'.png');await sdk().screenshot({path:file});const[imageIndex,textIndex]=await Promise.all([images.property('current'),labels.property('current')]),elapsed=Date.now()-start;if(Number(imageIndex)!==index||Number(textIndex)!==index||elapsed>=2300)continue;try{validate(value,content.slides[index])}catch(error){fs.writeFileSync(path.join(output,'failed-measurement.json'),JSON.stringify(value,null,2)+'\n');throw error}assert.ok(fs.statSync(file).size>100);return{...value,sample_ms:elapsed,screenshot:file}}throw new Error(game.id+'无法取得同图稳定采样；未暂停自动轮播或注入内部状态')
}
/** 逐个公开游戏检查当前配置，既有平台轮播也按服务端顺序完整核对。 */
async function run():Promise<void>{
  const games=await guard();fs.mkdirSync(output,{recursive:true});mini=await automator.connect({wsEndpoint:endpoint});mini.on('exception',(error:unknown)=>exceptions.push(redact(error)));mini.on('console',(entry:unknown)=>{const value=JSON.stringify(entry);if(/Some selectors are not allowed|不支持颜色|unsupported color/i.test(value))warnings.push(redact(value))})
  const native=await sdk().evaluate(()=>({window:wx.getWindowInfo(),capsule:wx.getMenuButtonBoundingClientRect()})) as {window:{windowWidth:number;windowHeight:number;statusBarHeight:number};capsule:Rect};assert.equal(native.window.windowWidth,width,'必须实际切换微信模拟器机型');assert.ok(native.window.windowHeight>0&&native.window.statusBarHeight>=0);assert.ok(native.capsule.top>=native.window.statusBarHeight&&native.capsule.height>0);nativeContentTop=native.window.statusBarHeight+2*(native.capsule.top-native.window.statusBarHeight)+native.capsule.height
  for(const game of games){stage=game.name;console.log('CHECK '+game.id+' '+game.name);const content=await local<PublicContent>('/public-content?game_id='+encodeURIComponent(game.id));assert.equal(content.game_id,game.id);const expected=copies[game.id],index=content.slides.findIndex(slide=>slide.labels.length===2&&slide.labels.every((label,i)=>label.text===expected[i]));assert.ok(index>=0,game.id+'应有对应的两段游戏文案');const slide=content.slides[index];assert.equal(slide.labels[0].position,'top-left');assert.equal(slide.labels[1].position,'bottom-right');assert.equal(slide.labels[0].font_size,24);assert.equal(slide.labels[1].font_size,16);const context=await homepage(game,content);measurements.push(await capture(context,game,content,index));assert.deepEqual((await local<PublicContent>('/public-content?game_id='+encodeURIComponent(game.id))).slides,content.slides,'验收期间公开运营图文不得变化');checks.push(game.id+'：两段完整纯文字、主24副16、透明背景、左上右下、原生安全区及原目录服务区通过');console.log('PASS '+game.id)}
  assert.equal(fs.statSync(clientPath).mtimeMs,stamp);assert.equal(measurements.length,13);assert.deepEqual(exceptions,[]);assert.deepEqual(warnings,[]);complete=true
}

run().catch(async error=>{failure=error;process.exitCode=1;console.error(redact(error));fs.mkdirSync(output,{recursive:true});if(mini)try{await sdk().screenshot({path:path.join(output,'failure.png')})}catch{/* 保留实际原始错误，不生成替代截图。 */}}).finally(async()=>{if(mini)try{await sdk().disconnect()}catch(error){cleanupErrors.push(redact(error));process.exitCode=1}fs.mkdirSync(output,{recursive:true});fs.writeFileSync(path.join(output,'report.json'),JSON.stringify({file_purpose:'13款游戏独立轮播文案的本地微信官方SDK验收',passed:complete&&!failure&&!cleanupErrors.length,stage,width,project,checks,measurements,exceptions,compatibility_warnings:warnings,cleanup_errors:cleanupErrors,...(failure?{error:redact(failure)}:{}),boundaries:['仅本地公开GET与正常页面导航，不创建认证会话，不改运营配置、角色、资金或交易。','如当前公告可见，只点击其正常关闭按钮；可能按既有逻辑记录已读，不修改公告内容。','未读取页面data或调用私有方法，未注入令牌、存储、假窗口或假二维码。','原生自动轮播每3秒保持开启，官方swipeTo只选择检查目标；同图稳定窗口才接受实测与截图。','仅当前真实320或390窗口；颜色可读性仍需人工复核原始截图，不等同真机、微信支付或线上小程序上传验收。'],completed_at:new Date().toISOString()},null,2)+'\n')})
