<!-- 文件作用：按交易阶段展示订单、服务约定和费用协商，独立聊天从底部入口进入，原资金及售后操作保持。 -->
<template>
  <AppShell title="订单详情" :game-id="order?.game_id" back>
    <view class="detail-content">
      <view v-if="error" class="error-panel" @click="load()">{{ error }} · 点击重试</view>
      <view v-if="!order&&!error" class="muted loading">正在加载订单…</view>
      <template v-if="order">
        <view class="order-heading"><text class="status-title">{{ statusLabel(order.status) }}</text><view class="order-number-row"><text class="order-number">订单编号 {{ order.order_no }}</text><button class="copy-number" aria-label="复制订单编号" @click="copyOrderNumber"><Icon name="copy" :size="13"/></button></view><text class="stage-description">{{ stageDescription }}</text></view>
        <view v-if="!['cancelled','refunded'].includes(order.status)" class="order-progress"><view v-for="(step,index) in progressSteps" :key="step.label" :class="['progress-step',{'progress-done':step.done,'progress-active':step.active}]"><view class="progress-icon"><Icon :name="step.done?'check-circle':'circle'" :size="18"/></view><view v-if="index<2" :class="['progress-line',{'progress-line-done':step.done}]"/><text class="progress-label">{{ step.label }}</text><text v-if="step.detail" class="progress-detail">{{ step.detail }}</text></view></view>
        <view class="summary-card"><image v-if="orderCover" class="summary-cover" :src="assetUrl(orderCover)" mode="aspectFill"/><view class="summary-copy"><view class="summary-row"><text class="order-title">{{ order.title }}</text><text class="price">¥{{ money(order.amount_cents) }}</text></view><text class="summary-game">{{ order.game_name }}</text><text class="summary-unit">{{ order.quantity }} {{ order.unit||'单' }}<text v-if="order.unit_price_cents"> · ¥{{ money(order.unit_price_cents) }}/{{ order.unit||'单' }}</text></text></view></view>
        <view class="people-card"><view class="person"><Icon name="user" :size="19"/><view class="person-copy"><text class="person-role">买家</text><text class="person-name">{{ order.buyer_name }}</text></view></view><view class="person"><Icon name="work" :size="19"/><view class="person-copy"><text class="person-role">服务方</text><text class="person-name">{{ order.provider_name||'等待接单' }}</text></view></view></view>
        <view class="detail-section"><button class="section-toggle" @click="agreementOpen=!agreementOpen"><Icon name="receipt" :size="18"/><text class="detail-section-title">服务约定</text><Icon :name="agreementOpen?'up':'arrow'" :size="14"/></button><view v-show="agreementOpen" class="section-body"><view v-if="order.service_description" class="agreement"><text class="agreement-title">下单时的服务说明</text><text class="agreement-body">{{ order.service_description }}</text></view><view class="requirements"><text class="agreement-title">需求与验收标准</text><text class="agreement-body">{{ order.requirements||'无额外要求，以服务详情约定为准。' }}</text></view><view v-if="order.delivery_note" class="delivery"><text class="agreement-title">交付说明</text><text class="agreement-body">{{ order.delivery_note }}</text></view></view></view>
        <view class="detail-section"><button class="section-toggle" :disabled="adjustmentProcessing" @click="feesOpen=!feesOpen"><Icon name="wallet" :size="18"/><text class="detail-section-title">费用与调价</text><Icon :name="feesOpen?'up':'arrow'" :size="14"/></button><view v-show="feesOpen" class="section-body fee-body"><OrderSettlement v-if="isProvider||state.user?.role==='admin'||(!isBuyer&&!!order.club_id)" :order="order"/><view v-else class="paid-row"><text>订单金额</text><text>¥{{ money(order.amount_cents) }}</text></view><OrderAdjustments ref="adjustmentsView" :order="order" @changed="load()" @processing="adjustmentProcessing=$event"/></view></view>
        <view class="detail-section"><button class="section-toggle" @click="afterSaleOpen=!afterSaleOpen"><Icon name="shield-check" :size="18"/><view class="after-sale-heading"><text class="detail-section-title">售后与协商</text><text class="after-sale-hint">{{ refundActive?refundTitle:'如有问题，可在订单内协商处理' }}</text></view><Icon :name="afterSaleOpen?'up':'arrow'" :size="14"/></button><view v-if="refundActive&&afterSaleOpen" class="refund-panel"><text>申请理由：{{ order.refund_reason }}</text><text v-if="order.refund_reply">处理说明：{{ order.refund_reply }}</text><text v-if="['requested','escalated','processing'].includes(order.refund_status)" class="muted">协商期间暂停验收与结算，订单证据完整保留。</text><text v-if="(order.payment_channel==='virtual'||order.has_virtual_payment)&&order.refund_status==='processing'" class="muted">{{isBuyer?'退款处理中，可使用下方入口刷新微信身份并同步退款。':'虚拟支付退款可能需要买家使用原小程序账号刷新微信身份后同步；平台不能代刷买家身份。'}}</text></view><view v-if="secondaryActions.length" class="actions"><button v-for="item in secondaryActions" :key="item.id" class="text-btn order-secondary" :disabled="busy||adjustmentProcessing" @click="prepare(item)">{{ item.label }}</button></view></view>
        <view v-if="state.user?.role==='admin'&&order.refund_status==='escalated'" class="admin-resolution"><text class="section-title">平台裁决</text><text class="muted">请先进入订单聊天，审阅交付及图片证据，再填写裁决依据。</text><button class="btn" @click="prepare({id:'resolve_refund',label:'裁决退款',note:true,tip:'请说明事实依据与退款原因'})">支持买方 · 原路退款</button><button class="btn-secondary btn-small" @click="prepare({id:'resolve_resume',label:'驳回退款',note:true,tip:'请说明驳回原因及后续履约要求'})">驳回申请 · 继续履约</button><button v-if="order.status==='delivered'" class="btn-secondary btn-small" @click="prepare({id:'resolve_complete',label:'裁决完成',note:true,tip:'确认服务已按约定交付，并说明证据依据'})">支持卖方 · 确认完成</button></view>
        <view v-if="canReview" class="form-section review-panel"><text class="section-title">这次并肩体验如何？</text><view class="stars"><button v-for="star in 5" :key="star" class="rating-button" @click="rating=star"><Icon name="star" :size="26" :class="{lit:star<=rating}"/></button></view><textarea v-model="reviewText" class="input review-input" maxlength="500" placeholder="留下真实反馈，帮助下一位玩家找到好队友"/><text class="hint">{{ Array.from(reviewText).length }}/500</text><button class="btn-secondary btn-small review-button" :disabled="busy" @click="submitReview">发布评价</button></view>
        <view v-if="reviews.length" class="review-result"><text>买家评价 · {{ reviews[0].rating }} / 5</text><text class="muted">{{ reviews[0].content||'用户未填写文字评价' }}</text></view>
        <SubscribeReminder v-if="isBuyer||isProvider" ref="subscriptionReminder" :scenes="isBuyer?['buyer_accepted']:['provider_order','provider_refund']" label="接收订单微信提醒"/>
        <text class="updated">订单更新于 {{ time(order.updated_at) }} · {{ state.devMode?'本地测试交易，不产生真实扣费':'所有金额以订单约定为准' }}</text>
      </template>
    </view>
    <view v-if="order" class="order-dock"><button class="chat-entry" :disabled="busy||adjustmentProcessing" @click="openChat"><Icon name="chat" :size="21"/><text>订单聊天</text></button><button v-if="primaryAction" class="btn dock-primary" :disabled="busy||adjustmentProcessing" @click="prepare(primaryAction)">{{ primaryAction.label }}</button></view>
    <view v-if="pendingAction" class="modal-mask" @click="pendingAction = null"><view class="modal" @click.stop><text class="modal-title">{{ pendingAction.label }}</text><text class="muted">{{ pendingAction.tip || '确认后，系统会在聊天室记录这次操作。' }}</text><textarea v-if="pendingAction.note" v-model="actionNote" class="input modal-input" maxlength="1000" placeholder="请详细说明，作为双方协商与平台判断的依据"/><view v-if="pendingAction.id === 'pay'" class="payment-summary"><text>{{virtualMode?'服务消费':'应付金额'}}</text><text>{{virtualMode?money(order?.amount_cents)+' 金币':'¥'+money(order?.amount_cents)}}</text></view><view v-if="pendingAction.id === 'pay'&&!virtualMode" class="payment-options"><button :class="['payment-option',{chosen:paymentChannel==='wechat'}]" :disabled="busy" @click="paymentChannel='wechat'"><Icon name="weixin" :size="21"/><text class="payment-option-copy">微信支付</text><Icon :name="paymentChannel==='wechat'?'checkbox-filled':'circle'" :size="20"/></button><button :class="['payment-option',{chosen:paymentChannel==='balance'}]" :disabled="busy||!paymentWallet||paymentWallet.spendable_cents<(order?.amount_cents||0)" @click="paymentChannel='balance'"><Icon name="wallet" :size="21"/><view class="payment-option-copy"><text>钱包余额</text><text class="payment-hint">{{ paymentWallet?'可用余额 ¥'+money(paymentWallet.spendable_cents):'正在读取余额…' }}{{ paymentWallet&&paymentWallet.spendable_cents<(order?.amount_cents||0)?' · 余额不足':'' }}</text></view><Icon :name="paymentChannel==='balance'?'checkbox-filled':'circle'" :size="20"/></button><text class="payment-hint">充值余额仅可消费，不能提现。</text><text v-if="walletError" class="payment-hint danger">{{ walletError }}</text></view><VirtualPaymentSummary v-if="pendingAction.id==='pay'&&virtualMode" :quote="paymentQuote" :loading="quoteLoading" :error="quoteError"/><text v-if="pendingAction.id === 'pay' && state.devMode" class="dev-warning">当前为本地测试支付，不会真实扣款。</text><view class="action-footer modal-actions"><button class="text-btn" :disabled="busy" @click="pendingAction = null">再想想</button><button class="btn" :disabled="busy || (pendingAction.note && !actionNote.trim()) || (pendingAction.id==='pay'&&virtualMode&&(quoteLoading||!paymentQuote||!!quoteError))" @click="execute">{{ busy ? '处理中…' : '确认' + pendingAction.label }}</button></view></view></view>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：驱动订单状态操作、服务评价和退款介入，所有权与状态最终由后端校验。 */
