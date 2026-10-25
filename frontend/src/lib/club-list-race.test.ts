/** 文件作用：执行真实俱乐部目录脚本，验证筛选、账号切换与离页后的延迟列表和创建结果不会覆盖当前身份。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch, effectScope } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { NativePageRefreshOptions } from './native-page-refresh'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

const storeSource=readFileSync(new URL('./store.ts',import.meta.url),'utf8')
const pageSource=pageScript('community/clubs/index.vue')
const accountA={id:'A',nickname:'俱乐部申请者'}
const accountB={id:'B',nickname:'另一位申请者'}

/** 只控制网络完成次序，保留真实业务API、Vue侦听和共享会话行为。 */
function harness(t: TestContext,manualManaged=false){
  const storage=new Map<string,string>([['jijie_token','token-A']]),requests: DeferredRequest[]=[],notices: unknown[]=[],navigations: string[]=[]
  const native:{options?:NativePageRefreshOptions}={}
  const lifecycle: PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({
    reactive,ref,computed,watch,useNativePageRefresh:(options:NativePageRefreshOptions)=>{native.options=options;return{}},requirePhone:async()=>true,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',
    /** 每次请求都由用例显式结束，迟到响应不会被模拟器自动丢弃。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){if(path.startsWith('/clubs?managed=1')&&method==='GET'&&!manualManaged)return Promise.resolve([]);if(path==='/financial-rules'&&method==='GET')return Promise.resolve({free_platform_rate_bps:0,club_platform_rate_bps:0,club_creation_deposit_cents:10000});return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
    notify:(value:unknown)=>notices.push(value),go:(value:string)=>navigations.push(value),
    onLoad:(callback:PageLifecycle['load'])=>{lifecycle.load=callback},onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource)+'\nglobalThis.store={state,login,clearSession}',context)
  context.store.state.ready=true;context.store.state.user={...accountA}
  const scope=effectScope()
  scope.run(()=>vm.runInContext(executable(pageSource)+'\nglobalThis.page={load,retryFailed,create,openCreate,clubs,query,mine,loading,error,creating,busy,form,hasMore,loadManaged,managedClubs,managementMore,managementPopup,openManagement}',context))
  t.after(()=>scope.stop())
  return{store:context.store,page:context.page,requests,notices,navigations,lifecycle,native,
    /** 取得实际发出的单次请求，重复或缺失调用直接失败。 */
    take(path:string,method='GET'){const request=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,`缺少请求 ${method} ${path}`);request.taken=true;return request},
  }
}

/** 排空Vue侦听与实际await，推进至下一个可控请求。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}

/** 通过实际登录函数同时改变用户、令牌和会话修订号。 */
async function switchAccount(subject: ReturnType<typeof harness>){const changing=subject.store.login('B');subject.take('/auth/dev','POST').resolve({token:'token-B',user:{...accountB}});await changing;await flush()}

/** 填写可提交的实际创建草稿，并打开真实创建弹层。 */
function prepareDraft(subject: ReturnType<typeof harness>,name='山海电竞'){subject.page.openCreate();Object.assign(subject.page.form,{name,description:'欢迎认真履约的游戏爱好者',game_ids:['delta']})}

test('俱乐部筛选只接收最新请求，旧成功和旧错误均不能覆盖新列表',async t=>{
  for(const oldResult of ['success','error'])await t.test(oldResult,async child=>{
    const subject=harness(child)
    subject.page.query.value='山海';const first=subject.page.load();await flush()
    const old=subject.take('/clubs?q='+encodeURIComponent('山海')+'&limit=20&offset=0')
    subject.page.query.value='云上';const latest=subject.page.load();await flush()
    subject.take('/clubs?q='+encodeURIComponent('云上')+'&limit=20&offset=0').resolve([{id:'latest'}]);await latest
    if(oldResult==='success')old.resolve([{id:'old'}]);else old.reject(new Error('过期筛选错误'))
    await first
    assert.equal(subject.page.clubs.value[0].id,'latest')
    assert.equal(subject.page.error.value,'')
    assert.equal(subject.page.loading.value,false)
  })
})

test('我的俱乐部切换账号立即清空原资料，旧响应不回填新身份',async t=>{
  const subject=harness(t);subject.page.mine.value=true
  const initial=subject.page.load();await flush();subject.take('/clubs?mine=1&limit=20&offset=0').resolve([{id:'A-loaded'}]);await initial
  const refreshing=subject.page.load();await flush();const old=subject.take('/clubs?mine=1&limit=20&offset=0')
  prepareDraft(subject)
  await switchAccount(subject)
  assert.equal(subject.page.clubs.value.length,0)
  assert.equal(subject.page.creating.value,false)
  assert.equal(subject.page.form.name,'')
  assert.equal(subject.page.form.game_ids.length,0)
  const latest=subject.take('/clubs?mine=1&limit=20&offset=0')
  old.resolve([{id:'old-A'}]);await refreshing
  assert.equal(subject.page.clubs.value.length,0)
  assert.equal(subject.page.loading.value,true,'旧请求结束不能解除新请求的加载状态')
  latest.resolve([{id:'B-loaded'}]);await flush()
  assert.equal(subject.page.clubs.value[0].id,'B-loaded')
  subject.store.clearSession();await flush()
  assert.equal(subject.page.clubs.value.length,0)
  assert.equal(subject.requests.filter(item=>!item.taken).length,0,'访客不请求本人挂靠')
})

