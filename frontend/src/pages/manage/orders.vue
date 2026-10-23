<!-- 文件作用：独立查询订单与售后记录，复用既有订单详情处理聊天室、证据和平台裁定。 -->
<template>
  <AppShell title="订单与售后" back>
    <ManageList v-model:query="query" placeholder="搜索订单号、标题或用户" :loading="loading" :error="error" :empty="!orders.length" :has-more="hasMore" :count="orders.length" @search="loadData()" @more="loadData(true)" @prepare="showPage">
      <template #filters><view class="order-filters"><button :class="['order-filter', { selected: !refundOnly }]" @click="chooseFilter(false)">全部订单</button><button :class="['order-filter', { selected: refundOnly }]" @click="chooseFilter(true)">退款与争议</button></view></template>
      <view v-for="order in orders" :key="order.id" class="order-row" @click="go('/pages/orders/detail?id=' + encodeURIComponent(order.id))"><view class="order-heading"><text class="order-title">{{ order.title }}</text><text class="order-amount">¥{{ money(order.amount_cents) }}</text></view><text class="order-number">订单号：{{ order.order_no }}</text><view class="order-parties"><text class="order-party">{{ order.buyer_name || '买方' }} → {{ order.provider_name || '等待接单' }}</text><Icon name="arrow" :size="16"/></view><view class="order-footer"><text class="order-status">{{ statusLabel(order.status) }}</text><text v-if="order.refund_status && order.refund_status !== 'none'" class="refund-status">{{ refundName(order.refund_status) }}</text><text v-else class="order-link">查看订单</text></view></view>
    </ManageList>
  </AppShell>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import Icon from '../../components/Icon.vue'
import { useManageResource } from '../../lib/manage-resource'
import { go, money, statusLabel } from '../../lib/format'
const refundOnly = ref(false)
const { items: orders, query, loading, error, hasMore, loadData, showPage } = useManageResource('orders', { params: () => refundOnly.value ? { refund: 1 } : {} })
/** 只使用后端支持的全部或退款争议查询，切换时重新读取第一页。 */
function chooseFilter(value: boolean): void { if (refundOnly.value === value) return; refundOnly.value = value; orders.value = []; hasMore.value = false; void loadData() }
/** 显示资金通道的实际售后状态，不把处理中当作退款到账。 */
function refundName(value: string): string { return ({ requested: '等待卖方处理', rejected: '卖方已拒绝', escalated: '申请平台介入', approved: '退款完成', processing: '退款通道处理中' } as Record<string, string>)[value] || value }
onLoad((options: Record<string, string | undefined> = {}) => { refundOnly.value = options.refund === '1' })
</script>
<style scoped>
.order-filters{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));min-width:0;border-bottom:1px solid var(--line-subtle);margin-bottom:7.6923rpx;gap:38.4615rpx}.order-filter{display:flex;align-items:center;justify-content:center;min-height:76.9231rpx;border:0;border-bottom:3.8462rpx solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:26.9231rpx}.order-filter.selected{border-bottom-color:var(--accent);color:var(--accent)}.order-row{display:flex;flex-direction:column;gap:15.3846rpx;min-width:0;padding:46.1538rpx 0;border-bottom:1px solid var(--line-subtle)}.order-heading{display:flex;align-items:flex-start;justify-content:space-between;gap:30.7692rpx;min-width:0}.order-title{min-width:0;flex:1;font-size:30.7692rpx;line-height:46.1538rpx;font-weight:650;overflow-wrap:anywhere}.order-amount{font-size:30.7692rpx;line-height:46.1538rpx;font-weight:650;flex-shrink:0;white-space:nowrap}.order-number{display:block;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.order-parties{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;color:var(--muted)}.order-party{flex:1;min-width:0;font-size:26.9231rpx;line-height:42.3077rpx;color:var(--text);overflow-wrap:anywhere}.order-footer{display:flex;align-items:center;flex-wrap:wrap;gap:23.0769rpx;min-width:0}.order-status{font-size:25rpx;color:var(--accent)}.refund-status{font-size:23.0769rpx;color:#d5ba8f;overflow-wrap:anywhere}.order-link{font-size:23.0769rpx;color:var(--muted)}
</style>
