/** 文件作用：执行真实组织资金组合函数，验证押金归属、未知原单、微信回查和账号隔离。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch, effectScope } from 'vue'
import { executable, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'
import { parseWalletAmount } from './wallet'

/** 使用真实业务方法，仅控制网络、微信交互完成顺序及账号变化。 */
function harness(t:TestContext,scope:'member_club'|'club_platform'='club_platform'){
  const storage=new Map<string,unknown>(),pending:DeferredRequest[]=[],notices:unknown[]=[],payments:Array<{resolve:()=>void;reject:(error:unknown)=>void}>=[]
  const hooks:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const state=reactive({user:{id:'user-A'},ready:true});let revision=0
  const wallet={available_cents:10000,recharge_cents:0,pending_cents:0,ongoing_cents:0,settling_cents:0,withdrawal_frozen_cents:0,deposit_cents:0,deposit_refunding_cents:0,withdrawal_min_cents:1,dev_mode:true,currency:'CNY',settlement_hold_days:7}
  const deposits={platform:{amount_cents:100,refunding_cents:0,required_cents:10000,can_manage:true},membership:{amount_cents:25,refunding_cents:0,required_cents:25}}
  const histories={topups:[] as unknown[],refunds:[] as unknown[],withdrawals:[] as unknown[]}
  const context=vm.createContext({ref,reactive,computed,watch,state,init:async()=>{},sessionRevision:()=>revision,token:()=>state.user.id+'token',parseWalletAmount,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:unknown)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    requirePhone:async()=>true,go:()=>{},notify:(value:unknown)=>notices.push(value),
    requestWechatPayment:()=>new Promise<void>((resolve,reject)=>payments.push({resolve,reject})),confirmWechatTransfer:async()=>{},
    onLoad:(callback:PageLifecycle['load'])=>hooks.load=callback,onShow:(callback:()=>void)=>hooks.show=callback,onHide:(callback:()=>void)=>hooks.hide=callback,onUnload:(callback:()=>void)=>hooks.unload=callback,
    /** 资金写入与原单同步由用例逐个结束；读取返回明确组织归属。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){
      if(method==='GET')return Promise.resolve(path==='/clubs/club-A'?{club:{id:'club-A',name:'测试俱乐部'},can_manage:true,members:[],services:[],my_membership:null}:path.endsWith('/deposits')?deposits:path.endsWith('/wallet')?{...wallet}:path.endsWith('/withdrawals')?histories.withdrawals:path==='/wallet/topups'?histories.topups:path==='/wallet/deposit-refunds'?histories.refunds:[])
      return new Promise((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))
    },
  })
  injectBusinessAPIs(context)
  const scopeEffect=effectScope();scopeEffect.run(()=>vm.runInContext(executable(readFileSync(new URL('./use-club-funds.ts',import.meta.url),'utf8'))+'\nglobalThis.page=useClubFunds('+JSON.stringify(scope)+')',context));t.after(()=>scopeEffect.stop())
  hooks.load({id:'club-A'})
  return{page:context.page,hooks,pending,storage,notices,payments,wallet,deposits,histories,
    /** 切换真实响应式身份并触发生产会话清理。 */
    switchUser(){revision++;state.user={id:'user-B'}},
    /** 取得确定发出的资金接口，缺失即失败。 */
    take(path:string){const row=pending.find(item=>!item.taken&&item.path===path);assert.ok(row,'缺少请求 '+path);row.taken=true;return row},
  }
}
/** 排空组合函数请求和Vue侦听微任务。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}

test('组织押金付款绑定club_id和用途，未知响应保留原请求号',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('pay');s.page.amount.value='10'
  const first=s.page.submit();await flush();const original=s.take('/wallet/topups');assert.equal(original.body.club_id,'club-A');assert.equal(original.body.purpose,'club_platform');original.reject(new Error('响应未知'));await first
  assert.match(s.page.formError.value,/未知/)
  const retry=s.page.submit();await flush();const repeated=s.take('/wallet/topups');assert.equal(repeated.body.request_id,original.body.request_id);repeated.reject(new Error('仍未知'));await retry
})
test('微信回调后必须查询原支付单，取消不伪造有效押金',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('pay');s.page.amount.value='10';const submitting=s.page.submit();await flush()
  s.take('/wallet/topups').resolve({mode:'wechat',payment:{id:'topup',status:'pending_payment'},pay_params:{}});await flush();assert.equal(s.payments.length,1)
  s.payments[0].reject(new Error('取消支付'));await flush();s.take('/wallet/topups/topup/sync').resolve({id:'topup',status:'pending_payment'});await submitting
  assert.equal(s.page.balance.value.amount_cents,100);assert.ok([...s.storage.keys()].some(key=>key.includes('club_platform')))
})
test('切换账号后旧组织付款响应不能拉起微信或回填余额',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('pay');s.page.amount.value='10';const submitting=s.page.submit();await flush();const old=s.take('/wallet/topups')
  s.switchUser();await flush();old.resolve({mode:'wechat',payment:{id:'old',status:'pending_payment'},pay_params:{}});await submitting
  assert.equal(s.payments.length,0);assert.equal(s.page.operation.value,null)
})
test('成员俱乐部押金退款不使用个人或组织平台押金范围',async t=>{
  const s=harness(t,'member_club');s.hooks.show();await flush();s.page.open('refund');assert.equal(s.page.amount.value,'0.25')
  const submitting=s.page.submit();await flush();const writing=s.take('/wallet/deposit-refunds');assert.equal(writing.body.scope,'member_club');assert.equal(writing.body.club_id,'club-A');assert.equal(writing.body.amount_cents,25)
  writing.resolve({id:'refund',status:'processing',amount_cents:25});await submitting;assert.equal(s.page.operation.value,null)
})
test('俱乐部提现强制填写负责人实名且不会动用押金',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('withdraw');s.page.amount.value='101';s.page.userName.value='负责人';await s.page.submit();assert.match(s.page.formError.value,/超过/);assert.equal(s.pending.length,0)
  s.page.amount.value='100';s.page.userName.value='';await s.page.submit();assert.match(s.page.formError.value,/实名/)
  s.page.userName.value='负责人';const applying=s.page.submit();await flush();const writing=s.take('/clubs/club-A/withdrawals');assert.equal(writing.body.user_name,'负责人');assert.equal(writing.body.amount_cents,10000)
  s.hooks.hide();assert.equal(s.page.userName.value,'');writing.resolve({id:'withdrawal',status:'pending_review'});await applying;assert.equal(s.page.userName.value,'')
})
test('已存在的未付款原单恢复时不新建另一笔付款',async t=>{
  const s=harness(t);s.hooks.show();await flush();const resuming=s.page.resume({id:'original',purpose:'club_platform',club_id:'club-A',amount_cents:100,status:'pending_payment'});await flush()
  s.take('/wallet/topups/original/pay').resolve({mode:'dev',payment:{id:'original',status:'pending_payment'}});await flush();s.take('/wallet/topups/original/sync').resolve({id:'original',status:'paid'});await resuming
  assert.equal(s.pending.filter(item=>item.path==='/wallet/topups').length,0)
})

test('并行查单已确认原提交时关闭原表单，迟到超时不诱导第二笔付款退款或提现',async t=>{
  for(const kind of ['pay','refund','withdraw'] as const)await t.test(kind,async child=>{
    const s=harness(child);s.hooks.show();await flush();s.page.open(kind);s.page.amount.value=kind==='refund'?'1':'10';if(kind==='withdraw')s.page.userName.value='负责人'
    const submitting=s.page.submit();await flush();const original=s.take(kind==='pay'?'/wallet/topups':kind==='refund'?'/wallet/deposit-refunds':'/clubs/club-A/withdrawals')
    const cents=kind==='refund'?100:1000,record={id:'confirmed',request_id:original.body.request_id,amount_cents:cents,club_id:'club-A',purpose:'club_platform',scope:'club_platform',status:kind==='pay'?'paid':kind==='refund'?'processing':'pending_review'}
    if(kind==='pay')s.histories.topups=[record];else if(kind==='refund')s.histories.refunds=[record];else s.histories.withdrawals=[record]
    await s.page.load();assert.equal(s.page.operation.value,null)
    original.reject(new Error('原提交响应迟到超时'));await submitting
    assert.equal(s.page.formError.value,'');assert.equal(s.page.operation.value,null);assert.equal(s.pending.length,1)
  })
})