test('俱乐部旧创建的成功或失败不能导航、关闭或解锁新身份的申请',async t=>{
  for(const oldResult of ['success','error'])await t.test(oldResult,async child=>{
    const subject=harness(child);prepareDraft(subject,'原账号俱乐部')
    const oldCreating=subject.page.create();await flush();const old=subject.take('/clubs','POST')
    await subject.page.create()
    assert.equal(subject.requests.filter(item=>item.method==='POST').length,1,'连续点击只创建一次')
    await switchAccount(subject)
    subject.take('/clubs?limit=20&offset=0').resolve([]);await flush()
    prepareDraft(subject,'新账号俱乐部')
    const creating=subject.page.create();await flush();const latest=subject.take('/clubs','POST')
    assert.equal(latest.body.name,'新账号俱乐部')
    if(oldResult==='success')old.resolve({id:'old-A'});else old.reject(new Error('原账号申请失败'))
    await oldCreating
    assert.deepEqual(subject.navigations,[])
    assert.deepEqual(subject.notices,[])
    assert.equal(subject.page.creating.value,true)
    assert.equal(subject.page.busy.value,true)
    assert.equal(subject.page.form.name,'新账号俱乐部')
    latest.resolve({id:'new-B'});await creating
    assert.deepEqual(subject.navigations,['/pages/community/clubs/detail?id=new-B'])
    assert.equal(subject.page.busy.value,false)
  })
})

test('离开俱乐部目录后在途读取与创建结果均不再影响界面',async t=>{
  for(const boundary of ['hide','unload'] as const)await t.test(boundary,async child=>{
    const subject=harness(child),loading=subject.page.load();await flush();const oldList=subject.take('/clubs?limit=20&offset=0')
    prepareDraft(subject);const creating=subject.page.create();await flush();const oldCreation=subject.take('/clubs','POST')
    subject.lifecycle[boundary]()
    oldList.resolve([{id:'late-list'}]);oldCreation.resolve({id:'late-club'});await Promise.all([loading,creating])
    assert.equal(subject.page.clubs.value.length,0)
    assert.equal(subject.page.busy.value,false)
    assert.deepEqual(subject.navigations,[])
    assert.deepEqual(subject.notices,[])
    if(boundary==='hide'){
      subject.lifecycle.show();await flush();subject.take('/clubs?limit=20&offset=0').resolve([]);await flush()
      assert.equal(subject.page.form.name,'山海电竞','同一身份返回仍保留未保存草稿')
    }
  })
})

test('同一用户会话更新后旧创建结果被丢弃但提交锁仍正常恢复',async t=>{
  const subject=harness(t);prepareDraft(subject)
  const creating=subject.page.create();await flush();const old=subject.take('/clubs','POST')
  const login=subject.store.login('A')
  subject.take('/auth/dev','POST').resolve({token:'refreshed-token-A',user:{...accountA}})
  await login;await flush();old.resolve({id:'stale-session-club'});await creating
  assert.deepEqual(subject.navigations,[])
  assert.deepEqual(subject.notices,[])
  assert.equal(subject.page.busy.value,false)
})


test('创建草稿响应丢失时复用请求号，待缴状态进入组织资金页',async t=>{
  const subject=harness(t);prepareDraft(subject)
  const first=subject.page.create();await flush();const original=subject.take('/clubs','POST');assert.ok(original.body.request_id);original.reject(new Error('响应丢失'));await first
  const retry=subject.page.create();await flush();const second=subject.take('/clubs','POST');assert.equal(second.body.request_id,original.body.request_id)
  second.resolve({id:'draft-A',status:'pending_deposit'});await retry
  assert.equal(subject.navigations.at(-1),'/pages/community/clubs/funds?id=draft-A')
  assert.equal(subject.page.creating.value,false)
})


test('创建俱乐部明确检查名称和介绍码点下限，失败不请求、不丢草稿',async t=>{
  for(const example of [{name:'山',description:'完整的团队介绍至少十个字',message:/名称需要2/},{name:'山海电竞',description:'太短',message:/介绍需要10/}])await t.test(example.name,async child=>{
    const subject=harness(child);prepareDraft(subject);Object.assign(subject.page.form,example)
    await subject.page.create()
    assert.equal(subject.requests.some(item=>item.method==='POST'),false)
    assert.match(String(subject.notices.at(-1)),example.message)
    assert.equal(subject.page.creating.value,true)
    assert.equal(subject.page.form.name,example.name)
  })
})