import { ref, computed, nextTick, watch } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import { useNativePageRefresh } from '../../lib/native-page-refresh'
import SubscribeReminder from '../../components/SubscribeReminder.vue'
import Icon from '../../components/Icon.vue'
import OrderSettlement from '../../components/OrderSettlement.vue'
import OrderAdjustments from '../../components/OrderAdjustments.vue'
import VirtualPaymentSummary from '../../components/VirtualPaymentSummary.vue'
import { getVirtualQuote } from '../../api/commerce.api'
import type { VirtualQuote } from '../../types/commerce'
import { getOrder, listOrderReviews, payOrder, syncOrderPayment, performOrderAction, submitOrderReview } from '../../api/orders.api'
import { retryOrderRefund, resolveOrder } from '../../api/admin.api'
import type { Order, Review } from '../../types/models'
import { token, errorMessage, errorStatus } from '../../api/client'
import { getWallet } from '../../api/wallet.api'
import type { Wallet } from '../../types/wallet'
import { requestWechatPayment,prepareVirtualPayment,paymentPlatform } from '../../lib/payment'
import { beginFinancialActivity } from '../../lib/update-manager'
import { newWalletRequestID } from '../../lib/wallet'
import { requirePhone } from '../../lib/phone'
import { requireIdentity } from '../../lib/identity'
import { state, init, sessionRevision } from '../../lib/store'
import { money, time, statusLabel, notify, assetUrl, go } from '../../lib/format'
/** 订单状态操作及确认表单配置。 */
interface OrderAction { id: string; label: string; primary?: boolean; note?: boolean; tip?: string }
const adjustmentsView=ref<{refresh:()=>Promise<void>}|null>(null)
const subscriptionReminder=ref<{offer:(next?:()=>void)=>void}|null>(null)
const id = ref(''), order = ref<Order | null>(null), reviews = ref<Review[]>([]), error = ref(''), busy = ref(false), pendingAction = ref<OrderAction | null>(null), actionNote = ref(''), rating = ref(5), reviewText = ref('')
const adjustmentProcessing=ref(false)
const agreementOpen=ref(true),feesOpen=ref(true),afterSaleOpen=ref(true)
useNativePageRefresh({refresh:refreshOrder,blocked:()=>busy.value||adjustmentProcessing.value||!!pendingAction.value,context:()=>[state.user?.id,state.user?.role,sessionRevision(),id.value]})
const paymentChannel=ref<'wechat'|'balance'>('wechat'),paymentWallet=ref<Wallet|null>(null),walletError=ref('')
const paymentQuote=ref<VirtualQuote|null>(null),quoteLoading=ref(false),quoteError=ref('')
const virtualMode=computed(()=>!state.devMode&&(order.value?.payment_channel?order.value.payment_channel==='virtual':state.commerce?.mode==='mini_program'))
let disposed=false,paymentPreparation=0
let timer: ReturnType<typeof setInterval> | undefined, polling = false, visible = false, viewerID = '', pageVersion = 0
/** 捕获一次订单操作的账号和会话，防止迟到响应进入其他账号。 */
function currentSession() { return { id:state.user?.id, role:state.user?.role, adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(), revision:sessionRevision() } }
/** 微信支付和原生选图可以临时隐藏页面，但只能由原账号继续请求。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return !disposed&&session.id===state.user?.id&&session.role===state.user?.role&&session.adminRole===state.user?.platform_admin_role&&session.status===state.user?.status&&session.token===token()&&session.revision===sessionRevision() }
/** 页面显示且版本、会话都一致时才回填私有订单和证据。 */
function currentRequest(version: number, session: ReturnType<typeof currentSession>): boolean { return visible&&version===pageVersion&&sameSession(session) }
/** 切换身份立即移除上一位参与者的私有内容和未发送草稿。 */
function clearPrivateOrder(): void {adjustmentProcessing.value=false;order.value=null;reviews.value=[];pendingAction.value=null;actionNote.value='';reviewText.value='';paymentWallet.value=null;paymentQuote.value=null;quoteError.value='';quoteLoading.value=false;walletError.value='';pageVersion++;polling=false}
watch(()=>[state.user?.id,state.user?.role,state.user?.platform_admin_role,state.user?.status,sessionRevision()].join('\n'),()=>{clearPrivateOrder();if(visible)load()})
const isBuyer = computed(() => order.value?.buyer_id === state.user?.id)
const isProvider = computed(() => order.value?.provider_id === state.user?.id)
const refundActive = computed(() => order.value?.refund_status && order.value.refund_status !== 'none')
const refundTitle = computed(() => (({ requested:'等待打手处理退款', rejected:'退款申请被拒绝，可补充证据并申请平台介入', escalated:'平台正在处理争议', approved:'退款已处理', processing:'微信正在处理退款，请稍后同步资金状态' } as Record<string,string>)[order.value?.refund_status || ''] || '售后记录'))
const stageDescription = computed(() => (({ pending_payment:'支付后会通知队友，请确认服务要求和金额。', open:'需求已发布，等待合适的队友接单。', accepted:'队友已接单，可以在聊天室确认集合时间。', in_progress:'服务正在进行，请及时同步进度与约定。', delivered:'队友已提交交付说明，请核对后确认完成。', completed:'本次服务已完成，感谢选择开机电竞。', cancelled:'订单已取消，双方沟通记录仍然保留。', refunded:'退款已完成，本次交易关闭。', disputed:'平台正在核对双方证据，请在聊天室补充说明。' } as Record<string,string>)[order.value?.status || ''] || ''))
const actions = computed(() => {
  const result: OrderAction[] = [], current = order.value
  if (!current) return result
  const status = current.status, refund = current.refund_status || 'none', suspended = ['requested','escalated','processing'].includes(refund)
  if (!state.devMode && (isBuyer.value || state.user?.role === 'admin') && (status === 'pending_payment' || refund === 'processing')) result.push({ id:'sync_payment', label:(current.payment_channel==='virtual'||current.has_virtual_payment)&&isBuyer.value?(refund==='processing'?'刷新微信身份并同步退款':'刷新微信身份并同步资金'):'同步资金状态' })
  if (!state.devMode && state.user?.role === 'admin' && refund === 'processing') result.push({ id:'retry_refund', label:'重试退款通道', tip:'沿用原退款单号重试未知结果，不会发起第二笔退款。' })
  if (isBuyer.value && status === 'pending_payment') result.push({ id:'pay', label:'支付订单', primary:true }, { id:'cancel', label:'取消订单' })
  if (isBuyer.value && status === 'open' && !suspended) result.push({ id:'cancel', label:'取消订单' })
  if (isProvider.value && status === 'open' && !suspended) result.push({ id:'accept', label:'接受订单', primary:true })
  if (isProvider.value && status === 'accepted' && !suspended) result.push({ id:'start', label:'开始服务', primary:true })
  if (isProvider.value && status === 'in_progress' && !suspended) result.push({ id:'deliver', label:'提交交付', primary:true, note:true, tip:'说明已经完成的内容，必要时在聊天室上传战绩或任务截图。' })
  if (isBuyer.value && status === 'delivered' && !suspended) result.push({ id:'confirm', label:'验收并完成', primary:true, tip:'确认服务已按约定完成。确认后进入7天售后待结算期，无退款争议后转为可提现收入。' })
  if (isBuyer.value && ['open','accepted','in_progress','delivered','completed'].includes(status) && !suspended && refund !== 'approved' && !current.refund_final) result.push({ id:'request_refund', label:refund === 'rejected' ? '补充退款申请' : '申请退款', note:true, tip:'请写清退款原因。打手可同意或拒绝；有争议可申请平台介入。' })
  if (isBuyer.value && !current.refund_final && ['requested','rejected'].includes(refund)) result.push({ id:'escalate', label:'申请平台介入', note:true, tip:'请说明争议要点，平台将结合订单要求、聊天和图片证据处理。' })
  if (isBuyer.value && !current.refund_final && ['requested','rejected'].includes(refund)) result.push({ id:'withdraw_refund', label:'撤销退款申请' })
  if (isProvider.value && refund === 'requested') result.push({ id:'approve_refund', label:'同意退款', primary:true }, { id:'reject_refund', label:'拒绝退款', note:true, tip:'请详细说明拒绝理由，并在聊天室补充履约证据。' })
  return result
})
/** 底部只保留一个当前主动作，其它合法动作仍可在售后分节操作。 */
const primaryAction=computed(()=>actions.value.find(action=>action.primary)||null)
const secondaryActions=computed(()=>actions.value.filter(action=>action!==primaryAction.value))
const orderCover=computed(()=>state.games?.find(game=>game.id===order.value?.game_id)?.cover_url||'')
const canReview=computed(()=>isBuyer.value&&order.value?.status==='completed'&&!reviews.value.length&&!['requested','escalated','processing'].includes(order.value.refund_status||'none'))
/** 三阶段只使用真实付款、交付和完成状态，缺少时间时不补造日期。 */
const progressSteps=computed(()=>{const current=order.value,paid=current?.payment_status==='paid',delivered=['delivered','completed'].includes(current?.status||''),completed=current?.status==='completed';return[{label:paid?'已付款':'待付款',done:paid,active:!paid,detail:current?.paid_at?time(current.paid_at):''},{label:current?.status==='open'?'待接单':'服务中',done:delivered,active:paid&&!delivered,detail:''},{label:completed?'已验收':'待验收',done:completed,active:delivered&&!completed,detail:current?.completed_at?time(current.completed_at):''}]})
/** 独立聊天室保留原订单编号，不在详情正文挂载聊天滚动容器。 */
function openChat():void{if(order.value)go('/pages/orders/chat?id='+encodeURIComponent(order.value.id))}
/** 复制真实订单编号，便于客服核对，不复制任何支付秘密。 */
function copyOrderNumber():void{if(order.value?.order_no)uni.setClipboardData({data:order.value.order_no})}
/** 只读取当前订单和评价，聊天由独立页面加载，减少详情轮询和布局负担。 */
async function load(silent=false) {
  if(!id.value||polling||!visible)return
  const version=pageVersion;let session:ReturnType<typeof currentSession>|undefined;polling=true
  try{await init();if(!visible||version!==pageVersion)return;session=currentSession();if(viewerID!==(state.user?.id||'')){viewerID=state.user?.id||'';order.value=null;reviews.value=[]}
    const data=await Promise.all([getOrder(id.value),listOrderReviews(id.value)]);if(!currentRequest(version,session))return;order.value=data[0];reviews.value=data[1];error.value=''
  }catch(err){if(!visible||version!==pageVersion||session&&!sameSession(session))return;if([401,403,404].includes(errorStatus(err)||0)){clearPrivateOrder();clearInterval(timer);error.value=errorMessage(err)}else if(!silent||!order.value)error.value=errorMessage(err)}finally{if(version===pageVersion)polling=false}
}
/** 下拉同时只读刷新调价记录，原资金同步按钮仍须用户明确操作。 */
async function refreshOrder():Promise<void>{const version=pageVersion,session=currentSession();await load();await nextTick();if(currentRequest(version,session))await adjustmentsView.value?.refresh()}
/** 准备可审阅的确认表单，不立即触发金额或状态变更。 */
async function prepare(action: OrderAction) {
  if(busy.value)return
  const prepared=++paymentPreparation
  pendingAction.value=action;actionNote.value=''
  if(action.id!=='pay')return
  paymentChannel.value='wechat';paymentWallet.value=null;walletError.value=''
  const session=currentSession(),version=pageVersion
  paymentQuote.value=null;quoteError.value='';quoteLoading.value=virtualMode.value
  if(virtualMode.value){try{const quote=await getVirtualQuote(order.value?.amount_cents||0,paymentPlatform(),{order_id:id.value});if(currentRequest(version,session)&&prepared===paymentPreparation&&pendingAction.value?.id==='pay')paymentQuote.value=quote}catch(err){if(currentRequest(version,session)&&prepared===paymentPreparation)quoteError.value=errorMessage(err)}finally{if(prepared===paymentPreparation)quoteLoading.value=false};return}
  try{const result=await getWallet();if(currentRequest(version,session)&&prepared===paymentPreparation&&pendingAction.value?.id==='pay')paymentWallet.value=result}
  catch(err){if(currentRequest(version,session)&&prepared===paymentPreparation&&pendingAction.value?.id==='pay')walletError.value=errorMessage(err)}
}
/** 余额付款原请求号跨未知响应重试保持一致，不保存资金或微信凭据。 */
function balanceRequestID():string {const key='jijie_order_balance_request:'+state.user?.id+':'+id.value;const value=String(uni.getStorageSync(key)||newWalletRequestID());uni.setStorageSync(key,value);return value}
/** 调用统一付款工具后查询服务端实付状态；本地回调不代表付款成功。 */
async function payCurrentOrder(session: ReturnType<typeof currentSession>):Promise<boolean>{
  if(!(await requirePhone())||!sameSession(session)||!visible)return false
  if(!(await requireIdentity())||!sameSession(session)||!visible)return false
  if(paymentChannel.value==='balance'&&(!paymentWallet.value||paymentWallet.value.spendable_cents<(order.value?.amount_cents||0)))throw new Error('钱包余额不足，请充值或选择微信支付')
  if(virtualMode.value){if(!paymentQuote.value||quoteLoading.value||quoteError.value)throw new Error('请重新打开付款确认，核对金币与手续费');await prepareVirtualPayment(true);if(!sameSession(session)||!visible)return false}
  const result=await payOrder(id.value,{channel:virtualMode.value?'wechat':paymentChannel.value,...(virtualMode.value?{platform:paymentPlatform(),quoted_total_cents:paymentQuote.value!.total_cents,quoted_topup_cents:paymentQuote.value!.topup_cents}:paymentChannel.value==='balance'?{request_id:balanceRequestID()}:{})})
  if(!sameSession(session))return false
  let paid=result.order
  if((result.mode==='wechat'||result.mode==='virtual')&&paid.payment_status!=='paid'){
    if(!visible)return false
    const params=result.mode==='virtual'?result.virtual_params:result.pay_params
    if(!params&&result.mode==='wechat')throw new Error('支付参数不完整，请同步原订单资金状态')
    try{if(params)await requestWechatPayment(params)}catch{/* 用户取消或微信未知响应均需查询原单，不假设已扣款。 */}
    if(!sameSession(session))return false
    paid=await syncOrderPayment(id.value)
  }
  if(!sameSession(session))return false
  if(paid.payment_status!=='paid'){if(visible)notify('支付尚未确认，请同步原订单资金状态');return false}
  return true
}
/** 付款、履约与退款操作使用服务端状态机，提交中禁用重复点击。 */
async function execute() {
  if (busy.value || !pendingAction.value) return
  const session=currentSession(),finishFinancial=beginFinancialActivity()
  busy.value = true
  try {
    const action = pendingAction.value.id
    if(action!=='pay'&&(order.value?.payment_channel==='virtual'||order.value?.has_virtual_payment)&&order.value?.buyer_id===session.id&&['sync_payment','request_refund','cancel'].includes(action)){await prepareVirtualPayment(true);if(!sameSession(session)||!visible)return}
    if (action === 'pay') { if(!(await payCurrentOrder(session)))return }
    else if (action === 'retry_refund') await retryOrderRefund(id.value)
    else if (action === 'sync_payment') await syncOrderPayment(id.value)
    else if (action.startsWith('resolve_')) await resolveOrder(id.value,action.slice(8),actionNote.value.trim())
    else await performOrderAction(id.value,action,actionNote.value.trim())
    if(!sameSession(session))return;pendingAction.value = null; if(visible){notify('操作成功，订单记录已更新');await load();if(action==='pay'&&sameSession(session))subscriptionReminder.value?.offer()}
  } catch (e) { if(sameSession(session)&&visible)notify(e) } finally { busy.value = false;finishFinancial() }
}
/** 完成后发表一次评价，重复评价由服务端拦截。 */
async function submitReview() { if (busy.value) return; if(!canReview.value)return notify('仅已完成且无进行中售后的订单买家可以评价');const count=Array.from(reviewText.value.trim()).length;if(count<2||count>500)return notify('评价需为2至500字');const session=currentSession(); busy.value = true; try { await submitOrderReview(id.value,{rating:rating.value,content:reviewText.value.trim()});if(sameSession(session)&&visible){await load();notify('评价已保存')} } catch (e) { if(sameSession(session)&&visible)notify(e) } finally { busy.value = false } }
/** 停止页面离开后的轮询，避免后台读取与泄露账号切换后的数据。 */
function stop() { visible = false;pageVersion++;polling=false;clearInterval(timer) }
onLoad(options => { id.value = options?.id || '';if(!id.value)error.value='订单不存在或缺少编号' })
onShow(() => { visible = true; load(); clearInterval(timer); timer = setInterval(() => { if (visible) load(true) }, 4000) })
onHide(stop)
onUnload(()=>{disposed=true;stop()})
</script>
<style scoped>
.loading{text-align:center;padding:76.9231rpx 0}.stage-description{display:block;margin-top:7.6923rpx;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted)}.order-number{color:var(--muted);font-size:19.2308rpx;line-height:34.6154rpx;overflow-wrap:anywhere}.status-title{display:block;font-weight:750;color:var(--accent)}.summary-row{display:flex;align-items:flex-start;justify-content:space-between;min-width:0}.order-title{min-width:0;flex:1;font-weight:650;overflow-wrap:anywhere}.price{flex-shrink:0;color:var(--accent);font-weight:700;white-space:nowrap}.agreement-title{display:block;font-size:23.0769rpx;line-height:38.4615rpx;font-weight:600;color:var(--text)}.agreement-body{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:25rpx;line-height:44.2308rpx;color:var(--muted)}.requirements,.delivery{display:flex;flex-direction:column;gap:7.6923rpx;margin-top:19.2308rpx;min-width:0}.refund-panel{display:flex;flex-direction:column;gap:15.3846rpx;padding:23.0769rpx;border-left:3.8462rpx solid #d7b27666;color:#d9c4a4;font-size:23.0769rpx;line-height:1.7;overflow-wrap:anywhere;white-space:pre-wrap}.admin-resolution{display:flex;flex-direction:column;gap:23.0769rpx;margin:30.7692rpx 0;padding:23.0769rpx;background:var(--surface);border-radius:15.3846rpx;min-width:0}.review-panel{padding-top:38.4615rpx}.stars{display:flex;gap:15.3846rpx;margin:23.0769rpx 0}.rating-button{display:flex;align-items:center;justify-content:center;width:69.2308rpx;min-width:103.125rpx;height:69.2308rpx;min-height:44px;background:transparent;padding:0;color:var(--muted)}.stars .lit{color:#e5bd7e}.review-input{height:184.6154rpx;margin-bottom:15.3846rpx}.review-button{margin-top:23.0769rpx;width:230.7692rpx}.review-result{display:flex;flex-direction:column;gap:15.3846rpx;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle);font-size:25rpx;overflow-wrap:anywhere;white-space:pre-wrap}.actions{display:flex;align-items:center;flex-wrap:wrap;min-width:0}.updated{display:block;margin-top:30.7692rpx;text-align:center;font-size:19.2308rpx;line-height:34.6154rpx;color:var(--muted)}
.modal-mask{position:fixed;inset:0;z-index:110;display:flex;align-items:center;justify-content:center;padding:38.4615rpx;background:#000b}.modal{display:flex;flex-direction:column;gap:30.7692rpx;width:100%;max-width:460px;max-height:90vh;overflow-y:auto;padding:38.4615rpx;background:var(--surface);border-radius:23.0769rpx;min-width:0}.modal-title{font-size:38.4615rpx;font-weight:700}.modal-input{height:230.7692rpx;background:#ffffff04;border:1px solid var(--line-subtle);border-radius:15.3846rpx}.modal-actions{padding:0;gap:23.0769rpx}.payment-summary{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;font-size:34.6154rpx}.dev-warning{color:#d7b276;font-size:23.0769rpx}.error-panel{padding:30.7692rpx 0;border-bottom:1px solid #d7b27644;color:#efb299;font-size:23.0769rpx;overflow-wrap:anywhere}
.payment-options{display:flex;flex-direction:column;gap:0;border:1px solid var(--line);border-radius:15.3846rpx;overflow:hidden}.payment-option{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:107.6923rpx;padding:23.0769rpx;background:transparent;color:var(--muted);border-radius:0;text-align:left}.payment-option.chosen{color:var(--accent)}.payment-option-copy{display:flex;flex-direction:column;gap:7.6923rpx;flex:1;min-width:0;font-size:26.9231rpx;color:var(--text)}.payment-hint{display:block;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.payment-options>.payment-hint{padding:15.3846rpx 23.0769rpx;border-top:1px solid var(--line-subtle)}

.detail-content{padding-bottom:calc(153.8462rpx + env(safe-area-inset-bottom))}.order-heading{display:block;margin-bottom:30.7692rpx}.status-title{font-size:46.1538rpx;line-height:65.3846rpx}.order-number-row{display:flex;align-items:center;gap:11.5385rpx;min-width:0}.order-number{flex:1;min-width:0}.copy-number{display:flex;align-items:center;justify-content:center;width:46.1538rpx;height:46.1538rpx;padding:0;background:transparent;color:var(--muted)}.order-progress{display:flex;margin:15.3846rpx 0 38.4615rpx}.progress-step{position:relative;display:flex;flex-direction:column;align-items:center;flex:1;min-width:0;gap:7.6923rpx;color:var(--muted);font-size:23.0769rpx;line-height:34.6154rpx}.progress-icon{position:relative;z-index:1;display:flex;background:var(--bg)}.progress-line{position:absolute;top:17.3077rpx;left:calc(50% + 19.2308rpx);right:calc(-50% + 19.2308rpx);height:1px;background:var(--line)}.progress-line-done{background:var(--accent)}.progress-done,.progress-active{color:var(--accent)}.progress-detail{font-size:17.3077rpx;color:var(--muted)}.summary-card{display:flex;align-items:center;gap:19.2308rpx;padding:23.0769rpx;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--surface);min-width:0}.summary-cover{display:block;flex-shrink:0;width:157.6923rpx;height:138.4615rpx;border-radius:11.5385rpx}.summary-copy{flex:1;min-width:0}.order-title{font-size:28.8462rpx;line-height:42.3077rpx;display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden}.summary-row{gap:11.5385rpx}.price{font-size:30.7692rpx;line-height:42.3077rpx;max-width:55%}.summary-game,.summary-unit{display:block;color:var(--muted);font-size:21.1538rpx;line-height:34.6154rpx;margin-top:7.6923rpx;overflow-wrap:anywhere}.people-card{display:flex;gap:23.0769rpx;padding:23.0769rpx;margin:15.3846rpx 0;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--surface)}.person{display:flex;align-items:center;gap:15.3846rpx;flex:1;min-width:0;color:var(--muted)}.person-copy{min-width:0}.person-role{display:block;color:var(--muted);font-size:19.2308rpx;line-height:30.7692rpx}.person-name{display:block;font-size:25rpx;line-height:38.4615rpx;color:var(--text);white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.detail-section{margin-top:15.3846rpx;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--surface);overflow:hidden}.section-toggle{display:flex;align-items:center;gap:19.2308rpx;width:100%;min-height:92.3077rpx;padding:19.2308rpx 23.0769rpx;border-radius:0;background:transparent;color:var(--accent);text-align:left}.detail-section-title{flex:1;min-width:0;font-size:28.8462rpx;line-height:42.3077rpx;font-weight:650;color:var(--text)}.section-body{padding:0 23.0769rpx 23.0769rpx}.agreement{padding-top:0;border-top:0}.after-sale-heading{flex:1;min-width:0}.after-sale-hint{display:block;font-size:21.1538rpx;line-height:34.6154rpx;color:var(--muted);font-weight:400;margin-top:3.8462rpx}.refund-panel{margin:0 23.0769rpx 23.0769rpx;white-space:pre-wrap}.actions{padding:0 23.0769rpx 15.3846rpx;border-bottom:0;gap:15.3846rpx 30.7692rpx;justify-content:flex-end}.order-secondary{font-size:23.0769rpx;min-height:44px}.paid-row{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;color:var(--muted);font-size:26.9231rpx;padding:15.3846rpx 0}.order-dock{position:fixed;z-index:30;bottom:0;left:0;right:0;display:flex;align-items:center;gap:19.2308rpx;width:100%;max-width:600px;margin:0 auto;padding:15.3846rpx 23.0769rpx calc(15.3846rpx + env(safe-area-inset-bottom));background:var(--bg);border-top:1px solid var(--line)}.chat-entry{display:flex;align-items:center;justify-content:center;gap:11.5385rpx;min-height:44px;height:84.6154rpx;padding:0 23.0769rpx;flex:1;min-width:0;border:1px solid var(--accent);border-radius:13.4615rpx;background:transparent;color:var(--accent);font-size:25rpx}.dock-primary{flex:1.5;min-width:0;min-height:44px;height:84.6154rpx;border-radius:13.4615rpx;font-size:26.9231rpx}
</style>
