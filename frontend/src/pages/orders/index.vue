<!-- 文件作用：按买方和打手身份查看订单、筛选履约阶段并进入四方聊天室。 -->
<template>
  <AppShell title="我的订单" subtitle="每一次并肩，都有迹可循" active="orders">
    <view class="orders-content">
      <view class="role-tabs"><button v-for="item in roles" :key="item.id" :class="{selected: role === item.id}" @click="chooseRole(item.id)">{{ item.label }}</button></view>
      <scroll-view scroll-x class="filters"><view class="filter-row"><button v-for="item in filters" :key="item.id" :class="['filter', {selected: filter === item.id}]" @click="filter = item.id">{{ item.label }}</button></view></scroll-view>
      <view v-if="error" class="error-panel" @click="load()">{{ error }} · 点击重试</view>
      <view v-if="loading" class="muted loading">正在同步订单…</view>
      <EmptyState v-else-if="!state.user" title="登录后查看订单" detail="下单记录与订单沟通会一直保存在这里" icon="receipt"><button class="btn" @click="go('/pages/login/index')">登录账号</button></EmptyState>
      <EmptyState v-else-if="!visible.length" title="还没有这类订单" detail="找到合拍的队友，开始下一次并肩" icon="receipt"><button class="btn-secondary" @click="go('/pages/index/index')">去找服务</button></EmptyState>
      <view v-for="item in visible" :key="item.id" class="order-card panel" @click="go('/pages/order/index?id=' + item.id)">
        <view class="order-top"><text class="muted">{{ gameName(item.game_id) }} · {{ time(item.created_at) }}</text><text class="order-state">{{ item.refund_status && !['none','approved'].includes(item.refund_status) ? refundLabel(item.refund_status) : statusLabel(item.status) }}</text></view>
        <view class="order-body"><image :src="assetUrl(gameCover(item.game_id))" mode="aspectFill"/><view class="order-info"><text class="order-title">{{ item.title }}</text><text class="muted">{{ item.buyer_id === state.user?.id ? '队友：' + (item.provider_name || '等待接单') : '老板：' + item.buyer_name }}</text><text class="order-number">{{ item.order_no }}</text></view></view>
        <view class="order-footer"><text class="amount">¥{{ money(item.amount_cents) }}</text><view class="enter">{{ item.status === 'pending_payment' ? '继续支付' : '详情与聊天室' }} <Icon name="chevron-right" :size="14"/></view></view>
      </view>
      <button v-if="hasMore" class="btn-secondary" :disabled="loading" @click="load(true)">加载更多订单</button>
    </view>
  </AppShell>
</template>
<script setup>
/** 文件作用：订单列表加载、状态过滤及身份切换逻辑。 */
import { ref, computed } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { api } from '../../lib/api'
import { state, init } from '../../lib/store'
import { money, time, statusLabel, go, assetUrl } from '../../lib/format'
const hasMore = ref(false)
let loadVersion = 0
const orders = ref([]), loading = ref(false), error = ref(''), role = ref('buyer'), filter = ref('all')
const roles = [{ id: 'buyer', label: '我下的单' }, { id: 'provider', label: '我接的单' }]
const filters = [{ id: 'all', label: '全部' }, { id: 'pending_payment', label: '待支付' }, { id: 'working', label: '进行中' }, { id: 'delivered', label: '待验收' }, { id: 'completed', label: '已完成' }, { id: 'refund', label: '退款 / 售后' }]
const visible = computed(() => orders.value.filter(order => filter.value === 'all' || (filter.value === 'working' ? ['open','accepted','in_progress'].includes(order.status) : filter.value === 'refund' ? order.status === 'refunded' || (order.refund_status && order.refund_status !== 'none') : order.status === filter.value)))
/** 加载当前身份有权访问的订单，失败时保留页面并允许重试。 */
async function load(append = false) { const version = ++loadVersion; loading.value = true; error.value = ''; try { await init(); const items = state.user ? await api('/orders?role=' + role.value + '&limit=100&offset=' + (append ? orders.value.length : 0)) : []; if (version !== loadVersion) return; orders.value = append ? [...orders.value, ...items] : items; hasMore.value = items.length === 100 } catch (e) { if(version === loadVersion) error.value = e.message } finally { if(version === loadVersion) loading.value = false } }
/** 切换买家与打手视角后重新请求，避免权限混用。 */
function chooseRole(value) { role.value = value; load() }
/** 获取游戏展示名称。 */
function gameName(id) { return state.games.find(game => game.id === id)?.name || '游戏服务' }
/** 获取订单所属游戏封面。 */
function gameCover(id) { return state.games.find(game => game.id === id)?.cover_url || '/static/covers/delta-1.jpg' }
/** 展示退款状态，避免将卖方拒绝误读成服务审核失败。 */
function refundLabel(value) { return { requested:'退款协商中', rejected:'退款被拒绝', escalated:'平台介入中', processing:'退款处理中' }[value] || statusLabel(value) }
onShow(() => load())
</script>
<style scoped>
.orders-content{padding:4px 0 24px}.role-tabs{display:flex;background:var(--surface);padding:4px;border-radius:14px;margin:10px 0 18px}.role-tabs button{flex:1;font-size:14px;background:transparent;color:#8a9690;border:0;border-radius:10px;line-height:42px}.role-tabs .selected{background:var(--accent);color:#07140e;font-weight:700}.filters{width:100%;margin-bottom:20px;white-space:nowrap}.filter-row{display:flex;gap:9px}.filter{background:transparent;color:#9ca8a1;font-size:12px;line-height:32px;padding:0 12px;border:1px solid #ffffff15;border-radius:8px;margin:0}.filter.selected{color:var(--accent);border-color:var(--accent)}.order-card{padding:16px;margin-bottom:14px}.order-top,.order-footer{display:flex;justify-content:space-between;gap:8px;align-items:center;font-size:11px}.order-state{color:var(--accent);white-space:nowrap}.order-body{display:flex;gap:12px;margin:16px 0}.order-body image{width:66px;height:66px;flex-shrink:0;border-radius:9px}.order-info{display:flex;flex-direction:column;gap:6px;min-width:0;font-size:12px}.order-title{font-size:15px;font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.order-number{font-size:10px;color:#68756e}.order-footer{border-top:1px solid #ffffff0c;padding-top:12px}.amount{font-size:22px;font-weight:750}.enter{display:flex;gap:5px;align-items:center;color:var(--accent);font-size:12px}.loading{text-align:center;padding:50px 0}.error-panel{padding:15px;border:1px solid #ed856644;color:#edb192;border-radius:12px;margin-bottom:15px;font-size:13px}
</style>
