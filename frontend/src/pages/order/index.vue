<!-- 文件作用：订单履约、四方证据聊天室、退款协商及平台裁决的完整操作界面。 -->
<template>
  <AppShell title="订单详情" back>
    <view class="detail-content">
      <view v-if="error" class="error-panel" @click="load">{{ error }} · 点击重试</view>
      <view v-if="!order && !error" class="muted loading">正在加载订单…</view>
      <template v-if="order">
        <view class="order-heading"><view><text class="eyebrow">{{ order.order_no }}</text><text class="status-title">{{ statusLabel(order.status) }}</text></view><view class="status-icon"><Icon :name="order.status === 'completed' ? 'check-circle' : 'shield-check'" :size="27"/></view></view>
        <text class="stage-description">{{ stageDescription }}</text>
        <view class="summary panel"><view class="summary-row"><text class="order-title">{{ order.title }}</text><text class="price">¥{{ money(order.amount_cents) }}</text></view><view class="people"><text>老板 · {{ order.buyer_name }}</text><text>打手 · {{ order.provider_name || '等待接单' }}</text></view><view v-if="order.service_description" class="agreement"><text class="agreement-title">下单时的服务约定</text><text>{{ order.service_description }}</text><text class="muted">¥{{ money(order.unit_price_cents) }} / {{ order.unit }} × {{ order.quantity }}</text></view><text class="requirements">{{ order.requirements || '无额外要求，以服务详情约定为准。' }}</text><text v-if="order.delivery_note" class="delivery">交付说明：{{ order.delivery_note }}</text></view>
        <view v-if="refundActive" class="refund-panel"><view class="refund-heading"><Icon name="shield-alert" :size="18"/><text>{{ refundTitle }}</text></view><text>申请理由：{{ order.refund_reason }}</text><text v-if="order.refund_reply">处理说明：{{ order.refund_reply }}</text><text v-if="['requested','escalated','processing'].includes(order.refund_status)" class="muted">协商期间暂停验收与结算，订单证据会完整保留。</text></view>
        <view v-if="actions.length" class="actions"><button v-for="item in actions" :key="item.id" :disabled="busy" :class="item.primary ? 'btn' : 'btn-secondary'" @click="prepare(item)">{{ item.label }}</button></view>
        <view v-if="state.user?.role === 'admin' && order.refund_status === 'escalated'" class="admin-resolution panel"><text class="section-title">平台裁决</text><text class="muted">先审阅下方聊天、交付与图片证据，再填写裁决原因。</text><button class="btn" @click="prepare({id:'resolve_refund',label:'裁决退款',note:true,tip:'请说明事实依据与退款原因'})">支持买方 · 原路退款</button><button class="btn-secondary" @click="prepare({id:'resolve_resume',label:'驳回退款',note:true,tip:'请说明驳回原因及后续履约要求'})">驳回申请 · 继续履约</button><button v-if="order.status === 'delivered'" class="btn-secondary" @click="prepare({id:'resolve_complete',label:'裁决完成',note:true,tip:'确认服务已按约定交付，并说明支持卖方的证据依据'})">支持卖方 · 确认完成</button></view>
        <view v-if="isBuyer && order.status === 'completed' && !reviews.length" class="review-panel panel"><text class="section-title">这次并肩体验如何？</text><view class="stars"><button v-for="star in 5" :key="star" @click="rating = star"><Icon name="star" :size="26" :class="{ lit: star <= rating }"/></button></view><textarea v-model="reviewText" class="input review-input" maxlength="300" placeholder="留下一点真实反馈，帮助下一个老板找到好队友"/><button class="btn-secondary" :disabled="busy" @click="submitReview">发布评价</button></view>
        <view v-if="reviews.length" class="panel review-result"><text>买家评价 · {{ reviews[0].rating }} / 5</text><text class="muted">{{ reviews[0].content || '用户未填写文字评价' }}</text></view>
        <view class="chat-heading"><view><text class="section-title">订单聊天室</text><text class="chat-sub">老板 · 打手 · 俱乐部 · 平台</text></view><view class="evidence-badge"><Icon name="shield-check" :size="13"/>证据留存</view></view>
        <view class="chat-panel"><text class="chat-notice">仅订单相关成员可见。请把要求、进度与协商留在这里，避免线下付款或提供游戏账号密码。</text><scroll-view scroll-y class="chat-scroll" :scroll-into-view="scrollTarget" scroll-with-animation>
          <button v-if="hasEarlier" class="history-button" :disabled="loadingEarlier" @click="loadEarlier">{{ loadingEarlier ? '加载中…' : '查看更早的沟通记录' }}</button><view v-if="!messages.length" class="chat-empty">说清需求，合作从这里开始。</view>
          <view v-for="message in messages" :key="message.id" :id="'msg-' + message.id" :class="['message', { mine: message.sender_id === state.user?.id, system: message.kind === 'system' || message.sender_role === 'system' }]">
            <text v-if="message.kind === 'system' || message.sender_role === 'system'" class="system-message">{{ message.content }} · {{ time(message.created_at) }}</text>
            <template v-else><view class="message-meta"><text>{{ message.sender_name }}<text class="role-badge">{{ roleName(message.sender_role) }}</text></text><text>{{ time(message.created_at) }}</text></view><view class="bubble"><text v-if="message.content">{{ message.content }}</text><image v-if="message.image_url" :src="evidenceUrl(message.image_url)" mode="widthFix" @click="previewImage(message)" @error="refreshEvidence(message)"/></view></template>
          </view><view id="chat-bottom" class="chat-bottom"/>
        </scroll-view><view class="composer"><button class="image-button" :disabled="sending" aria-label="上传图片证据" @click="sendImage"><Icon name="image" :size="23"/></button><input v-model="draft" maxlength="1000" confirm-type="send" placeholder="发送消息，或上传图片证据" @confirm="sendMessage"/><button class="send-button" :disabled="sending || !draft.trim()" @click="sendMessage">发送</button></view></view>
        <text class="updated">订单更新于 {{ time(order.updated_at) }} · {{ state.devMode ? '本地测试交易，不产生真实扣费' : '所有金额以订单约定为准' }}</text>
      </template>
    </view>
    <view v-if="pendingAction" class="modal-mask" @click="pendingAction = null"><view class="modal panel" @click.stop><text class="modal-title">{{ pendingAction.label }}</text><text class="muted">{{ pendingAction.tip || '确认后，系统会在聊天室记录这次操作。' }}</text><textarea v-if="pendingAction.note" v-model="actionNote" class="input" maxlength="1000" placeholder="请详细说明，作为双方协商与平台判断的依据"/><view v-if="pendingAction.id === 'pay'" class="payment-summary"><text>应付金额</text><text>¥{{ money(order.amount_cents) }}</text></view><text v-if="pendingAction.id === 'pay' && state.devMode" class="dev-warning">当前为本地测试支付，不会真实扣款。</text><view class="modal-actions"><button class="btn-secondary" :disabled="busy" @click="pendingAction = null">再想想</button><button class="btn" :disabled="busy || (pendingAction.note && !actionNote.trim())" @click="execute">{{ busy ? '处理中…' : '确认' + pendingAction.label }}</button></view></view></view>
  </AppShell>
