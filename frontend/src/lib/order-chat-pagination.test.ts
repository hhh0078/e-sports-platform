/** 文件作用：执行独立订单聊天真实方法，验证历史游标、阅读锚点、轮询增量与草稿保护。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { computed,ref,watch,nextTick,effectScope,reactive } from 'vue'
import { executable,pageScript,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'

/** 运行生产页面，只控制GET完成顺序，不实现第二套聊天分页算法。 */
function harness(t:TestContext){
  const requests:DeferredRequest[]=[],notices:unknown[]=[]
  const state=reactive({user:{id:'A',role:'user'},devMode:true})
  const context=vm.createContext({ref,computed,watch,nextTick,state,init:async()=>{},sessionRevision:()=>1,token:()=>state.user.id,notify:(v:unknown)=>notices.push(v),uni:{},setInterval,clearInterval,onLoad:()=>{},onShow:()=>{},onHide:()=>{},onUnload:()=>{},onResize:()=>{},api(path:string,method='GET',body:Record<string,unknown>={}){if(path==='/orders/order-A')return Promise.resolve({id:'order-A',buyer_id:'A',status:'in_progress'});if(path==='/orders/order-A/reviews')return Promise.resolve([]);return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('orders/chat.vue'))+"\nvisible=true;id.value='order-A';viewerID='A';chatInitialized=true;globalThis.page={load,refreshOrder,loadEarlier,order,error,messages,hasEarlier,historyError,loadingEarlier,scrollTarget,followChatBottom,chatScrolled,draft,sending,sendMessage,refreshEvidence,stop}",context));t.after(()=>scope.stop())
  return{page:context.page,requests,notices,state,
    /** 取得真实发出的历史或增量请求，不允许没有网络调用的假通过。 */
    take(path:string,method='GET'){const item=requests.find(row=>!row.taken&&row.path===path&&row.method===method);assert.ok(item,'缺少GET '+path);item.taken=true;return item},
  }
}
/** 提供按服务端稳定时间顺序的消息夹具。 */
function message(index:number){return{id:'msg-'+String(index).padStart(3,'0'),created_at:'2026-10-24T01:'+String(Math.floor(index/60)).padStart(2,'0')+':'+String(index%60).padStart(2,'0')+'Z',content:'消息'+index,sender_id:'A'}}
/** 排空页面异步读取和Vue锚点更新。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}

test('聊天室触顶只取一次更早GET，合并后保留原首条锚点和输入',async t=>{
  const s=harness(t);s.page.messages.value=Array.from({length:100},(_,i)=>message(i+10));s.page.hasEarlier.value=true;s.page.draft.value='未发送聊天'
  const first=s.page.loadEarlier();await s.page.loadEarlier();assert.equal(s.requests.length,1)
  s.take('/orders/order-A/messages?limit=100&before_id=msg-010').resolve([message(8),message(9)]);await first
  assert.equal(s.page.messages.value.length,102);assert.equal(s.page.scrollTarget.value,'msg-msg-010');assert.equal(s.page.hasEarlier.value,false);assert.equal(s.page.followChatBottom.value,false)
  assert.equal(s.page.draft.value,'未发送聊天');assert.ok(s.requests.every(row=>row.method==='GET'))
})

test('历史读取失败保留原消息，重试沿用相同before_id',async t=>{
  const s=harness(t);s.page.messages.value=[message(10)];s.page.hasEarlier.value=true
  const first=s.page.loadEarlier();s.take('/orders/order-A/messages?limit=100&before_id=msg-010').reject(new Error('网络故障'));await first
  assert.equal(s.page.messages.value.length,1);assert.equal(s.page.hasEarlier.value,true);assert.match(s.page.historyError.value,/重试/)
  const retry=s.page.loadEarlier();s.take('/orders/order-A/messages?limit=100&before_id=msg-010').resolve([]);await retry;assert.equal(s.page.hasEarlier.value,false);assert.equal(s.page.historyError.value,'')
})

test('向上阅读历史时轮询新消息只合并，不抢走当前阅读位置',async t=>{
  const s=harness(t);s.page.messages.value=[message(10)];s.page.scrollTarget.value='history-anchor';s.page.chatScrolled({detail:{scrollTop:100}});s.page.chatScrolled({detail:{scrollTop:30}})
  const polling=s.page.load(true);await flush();s.take('/orders/order-A/messages?limit=100&after_id=msg-010').resolve([message(11)]);await polling
  assert.equal(s.page.messages.value.length,2);assert.equal(s.page.scrollTarget.value,'history-anchor');assert.equal(s.page.followChatBottom.value,false)
})

test('页面隐藏后历史迟到回包不能追加聊天或恢复旧锚点',async t=>{
  const s=harness(t);s.page.messages.value=[message(10)];s.page.hasEarlier.value=true;const first=s.page.loadEarlier(),reading=s.take('/orders/order-A/messages?limit=100&before_id=msg-010');s.page.stop();reading.resolve([message(9)]);await first
  assert.equal(s.page.messages.value.length,1);assert.equal(s.page.loadingEarlier.value,false);assert.equal(s.page.scrollTarget.value,'')
})

test('独立聊天下拉只读刷新消息，保留草稿且不触发资金或评价请求',async t=>{
  const s=harness(t);s.page.messages.value=[message(10)];s.page.draft.value='保留聊天草稿'
  const refresh=s.page.refreshOrder();await flush();s.take('/orders/order-A/messages?limit=100&after_id=msg-010').resolve([]);await refresh
  assert.ok(s.requests.every(item=>item.method==='GET'));assert.equal(s.page.draft.value,'保留聊天草稿');assert.ok(!s.requests.some(item=>item.path.includes('adjustments')||item.path.includes('reviews')||item.path.includes('payment')))
})


test('独立聊天失去历史读取权限后立即清除旧订单、证据和草稿',async t=>{
  const s=harness(t);s.page.order.value={id:'order-A'};s.page.messages.value=[message(10)];s.page.hasEarlier.value=true;s.page.draft.value='尚未发送'
  const reading=s.page.loadEarlier();s.take('/orders/order-A/messages?limit=100&before_id=msg-010').reject({status:403,message:'已失去订单权限'});await reading
  assert.equal(s.page.order.value,null);assert.equal(s.page.messages.value.length,0);assert.equal(s.page.draft.value,'');assert.match(s.page.error.value,/失去/)
})

test('发送失败保留草稿，连点只写一次，成功消息即时去重进入独立聊天',async t=>{
  const s=harness(t);s.page.messages.value=[message(10)];s.page.draft.value='新的沟通内容'
  const failed=s.page.sendMessage();await s.page.sendMessage();assert.equal(s.requests.length,1);s.take('/orders/order-A/messages','POST').reject(new Error('发送网络故障'));await failed
  assert.equal(s.page.draft.value,'新的沟通内容');assert.equal(s.page.sending.value,false)
  const sent=s.page.sendMessage();s.take('/orders/order-A/messages','POST').resolve(message(11));await flush();s.take('/orders/order-A/messages?limit=100&after_id=msg-011').resolve([message(11)]);await sent
  assert.deepEqual(Array.from(s.page.messages.value,(m:{id:string})=>m.id),['msg-010','msg-011']);assert.equal(s.page.draft.value,'')
})

test('旧账号证据续签迟到后不能修改图片或恢复已清理的私密聊天',async t=>{
  const s=harness(t),id='a'.repeat(32),old={...message(10),image_url:'/api/evidence/'+id+'?signature=old'};s.page.messages.value=[old]
  const refreshing=s.page.refreshEvidence(old),request=s.take('/orders/order-A/evidence-link?evidence_id='+id)
  s.state.user.id='B';await flush();request.resolve({url:'/api/evidence/'+id+'?signature=old-account-renewed'});await refreshing
  assert.equal(old.image_url,'/api/evidence/'+id+'?signature=old');assert.equal(s.page.messages.value.length,0)
})
