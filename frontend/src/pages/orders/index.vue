<!-- 文件作用：按生成设计稿呈现双身份订单、服务端状态筛选与四方沟通入口。 -->
<template>
  <AppShell active="orders" immersive>
    <SectionHero eyebrow="订单" title="每一单，都有迹可循" subtitle="进度、沟通与售后，都在这里。" />
    <view class="orders-content">
      <view class="role-tabs"><button v-for="item in roles" :key="item.id" :class="{selected: role === item.id}" @click="chooseRole(item.id)">{{ item.label }}</button></view>
      <scroll-view scroll-x :show-scrollbar="false" class="filters"><view class="filter-row"><button v-for="item in orderFilters" :key="item.id" :class="['filter', {selected: filter === item.id}]" @click="chooseFilter(item.id)">{{ item.label }}</button></view></scroll-view>
      <view v-if="error" class="error-panel"><text>{{ error }}</text><button class="text-btn" @click="load()">重新加载</button></view>
      <view v-else-if="loading" class="loading-line">正在同步订单…</view>
      <view v-else-if="!state.user" class="guest-note" @click="signIn">请登录后查看</view>
      <EmptyState v-else-if="!orders.length" title="还没有这类订单" :detail="role === 'provider' ? '发布服务或去接单大厅，寻找下一次并肩' : '找到合拍的队友，开始下一次并肩'" icon="receipt"><button class="btn-secondary" @click="go(role === 'provider' ? '/pages/worker/index' : '/pages/index/index')">{{ role === 'provider' ? '前往打手工作台' : '去找服务' }}</button></EmptyState>
      <template v-else>
        <view v-for="item in orders" :key="item.id" class="order-card" @click="openOrder(item)">
          <view class="order-top"><text class="order-meta">{{ item.game_name || gameInfo(item.game_id).name || '游戏服务' }} · {{ time(item.created_at) }}</text><text class="order-state" :class="stateTone(item)">{{ orderStatus(item) }}</text></view>
          <view class="order-body"><image v-if="gameInfo(item.game_id).cover_url" :src="assetUrl(gameInfo(item.game_id).cover_url)" mode="aspectFill"/><view v-else class="order-placeholder"><Icon name="game" :size="26"/></view><view class="order-info"><text class="order-title">{{ item.title }}</text><text class="counterparty">{{ item.buyer_id === state.user?.id ? '队友：' + (item.provider_name || '等待接单') : '老板：' + item.buyer_name }}</text></view></view>
          <view class="order-footer"><text class="amount">¥{{ money(item.amount_cents) }}</text><button class="enter" @click.stop="openOrder(item)">{{ orderActionLabel(item, state.user?.id) }} <Icon name="chevron-right" :size="16"/></button></view>
        </view>
      </template>
      <view v-if="moreError" class="more-error">{{ moreError }}</view>
      <button v-if="hasMore && !error && !loading" class="btn-secondary load-more" :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '正在加载…' : moreError ? '重试加载' : '加载更多订单' }}</button>
      <view v-if="state.user && !loading && !error" class="evidence-note"><Icon name="info" :size="16"/><text>沟通与交付记录，随订单保存。</text></view>
    </view>
  </AppShell>
