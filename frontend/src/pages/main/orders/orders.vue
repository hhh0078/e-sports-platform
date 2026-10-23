<!-- 文件作用：按订单设计图展示双身份筛选与订单卡片，常驻主视图保留状态。 -->
<template>
  <view class="orders-content">
    <view class="role-safe-row"><scroll-view scroll-x :show-scrollbar="false" class="role-scroll"><view class="role-tabs"><button class="role-tab" v-for="item in roles" :key="item.id" :class="{selected:role===item.id}" @click="chooseRole(item.id)">{{ item.label }}</button></view></scroll-view></view>
    <scroll-view scroll-x :show-scrollbar="false" class="filters"><view class="filter-row"><button v-for="item in orderFilters" :key="item.id" :class="['filter',{selected:filter===item.id}]" @click="chooseFilter(item.id)">{{ item.label }}</button></view></scroll-view>
    <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load()">重新加载</button></view>
    <view v-else-if="loading&&!hasLoaded&&!orders.length" class="loading-line">正在同步订单…</view>
    <view v-else-if="!state.user||!orders.length" class="orders-empty"><EmptyState title="还没有订单" :detail="role==='provider'?'认真接单，每一次并肩都有记录。':'下单找队友，进度与沟通都在这里。'" illustration="/static/illustrations/empty-orders-light.png" compact><button v-if="!state.user" class="guest-note" @click="signIn">请登录后查看</button><button class="discover-entry" @click="go('/pages/main/index?tab='+(role==='provider'?'worker':'player'))">{{ role==='provider'?'去接单大厅':'去发现服务' }}<Icon name="arrow" :size="14"/></button></EmptyState></view>
    <view v-for="item in orders" v-else :key="item.id" class="order-card" @click="openOrder(item)">
      <view class="order-top"><view class="order-game"><image class="order-game-icon" v-if="gameInfo(item.game_id)?.icon_url" :src="assetUrl(gameInfo(item.game_id)?.icon_url)" mode="aspectFill"/><text class="order-game-name">{{ item.game_name||gameInfo(item.game_id)?.name||'游戏服务' }}</text><Icon name="arrow" :size="12"/></view><text :class="['order-state',stateTone(item)]">{{ orderStatus(item) }}</text></view>
      <view class="order-body"><image v-if="gameInfo(item.game_id)?.cover_url" class="order-cover" :src="assetUrl(gameInfo(item.game_id)?.cover_url)" mode="aspectFill"/><view v-else class="order-cover order-placeholder"><Icon name="game" :size="24"/></view><view class="order-info"><text class="order-title">{{ item.title }}</text><text class="counterparty">{{ item.buyer_id===state.user?.id?'队友：'+(item.provider_name||'等待接单'):'老板：'+item.buyer_name }}</text><text class="order-time">{{ time(item.created_at) }}</text></view></view>
      <view class="order-footer"><view class="paid"><text class="paid-label">实付款</text><text class="paid-amount">¥{{ money(item.amount_cents) }}</text></view><button class="order-detail-entry" @click.stop="openOrder(item)">{{ orderActionLabel(item,state.user?.id) }}<Icon name="arrow" :size="12"/></button></view>
    </view>
    <ListLoadState v-if="!error&&(orders.length||moreError)" :loading="loadingMore" :has-more="hasMore" :error="moreError" @retry="retryList"/>
  </view>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import EmptyState from '../../../components/EmptyState.vue'
