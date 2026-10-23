/** 文件作用：执行真实服务下单和发布脚本、手机号门槛与共享会话，验证授权失败、连点、账号切换和迟到创建不会串号或重复写入。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'
import { readSafeArea } from './safe-area'
import { normalizeLabelText, newLabel, POSITIONS, gameCovers, randomGameCover } from './cover'

const storeSource=readFileSync(new URL('./store.ts',import.meta.url),'utf8')
const phoneSource=readFileSync(new URL('./phone.ts',import.meta.url),'utf8')
const sources={purchase:pageScript('services/detail.vue'),publish:pageScript('services/publish.vue')}
type PageKind=keyof typeof sources
const accountA={id:'A',nickname:'服务用户',phone_verified:false,provider_enabled:true}
const accountB={id:'B',nickname:'另一位用户',phone_verified:true,provider_enabled:true}

/** 用真实脚本和具名API执行提交，仅控制网络完成顺序与原生生命周期边界。 */
function harness(kind:PageKind,providerEnabled=true){
  const storage=new Map<string,string>([['jijie_token','token-A']]),requests:DeferredRequest[]=[],notices:unknown[]=[],navigations:string[]=[]
  const lifecycle:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({
    reactive,ref,computed,watch,readSafeArea,normalizeLabelText,newLabel,POSITIONS,gameCovers,randomGameCover,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',
    /** 每个网络调用由测试显式结束，重复写入不会被模拟层吞掉。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
    notify:(value:unknown)=>notices.push(value),go:(value:string)=>navigations.push(value),
    onLoad:(callback:PageLifecycle['load'])=>{lifecycle.load=callback},onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource)+'\nglobalThis.store={state,login,refreshMe,sessionRevision}',context)
  context.store.state.ready=true;context.store.state.user={...accountA,provider_enabled:providerEnabled}
  vm.runInContext('(function(){'+executable(phoneSource)+';globalThis.requirePhone=requirePhone;globalThis.phone={phoneGate,finishPhone}})()',context)
  if(kind==='purchase')vm.runInContext('const submitServiceOrder=createOrder',context)
  const exposed=kind==='purchase'?'submit:createOrder,busy:submitting,requirements,quantity,error,service,coverReserveTop,coverMinLabelArea,providerDepositCents':'submit:save,load,clubs,clubOptions,busy,form,serviceId,error,previewService,changeGame'
  vm.runInContext(executable(sources[kind])+'\nglobalThis.page={'+exposed+'}',context)
  if(kind==='purchase'){vm.runInContext("id='service-A'",context);context.page.requirements.value=' 晚上八点陪练一小时 ';context.page.quantity.value=2}
  else Object.assign(context.page.form,{game_id:'delta',title:' 三角洲陪练 ',description:'按约定时段双排教学，耐心沟通。',category:'陪练教学',price:'39.90',unit:'小时',cover_url:'/static/covers/official/delta-3.jpg',labels:[],tagsText:'耐心,教学',club_id:''})
  return{
    store:context.store,page:context.page,phone:context.phone,requests,notices,navigations,lifecycle,storage,creationPath:kind==='purchase'?'/orders':'/services',
    /** 比较取消、失败和旧会话后仍保留的原始表单内容。 */
    draft(){return JSON.stringify(kind==='purchase'?{requirements:context.page.requirements.value,quantity:context.page.quantity.value}:context.page.form)},
    /** 取得真实发出的请求，缺失调用直接失败。 */
    take(path:string,method='GET'){const request=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,`缺少请求 ${method} ${path}`);request.taken=true;return request},
  }
}

/** 排空当前微任务，使真实提交推进到受控请求。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}

/** 通过真实登录使用户、令牌和会话版本同时变化。 */
async function switchAccount(subject:ReturnType<typeof harness>){const changing=subject.store.login('B');subject.take('/auth/dev','POST').resolve({token:'token-B',user:{...accountB}});await changing}

test('服务下单和发布的手机号读取失败均被捕获并保留草稿',async t=>{
  for(const kind of ['purchase','publish'] as const)await t.test(kind,async()=>{
    const subject=harness(kind),draft=subject.draft(),submitting=subject.page.submit()
    subject.take('/me').reject(new Error('手机号状态读取失败'));await submitting
    assert.equal(subject.page.busy.value,false)
    assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
    assert.equal(subject.notices.length,1)
    assert.equal(subject.draft(),draft)
  })
})

