<!-- 文件作用：统一显示订单冻结的抽成与净收入，所有金额取自服务端快照，旧账不猜测。 -->
<template>
  <view class="order-settlement">
    <text class="settlement-title">收入明细</text>
    <template v-if="order.settlement?.version">
      <view class="settlement-row"><text>订单实付金额</text><text>¥{{ money(order.amount_cents) }}</text></view>
      <view class="settlement-row"><text>平台服务费 · {{ formatRate(order.settlement.platform_rate_bps) }}</text><text>− ¥{{ money(order.settlement.platform_fee_cents) }}</text></view>
      <view v-if="order.settlement.club_id" class="settlement-row"><text>俱乐部佣金 · {{ formatRate(order.settlement.club_rate_bps) }}</text><text>− ¥{{ money(order.settlement.club_fee_cents) }}</text></view>
      <view class="settlement-row settlement-net"><text>{{ order.payment_status!=='paid'?'预计净收入':'打手净收入' }}</text><text>¥{{ money(order.settlement.provider_income_cents) }}</text></view>
      <text class="settlement-note">{{ description }}</text>
      <text class="settlement-note">按本订单快照计费，费率修改不影响旧订单。</text>
    </template>
    <text v-else class="settlement-note">该历史订单缺少完整分账快照，需平台核账后确认收入。</text>
  </view>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { Order } from '../types/models'
import { money } from '../lib/format'
import { formatRate } from '../lib/finance'
const props=defineProps<{order:Order}>()
const description=computed(()=>props.order.status==='refunded'?'本单已退款，原分账已撤回，不计入可提现收入。':props.order.status==='cancelled'?'订单已取消，不产生可提现收入。':props.order.payment_status!=='paid'?'订单尚未完成付款，以上为预计分账。':props.order.refund_status&&['requested','escalated','processing'].includes(props.order.refund_status)?'售后处理中，收入暂不释放。':props.order.status==='completed'?'已完成，满7天售后期且无争议后转为可提现收入。':'履约中的收入暂由平台托管，完成后进入7天售后期。')
</script>
<style scoped>
.order-settlement{padding:20px 0;border-bottom:1px solid var(--line-subtle);min-width:0}.settlement-title{display:block;margin-bottom:12px;font-size:16px;line-height:24px;font-weight:650}.settlement-row{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:9px 0;font-size:14px;color:var(--muted);min-width:0;overflow-wrap:anywhere}.settlement-net{margin-top:6px;padding-top:16px;border-top:1px solid var(--line-subtle);font-size:17px;font-weight:650;color:var(--accent)}.settlement-note{display:block;margin-top:10px;font-size:12px;line-height:21px;color:var(--muted);overflow-wrap:anywhere}
</style>
