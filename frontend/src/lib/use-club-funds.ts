/** 文件作用：管理成员托管押金与俱乐部独立资金，按账号、组织和用途隔离请求、原单及实名。 */
import { computed, ref, watch } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import { state, init, sessionRevision } from './store'
import { token, errorMessage } from '../api/client'
import { getClub } from '../api/clubs.api'
import { getClubDeposits, getClubWallet, listClubWithdrawals, listClubWalletEntries, createClubWithdrawal } from '../api/finance.api'
import { createWalletTopup, payWalletTopup, syncWalletTopup, listWalletTopups, listDepositRefunds, createDepositRefund, syncDepositRefund, syncWalletWithdrawal } from '../api/wallet.api'
import type { Club, ClubDetail } from '../types/models'
import type { ClubDeposits, ClubWallet, WalletTopup, WalletDepositRefund, WalletWithdrawal, WalletEntry } from '../types/wallet'
import { fundsIntentKey, mergeFundRecords } from './finance'
import { newWalletRequestID, parseWalletAmount } from './wallet'
import { requestWechatPayment, confirmWechatTransfer } from './payment'
import { requirePhone } from './phone'
import { requireIdentity } from './identity'
import { go, notify } from './format'
import { canConfirmClubTransfer,isClubManager } from './management-permissions'
import { beginFinancialActivity } from './update-manager'

