<!-- 文件作用：按先行设计逐笔展示提现来源核验，复核通过后才发起转账，处理中只沿原申请同步。 -->
<template>
  <AppShell title="审核明细" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="付款来源与提现审核仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading&&!audit" class="loading-line">正在核验原申请资金来源…</view>
      <template v-if="audit">
        <view class="audit-summary"><text class="detail-amount">提现 ¥{{ fixed(audit.withdrawal.amount_cents) }}</text><text :class="['detail-verdict',{'verdict-failed':!verified}]">{{ verified?'核验通过':'核验未通过' }} · 来源合计 ¥{{ fixed(audit.total_cents) }}</text><text class="detail-note">{{ audit.withdrawal.club_id?'俱乐部佣金提现 · '+audit.withdrawal.club_id:'个人提现 · '+audit.withdrawal.user_id }}</text><view class="detail-status-row"><text class="detail-note">{{ walletStatus(audit.withdrawal.status) }} · {{ time(audit.withdrawal.created_at) }}</text><button class="text-btn detail-refresh" :disabled="loading||busy" @click="load">重新核验</button></view><text v-if="audit.withdrawal.failure_reason" class="detail-warning">{{ audit.withdrawal.failure_reason }}</text></view>
        <view v-if="audit.issues.length" class="audit-issues"><text class="issue-heading">需要处理的异常</text><text v-for="(issue,index) in audit.issues" :key="index" class="issue-copy">{{ index+1 }}. {{ issue }}</text></view>
        <text class="source-flow">付款 → 订单 → 收入 → 本次提现</text>
        <view v-if="!audit.sources.length" class="source-empty">未找到可核验的资金来源，不可发起转账。</view>
        <view v-for="(source,index) in audit.sources" :key="source.source_id+'-'+index" class="source-row"><view class="source-heading"><text class="source-title">资金来源 {{ String(index+1).padStart(2,'0') }}</text><text class="source-amount">¥{{ fixed(source.amount_cents) }}</text></view><text class="detail-note">{{ channelName(source.channel) }} · 本次分配金额</text><text class="source-reference">来源编号 {{ source.source_id }}</text><text class="source-reference">付款凭证 {{ source.payment_id||'未提供' }}</text><view v-if="source.order_id" class="source-order"><text class="source-reference source-order-id">订单 {{ source.order_id }}</text><button class="text-btn source-order-link" @click="openOrder(source.order_id)">查看订单<Icon name="arrow" :size="12"/></button></view></view>
        <text class="audit-explanation">审核同时检查来源金额、资金归属、售后状态和重复占用。执行转账前，服务端会再次核验。</text>
        <text v-if="actionError" class="detail-warning action-error">{{ actionError }}</text>
        <view v-if="audit.withdrawal.status==='pending_review'" class="detail-actions"><button class="btn btn-secondary detail-reject" :disabled="!fresh||busy" @click="prepare('reject')">拒绝</button><button class="btn detail-approve" :disabled="!canApprove" @click="prepare('approve')">{{ busy?'正在处理…':'审核并发起转账' }}</button></view>
        <view v-else-if="canSynchronize" class="detail-sync"><text class="detail-note">已发起申请只能查询原微信转账单，未知结果保持冻结。</text><button class="btn detail-sync-button" :disabled="!fresh||busy" @click="prepare('sync')">{{ busy?'正在同步…':'同步原转账单' }}</button></view>
        <text v-else class="detail-terminal">当前状态：{{ walletStatus(audit.withdrawal.status) }}，不再发起转账。</text>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
    <UniPopup ref="popup" type="bottom" :animation="false" :is-mask-click="!busy" background-color="transparent" :safe-area="false" @change="popupChanged"><view class="audit-action-sheet"><view class="action-sheet-heading"><text class="action-sheet-title">{{ pendingAction==='reject'?'拒绝提现申请':pendingAction==='sync'?'同步原转账单':'确认审核转账' }}</text><button class="text-btn" :disabled="busy" @click="closeAction">关闭</button></view><text class="action-sheet-amount">¥{{ fixed(audit?.withdrawal.amount_cents||0) }}</text><text class="detail-note">申请编号 {{ id }}</text><textarea v-if="pendingAction==='reject'" v-model="rejectReason" class="input reject-reason" maxlength="500" :disabled="busy" placeholder="请填写2—500字拒绝原因，未发起的资金将由服务端解冻。"/><text v-else class="detail-note action-sheet-note">{{ pendingAction==='sync'?'只同步本申请原微信转账单，未确认结果不会解冻。':'确认后重新核验完整资金来源，通过后按原申请编号发起转账。' }}</text><view v-if="state.devMode&&pendingAction!=='reject'" class="simulation-options"><text class="detail-note">本地模拟资金，不会真实转账</text><radio-group class="simulation-choices" @change="changeSimulation"><label class="simulation-choice"><radio class="simulation-radio" value="success" :checked="devOutcome==='success'" :disabled="busy"/>模拟成功</label><label class="simulation-choice"><radio class="simulation-radio" value="failed" :checked="devOutcome==='failed'" :disabled="busy"/>模拟失败</label></radio-group></view><text v-if="actionError" class="detail-warning">{{ actionError }}</text><view class="action-footer action-sheet-footer"><button class="text-btn" :disabled="busy" @click="closeAction">取消</button><button class="btn action-sheet-confirm" :disabled="busy" @click="confirmAction">{{ busy?'正在处理…':pendingAction==='reject'?'确认拒绝':pendingAction==='sync'?'确认同步':'确认审核转账' }}</button></view></view></UniPopup>
  </AppShell>
