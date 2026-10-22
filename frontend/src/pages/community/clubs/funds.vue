<!-- 文件作用：负责人管理俱乐部独立佣金、来源审查提现及俱乐部缴给平台的押金。 -->
<template>
  <AppShell title="俱乐部资金" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <view v-if="loading&&!club" class="loading-line">正在读取俱乐部资金…</view>
    <view v-else-if="club&&!canManage" class="subtle-note">仅俱乐部负责人可管理资金。</view>
    <template v-else-if="canManage">
      <view class="funds-balance"><text class="funds-label">{{ club?.name }} · 可提现佣金</text><text class="funds-value">{{ wallet?'¥'+fixed(wallet.available_cents):'—' }}</text><text class="subtle-note">审核后转入当前负责人的实名微信账户</text><view class="funds-breakdown"><view class="funds-stat"><text class="funds-label">进行中</text><text>¥{{ fixed(wallet?.ongoing_cents) }}</text></view><view class="funds-stat"><text class="funds-label">结算中</text><text>¥{{ fixed(wallet?.settling_cents) }}</text></view><view class="funds-stat"><text class="funds-label">提现中</text><text>¥{{ fixed(wallet?.withdrawal_frozen_cents) }}</text></view></view><button class="btn funds-primary" :disabled="busy||!wallet||!wallet.available_cents" @click="open('withdraw')">申请提现</button><WithdrawalRules club :minimum="wallet?.withdrawal_min_cents"/></view>
      <view class="funds-security"><text class="section-title">俱乐部平台押金</text><text class="funds-deposit">¥{{ fixed(balance?.amount_cents) }}</text><text class="subtle-note">当前要求 ¥{{ fixed(balance?.required_cents) }}，{{ !balance?.required_cents?'当前规则免缴，不代表已缴押金。':'缴纳后在俱乐部主页展示保障标识。' }}</text><text class="subtle-note">有认证成员，或有该俱乐部关联的在途、售后订单时，不能退还。押金由平台托管，不计入佣金。</text><text v-if="balance?.refunding_cents" class="subtle-note">原路退还处理中 ¥{{ fixed(balance.refunding_cents) }}</text><view class="funds-actions"><button class="btn btn-secondary" :disabled="busy||!balance||balance.required_cents<=balance.amount_cents" @click="open('pay')">{{ balance&&balance.required_cents<=balance.amount_cents?'无需补缴':'缴纳平台押金' }}</button><button class="text-btn" :disabled="busy||!balance?.amount_cents" @click="open('refund')">申请原路退还</button></view></view>
      <FundsProgress :payments="topups" :refunds="refunds" :withdrawals="withdrawals" :busy="busy" @resume="resume" @sync="sync"/>
      <view class="funds-ledger"><text class="section-title">最近佣金明细</text><text v-if="!entries.length" class="subtle-note">暂无佣金记录</text><view v-for="entry in entries" :key="entry.id" class="funds-ledger-row"><view class="funds-ledger-copy"><text>{{ entry.note }}</text><text class="funds-label">{{ time(entry.created_at) }}</text></view><text>¥{{ fixed(entry.amount_cents) }}</text></view></view>
      <text v-if="wallet?.dev_mode" class="subtle-note">本地测试资金，未发生真实微信扣款或到账。</text>
    </template>
    <FundsForm :opened="!!operation" :title="operation==='pay'?'缴纳押金':operation==='refund'?'退还押金':'佣金提现'" :description="operation==='withdraw'?'平台将逐笔审查订单及付款来源，确认后转入俱乐部负责人实名微信。':operation==='refund'?'认证成员为0、无在途及售后订单时，按原付款单退还。':'这笔款项属于俱乐部平台押金，与个人押金和可提现佣金分别托管。'" v-model:amount="amount" v-model:user-name="userName" :name-required="operation==='withdraw'" :error="formError" :busy="busy" @close="close" @submit="submit"/>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import FundsForm from '../../../components/FundsForm.vue'
import FundsProgress from '../../../components/FundsProgress.vue'
import WithdrawalRules from '../../../components/WithdrawalRules.vue'
import { useClubFunds } from '../../../lib/use-club-funds'
import { time } from '../../../lib/format'
const {club,balance,wallet,entries,topups,refunds,withdrawals,canManage,loading,busy,error,formError,operation,amount,userName,load,open,close,submit,resume,sync}=useClubFunds('club_platform')
/** 组织金额保留两位小数，不将未知余额当成可出款证明。 */
function fixed(cents?:number):string{return ((cents||0)/100).toFixed(2)}
</script>
<style scoped>
.funds-balance,.funds-security{padding:16px 0 24px;border-bottom:1px solid var(--line-subtle);min-width:0}.funds-label{display:block;font-size:13px;line-height:22px;color:var(--muted);overflow-wrap:anywhere}.funds-value{display:block;margin:12px 0;font-size:38px;line-height:48px;font-weight:700;overflow-wrap:anywhere}.funds-breakdown{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;margin:24px 0}.funds-stat{display:flex;flex-direction:column;gap:7px;min-width:0;font-size:15px;overflow-wrap:anywhere}.funds-primary{width:100%;margin-bottom:12px}.funds-security{padding-top:24px}.funds-deposit{display:block;font-size:28px;line-height:38px;color:var(--accent);margin:16px 0}.funds-actions{display:flex;align-items:center;gap:16px;flex-wrap:wrap;margin-top:20px}.funds-actions .btn{flex:1;min-width:140px}.funds-ledger{padding:24px 0}.funds-ledger-row{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0;padding:16px 0;border-bottom:1px solid var(--line-subtle);font-size:14px}.funds-ledger-copy{flex:1;min-width:0;overflow-wrap:anywhere}
</style>
