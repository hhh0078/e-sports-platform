/** 文件作用：执行真实组织资金组合函数，验证押金归属、未知原单、微信回查和账号隔离。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive, ref, computed, watch, effectScope } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest, PageLifecycle } from './sfc-test'
import { parseWalletAmount } from './wallet'

/** 使用真实业务方法，仅控制网络、微信交互完成顺序及账号变化。 */
function harness(t:TestContext,scope:'member_club'|'club_platform'='club_platform',summaryOnly=false){
  const storage=new Map<string,unknown>(),pending:DeferredRequest[]=[],notices:unknown[]=[],payments:Array<{resolve:()=>void;reject:(error:unknown)=>void}>=[]
  const hooks:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const state=reactive({user:{id:'user-A'},ready:true});let revision=0,bottom=()=>{}
  const wallet={available_cents:10000,recharge_cents:0,pending_cents:0,ongoing_cents:0,settling_cents:0,withdrawal_frozen_cents:0,deposit_cents:0,deposit_refunding_cents:0,withdrawal_min_cents:1,dev_mode:true,currency:'CNY',settlement_hold_days:7}
  const deposits={platform:{amount_cents:100,refunding_cents:0,required_cents:10000,shortfall_cents:9900,qualified:false,can_manage:true},membership:{amount_cents:25,refunding_cents:0,required_cents:25}}
  const histories={entries:[] as unknown[],topups:[] as unknown[],refunds:[] as unknown[],withdrawals:[] as unknown[]}
  const reads:string[]=[];let nextReadFailure=''
  const context=vm.createContext({ref,reactive,computed,watch,state,onReachBottom:(callback:()=>void)=>{bottom=callback},init:async()=>{},sessionRevision:()=>revision,token:()=>state.user.id+'token',parseWalletAmount,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:unknown)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key)},
    requirePhone:async()=>true,go:()=>{},notify:(value:unknown)=>notices.push(value),
    requestWechatPayment:()=>new Promise<void>((resolve,reject)=>payments.push({resolve,reject})),confirmWechatTransfer:async()=>{},
    onLoad:(callback:PageLifecycle['load'])=>hooks.load=callback,onShow:(callback:()=>void)=>hooks.show=callback,onHide:(callback:()=>void)=>hooks.hide=callback,onUnload:(callback:()=>void)=>hooks.unload=callback,
    /** 资金写入与原单同步由用例逐个结束；读取返回明确组织归属。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){
      if(method==='GET'){reads.push(path);if(nextReadFailure&&path.startsWith(nextReadFailure)){nextReadFailure='';return Promise.reject(new Error('读取失败'))};const url=new URL(path,'https://test.local'),base=url.pathname,offset=Number(url.searchParams.get('offset')||0),limit=Number(url.searchParams.get('limit')||30);return Promise.resolve(base==='/clubs/club-A'?{club:{id:'club-A',name:'测试俱乐部'},can_manage:true,members:[],services:[],my_membership:null}:base.endsWith('/deposits')?deposits:base.endsWith('/wallet')?{...wallet}:(base.endsWith('/withdrawals')?histories.withdrawals:base.endsWith('/entries')?histories.entries:base==='/wallet/topups'?histories.topups:base==='/wallet/deposit-refunds'?histories.refunds:[]).slice(offset,offset+limit))}
      return new Promise((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))
    },
  })
  injectBusinessAPIs(context)
  const scopeEffect=effectScope();scopeEffect.run(()=>vm.runInContext(executable(readFileSync(new URL('./use-club-funds.ts',import.meta.url),'utf8'))+'\nglobalThis.page=useClubFunds('+JSON.stringify(scope)+','+JSON.stringify({summaryOnly})+')',context));t.after(()=>scopeEffect.stop())
  hooks.load({id:'club-A'})
  return{
    /** 挂载负责人资金或成员押金页的真实原生分页回调，复用当前资金模型。 */
    attachBottom(){scopeEffect.run(()=>{vm.runInContext(executable(readFileSync(new URL('./native-page-refresh.ts',import.meta.url),'utf8')),context);vm.runInContext('(function(useClubFunds){'+executable(pageScript('community/clubs/'+(scope==='club_platform'?'funds':'deposit')+'.vue'))+'})(()=>globalThis.page)',context)})},
    /** 触发页面实际注册的触底事件。 */
    bottom(){bottom()},page:context.page,hooks,pending,storage,notices,payments,wallet,deposits,histories,reads,
    /** 让指定下一页读取失败，验证重试不会跳过历史。 */
    failRead(prefix:string){nextReadFailure=prefix},
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


test('组织补缴默认服务端差额，门槛提高不改变已托管原款',async t=>{
  const s=harness(t);s.deposits.platform.amount_cents=9950;s.deposits.platform.shortfall_cents=50;s.hooks.show();await flush();s.page.open('pay');assert.equal(s.page.amount.value,'0.50');assert.equal(s.page.balance.value.amount_cents,9950);const applying=s.page.submit();await flush();const request=s.take('/wallet/topups');assert.equal(request.body.amount_cents,50);request.reject(new Error('停止测试'));await applying;assert.equal(s.page.balance.value.amount_cents,9950)
})

