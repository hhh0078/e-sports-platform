/** 文件作用：管理钱包真实数据、原单恢复和资金提交生命周期，切号与隐藏时隔离私人内容及收款凭据。 */
import { ref,computed,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import { getWallet,listWalletEntries,listWalletTopups,listWalletWithdrawals,listDepositRefunds,createWalletTopup,payWalletTopup,syncWalletTopup,createWalletWithdrawal,syncWalletWithdrawal,createDepositRefund,syncDepositRefund } from '../api/wallet.api'
import type { Wallet,WalletEntry,WalletTopup,WalletWithdrawal,WalletDepositRefund } from '../types/wallet'
import { state,init,sessionRevision } from './store'
import { token,errorMessage } from '../api/client'
import { requirePhone } from './phone'
import { go,notify } from './format'
import { parseWalletAmount,newWalletRequestID } from './wallet'
import { requestWechatPayment,confirmWechatTransfer } from './payment'
/** 钱包弹层对应的唯一资金意图。 */
export type WalletOperation='balance'|'deposit'|'withdrawal'|'refund'
/** 按本人、资金用途及整数分定位原请求缓存，不保存实名或支付参数。 */
function intentKey(userID:string,kind:WalletOperation,cents:number):string{return 'jijie_wallet_intent:'+userID+':'+kind+':'+cents}
/** 同一账号同金额的未知请求跨页面重试保持原号，缓存不含实名或支付参数。 */
function intentID(userID:string,kind:WalletOperation,cents:number):{key:string;id:string}{
  const key=intentKey(userID,kind,cents)
  const id=String(uni.getStorageSync(key)||newWalletRequestID());uni.setStorageSync(key,id);return{key,id}
}
/** 为钱包页提供服务端余额、原单进度与提交动作，不在本地增减余额。 */
export function useWallet(){
  const wallet=ref<Wallet|null>(null),entries=ref<WalletEntry[]>([]),topups=ref<WalletTopup[]>([]),withdrawals=ref<WalletWithdrawal[]>([]),refunds=ref<WalletDepositRefund[]>([])
  const loading=ref(false),busy=ref(false),error=ref(''),operation=ref<WalletOperation|null>(null),amount=ref(''),userName=ref(''),formError=ref(''),filter=ref('all'),hasMore=ref(false)
  let visible=false,disposed=false,version=0
  let activeRequest:{ownerID:string;kind:WalletOperation;cents:number;id:string}|null=null
  /** 捕获资金请求真正所属的会话。 */
  function session(){return{id:state.user?.id||'',token:token(),revision:sessionRevision()}}
  /** 原生支付临时隐藏页面不会更换请求账号，页面销毁后禁止后续操作。 */
  function same(value:ReturnType<typeof session>):boolean{return!disposed&&!!value.id&&value.id===state.user?.id&&value.token===token()&&value.revision===sessionRevision()}
  /** 请求仍属当前可见页才更新私人余额与凭据。 */
  function current(value:ReturnType<typeof session>,requestVersion:number):boolean{return visible&&version===requestVersion&&same(value)}
  /** 清除上个账号的资金及表单，不保留实名和收款凭据。 */
  function clear(){activeRequest=null;wallet.value=null;entries.value=[];topups.value=[];withdrawals.value=[];refunds.value=[];operation.value=null;userName.value='';amount.value='';error.value='';formError.value='';hasMore.value=false;version++;loading.value=false}
  watch(()=>state.user?.id,()=>{clear();if(visible)void load()})
  /** 仅清理服务端确认存在的同账号原请求，旧记录不得删除未来同金额的新业务号。
   * 本次提交若已被并行查单确认，则关闭其表单，避免迟到网络错误诱导再次提交。
   */
  function reconcileRequest(owner:ReturnType<typeof session>,kind:WalletOperation,cents:number,requestID?:string){
    if(!same(owner)||!requestID)return
    const key=intentKey(owner.id,kind,cents)
    if(String(uni.getStorageSync(key)||'')===requestID)uni.removeStorageSync(key)
    if(activeRequest?.ownerID===owner.id&&activeRequest.kind===kind&&activeRequest.cents===cents&&activeRequest.id===requestID&&operation.value===kind){operation.value=null;userName.value='';formError.value=''}
  }
  /** 充值只有付款确认或退还后才完成意图，未付款和未知状态保留原号用于恢复。
   */
  function reconcileTopup(owner:ReturnType<typeof session>,record:WalletTopup,fallbackRequestID?:string){
    if(record.status==='paid'||record.status==='refunded')reconcileRequest(owner,record.purpose,record.amount_cents,record.request_id||fallbackRequestID)
  }
  /** 余额和资金单整体更新；失败保留上次真实金额并显示错误，绝不写假零余额。 */
  async function load(append=false){
    if(!visible||disposed)return
    const requestVersion=++version;loading.value=true
    let owner:ReturnType<typeof session>|undefined
    try{
      await init();if(!visible||requestVersion!==version)return
      if(!state.user){clear();return}
      owner=session()
      const offset=append?entries.value.length:0
      const summary=await getWallet()
      if(!current(owner,requestVersion))return
      // 余额读取会释放到期托管，随后读取账单才能包含这次结算分录。
      const histories=await Promise.all([listWalletEntries({limit:30,offset}),listWalletTopups(),listWalletWithdrawals(),listDepositRefunds()])
      if(!current(owner,requestVersion))return
      histories[1].forEach(record=>reconcileTopup(owner!,record));histories[2].forEach(record=>reconcileRequest(owner!,'withdrawal',record.amount_cents,record.request_id));histories[3].forEach(record=>reconcileRequest(owner!,'refund',record.amount_cents,record.request_id))
      wallet.value=summary;entries.value=append?[...entries.value,...histories[0]]:histories[0];topups.value=histories[1];withdrawals.value=histories[2];refunds.value=histories[3];hasMore.value=histories[0].length===30;error.value=''
    }catch(err){if(visible&&requestVersion===version&&(!owner||same(owner)))error.value=errorMessage(err)}finally{if(requestVersion===version)loading.value=false}
  }
  const filteredEntries=computed(()=>entries.value.filter(entry=>filter.value==='all'||(filter.value==='deposit'?!!entry.deposit_delta||!!entry.deposit_refunding_delta:filter.value==='withdrawal'?/withdraw/.test(entry.kind):/topup|recharge/.test(entry.kind)&&!!entry.recharge_delta)))
  /** 弹层每次显式打开才初始化草稿，不在错误时丢弃输入。 */
  function open(kind:WalletOperation){if(busy.value)return;if(!state.user){go('/pages/account/login?next='+encodeURIComponent('/pages/account/wallet'));return}operation.value=kind;amount.value=kind==='refund'&&wallet.value?String(wallet.value.deposit_cents/100):'';userName.value='';formError.value=''}
  /** 用户关闭普通表单时释放姓名，处理中保持交易锁。 */
  function close(){if(busy.value)return;operation.value=null;userName.value='';formError.value=''}
  /** 创建充值/押金支付并查原单，微信本地回调不会修改余额。 */
  async function payTopup(owner:ReturnType<typeof session>,kind:'balance'|'deposit',cents:number,intent?:ReturnType<typeof intentID>,originalID?:string){
    if(!originalID&&!intent)throw new Error('付款原请求缺失，请重新打开付款单')
    const result=originalID?await payWalletTopup(originalID):await createWalletTopup({purpose:kind,amount_cents:cents,request_id:intent!.id})
    if(!same(owner)||!visible)return
    let nativeError:unknown
    if(result.payment.status==='pending_payment'&&result.mode==='wechat'){
      if(!result.pay_params)throw new Error('微信支付参数缺失，请从原付款单同步状态')
      try{await requestWechatPayment(result.pay_params)}catch(err){nativeError=err}
    }
    if(!same(owner))return
    const payment=await syncWalletTopup(result.payment.id)
    if(!same(owner))return
    if(payment.status==='paid'||payment.status==='refunded'){reconcileTopup(owner,payment,result.payment.request_id||intent?.id);if(visible)notify(payment.status==='paid'?'已确认到账':'该付款单已退还')}
    else if(visible)notify(nativeError?'支付未确认，可从原付款单继续支付或同步':'支付处理中，请同步原付款单状态')
  }
  /** 恢复服务端本人原付款单，跨设备也不会错误创建另一笔充值。 */
  async function resumeTopup(record:WalletTopup){
    if(busy.value||!visible||!state.user)return
    const owner=session();busy.value=true
    try{if(!(await requirePhone())||!same(owner)||!visible)return;await payTopup(owner,record.purpose,record.amount_cents,undefined,record.id);if(same(owner))await load()}
    catch(err){if(same(owner)&&visible)notify(err)}finally{busy.value=false}
  }
  /** 同一提交锁覆盖手机号与资金动作，错误或网络未知保持原请求号。 */
  async function submit(){
    if(busy.value||!operation.value||!visible||!state.user)return
    const owner=session(),kind=operation.value;busy.value=true;formError.value=''
    try{
      const cents=parseWalletAmount(amount.value,kind==='withdrawal'?20000000:10000000)
      if(kind==='withdrawal'&&wallet.value&&cents>wallet.value.available_cents)throw new Error('提现金额超过可提现服务收入')
      const requestName=userName.value.trim()
      if(kind==='withdrawal'&&cents>=200000&&!requestName)throw new Error('单笔提现达到2000元，请填写微信实名认证姓名')
      if(!(await requirePhone())||!same(owner)||!visible)return
      const intent=intentID(owner.id,kind,cents)
      activeRequest={ownerID:owner.id,kind,cents,id:intent.id}
      if(kind==='balance'||kind==='deposit')await payTopup(owner,kind,cents,intent)
      else{
        const record=kind==='withdrawal'?await createWalletWithdrawal({amount_cents:cents,request_id:intent.id,...(cents>=200000?{user_name:requestName}:{})}):await createDepositRefund({amount_cents:cents,request_id:intent.id})
        if(!same(owner))return
        reconcileRequest(owner,kind,cents,record.request_id||intent.id);if(visible)notify(kind==='withdrawal'?'提现已提交，等待平台审核':'已申请原路退还，请查看退款进度')
      }
      if(!same(owner))return;operation.value=null;userName.value='';await load()
    }catch(err){if(same(owner)&&visible&&operation.value===kind)formError.value=errorMessage(err)}finally{activeRequest=null;busy.value=false}
  }
  /** 对原记录查单或拉起官方收款确认，状态始终取后端。 */
  async function syncRecord(kind:'topup'|'withdrawal'|'refund',id:string,confirm=false){
    if(busy.value||!visible||!state.user)return
    const owner=session();busy.value=true
    try{
      if(kind==='topup'){const record=await syncWalletTopup(id);reconcileTopup(owner,record)}
      else if(kind==='refund'){const record=await syncDepositRefund(id);reconcileRequest(owner,'refund',record.amount_cents,record.request_id)}
      else{
        const record=await syncWalletWithdrawal(id)
        reconcileRequest(owner,'withdrawal',record.amount_cents,record.request_id)
        if(!same(owner)||!visible)return
        if(confirm&&record.status==='wait_user_confirm'&&record.confirmation){try{await confirmWechatTransfer(record.confirmation)}finally{if(same(owner))await syncWalletWithdrawal(id)}}
      }
      if(same(owner))await load()
    }catch(err){if(same(owner)&&visible)notify(err);if(same(owner))await load()}finally{busy.value=false}
  }
  /** 官方支付页会暂时隐藏钱包，隐藏时仅清理实名、收款凭据与迟到读取。 */
  function hide(){visible=false;version++;loading.value=false;userName.value='';withdrawals.value=withdrawals.value.map(({confirmation,...record})=>record)}
  onShow(()=>{visible=true;void load()});onHide(hide);onUnload(()=>{disposed=true;hide();clear()})
  return{wallet,entries,topups,withdrawals,refunds,filteredEntries,loading,busy,error,operation,amount,userName,formError,filter,hasMore,load,open,close,submit,syncRecord,resumeTopup}
}
