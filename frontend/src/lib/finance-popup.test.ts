/** 文件作用：执行真实资金弹层脚本，验证官方遮罩、忙碌关闭保护、晚绑定和生命周期清理。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, effectScope, nextTick, reactive, ref, watch } from 'vue'
import { executable } from './sfc-test'
import { designRpx } from './design-units'

/** 仅替代官方弹层实例和宿主生命周期，保留真实Vue侦听与生产显示意图。 */
function harness(t:TestContext){
  const props=reactive({opened:false,title:'补缴平台押金',busy:false}),state=reactive({user:{id:'A'}}),calls:string[]=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{},unmount:()=>{}}
  let revision=0,closed=0
  const context=vm.createContext({ref,watch,computed,nextTick,designRpx,state,sessionRevision:()=>revision,pageTheme:()=>({'--accent':'#8de8b6','--bg':'#0a1110','--surface':'#14201b','--text':'#edf4ef'}),defineProps:()=>props,withDefaults:(value:unknown)=>value,defineEmits:()=>()=>{closed++;if(!props.busy)props.opened=false},onShow:(value:()=>void)=>{hooks.show=value},onHide:(value:()=>void)=>{hooks.hide=value},onUnload:(value:()=>void)=>{hooks.unload=value},onBeforeUnmount:(value:()=>void)=>{hooks.unmount=value}})
  const script=readFileSync(new URL('../components/FinancePopup.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(script)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(script)+'\nglobalThis.subject={popup,intent,requestClose,popupChanged,popupTheme}',context));t.after(()=>scope.stop())
  const instance=reactive({showPopup:false,open(){calls.push('open');instance.showPopup=true;context.subject.popupChanged({show:true})},close(){calls.push('close');context.subject.popupChanged({show:false})}})
  context.subject.popup.value=instance
  return{subject:context.subject,props,calls,hooks,instance,closed:()=>closed,finishClose:()=>{instance.showPopup=false},switchUser:()=>{revision++;state.user={id:'B'}}}
}
/** 排空真实Vue的nextTick与侦听队列。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}

test('资金弹框使用官方UniPopup与微信root-portal，主题和边框次要颜色显式补齐',t=>{
  const source=readFileSync(new URL('../components/FinancePopup.vue',import.meta.url),'utf8'),s=harness(t)
  assert.match(source,/<!-- #ifdef MP-WEIXIN -->\s*<root-portal>/)
  assert.match(source,/<UniPopup[^>]*:is-mask-click="!busy"/)
  assert.equal(s.subject.popupTheme.value['--bg'],'#0a1110');assert.ok(s.subject.popupTheme.value['--muted']);assert.ok(s.subject.popupTheme.value['--line']);assert.equal(s.subject.popupTheme.value['--control-height'],designRpx(44));assert.equal(s.subject.popupTheme.value['--compact-height'],designRpx(36))
  for(const file of ['FundsForm','WithdrawalRules']){const dependent=readFileSync(new URL('../components/'+file+'.vue',import.meta.url),'utf8');assert.match(dependent,/<FinancePopup/);assert.doesNotMatch(dependent,/class="modal-mask/)}
  const wallet=readFileSync(new URL('../pages/account/wallet.vue',import.meta.url),'utf8');assert.match(wallet,/<FundsForm/);assert.doesNotMatch(wallet,/class="modal-mask/)
})

test('忙碌期间关闭动作不发取消事件，解锁后可关闭且原请求由业务函数保护',async t=>{
  const s=harness(t);s.props.opened=true;await flush();assert.equal(s.instance.showPopup,true)
  s.props.busy=true;s.subject.requestClose();await flush();assert.equal(s.closed(),0);assert.equal(s.subject.intent.value,true)
  s.props.busy=false;s.subject.requestClose();await flush();assert.equal(s.closed(),1);assert.equal(s.subject.intent.value,false);assert.deepEqual(s.calls,['open','close'])
})

test('关闭后立刻重开等待官方关闭动画结束，再恢复当前新意图',async t=>{
  const s=harness(t);s.props.opened=true;await flush();s.subject.requestClose();await flush();s.props.opened=true;await flush()
  assert.deepEqual(s.calls,['open','close']);s.finishClose();await flush();assert.deepEqual(s.calls,['open','close','open']);assert.equal(s.subject.intent.value,true)
})

test('官方遮罩关闭撤销表单，父组件状态同步后不自动弹回',async t=>{
  const s=harness(t);s.props.opened=true;await flush();s.subject.popupChanged({show:false});s.finishClose();await flush()
  assert.equal(s.closed(),1);assert.equal(s.props.opened,false);assert.equal(s.subject.intent.value,false);assert.equal(s.calls.filter(value=>value==='open').length,1)
})

test('等待ref时隐藏，延迟绑定不会复活旧资金表单',async t=>{
  const s=harness(t);s.subject.popup.value=null;s.props.opened=true;s.hooks.hide();s.subject.popup.value=s.instance;await flush();s.hooks.show();await flush()
  assert.equal(s.subject.intent.value,false);assert.deepEqual(s.calls,[])
})

test('隐藏切号卸载均撤销临时显示，忙碌标志不能保留跨页面旧表单',async t=>{
  for(const boundary of ['hide','switch','unload','unmount'])await t.test(boundary,async child=>{const s=harness(child);s.props.opened=true;await flush();s.props.busy=true;if(boundary==='hide')s.hooks.hide();if(boundary==='switch')s.switchUser();if(boundary==='unload')s.hooks.unload();if(boundary==='unmount')s.hooks.unmount();await flush();s.finishClose();s.hooks.show();await flush();assert.equal(s.subject.intent.value,false);assert.equal(s.calls.filter(value=>value==='open').length,1)})
})