test('组织资金表单隐藏返回清草稿且显示迟到失败，不丢原单恢复键',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('pay');s.page.amount.value='99';const applying=s.page.submit();await flush();const request=s.take('/wallet/topups');s.hooks.hide();assert.equal(s.page.operation.value,null);assert.equal(s.page.amount.value,'');s.hooks.show();await flush();request.reject(new Error('结果未知'));await applying;assert.equal(s.notices.length,1);assert.ok([...s.storage.keys()].some(key=>key.includes('club_platform')))
})

test('组织提现超过100条仍可触底读完，组织付款退款由服务端按本人用途筛选',async t=>{
  const s=harness(t);s.histories.entries=Array.from({length:35},(_,i)=>({id:'entry-'+i}));s.histories.withdrawals=Array.from({length:105},(_,i)=>({id:'withdraw-'+i,status:'pending_review',amount_cents:1}));s.histories.topups=Array.from({length:33},(_,i)=>({id:'payment-'+i,status:'pending_payment'}));s.histories.refunds=Array.from({length:32},(_,i)=>({id:'refund-'+i,status:'processing'}))
  s.hooks.show();await flush();assert.equal(s.page.withdrawals.value.length,30);s.page.open('withdraw');s.page.amount.value='10';s.page.userName.value='负责人实名';s.reads.length=0
  for(let i=0;i<3;i++)await s.page.load(true)
  assert.equal(s.page.withdrawals.value.length,105);assert.equal(s.page.entries.value.length,35);assert.equal(s.page.topups.value.length,33);assert.equal(s.page.refunds.value.length,32);assert.equal(s.page.hasMore.value,false)
  assert.ok(s.reads.includes('/clubs/club-A/withdrawals?limit=30&offset=90'));assert.equal(s.reads.some(path=>path==='/clubs/club-A'||path.endsWith('/wallet')),false)
  const topupRequest=s.reads.find(path=>path.startsWith('/wallet/topups?'));assert.ok(topupRequest);const query=new URL(topupRequest,'https://test.local').searchParams;assert.equal(query.get('club_id'),'club-A');assert.equal(query.get('purpose'),'club_platform')
  assert.equal(s.page.amount.value,'10');assert.equal(s.page.userName.value,'负责人实名');assert.equal(s.pending.length,0)
})

test('组织历史读取失败不清空押金输入，重试不推进失败页面',async t=>{
  const s=harness(t);s.histories.topups=Array.from({length:31},(_,i)=>({id:'payment-'+i,status:'pending_payment'}));s.hooks.show();await flush();s.page.open('pay');s.page.amount.value='98.76';s.failRead('/wallet/topups')
  await s.page.load(true);assert.equal(s.page.topups.value.length,30);assert.equal(s.page.amount.value,'98.76');assert.match(s.page.error.value,/读取失败/)
  await s.page.load(true);assert.equal(s.page.topups.value.length,31);assert.equal(s.page.operation.value,'pay');assert.equal(s.page.error.value,'')
  assert.equal(s.reads.filter(path=>path.includes('/wallet/topups?')&&path.includes('offset=30')).length,2)
})

test('俱乐部控制台只GET组织权限，不为下拉读取资金或历史',async t=>{
  const s=harness(t,'club_platform',true);s.hooks.show();await flush();assert.deepEqual(s.reads,['/clubs/club-A']);assert.equal(s.page.wallet.value,null);assert.equal(s.page.hasMore.value,false)
  await s.page.load();assert.deepEqual(s.reads,['/clubs/club-A','/clubs/club-A']);assert.equal(s.pending.length,0)
})

test('组织资金刷新与追加错误分别重试原动作，不用旧hasMore推断',async t=>{
  const s=harness(t);s.histories.topups=Array.from({length:40},(_,i)=>({id:'payment-'+i,status:'pending_payment'}));s.hooks.show();await flush();assert.equal(s.page.hasMore.value,true)
  s.failRead('/clubs/club-A');await s.page.load();s.reads.length=0;await s.page.retryFailed()
  assert.ok(s.reads.includes('/clubs/club-A'));assert.ok(s.reads.some(path=>path.startsWith('/wallet/topups?')&&path.includes('offset=0')));assert.equal(s.reads.some(path=>path.includes('offset=30')),false)
  s.failRead('/wallet/topups');await s.page.load(true);s.reads.length=0;await s.page.retryFailed();assert.equal(s.reads.length,1);assert.ok(s.reads[0].includes('offset=30'));assert.equal(s.page.topups.value.length,40)
})

test('两个俱乐部资金页面原生触底按失败方向重试，不跳过刷新或追加页',async t=>{for(const scope of ['club_platform','member_club'] as const)await t.test(scope,async subtest=>{const s=harness(subtest,scope);s.histories.topups=Array.from({length:40},(_,i)=>({id:'payment-'+i,status:'pending_payment'}));s.hooks.show();await flush();s.attachBottom();s.failRead('/clubs/club-A');await s.page.load();s.reads.length=0;s.bottom();await flush();assert.ok(s.reads.some(path=>path.startsWith('/wallet/topups')&&path.includes('offset=0')));assert.equal(s.reads.some(path=>path.includes('offset=30')),false);s.failRead('/wallet/topups');await s.page.load(true);s.reads.length=0;s.bottom();await flush();assert.equal(s.reads.length,1);assert.ok(s.reads[0].includes('offset=30'));assert.equal(s.page.topups.value.length,40)})})
