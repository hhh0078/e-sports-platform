<!-- 文件作用：订单履约、四方证据聊天室、退款协商及平台裁决的完整操作界面。 -->
<template>
  <AppShell title="订单详情" :game-id="order?.game_id" back>
    <view class="detail-content">
      <view v-if="error" class="error-panel" @click="load()">{{ error }} · 点击重试</view>
      <view v-if="!order && !error" class="muted loading">正在加载订单…</view>
      <template v-if="order">
        <view class="order-heading">
          <view class="status-icon"><Icon :name="order.status === 'completed' ? 'check-circle' : 'shield-check'" :size="28"/></view>
          <view class="status-copy"><text class="status-title">{{ statusLabel(order.status) }}</text><text class="order-number">订单编号 {{ order.order_no }}</text><text class="stage-description">{{ stageDescription }}</text></view>
        </view>
        <view class="summary">
          <view class="summary-row"><text class="order-title">{{ order.title }}</text><text class="price">¥{{ money(order.amount_cents) }}</text></view>
          <view class="people"><view class="person-line"><Icon name="user" :size="13"/><text>老板 · {{ order.buyer_name }}</text></view><view class="person-line"><Icon name="work" :size="13"/><text>打手 · {{ order.provider_name || '等待接单' }}</text></view></view>
          <view v-if="order.service_description" class="agreement"><text class="agreement-title">下单时的服务约定</text><text class="agreement-body">{{ order.service_description }}</text><text class="agreement-price">¥{{ money(order.unit_price_cents) }} / {{ order.unit }} × {{ order.quantity }}</text></view>
          <view class="requirements"><text class="agreement-title">额外要求</text><text class="agreement-body">{{ order.requirements || '无额外要求，以服务详情约定为准。' }}</text></view>
          <view v-if="order.delivery_note" class="delivery"><text class="agreement-title">交付说明</text><text class="agreement-body">{{ order.delivery_note }}</text></view>
        </view>
        <view v-if="refundActive" class="refund-panel"><view class="refund-heading"><Icon name="shield-alert" :size="18"/><text>{{ refundTitle }}</text></view><text>申请理由：{{ order.refund_reason }}</text><text v-if="order.refund_reply">处理说明：{{ order.refund_reply }}</text><text v-if="['requested','escalated','processing'].includes(order.refund_status)" class="muted">协商期间暂停验收与结算，订单证据会完整保留。</text></view>
        <view v-if="actions.length" class="actions"><button v-for="item in actions" :key="item.id" :disabled="busy" :class="item.primary ? 'btn order-primary' : 'btn-secondary btn-small order-secondary'" @click="prepare(item)">{{ item.label }}</button></view>
        <view v-if="state.user?.role === 'admin' && order.refund_status === 'escalated'" class="admin-resolution"><text class="section-title">平台裁决</text><text class="muted">先审阅下方聊天、交付与图片证据，再填写裁决原因。</text><button class="btn" @click="prepare({id:'resolve_refund',label:'裁决退款',note:true,tip:'请说明事实依据与退款原因'})">支持买方 · 原路退款</button><button class="btn-secondary btn-small" @click="prepare({id:'resolve_resume',label:'驳回退款',note:true,tip:'请说明驳回原因及后续履约要求'})">驳回申请 · 继续履约</button><button v-if="order.status === 'delivered'" class="btn-secondary btn-small" @click="prepare({id:'resolve_complete',label:'裁决完成',note:true,tip:'确认服务已按约定交付，并说明支持卖方的证据依据'})">支持卖方 · 确认完成</button></view>
        <view v-if="isBuyer && order.status === 'completed' && !reviews.length" class="form-section review-panel"><text class="section-title">这次并肩体验如何？</text><view class="stars"><button v-for="star in 5" :key="star" class="rating-button" @click="rating = star"><Icon name="star" :size="26" :class="{ lit: star <= rating }"/></button></view><textarea v-model="reviewText" class="input review-input" maxlength="300" placeholder="留下一点真实反馈，帮助下一个老板找到好队友"/><button class="btn-secondary btn-small review-button" :disabled="busy" @click="submitReview">发布评价</button></view>
        <view v-if="reviews.length" class="review-result"><text>买家评价 · {{ reviews[0].rating }} / 5</text><text class="muted">{{ reviews[0].content || '用户未填写文字评价' }}</text></view>
        <view class="chat-heading"><view><text class="section-title">订单聊天室</text><text class="chat-sub">老板 · 打手 · 俱乐部 · 平台</text></view><view class="evidence-badge"><Icon name="shield-check" :size="13"/>证据留存</view></view>
        <view class="chat-panel"><text class="chat-notice">仅订单相关成员可见。请把要求、进度与协商留在这里，避免线下付款或提供游戏账号密码。</text><scroll-view scroll-y class="chat-scroll" :scroll-into-view="scrollTarget" scroll-with-animation>
          <button v-if="hasEarlier" class="history-button" :disabled="loadingEarlier" @click="loadEarlier">{{ loadingEarlier ? '加载中…' : '查看更早的沟通记录' }}</button><view v-if="!messages.length" class="chat-empty">说清需求，合作从这里开始。</view>
          <view v-for="message in messages" :key="message.id" :id="'msg-' + message.id" :class="['message', { mine: message.sender_id === state.user?.id, system: message.kind === 'system' || message.sender_role === 'system' }]">
            <text v-if="message.kind === 'system' || message.sender_role === 'system'" class="system-message">{{ message.content }} · {{ time(message.created_at) }}</text>
            <template v-else><view class="message-meta"><text>{{ message.sender_name }}<text class="role-badge">{{ roleName(message.sender_role) }}</text></text><text>{{ time(message.created_at) }}</text></view><view class="bubble"><text v-if="message.content">{{ message.content }}</text><image v-if="message.image_url" class="evidence-image" :src="evidenceUrl(message.image_url)" mode="widthFix" @click="previewImage(message)" @error="refreshEvidence(message)"/></view></template>
          </view><view id="chat-bottom" class="chat-bottom"/>
        </scroll-view><view class="composer"><button class="image-button" :disabled="sending" aria-label="上传图片证据" @click="sendImage"><Icon name="image" :size="21"/></button><input v-model="draft" class="composer-input" maxlength="1000" confirm-type="send" placeholder="发送消息，或上传图片证据" @confirm="sendMessage"/><button class="send-button" :disabled="sending || !draft.trim()" @click="sendMessage">发送</button></view></view>
        <text class="updated">订单更新于 {{ time(order.updated_at) }} · {{ state.devMode ? '本地测试交易，不产生真实扣费' : '所有金额以订单约定为准' }}</text>
      </template>
    </view>
    <view v-if="pendingAction" class="modal-mask" @click="pendingAction = null"><view class="modal" @click.stop><text class="modal-title">{{ pendingAction.label }}</text><text class="muted">{{ pendingAction.tip || '确认后，系统会在聊天室记录这次操作。' }}</text><textarea v-if="pendingAction.note" v-model="actionNote" class="input modal-input" maxlength="1000" placeholder="请详细说明，作为双方协商与平台判断的依据"/><view v-if="pendingAction.id === 'pay'" class="payment-summary"><text>应付金额</text><text>¥{{ money(order?.amount_cents) }}</text></view><text v-if="pendingAction.id === 'pay' && state.devMode" class="dev-warning">当前为本地测试支付，不会真实扣款。</text><view class="action-footer modal-actions"><button class="text-btn" :disabled="busy" @click="pendingAction = null">再想想</button><button class="btn" :disabled="busy || (pendingAction.note && !actionNote.trim())" @click="execute">{{ busy ? '处理中…' : '确认' + pendingAction.label }}</button></view></view></view>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：驱动订单状态操作、证据聊天和退款介入，所有权与状态最终由后端校验。 */
