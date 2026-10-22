/** 文件作用：执行真实共享游戏选择器及首页包装器，验证受控选择、取消、会话生命周期和全局偏好边界。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'
import { executable } from './sfc-test'

/** 测试可控制的选择器公开属性，与真实组件的默认配置一致。 */
interface SelectorProps { value:string; values:string[]; multiple:boolean; disabled:boolean; active:boolean; field:boolean; title:string; placeholder:string; minSelections:number; maxSelections:number }
const names=['三角洲行动','王者荣耀','和平精英','暗区突围','无畏契约','穿越火线手游','金铲铲之战','英雄联盟手游','使命召唤手游','蛋仔派对','第五人格','无畏契约手游','永劫无间手游']

/** 读取真实组件脚本，测试不复制选择或缓存实现。 */
function componentScript(name:string):string{const script=readFileSync(new URL('../components/'+name+'.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(script);return script}
/** 搭建真实store和Vue响应式环境，只替代原生弹层与生命周期出口。 */
function harness(overrides:Partial<SelectorProps>={},component='GameSelector'){
  const props=reactive<SelectorProps>({value:'game-0',values:[],multiple:false,disabled:false,active:true,field:true,title:'选择游戏',placeholder:'请选择游戏',minSelections:1,maxSelections:32,...overrides})
  const storage=new Map([['jijie_token','token-A'],['jijie_game','game-0']]),writes:Array<[string,string]>=[],events:Array<[string,unknown]>=[],notices:unknown[]=[],popupCalls:string[]=[],scope=effectScope()
  const lifecycle={show:()=>{},hide:()=>{},unload:()=>{},unmount:()=>{}}
  const context=vm.createContext({computed,ref,reactive,watch,nextTick,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>{storage.set(key,value);writes.push([key,value])},removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',notify:(message:unknown)=>notices.push(message),
    defineProps:()=>props,withDefaults:(value:unknown)=>value,defineEmits:()=>(name:string,value:unknown)=>events.push([name,value]),defineExpose:()=>{},
    onShow:(callback:()=>void)=>{lifecycle.show=callback},onHide:(callback:()=>void)=>{lifecycle.hide=callback},onUnload:(callback:()=>void)=>{lifecycle.unload=callback},onBeforeUnmount:(callback:()=>void)=>{lifecycle.unmount=callback},
  })
  vm.runInContext(executable(readFileSync(new URL('./store.ts',import.meta.url),'utf8'))+'\nglobalThis.store={state,clearSession,sessionRevision}',context)
  context.store.state.user={id:'A'};context.store.state.games=names.map((name,index)=>({id:'game-'+index,name,short_name:index===0?'三角洲':name,icon_url:'/icon-'+index+'.png',cover_url:'/cover-'+index+'.png',enabled:true}))
  const exposed=component==='GameSelector'?'open,close,popup,popupChanged,choose,confirm,search,draft,opened,filteredGames,options,summary,canConfirm':'chooseGame,openCatalog,selector'
  scope.run(()=>vm.runInContext(executable(componentScript(component))+'\nglobalThis.subject={'+exposed+'}',context))
  if(component==='GameSelector')context.subject.popup.value={open:()=>popupCalls.push('open'),close:()=>popupCalls.push('close')}
  else context.subject.selector.value={open:()=>popupCalls.push('open')}
  return{subject:context.subject,store:context.store,props,storage,writes,events,notices,popupCalls,lifecycle,scope}
}

test('单选打开复制当前值，搜索全部13个配置并在有效选择后即选即回',async()=>{
  const h=harness();h.subject.open();await nextTick()
  assert.equal(h.subject.filteredGames.value.length,13);assert.deepEqual(h.popupCalls,['open'])
  h.subject.search.value='  三角洲 ';assert.equal(h.subject.filteredGames.value[0].id,'game-0')
  h.subject.search.value='GAME-12';assert.equal(h.subject.filteredGames.value[0].name,'永劫无间手游')
  h.subject.search.value='不存在';assert.equal(h.subject.filteredGames.value.length,0)
  h.subject.choose('missing');assert.equal(h.events.length,0)
  h.subject.choose('game-1');assert.deepEqual(h.events,[['change','game-1']]);assert.equal(h.subject.opened.value,false)
  assert.equal(h.store.state.gameId,'game-0');assert.equal(h.storage.get('jijie_game'),'game-0');assert.equal(h.writes.length,0)
  h.subject.open();await nextTick();assert.equal(h.subject.search.value,'');assert.deepEqual(Array.from(h.subject.draft.value),['game-0']);h.scope.stop()
})

test('关闭与官方遮罩取消不发出选择，不修改传入数组或其他表单内容',async()=>{
  const h=harness({multiple:true,values:['game-0','game-0']}),original=h.props.values
  h.subject.open();await nextTick();h.subject.choose('game-1')
  assert.deepEqual(Array.from(original),['game-0','game-0']);assert.deepEqual(Array.from(h.subject.draft.value),['game-0','game-1'])
  h.subject.popupChanged({show:false});assert.equal(h.subject.opened.value,false);assert.equal(h.events.length,0)
  h.subject.open();await nextTick();assert.deepEqual(Array.from(h.subject.draft.value),['game-0']);h.subject.close()
  assert.equal(h.props.values,original);assert.equal(h.events.length,0);h.scope.stop()
})

test('多选确认才回填去重且独立的新数组，外部修改不会影响已回填快照',async()=>{
  const h=harness({multiple:true,values:['game-0','game-0']})
  h.subject.open();await nextTick();h.subject.choose('game-1');h.subject.confirm()
  assert.equal(h.events[0][0],'update:values');const emitted=h.events[0][1] as string[]
  assert.deepEqual(Array.from(emitted),['game-0','game-1']);assert.equal(h.subject.opened.value,false)
  h.props.values.push('game-2');assert.deepEqual(Array.from(emitted),['game-0','game-1']);assert.equal(h.writes.length,0);h.scope.stop()
})

test('多选最少与最多数量受控，超限不能添加、空选不能确认',async()=>{
  const h=harness({multiple:true,values:['game-0','game-1'],maxSelections:2})
  h.subject.open();await nextTick();h.subject.choose('game-2');assert.equal(h.subject.draft.value.length,2);assert.equal(h.notices.length,1)
  h.subject.choose('game-0');h.subject.choose('game-1');assert.equal(h.subject.canConfirm.value,false)
  h.subject.confirm();assert.equal(h.events.length,0);assert.equal(h.subject.opened.value,true)
  h.subject.choose('game-2');h.subject.confirm();assert.deepEqual(Array.from(h.events[0][1] as string[]),['game-2']);h.scope.stop()
})

test('未知或停用的原选择不会无声丢失，可明确取消但不能重新加入停用游戏',async()=>{
  const h=harness({multiple:true,values:['legacy-game','game-0']});h.store.state.games[0].enabled=false
  h.subject.open();await nextTick();assert.equal(h.subject.options.value.find((item:{id:string})=>item.id==='legacy-game').available,false)
  h.subject.choose('game-1');h.subject.confirm();assert.deepEqual(Array.from(h.events[0][1] as string[]),['legacy-game','game-0','game-1'])
  h.subject.open();await nextTick();h.subject.choose('legacy-game');h.subject.choose('game-0');h.subject.choose('game-0');h.subject.choose('game-2');h.subject.confirm()
  assert.deepEqual(Array.from(h.events[1][1] as string[]),['game-2']);h.scope.stop()
})

test('停用的单选只能另选有效游戏，无法选择停用或伪造ID',async()=>{
  const h=harness();h.store.state.games[0].enabled=false;h.subject.open();await nextTick()
  h.subject.choose('game-0');h.subject.choose('invented');assert.equal(h.events.length,0)
  h.subject.choose('game-1');assert.deepEqual(h.events,[['change','game-1']]);h.scope.stop()
})

test('隐藏、卸载、失活、禁用、切号和父值刷新都作废打开中的临时选择',async t=>{
  for(const boundary of ['hide','unload','unmount','inactive','disabled','identity','revision','token','parent'])await t.test(boundary,async()=>{
    const h=harness({multiple:true,values:['game-0']});h.subject.open();await nextTick();h.subject.choose('game-1')
    if(boundary==='hide')h.lifecycle.hide();if(boundary==='unload')h.lifecycle.unload();if(boundary==='unmount')h.lifecycle.unmount()
    if(boundary==='inactive')h.props.active=false;if(boundary==='disabled')h.props.disabled=true
    if(boundary==='identity')h.store.state.user={id:'B'};if(boundary==='revision')h.store.clearSession()
    if(boundary==='token')h.storage.set('jijie_token','different-token');if(boundary==='parent')h.props.values=['game-3']
    h.subject.confirm();h.subject.choose('game-2');assert.equal(h.events.length,0);assert.equal(h.subject.opened.value,false)
    assert.deepEqual(Array.from(h.props.values),boundary==='parent'?['game-3']:['game-0']);h.scope.stop()
  })
})

test('打开后的下一帧尚未展示时隐藏不会打开残留弹框，重显可重新打开',async()=>{
  const h=harness();h.subject.open();h.lifecycle.hide();await nextTick();assert.equal(h.popupCalls.includes('open'),false)
  h.subject.open();await nextTick();assert.equal(h.subject.opened.value,false)
  h.lifecycle.show();h.subject.open();await nextTick();assert.equal(h.subject.opened.value,true);assert.equal(h.popupCalls.filter(item=>item==='open').length,1);h.scope.stop()
})

test('原生ref晚于下一帧绑定仍能展示，取消后迟到绑定不会复活弹框',async()=>{
  const h=harness(),instance=h.subject.popup.value;h.subject.popup.value=null
  h.subject.open();await nextTick();assert.equal(h.popupCalls.includes('open'),false)
  h.subject.popup.value=instance;await nextTick();assert.equal(h.popupCalls.filter(item=>item==='open').length,1)
  h.subject.close();h.subject.popup.value=null;h.subject.open();await nextTick();h.lifecycle.hide()
  h.subject.popup.value=instance;await nextTick();assert.equal(h.popupCalls.filter(item=>item==='open').length,1);assert.equal(h.subject.opened.value,false);h.scope.stop()
})

test('官方旧关闭动画尚未结束时再次打开，等待实例隐藏后展示新选择而不丢操作',async()=>{
  const h=harness();h.subject.open();await nextTick();assert.equal(h.popupCalls.filter(item=>item==='open').length,1)
  h.subject.popup.value.showPopup=true;h.subject.close();h.subject.open();await nextTick();assert.equal(h.popupCalls.filter(item=>item==='open').length,1)
  h.subject.popup.value.showPopup=false;await nextTick();assert.equal(h.popupCalls.filter(item=>item==='open').length,2)
  h.subject.choose('game-1');assert.deepEqual(h.events,[['change','game-1']]);h.scope.stop()
})

test('未打开选择器就隐藏不会创建官方关闭timer，立即重显打开后持续可见',async t=>{
  t.mock.timers.enable({apis:['setTimeout']})
  const h=harness();let closes=0
  const instance=reactive({showPopup:false,
    /** 模拟官方打开后的公开可见状态，供真实组件监听。 */
    open(){instance.showPopup=true},
    /** 保留官方close即使已隐藏也创建300ms定时器的行为。 */
    close(){closes++;setTimeout(()=>{instance.showPopup=false},300)},
  })
  h.subject.popup.value=instance;await nextTick();h.lifecycle.hide();assert.equal(closes,0)
  h.lifecycle.show();h.subject.open();await nextTick();assert.equal(instance.showPopup,true)
  t.mock.timers.tick(300);await nextTick();assert.equal(instance.showPopup,true);assert.equal(h.subject.opened.value,true);assert.equal(closes,0)
  h.subject.close();assert.equal(closes,1);t.mock.timers.tick(300);await nextTick();assert.equal(instance.showPopup,false)
  h.scope.stop()
})

test('空配置诚实显示空列表，禁用或失活时不能打开，重新打开清空搜索',async()=>{
  const h=harness({value:'',active:false});h.subject.open();assert.equal(h.subject.opened.value,false)
  h.props.active=true;h.props.disabled=true;h.subject.open();assert.equal(h.subject.opened.value,false)
  h.props.disabled=false;h.store.state.games=[];h.subject.open();await nextTick();assert.equal(h.subject.filteredGames.value.length,0)
  assert.equal(h.subject.summary.value,'请选择游戏');h.subject.close();h.scope.stop()
})

test('首页与接单大厅包装器仅在可见时显式选择有效游戏并保留原偏好语义',()=>{
  const h=harness({},'GamePicker');h.subject.chooseGame('game-1');assert.equal(h.store.state.gameId,'game-1');assert.deepEqual(h.writes,[['jijie_game','game-1']])
  h.subject.chooseGame('missing');h.props.active=false;h.subject.chooseGame('game-2');h.subject.openCatalog();assert.equal(h.writes.length,1);assert.equal(h.popupCalls.length,0)
  h.props.active=true;h.subject.openCatalog();assert.deepEqual(h.popupCalls,['open']);h.scope.stop()
})
