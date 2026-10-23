/** 文件作用：执行三个真实资金管理页，验证来源复核阻断、原单同步、跨会话隔离和费率整数转换。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed,effectScope,nextTick,reactive,ref,watch } from 'vue'
import { executable,pageScript,injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest,PageLifecycle } from './sfc-test'
import type { FinancialSettings,WalletWithdrawal,WithdrawalAudit } from '../types/wallet'

/** 页面VM依赖真实API方法和Vue响应式实现，仅将网络与原生生命周期设为可控制边界。 */
function harness(t:TestContext,name:'detail'|'index'|'settings'='detail',devMode=false){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],routes:string[]=[]
  const state=reactive({ready:true,devMode,user:{id:'admin-A',role:'admin'}})
  let authToken='token-A',revision=1
  const hooks:PageLifecycle={load:()=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({ref,reactive,computed,watch,nextTick,state,token:()=>authToken,sessionRevision:()=>revision,init:async()=>{},notify:(value:unknown)=>notices.push(value),go:(url:string)=>routes.push(url),time:(value:string)=>value,onLoad:(callback:PageLifecycle['load'])=>{hooks.load=callback},onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('./wallet.ts',import.meta.url),'utf8')),context)
  const exposed={detail:'load,prepare,confirmAction,audit,verified,fresh,canApprove,canSynchronize,pendingAction,rejectReason,devOutcome,actionError,busy,error',index:'load,chooseStatus,withdrawals,status,hasMore,error',settings:'load,save,form,formError,loaded,error'}[name]
  const scope=effectScope()
  scope.run(()=>vm.runInContext(executable(pageScript('manage/finance/'+name+'.vue'))+'\nglobalThis.page={'+exposed+'}',context))
  hooks.load({id:'request-A'})
  t.after(()=>{hooks.unload();scope.stop()})
  return{page:context.page,requests,notices,routes,hooks,state,
    /** 取得实际发出的单个请求，禁止将缺少网络调用的假通过当作验证结果。 */
    take(path:string,method='GET'){const request=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(request,`缺少请求 ${method} ${path}`);request.taken=true;return request},
    /** 隐藏后更换管理员，触发页面真实的同步会话清理。 */
    switchAdmin(){hooks.hide();authToken='token-B';revision++;state.user={id:'admin-B',role:'admin'}},
  }
}

/** 推进到页面明确发出下一次异步请求的位置。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 构造包含五笔付款的核验返回，不复刻页面或后端审核算法。 */
function auditResult(status:WalletWithdrawal['status']='pending_review',passed=true):WithdrawalAudit{return{withdrawal:{id:'request-A',user_id:'provider-A',account_type:'user',amount_cents:10000,status,created_at:'2026-10-22T10:00:00Z',updated_at:'2026-10-22T10:00:00Z'},passed,issues:passed?[]:['付款来源已退款'],total_cents:10000,sources:Array.from({length:5},(_,index)=>({source_id:'source-'+index,order_id:'order-'+index,payment_id:'payment-'+index,amount_cents:2000,channel:'wechat'}))}}
/** 进入详情并让首次真实来源核验完成。 */
async function readyDetail(t:TestContext,value=auditResult(),devMode=false){const subject=harness(t,'detail',devMode);subject.hooks.show();await flush();subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(value);await flush();return subject}

test('来源核验未通过时无法打开转账确认且不发起出款',(async t=>{
  const subject=await readyDetail(t,auditResult('pending_review',false))
  assert.equal(subject.page.verified.value,false);assert.equal(subject.page.canApprove.value,false)
  subject.page.prepare('approve');await subject.page.confirmAction()
  assert.equal(subject.page.pendingAction.value,null)
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
}))

test('首次通过但确认前重核失败，必须阻断转账并显示具体异常',async t=>{
  const subject=await readyDetail(t)
  subject.page.prepare('approve')
  const pending=subject.page.confirmAction()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('pending_review',false))
  await pending
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
  assert.equal(subject.page.verified.value,false)
  assert.match(subject.page.actionError.value,/已退款/)
})

