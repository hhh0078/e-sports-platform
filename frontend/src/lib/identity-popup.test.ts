/** 文件作用：运行真实身份弹层与状态逻辑，验证未成年零请求、错误展示、提示排队及离开清理。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {computed,effectScope,nextTick,reactive,ref,watch} from 'vue'
import {validateIdentityInput} from './identity-card'
import {designRpx} from './design-units'
import {executable,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest} from './sfc-test'

/** 按出生日期生成仅用于离线测试的格式有效号码。 */
function fixture(birthday:string):string{const raw='110101'+birthday+'001',weights=[7,9,10,5,8,4,2,1,6,3,7,9,10,5,8,4,2];return raw+'10X98765432'[weights.reduce((sum,value,index)=>sum+Number(raw[index])*value,0)%11]}
/** 直接运行生产组件，控制官方弹层、服务器与系统提示槽。 */
function harness(t:TestContext,remind=false){
  const storage=new Map<string,unknown>(),requests:DeferredRequest[]=[],hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  const state=reactive({ready:true,user:{id:'A',is_minor:true}}),status={financialBusy:false,owner:null as string|null},listeners=new Set<()=>void>()
  let revision=0
  const context=vm.createContext({reactive,ref,computed,watch,nextTick,validateIdentityInput,designRpx,state,sessionRevision:()=>revision,token:()=>state.user.id+'-token',pageTheme:()=>({'--accent':'#8de8b6'}),go:()=>{},isTimelinePreview:()=>false,defineProps:()=>({remind}),withDefaults:(value:object,defaults:object)=>({...defaults,...value}),getUpdatePromptState:()=>({...status}),onUpdatePromptStateChange:(listener:()=>void)=>{listeners.add(listener);return()=>listeners.delete(listener)},acquireSystemPrompt:(kind:string)=>{if(status.owner)return null;status.owner=kind;let released=false;return()=>{if(!released){released=true;status.owner=null}}},
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:unknown)=>storage.set(key,value)},
    onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
    api:(path:string,method='GET',body:Record<string,unknown>={})=>new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false})),
  })
  injectBusinessAPIs(context)
  vm.runInContext('(function(){'+executable(readFileSync(new URL('./identity.ts',import.meta.url),'utf8'))+';Object.assign(globalThis,{identityGate,isCurrentIdentityGate,finishIdentity,acceptIdentityResult,requestIdentityReminder,markIdentityPresented,requireIdentity});globalThis.gate={identityGate,requireIdentity,finishIdentity}})()',context)
  const source=readFileSync(new URL('../components/IdentityVerification.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(source)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(source)+'\nglobalThis.component={popup,formOpen,name,idcard,consent,submit,close,begin,popupChanged,syncPopup}',context))
  const popup=reactive({showPopup:false,
    /** 官方打开事件代表真实展示，触发生产组件的首次提醒标记。 */
    open(){popup.showPopup=true;context.component.popupChanged({show:true})},
    /** 官方关闭同步触发清理，避免等待动画时间掩盖状态问题。 */
    close(){popup.showPopup=false;context.component.popupChanged({show:false})},
  })
  context.component.popup.value=popup;t.after(()=>{hooks.unload();scope.stop()})
  return{gate:context.gate,component:context.component,popup,storage,requests,hooks,status,
    /** 通知组件系统提示槽或资金状态已有变化。 */
    tick(){for(const listener of listeners)listener()},
    /** 真实身份变化使尚未完成的核验结果失效。 */
    switchAccount(){revision++;state.user={id:'B',is_minor:true}},
    /** 定位真实业务请求，未实际发起时立即失败。 */
    take(path:string){const request=requests.find(row=>!row.taken&&row.path===path);assert.ok(request,'缺少请求 '+path);request.taken=true;return request},
  }
}
/** 排空组件侦听和nextTick。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 从真实交易预检打开身份表单，并返回等待用户核验的交易Promise。 */
async function openRequired(h:ReturnType<typeof harness>){const pending=h.gate.requireIdentity();h.take('/me/identity').resolve({is_minor:true,verified:false});await flush();assert.equal(h.popup.showPopup,true);return{pending}}

test('首次软提醒在资金busy期间排队，真正展示后才记已读并释放槽',async t=>{
  const h=harness(t,true);h.status.financialBusy=true;await flush();assert.equal(h.popup.showPopup,false);assert.equal(h.storage.size,0)
  h.status.financialBusy=false;h.tick();await flush();assert.equal(h.popup.showPopup,true);assert.equal(h.storage.get('jijie_identity_reminded'),true);assert.equal(h.status.owner,'identity')
  h.component.close();await flush();assert.equal(h.status.owner,null);assert.equal(h.popup.showPopup,false)
})
test('强制身份表单允许在自身资金锁下打开，已有公告占槽则等待',async t=>{
  const h=harness(t);h.status.financialBusy=true;h.status.owner='announcement'
  const pending=h.gate.requireIdentity();h.take('/me/identity').resolve({is_minor:true,verified:false});await flush();assert.equal(h.popup.showPopup,false)
  h.status.owner=null;h.tick();await flush();assert.equal(h.popup.showPopup,true);assert.equal(h.component.formOpen.value,true)
  h.component.close();assert.equal(await pending,false);await flush();assert.equal(h.status.owner,null)
})
test('未满18岁及未同意都不发核验请求，错误留在弹框下方',async t=>{
  const h=harness(t),gate=await openRequired(h);h.component.name.value='测试姓名';h.component.idcard.value=fixture('20150101');h.component.consent.value=true
  await h.component.submit();assert.match(h.gate.identityGate.error,/您未满18岁/);assert.equal(h.requests.filter(row=>row.path.endsWith('/verify')).length,0)
  h.component.idcard.value=fixture('19930101');h.component.consent.value=false;await h.component.submit();assert.match(h.gate.identityGate.error,/同意/);assert.equal(h.requests.filter(row=>row.path.endsWith('/verify')).length,0)
  h.component.close();assert.equal(await gate.pending,false)
})
test('供应商msg错误在弹框保留，同账号可重试并且成功才继续原交易',async t=>{
  const h=harness(t),gate=await openRequired(h);h.component.name.value='测试姓名';h.component.idcard.value=fixture('19930101');h.component.consent.value=true
  const first=h.component.submit();h.take('/me/identity/verify').reject(new Error('姓名与身份证不一致'));await first
  assert.equal(h.gate.identityGate.error,'姓名与身份证不一致');assert.equal(h.component.name.value,'测试姓名');assert.equal(h.gate.identityGate.busy,false)
  const retry=h.component.submit();h.take('/me/identity/verify').resolve({is_minor:false,verified:true});await retry;assert.equal(await gate.pending,true);await flush();assert.equal(h.component.idcard.value,'');assert.equal(h.status.owner,null)
})
test('隐藏立即清除身份证并释放提示槽，迟到核验结果不能放行新账号',async t=>{
  const h=harness(t),gate=await openRequired(h);h.component.name.value='测试姓名';h.component.idcard.value=fixture('19930101');h.component.consent.value=true
  const verifying=h.component.submit(),request=h.take('/me/identity/verify');h.hooks.hide();assert.equal(await gate.pending,false);assert.equal(h.component.name.value,'');assert.equal(h.component.idcard.value,'');assert.equal(h.status.owner,null)
  h.switchAccount();request.resolve({is_minor:false,verified:true});await verifying;assert.equal(h.gate.identityGate.visible,false)
})