test('服务下单和发布授权期间切换账号不会创建旧草稿',async t=>{
  for(const kind of ['purchase','publish'] as const)await t.test(kind,async()=>{
    const subject=harness(kind),draft=subject.draft(),submitting=subject.page.submit()
    subject.take('/me').resolve({...accountA});await flush();assert.equal(subject.phone.phoneGate.visible,true)
    await switchAccount(subject);subject.phone.finishPhone(true);await submitting
    assert.equal(subject.requests.filter(item=>item.path===subject.creationPath).length,0)
    assert.equal(subject.page.busy.value,false)
    assert.deepEqual(subject.navigations,[])
    assert.equal(subject.draft(),draft)
  })
})

test('服务下单和发布在手机号授权与创建期间连点均只写入一次',async t=>{
  for(const kind of ['purchase','publish'] as const)await t.test(kind,async()=>{
    const subject=harness(kind),first=subject.page.submit(),second=subject.page.submit()
    assert.equal(subject.page.busy.value,true)
    assert.equal(subject.requests.filter(item=>item.path==='/me').length,1)
    subject.take('/me').resolve({...accountA});await flush();await subject.page.submit()
    const refreshing=subject.store.refreshMe();subject.take('/me').resolve({...accountA,phone_verified:true});await refreshing
    subject.phone.finishPhone(true);await flush()
    const creating=subject.take(subject.creationPath,'POST');await subject.page.submit()
    assert.equal(subject.requests.filter(item=>item.path===subject.creationPath).length,1)
    if(kind==='purchase'){assert.equal(creating.body.service_id,'service-A');assert.equal(creating.body.quantity,2)}
    else{assert.equal(creating.body.price_cents,3990);assert.equal(creating.body.title,'三角洲陪练')}
    creating.resolve({id:'created'});await Promise.all([first,second])
    assert.equal(subject.page.busy.value,false)
    assert.equal(subject.navigations.length,1)
  })
})

test('服务创建的迟到结果不能触发新账号或已销毁页面的导航',async t=>{
  for(const kind of ['purchase','publish'] as const)for(const boundary of ['switch','unload'])await t.test(kind+'-'+boundary,async()=>{
    const subject=harness(kind),draft=subject.draft(),submitting=subject.page.submit()
    subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
    const creating=subject.take(subject.creationPath,'POST')
    if(boundary==='switch')await switchAccount(subject);else subject.lifecycle.unload()
    creating.resolve({id:'old-result'});await submitting
    assert.deepEqual(subject.navigations,[])
    assert.deepEqual(subject.notices,[])
    assert.equal(subject.page.busy.value,false)
    assert.equal(subject.draft(),draft)
  })
})

test('旧本地接单标记不触发启停接口，手机号验证通过后直接发布',async()=>{
  const subject=harness('publish',false),submitting=subject.page.submit()
  subject.take('/me').resolve({...accountA,provider_enabled:false,phone_verified:true});await flush()
  assert.equal(subject.requests.filter(item=>item.path==='/me'&&item.method==='PATCH').length,0)
  const creating=subject.take('/services','POST')
  assert.equal(creating.body.title,'三角洲陪练')
  creating.resolve({id:'default-provider-service'});await submitting
  assert.deepEqual(subject.navigations,['/pages/main/index?tab=worker'])
  assert.equal(subject.page.busy.value,false)
})

test('AI拒绝或故障不会丢失服务发布和下单草稿，可修改后重新提交',async t=>{
  for(const kind of ['purchase','publish'] as const)for(const code of ['moderation_rejected','moderation_unavailable'])await t.test(kind+'-'+code,async()=>{
    const subject=harness(kind),draft=subject.draft(),submitting=subject.page.submit()
    subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
    const failure={code,message:'内容审核未通过：请修改后再提交',status:code==='moderation_rejected'?422:503}
    subject.take(subject.creationPath,'POST').reject(failure);await submitting
    assert.equal(subject.draft(),draft)
    assert.equal(subject.page.busy.value,false)
    if(kind==='publish')assert.equal(subject.page.error.value,failure.message)
    assert.equal(subject.notices[0],failure)
    assert.deepEqual(subject.navigations,[])
    const retry=subject.page.submit();subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
    subject.take(subject.creationPath,'POST').resolve({id:'approved-after-edit'});await retry
    assert.equal(subject.navigations.length,1)
  })
})