test('重核结果金额与申请不一致，即使passed为true也不能出款',async t=>{
  const subject=await readyDetail(t)
  subject.page.prepare('approve');const pending=subject.page.confirmAction()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve({...auditResult(),total_cents:9999})
  await pending
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
  assert.match(subject.page.actionError.value,/核验未通过/)
})

test('重核期间更换管理员，旧passed响应不能继续发出转账',async t=>{
  const subject=await readyDetail(t)
  subject.page.prepare('approve');const pending=subject.page.confirmAction()
  const old=subject.take('/admin/wallet/withdrawals/request-A/audit')
  subject.switchAdmin();subject.hooks.show();await flush()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('pending_review',false));await flush()
  old.resolve(auditResult());await pending
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
  assert.equal(subject.page.verified.value,false);assert.equal(subject.notices.length,0)
})

test('确认防连点且真实转账不含模拟参数，迟到结果不能污染新管理员',async t=>{
  const subject=await readyDetail(t)
  subject.page.prepare('approve');const pending=subject.page.confirmAction()
  await subject.page.confirmAction()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult());await flush()
  const writing=subject.take('/admin/wallet/withdrawals/request-A/execute','POST')
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,1)
  assert.deepEqual(Object.keys(writing.body),[])
  subject.switchAdmin();subject.hooks.show();await flush()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('pending_review',false));await flush()
  writing.resolve(auditResult('success').withdrawal);await pending
  assert.equal(subject.page.audit.value.withdrawal.status,'pending_review')
  assert.equal(subject.page.verified.value,false);assert.equal(subject.notices.length,0)
})

test('只有待审申请可拒绝，拒绝原因校验后调用原申请接口',async t=>{
  for(const status of ['processing','wait_user_confirm','success','failed','rejected'] as const){const subject=await readyDetail(t,auditResult(status));subject.page.prepare('reject');assert.equal(subject.page.pendingAction.value,null)}
  const subject=await readyDetail(t)
  subject.page.prepare('reject');subject.page.rejectReason.value='短';await subject.page.confirmAction()
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
  subject.page.rejectReason.value='核对后不符合提现规则'
  const pending=subject.page.confirmAction(),writing=subject.take('/admin/wallet/withdrawals/request-A/reject','POST')
  assert.equal(writing.body.reason,'核对后不符合提现规则')
  writing.resolve(auditResult('rejected').withdrawal);await flush()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('rejected'));await pending
  assert.equal(subject.page.pendingAction.value,null);assert.equal(subject.page.audit.value.withdrawal.status,'rejected')
})

test('处理中与待收款只能同步原申请，禁止重新审核和拒绝',async t=>{
  for(const status of ['processing','wait_user_confirm'] as const){
    const subject=await readyDetail(t,auditResult(status,false))
    assert.equal(subject.page.canApprove.value,false);assert.equal(subject.page.canSynchronize.value,true)
    subject.page.prepare('approve');assert.equal(subject.page.pendingAction.value,null)
    subject.page.prepare('reject');assert.equal(subject.page.pendingAction.value,null)
    subject.page.prepare('sync');const pending=subject.page.confirmAction()
    const writing=subject.take('/admin/wallet/withdrawals/request-A/execute','POST')
    assert.deepEqual(Object.keys(writing.body),[])
    assert.equal(subject.requests.filter(item=>item.method==='POST').length,1)
    writing.resolve(auditResult('success').withdrawal);await flush()
    subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('success'));await pending
    assert.equal(subject.page.audit.value.withdrawal.status,'success')
  }
})

test('开发模拟明确发送选定结果且仍须通过确认前来源重核',async t=>{
  const subject=await readyDetail(t,auditResult(),true)
  subject.page.prepare('approve');subject.page.devOutcome.value='failed';const pending=subject.page.confirmAction()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult());await flush()
  const writing=subject.take('/admin/wallet/withdrawals/request-A/execute','POST')
  assert.equal(writing.body.dev_outcome,'failed')
  writing.resolve(auditResult('failed').withdrawal);await flush()
  subject.take('/admin/wallet/withdrawals/request-A/audit').resolve(auditResult('failed'));await pending
})