import { ref, computed, nextTick, watch } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { prepareImage } from '../../lib/media'
import { getOrder, listMessages, listOrderReviews, payOrder, syncOrderPayment, performOrderAction, sendOrderMessage, submitOrderReview, uploadEvidence, refreshEvidenceLink } from '../../api/orders.api'
import { retryOrderRefund, resolveOrder } from '../../api/admin.api'
import type { Order, Message, Review } from '../../types/models'
import { token, errorMessage, errorStatus, isCancelled } from '../../api/client'
import { state, init, sessionRevision } from '../../lib/store'
import { money, time, statusLabel, notify, assetUrl } from '../../lib/format'
/** 订单状态操作及确认表单配置。 */
interface OrderAction { id: string; label: string; primary?: boolean; note?: boolean; tip?: string }
const id = ref(''), order = ref<Order | null>(null), messages = ref<Message[]>([]), reviews = ref<Review[]>([]), error = ref(''), busy = ref(false), sending = ref(false), draft = ref(''), scrollTarget = ref(''), pendingAction = ref<OrderAction | null>(null), actionNote = ref(''), rating = ref(5), reviewText = ref('')
const hasEarlier = ref(false), loadingEarlier = ref(false)
let timer: ReturnType<typeof setInterval> | undefined, polling = false, visible = false, chatInitialized = false, viewerID = '', pageVersion = 0
/** 捕获一次订单操作的账号和会话，防止迟到响应进入其他账号。 */
function currentSession() { return { id:state.user?.id, token:token(), revision:sessionRevision() } }
/** 微信支付和原生选图可以临时隐藏页面，但只能由原账号继续请求。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return session.id===state.user?.id&&session.token===token()&&session.revision===sessionRevision() }
/** 页面显示且版本、会话都一致时才回填私有订单和证据。 */
function currentRequest(version: number, session: ReturnType<typeof currentSession>): boolean { return visible&&version===pageVersion&&sameSession(session) }
/** 切换身份立即移除上一位参与者的私有内容和未发送草稿。 */
function clearPrivateOrder(): void { order.value=null;messages.value=[];reviews.value=[];draft.value='';pendingAction.value=null;actionNote.value='';reviewText.value='';chatInitialized=false;hasEarlier.value=false;loadingEarlier.value=false;pageVersion++;polling=false }
watch(()=>state.user?.id,()=>{clearPrivateOrder();if(visible)load()})
const isBuyer = computed(() => order.value?.buyer_id === state.user?.id)
const isProvider = computed(() => order.value?.provider_id === state.user?.id)
const refundActive = computed(() => order.value?.refund_status && order.value.refund_status !== 'none')
const refundTitle = computed(() => (({ requested:'等待打手处理退款', rejected:'退款申请被拒绝，可补充证据并申请平台介入', escalated:'平台正在处理争议', approved:'退款已处理', processing:'微信正在处理退款，请稍后同步资金状态' } as Record<string,string>)[order.value?.refund_status || ''] || '售后记录'))
const stageDescription = computed(() => (({ pending_payment:'支付后会通知队友，请确认服务要求和金额。', open:'需求已发布，等待合适的队友接单。', accepted:'队友已接单，可以在聊天室确认集合时间。', in_progress:'服务正在进行，请及时同步进度与约定。', delivered:'队友已提交交付说明，请核对后确认完成。', completed:'本次服务已完成，感谢选择集结。', cancelled:'订单已取消，双方沟通记录仍然保留。', refunded:'退款已完成，本次交易关闭。', disputed:'平台正在核对双方证据，请在聊天室补充说明。' } as Record<string,string>)[order.value?.status || ''] || ''))
const actions = computed(() => {
  const result: OrderAction[] = [], current = order.value
  if (!current) return result
  const status = current.status, refund = current.refund_status || 'none', suspended = ['requested','escalated','processing'].includes(refund)
  if (!state.devMode && (isBuyer.value || state.user?.role === 'admin') && (status === 'pending_payment' || refund === 'processing')) result.push({ id:'sync_payment', label:'同步资金状态' })
  if (!state.devMode && state.user?.role === 'admin' && refund === 'processing') result.push({ id:'retry_refund', label:'重试退款通道', tip:'沿用原退款单号重试未知结果，不会发起第二笔退款。' })
  if (isBuyer.value && status === 'pending_payment') result.push({ id:'pay', label:'支付订单', primary:true }, { id:'cancel', label:'取消订单' })
  if (isBuyer.value && status === 'open' && !suspended) result.push({ id:'cancel', label:'取消订单' })
  if (isProvider.value && status === 'open' && !suspended) result.push({ id:'accept', label:'接受订单', primary:true })
  if (isProvider.value && status === 'accepted' && !suspended) result.push({ id:'start', label:'开始服务', primary:true })
  if (isProvider.value && status === 'in_progress' && !suspended) result.push({ id:'deliver', label:'提交交付', primary:true, note:true, tip:'说明已经完成的内容，必要时在聊天室上传战绩或任务截图。' })
  if (isBuyer.value && status === 'delivered' && !suspended) result.push({ id:'confirm', label:'验收并完成', primary:true, tip:'确认服务已按约定完成。确认后该订单计入打手收入。' })
  if (isBuyer.value && ['open','accepted','in_progress','delivered','completed'].includes(status) && !suspended && refund !== 'approved') result.push({ id:'request_refund', label:refund === 'rejected' ? '补充退款申请' : '申请退款', note:true, tip:'请写清退款原因。打手可同意或拒绝；有争议可申请平台介入。' })
  if (isBuyer.value && ['requested','rejected'].includes(refund)) result.push({ id:'escalate', label:'申请平台介入', note:true, tip:'请说明争议要点，平台将结合订单要求、聊天和图片证据处理。' })
  if (isBuyer.value && ['requested','rejected'].includes(refund)) result.push({ id:'withdraw_refund', label:'撤销退款申请' })
  if (isProvider.value && refund === 'requested') result.push({ id:'approve_refund', label:'同意退款', primary:true }, { id:'reject_refund', label:'拒绝退款', note:true, tip:'请详细说明拒绝理由，并在聊天室补充履约证据。' })
  return result
})
/** 加载订单与参与者可见的证据，轮询失败只显示错误、不销毁现有记录。 */
async function load(silent = false) {
  if (!id.value || polling || !visible) return
  const version=pageVersion
  let session: ReturnType<typeof currentSession> | undefined
  polling = true
  try {
    await init()
    if(!visible||version!==pageVersion)return
    session=currentSession()
    if (viewerID !== (state.user?.id || '')) { viewerID = state.user?.id || ''; order.value = null; messages.value = []; reviews.value = []; chatInitialized = false; hasEarlier.value = false }
    const last = messages.value[messages.value.length - 1]
    const messageQuery = {limit:100,...(chatInitialized && last ? {after_id:last.id} : {})}
    const data = await Promise.all([getOrder(id.value), listMessages(id.value,messageQuery), listOrderReviews(id.value)])
    if(!currentRequest(version,session))return
    order.value = data[0]; reviews.value = data[2]; error.value = ''
    if (!chatInitialized) { hasEarlier.value = data[1].length === 100; chatInitialized = true }
    const changed = data[1].some(item => !messages.value.some(old => old.id === item.id))
    mergeMessages(data[1])
    if (changed) await scrollChat()
  } catch (e) { if(!visible||version!==pageVersion||(session&&!sameSession(session)))return;if ([401,403,404].includes(errorStatus(e) || 0)) { clearPrivateOrder(); clearInterval(timer);error.value = errorMessage(e) } else if (!silent || !order.value) error.value = errorMessage(e) } finally { if(version===pageVersion)polling = false }
}
/** 按稳定时间和编号去重合并，轮询不会丢弃已加载的历史证据。 */
function mergeMessages(items: Message[]) {
  const merged = new Map(messages.value.map(item => [item.id, item]))
  items.forEach(item => merged.set(item.id, item))
  messages.value = [...merged.values()].sort((a,b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id))
}
/** 用最早一条消息的游标加载历史，新消息到达不会使分页偏移。 */
async function loadEarlier() {
  if (loadingEarlier.value || !messages.value.length) return
  const version=pageVersion,session=currentSession()
  loadingEarlier.value = true
  const first = messages.value[0].id
  try { const items = await listMessages(id.value,{limit:100,before_id:first});if(!currentRequest(version,session))return;hasEarlier.value = items.length === 100; mergeMessages(items); scrollTarget.value = ''; await nextTick();if(currentRequest(version,session))scrollTarget.value = 'msg-' + first } catch(e) { if(currentRequest(version,session))notify(e) } finally { if(version===pageVersion)loadingEarlier.value = false }
}
/** 让新消息滚动到末尾，避免重复设置同一锚点无效。 */
async function scrollChat() { scrollTarget.value = ''; await nextTick(); scrollTarget.value = 'chat-bottom' }
/** 准备可审阅的确认表单，不立即触发金额或状态变更。 */
function prepare(action: OrderAction) { pendingAction.value = action; actionNote.value = '' }
/** 付款、履约与退款操作使用服务端状态机，提交中禁用重复点击。 */
async function execute() {
  if (busy.value || !pendingAction.value) return
  const session=currentSession()
  busy.value = true
  try {
    const action = pendingAction.value.id
    if (action === 'pay') { const result = await payOrder(id.value);if(!sameSession(session))return;if (result.mode !== 'dev' && result.pay_params) { const params=result.pay_params; await new Promise<void>((resolve, reject) => uni.requestPayment({ provider:'wxpay', ...params, success:()=>resolve(), fail:reject }));if(!sameSession(session))return;await syncOrderPayment(id.value) } }
    else if (action === 'retry_refund') await retryOrderRefund(id.value)
    else if (action === 'sync_payment') await syncOrderPayment(id.value)
    else if (action.startsWith('resolve_')) await resolveOrder(id.value,action.slice(8),actionNote.value.trim())
    else await performOrderAction(id.value,action,actionNote.value.trim())
    if(!sameSession(session))return;pendingAction.value = null; if(visible){notify('操作成功，订单记录已更新');await load()}
  } catch (e) { if(sameSession(session)&&visible)notify(e) } finally { busy.value = false }
}
/** 发送纯文字消息，失败时保留输入，成功后才清空。 */
async function sendMessage() { if (sending.value || !draft.value.trim()) return;const session=currentSession(); sending.value = true; try { await sendOrderMessage(id.value,{content:draft.value.trim()});if(!sameSession(session))return;draft.value = '';if(visible){await load();await scrollChat()} } catch (e) { if(sameSession(session)&&visible)notify(e) } finally { sending.value = false } }
/** 选择一张图片并作为订单证据发送，取消选择不显示错误。 */
async function sendImage() { if (sending.value) return;const session=currentSession(); sending.value = true; try { const path = await prepareImage('evidence');if(!sameSession(session))return;const evidence = await uploadEvidence(id.value,path);if(!sameSession(session))return;await sendOrderMessage(id.value,{content:'',image_url:evidence.url});if(visible&&sameSession(session)){await load();await scrollChat()} } catch (e) { if(sameSession(session)&&visible&&!isCancelled(e)) notify(e) } finally { sending.value = false } }
/** 将安全的签名证据相对路径转换为实际服务地址。 */
function evidenceUrl(url: string | undefined): string { return assetUrl(url) }
const refreshingImages = new Set<string>()
/** 重新授权即将阅读的历史图片，避免十五分钟签名过期影响证据查看。 */
async function refreshEvidence(message: Message) {
  if (!message.image_url || refreshingImages.has(message.id)) return
  const version=pageVersion,session=currentSession()
  refreshingImages.add(message.id)
  try { const evidenceID = message.image_url.split('?')[0].split('/').pop() || ''; const result = await refreshEvidenceLink(id.value,evidenceID);if(!currentRequest(version,session))return;message.image_url = result.url; return result.url } catch (e) { if(currentRequest(version,session))notify(e) } finally { refreshingImages.delete(message.id) }
}
/** 打开图片证据前刷新访问签名，不使用过期链接。 */
async function previewImage(message: Message) { const url = await refreshEvidence(message); if (url) uni.previewImage({ urls:[evidenceUrl(url)] }) }
/** 将证据中的发送身份映射为明确的角色名称。 */
function roleName(role: string) { return ({ buyer:'老板',provider:'打手',club_admin:'俱乐部',admin:'平台',system:'系统' } as Record<string,string>)[role] || '成员' }
/** 完成后发表一次评价，重复评价由服务端拦截。 */
async function submitReview() { if (busy.value) return; if (reviewText.value.trim().length < 2) return notify('请填写至少两个字的真实评价');const session=currentSession(); busy.value = true; try { await submitOrderReview(id.value,{rating:rating.value,content:reviewText.value.trim()});if(sameSession(session)&&visible){await load();notify('评价已保存')} } catch (e) { if(sameSession(session)&&visible)notify(e) } finally { busy.value = false } }
/** 停止页面离开后的轮询，避免后台读取与泄露账号切换后的数据。 */
function stop() { visible = false;pageVersion++;polling=false;loadingEarlier.value=false;clearInterval(timer) }
onLoad(options => { id.value = options?.id || '';if(!id.value)error.value='订单不存在或缺少编号' })
onShow(() => { visible = true; load(); clearInterval(timer); timer = setInterval(() => { if (visible) load(true) }, 4000) })
onHide(stop)
onUnload(stop)
</script>
<style scoped>
.detail-content{min-width:0;padding-bottom:24px}.loading{text-align:center;padding:40px 0}.order-heading{display:flex;align-items:flex-start;gap:10px;margin-bottom:20px;min-width:0}.status-icon{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:30px;height:32px;color:var(--accent)}.status-copy{flex:1;min-width:0}.status-title{display:block;font-size:21px;line-height:30px;font-weight:750;color:var(--accent)}.order-number{display:block;margin-top:2px;color:var(--muted);font-size:10px;line-height:18px;overflow-wrap:anywhere}.stage-description{display:block;margin-top:4px;font-size:11px;line-height:18px;color:var(--muted)}
.summary{min-width:0;padding-bottom:16px;border-bottom:1px solid var(--line-subtle)}.summary-row{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;min-width:0}.order-title{min-width:0;flex:1;font-size:18px;line-height:27px;font-weight:700;overflow-wrap:anywhere}.price{flex-shrink:0;max-width:50%;color:var(--accent);font-size:18px;line-height:27px;font-weight:700;white-space:nowrap}.people{display:flex;flex-direction:column;gap:4px;color:var(--muted);font-size:12px;margin:8px 0 12px;min-width:0}.person-line{display:flex;align-items:center;gap:8px;min-width:0;overflow-wrap:anywhere}.agreement{display:flex;flex-direction:column;gap:4px;padding-top:12px;border-top:1px solid var(--line-subtle)}.agreement-title{display:block;font-size:12px;line-height:20px;font-weight:600;color:var(--text)}.agreement-body{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:12px;line-height:21px;color:var(--muted)}.agreement-price{font-size:12px;line-height:20px;color:var(--accent)}.requirements,.delivery{display:flex;flex-direction:column;gap:4px;margin-top:12px;min-width:0}
.actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px;padding:12px 0 16px;border-bottom:1px solid var(--line-subtle);min-width:0}.order-primary{flex:1;min-width:160px}.order-secondary{flex-shrink:0;max-width:100%}.refund-panel{display:flex;flex-direction:column;gap:8px;padding:12px;border-left:2px solid #d7b27666;border-radius:8px;background:#a5782010;color:#d9c4a4;font-size:12px;line-height:1.7;margin-top:16px;overflow-wrap:anywhere}.refund-heading{display:flex;align-items:center;gap:8px;font-weight:650;color:#e5bd7e}.admin-resolution{display:flex;flex-direction:column;gap:12px;margin:24px 0;padding:16px;background:var(--surface);border-radius:8px;min-width:0}.review-panel{padding-top:24px}.stars{display:flex;gap:8px;margin:12px 0}.rating-button{display:flex;align-items:center;justify-content:center;width:36px;height:36px;background:transparent;padding:0;color:var(--muted)}.stars .lit{color:#e5bd7e}.review-input{height:96px;margin-bottom:12px}.review-button{width:112px}.review-result{display:flex;flex-direction:column;gap:8px;padding:20px 0;border-bottom:1px solid var(--line-subtle);font-size:13px;overflow-wrap:anywhere}
.chat-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:24px 0 12px;min-width:0}.chat-sub{display:block;color:var(--muted);font-size:10px;margin-top:4px}.evidence-badge{display:flex;align-items:center;gap:4px;flex-shrink:0;font-size:10px;color:var(--accent);border-radius:12px;padding:3px 7px;background:#83e7b712}.chat-panel{border-radius:8px;overflow:hidden;background:var(--surface);min-width:0}.chat-notice{display:block;padding:12px;font-size:10px;line-height:1.7;color:var(--muted);border-bottom:1px solid var(--line-subtle)}.chat-scroll{height:340px;padding:0 12px;box-sizing:border-box;max-width:100%}.history-button{display:flex;align-items:center;justify-content:center;width:100%;min-height:36px;font-size:11px;background:transparent;color:var(--accent)}.chat-empty{padding:64px 0;font-size:12px;text-align:center;color:var(--muted)}.message{display:flex;flex-direction:column;align-items:flex-start;margin:16px 0;min-width:0}.message.mine{align-items:flex-end}.message.system{align-items:center}.system-message{max-width:100%;font-size:10px;line-height:1.7;color:var(--muted);text-align:center;overflow-wrap:anywhere}.message-meta{display:flex;align-items:center;flex-wrap:wrap;gap:8px;max-width:100%;font-size:10px;color:var(--muted);margin-bottom:6px;overflow-wrap:anywhere}.role-badge{margin-left:6px;font-size:9px;padding:1px 4px;border-radius:4px;background:#83e7b71a;color:var(--accent)}.bubble{max-width:88%;padding:10px 12px;border-radius:2px 8px 8px;background:#ffffff09;color:var(--text);font-size:13px;line-height:1.7;overflow-wrap:anywhere;white-space:pre-wrap}.mine .bubble{background:var(--accent);color:#0a2116;border-radius:8px 2px 8px 8px}.evidence-image{display:block;width:180px;max-width:100%;border-radius:6px}.composer{display:flex;align-items:center;gap:8px;min-width:0;padding:12px;border-top:1px solid var(--line-subtle)}.composer-input{flex:1;min-width:0;height:36px;padding:0 12px;border:1px solid var(--line-subtle);border-radius:18px;background:#ffffff04;font-size:11px}.image-button{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:36px;height:36px;padding:0;border-radius:8px;border:1px solid var(--line);background:transparent;color:var(--text)}.send-button{display:flex;align-items:center;justify-content:center;flex-shrink:0;height:36px;padding:0 14px;border-radius:18px;background:var(--accent);color:#0a2116;font-size:12px}.updated{display:block;margin-top:16px;text-align:center;font-size:10px;line-height:18px;color:var(--muted)}.chat-bottom{height:4px}
.modal-mask{position:fixed;inset:0;z-index:110;display:flex;align-items:center;justify-content:center;padding:20px;background:#000b}.modal{display:flex;flex-direction:column;gap:16px;width:100%;max-width:460px;max-height:90vh;overflow-y:auto;padding:20px;background:var(--surface);border-radius:12px;min-width:0}.modal-title{font-size:20px;font-weight:700}.modal-input{height:120px;background:#ffffff04;border:1px solid var(--line-subtle);border-radius:8px}.modal-actions{padding:0;gap:12px}.payment-summary{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0;font-size:18px}.dev-warning{color:#d7b276;font-size:12px}.error-panel{padding:16px 0;border-bottom:1px solid #d7b27644;color:#efb299;font-size:12px;overflow-wrap:anywhere}
</style>
