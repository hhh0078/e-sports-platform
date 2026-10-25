/** 文件作用：仅在本地开发后端用微信官方SDK逐项验收服务卡片、编辑预览及详情的文字/押金边界。 */
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import type MiniProgram from '../frontend/node_modules/miniprogram-automator/out/MiniProgram'
import type Page from '../frontend/node_modules/miniprogram-automator/out/Page'
import type Element from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { InputElement } from '../frontend/node_modules/miniprogram-automator/out/Element'
import type { Service, Game, AuthSession, CoverLabel } from '../frontend/src/types/models'
import { normalizeLabelText } from '../frontend/src/lib/cover'

const root=process.cwd(),api='http://127.0.0.1:18091/api',out=path.resolve(process.env.UI_TEST_OUTPUT||'output/deposit-finance/card-390')
const require=createRequire(path.join(root,'frontend/package.json')),automator=require('miniprogram-automator') as {connect(options:{wsEndpoint:string}):Promise<MiniProgram>}
let mini:MiniProgram|undefined
const checks:string[]=[],exceptions:string[]=[]
let stage='准备本地验收数据'
/** 微信逻辑层仅调用原生窗口和路由能力，不写入登录或业务状态。 */
declare const wx:{getWindowInfo():{windowWidth:number;windowHeight:number};navigateTo(options:{url:string}):void;reLaunch(options:{url:string}):void;pageScrollTo(options:{scrollTop:number;duration:number}):void}
/** 从已连接的官方自动化对象读取界面。 */
function sdk():MiniProgram{assert.ok(mini);return mini}
/** 限时等待明确业务元素，不以盲等替代断言。 */
async function until<T>(read:()=>Promise<T|null|false|undefined>,label:string):Promise<T>{const start=Date.now();while(Date.now()-start<15000){const result=await read();if(result)return result;await new Promise(resolve=>setTimeout(resolve,120))}throw new Error('等待超时：'+label)}
/** 本地夹具调用不打印会话或微信密钥。 */
async function call<T>(route:string,method='GET',body?:unknown,token=''):Promise<T>{const response=await fetch(api+route,{method,headers:{'Content-Type':'application/json',...(token?{Authorization:'Bearer '+token}:{})},...(body?{body:JSON.stringify(body)}:{})});const result=await response.json() as {data:T;error?:string};assert.ok(response.ok,route+' '+response.status+' '+(result.error||''));return result.data}
/** 原生导航结束后按真实路由确认页面就绪。 */
async function open(url:string,reset=false):Promise<Page>{if(reset){const page=await sdk().reLaunch(url);assert.ok(page);return page}await sdk().evaluate((value:string)=>{setTimeout(()=>wx.navigateTo({url:value}),30);return true},url);return until(async()=>{const page=await sdk().currentPage();return page?.path===url.split('?')[0].slice(1)?page:null},url)}
/** 跨uni-app注册路径查找共享组件。 */
async function child(scope:Page|Element,names:string[]):Promise<Element|null>{for(const name of names){const item=await scope.$(name);if(item)return item}return null}
/** 找到用户可见文案按钮，原生点击完成登录与跳过资料。 */
async function button(part:string):Promise<Element>{return until(async()=>{const page=await sdk().currentPage();if(!page)return null;for(const item of await page.$$('button'))if((await item.text()).includes(part))return item;return null},'按钮 '+part)}
/** 通过真实协议勾选和开发身份登录，不注入令牌。 */
async function login():Promise<void>{const page=await open('/pages/account/login',true);await(await until(()=>page.$('.consent-line label'),'两协议勾选')).tap();await(await button('开发')).tap();await(await button('北辰')).tap();await(await button('稍后')).tap();await until(async()=>(await sdk().currentPage())?.path==='pages/main/index','回到主页面')}
/** 读取实际元素像素矩形。 */
async function rect(element:Element){const [offset,size]=await Promise.all([element.offset(),element.size()]);return{left:Number(offset.left),top:Number(offset.top),width:Number(size.width),height:Number(size.height)}}
type Rect=Awaited<ReturnType<typeof rect>>
/** 带一像素容差检查两个可见块没有交叠。 */
function separate(a:Rect,b:Rect):boolean{return a.left+a.width<=b.left+1||b.left+b.width<=a.left+1||a.top+a.height<=b.top+1||b.top+b.height<=a.top+1}
/** 检查内容处于指定图片/卡片范围。 */
function inside(a:Rect,b:Rect,label:string):void{assert.ok(a.left>=b.left-1&&a.top>=b.top-1&&a.left+a.width<=b.left+b.width+1&&a.top+a.height<=b.top+b.height+1,label+'越界')}
/** 原始PNG截图保存到验收目录，不能写入设计目录。 */
async function screenshot(name:string):Promise<void>{await new Promise(resolve=>setTimeout(resolve,250));await sdk().screenshot({path:path.join(out,name+'.png')})}
/** 检查共享封面的所有自定义文案、系统标题标签及押金互不相撞。 */
async function checkCover(card:Page|Element,service:Pick<Service,'labels'|'provider_deposit_cents'|'provider_club_deposit_cents'>&{club_verified?:boolean},detail=false):Promise<void>{
  const cover=await until(()=>child(card,['cover-art','components\\/CoverArt']),'共享封面'),frame=await until(()=>cover.$('.cover-art'),'封面矩形'),bounds=await rect(frame)
  const labels=await cover.$$('.cover-label');assert.equal(labels.length,service.labels.length,'不能隐藏用户文案')
  const areas:Rect[]=[]
  const gradient=await(detail?card.$('.detail-gradient'):cover.$('.cover-shade'));assert.ok(gradient,'封面应保留渐变背景')
  for(let index=0;index<labels.length;index++){const label=labels[index],value=await rect(label);inside(value,bounds,'自定义文案');assert.ok(value.width>0&&value.height>0,'文案必须有实际尺寸');assert.notEqual(await label.style('visibility'),'hidden','文案不可隐藏');assert.ok(Number(await label.style('opacity'))>0,'文案不可透明隐藏');assert.ok(Number(await label.style('z-index'))>Number(await gradient.style('z-index')),'渐变不能压暗用户自选的文字与背景');assert.equal((await label.text()).trim(),service.labels[index].text,'文案必须完整保留');assert.ok(parseFloat(await label.style('font-size'))>=(detail?13.9:9.9),'封面文字过小');areas.push(value)}
  for(let i=0;i<areas.length;i++)for(let j=i+1;j<areas.length;j++)assert.ok(separate(areas[i],areas[j]),'自定义文案重叠')
  const system:Rect[]=[]
  const amounts=[service.provider_deposit_cents||0,service.provider_club_deposit_cents||0].filter(value=>Number.isSafeInteger(value)&&value>0),depositSelector=detail?'.detail-deposits':'.service-deposits'
  /** 金额版仍横排，紧凑版改为共享盾牌；只沿真实公开组件查找。 */
  async function depositContent(badge:Element):Promise<{inner:Element;iconScope:Element}|null>{const amount=await badge.$('.deposit-badge');if(amount)return{inner:amount,iconScope:badge};const marker=await child(badge,['security-marker','components\\/SecurityMarker']);if(!marker)return null;const inner=await marker.$('.security-marker');return inner?{inner,iconScope:marker}:null}
  await until(async()=>{let visible=0;for(const badge of await card.$$('components\\/DepositBadge'))if(await depositContent(badge))visible++;return visible===amounts.length},'实缴押金徽标数量')
  const visibleBadges:Element[]=[]
  for(const badge of await card.$$('components\\/DepositBadge'))if(await depositContent(badge))visibleBadges.push(badge)
  const kinds=[{amount:service.provider_deposit_cents||0,label:detail?'平台押金':'押金'},{amount:service.provider_club_deposit_cents||0,label:detail?'俱乐部押金':'俱乐部'}].filter(item=>Number.isSafeInteger(item.amount)&&item.amount>0)
  for(let index=0;index<visibleBadges.length;index++){const badge=visibleBadges[index],text=await badge.text(),content=await depositContent(badge);assert.ok(content);const{inner,iconScope}=content;assert.ok(text.includes(kinds[index].label),'押金类型必须明确');assert.ok(await child(iconScope,['icon','components\\/Icon']),'押金必须保留共享实心安全图标');if(detail){assert.match(await inner.style('background-color'),/rgba?\(24,\s*21,\s*14(?:,.*)?\)/,'金额版保留黑金底色');assert.equal(await inner.style('color'),'rgb(228, 189, 112)');assert.ok(text.includes((amounts[index]/100).toFixed(2).replace(/\.00$/,'').replace(/(\.\d)0$/,'$1')+'元'),'详情押金应显示真实金额')}else{assert.equal(await inner.style('color'),'rgb(229, 189, 112)','紧凑盾牌统一金色');const value=await rect(inner);assert.ok(value.width<=30&&value.height<=34,'小盾牌不能遮挡卡片');const shape=await iconScope.$('.security-marker-shape');assert.ok(shape,'盾形轮廓必须独立于文字');inside(value,bounds,'小盾牌')}}
  if(!detail&&(amounts.length||service.club_verified)){const group=await card.$('.service-deposits');assert.ok(group);const value=await rect(group);assert.ok(Math.abs(value.top-bounds.top)<=1&&Math.abs(value.left+value.width-bounds.left-bounds.width)<=1,'盾牌组应贴齐图片右上角')}
  for(const selector of detail?['.detail-title','.detail-labels',depositSelector]:['.service-title','.service-tags',depositSelector]){const item=await card.$(selector);if(selector===depositSelector&&!amounts.length&&(detail||!service.club_verified)){assert.equal(item,null,'零押金不能有系统徽标容器');continue}assert.ok(item,'缺少系统内容 '+selector);const value=await rect(item);inside(value,bounds,selector);system.push(value);for(const label of areas)assert.ok(separate(label,value),'用户文案与'+selector+'重叠')}
  for(let i=0;i<system.length;i++)for(let j=i+1;j<system.length;j++)assert.ok(separate(system[i],system[j]),'系统标题/标签/押金重叠')
  if(!detail){const title=await card.$('.service-title'),tags=await card.$('.service-tags');assert.ok(title&&tags);const t=await rect(title),g=await rect(tags);assert.ok(t.top+t.height<=g.top+1,'标题应在标签上方');const price=await card.$('.service-price');assert.ok(price,'正式卡片和完整发布预览必须有价格');const p=await rect(price);assert.ok(p.top>=bounds.top+bounds.height-1,'价格不能盖住封面');assert.ok(p.top-(g.top+g.height)<=24,'价格与标签间留白过大')}
}
/** 构造独立样式边界矩阵，文字只作本地验收，不触发真实交易。 */
function cases(games:Game[]):Array<{name:string;game:Game;labels:CoverLabel[];title:string;unit:string;price:number}>{
  const positions=['top-left','top-center','top-right','middle-left','middle-center','middle-right','bottom-left','bottom-center','bottom-right']
  return Array.from({length:12},(_,i)=>{
    const count=i===0?0:i<3?1:6,game=games[i%games.length]
    const labels=Array.from({length:count},(_,j)=>({id:'qa-'+j,text:normalizeLabelText(i===5?'👨‍👩‍👧‍👦队友协作稳定陪玩':i===6?'TEAMMATE2026':'耐心沟通认真协作一起玩'),position:i===3?'bottom-left':i===4?'top-right':positions[(i+j)%positions.length],font_size:i%2?24:12,color:i%3?'#ffffff':'#101714',background:i%3===0?'#f4c36c':i%3===1?'#07130f':'transparent'}))
    return{name:'case-'+String(i+1).padStart(2,'0'),game,labels,title:'样式验收'+String(i+1).padStart(2,'0')+' · '+(i>7?'长标题游戏爱好者的耐心陪练任务协作与周末组队服务':'耐心陪练'),unit:i===11?'每次任务':'小时',price:i===11?999999:2000}
  })
}
/** 发布本地样式夹具并审核，已有外部接口能力不由客户端伪造。 */
async function fixtures():Promise<Service[]>{
  const health=await call<{dev_mode:boolean}>('/health');assert.equal(health.dev_mode,true)
  const worker=await call<AuthSession>('/auth/dev','POST',{user_id:'worker'}),admin=await call<AuthSession>('/auth/dev','POST',{user_id:'admin'})
  if(!worker.user.phone_verified)await call('/me/phone/dev','POST',{phone_number:'13300000108'},worker.token)
  const games=(await call<{games:Game[]}>('/bootstrap')).games.filter(game=>game.enabled)
  assert.ok(games.length>=7)
  const previous=await call<Service[]>('/services?mine=1&limit=100','GET',undefined,worker.token),result:Service[]=[]
  for(const scenario of cases(games)){
    const body={game_id:scenario.game.id,title:scenario.title,description:'本地样式验收数据，用于检查文字颜色、位置、字号与边界。',category:scenario.game.categories[0],price_cents:scenario.price,unit:scenario.unit,cover_url:scenario.game.covers[0]||scenario.game.cover_url,labels:scenario.labels,tags:['本地样式验收'],club_id:scenario.game.id==='delta'?'shanhai':''}
    const old=previous.find(item=>item.title===body.title),saved=await call<Service>(old?'/services/'+old.id:'/services',old?'PATCH':'POST',body,worker.token)
    await call('/admin/services/'+saved.id,'PATCH',{status:'active'},admin.token);result.push(await call<Service>('/services/'+saved.id))
  }
  const personal=await call<{deposit_cents:number}>('/wallet','GET',undefined,worker.token)
  if(!personal.deposit_cents){const payment=await call<{payment:{id:string}}>('/wallet/topups','POST',{purpose:'deposit',amount_cents:100,request_id:'cardmatrix-personal-deposit'},worker.token);await call('/wallet/topups/'+payment.payment.id+'/sync','POST',{},worker.token)}
  const security=await call<{membership:{amount_cents:number}}>('/clubs/shanhai/deposits','GET',undefined,worker.token)
  if(!security.membership.amount_cents){const payment=await call<{payment:{id:string}}>('/wallet/topups','POST',{purpose:'member_club',club_id:'shanhai',amount_cents:25,request_id:'cardmatrix-club-deposit'},worker.token);await call('/wallet/topups/'+payment.payment.id+'/sync','POST',{},worker.token)}
  return Promise.all(result.map(item=>call<Service>('/services/'+item.id)))
}
/** 遍历公开卡片、同数据编辑预览与详情，同时核对空白/长输入及独立文本层。 */
async function run():Promise<void>{
  fs.mkdirSync(out,{recursive:true});const services=await fixtures()
  if(process.env.MATRIX_PREPARE_ONLY==='1'){fs.writeFileSync(path.join(out,'fixtures.json'),JSON.stringify({prepared_only:true,services},null,2)+'\n');console.log('prepared_local_scenarios='+services.length);return}
  mini=await automator.connect({wsEndpoint:process.env.WX_AUTOMATOR_ENDPOINT||'ws://127.0.0.1:9422'});mini.on('exception',(value:unknown)=>exceptions.push(String(value)))
  const viewport=await sdk().evaluate(()=>wx.getWindowInfo());if(process.env.UI_EXPECT_WIDTH)assert.equal(viewport.windowWidth,Number(process.env.UI_EXPECT_WIDTH))
  await login()
  const profile=await open('/pages/community/players/detail?id=worker',true)
  await until(async()=>!await profile.$('.loading-line')&&await profile.$('.public-section'),'公开服务完整加载')
  for(const service of services){
    stage='卡片：'+service.title;console.log(stage)
    const card=await until(async()=>{const cards=await profile.$$('components\\/ServiceCard');for(const item of cards)if((await item.text()).includes(service.title))return item;return null},service.title)
    await checkCover(card,service);checks.push(service.title+' 卡片边界');const offset=await card.offset();await sdk().pageScrollTo(Math.max(0,await profile.scrollTop()+offset.top-110));await screenshot('card-'+services.indexOf(service))
  }
  await screenshot('cards-first-screen')
  for(let i=0;i<services.length;i++){
    stage='发布预览：'+services[i].title;console.log(stage)
    const service=services[i],page=await open('/pages/services/publish?id='+service.id,true),card=await until(()=>child(page,['service-card','components\\/ServiceCard']),'编辑预览')
    await until(async()=>{const title=await card.$('.service-title');return title&&(await title.text())===service.title},'预览真实数据')
    await checkCover(card,service);checks.push(service.title+' 编辑预览');await screenshot('preview-'+i)
    stage='服务详情：'+service.title;console.log(stage);const detail=await open('/pages/services/detail?id='+service.id,true)
    const title=await until(()=>detail.$('.detail-title'),'详情标题');assert.equal(await title.text(),service.title)
    await checkCover(detail,service,true);checks.push(service.title+' 详情边界');await screenshot('detail-'+i)
  }
  const create=await open('/pages/services/publish',true),titleInput=await until(async()=>{for(const item of await create.$$('input'))if((await item.attribute('placeholder')).includes('护航'))return item;return null},'创建服务标题') as InputElement
  await titleInput.input('创建验收 · 填写后实时预览');const preview=await until(()=>child(create,['service-card','components\\/ServiceCard']),'新建预览');await until(async()=>{const title=await preview.$('.service-title');return title&&(await title.text()).includes('创建验收')},'创建标题实时显示');await checkCover(preview,{labels:[],provider_deposit_cents:services[0].provider_deposit_cents});checks.push('新建服务标题实时预览');await screenshot('create-input')
  assert.deepEqual(exceptions,[])
  fs.writeFileSync(path.join(out,'report.json'),JSON.stringify({passed:true,viewport,scenarios:services.length,checks,runtime_exceptions:exceptions},null,2)+'\n')
  fs.rmSync(path.join(out,'failure.json'),{force:true})
  console.log('PASS '+checks.length+'项，'+services.length+'组服务样式，宽度'+viewport.windowWidth)
}
run().catch(error=>{fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'failure.json'),JSON.stringify({passed:false,stage,error:String(error),checks,runtime_exceptions:exceptions},null,2)+'\n');console.error(error);process.exitCode=1}).finally(()=>mini?.disconnect())