</template>
<script setup>
/** 文件作用：驱动订单状态操作、证据聊天和退款介入，所有权与状态最终由后端校验。 */
import { ref, computed, nextTick } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { prepareImage } from '../../lib/media'
import { api, uploadEvidence, API_BASE } from '../../lib/api'
import { state, init } from '../../lib/store'
import { money, time, statusLabel, notify } from '../../lib/format'
const id = ref(''), order = ref(null), messages = ref([]), reviews = ref([]), error = ref(''), busy = ref(false), sending = ref(false), draft = ref(''), scrollTarget = ref(''), pendingAction = ref(null), actionNote = ref(''), rating = ref(5), reviewText = ref('')
const hasEarlier = ref(false), loadingEarlier = ref(false)
let timer, polling = false, visible = false, chatInitialized = false, viewerID = ''
const isBuyer = computed(() => order.value?.buyer_id === state.user?.id)
const isProvider = computed(() => order.value?.provider_id === state.user?.id)
const refundActive = computed(() => order.value?.refund_status && order.value.refund_status !== 'none')
const refundTitle = computed(() => ({ requested:'等待打手处理退款', rejected:'退款申请被拒绝，可补充证据并申请平台介入', escalated:'平台正在处理争议', approved:'退款已处理', processing:'微信正在处理退款，请稍后同步资金状态' }[order.value?.refund_status] || '售后记录'))
const stageDescription = computed(() => ({ pending_payment:'支付后会通知队友，请确认服务要求和金额。', open:'需求已发布，等待合适的队友接单。', accepted:'队友已接单，可以在聊天室确认集合时间。', in_progress:'服务正在进行，请及时同步进度与约定。', delivered:'队友已提交交付说明，请核对后确认完成。', completed:'本次服务已完成，感谢选择集结。', cancelled:'订单已取消，双方沟通记录仍然保留。', refunded:'退款已完成，本次交易关闭。', disputed:'平台正在核对双方证据，请在聊天室补充说明。' }[order.value?.status] || ''))
const actions = computed(() => {
  const result = [], current = order.value
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
  if (!id.value || polling) return
  polling = true
  try {
    await init()
    if (viewerID !== (state.user?.id || '')) { viewerID = state.user?.id || ''; order.value = null; messages.value = []; reviews.value = []; chatInitialized = false; hasEarlier.value = false }
    const last = messages.value[messages.value.length - 1]
    const messagePath = '/orders/' + id.value + '/messages?limit=100' + (chatInitialized && last ? '&after_id=' + encodeURIComponent(last.id) : '')
    const data = await Promise.all([api('/orders/' + id.value), api(messagePath), api('/orders/' + id.value + '/reviews')])
    order.value = data[0]; reviews.value = data[2]; error.value = ''
    if (!chatInitialized) { hasEarlier.value = data[1].length === 100; chatInitialized = true }
    const changed = data[1].some(item => !messages.value.some(old => old.id === item.id))
    mergeMessages(data[1])
    if (changed) await scrollChat()
  } catch (e) { if ([401,403,404].includes(e.status)) { stop(); order.value = null; messages.value = []; reviews.value = []; chatInitialized = false; error.value = e.message } else if (!silent || !order.value) error.value = e.message } finally { polling = false }
}
/** 按稳定时间和编号去重合并，轮询不会丢弃已加载的历史证据。 */
function mergeMessages(items) {
  const merged = new Map(messages.value.map(item => [item.id, item]))
  items.forEach(item => merged.set(item.id, item))
  messages.value = [...merged.values()].sort((a,b) => a.created_at.localeCompare(b.created_at) || a.id.localeCompare(b.id))
}
/** 用最早一条消息的游标加载历史，新消息到达不会使分页偏移。 */
async function loadEarlier() {
  if (loadingEarlier.value || !messages.value.length) return
  loadingEarlier.value = true
  const first = messages.value[0].id
  try { const items = await api('/orders/' + id.value + '/messages?limit=100&before_id=' + encodeURIComponent(first)); hasEarlier.value = items.length === 100; mergeMessages(items); scrollTarget.value = ''; await nextTick(); scrollTarget.value = 'msg-' + first } catch(e) { notify(e) } finally { loadingEarlier.value = false }
}
/** 让新消息滚动到末尾，避免重复设置同一锚点无效。 */
async function scrollChat() { scrollTarget.value = ''; await nextTick(); scrollTarget.value = 'chat-bottom' }
/** 准备可审阅的确认表单，不立即触发金额或状态变更。 */
function prepare(action) { pendingAction.value = action; actionNote.value = '' }
/** 付款、履约与退款操作使用服务端状态机，提交中禁用重复点击。 */
async function execute() {
  if (busy.value || !pendingAction.value) return
  busy.value = true
  try {
    const action = pendingAction.value.id
    if (action === 'pay') { const result = await api('/orders/' + id.value + '/pay','POST',{}); if (result.mode !== 'dev' && result.pay_params) { await new Promise((resolve, reject) => uni.requestPayment({ ...result.pay_params, success:resolve, fail:reject })); await api('/orders/' + id.value + '/sync-payment','POST',{}) } }
    else if (action === 'retry_refund') await api('/admin/orders/' + id.value + '/retry-refund','POST',{})
    else if (action === 'sync_payment') await api('/orders/' + id.value + '/sync-payment','POST',{})
    else if (action.startsWith('resolve_')) await api('/admin/orders/' + id.value + '/resolve','POST',{ resolution:action.slice(8), note:actionNote.value.trim() })
    else await api('/orders/' + id.value + '/actions','POST',{ action, note:actionNote.value.trim() })
    pendingAction.value = null; notify('操作成功，订单记录已更新'); await load()
  } catch (e) { notify(e) } finally { busy.value = false }
}
/** 发送纯文字消息，失败时保留输入，成功后才清空。 */
async function sendMessage() { if (sending.value || !draft.value.trim()) return; sending.value = true; try { await api('/orders/' + id.value + '/messages','POST',{content:draft.value.trim()}); draft.value = ''; await load(); await scrollChat() } catch (e) { notify(e) } finally { sending.value = false } }
/** 选择一张图片并作为订单证据发送，取消选择不显示错误。 */
async function sendImage() { if (sending.value) return; sending.value = true; try { const path = await prepareImage('evidence'); const evidence = await uploadEvidence(id.value,path); await api('/orders/' + id.value + '/messages','POST',{content:'',image_url:evidence.url}); await load(); await scrollChat() } catch (e) { if(!e.cancelled) notify(e) } finally { sending.value = false } }
/** 将安全的签名证据相对路径转换为实际服务地址。 */
function evidenceUrl(url) { return url?.startsWith('/') ? API_BASE.replace(/\/api\/?$/, '') + url : url }
const refreshingImages = new Set()
/** 重新授权即将阅读的历史图片，避免十五分钟签名过期影响证据查看。 */
async function refreshEvidence(message) {
  if (!message.image_url || refreshingImages.has(message.id)) return
  refreshingImages.add(message.id)
  try { const evidenceID = message.image_url.split('?')[0].split('/').pop(); const result = await api('/orders/' + id.value + '/evidence-link?evidence_id=' + encodeURIComponent(evidenceID)); message.image_url = result.url; return result.url } catch (e) { notify(e) } finally { refreshingImages.delete(message.id) }
}
/** 打开图片证据前刷新访问签名，不使用过期链接。 */
async function previewImage(message) { const url = await refreshEvidence(message); if (url) uni.previewImage({ urls:[evidenceUrl(url)] }) }
/** 将证据中的发送身份映射为明确的角色名称。 */
function roleName(role) { return { buyer:'老板',provider:'打手',club_admin:'俱乐部',admin:'平台',system:'系统' }[role] || '成员' }
/** 完成后发表一次评价，重复评价由服务端拦截。 */
async function submitReview() { if (busy.value) return; if (reviewText.value.trim().length < 2) return notify('请填写至少两个字的真实评价'); busy.value = true; try { await api('/orders/' + id.value + '/review','POST',{rating:rating.value,content:reviewText.value.trim()}); await load(); notify('评价已保存') } catch (e) { notify(e) } finally { busy.value = false } }
/** 停止页面离开后的轮询，避免后台读取与泄露账号切换后的数据。 */
function stop() { visible = false; clearInterval(timer) }
onLoad(options => { id.value = options.id || ''; load() })
onShow(() => { visible = true; load(); clearInterval(timer); timer = setInterval(() => { if (visible) load(true) }, 4000) })
onHide(stop)
onUnload(stop)
</script>
<style scoped>
.agreement{display:flex;flex-direction:column;gap:8px;font-size:12px;line-height:1.8;margin:12px 0;white-space:pre-wrap}.agreement-title{color:var(--accent);font-weight:650}.history-button{background:transparent;color:var(--accent);font-size:11px;line-height:36px}.detail-content{padding:4px 0 32px}.loading{text-align:center;padding:50px}.order-heading{display:flex;justify-content:space-between;align-items:center;margin:14px 0 9px}.eyebrow{display:block;font-size:10px;color:#70847a;letter-spacing:1px}.status-title{display:block;font-size:29px;font-weight:800;margin-top:7px}.status-icon{color:var(--accent);background:#ffffff08;padding:15px;border-radius:18px}.stage-description{font-size:12px;color:#9daaa3;line-height:1.8;display:block;margin-bottom:22px}.summary{padding:17px}.summary-row{display:flex;justify-content:space-between;gap:14px}.order-title{font-size:16px;font-weight:700}.price{font-size:22px;font-weight:800;color:var(--accent);white-space:nowrap}.people{display:flex;justify-content:space-between;gap:12px;color:#a8b5ae;font-size:11px;margin:14px 0}.requirements{display:block;border-top:1px solid #ffffff0e;padding-top:12px;line-height:1.8;font-size:12px;color:#b4bfb9;white-space:pre-wrap}.delivery{display:block;margin-top:12px;color:var(--accent);font-size:12px;line-height:1.8;white-space:pre-wrap}.actions{display:flex;gap:9px;flex-wrap:wrap;margin:16px 0}.actions button{font-size:12px;min-width:100px;flex:1;white-space:nowrap}.refund-panel{border:1px solid #e9b15a40;background:#c1872010;padding:15px;border-radius:14px;margin-top:14px;display:flex;flex-direction:column;gap:10px;font-size:12px;line-height:1.7;color:#d8c7a6}.refund-heading{display:flex;align-items:center;gap:7px;color:#ecc078;font-weight:700}.admin-resolution{display:flex;flex-direction:column;gap:12px;padding:16px;margin:14px 0}.chat-heading{display:flex;justify-content:space-between;align-items:center;margin:28px 0 14px}.chat-sub{display:block;color:#7f9186;font-size:10px;margin-top:6px}.evidence-badge{display:flex;align-items:center;gap:4px;font-size:10px;color:var(--accent)}.chat-panel{border:1px solid #ffffff13;border-radius:16px;overflow:hidden;background:#00000018}.chat-notice{display:block;padding:12px 14px;font-size:10px;line-height:1.7;background:#ffffff04;color:#88968e}.chat-scroll{height:370px;padding:0 12px;box-sizing:border-box}.chat-empty{text-align:center;color:#6c7d73;font-size:12px;padding:60px 0}.message{margin:14px 0;display:flex;flex-direction:column;align-items:flex-start}.message.mine{align-items:flex-end}.message.system{align-items:center}.system-message{color:#75867c;font-size:10px;line-height:1.6;text-align:center;padding:5px 10px}.message-meta{display:flex;align-items:center;gap:12px;font-size:9px;color:#7e8e83;margin-bottom:6px}.role-badge{color:var(--accent);margin-left:5px;font-size:9px}.bubble{max-width:88%;padding:11px 13px;border-radius:2px 12px 12px 12px;background:#ffffff0a;color:#e3ebe6;font-size:12px;line-height:1.7;word-break:break-all;white-space:pre-wrap}.mine .bubble{background:#ffffff13;border-radius:12px 2px 12px 12px}.bubble image{width:180px;max-width:100%;display:block;border-radius:6px}.composer{display:flex;align-items:center;gap:8px;padding:12px 10px;border-top:1px solid #ffffff0c}.composer input{flex:1;min-width:0;height:38px;background:#ffffff06;border-radius:8px;padding:0 9px;font-size:11px}.image-button{background:transparent;color:#a5b5aa;padding:5px;line-height:1;margin:0;flex-shrink:0}.send-button{background:var(--accent);color:#091b11;line-height:34px;font-size:11px;padding:0 12px;border-radius:8px;margin:0}.send-button[disabled]{opacity:.35}.updated{display:block;font-size:9px;color:#617469;text-align:center;margin-top:16px}.chat-bottom{height:5px}.modal-mask{position:fixed;inset:0;background:#000b;z-index:110;display:flex;align-items:center;justify-content:center;padding:22px}.modal{max-width:460px;width:100%;padding:22px;display:flex;flex-direction:column;gap:17px;background:var(--surface)}.modal-title{font-size:21px;font-weight:700}.modal .input{width:100%;height:120px;box-sizing:border-box;font-size:13px;line-height:1.7}.modal-actions{display:flex;gap:10px}.modal-actions button{flex:1;font-size:13px}.payment-summary{display:flex;justify-content:space-between;font-size:18px}.dev-warning{color:#d3b982;font-size:12px}.error-panel{color:#e1ad8b;padding:14px;border:1px solid #b2805944;border-radius:10px}.review-panel{padding:17px;margin-top:16px}.stars{display:flex;gap:10px;margin:14px 0}.stars button{background:transparent;padding:0;line-height:1;margin:0;color:#5f6d65}.stars .lit{color:#f0bd64}.review-input{width:100%;height:75px;margin-bottom:12px;box-sizing:border-box;font-size:12px}.review-result{display:flex;flex-direction:column;gap:8px;padding:15px;margin-top:15px;font-size:12px}
</style>
