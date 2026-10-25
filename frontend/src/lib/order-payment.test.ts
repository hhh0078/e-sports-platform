/** 文件作用：执行订单详情真实支付脚本，验证双渠道确认、原单查款、最终裁决与账号离开保护。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import { ref,computed,watch,nextTick,effectScope } from 'vue'
import { executable,pageScript,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import { newWalletRequestID } from './wallet'

/** 运行生产订单页，只控制后端结果与微信付款响应。 */
function harness(t:TestContext){
  const pending:DeferredRequest[]=[],reads:string[]=[],notices:unknown[]=[],storage=new Map<string,string>(),hooks={hide:()=>{},unload:()=>{}}
  const state={user:{id:'A',role:'user'},devMode:true},order={id:'order-A',buyer_id:'A',provider_id:'B',status:'pending_payment',payment_status:'unpaid',amount_cents:1000,refund_status:'none'}
  let rejectNative=false,nativeCalls=0
  const context=vm.createContext({ref,computed,watch,nextTick,state,newWalletRequestID,init:async()=>{},sessionRevision:()=>0,requirePhone:async()=>true,token:()=>state.user.id,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value)},notify:(value:unknown)=>notices.push(value),
    setInterval,clearInterval,onLoad:()=>{},onShow:()=>{},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
    requestWechatPayment:async()=>{nativeCalls++;if(rejectNative)throw new Error('用户取消')},
    /** 每一次支付及查款都由测试明确完成，GET仅返回当前真实状态夹具。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){
      if(method==='GET'&&path!=='/wallet'){reads.push(path);return Promise.resolve(path==='/orders/order-A'?{...order}:[])}
      return new Promise((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))
    },
  })
  injectBusinessAPIs(context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('orders/detail.vue'))+"\nvisible=true;id.value='order-A';globalThis.page={order,prepare,execute,pendingAction,paymentWallet,paymentChannel,walletError,actions,busy,reviews,canReview,reviewText,actionNote,submitReview,refreshOrder,adjustmentsView}",context));t.after(()=>scope.stop())
  context.page.order.value={...order}
  return{page:context.page,state,order,hooks,pending,reads,notices,storage,
    cancelNative(){rejectNative=true},nativeCalls:()=>nativeCalls,
    /** 断言资金接口确实发出，避免空测试通过。 */
    take(path:string){const item=pending.find(row=>!row.taken&&row.path===path);assert.ok(item,'缺少请求 '+path);item.taken=true;return item},
  }
}
/** 等待真实异步函数推进到下一受控边界。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 通过真实准备函数打开余额支付确认。 */
async function prepareBalance(s:ReturnType<typeof harness>){const showing=s.page.prepare({id:'pay',label:'支付订单'});s.take('/wallet').resolve({spendable_cents:2000});await showing;s.page.paymentChannel.value='balance'}

