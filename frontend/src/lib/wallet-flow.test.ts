/** 文件作用：执行真实钱包逻辑与支付封装，验证金额、原单幂等、微信回调查单及切号隔离。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { reactive,ref,computed,watch,effectScope } from 'vue'
import { executable,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import { parseWalletAmount } from './wallet'

/** 执行生产组合函数，只控制HTTP完成顺序和原生支付回调。 */
function harness(t:TestContext){
  const storage=new Map<string,string>([['jijie_token','token-A']]),pending:DeferredRequest[]=[],notices:unknown[]=[],payments:Array<{success:()=>void;fail:(error:unknown)=>void}>=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  let revision=0
  let phone=async()=>true
  const histories:{topups:unknown[];withdrawals:unknown[];refunds:unknown[]}={topups:[],withdrawals:[],refunds:[]}
  const state=reactive({user:{id:'A',phone_verified:true},ready:true})
  const balance={available_cents:10000,recharge_cents:0,spendable_cents:10000,pending_cents:0,withdrawal_frozen_cents:0,deposit_cents:0,deposit_refunding_cents:0,settlement_hold_days:7,dev_mode:true,currency:'CNY'}
  const context=vm.createContext({reactive,ref,computed,watch,state,init:async()=>{},sessionRevision:()=>revision,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>storage.set(key,value),removeStorageSync:(key:string)=>storage.delete(key),requestPayment:(options:typeof payments[number])=>payments.push(options)},
    token:()=>storage.get('jijie_token'),notify:(value:unknown)=>notices.push(value),go:()=>{},requirePhone:()=>phone(),
    onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},
    /** 查询默认返回合法空列表，资金写入和查单逐个受控。 */
    api(path:string,method='GET',body:Record<string,unknown>={}){
      if(method==='GET')return Promise.resolve(path==='/wallet'?{...balance}:path==='/wallet/topups'?histories.topups:path==='/wallet/withdrawals'?histories.withdrawals:path==='/wallet/deposit-refunds'?histories.refunds:[])
      return new Promise((resolve,reject)=>pending.push({path,method,body,resolve,reject,taken:false}))
    },
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('./wallet.ts',import.meta.url),'utf8')),context)
  vm.runInContext(executable(readFileSync(new URL('./payment.ts',import.meta.url),'utf8')),context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(readFileSync(new URL('./use-wallet.ts',import.meta.url),'utf8'))+'\nglobalThis.page=useWallet()',context));t.after(()=>scope.stop())
  return{page:context.page,hooks,pending,storage,notices,payments,balance,histories,
    /** 控制手机号原生授权前后的页面生命周期。 */
    setPhone(callback:()=>Promise<boolean>){phone=callback},
    /** 真实会话变化触发watch清理，不直接操作页面内部状态。 */
    switchAccount(){revision++;storage.set('jijie_token','token-B');state.user={id:'B',phone_verified:true}},
    /** 读取实际发出的接口，不存在则让测试立即失败。 */
    take(path:string){const result=pending.find(row=>!row.taken&&row.path===path);assert.ok(result,'缺少请求 '+path);result.taken=true;return result},
  }
}
/** 排空响应与watch微任务。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
const topup={id:'topup-A',purpose:'balance',amount_cents:1000,status:'pending_payment',refunded_cents:0,created_at:''}
const params={timeStamp:'1',nonceStr:'x',package:'prepay_id=test',signType:'RSA',paySign:'signature'}

test('钱包金额严格拒绝负数、指数、超额与多余小数',()=>{
  assert.equal(parseWalletAmount('12.34'),1234);assert.equal(parseWalletAmount('1'),100)
  for(const amount of ['0','0.99','-1','1e3','1.001','NaN','100000.01',''])assert.throws(()=>parseWalletAmount(amount))
})

test('微信原生付款成功后仍须原单查单，不能直接增发余额',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('balance');s.page.amount.value='10'
  const first=s.page.submit();await flush();await s.page.submit();assert.equal(s.pending.length,1)
  s.take('/wallet/topups').resolve({payment:topup,mode:'wechat',pay_params:params});await flush()
  assert.equal(s.payments.length,1);assert.equal(s.page.wallet.value.recharge_cents,0)
  s.payments[0].success();await flush();assert.equal(s.page.wallet.value.recharge_cents,0)
  s.balance.recharge_cents=1000;s.take('/wallet/topups/topup-A/sync').resolve({...topup,status:'paid'});await first
  assert.equal(s.page.wallet.value.recharge_cents,1000);assert.equal(s.page.operation.value,null)
})

test('充值请求未知时重复提交沿原请求号，失败保留输入',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('deposit');s.page.amount.value='50'
  const first=s.page.submit();await flush();const original=s.take('/wallet/topups');original.reject(new Error('网络结果未知'));await first
  assert.equal(s.page.amount.value,'50');assert.match(s.page.formError.value,/未知/)
  const retry=s.page.submit();await flush();const repeated=s.take('/wallet/topups');assert.equal(repeated.body.request_id,original.body.request_id);assert.equal(repeated.body.purpose,'deposit')
  repeated.reject(new Error('未配置微信支付'));await retry;assert.equal(s.page.wallet.value.deposit_cents,0)
})

test('旧账号迟到的支付参数不允许拉起新账号的微信付款',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('balance');s.page.amount.value='10';const first=s.page.submit();await flush();const writing=s.take('/wallet/topups')
  s.switchAccount();await flush();writing.resolve({payment:topup,mode:'wechat',pay_params:params});await first
  assert.equal(s.payments.length,0);assert.equal(s.page.operation.value,null);assert.equal(s.pending.filter(row=>row.path.endsWith('/sync')).length,0)
})

test('离开页面后迟到支付参数不弹付款页，原请求号可恢复',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('balance');s.page.amount.value='10';const first=s.page.submit();await flush();const writing=s.take('/wallet/topups');s.hooks.hide()
  writing.resolve({payment:topup,mode:'wechat',pay_params:params});await first
  assert.equal(s.payments.length,0);assert.ok([...s.storage.keys()].some(key=>key.startsWith('jijie_wallet_intent:')))
})

test('原生付款隐藏页面后仍用原账号查询，不靠隐藏状态误报失败',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('balance');s.page.amount.value='10';const first=s.page.submit();await flush()
  s.take('/wallet/topups').resolve({payment:topup,mode:'wechat',pay_params:params});await flush();s.hooks.hide();s.payments[0].success();await flush()
  s.take('/wallet/topups/topup-A/sync').resolve({...topup,status:'paid'});await first
  assert.equal(s.notices.length,0);s.hooks.show();await flush();assert.equal(s.page.operation.value,null)
})

test('提现不能动用充值余额且实名只在达到门槛时发送',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('withdrawal');s.page.amount.value='101';await s.page.submit();assert.equal(s.pending.length,0);assert.match(s.page.formError.value,/超过/)
  s.page.wallet.value.available_cents=300000;s.page.amount.value='2000';await s.page.submit();assert.equal(s.pending.length,0);assert.match(s.page.formError.value,/实名/)
  s.page.userName.value='测试实名';const applying=s.page.submit();await flush();const writing=s.take('/wallet/withdrawals');assert.equal(writing.body.user_name,'测试实名');writing.resolve({id:'withdraw-A',status:'pending_review'});await applying
  assert.equal(s.page.userName.value,'');assert.equal(s.page.wallet.value.available_cents,10000)
})

test('退押金的服务端进行中订单拒绝保留金额且不移除实际押金',async t=>{
  const s=harness(t);s.balance.deposit_cents=10000;s.hooks.show();await flush();s.page.open('refund');assert.equal(s.page.amount.value,'100')
  const submitting=s.page.submit();await flush();s.take('/wallet/deposit-refunds').reject(new Error('存在进行中订单，暂不可退还'));await submitting
  assert.equal(s.page.wallet.value.deposit_cents,10000);assert.equal(s.page.amount.value,'100');assert.match(s.page.formError.value,/进行中/)
})

test('个人押金不足1元的尾款可申请原路退清，不能被充值最低额误拦',async t=>{
  const s=harness(t);s.balance.deposit_cents=50;s.hooks.show();await flush();s.page.open('refund')
  assert.equal(s.page.amount.value,'0.5');const submitting=s.page.submit();await flush();const original=s.take('/wallet/deposit-refunds')
  assert.equal(original.body.amount_cents,50);original.resolve({id:'small-refund',amount_cents:50,status:'processing'});await submitting
  assert.equal(s.page.operation.value,null)
})

test('隐藏或销毁清除实名与短暂收款凭据',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.userName.value='不应缓存';s.page.withdrawals.value=[{id:'w',confirmation:{package_info:'secret'}}];s.hooks.hide()
  assert.equal(s.page.userName.value,'');assert.equal(s.page.withdrawals.value[0].confirmation,undefined)
  s.hooks.unload();assert.equal(s.page.wallet.value,null);assert.equal(s.storage.has('user_name'),false)
})


test('未知充值从同步确认到账后，同金额新充值换号且旧记录不删除新意图',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('balance');s.page.amount.value='10'
  const first=s.page.submit();await flush();const original=s.take('/wallet/topups');original.reject(new Error('未知'));await first
  const record={...topup,status:'paid',request_id:original.body.request_id};s.histories.topups=[record]
  const syncing=s.page.syncRecord('topup',topup.id);await flush();s.take('/wallet/topups/topup-A/sync').resolve(record);await syncing
  assert.ok(![...s.storage.keys()].some(key=>key.startsWith('jijie_wallet_intent:')))
  s.page.open('balance');s.page.amount.value='10';const next=s.page.submit();await flush();const creating=s.take('/wallet/topups');assert.notEqual(creating.body.request_id,original.body.request_id)
  await s.page.load();const key=[...s.storage.keys()].find(value=>value.startsWith('jijie_wallet_intent:'));assert.ok(key);assert.equal(s.storage.get(key),creating.body.request_id)
  creating.reject(new Error('保留新原号'));await next
})

test('同步仍未付款的充值保留原请求号，不误认为新充值',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('deposit');s.page.amount.value='10'
  const first=s.page.submit();await flush();const original=s.take('/wallet/topups');original.reject(new Error('未知'));await first
  const syncing=s.page.syncRecord('topup',topup.id);await flush();s.take('/wallet/topups/topup-A/sync').resolve({...topup,purpose:'deposit',request_id:original.body.request_id});await syncing
  const key=[...s.storage.keys()].find(value=>value.startsWith('jijie_wallet_intent:'));assert.ok(key);assert.equal(s.storage.get(key),original.body.request_id)
})

test('提现未知后列表确认原申请存在，失败后同金额可新提且旧记录不清新号',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('withdrawal');s.page.amount.value='10'
  const first=s.page.submit();await flush();const original=s.take('/wallet/withdrawals');original.reject(new Error('未知'));await first
  s.histories.withdrawals=[{id:'withdraw-A',amount_cents:1000,status:'failed',request_id:original.body.request_id}];await s.page.load()
  assert.ok(![...s.storage.keys()].some(key=>key.startsWith('jijie_wallet_intent:')))
  const second=s.page.submit();await flush();const next=s.take('/wallet/withdrawals');assert.notEqual(next.body.request_id,original.body.request_id)
  await s.page.load();const key=[...s.storage.keys()].find(value=>value.startsWith('jijie_wallet_intent:'));assert.ok(key);assert.equal(s.storage.get(key),next.body.request_id)
  next.resolve({id:'withdraw-B',amount_cents:1000,status:'pending_review',request_id:next.body.request_id});await second;assert.equal(s.page.operation.value,null)
})

test('退押金未知后同步查到处理中原申请即解除缓存，不重用旧退款请求号',async t=>{
  const s=harness(t);s.balance.deposit_cents=10000;s.hooks.show();await flush();s.page.open('refund')
  const first=s.page.submit();await flush();const original=s.take('/wallet/deposit-refunds');original.reject(new Error('未知'));await first
  const syncing=s.page.syncRecord('refund','refund-A');await flush();s.take('/wallet/deposit-refunds/refund-A/sync').resolve({id:'refund-A',amount_cents:10000,status:'processing',request_id:original.body.request_id});await syncing
  const next=s.page.submit();await flush();const creating=s.take('/wallet/deposit-refunds');assert.notEqual(creating.body.request_id,original.body.request_id);creating.reject(new Error('停止'));await next
})

test('跨设备恢复充值按服务端原单付款，不建立同金额无关请求缓存',async t=>{
  const s=harness(t);s.hooks.show();await flush();const original={...topup,request_id:'original-request'}
  const resuming=s.page.resumeTopup(original);await flush();s.take('/wallet/topups/topup-A/pay').resolve({payment:{...original,status:'paid'},mode:'wechat'});await flush();s.take('/wallet/topups/topup-A/sync').resolve({...original,status:'paid'});await resuming
  assert.ok(![...s.storage.keys()].some(key=>key.startsWith('jijie_wallet_intent:')));assert.equal(s.pending.filter(row=>row.path==='/wallet/topups').length,0)
  const key='jijie_wallet_intent:A:balance:1000';s.storage.set(key,'future-request')
  const old=s.page.resumeTopup(original);await flush();s.take('/wallet/topups/topup-A/pay').resolve({payment:{...original,status:'paid'},mode:'wechat'});await flush();s.take('/wallet/topups/topup-A/sync').resolve({...original,status:'paid'});await old
  assert.equal(s.storage.get(key),'future-request')
})

test('手机号授权导致隐藏会清除姓名界面，但本次已校验提现仍发送正确短暂实名',async t=>{
  const s=harness(t);s.balance.available_cents=300000;s.hooks.show();await flush();s.page.open('withdrawal');s.page.amount.value='2000';s.page.userName.value='测试实名'
  let finish:(value:boolean)=>void=()=>{};s.setPhone(()=>new Promise(resolve=>{finish=resolve}))
  const applying=s.page.submit();await flush();s.hooks.hide();assert.equal(s.page.userName.value,'');s.hooks.show();await flush();finish(true);await flush()
  const writing=s.take('/wallet/withdrawals');assert.equal(writing.body.user_name,'测试实名');writing.resolve({id:'withdraw-A',request_id:writing.body.request_id});await applying
  assert.equal(s.page.userName.value,'');assert.ok(![...s.storage.values()].includes('测试实名'))
})

test('未知资金提交已由并行读取确认时关闭原表单，迟到错误不诱导重复提交',async t=>{
  const s=harness(t);s.hooks.show();await flush();s.page.open('withdrawal');s.page.amount.value='10'
  const applying=s.page.submit();await flush();const writing=s.take('/wallet/withdrawals')
  s.histories.withdrawals=[{id:'withdraw-A',amount_cents:1000,status:'pending_review',request_id:writing.body.request_id}];await s.page.load()
  assert.equal(s.page.operation.value,null);writing.reject(new Error('原请求响应丢失'));await applying
  assert.equal(s.page.formError.value,'');assert.ok(![...s.storage.keys()].some(key=>key.startsWith('jijie_wallet_intent:')))
})
