/** 文件作用：执行真实俱乐部目录脚本，验证筛选、账号切换与离页后的延迟列表和创建结果不会覆盖当前身份。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch, effectScope } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

const storeSource=readFileSync(new URL('./store.ts',import.meta.url),'utf8')
const pageSource=pageScript('community/clubs/index.vue')
const accountA={id:'A',nickname:'俱乐部申请者'}
const accountB={id:'B',nickname:'另一位申请者'}

/** 只控制网络完成次序，保留真实业务API、Vue侦听和共享会话行为。 */
function harness(t: TestContext){
  const storage=new Map<string,string>([['jijie_token','token-A']]),requests: DeferredRequest[]=[],notices: unknown[]=[],navigations: string[]=[]
  const lifecycle: PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({
    reactive,ref,computed,watch,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',
    /** 每次请求都由用例显式结束，迟到响应不会被模拟器自动丢弃。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
    notify:(value:unknown)=>notices.push(value),go:(value:string)=>navigations.push(value),
    onLoad:(callback:PageLifecycle['load'])=>{lifecycle.load=callback},onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource)+'\nglobalThis.store={state,login,clearSession}',context)
  context.store.state.ready=true;context.store.state.user={...accountA}
  const scope=effectScope()
  scope.run(()=>vm.runInContext(executable(pageSource)+'\nglobalThis.page={load,create,openCreate,clubs,query,mine,loading,error,creating,busy,form}',context))
  t.after(()=>scope.stop())
  return{store:context.store,page:context.page,requests,notices,navigations,lifecycle,
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
    const old=subject.take('/clubs?q='+encodeURIComponent('山海'))
    subject.page.query.value='云上';const latest=subject.page.load();await flush()
    subject.take('/clubs?q='+encodeURIComponent('云上')).resolve([{id:'latest'}]);await latest
    if(oldResult==='success')old.resolve([{id:'old'}]);else old.reject(new Error('过期筛选错误'))
    await first
    assert.equal(subject.page.clubs.value[0].id,'latest')
    assert.equal(subject.page.error.value,'')
    assert.equal(subject.page.loading.value,false)
  })
})

test('我的俱乐部切换账号立即清空原资料，旧响应不回填新身份',async t=>{
  const subject=harness(t);subject.page.mine.value=true
  const initial=subject.page.load();await flush();subject.take('/clubs?mine=1').resolve([{id:'A-loaded'}]);await initial
  const refreshing=subject.page.load();await flush();const old=subject.take('/clubs?mine=1')
  prepareDraft(subject)
  await switchAccount(subject)
  assert.equal(subject.page.clubs.value.length,0)
  assert.equal(subject.page.creating.value,false)
  assert.equal(subject.page.form.name,'')
  assert.equal(subject.page.form.game_ids.length,0)
  const latest=subject.take('/clubs?mine=1')
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
    const oldCreating=subject.page.create(),old=subject.take('/clubs','POST')
    await subject.page.create()
    assert.equal(subject.requests.filter(item=>item.method==='POST').length,1,'连续点击只创建一次')
    await switchAccount(subject)
    subject.take('/clubs').resolve([]);await flush()
    prepareDraft(subject,'新账号俱乐部')
    const creating=subject.page.create(),latest=subject.take('/clubs','POST')
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
    const subject=harness(child),loading=subject.page.load();await flush();const oldList=subject.take('/clubs')
    prepareDraft(subject);const creating=subject.page.create(),oldCreation=subject.take('/clubs','POST')
    subject.lifecycle[boundary]()
    oldList.resolve([{id:'late-list'}]);oldCreation.resolve({id:'late-club'});await Promise.all([loading,creating])
    assert.equal(subject.page.clubs.value.length,0)
    assert.equal(subject.page.busy.value,false)
    assert.deepEqual(subject.navigations,[])
    assert.deepEqual(subject.notices,[])
    if(boundary==='hide'){
      subject.lifecycle.show();await flush();subject.take('/clubs').resolve([]);await flush()
      assert.equal(subject.page.form.name,'山海电竞','同一身份返回仍保留未保存草稿')
    }
  })
})

test('同一用户会话更新后旧创建结果被丢弃但提交锁仍正常恢复',async t=>{
  const subject=harness(t);prepareDraft(subject)
  const creating=subject.page.create(),old=subject.take('/clubs','POST')
  const login=subject.store.login('A')
  subject.take('/auth/dev','POST').resolve({token:'refreshed-token-A',user:{...accountA}})
  await login;await flush();old.resolve({id:'stale-session-club'});await creating
  assert.deepEqual(subject.navigations,[])
  assert.deepEqual(subject.notices,[])
  assert.equal(subject.page.busy.value,false)
})