test('服务详情按非空自定义标签数量保留24、48、72像素文字带',()=>{
  const subject=harness('purchase')
  assert.equal(subject.page.coverReserveTop,56)
  for(const [count,area] of [[0,0],[1,24],[2,24],[3,48],[4,48],[5,72],[6,72]]){
    subject.page.service.value={labels:Array.from({length:count},(_,id)=>({id,text:'自定义服务文字'}))}
    assert.equal(subject.page.coverMinLabelArea.value,area)
  }
  subject.page.service.value={labels:[{text:'   '},{text:'\n'},{text:'有效文案'}]}
  assert.equal(subject.page.coverMinLabelArea.value,24)
  subject.page.service.value={labels:Array.from({length:7},(_,id)=>({text:id<6?'':'超出上限'}))}
  assert.equal(subject.page.coverMinLabelArea.value,0)
  subject.page.service.value=null
  assert.equal(subject.page.coverMinLabelArea.value,0)
})

/** 为真实发布加载配置当前游戏素材，不模拟页面的默认封面或分类实现。 */
function publishLoadSubject(){const subject=harness('publish');subject.store.state.games=[{id:'delta',categories:['任务协作'],covers:['/static/covers/official/delta-3.jpg']}];return subject}

test('服务切换局部游戏只更新对应分类底图与失效挂靠，保留其余草稿和首页偏好',()=>{
  const subject=harness('publish');subject.storage.set('jijie_game','delta')
  subject.store.state.games=[{id:'delta',categories:['任务协作'],covers:['/delta.jpg']},{id:'honor',categories:['排位陪练'],covers:['/honor.jpg']},{id:'off',enabled:false,categories:['停用类型'],covers:['/off.jpg']}]
  subject.page.clubs.value=[{id:'delta-club',name:'三角洲俱乐部',status:'active',membership_status:'owner',game_ids:['delta']},{id:'all-club',name:'综合俱乐部',status:'active',membership_status:'active',game_ids:['delta','honor']}]
  Object.assign(subject.page.form,{club_id:'delta-club',labels:[{id:'custom',text:'保留文字',position:'bottom-left',color:'#ffffff',background:'transparent',font_size:16}]})
  const before=JSON.stringify({title:subject.page.form.title,description:subject.page.form.description,price:subject.page.form.price,unit:subject.page.form.unit,labels:subject.page.form.labels,tagsText:subject.page.form.tagsText})
  subject.page.changeGame('honor')
  assert.equal(subject.page.form.game_id,'honor');assert.equal(subject.page.form.category,'排位陪练');assert.equal(subject.page.form.cover_url,'/honor.jpg');assert.equal(subject.page.form.club_id,'')
  assert.equal(JSON.stringify({title:subject.page.form.title,description:subject.page.form.description,price:subject.page.form.price,unit:subject.page.form.unit,labels:subject.page.form.labels,tagsText:subject.page.form.tagsText}),before)
  const switched=JSON.stringify(subject.page.form);subject.page.changeGame('off');subject.page.changeGame('missing');subject.page.changeGame('honor');assert.equal(JSON.stringify(subject.page.form),switched)
  subject.page.form.club_id='all-club';subject.page.changeGame('delta');assert.equal(subject.page.form.club_id,'all-club')
  assert.equal(subject.store.state.gameId,'delta');assert.equal(subject.storage.get('jijie_game'),'delta')
})

/** 提供读取阶段需要回填的真实服务字段，区分旧响应与新草稿。 */
function serviceData(title: string){return{id:'service-A',game_id:'delta',title,description:'按照订单约定完成任务并提交记录。',category:'任务协作',price_cents:3900,unit:'小时',cover_url:'/static/covers/official/delta-3.jpg',labels:[],tags:[],club_id:''}}

/** 提供当前身份确有挂靠权限的俱乐部选项。 */
function clubData(id: string){return{id,name:id,game_ids:['delta'],status:'active',membership_status:'owner'}}

test('发布页切换账号后服务与挂靠的旧成功或旧错误都不能回填',async t=>{
  for(const stage of ['service','clubs'])for(const outcome of ['success','error'])await t.test(stage+'-'+outcome,async()=>{
    const subject=publishLoadSubject()
    if(stage==='service')subject.page.serviceId.value='service-A'
    const loading=subject.page.load();await flush()
    const old=subject.take(stage==='service'?'/services/service-A':'/clubs?mine=1')
    await switchAccount(subject)
    const draft=subject.draft()
    if(outcome==='error')old.reject(new Error('旧身份读取失败'))
    else old.resolve(stage==='service'?serviceData('原身份服务'):[clubData('old-club-A')])
    await loading
    assert.equal(subject.store.state.user.id,'B')
    assert.equal(subject.draft(),draft)
    assert.equal(subject.page.clubs.value.length,0)
    assert.equal(subject.page.clubOptions.value.length,1)
    assert.equal(subject.page.error.value,'')
  })
})

