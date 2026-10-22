/** 文件作用：执行真实手机号门槛和官方弹层脚本，验证跨账号、同账号重开及原生事件迟到的并发隔离。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed,effectScope,nextTick,reactive,ref,watch } from 'vue'
import { executable,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'

/** 只模拟原生窗口和网络完成次序，身份、手机号门槛与组件行为均运行生产代码。 */
function harness(t:TestContext){
  const requests:DeferredRequest[]=[],storage=new Map<string,string>([['jijie_token','token-A']]),popupEvents:string[]=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({reactive,ref,computed,watch,nextTick,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},token:()=>storage.get('jijie_token')||'',go:()=>{},
    onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
    /** 每个鉴权和手机号请求由用例显式完成，迟到结果不会被测试框架提前丢弃。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('./store.ts',import.meta.url),'utf8'))+'\nglobalThis.store={state,login,sessionRevision}',context)
  context.store.state.ready=true;context.store.state.user={id:'A',phone_verified:false}
  vm.runInContext('(function(){'+executable(readFileSync(new URL('./phone.ts',import.meta.url),'utf8'))+';globalThis.gate={requirePhone,finishPhone,isCurrentPhoneGate,phoneGate};globalThis.phoneGate=phoneGate;globalThis.finishPhone=finishPhone;globalThis.isCurrentPhoneGate=isCurrentPhoneGate})()',context)
  const source=readFileSync(new URL('../components/PhoneVerification.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1]
  assert.ok(source)
  const scope=effectScope()
  scope.run(()=>vm.runInContext(executable(source)+'\nglobalThis.component={popup,prepareNative,verifyWechat,verifyDev,popupChanged,syncPopup,nativePending}',context))
  const instance=reactive({showPopup:false,
    /** 原生弹层只记录显示状态，页面代次和取消仍由真实组件处理。 */
    open(){popupEvents.push('open');instance.showPopup=true;context.component.popupChanged({show:true})},
    /** 模拟官方同步关闭事件，避免用真实动画时长代替并发断言。 */
    close(){popupEvents.push('close');context.component.popupChanged({show:false});instance.showPopup=false},
  })
  context.component.popup.value=instance
  t.after(()=>{hooks.unload();scope.stop()})
  return{gate:context.gate,component:context.component,store:context.store,hooks,requests,popupEvents,instance,
    /** 取出实际发出的指定请求，重复或未发起均立即失败。 */
    take(path:string,method='GET'){const item=requests.find(row=>!row.taken&&row.path===path&&row.method===method);assert.ok(item,'缺少请求 '+method+' '+path);item.taken=true;return item},
  }
}

/** 排空真实Vue侦听与网络await，推进到明确的下一次请求。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 刷新用户后打开其手机号门槛，返回尚未授权的真实等待Promise。 */
async function openGate(subject:ReturnType<typeof harness>,id='A'){
  const pending=subject.gate.requirePhone()
  subject.take('/me').resolve({id,phone_verified:false})
  await flush()
  assert.equal(subject.gate.phoneGate.visible,true)
  return{pending,epoch:subject.gate.phoneGate.epoch as number}
}
/** 使用真实登录流程切换用户、令牌和会话修订。 */
async function switchAccount(subject:ReturnType<typeof harness>,id='B'){
  const logging=subject.store.login(id)
  subject.take('/auth/dev','POST').resolve({token:'token-'+id,user:{id,phone_verified:false}})
  await logging;await flush()
}
/** 原生授权先固定弹层代次，再发出服务端一次性code兑换。 */
function beginBinding(subject:ReturnType<typeof harness>,code:string){subject.component.prepareNative();return subject.component.verifyWechat({detail:{code}})}

test('A旧手机号响应不能取消或解锁B正在验证的新弹框',async t=>{
  const subject=harness(t),oldGate=await openGate(subject)
  const oldBinding=beginBinding(subject,'code-A'),old=subject.take('/me/phone/wechat','POST')
  subject.hooks.hide();assert.equal(await oldGate.pending,false)
  await switchAccount(subject);subject.hooks.show();const current=await openGate(subject,'B')
  const newBinding=beginBinding(subject,'code-B'),latest=subject.take('/me/phone/wechat','POST')
  assert.equal(subject.gate.phoneGate.busy,true)
  old.resolve({});await oldBinding
  assert.equal(subject.gate.phoneGate.epoch,current.epoch);assert.equal(subject.gate.phoneGate.visible,true);assert.equal(subject.gate.phoneGate.busy,true)
  assert.equal(subject.requests.filter(item=>item.path==='/me'&&!item.taken).length,0,'旧响应不得刷新新账号资料')
  latest.resolve({});await flush();subject.take('/me').resolve({id:'B',phone_verified:true});await newBinding
  assert.equal(await current.pending,true);assert.equal(subject.gate.phoneGate.visible,false)
})

