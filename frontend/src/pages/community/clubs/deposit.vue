<!-- 文件作用：用户查看和缴退自己在指定俱乐部的成员押金，展示平台托管及原路退还规则。 -->
<template>
  <AppShell title="成员押金" back>
    <view v-if="error" class="error-box" @click="retryFailed">{{ error }} · 点击重试</view>
    <view v-if="loading&&!club" class="loading-line">正在读取成员押金…</view>
    <template v-if="club">
      <view class="member-club"><image v-if="club.avatar_url" class="member-club-avatar" :src="assetUrl(club.avatar_url)" mode="aspectFill"/><Icon v-else name="club" :size="32"/><view class="member-club-copy"><text class="section-title">{{ club.name }}</text><text class="subtle-note">成员俱乐部押金 · 平台独立托管</text></view></view>
      <view class="member-balance"><text class="subtle-note">已托管押金</text><text class="member-amount">¥{{ fixed(balance?.amount_cents) }}</text><view class="member-rule"><text>当前加入门槛</text><text>¥{{ fixed(balance?.required_cents??club.deposit_cents) }}</text></view><view class="member-rule"><text>退还处理中</text><text>¥{{ fixed(balance?.refunding_cents) }}</text></view></view>
      <view class="member-explain"><text class="section-title">平台独立托管</text><text class="subtle-note">这笔押金仅用于你在本俱乐部的认证保障，俱乐部不能提现或自行扣罚。</text><text class="subtle-note">退出认证且关联订单、售后结束后，可申请退回原支付渠道。退款中的金额不再展示为已缴押金。</text><text v-if="!state.user" class="subtle-note">登录后查看本人押金和付款记录。</text></view>
      <button class="btn member-pay" :disabled="busy||!!state.user&&(!balance||balance.required_cents<=balance.amount_cents)" @click="open('pay')">{{ !state.user?'登录后缴纳':balance&&balance.required_cents<=balance.amount_cents?'当前无需补缴':'缴纳加入押金' }}</button><button class="text-btn member-refund" :disabled="busy||!balance?.amount_cents" @click="open('refund')">申请原路退还</button>
      <FundsProgress :payments="topups" :refunds="refunds" :busy="busy" @resume="resume" @sync="sync"/><ListLoadState v-if="state.user" :loading="loading" :has-more="hasMore" :error="error" @retry="retryFailed"/>
    </template>
    <FundsForm :opened="!!operation" :title="operation==='pay'?'缴纳押金':'退还押金'" :description="operation==='pay'?'付款经微信核验后入平台托管账户，不转入俱乐部可提现佣金。':'请先退出俱乐部认证；没有关联的在途和售后订单后，退回原付款渠道。'" v-model:amount="amount" :error="formError" :busy="busy" @close="close" @submit="submit"/>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import Icon from '../../../components/Icon.vue'
import FundsForm from '../../../components/FundsForm.vue'
import ListLoadState from '../../../components/ListLoadState.vue'
import { useNativePageRefresh } from '../../../lib/native-page-refresh'
import FundsProgress from '../../../components/FundsProgress.vue'
import { useClubFunds } from '../../../lib/use-club-funds'
import { state,sessionRevision } from '../../../lib/store'
import { assetUrl } from '../../../lib/format'
const {id,club,balance,topups,refunds,hasMore,loading,busy,error,formError,operation,amount,load,retryFailed,open,close,submit,resume,sync}=useClubFunds('member_club')
useNativePageRefresh({refresh:()=>load(),more:()=>error.value?retryFailed():load(true),blocked:()=>busy.value||!!operation.value,context:()=>[state.user?.id,state.user?.role,sessionRevision(),id.value]})
/** 将服务端整数分显示为固定两位金额。 */
function fixed(cents?:number):string{return ((cents||0)/100).toFixed(2)}
</script>
<style scoped>
.member-club{display:flex;align-items:center;gap:23.0769rpx;padding:30.7692rpx 0 46.1538rpx;min-width:0}.member-club-avatar{width:100rpx;height:100rpx;border-radius:30.7692rpx;flex-shrink:0;background:var(--surface)}.member-club-copy{flex:1;min-width:0;overflow-wrap:anywhere}.member-balance{padding:30.7692rpx 0 46.1538rpx;border-bottom:1px solid var(--line-subtle)}.member-amount{display:block;font-size:73.0769rpx;line-height:96.1538rpx;margin:23.0769rpx 0 53.8462rpx;color:var(--accent);font-weight:700;overflow-wrap:anywhere}.member-rule{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;padding:30.7692rpx 0;border-bottom:1px solid var(--line-subtle);font-size:26.9231rpx;color:var(--muted)}.member-explain{padding:46.1538rpx 0}.member-pay{width:100%}.member-refund{display:flex;margin:30.7692rpx auto}
</style>