test('发布页隐藏再显示后旧服务或挂靠响应不能覆盖新读取与草稿',async t=>{
  for(const stage of ['service','clubs'])for(const outcome of ['success','error'])await t.test(stage+'-'+outcome,async()=>{
    const subject=publishLoadSubject();subject.page.serviceId.value='service-A'
    const reading=subject.page.load();await flush()
    let old=subject.take('/services/service-A')
    if(stage==='clubs'){old.resolve(serviceData('隐藏前服务'));await flush();old=subject.take('/clubs?mine=1')}
    subject.lifecycle.hide();subject.lifecycle.show();await flush()
    subject.take('/services/service-A').resolve(serviceData('重新读取的服务'));await flush()
    subject.take('/clubs?mine=1').resolve([clubData('current-club-A')]);await flush()
    subject.page.form.title='重显后尚未保存的草稿';subject.page.form.club_id='current-club-A'
    const draft=subject.draft()
    if(outcome==='error')old.reject(new Error('隐藏期间的旧错误'))
    else old.resolve(stage==='service'?serviceData('迟到旧服务'):[clubData('outdated-club-A')])
    await reading
    assert.equal(subject.draft(),draft)
    assert.equal(subject.page.clubs.value[0].id,'current-club-A')
    assert.equal(subject.page.error.value,'')
    assert.equal(subject.requests.filter(request=>request.path==='/services/service-A').length,2)
    assert.equal(subject.requests.filter(request=>request.path==='/clubs?mine=1').length,stage==='clubs'?2:1)
  })
})

test('发布页读取完成后同身份隐藏返回保留全部草稿且不重新初始化',async()=>{
  const subject=publishLoadSubject(),loading=subject.page.load();await flush()
  subject.take('/clubs?mine=1').resolve([clubData('club-A')]);await loading
  Object.assign(subject.page.form,{title:'尚未保存的标题',club_id:'club-A',tagsText:'草稿标签',labels:[{text:'独立文案',position:'top-right',font_size:24}]})
  const draft=subject.draft(),requestCount=subject.requests.length
  subject.lifecycle.hide();subject.lifecycle.show();await flush()
  assert.equal(subject.draft(),draft)
  assert.equal(subject.requests.slice(requestCount).every(request=>request.path==='/wallet'||request.path==='/clubs/club-A/deposits'),true)
  assert.equal(subject.requests.filter(request=>request.path==='/clubs?mine=1').length,1)
})

test('发布页隐藏期间换账号，重显只接收新账号挂靠且旧响应不清除新选择',async()=>{
  const subject=publishLoadSubject(),reading=subject.page.load();await flush()
  const old=subject.take('/clubs?mine=1')
  subject.lifecycle.hide();await switchAccount(subject);subject.lifecycle.show();await flush()
  subject.take('/clubs?mine=1').resolve([clubData('club-B')]);await flush()
  subject.page.form.club_id='club-B'
  old.resolve([clubData('old-club-A')]);await reading
  assert.equal(subject.page.clubs.value[0].id,'club-B')
  assert.equal(subject.page.form.club_id,'club-B')
  assert.equal(subject.page.clubOptions.value.some((club:{id:string})=>club.id==='old-club-A'),false)
})

test('发布页首次加载紧接显示生命周期不会重复读取',async()=>{
  const subject=publishLoadSubject()
  subject.lifecycle.load({id:'service-A'});subject.lifecycle.show();await flush()
  assert.equal(subject.requests.filter(request=>request.path==='/services/service-A').length,1)
  subject.take('/services/service-A').resolve(serviceData('首屏服务'));await flush()
  subject.take('/clubs?mine=1').resolve([]);await flush()
  assert.equal(subject.page.form.title,'首屏服务')
})


test('服务详情只展示后端有效平台押金，未知或退款后零金额隐藏',()=>{
  const subject=harness('purchase')
  for(const amount of [undefined,0,-1,NaN,Infinity,1.5]){
    subject.page.service.value={provider_deposit_cents:amount}
    assert.equal(subject.page.providerDepositCents.value,0)
  }
  subject.page.service.value={provider_deposit_cents:20000}
  assert.equal(subject.page.providerDepositCents.value,20000)
  subject.page.service.value={provider_deposit_cents:0}
  assert.equal(subject.page.providerDepositCents.value,0)
})

