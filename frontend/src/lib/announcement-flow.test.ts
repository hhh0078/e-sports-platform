/** 文件作用：执行真实公告SFC和版本协调器，验证已读时点、账号版本隔离、关闭动画、更新与资金互斥及生命周期清理。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {effectScope,nextTick,reactive,ref,watch} from 'vue'
import {executable} from './sfc-test'
import type {ContentAnnouncement,ContentTarget} from '../types/content'
import type {WechatUpdateManager,UpdateRuntime} from './update-manager'

/** 排空真实Vue和Promise微任务，不依赖动画时长。 */
async function flush():Promise<void>{await nextTick();await new Promise(resolve=>setImmediate(resolve))}
/** 每用例隔离协调器，只控制HTTP、原生弹层和官方更新回调。 */
function harness(t:TestContext,options:{guest?:boolean;frequency?:'once'|'always';autoShow?:boolean;storageFails?:boolean}={}){
  const announcement:ContentAnnouncement={id:'home',version:1,enabled:true,frequency:options.frequency||'once',mode:'text',image_url:'',title:'平台公告',body:'真实运营内容',target:null}
  const props=reactive({active:false,announcement}),state=reactive<{user:{id:string}|null}>({user:options.guest?null:{id:'A'}}),revision=ref(1)
  const states:Array<{version:number;user:string;resolve:(value:{seen:boolean})=>void;reject:(error:unknown)=>void}>=[],acks:Array<{version:number;user:string;resolve:()=>void;reject:(error:unknown)=>void}>=[],opens:number[]=[],closes:number[]=[],targets:ContentTarget[]=[],storage=new Map<string,number>()
  const hooks={show:()=>{},hide:()=>{},unmount:()=>{}}
  const timers=new Map<number,()=>void>(),modals:Parameters<UpdateRuntime['showModal']>[0][]=[],callbacks={ready:()=>{}}
  let timer=0,applies=0
  const context=vm.createContext({ref,reactive,watch,nextTick,props,state,sessionRevision:()=>revision.value,defineProps:()=>props,init:async()=>{},pageTheme:()=>({}),assetUrl:(value:string)=>value,openContentTarget:(target:ContentTarget)=>targets.push({...target}),onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onBeforeUnmount:(fn:()=>void)=>{hooks.unmount=fn},setTimeout:(fn:()=>void)=>{timers.set(++timer,fn);return timer},clearTimeout:(id:number)=>timers.delete(id),uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:number)=>{if(options.storageFails)throw new Error('存储不可用');storage.set(key,value)}},getAnnouncementState:(version:number)=>new Promise((resolve,reject)=>states.push({version,user:state.user?.id||'',resolve,reject})),acknowledgeAnnouncement:(version:number)=>new Promise<void>((resolve,reject)=>acks.push({version,user:state.user?.id||'',resolve,reject}))})
  vm.runInContext(executable(readFileSync(new URL('./update-manager.ts',import.meta.url),'utf8'))+';globalThis.coordinator={initializeUpdateManager,setUpdateAppVisible,beginFinancialActivity,acquireSystemPrompt,getUpdatePromptState}',context)
  const coordinator=context.coordinator as typeof import('./update-manager')
  const manager:WechatUpdateManager={onCheckForUpdate:()=>{},onUpdateReady:fn=>{callbacks.ready=fn},onUpdateFailed:()=>{},applyUpdate:()=>{applies++}}
  coordinator.initializeUpdateManager({getUpdateManager:()=>manager,showModal:value=>{modals.push(value)}});coordinator.setUpdateAppVisible(true)
  const script=readFileSync(new URL('../components/HomeAnnouncement.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(script)
  const effect=effectScope();effect.run(()=>vm.runInContext(executable(script)+';globalThis.page={popup,popupInstance,presented,consider,changed,close,navigate}',context))
  const page=context.page
  page.popup.value={open:()=>{opens.push(page.popupInstance.value);if(options.autoShow)page.changed({show:true})},close:()=>{closes.push(page.popupInstance.value);page.changed({show:false})}}
  t.after(()=>{hooks.unmount();effect.stop()})
  /** 执行受控官方调度，循环保护能捕获抢锁导致的重复打开。 */
  function tick():void{let count=0;while(timers.size){assert.ok(count++<30,'公告与更新调度不能循环');const pending=[...timers.values()];timers.clear();for(const fn of pending)fn()}}
  return{props,state,revision,states,acks,opens,closes,targets,storage,hooks,page,coordinator,modals,callbacks,tick,applies:()=>applies,
    /** 切换真实响应式身份，旧请求由生产watch撤销。 */
    switchUser(id:string):void{state.user={id};revision.value++},
    /** 从后台或其它主标签重新进入首页。 */
    async enter():Promise<void>{props.active=false;await flush();props.active=true;await flush()},
  }
}

test('公告只有真正show才确认，主动关闭本次不再弹，always下次进入可再弹',async t=>{
  const s=harness(t,{frequency:'always'});await s.enter();assert.equal(s.opens.length,1);assert.equal(s.acks.length,0)
  s.page.changed({show:true});s.page.changed({show:true});assert.equal(s.acks.length,1);s.acks[0].resolve();s.page.close()
  const busy=s.coordinator.beginFinancialActivity();busy();s.tick();await flush();assert.equal(s.opens.length,1)
  await s.enter();assert.equal(s.opens.length,2);assert.notEqual(s.opens[0],s.opens[1],'每次重建官方关闭动画实例')
})

test('once已读不展示，新版本独立查询且尚未show不能提前ack',async t=>{
  const s=harness(t);await s.enter();assert.equal(s.states.length,1);s.states[0].resolve({seen:true});await flush();assert.equal(s.opens.length,0);assert.equal(s.acks.length,0)
  s.props.announcement.version=2;await flush();assert.equal(s.states.length,2);s.states[1].resolve({seen:false});await flush();assert.equal(s.opens.length,1);assert.equal(s.acks.length,0)
  s.page.changed({show:true});assert.equal(s.acks[0].version,2)
})

test('访客本机写入失败只抑制本次，下次再试；成功写入后once不重复',async t=>{
  const failing=harness(t,{guest:true,storageFails:true,autoShow:true});await failing.enter();assert.equal(failing.opens.length,1);failing.page.close();await failing.enter();assert.equal(failing.opens.length,2);assert.equal(failing.acks.length,0)
  const saved=harness(t,{guest:true,autoShow:true});await saved.enter();saved.page.close();await saved.enter();assert.equal(saved.opens.length,1);assert.equal(saved.storage.get('jijie_announcement_guest_version'),1)
})

test('账号确认已读失败不伪装成功，下次进入仍重新查询',async t=>{
  const s=harness(t,{autoShow:true});await s.enter();s.states[0].resolve({seen:false});await flush();assert.equal(s.acks.length,1)
  s.acks[0].reject(new Error('确认网络失败'));await flush();s.page.close();const busy=s.coordinator.beginFinancialActivity();busy();s.tick();await flush();assert.equal(s.opens.length,1)
  await s.enter();assert.equal(s.states.length,2);s.states[1].resolve({seen:false});await flush();assert.equal(s.opens.length,2)
})

test('旧账号、旧版本和隐藏期间的迟到查询不能弹框或确认新身份',async t=>{
  const s=harness(t);await s.enter();s.switchUser('B');await flush();assert.ok(s.states.length>=2)
  s.states[0].resolve({seen:false});await flush();assert.equal(s.opens.length,0)
  const oldVersion=s.states.at(-1)!;s.props.announcement.version=2;await flush();oldVersion.resolve({seen:false});await flush();assert.equal(s.opens.length,0)
  const hidden=s.states.at(-1)!;s.hooks.hide();hidden.resolve({seen:false});await flush();assert.equal(s.opens.length,0);assert.equal(s.acks.length,0)
  s.hooks.show();await flush();const fresh=s.states.at(-1)!;assert.equal(fresh.user,'B');assert.equal(fresh.version,2);fresh.resolve({seen:false});await flush();s.page.changed({show:true});assert.equal(s.acks.length,1);assert.equal(s.acks[0].user,'B')
})

test('资金忙碌与其它公告占槽时延后，释放后自动补弹且自己抢锁不重入',async t=>{
  const s=harness(t,{frequency:'always',autoShow:true}),busy=s.coordinator.beginFinancialActivity();await s.enter();assert.equal(s.opens.length,0)
  busy();await flush();assert.equal(s.opens.length,1);assert.equal(s.acks.length,1);assert.equal(s.coordinator.getUpdatePromptState().owner,'announcement')
  s.page.close();const occupied=s.coordinator.acquireSystemPrompt('announcement');assert.ok(occupied);await s.enter();assert.equal(s.opens.length,1);occupied();await flush();assert.equal(s.opens.length,2);assert.equal(s.acks.length,2)
})

test('下载完成优先尚未展示的公告，更新失败释放后公告自动恢复',async t=>{
  const s=harness(t,{frequency:'always',autoShow:true});s.callbacks.ready();await s.enter();assert.equal(s.opens.length,0);s.tick();assert.equal(s.modals.length,1)
  s.modals[0].fail();s.tick();await flush();assert.equal(s.opens.length,1);assert.equal(s.coordinator.getUpdatePromptState().owner,'announcement')
})

test('已有公告不被更新覆盖，关闭后唯一更新确认；卸载释放且不补弹',async t=>{
  const s=harness(t,{frequency:'always',autoShow:true});await s.enter();s.callbacks.ready();s.tick();assert.equal(s.modals.length,0)
  s.page.close();s.tick();assert.equal(s.modals.length,1);s.modals[0].success({confirm:true});s.tick();assert.equal(s.applies(),1)
  const other=harness(t,{frequency:'always',autoShow:true});await other.enter();other.hooks.unmount();assert.equal(other.coordinator.getUpdatePromptState().owner,null);const done=other.coordinator.beginFinancialActivity();done();other.tick();await flush();assert.equal(other.opens.length,1)
})

test('官方弹层打开关闭异常都释放提示槽，不会阻止后续版本更新',async t=>{
  const s=harness(t,{frequency:'always'});s.page.popup.value={open:()=>{throw new Error('打开失败')},close:()=>{throw new Error('关闭失败')}}
  await s.enter();assert.equal(s.coordinator.getUpdatePromptState().owner,null);s.callbacks.ready();s.tick();assert.equal(s.modals.length,1)
})

test('同一已展示公告双击只导航一次，关闭动画中的第二次点击被忽略',async t=>{
  const s=harness(t,{frequency:'always',autoShow:true});s.props.announcement.target={kind:'article',id:'guide'};await s.enter()
  s.page.navigate();s.page.navigate();assert.deepEqual(s.targets,[{kind:'article',id:'guide'}]);assert.equal(s.coordinator.getUpdatePromptState().owner,null)
})

test('关闭、隐藏、旧账号或旧版本的公告点击均不跳转',async t=>{
  for(const change of ['close','hide','account','revision','version'] as const){const s=harness(t,{frequency:'always',autoShow:true});s.props.announcement.target={kind:'article',id:'guide'};s.page.navigate();assert.equal(s.targets.length,0);await s.enter();if(change==='close')s.page.close();else if(change==='hide')s.hooks.hide();else if(change==='account')s.switchUser('B');else if(change==='revision')s.revision.value++;else s.props.announcement.version++;s.page.navigate();assert.equal(s.targets.length,0,change)}
})

test('呈现的跳转是独立快照，后台撤链或同版本改变目标后旧图片不得导航',async t=>{
  const s=harness(t,{frequency:'always',autoShow:true});s.props.announcement.target={kind:'article',id:'guide'};await s.enter();s.props.announcement.target.id='new-guide';s.page.navigate();assert.equal(s.targets.length,0);assert.equal(s.page.presented.value.target.id,'guide')
  s.props.announcement.target=null;s.page.navigate();assert.equal(s.targets.length,0);s.props.announcement.target={kind:'article',id:'guide'};s.page.navigate();assert.deepEqual(s.targets,[{kind:'article',id:'guide'}])
})
