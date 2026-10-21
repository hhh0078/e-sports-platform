/** 文件作用：执行真实需求发布页、手机号门槛及共享会话，验证授权期间连点、失败恢复和账号切换不会重复或串号创建订单。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'

const storeSource=readFileSync(new URL('./store.ts',import.meta.url),'utf8')
const phoneSource=readFileSync(new URL('./phone.ts',import.meta.url),'utf8')
const publishSource=pageScript('requests/publish.vue')
const accountA={id:'A',nickname:'需求发布者',phone_verified:false}
const accountB={id:'B',nickname:'另一位玩家',phone_verified:true}

/** 注入可控制返回顺序的传输边界，保持手机号和提交函数使用真实生产实现。 */
function harness(){
  const storage=new Map<string,string>([['jijie_token','token-A']]),requests:DeferredRequest[]=[],notices:unknown[]=[],navigations:string[]=[]
  const lifecycle:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({
    reactive,ref,computed,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',
    /** 网络调用必须由用例明确完成，重复请求直接进入可断言记录。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
    notify:(value:unknown)=>notices.push(value),go:(value:string)=>navigations.push(value),
    onLoad:(callback:PageLifecycle['load'])=>{lifecycle.load=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(storeSource)+'\nglobalThis.store={state,login,refreshMe,sessionRevision}',context)
  context.store.state.ready=true;context.store.state.user={...accountA}
  vm.runInContext('(function(){'+executable(phoneSource)+';globalThis.requirePhone=requirePhone;globalThis.phone={phoneGate,finishPhone}})()',context)
  vm.runInContext(executable(publishSource)+'\nglobalThis.page={submit,title,requirements,price,busy,error}',context)
  context.page.title.value='  三角洲双排陪练  ';context.page.requirements.value=' 晚上八点，耐心教学一小时 ';context.page.price.value='34.90'
  return{
    store:context.store,page:context.page,phone:context.phone,requests,notices,navigations,lifecycle,
    /** 只取得一次实际发出的请求，防止用例错误地忽略重复调用。 */
    take(path:string,method='GET'){const request=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,`缺少请求 ${method} ${path}`);request.taken=true;return request},
  }
}

/** 排空微任务，使真实 await 推进至下一次受控网络边界。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}

/** 完成绑定后以服务端刷新结果结束真实手机号门槛。 */
async function authorize(subject:ReturnType<typeof harness>){const refreshing=subject.store.refreshMe();subject.take('/me').resolve({...accountA,phone_verified:true});await refreshing;subject.phone.finishPhone(true);await flush()}

/** 通过真实登录流程改变令牌、用户和会话版本。 */
async function switchAccount(subject:ReturnType<typeof harness>){const login=subject.store.login('B');subject.take('/auth/dev','POST').resolve({token:'token-B',user:{...accountB}});await login}

test('手机号授权前后连续提交只创建一笔需求订单',async()=>{
  const subject=harness(),first=subject.page.submit(),second=subject.page.submit()
  assert.equal(subject.page.busy.value,true)
  assert.equal(subject.requests.filter(item=>item.path==='/me').length,1)
  subject.take('/me').resolve({...accountA});await flush()
  assert.equal(subject.phone.phoneGate.visible,true)
  await subject.page.submit()
  assert.equal(subject.requests.filter(item=>item.path==='/requests').length,0)
  await authorize(subject)
  const creating=subject.take('/requests','POST')
  assert.equal(creating.body.amount_cents,3490)
  assert.equal(creating.body.title,'三角洲双排陪练')
  await subject.page.submit()
  assert.equal(subject.requests.filter(item=>item.path==='/requests').length,1)
  creating.resolve({id:'single-order'});await Promise.all([first,second])
  assert.equal(subject.page.busy.value,false)
  assert.deepEqual(subject.navigations,['/pages/orders/detail?id=single-order'])
})

test('关闭手机号授权不创建订单并允许重新提交',async()=>{
  const subject=harness(),cancelled=subject.page.submit()
  subject.take('/me').resolve({...accountA});await flush();subject.phone.finishPhone(false);await cancelled
  assert.equal(subject.requests.filter(item=>item.path==='/requests').length,0)
  assert.equal(subject.page.busy.value,false)
  const retry=subject.page.submit();subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
  subject.take('/requests','POST').resolve({id:'retry-order'});await retry
  assert.deepEqual(subject.navigations,['/pages/orders/detail?id=retry-order'])
})

test('手机号状态读取失败被页面处理且提交锁恢复',async()=>{
  const subject=harness(),submitting=subject.page.submit()
  subject.take('/me').reject(new Error('手机号状态读取失败'));await submitting
  assert.equal(subject.page.busy.value,false)
  assert.equal(subject.page.error.value,'手机号状态读取失败')
  assert.equal(subject.requests.filter(item=>item.path==='/requests').length,0)
  assert.equal(subject.notices.length,1)
})

test('授权等待期间切换账号不能替新账号创建旧草稿',async()=>{
  const subject=harness(),submitting=subject.page.submit()
  subject.take('/me').resolve({...accountA});await flush();await switchAccount(subject)
  subject.phone.finishPhone(true);await submitting
  assert.equal(subject.requests.filter(item=>item.path==='/requests').length,0)
  assert.equal(subject.store.state.user.id,'B')
  assert.deepEqual(subject.navigations,[])
  assert.equal(subject.page.busy.value,false)
})

test('创建订单的旧结果不能把新账号或已销毁页面跳转到旧订单',async t=>{
  for(const action of ['switch','unload'])await t.test(action,async()=>{
    const subject=harness(),submitting=subject.page.submit()
    subject.take('/me').resolve({...accountA,phone_verified:true});await flush()
    const creating=subject.take('/requests','POST')
    if(action==='switch')await switchAccount(subject);else subject.lifecycle.unload()
    creating.resolve({id:'old-order'});await submitting
    assert.deepEqual(subject.navigations,[])
    assert.equal(subject.page.busy.value,false)
  })
})