test('支付准备真正回填后端可用余额，旧弹层响应不覆盖新动作',async t=>{
  const s=harness(t);await prepareBalance(s);assert.equal(s.page.paymentWallet.value.spendable_cents,2000)
  const old=s.page.prepare({id:'pay',label:'支付订单'}),reading=s.take('/wallet');await s.page.prepare({id:'cancel',label:'取消订单'});reading.resolve({spendable_cents:99999});await old
  assert.equal(s.page.paymentWallet.value,null);assert.equal(s.page.pendingAction.value.id,'cancel')
})
test('余额付款连点只提交一次，未知结果重试保留业务号',async t=>{
  const s=harness(t);await prepareBalance(s);const first=s.page.execute();await flush();await s.page.execute();const paying=s.take('/orders/order-A/pay')
  assert.equal(paying.body.channel,'balance');assert.ok(paying.body.request_id);paying.reject(new Error('网络结果未知'));await first
  const retry=s.page.execute();await flush();const second=s.take('/orders/order-A/pay');assert.equal(second.body.request_id,paying.body.request_id)
  s.order.payment_status='paid';s.order.status='open';second.resolve({mode:'balance',order:{...s.order}});await retry
  assert.equal(s.nativeCalls(),0);assert.equal(s.page.pendingAction.value,null)
})
test('微信取消仍查原单，未确认付款不显示成功',async t=>{
  const s=harness(t);await prepareBalance(s);s.page.paymentChannel.value='wechat';s.cancelNative();const paying=s.page.execute();await flush()
  s.take('/orders/order-A/pay').resolve({mode:'wechat',order:{...s.order},pay_params:{package:'prepay_id=test'}});await flush();s.take('/orders/order-A/sync-payment').resolve({...s.order});await paying
  assert.equal(s.nativeCalls(),1);assert.ok(s.page.pendingAction.value);assert.ok(s.notices.some(note=>String(note).includes('尚未确认')));assert.ok(!s.notices.some(note=>String(note).includes('操作成功')))
})
test('支付响应在页面销毁后不允许拉起微信',async t=>{
  const s=harness(t);await prepareBalance(s);s.page.paymentChannel.value='wechat';const paying=s.page.execute();await flush();const response=s.take('/orders/order-A/pay');s.hooks.unload()
  response.resolve({mode:'wechat',order:{...s.order},pay_params:{package:'prepay_id=test'}});await paying;assert.equal(s.nativeCalls(),0);assert.equal(s.notices.length,0)
})
test('平台最终裁决后不再出现重复退款和介入操作',t=>{
  const s=harness(t);Object.assign(s.page.order.value,{status:'completed',refund_status:'rejected',refund_final:true})
  const ids=s.page.actions.value.map((item:{id:string})=>item.id);assert.ok(!ids.includes('request_refund'));assert.ok(!ids.includes('escalate'));assert.ok(!ids.includes('withdraw_refund'))
})


test('评价仅已完成且无进行中售后的原买家可写，其他参与者只读',t=>{
  const s=harness(t);Object.assign(s.page.order.value,{status:'completed',refund_status:'none'});assert.equal(s.page.canReview.value,true)
  for(const refund of ['requested','escalated','processing']){s.page.order.value.refund_status=refund;assert.equal(s.page.canReview.value,false,refund)}
  s.page.order.value.refund_status='none';s.page.order.value.status='delivered';assert.equal(s.page.canReview.value,false)
  s.page.order.value.status='completed';s.page.order.value.buyer_id='another-buyer';assert.equal(s.page.canReview.value,false)
  s.page.order.value.buyer_id='A';s.page.reviews.value=[{id:'already-reviewed'}];assert.equal(s.page.canReview.value,false)
})

test('评价与后端统一500字上限，超长不提交，合法内容只提交原订单',async t=>{
  const s=harness(t);Object.assign(s.page.order.value,{status:'completed',refund_status:'none'});s.page.reviewText.value='文'.repeat(501);await s.page.submitReview();assert.equal(s.pending.length,0)
  s.page.reviewText.value='文'.repeat(500);const posting=s.page.submitReview();const write=s.take('/orders/order-A/review');assert.equal(String(write.body.content).length,500);write.resolve({id:'review-A'});await posting;assert.equal(s.page.busy.value,false)
})

test('订单下拉只读取订单评价及原费用记录，保留评价和协商草稿并不加载聊天',async t=>{
  const s=harness(t);s.page.reviewText.value='评价草稿';s.page.actionNote.value='售后协商草稿';let adjustmentReads=0;s.page.adjustmentsView.value={refresh:async()=>{adjustmentReads++}}
  await s.page.refreshOrder();assert.deepEqual(s.reads.sort(),['/orders/order-A','/orders/order-A/reviews']);assert.equal(adjustmentReads,1);assert.equal(s.page.reviewText.value,'评价草稿');assert.equal(s.page.actionNote.value,'售后协商草稿');assert.equal(s.pending.length,0)
})