test('编辑预览只使用本人服务端押金，草稿不能伪造或提交押金标记',async()=>{
  const subject=publishLoadSubject();subject.page.serviceId.value='service-A'
  const loading=subject.page.load();await flush()
  subject.take('/services/service-A').resolve({...serviceData('本人服务'),provider_id:'A',provider_deposit_cents:20000});await flush()
  subject.take('/clubs?mine=1').resolve([]);await loading
  assert.equal(subject.page.previewService.value.provider_deposit_cents,20000)
  subject.page.form.provider_deposit_cents=99999999
  assert.equal(subject.page.previewService.value.provider_deposit_cents,20000)
  const saving=subject.page.submit()
  subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
  const writing=subject.take('/services/service-A','PATCH')
  assert.equal('provider_deposit_cents' in writing.body,false)
  assert.equal('provider_club_deposit_cents' in writing.body,false)
  writing.resolve({id:'service-A'});await saving
})

test('发布预览换身份立即清除已读押金，旧身份迟到响应不能恢复标识',async()=>{
  const subject=publishLoadSubject();subject.page.serviceId.value='service-A'
  const loading=subject.page.load();await flush()
  const old=subject.take('/services/service-A')
  await switchAccount(subject)
  old.resolve({...serviceData('旧身份服务'),provider_id:'A',provider_deposit_cents:20000});await loading
  assert.equal(subject.page.previewService.value.provider_deposit_cents,0)
  assert.equal(subject.store.state.user.id,'B')
})

test('新建服务完整预览读取本人两类实缴押金，不使用规则或组织平台押金',async()=>{
  const subject=publishLoadSubject(),loading=subject.page.load();await flush()
  subject.take('/clubs?mine=1').resolve([clubData('club-A')]);await loading
  subject.take('/wallet').resolve({deposit_cents:12500,deposit_qualified:true});await flush()
  assert.equal(subject.page.previewService.value.provider_deposit_cents,12500)
  subject.page.form.club_id='club-A'
  subject.take('/clubs/club-A/deposits').resolve({platform:{amount_cents:999999},membership:{amount_cents:23000,required_cents:50000}});await flush()
  assert.equal(subject.page.previewService.value.provider_club_deposit_cents,23000)
  subject.page.form.provider_deposit_cents=999999;subject.page.form.provider_club_deposit_cents=999999
  assert.equal(subject.page.previewService.value.provider_deposit_cents,12500);assert.equal(subject.page.previewService.value.provider_club_deposit_cents,23000)
  const source=readFileSync(new URL('../pages/services/publish.vue',import.meta.url),'utf8')
  assert.match(source,/<ServiceCard :service="previewService" :interactive="false"\s*\/>/)
  assert.doesNotMatch(source,/cover-only/)
})

test('发布预览切换俱乐部或账号后，迟到押金响应不能恢复旧徽标',async()=>{
  const subject=publishLoadSubject(),loading=subject.page.load();await flush()
  subject.take('/clubs?mine=1').resolve([clubData('club-A'),clubData('club-B')]);await loading
  const oldWallet=subject.take('/wallet')
  subject.page.form.club_id='club-A';const oldClub=subject.take('/clubs/club-A/deposits')
  subject.page.form.club_id='club-B';assert.equal(subject.page.previewService.value.provider_club_deposit_cents,0)
  subject.take('/clubs/club-B/deposits').resolve({membership:{amount_cents:34000}});await flush()
  oldClub.resolve({membership:{amount_cents:12000}});await flush()
  assert.equal(subject.page.previewService.value.provider_club_deposit_cents,34000)
  await switchAccount(subject);assert.equal(subject.page.previewService.value.provider_deposit_cents,0);assert.equal(subject.page.previewService.value.provider_club_deposit_cents,0)
  oldWallet.resolve({deposit_cents:99999});await flush()
  assert.equal(subject.page.previewService.value.provider_deposit_cents,0);assert.equal(subject.page.previewService.value.provider_club_deposit_cents,0)
})


test('发布预览不把低于最低额或缺少资格字段的保留押金展示为保障标识',async()=>{
  for(const qualified of [false,undefined]){const subject=publishLoadSubject(),loading=subject.page.load();await flush();subject.take('/clubs?mine=1').resolve([]);await loading;subject.take('/wallet').resolve({deposit_cents:6000,deposit_required_cents:10000,deposit_shortfall_cents:4000,deposit_qualified:qualified});await flush();assert.equal(subject.page.previewService.value.provider_deposit_cents,0)}
})