</template>
<script setup>
import { ref, computed, onBeforeUnmount } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import SectionHero from '../../components/SectionHero.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { api, token } from '../../lib/api'
import { state, init, clearSession } from '../../lib/store'
import { money, time, statusLabel, go, assetUrl } from '../../lib/format'
import { orderFilters, parseOrderQuery, orderListPath, orderActionLabel } from '../../lib/order-list'
const PAGE_SIZE = 20
const orders = ref([]), loading = ref(true), loadingMore = ref(false), error = ref(''), moreError = ref(''), hasMore = ref(false)
const role = ref('buyer'), filter = ref('all'), showAllRole = ref(false)
let loadVersion = 0
const roles = computed(() => [...(showAllRole.value ? [{ id: 'all', label: '全部身份' }] : []), { id: 'buyer', label: '我下的单' }, { id: 'provider', label: '我接的单' }])
/** 在服务端按身份和阶段筛选后分页，切换时清空旧数据并忽略迟到响应。 */
async function load(append = false) {
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const version = ++loadVersion, selectedRole = role.value, selectedFilter = filter.value
  if (adding) loadingMore.value = true
  else { loading.value = true; orders.value = []; hasMore.value = false; error.value = '' }
  moreError.value = ''
  let requestToken
  try {
    await init()
    requestToken = token()
    const items = state.user ? await api('/orders?role=' + selectedRole + '&status=' + selectedFilter + '&limit=' + PAGE_SIZE + '&offset=' + (adding ? orders.value.length : 0)) || [] : []
    if (version !== loadVersion || requestToken !== token()) return
    orders.value = adding ? [...orders.value, ...items] : items
    hasMore.value = items.length === PAGE_SIZE
  } catch (err) { if (version === loadVersion && (requestToken === undefined || requestToken === token())) { if (err.status === 401) { clearSession(); orders.value = []; hasMore.value = false } else if (adding) moreError.value = err.message; else error.value = err.message } }
  finally { if (version === loadVersion) { loading.value = false; loadingMore.value = false } }
}
/** 切换买卖视角，不能保留上个视角的列表。 */
function chooseRole(value) { if (role.value !== value) { role.value = value; load() } }
/** 切换履约阶段，并从第一页重新查询。 */
function chooseFilter(value) { if (filter.value !== value) { filter.value = value; load() } }
/** 保留当前筛选条件，登录后回到同一个订单入口。 */
function signIn() { go('/pages/login/index?next=' + encodeURIComponent(orderListPath(role.value, filter.value))) }
/** 进入具体订单，所有付款、聊天和售后仍由详情页按权限处理。 */
function openOrder(order) { go('/pages/order/index?id=' + encodeURIComponent(order.id)) }
/** 取得订单游戏资料，停用或缺失游戏采用文字与本地图兜底。 */
function gameInfo(id) { return state.games.find(item => item.id === id) || {} }
/** 退款阶段优先于履约阶段，明确展示拒绝和平台介入。 */
function orderStatus(order) { return { requested: '退款协商中', rejected: '退款被拒绝', escalated: '平台介入中', processing: '退款处理中' }[order.refund_status] || statusLabel(order.status) }
/** 将售后、结束和正在履约状态分别映射为警示、弱化与主题色。 */
function stateTone(order) { if (['requested','rejected','escalated','processing'].includes(order.refund_status)) return 'warning'; return ['refunded','cancelled','completed'].includes(order.status) ? 'settled' : '' }
onLoad(options => { const query = parseOrderQuery(options); role.value = query.role; filter.value = query.filter; showAllRole.value = query.role === 'all' })
onShow(() => load())
onBeforeUnmount(() => { loadVersion++ })
</script>
<style scoped>
.guest-note{min-height:180px;display:flex;align-items:center;justify-content:center;color:#89968f;font-size:12px;cursor:pointer}.orders-content{padding:0 14px 20px}.role-tabs{display:flex;background:#ffffff03;border:1px solid #ffffff16;padding:3px;border-radius:12px;margin:0 0 12px}.role-tabs button{flex:1;font-size:14px;background:transparent;color:#a7b1b4;border:0;border-radius:9px;line-height:38px;padding:0 4px}.role-tabs .selected{background:var(--accent);color:#06170f;font-weight:750}.filters{width:100%;margin-bottom:13px;white-space:nowrap}.filter-row{display:flex;gap:6px;padding-right:14px}.filter{flex-shrink:0;background:transparent;color:#b7bec0;font-size:12px;line-height:30px;padding:0 10px;border:1px solid #ffffff26;border-radius:7px;margin:0}.filter.selected{color:var(--accent);border-color:var(--accent);font-weight:700}.order-card{padding:12px;margin-bottom:12px;border:1px solid #ffffff17;border-radius:12px;background:#ffffff02}.order-top,.order-footer{display:flex;justify-content:space-between;gap:8px;align-items:center}.order-meta{font-size:12px;color:#aab5ba;min-width:0}.order-state{color:var(--accent);border:1px solid var(--accent);border-radius:8px;line-height:20px;padding:0 9px;font-size:11px;font-weight:600;white-space:nowrap}.order-state.warning{color:#f2c66e;border-color:#dca84a}.order-state.settled{color:#b1b7b9;border-color:#73817d;background:#ffffff04}.order-body{display:flex;align-items:center;gap:12px;margin:5px 0}.order-placeholder{display:flex;align-items:center;justify-content:center;background:#ffffff08;color:#91a69c}.order-body image,.order-placeholder{width:86px;height:64px;flex-shrink:0;border-radius:8px}.order-info{display:flex;flex-direction:column;gap:5px;min-width:0}.order-title{font-size:15px;line-height:1.45;font-weight:750;display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden}.counterparty{font-size:12px;color:#aab5ba;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}.order-footer{border-top:1px solid #ffffff12;padding-top:5px}.amount{font-size:23px;line-height:30px;font-weight:750;color:#fff}.enter{display:flex;gap:5px;align-items:center;justify-content:center;color:var(--accent);font-size:13px;font-weight:700;line-height:28px;padding:0 12px;border:1px solid var(--accent);border-radius:8px;background:transparent}.error-panel{padding:15px;border:1px solid #ed856644;color:#edb192;border-radius:12px;margin-bottom:15px;font-size:13px}.error-panel button{margin-top:8px}.more-error{color:#f2c66e;font-size:12px;margin:10px 0}.load-more{width:100%;min-height:40px;font-size:12px}.evidence-note{display:flex;justify-content:center;align-items:center;gap:6px;font-size:11px;color:#9ca7ac;margin-top:20px}@media(max-width:350px){.guest-note{min-height:180px;display:flex;align-items:center;justify-content:center;color:#89968f;font-size:12px;cursor:pointer}.orders-content{padding-left:12px;padding-right:12px}.order-card{padding:11px}.order-meta{font-size:10px}.order-state{font-size:10px;padding:0 6px}.order-placeholder{display:flex;align-items:center;justify-content:center;background:#ffffff08;color:#91a69c}.order-body image,.order-placeholder{width:70px;height:58px}.order-title{font-size:14px}.enter{font-size:11px;padding:0 8px}.filter{padding:0 10px}}
</style>
