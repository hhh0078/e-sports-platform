<!-- 文件作用：统一展示三类押金付款、退款与组织提现的原单进度和恢复入口。 -->
<template>
  <view class="funds-progress">
    <view v-if="payments.length||refunds.length||withdrawals.length" class="funds-progress-heading"><text class="section-title">资金进度</text></view>
    <view v-for="item in payments" :key="item.id" class="funds-progress-row"><view class="funds-progress-copy"><text>{{ paymentPurposeName(item.purpose) }} · ¥{{ money(item.amount_cents) }}</text><text class="funds-progress-note">{{ walletStatus(item.status) }} · {{ time(item.created_at) }}</text></view><view v-if="item.status==='pending_payment'" class="funds-progress-actions"><button class="text-btn" :disabled="busy" @click="emit('resume',item)">继续支付</button><button class="text-btn" :disabled="busy" @click="emit('sync','payment',item.id)">同步</button></view></view>
    <view v-for="item in refunds" :key="item.id" class="funds-progress-row"><view class="funds-progress-copy"><text>原路退还 · ¥{{ money(item.amount_cents) }}</text><text class="funds-progress-note">{{ walletStatus(item.status) }} · {{ time(item.created_at) }}</text></view><button v-if="item.status!=='success'" class="text-btn" :disabled="busy" @click="emit('sync','refund',item.id)">同步</button></view>
    <view v-for="item in withdrawals" :key="item.id" class="funds-progress-row"><view class="funds-progress-copy"><text>佣金提现 · ¥{{ money(item.amount_cents) }}</text><text class="funds-progress-note">{{ walletStatus(item.status) }} · {{ time(item.created_at) }}</text><text v-if="item.failure_reason" class="funds-progress-note">{{ item.failure_reason }}</text></view><button v-if="['pending_review','processing','wait_user_confirm'].includes(item.status)" class="text-btn" :disabled="busy" @click="emit('sync','withdraw',item.id,item.status==='wait_user_confirm')">{{ item.status==='wait_user_confirm'?'确认收款':'同步' }}</button></view>
  </view>
</template>
<script setup lang="ts">
import type { WalletTopup, WalletDepositRefund, WalletWithdrawal } from '../types/wallet'
import { money, time } from '../lib/format'
import { walletStatus } from '../lib/wallet'
import { paymentPurposeName } from '../lib/finance'
withDefaults(defineProps<{payments:WalletTopup[];refunds:WalletDepositRefund[];withdrawals?:WalletWithdrawal[];busy?:boolean}>(),{withdrawals:()=>[],busy:false})
const emit=defineEmits<{resume:[record:WalletTopup];sync:[kind:'payment'|'refund'|'withdraw',id:string,confirm?:boolean]}>()
</script>
<style scoped>
.funds-progress{min-width:0}.funds-progress-heading{margin:24px 0 8px}.funds-progress-row{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:16px 0;border-bottom:1px solid var(--line-subtle);min-width:0}.funds-progress-copy{flex:1;min-width:0;display:flex;flex-direction:column;gap:5px;font-size:14px;line-height:22px;overflow-wrap:anywhere}.funds-progress-note{font-size:12px;color:var(--muted);line-height:20px}.funds-progress-actions{display:flex;flex-direction:column;gap:8px;flex-shrink:0}
</style>