test('创建介绍与编辑一致支持1000码点，扩展字符按单个码点计数',async t=>{
  const subject=harness(t);prepareDraft(subject)
  Object.assign(subject.page.form,{name:'😀😀',description:'游'.repeat(1000)})
  const creating=subject.page.create();await flush()
  const write=subject.take('/clubs','POST');assert.equal(write.body.description,'游'.repeat(1000));assert.equal(write.body.name,'😀😀')
  write.resolve({id:'unicode-club'});await creating
})

test('俱乐部触底按原始分页前进且下拉覆盖迟到的旧追加页',async t=>{
  const s=harness(t),initial=s.page.load();await flush();s.take('/clubs?limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>({id:'club'+i})));await initial
  const second=s.native.options?.more?.();await flush();s.take('/clubs?limit=20&offset=20').resolve(Array.from({length:20},(_,i)=>({id:'club'+(i+19)})));await second;assert.equal(s.page.clubs.value.length,39)
  const third=s.native.options?.more?.();await flush();const old=s.take('/clubs?limit=20&offset=40')
  const refreshing=s.native.options?.refresh();await flush();s.take('/clubs?limit=20&offset=0').resolve([{id:'fresh'}]);await refreshing
  old.resolve([{id:'stale'}]);await third;assert.deepEqual([...s.page.clubs.value.map((item:{id:string})=>item.id)],['fresh']);assert.equal(s.page.hasMore.value,false)
})

test('俱乐部触底不提交未确认搜索，明确搜索不重建创建草稿',async t=>{
  const s=harness(t),initial=s.page.load();await flush();s.take('/clubs?limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>({id:'club'+i})));await initial
  prepareDraft(s);const draft=JSON.stringify(s.page.form);assert.equal(s.native.options?.blocked?.(),true)
  s.page.creating.value=false;s.page.query.value='新游戏';await s.native.options?.more?.();assert.equal(s.requests.filter(item=>!item.taken).length,0);assert.equal(s.page.clubs.value.length,20)
  const refreshing=s.page.load();await flush();s.take('/clubs?q='+encodeURIComponent('新游戏')+'&limit=20&offset=0').resolve([{id:'new-game'}]);await refreshing
  assert.equal(JSON.stringify(s.page.form),draft);assert.deepEqual([...s.page.clubs.value.map((item:{id:string})=>item.id)],['new-game'])
})

test('俱乐部目录失败重试保留原首读和追加偏移，不被搜索草稿改写',async t=>{
  const s=harness(t),initial=s.page.load();await flush();s.take('/clubs?limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>({id:'club'+i})));await initial
  const refresh=s.native.options?.refresh();await flush();s.take('/clubs?limit=20&offset=0').reject(new Error('首读失败'));await refresh
  const firstRetry=s.page.retryFailed();await flush();s.take('/clubs?limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>({id:'club'+i})));await firstRetry
  const more=s.native.options?.more?.();await flush();s.take('/clubs?limit=20&offset=20').reject(new Error('追加失败'));await more;s.page.query.value='未确认草稿'
  const secondRetry=s.page.retryFailed();await flush();s.take('/clubs?limit=20&offset=20').resolve([{id:'last'}]);await secondRetry
  assert.equal(s.page.clubs.value.length,21);assert.equal(s.page.query.value,'未确认草稿')
})

test('俱乐部管理入口使用独立managed范围，一家直达且多家打开官方选择器',async t=>{
  const s=harness(t,true),reading=s.page.loadManaged();await flush();s.take('/clubs?managed=1&limit=20&offset=0').resolve([{id:'deputy-club',management_role:'deputy'}]);await reading;s.page.openManagement();assert.equal(s.navigations[0],'/pages/community/clubs/console?id=deputy-club')
  let opened=0;s.page.managementPopup.value={open:()=>{opened++},close:()=>{}}
  const latest=s.page.loadManaged();await flush();s.take('/clubs?managed=1&limit=20&offset=0').resolve([{id:'own',management_role:'owner'},{id:'deputy',management_role:'deputy'}]);await latest;s.page.openManagement();assert.equal(opened,1)
})

test('本人管理俱乐部超过20家按服务端分页，切号后旧管理权不回填',async t=>{
  const s=harness(t,true),reading=s.page.loadManaged();await flush();s.take('/clubs?managed=1&limit=20&offset=0').resolve(Array.from({length:20},(_,i)=>({id:'managed-'+i,management_role:'owner'})));await reading
  const next=s.page.loadManaged(true);await flush();const old=s.take('/clubs?managed=1&limit=20&offset=20');await switchAccount(s);assert.equal(s.page.managedClubs.value.length,0);old.resolve([{id:'old-managed'}]);await next;assert.equal(s.page.managedClubs.value.length,0)
})

test('俱乐部介绍创建保留正文内部换行与空行',async t=>{
  const s=harness(t);prepareDraft(s);s.page.form.description='  第一段团队介绍\n\n第二段履约说明  ';const creating=s.page.create();await flush();const writing=s.take('/clubs','POST');assert.equal(writing.body.description,'第一段团队介绍\n\n第二段履约说明');writing.resolve({id:'multiline-club',status:'pending_deposit'});await creating
})