test('来源刷新失败后保留可读明细但不允许沿用旧通过结果转账',async t=>{
  const subject=await readyDetail(t),pending=subject.page.load()
  await flush()
  subject.take('/admin/wallet/withdrawals/request-A/audit').reject(new Error('网络不可用'));await pending
  assert.equal(subject.page.audit.value.sources.length,5)
  assert.equal(subject.page.canApprove.value,false);assert.match(subject.page.error.value,/网络不可用/)
  subject.page.prepare('approve');await subject.page.confirmAction()
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)
})

test('交易规则直接对象回填，百分比和元转换整数且两个档位互不相加',async t=>{
  const subject=harness(t,'settings'),initial:FinancialSettings={free_platform_rate_bps:500,club_platform_rate_bps:1000,personal_platform_deposit_cents:10000,club_creation_deposit_cents:20000}
  subject.hooks.show();await flush();subject.take('/admin/financial-settings').resolve(initial);await flush()
  assert.equal(subject.page.form.freeRate,'5.00');assert.equal(subject.page.form.clubDeposit,'200.00');assert.equal(subject.page.form.personalDeposit,'100.00')
  Object.assign(subject.page.form,{freeRate:'0.15',clubRate:'99.99',personalDeposit:'88.88',clubDeposit:'0.01'})
  const pending=subject.page.save(),writing=subject.take('/admin/financial-settings','PATCH')
  assert.equal(writing.body.free_platform_rate_bps,15);assert.equal(writing.body.club_platform_rate_bps,9999);assert.equal(writing.body.club_creation_deposit_cents,1);assert.equal(writing.body.personal_platform_deposit_cents,8888)
  writing.resolve(writing.body);await pending
  assert.equal(subject.page.form.freeRate,'0.15')
  for(const value of ['100.01','-1','1e2','0.001']){subject.page.form.freeRate=value;await subject.page.save()}
  assert.equal(subject.requests.filter(item=>item.method==='PATCH').length,1)
  subject.page.form.freeRate='0';subject.page.form.clubDeposit='0';const zero=subject.page.save(),zeroWrite=subject.take('/admin/financial-settings','PATCH')
  assert.equal(zeroWrite.body.club_creation_deposit_cents,0);zeroWrite.resolve(zeroWrite.body);await zero
})

test('旧管理员迟到的交易规则保存不能覆盖新管理员草稿',async t=>{
  const subject=harness(t,'settings'),first={free_platform_rate_bps:500,club_platform_rate_bps:1000,personal_platform_deposit_cents:10000,club_creation_deposit_cents:20000}
  subject.hooks.show();await flush();subject.take('/admin/financial-settings').resolve(first);await flush()
  subject.page.form.freeRate='6';const pending=subject.page.save(),old=subject.take('/admin/financial-settings','PATCH')
  subject.switchAdmin();subject.hooks.show();await flush();subject.take('/admin/financial-settings').resolve({...first,free_platform_rate_bps:700});await flush()
  old.resolve({...first,free_platform_rate_bps:600});await pending
  assert.equal(subject.page.form.freeRate,'7.00');assert.equal(subject.notices.length,0)
})

test('独立资金列表筛选的迟到响应不覆盖新状态，错误保留同账号数据',async t=>{
  const subject=harness(t,'index');subject.hooks.show();await flush()
  const old=subject.take('/admin/wallet/withdrawals?status=pending_review&limit=20&offset=0')
  subject.page.chooseStatus('processing');await flush()
  subject.take('/admin/wallet/withdrawals?status=processing&limit=20&offset=0').resolve([auditResult('processing').withdrawal]);await flush()
  old.resolve([{...auditResult().withdrawal,id:'old'}]);await flush()
  assert.equal(subject.page.withdrawals.value[0].id,'request-A')
  const refreshing=subject.page.load();await flush();subject.take('/admin/wallet/withdrawals?status=processing&limit=20&offset=0').reject(new Error('刷新失败'));await refreshing
  assert.equal(subject.page.withdrawals.value[0].id,'request-A');assert.match(subject.page.error.value,/刷新失败/)
})
