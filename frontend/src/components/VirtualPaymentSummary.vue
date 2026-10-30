<!-- 文件作用：在用户确认前展示服务端报价、现有金币、购买差额及费用，状态未知时禁止推断到账。 -->
<template>
  <view class="virtual-summary"><text v-if="loading" class="virtual-note">正在核对金币与手续费…</text><text v-else-if="error" class="virtual-error">{{error}}</text><template v-else-if="quote"><view class="virtual-line"><text>本次服务</text><text>{{coins(quote.service_cents)}} 金币</text></view><view class="virtual-line"><text>渠道手续费 · {{quote.fee_bearer==='buyer'?'买家承担':'平台承担'}}</text><text>{{coins(quote.fee_cents)}} 金币</text></view><view class="virtual-line"><text>金币余额</text><text>{{coins(quote.coin_balance_cents)}} 金币</text></view><view class="virtual-line"><text>本次购买</text><text>{{coins(quote.topup_cents)}} 金币</text></view><view class="virtual-line virtual-total"><text>现金实付</text><text>¥{{coins(quote.topup_cents)}}</text></view><text class="virtual-note">1金币=1元。官方确认到账后支付本订单，金币不可提现。订单退款退回金币；iOS现金退款由Apple处理。</text></template></view>
</template>
<script setup lang="ts">
import type { VirtualQuote } from '../types/commerce'
defineProps<{quote:VirtualQuote|null;loading?:boolean;error?:string}>()
/** 整数分金币转换为显示面额，不在客户端增减或四舍五入资金。 */
function coins(cents:number):string{return(cents/100).toFixed(2)}
</script>
<style scoped>
.virtual-summary{min-width:0;margin:30.7692rpx 0}.virtual-line{display:flex;justify-content:space-between;gap:23.0769rpx;padding:11.5385rpx 0;color:var(--muted);font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.virtual-total{margin-top:15.3846rpx;border-top:1px solid var(--line);padding-top:23.0769rpx;color:var(--accent);font-size:32.6923rpx;font-weight:650}.virtual-note,.virtual-error{display:block;margin-top:23.0769rpx;font-size:23.0769rpx;line-height:38.4615rpx;overflow-wrap:anywhere}.virtual-note{color:var(--muted)}.virtual-error{color:#efb299}
</style>