import ListLoadState from '../../../components/ListLoadState.vue'
import { appendUniqueRows } from '../../../lib/list-page'
import Icon from '../../../components/Icon.vue'
import { token, errorMessage, errorStatus } from '../../../api/client'
import { listOrders } from '../../../api/orders.api'
import { state, init, clearSession, sessionRevision } from '../../../lib/store'
import { money, time, statusLabel, go, assetUrl } from '../../../lib/format'
import { orderFilters, parseOrderQuery, orderListPath, orderActionLabel } from '../../../lib/order-list'
import { mainNavigation, useMainVisibility, refreshMainList } from '../../../lib/navigation'
import type { Order } from '../../../types/models'
const props = defineProps<{ active: boolean }>()
/** 查询上下文变化使父容器撤销旧刷新显示，不等待被替代的网络请求。 */
const refreshKey=ref(0)
const PAGE_SIZE = 20
const orders = ref<Order[]>([]), loading = ref(false), loadingMore = ref(false), error = ref(''), moreError = ref(''), hasMore = ref(false)
/** 已成功显示的空态在重新激活刷新时保留，避免用短加载行截断原生滚动位置。 */
const hasLoaded = ref(false)
const role = ref('buyer'), filter = ref('all'), showAllRole = ref(false)
let loadVersion = 0, appliedRevision = -1, visible = false, nextOffset = 0, failedAppend = false
const roles = computed(() => [...(showAllRole.value ? [{ id: 'all', label: '全部身份' }] : []), { id: 'buyer', label: '我下的单' }, { id: 'provider', label: '我接的单' }])
/** 服务端筛选后分页，过期身份或请求不得更新界面。 */
async function load(append = false, preserve = false): Promise<void> {
  if (!visible) return
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const version = ++loadVersion, selectedRole = role.value, selectedFilter = filter.value
  if (adding) loadingMore.value = true
  else { loading.value = true; if (!preserve) { refreshKey.value++; nextOffset = 0; hasMore.value = false; hasLoaded.value = false } error.value = '' }
  moreError.value = ''; let requestToken: string | undefined, revision = sessionRevision()
  const current = () => visible && version === loadVersion && revision === sessionRevision() && (requestToken === undefined || requestToken === token())
  try {
    await init(); if (!current()) return; requestToken = token(); revision = sessionRevision()
    if (!state.user) { orders.value = []; nextOffset = 0; hasMore.value = false; hasLoaded.value = true; return }
    /** 按当前买卖身份和阶段读取指定范围，保持筛选快照。 */
    const read = (limit: number, offset: number) => listOrders({ role: selectedRole, status: selectedFilter, limit, offset })
    if (preserve && orders.value.length) { const result = await refreshMainList(Math.max(nextOffset, orders.value.length), PAGE_SIZE, read, current); if (result) { nextOffset = result.items.length; orders.value = appendUniqueRows([], result.items); hasMore.value = result.hasMore } }
    else { const offset = adding ? nextOffset : 0, items = await read(PAGE_SIZE, offset); if (!current()) return; nextOffset = offset + items.length; orders.value = appendUniqueRows(adding ? orders.value : [], items); hasMore.value = items.length === PAGE_SIZE }
    if (current()) hasLoaded.value = true
  }
  catch (err) { if (current()) { if (errorStatus(err) === 401) { clearSession(); orders.value = []; nextOffset = 0; hasMore.value = false } else if (adding || (preserve && (hasLoaded.value || orders.value.length))) {failedAppend=adding;moreError.value = errorMessage(err);} else error.value = errorMessage(err) } }
  finally { if (version === loadVersion) { loading.value = false; loadingMore.value = false } }
}
/** 切换本人买卖视角。 */
function chooseRole(value: string): void { if (role.value !== value) { role.value = value; orders.value = []; nextOffset = 0; void load() } }
/** 从第一页重新读取新的订单阶段。 */
function chooseFilter(value: string): void { if (filter.value !== value) { filter.value = value; orders.value = []; nextOffset = 0; void load() } }
/** 登录后继续原筛选入口。 */
function signIn(): void { go('/pages/account/login?next=' + encodeURIComponent(orderListPath(role.value, filter.value))) }
/** 进入订单履约、聊天和售后详情。 */
function openOrder(order: Order): void { go('/pages/orders/detail?id=' + encodeURIComponent(order.id)) }
/** 取得订单游戏素材，缺失配置时由视图展示占位。 */
function gameInfo(id: string) { return state.games.find(item => item.id === id) }
/** 优先展示售后阶段，再显示履约状态。 */
function orderStatus(order: Order): string { const labels: Record<string, string> = { requested: '退款协商中', rejected: '退款被拒绝', escalated: '平台介入中', processing: '退款处理中' }; return labels[order.refund_status] || statusLabel(order.status) }
/** 将售后、已结束与进行中映射为统一状态颜色。 */
function stateTone(order: Order): string { return ['requested', 'rejected', 'escalated', 'processing'].includes(order.refund_status) ? 'warning' : ['refunded', 'cancelled', 'completed'].includes(order.status) ? 'settled' : '' }
/** 主页面深链带筛选时应用新条件，普通标签切换保留上次条件。 */
function applyQuery(): boolean { if (appliedRevision === mainNavigation.revision) return false; appliedRevision = mainNavigation.revision; if (!mainNavigation.query.role && !mainNavigation.query.status) return false; const query = parseOrderQuery(mainNavigation.query), changed = role.value !== query.role || filter.value !== query.filter; role.value = query.role; filter.value = query.filter; showAllRole.value = query.role === 'all'; return changed }
/** 切到订单视图后获取当前列表。 */
function showPage(): void { visible = true; if (applyQuery()) { orders.value = []; nextOffset = 0; hasMore.value = false; hasLoaded.value = false } void load(false, true) }
/** 离开当前视图后禁止迟到请求提交。 */
function hidePage(): void { visible = false; loadVersion++; loading.value = false; loadingMore.value = false }
watch(() => mainNavigation.revision, () => { if (visible && props.active && applyQuery()) { orders.value = []; nextOffset = 0; hasMore.value = false; hasLoaded.value = false; void load() } })
useMainVisibility(() => props.active, showPage, hidePage)
/** 即使主页面隐藏，退出、换号及同账号会话轮换也立即移除私有列表并作废请求。 */
watch([() => state.user?.id, sessionRevision], () => { loadVersion++; orders.value = []; nextOffset = 0; hasMore.value = false; hasLoaded.value = false; error.value = ''; moreError.value = ''; loading.value = false; loadingMore.value = false; if (visible) void load() }, { flush: 'sync' })
/** 下拉按当前买卖身份与阶段重读已加载范围，不清空原列表。 */
async function refresh():Promise<void>{if(!visible||loading.value||loadingMore.value)return;await load(false,true)}
/** 触底继续同一筛选的下一页，列表内部防重复和越界。 */
async function loadMore():Promise<void>{if(!visible||loading.value||loadingMore.value)return;if(moreError.value&&!failedAppend)await load(false,true);else await load(true)}
/** 失败后恢复当前分页或重新同步已加载范围。 */
async function retryList():Promise<void>{await loadMore()}
defineExpose({refresh,loadMore,refreshKey})
</script>
<style scoped>
.orders-content{display:flex;flex-direction:column;gap:23.0769rpx;padding:15.3846rpx 30.7692rpx 46.1538rpx;min-width:0}.role-safe-row{min-height:44px;padding-right:var(--capsule-right-inset,100px);min-width:0}.role-scroll{width:100%;white-space:nowrap}.role-tabs{display:inline-flex;align-items:center;gap:46.1538rpx;min-width:100%}.role-tab{display:flex;flex-shrink:0;align-items:center;justify-content:center;height:84.6154rpx;padding:0 7.6923rpx;border:0;border-bottom:3.8462rpx solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:30.7692rpx;line-height:46.1538rpx;white-space:nowrap}.role-tab.selected{color:var(--accent);border-bottom-color:var(--accent);font-weight:650}.filters{width:100%;white-space:nowrap}.filter-row{display:inline-flex;gap:15.3846rpx;min-width:100%;padding:3.8462rpx 0 15.3846rpx}.filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-width:119.2308rpx;min-height:57.6923rpx;padding:0 23.0769rpx;border:1px solid var(--line);border-radius:34.6154rpx;background:#ffffff02;font-size:21.1538rpx;color:var(--muted)}.filter.selected{color:var(--accent);border-color:#83e7b777;background:#83e7b70a}.orders-empty{display:flex;align-items:center;justify-content:center;min-height:calc(100vh - var(--status-bar-height,24px) - 423.0769rpx);padding:76.9231rpx 0 92.3077rpx}.guest-note{display:flex;align-items:center;justify-content:center;min-height:53.8462rpx;padding:0;background:transparent;color:var(--muted);font-size:21.1538rpx}.discover-entry{display:flex;align-items:center;justify-content:center;gap:13.4615rpx;min-height:53.8462rpx;margin-top:15.3846rpx;padding:0 0 5.7692rpx;border:0;border-bottom:1px solid #83e7b744;border-radius:0;background:transparent;color:var(--accent);font-size:23.0769rpx}.order-card{padding:30.7692rpx 0;border:0;border-bottom:1px solid var(--line-subtle);border-radius:0;background:transparent}.order-top,.order-game,.order-footer{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.order-game{justify-content:flex-start;flex:1;font-size:23.0769rpx;font-weight:650;min-width:0}.order-game-icon{width:46.1538rpx;height:46.1538rpx;border-radius:9.6154rpx}.order-game-name{overflow:hidden;white-space:nowrap;text-overflow:ellipsis;min-width:0}.order-state{flex-shrink:0;font-size:19.2308rpx;color:var(--accent);white-space:nowrap}.order-state.warning{color:#ebc67a}.order-state.settled{color:var(--muted)}.order-body{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0;margin:23.0769rpx 0}.order-cover{width:38%;height:161.5385rpx;border-radius:13.4615rpx;flex-shrink:0}.order-placeholder{display:flex;align-items:center;justify-content:center;background:#ffffff06;color:var(--muted)}.order-info{display:flex;flex:1;flex-direction:column;gap:15.3846rpx;min-width:0}.order-title{font-size:26.9231rpx;font-weight:650;line-height:38.4615rpx;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.counterparty,.order-time{font-size:19.2308rpx;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.order-footer{padding-top:7.6923rpx}.paid{display:flex;align-items:baseline;gap:11.5385rpx;min-width:0}.paid-label{font-size:19.2308rpx;color:var(--muted)}.paid-amount{font-size:38.4615rpx;font-weight:700}.order-detail-entry{display:flex;align-items:center;justify-content:center;gap:9.6154rpx;min-height:53.8462rpx;padding:0;background:transparent;color:var(--accent);font-size:21.1538rpx;white-space:nowrap}.load-more{display:flex;align-items:center;justify-content:center;gap:9.6154rpx;width:100%;min-height:69.2308rpx;background:transparent;color:var(--muted);font-size:23.0769rpx}
@media(max-width:350px){.role-tabs{gap:30.7692rpx}.role-tab{font-size:26.9231rpx;padding:0 3.8462rpx}.filter-row{gap:11.5385rpx}.filter{min-width:111.5385rpx;padding:0 19.2308rpx}.orders-empty{padding:46.1538rpx 0 76.9231rpx}}
</style>
