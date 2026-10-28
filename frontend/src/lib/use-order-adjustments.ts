/** 文件作用：驱动订单费用协商、双方确认与独立补款，守护账号/页面竞态及未知资金结果。 */
import {computed,ref,watch,onBeforeUnmount} from 'vue'
import {onShow,onHide,onUnload} from '@dcloudio/uni-app'
import {state,sessionRevision} from './store'
import {token,errorMessage} from '../api/client'
import {listOrderAdjustments,createOrderAdjustment,respondOrderAdjustment,payOrderAdjustment,syncOrderAdjustment} from '../api/adjustments.api'
import {getWallet} from '../api/wallet.api'
import {requirePhone} from './phone'
import {requireIdentity} from './identity'
import {requestWechatPayment} from './payment'
import {beginFinancialActivity} from './update-manager'
import {newWalletRequestID} from './wallet'
import {parseAdjustmentAmount,adjustmentPending,canProposeAdjustment} from './order-adjustments'
import type {Order} from '../types/models'
import type {AdjustmentKind,OrderAdjustment} from '../types/adjustments'
import type {Wallet} from '../types/wallet'

/** 弹层草稿仅表达当前操作，不持有或持久化任何微信支付秘密。 */
export type AdjustmentDialog = 'create'|'accept'|'reject'|'withdraw'|'pay'
/** 以当前订单读取函数驱动共享组件，变更后通知父页面重新取订单与聊天。 */
export function useOrderAdjustments(readOrder:()=>Order|null,changed:()=>void){
  const records=ref<OrderAdjustment[]>([]),loading=ref(false),busy=ref(false),error=ref(''),formError=ref(''),dialog=ref<AdjustmentDialog|null>(null),kind=ref<AdjustmentKind>('increase'),amount=ref(''),reason=ref(''),selected=ref<OrderAdjustment|null>(null),channel=ref<'wechat'|'balance'>('wechat'),wallet=ref<Wallet|null>(null),walletLoading=ref(false),walletError=ref('')
  let visible=true,disposed=false,readVersion=0,operation=0,walletVersion=0
  let currentRead:Promise<void>|null=null
  let resumeDraft:{kind:AdjustmentKind;amount:string;reason:string}|null=null
  const pending=computed(()=>records.value.find(record=>record.id===readOrder()?.active_adjustment_id&&adjustmentPending(record))||null)
  const canIncrease=computed(()=>!loading.value&&!error.value&&!pending.value&&canProposeAdjustment(readOrder(),state.user?.id||'','increase'))
  const canDecrease=computed(()=>!loading.value&&!error.value&&!pending.value&&canProposeAdjustment(readOrder(),state.user?.id||'','decrease'))
  /** 捕获参与者账号、令牌和订单，旧页面不能继续执行新资金操作。 */
  function ticket(){return{id:state.user?.id,token:token(),revision:sessionRevision(),order:readOrder()?.id}}
  /** 原生微信支付会短暂隐藏页面，同一账号仍可按原单查询，不可回填到别的订单。 */
  function same(t:ReturnType<typeof ticket>):boolean{return!disposed&&t.id===state.user?.id&&t.token===token()&&t.revision===sessionRevision()&&t.order===readOrder()?.id}
  /** 只有当前页面与当前版本可修改可见记录或发起后续资金操作。 */
  function current(t:ReturnType<typeof ticket>,version:number):boolean{return visible&&same(t)&&version===operation}
  /** 订单下拉与updated_at监听复用同一在途GET；失败保留历史和调价草稿。 */
  function load():Promise<void>{
    const id=readOrder()?.id
    if(!visible||disposed||!id||!state.user||busy.value)return Promise.resolve()
    if(loading.value&&currentRead)return currentRead
    const version=++readVersion,t=ticket();loading.value=true;error.value=''
    currentRead=(async()=>{
      try{const result=await listOrderAdjustments(id);if(visible&&same(t)&&version===readVersion){records.value=result;for(const record of result)if(record.proposer_id===t.id&&record.request_id)clearCreationRequest(record.request_id)}}
      catch(e){if(visible&&same(t)&&version===readVersion)error.value=errorMessage(e)}
      finally{if(version===readVersion){loading.value=false;currentRead=null}}
    })()
    return currentRead
  }
  /** 清空确认和输入，仅当用户主动取消或切页时丢弃未提交草稿。 */
  function close():void{if(busy.value)return;walletVersion++;dialog.value=null;selected.value=null;wallet.value=null;walletLoading.value=false;walletError.value='';formError.value=''}
  /** 打开创建表单，尚无双方确认或资金请求。 */
  function openCreate(next:AdjustmentKind):void{if(!visible||busy.value||(next==='increase'?!canIncrease.value:!canDecrease.value))return;close();kind.value=next;amount.value='';reason.value='';dialog.value='create'}
  /** 仅另一位交易当事人可以接受/拒绝，发起人只能撤回普通未付款方案。 */
  function canRespond(record:OrderAdjustment,action:'accept'|'reject'|'withdraw'):boolean{const order=readOrder(),user=state.user?.id;if(!order||!user||![order.buyer_id,order.provider_id].includes(user))return false;if(action==='withdraw')return record.proposer_id===user&&['proposed','awaiting_payment'].includes(record.status);return record.proposer_id!==user&&record.status==='proposed'}
  /** 只为本订单买家展示已同意加价的支付入口，支付中只允许查询原单。 */
  function canPay(record:OrderAdjustment):boolean{return record.kind==='increase'&&record.status==='awaiting_payment'&&readOrder()?.buyer_id===state.user?.id}
  /** 与资金同步接口保持相同权限，打手和俱乐部仅查看结果，不显示必被拒绝的操作。 */
  function canSynchronize():boolean{return!!state.user&&(state.user.id===readOrder()?.buyer_id||state.user.role==='admin')}
  /** 准备明确的接受/拒绝/撤回确认，不在点击记录时直接改资金。 */
  function prepare(record:OrderAdjustment,action:'accept'|'reject'|'withdraw'):void{if(!visible||busy.value||!canRespond(record,action))return;close();selected.value=record;dialog.value=action}
  /** 补款弹层只读当前余额，旧账号或关闭后的读取不能回填。 */
  async function openPay(record:OrderAdjustment):Promise<void>{if(!visible||busy.value||!canPay(record))return;close();selected.value=record;channel.value='wechat';dialog.value='pay';const t=ticket(),version=++walletVersion;walletLoading.value=true;try{const result=await getWallet();if(visible&&same(t)&&version===walletVersion&&dialog.value==='pay')wallet.value=result}catch(e){if(visible&&same(t)&&version===walletVersion)walletError.value=errorMessage(e)}finally{if(version===walletVersion)walletLoading.value=false}}
  /** 正式付款前保持同一业务键；未知响应重试也沿用原补款，而非另开一笔。 */
  function paymentRequest(id:string):string{const key='jijie_adjustment_payment:'+state.user?.id+':'+id;const saved=uni.getStorageSync(key),value=typeof saved==='string'&&saved?saved:newWalletRequestID();uni.setStorageSync(key,value);return value}
  /** 创建重试按原账号、订单和完整草稿绑定请求号；不保存令牌或支付参数。 */
  function creationRequest(cents:number):string{const key='jijie_adjustment_create:'+state.user?.id+':'+readOrder()?.id,payload=JSON.stringify([kind.value,cents,reason.value.trim()]);const raw=uni.getStorageSync(key);let previous:{payload?:string;id?:string}={};try{if(typeof raw==='string')previous=JSON.parse(raw) as typeof previous}catch{/* 损坏的本地草稿不影响服务端幂等校验。 */}const id=previous.payload===payload&&typeof previous.id==='string'?previous.id:newWalletRequestID();uni.setStorageSync(key,JSON.stringify({payload,id}));return id}
  /** 只有原业务号已被服务器确认存在才结束重试意图，避免下一次相同服务方案重用旧终态。 */
  function clearCreationRequest(id:string):void{const key='jijie_adjustment_create:'+state.user?.id+':'+readOrder()?.id;try{const saved=JSON.parse(String(uni.getStorageSync(key)||'null')) as {id?:string}|null;if(saved?.id===id)uni.removeStorageSync(key)}catch{/* 损坏草稿不允许清除其它新的业务意图。 */}}
  /** 同一有效页面确认后再执行；未知付款/退款只靠原单查询与服务端核验入账。 */
  async function confirm():Promise<void>{
    const action=dialog.value,order=readOrder(),record=selected.value
    if(!visible||disposed||busy.value||!action||!order)return
    const t=ticket(),version=++operation,finishFinancial=beginFinancialActivity();busy.value=true;formError.value='';readVersion++;loading.value=false;currentRead=null
    try{
      if(action==='create'){
        const cents=parseAdjustmentAmount(amount.value),text=reason.value.trim();if(Array.from(text).length<2||Array.from(text).length>500)throw new Error('请填写2—500字原因');if(kind.value==='decrease'&&cents>order.amount_cents)throw new Error('退差金额不能超过当前订单金额');if(!(await requirePhone())||!current(t,version))return
        const requestID=creationRequest(cents);await createOrderAdjustment(order.id,{kind:kind.value,amount_cents:cents,reason:text,request_id:requestID});if(current(t,version))clearCreationRequest(requestID)
      }else if(action==='pay'){
        if(!record||!canPay(record))return;if(channel.value==='balance'&&(!wallet.value||wallet.value.spendable_cents<record.amount_cents))throw new Error('钱包余额不足，请选择微信支付');if(!(await requirePhone())||!current(t,version))return
        if(!(await requireIdentity())||!current(t,version))return
        const result=await payOrderAdjustment(order.id,record.id,{channel:channel.value,...(channel.value==='balance'?{request_id:paymentRequest(record.id)}:{})});if(!current(t,version))return
        if(result.mode==='wechat'&&result.adjustment.status!=='completed'){
          if(!result.pay_params)throw new Error('缺少微信补款参数，请同步原补款单')
          try{await requestWechatPayment(result.pay_params)}catch{/* 用户取消或未知结果均读取原单，不能本地认定扣款成功或换单。 */}
          if(!current(t,version))return
          await syncOrderAdjustment(order.id,record.id)
        }
      }else{
        if(!record||!canRespond(record,action))return
        await respondOrderAdjustment(order.id,record.id,action)
      }
      if(!current(t,version))return;busy.value=false;close();changed();await load()
    }catch(e){if(current(t,version))formError.value=errorMessage(e)}finally{if(version===operation)busy.value=false;finishFinancial()}
  }
  /** 买家及平台只同步既有付款/退款结果，不代替双方同意或另建资金单。 */
  async function synchronize(record:OrderAdjustment):Promise<void>{if(!visible||busy.value||!canSynchronize())return;const t=ticket(),version=++operation,finishFinancial=beginFinancialActivity();busy.value=true;error.value='';try{await syncOrderAdjustment(t.order||'',record.id);if(!current(t,version))return;busy.value=false;changed();await load()}catch(e){if(current(t,version))error.value=errorMessage(e)}finally{if(version===operation)busy.value=false;finishFinancial()}}
  /** 切账号/订单立即清空旧证据、草稿与所有未发出的后续资金操作。 */
  function clear():void{operation++;readVersion++;walletVersion++;resumeDraft=null;records.value=[];loading.value=false;currentRead=null;busy.value=false;error.value='';dialog.value=null;selected.value=null;wallet.value=null;walletLoading.value=false;walletError.value='';formError.value='';amount.value='';reason.value=''}
  /** 原生授权临时隐藏只撤销旧操作和支付意图，同账号创建草稿保留供用户重新打开。 */
  function hide():void{visible=false;const draft=dialog.value==='create'?{kind:kind.value,amount:amount.value,reason:reason.value}:null;clear();resumeDraft=draft}
  watch([()=>readOrder()?.id,()=>state.user?.id,sessionRevision],()=>{clear();if(visible)void load()},{immediate:true,flush:'sync'})
  watch(()=>readOrder()?.updated_at,()=>{if(visible&&!busy.value)void load()})
  onShow(()=>{visible=true;if(resumeDraft){kind.value=resumeDraft.kind;amount.value=resumeDraft.amount;reason.value=resumeDraft.reason;dialog.value='create';resumeDraft=null}void load()});onHide(hide);onUnload(()=>{disposed=true;hide();resumeDraft=null});onBeforeUnmount(()=>{disposed=true;hide();resumeDraft=null})
  return{records,pending,loading,busy,error,formError,dialog,kind,amount,reason,selected,channel,wallet,walletLoading,walletError,canIncrease,canDecrease,load,close,openCreate,canRespond,canPay,canSynchronize,prepare,openPay,confirm,synchronize}
}