</template>
<script setup lang="ts">
import { computed,nextTick,ref,watch } from 'vue'
import { onLoad,onShow,onHide,onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import { state,init,sessionRevision } from '../../../lib/store'
import { token,errorMessage } from '../../../api/client'
import { getWithdrawalAudit } from '../../../api/finance.api'
import { executeAdminWithdrawal,rejectAdminWithdrawal } from '../../../api/wallet.api'
import { go,notify,time } from '../../../lib/format'
import { walletStatus } from '../../../lib/wallet'
import type { WithdrawalAudit } from '../../../types/wallet'

/** 弹层只保存操作类型，申请与来源始终来自服务器审核结果。 */
type AuditAction='approve'|'reject'|'sync'
const id=ref(''),audit=ref<WithdrawalAudit|null>(null),loading=ref(false),busy=ref(false),error=ref(''),actionError=ref(''),pendingAction=ref<AuditAction|null>(null),rejectReason=ref(''),devOutcome=ref<'success'|'failed'>('success')
const popup=ref<{open:(type?:string)=>void;close:()=>void}|null>(null)
let visible=false,version=0,actionVersion=0,popupVersion=0,loadedSession:ReturnType<typeof currentSession>|null=null
/** 捕获当前管理员会话，所有读取与处理均绑定本次身份。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 隐藏、登出和更换管理员后，旧响应与旧弹层均不再有效。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 核验通过必须由后端明确返回，展示合计同时与原申请金额一致。 */
const verified=computed(()=>!!audit.value&&audit.value.passed&&audit.value.issues.length===0&&audit.value.total_cents===audit.value.withdrawal.amount_cents)
/** 读取错误或页面不可见时，不允许使用旧核验结果触发资金操作。 */
const fresh=computed(()=>!!audit.value&&!loading.value&&!error.value&&sameSession(loadedSession))
/** 仅当前待审核且后端核验通过的申请可提交审核。 */
const canApprove=computed(()=>fresh.value&&verified.value&&!busy.value&&audit.value?.withdrawal.status==='pending_review')
/** 已出网的申请只提供同步原单，不提供拒绝或新建提现。 */
const canSynchronize=computed(()=>audit.value?.withdrawal.status==='processing'||audit.value?.withdrawal.status==='wait_user_confirm')
/** 金额固定显示到分位。 */
function fixed(cents:number):string{return(cents/100).toFixed(2)}
/** 用可辨识中文说明来源渠道，不把模拟付款标成真实微信付款。 */
function channelName(value:string):string{return({wechat:'微信支付',dev:'本地模拟支付',balance:'余额上游付款',recharge:'充值付款',income:'订单收入'} as Record<string,string>)[value]||value||'未提供支付渠道'}
/** 对原申请重新读取完整来源，失败时保留明细但禁用转账按钮。 */
async function load():Promise<void>{if(!visible||busy.value)return;const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==version)return;if(state.user?.role!=='admin')return;if(!id.value)throw new Error('缺少提现申请编号');session=currentSession();const result=await getWithdrawalAudit(id.value);if(current!==version||!sameSession(session))return;audit.value=result;loadedSession=session}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 查看来源对应的订单，保留当前审核页供返回后复核。 */
function openOrder(orderID:string):void{if(fresh.value)go('/pages/orders/detail?id='+encodeURIComponent(orderID))}
/** 打开官方确认弹层并固定当前有效操作，延迟挂载不得在页面隐藏后重新弹出。 */
function prepare(kind:AuditAction):void{if(!fresh.value||busy.value||(kind==='approve'&&!canApprove.value)||(kind==='reject'&&audit.value?.withdrawal.status!=='pending_review')||(kind==='sync'&&!canSynchronize.value))return;pendingAction.value=kind;rejectReason.value='';actionError.value='';devOutcome.value='success';const current=++popupVersion;void nextTick(()=>{if(visible&&current===popupVersion&&pendingAction.value===kind)popup.value?.open('bottom')})}
/** 处理中禁止主动关闭；页面生命周期关闭则会作废所有回调。 */
function closeAction():void{if(busy.value)return;popupVersion++;popup.value?.close();pendingAction.value=null;rejectReason.value=''}
/** 官方遮罩关闭后同步清空未提交草稿。 */
function popupChanged(event:{show:boolean}):void{if(!event.show&&!busy.value){pendingAction.value=null;rejectReason.value='';popupVersion++}}
/** 开发模式只接收已知模拟终态，生产请求不带该字段。 */
function changeSimulation(event:{detail:{value:string}}):void{if(event.detail.value==='success'||event.detail.value==='failed')devOutcome.value=event.detail.value}
/** 审核发起前再取新核验结果，处理中仅调用原申请接口；所有最终资格由后端再次判断。 */
async function confirmAction():Promise<void>{
  const kind=pendingAction.value
  if(!kind||!fresh.value||busy.value||(kind==='approve'&&!canApprove.value))return
  const reason=rejectReason.value.trim()
  if(kind==='reject'&&(Array.from(reason).length<2||Array.from(reason).length>500)){actionError.value='请填写2—500字拒绝原因';return}
  const current=++actionVersion,session=currentSession(),withdrawalID=id.value
  busy.value=true;actionError.value=''
  try{
    if(kind==='approve'){const latest=await getWithdrawalAudit(withdrawalID);if(current!==actionVersion||!sameSession(session))return;audit.value=latest;loadedSession=session;if(latest.withdrawal.status!=='pending_review')throw new Error('申请状态已变化，请关闭弹层后查看最新进度');if(!latest.passed||latest.issues.length||latest.total_cents!==latest.withdrawal.amount_cents)throw new Error(latest.issues.join('；')||'资金来源核验未通过，不能发起转账')}
    if(kind==='sync'&&!canSynchronize.value)throw new Error('申请状态已变化，请重新核验')
    const result=kind==='reject'?await rejectAdminWithdrawal(withdrawalID,reason):await executeAdminWithdrawal(withdrawalID,state.devMode?{dev_outcome:devOutcome.value}:{})
    if(current!==actionVersion||!sameSession(session))return
    busy.value=false;closeAction();notify('处理结果：'+walletStatus(result.status));await load()
  }catch(err){if(current===actionVersion&&sameSession(session))actionError.value=errorMessage(err)}finally{if(current===actionVersion)busy.value=false}
}
/** 账号变化彻底清空来源与弹层，不让新身份看到旧管理员资金资料。 */
function clear():void{version++;actionVersion++;popupVersion++;audit.value=null;loadedSession=null;loading.value=false;busy.value=false;error.value='';actionError.value='';pendingAction.value=null;rejectReason.value='';popup.value?.close()}
/** 返回页面始终重新核验，订单或售后页面可能改变资金状态。 */
function showPage():void{visible=true;void load()}
/** 隐藏后不再执行尚未发出的后续转账，已出网结果须下次按原单读取。 */
function hidePage():void{visible=false;version++;actionVersion++;popupVersion++;loading.value=false;busy.value=false;pendingAction.value=null;rejectReason.value='';popup.value?.close()}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
onLoad((options:Record<string,string|undefined>={})=>{id.value=options.id||''})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{hidePage();clear()})
</script>
<style scoped>
.audit-summary{display:flex;flex-direction:column;gap:23.0769rpx;padding:15.3846rpx 7.6923rpx 46.1538rpx;min-width:0}.detail-amount{font-size:55.7692rpx;line-height:76.9231rpx;font-weight:550;overflow-wrap:anywhere}.detail-verdict{font-size:26.9231rpx;line-height:42.3077rpx;color:var(--accent)}.verdict-failed,.detail-warning{color:#e9b69d}.detail-note,.source-reference{display:block;min-width:0;font-size:26.9231rpx;line-height:42.3077rpx;color:var(--muted);overflow-wrap:anywhere}.detail-status-row{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.detail-refresh{flex-shrink:0;font-size:25rpx}.detail-warning{display:block;font-size:26.9231rpx;line-height:46.1538rpx;white-space:pre-wrap;overflow-wrap:anywhere}.audit-issues{display:flex;flex-direction:column;gap:19.2308rpx;padding:30.7692rpx 23.0769rpx;margin-bottom:46.1538rpx;border:1px solid #e9b69d44;border-radius:15.3846rpx;background:#e9b69d0a}.issue-heading{font-size:28.8462rpx;line-height:42.3077rpx;font-weight:650;color:#e9b69d}.issue-copy{font-size:26.9231rpx;line-height:46.1538rpx;color:#e9b69d;white-space:pre-wrap;overflow-wrap:anywhere}.source-flow{display:block;margin:30.7692rpx 7.6923rpx 15.3846rpx;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted)}.source-empty{padding:46.1538rpx 7.6923rpx;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted)}.source-row{display:flex;flex-direction:column;gap:15.3846rpx;min-width:0;padding:30.7692rpx 7.6923rpx;border-bottom:1px solid var(--line)}.source-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:23.0769rpx;min-width:0}.source-title{min-width:0;font-size:30.7692rpx;line-height:46.1538rpx}.source-amount{flex-shrink:0;max-width:50%;overflow-wrap:anywhere;text-align:right;font-size:32.6923rpx;line-height:46.1538rpx;color:var(--accent)}.source-reference{font-size:23.0769rpx;line-height:38.4615rpx}.source-order{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0}.source-order-id{flex:1;min-width:0}.source-order-link{flex-shrink:0;align-self:center;font-size:23.0769rpx}.audit-explanation{display:block;margin:61.5385rpx 7.6923rpx 0;color:var(--muted);font-size:26.9231rpx;line-height:46.1538rpx}.detail-actions{display:flex;gap:26.9231rpx;margin-top:76.9231rpx;min-width:0}.detail-reject{flex:0 0 30%;min-width:0}.detail-approve{flex:1;min-width:0;font-size:28.8462rpx}.detail-sync{display:flex;flex-direction:column;gap:38.4615rpx;margin-top:61.5385rpx}.detail-sync-button{width:100%}.detail-terminal{display:block;margin:61.5385rpx 7.6923rpx;color:var(--muted);font-size:26.9231rpx;line-height:46.1538rpx}.action-error{margin-top:38.4615rpx}.audit-action-sheet{width:100%;max-width:600px;max-height:85vh;overflow:auto;box-sizing:border-box;padding:46.1538rpx 30.7692rpx calc(46.1538rpx + env(safe-area-inset-bottom));border-radius:30.7692rpx 30.7692rpx 0 0;background:var(--surface);color:var(--text)}.action-sheet-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.action-sheet-title{font-size:36.5385rpx;line-height:53.8462rpx;font-weight:650}.action-sheet-amount{display:block;margin:30.7692rpx 0 23.0769rpx;font-size:57.6923rpx;line-height:76.9231rpx;color:var(--accent);overflow-wrap:anywhere}.reject-reason{height:253.8462rpx;margin:46.1538rpx 0 30.7692rpx;font-size:26.9231rpx;line-height:46.1538rpx}.action-sheet-note{margin:38.4615rpx 0}.simulation-options{display:flex;flex-direction:column;gap:23.0769rpx;margin:38.4615rpx 0}.simulation-choices{display:flex;flex-wrap:wrap;gap:30.7692rpx;font-size:26.9231rpx}.simulation-choice{display:flex;align-items:center;gap:7.6923rpx}.simulation-radio{transform:scale(.8);margin-right:-7.6923rpx}.action-sheet-footer{display:flex;align-items:center;justify-content:flex-end;gap:46.1538rpx;margin-top:46.1538rpx}.action-sheet-confirm{min-width:261.5385rpx}
</style>