test('同账号关闭重开也有独立代次，旧成功不得结束新验证',async t=>{
  const subject=harness(t),oldGate=await openGate(subject)
  const oldBinding=beginBinding(subject,'first'),old=subject.take('/me/phone/wechat','POST')
  subject.hooks.hide();assert.equal(await oldGate.pending,false);subject.hooks.show()
  const current=await openGate(subject),newBinding=beginBinding(subject,'second'),latest=subject.take('/me/phone/wechat','POST')
  old.resolve({});await oldBinding
  assert.equal(subject.gate.phoneGate.epoch,current.epoch);assert.equal(subject.gate.phoneGate.busy,true)
  latest.resolve({});await flush();subject.take('/me').resolve({id:'A',phone_verified:true});await newBinding
  assert.equal(await current.pending,true)
})

test('旧手机号兑换失败和finally不能覆盖新弹层错误或解除新请求锁',async t=>{
  const subject=harness(t),oldGate=await openGate(subject)
  const oldBinding=beginBinding(subject,'old-code'),old=subject.take('/me/phone/wechat','POST')
  subject.hooks.hide();await oldGate.pending;await switchAccount(subject);subject.hooks.show()
  const current=await openGate(subject,'B'),newBinding=beginBinding(subject,'new-code'),latest=subject.take('/me/phone/wechat','POST')
  subject.gate.phoneGate.error='B当前提示'
  old.reject(new Error('A旧错误'));await oldBinding
  assert.equal(subject.gate.phoneGate.error,'B当前提示');assert.equal(subject.gate.phoneGate.busy,true)
  latest.reject(new Error('B当前错误'));await newBinding
  assert.equal(subject.gate.phoneGate.error,'B当前错误');assert.equal(subject.gate.phoneGate.visible,true);assert.equal(subject.gate.phoneGate.busy,false)
  subject.gate.finishPhone(false,current.epoch);assert.equal(await current.pending,false)
})

test('旧原生授权事件不能用其code绑定到后来的账号',async t=>{
  const subject=harness(t),oldGate=await openGate(subject)
  subject.component.prepareNative();assert.equal(subject.component.nativePending.value,true)
  subject.hooks.hide();await oldGate.pending;await switchAccount(subject);subject.hooks.show()
  const current=await openGate(subject,'B')
  await subject.component.verifyWechat({detail:{code:'late-native-A'}})
  assert.equal(subject.requests.filter(item=>item.path==='/me/phone/wechat').length,0)
  assert.equal(subject.gate.phoneGate.visible,true);assert.equal(subject.component.nativePending.value,false)
  const newBinding=beginBinding(subject,'native-B'),latest=subject.take('/me/phone/wechat','POST')
  assert.equal(latest.body.code,'native-B')
  latest.reject(new Error('保留B草稿'));await newBinding
  subject.gate.finishPhone(false,current.epoch);assert.equal(await current.pending,false)
})

test('旧代次显式finish和隐藏组件重复hide都不能关闭新的全局门槛',async t=>{
  const subject=harness(t),old=await openGate(subject)
  subject.hooks.hide();assert.equal(await old.pending,false)
  const current=await openGate(subject)
  subject.gate.finishPhone(false,old.epoch);subject.hooks.hide()
  assert.equal(subject.gate.phoneGate.visible,true);assert.equal(subject.gate.phoneGate.epoch,current.epoch)
  subject.gate.finishPhone(false,current.epoch);assert.equal(await current.pending,false)
})

test('延迟手机号预检在会话变化后既不能打开弹框也不能复用新账号门槛',async t=>{
  const subject=harness(t),old=subject.gate.requirePhone(),refresh=subject.take('/me')
  await switchAccount(subject)
  refresh.resolve({id:'A',phone_verified:false});assert.equal(await old,false)
  assert.equal(subject.gate.phoneGate.visible,false)
  const current=await openGate(subject,'B')
  subject.gate.finishPhone(false,current.epoch);assert.equal(await current.pending,false)
})

test('取消后的迟到nextTick不会重新打开官方手机号弹层',async t=>{
  const subject=harness(t),current=await openGate(subject),opened=subject.popupEvents.filter(event=>event==='open').length
  subject.instance.showPopup=false
  const presenting=subject.component.syncPopup()
  subject.gate.finishPhone(false,current.epoch);await current.pending;await presenting;await flush()
  assert.equal(subject.gate.phoneGate.visible,false);assert.equal(subject.instance.showPopup,false)
  assert.equal(subject.popupEvents.filter(event=>event==='open').length,opened)
})