/** 根据资金用途加载当前成员押金或负责人独立钱包，不在本地推算到账。 */
export function useClubFunds(scope: 'member_club' | 'club_platform', options: { summaryOnly?: boolean } = {}) {
  const id=ref(''), club=ref<Club|null>(null), detail=ref<ClubDetail|null>(null), deposits=ref<ClubDeposits|null>(null), wallet=ref<ClubWallet|null>(null)
  const entries=ref<WalletEntry[]>([]), topups=ref<WalletTopup[]>([]), refunds=ref<WalletDepositRefund[]>([]), withdrawals=ref<WalletWithdrawal[]>([])
  const hasMore=ref(false), loading=ref(false), busy=ref(false), error=ref(''), formError=ref(''), operation=ref<'pay'|'refund'|'withdraw'|null>(null), amount=ref(''), userName=ref('')
  let visible=false, disposed=false, loadVersion=0, actionVersion=0, hideRevision=0
  let lastFailedAppend=false
  const historyPageSize=30
  const cursors={entries:{offset:0,more:true},topups:{offset:0,more:true},withdrawals:{offset:0,more:true},refunds:{offset:0,more:true}}
  let activeRequest:{ownerID:string;clubID:string;kind:string;cents:number;requestID:string}|null=null
  const balance=computed(()=>scope==='club_platform'?deposits.value?.platform:deposits.value?.membership)
  const canManage=computed(()=>isClubManager(detail.value))
  const canConfirmTransfer=computed(()=>canConfirmClubTransfer(state.user,club.value))
  /** 捕获具体账号、令牌、俱乐部和会话，原生页面往返不改变交易归属。 */
  function session() { return { id:state.user?.id||'', token:token(), revision:sessionRevision(), clubID:id.value } }
  /** 迟到结果不能进入另一个组织或账号，页面销毁后不得继续操作。 */
  function same(owner:ReturnType<typeof session>):boolean { return !disposed&&owner.id===(state.user?.id||'')&&owner.token===token()&&owner.revision===sessionRevision()&&owner.clubID===id.value }
  /** 清除上个身份的组织资金、表单与收款秘密。 */
  function clear():void { activeRequest=null;loadVersion++; actionVersion++; club.value=null;detail.value=null;deposits.value=null;wallet.value=null;entries.value=[];topups.value=[];refunds.value=[];withdrawals.value=[];operation.value=null;userName.value='';amount.value='';error.value='';formError.value='';busy.value=false;loading.value=false;hasMore.value=false;lastFailedAppend=false;Object.values(cursors).forEach(cursor=>{cursor.offset=0;cursor.more=true}) }
  /** 服务端已接收的原请求才移除对应缓存，旧记录不能删除后来的同金额意图。 */
  function reconcile(owner:ReturnType<typeof session>,kind:string,cents:number,requestID?:string):void {
    if(!same(owner)||!requestID)return
    const key=fundsIntentKey(owner.id,owner.clubID,scope+':'+kind,cents)
    if(String(uni.getStorageSync(key)||'')===requestID)uni.removeStorageSync(key)
    if(activeRequest?.ownerID===owner.id&&activeRequest.clubID===owner.clubID&&activeRequest.kind===kind&&activeRequest.cents===cents&&activeRequest.requestID===requestID&&operation.value===kind){operation.value=null;userName.value='';formError.value=''}
  }
  /** 首屏读取组织和押金，触底仅追加有权限的本人组织历史，不重置资金表单或查原付款单。 */
  async function load(append=false):Promise<void> {
    if(!visible||disposed||!id.value||(append&&(loading.value||!hasMore.value)))return
    const current=++loadVersion;loading.value=true;error.value='';let owner:ReturnType<typeof session>|undefined
    try {
      await init();if(!visible||disposed||current!==loadVersion)return;owner=session()
      const result=append?detail.value:await getClub(id.value)
      if(!result||!visible||current!==loadVersion||!same(owner))return
      club.value=result.club;detail.value=result
      if(options.summaryOnly||!state.user)return
      const summary=!append&&scope==='club_platform'&&result.can_manage?await getClubWallet(id.value):wallet.value
      if(!visible||current!==loadVersion||!same(owner))return
      const positions={entries:append?cursors.entries.offset:0,topups:append?cursors.topups.offset:0,withdrawals:append?cursors.withdrawals.offset:0,refunds:append?cursors.refunds.offset:0}
      const manages=scope==='club_platform'&&result.can_manage
      const [security,payments,returned,history,ledger]=await Promise.all([
        append?Promise.resolve(deposits.value):getClubDeposits(id.value),
        !append||cursors.topups.more?listWalletTopups({club_id:id.value,purpose:scope,limit:historyPageSize,offset:positions.topups}):Promise.resolve(null),
        !append||cursors.refunds.more?listDepositRefunds({club_id:id.value,scope,limit:historyPageSize,offset:positions.refunds}):Promise.resolve(null),
        manages&&(!append||cursors.withdrawals.more)?listClubWithdrawals(id.value,{limit:historyPageSize,offset:positions.withdrawals}):Promise.resolve(null),
        manages&&(!append||cursors.entries.more)?listClubWalletEntries(id.value,{limit:historyPageSize,offset:positions.entries}):Promise.resolve(null),
      ])
      if(!visible||current!==loadVersion||!same(owner))return
      deposits.value=security;wallet.value=summary
      if(payments)topups.value=mergeFundRecords(append?topups.value:[],payments)
      if(returned)refunds.value=mergeFundRecords(append?refunds.value:[],returned)
      if(history)withdrawals.value=mergeFundRecords(append?withdrawals.value:[],history)
      if(ledger)entries.value=mergeFundRecords(append?entries.value:[],ledger)
      payments?.filter(item=>item.status==='paid'||item.status==='refunded').forEach(item=>reconcile(owner!,'pay',item.amount_cents,item.request_id))
      returned?.forEach(item=>reconcile(owner!,'refund',item.amount_cents,item.request_id))
      history?.forEach(item=>reconcile(owner!,'withdraw',item.amount_cents,item.request_id))
      const pages={entries:ledger,topups:payments,withdrawals:history,refunds:returned}
      ;(Object.keys(cursors) as Array<keyof typeof cursors>).forEach(key=>{const rows=pages[key];if(rows){cursors[key].offset=positions[key]+rows.length;cursors[key].more=rows.length===historyPageSize}else if(!manages&&(key==='entries'||key==='withdrawals'))cursors[key].more=false})
      hasMore.value=Object.values(cursors).some(cursor=>cursor.more);lastFailedAppend=false
    } catch(err) { if(visible&&current===loadVersion&&(!owner||same(owner))){lastFailedAppend=append;error.value=errorMessage(err)} }
    finally { if(current===loadVersion)loading.value=false }
  }
  /** 保留失败时原始分页动作，刷新失败不能错误地跳到下一页。 */
  async function retryFailed():Promise<void>{if(loading.value||busy.value||operation.value)return;await load(lastFailedAppend)}
  /** 打开明确用途的资金表单；访客保留登录后目标。 */
  function open(kind:'pay'|'refund'|'withdraw'):void { if(busy.value)return;if(!state.user){go('/pages/account/login?next='+encodeURIComponent('/pages/community/clubs/'+(scope==='club_platform'?'funds':'deposit')+'?id='+id.value));return}if(scope==='club_platform'&&!canManage.value)return notify('仅俱乐部管理者可操作');operation.value=kind;formError.value='';userName.value='';const shortfall=balance.value?.shortfall_cents??Math.max(0,(balance.value?.required_cents||0)-(balance.value?.amount_cents||0));amount.value=kind==='pay'?(shortfall>0?(shortfall/100).toFixed(2):''):kind==='refund'?String((balance.value?.amount_cents||0)/100):'' }
  /** 处理中不能关闭表单把未知结果误认为取消。 */
  function close():void { if(!busy.value){operation.value=null;userName.value='';formError.value=''} }
  /** 原生支付只完成交互，必须按原付款单查询服务端。 */
  async function pay(owner:ReturnType<typeof session>,cents:number,requestID:string,originalID?:string):Promise<void> {
    const result=originalID?await payWalletTopup(originalID):await createWalletTopup({purpose:scope,club_id:owner.clubID,amount_cents:cents,request_id:requestID})
    if(!same(owner)||!visible)return
    if(result.mode==='wechat'&&result.payment.status==='pending_payment'){if(!result.pay_params)throw new Error('付款参数缺失，请查询原付款单');try{await requestWechatPayment(result.pay_params)}catch{/* 取消或未知结果均查询原单，不从回调推断扣款。 */}}
    if(!same(owner))return
    const payment=await syncWalletTopup(result.payment.id)
    if(!same(owner))return
    if(payment.status==='paid'||payment.status==='refunded')reconcile(owner,'pay',cents,payment.request_id||requestID)
    if(visible)notify(payment.status==='paid'?'已确认押金到账':'支付尚未确认，请继续或同步原付款单')
  }
  /** 校验金额、手机号和实名后提交稳定业务号，跨账号迟到结果不回填。 */
  async function submit():Promise<void> {
    const kind=operation.value;if(!kind||busy.value||!visible||!state.user)return
    const owner=session(),action=++actionVersion,hiddenAtStart=hideRevision,finishFinancial=beginFinancialActivity();busy.value=true;formError.value=''
    try {
      const cents=parseWalletAmount(amount.value,kind==='withdraw'?20000000:10000000,kind==='withdraw'?(wallet.value?.withdrawal_min_cents||1):1)
      const name=userName.value.trim()
      if(kind==='withdraw'&&cents>(wallet.value?.available_cents||0))throw new Error('金额超过俱乐部可提现佣金')
      if(kind==='withdraw'&&!name)throw new Error('请填写俱乐部负责人微信实名认证姓名')
      if(kind==='refund'&&cents>(balance.value?.amount_cents||0))throw new Error('金额超过当前有效押金')
      if(!(await requirePhone())||!same(owner)||!visible)return
      if(kind==='pay'&&(!(await requireIdentity())||!same(owner)||!visible))return
      const key=fundsIntentKey(owner.id,owner.clubID,scope+':'+kind,cents),requestID=String(uni.getStorageSync(key)||newWalletRequestID());uni.setStorageSync(key,requestID)
      activeRequest={ownerID:owner.id,clubID:owner.clubID,kind,cents,requestID}
      if(kind==='pay')await pay(owner,cents,requestID)
      else if(kind==='refund'){const record=await createDepositRefund({scope,club_id:owner.clubID,amount_cents:cents,request_id:requestID});if(!same(owner))return;reconcile(owner,kind,cents,record.request_id||requestID);if(visible)notify('已申请原路退还，请查看资金进度')}
      else {const record=await createClubWithdrawal(owner.clubID,{amount_cents:cents,request_id:requestID,user_name:name});if(!same(owner))return;reconcile(owner,kind,cents,record.request_id||requestID);if(visible)notify('提现已提交，等待平台审核资金来源')}
      if(!same(owner))return;operation.value=null;userName.value='';if(visible)await load()
    } catch(err) { if(same(owner)&&visible){if(operation.value===kind)formError.value=errorMessage(err);else if(!operation.value&&hideRevision>hiddenAtStart)notify(err)} }
    finally { if(action===actionVersion){activeRequest=null;busy.value=false};finishFinancial() }
  }
  /** 跨设备恢复未付款原单，不按相同金额新建付款。 */
  async function resume(record:WalletTopup):Promise<void> { if(busy.value||!visible||!state.user)return;const owner=session();if(scope==='club_platform'&&record.user_id!==owner.id)return notify('仅原付款人可继续支付，请同步原付款单');const action=++actionVersion,finishFinancial=beginFinancialActivity();busy.value=true;try{if(!(await requirePhone())||!same(owner)||!visible)return;if(!(await requireIdentity())||!same(owner)||!visible)return;await pay(owner,record.amount_cents,record.request_id||'',record.id);if(same(owner)&&visible)await load()}catch(err){if(same(owner)&&visible)notify(err)}finally{if(action===actionVersion)busy.value=false;finishFinancial()} }
  /** 同步原支付、退款或提现，微信待收款确认后仍查询实际终态。 */
  async function sync(kind:'payment'|'refund'|'withdraw',recordID:string,confirm=false):Promise<void> {
    if(busy.value||!visible||!state.user)return;const owner=session(),action=++actionVersion,finishFinancial=beginFinancialActivity();busy.value=true
    try { if(kind==='payment')await syncWalletTopup(recordID);else if(kind==='refund')await syncDepositRefund(recordID);else{const record=await syncWalletWithdrawal(recordID);if(!same(owner)||!visible)return;if(confirm&&canConfirmClubTransfer(state.user,club.value,record)&&record.status==='wait_user_confirm'&&record.confirmation){try{await confirmWechatTransfer(record.confirmation)}finally{if(same(owner))await syncWalletWithdrawal(recordID)}}}if(same(owner)&&visible)await load() }
    catch(err){if(same(owner)&&visible)notify(err)}finally{if(action===actionVersion)busy.value=false;finishFinancial()}
  }
  /** 原生授权或支付暂时隐藏时清除实名与短暂收款凭据，仅使读取失效。 */
  function hide():void {visible=false;hideRevision++;loadVersion++;loading.value=false;operation.value=null;amount.value='';formError.value='';userName.value='';withdrawals.value=withdrawals.value.map(({confirmation,...item})=>item)}
  watch([()=>state.user?.id,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
  onLoad((options:Record<string,string|undefined>={})=>{id.value=options.id||'';if(!id.value)error.value='缺少俱乐部编号'})
  onShow(()=>{visible=true;void load()});onHide(hide);onUnload(()=>{disposed=true;hide();clear()})
  return {id,club,detail,deposits,balance,wallet,entries,topups,refunds,withdrawals,canManage,canConfirmTransfer,hasMore,loading,busy,error,formError,operation,amount,userName,load,retryFailed,open,close,submit,resume,sync}
}
