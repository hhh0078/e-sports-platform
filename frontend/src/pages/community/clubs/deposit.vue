<!-- 文件作用：用户查看和缴退自己在指定俱乐部的成员押金，展示平台托管及原路退还规则。 -->
<template>
  <AppShell title="成员押金" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <view v-if="loading&&!club" class="loading-line">正在读取成员押金…</view>
    <template v-if="club">
      <view class="member-club"><image v-if="club.avatar_url" class="member-club-avatar" :src="assetUrl(club.avatar_url)" mode="aspectFill"/><Icon v-else name="club" :size="32"/><view class="member-club-copy"><text class="section-title">{{ club.name }}</text><text class="subtle-note">成员俱乐部押金 · 平台独立托管</text></view></view>
      <view class="member-balance"><text class="subtle-note">已托管押金</text><text class="member-amount">¥{{ fixed(balance?.amount_cents) }}</text><view class="member-rule"><text>当前加入门槛</text><text>¥{{ fixed(balance?.required_cents??club.deposit_cents) }}</text></view><view class="member-rule"><text>退还处理中</text><text>¥{{ fixed(balance?.refunding_cents) }}</text></view></view>
      <view class="member-explain"><text class="section-title">平台独立托管</text><text class="subtle-note">这笔押金仅用于你在本俱乐部的认证保障，俱乐部不能提现或自行扣罚。</text><text class="subtle-note">退出认证且关联订单、售后结束后，可申请退回原支付渠道。退款中的金额不再展示为已缴押金。</text><text v-if="!state.user" class="subtle-note">登录后查看本人押金和付款记录。</text></view>
      <button class="btn member-pay" :disabled="busy||!!state.user&&(!balance||balance.required_cents<=balance.amount_cents)" @click="open('pay')">{{ !state.user?'登录后缴纳':balance&&balance.required_cents<=balance.amount_cents?'当前无需补缴':'缴纳加入押金' }}</button><button class="text-btn member-refund" :disabled="busy||!balance?.amount_cents" @click="open('refund')">申请原路退还</button>
      <FundsProgress :payments="topups" :refunds="refunds" :busy="busy" @resume="resume" @sync="sync"/>
    </template>
    <FundsForm :opened="!!operation" :title="operation==='pay'?'缴纳押金':'退还押金'" :description="operation==='pay'?'付款经微信核验后入平台托管账户，不转入俱乐部可提现佣金。':'请先退出俱乐部认证；没有关联的在途和售后订单后，退回原付款渠道。'" v-model:amount="amount" :error="formError" :busy="busy" @close="close" @submit="submit"/>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import Icon from '../../../components/Icon.vue'
import FundsForm from '../../../components/FundsForm.vue'
import FundsProgress from '../../../components/FundsProgress.vue'
import { useClubFunds } from '../../../lib/use-club-funds'
import { state } from '../../../lib/store'
import { assetUrl } from '../../../lib/format'
const {club,balance,topups,refunds,loading,busy,error,formError,operation,amount,load,open,close,submit,resume,sync}=useClubFunds('member_club')
/** 将服务端整数分显示为固定两位金额。 */
function fixed(cents?:number):string{return ((cents||0)/100).toFixed(2)}
</script>
<style scoped>
.member-club{display:flex;align-items:center;gap:12px;padding:16px 0 24px;min-width:0}.member-club-avatar{width:52px;height:52px;border-radius:16px;flex-shrink:0;background:var(--surface)}.member-club-copy{flex:1;min-width:0;overflow-wrap:anywhere}.member-balance{padding:16px 0 24px;border-bottom:1px solid var(--line-subtle)}.member-amount{display:block;font-size:38px;line-height:50px;margin:12px 0 28px;color:var(--accent);font-weight:700;overflow-wrap:anywhere}.member-rule{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:16px 0;border-bottom:1px solid var(--line-subtle);font-size:14px;color:var(--muted)}.member-explain{padding:24px 0}.member-pay{width:100%}.member-refund{display:flex;margin:16px auto}
</style>
