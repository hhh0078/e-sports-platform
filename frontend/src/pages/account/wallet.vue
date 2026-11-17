<!-- 文件作用：按先行设计展示真实钱包分区、资金原单与自愿平台押金，所有资金变化由服务端记账。 -->
<template>
  <AppShell title="我的钱包" back>
    <view v-if="error" class="error-box" @click="retryFailed">{{ error }} · 点击重试</view>
    <view class="wallet-toolbar"><FinanceGuide kind="wallet" label="钱包规则" :hold-days="wallet?.settlement_hold_days||7"/></view>
    <view v-if="miniCoinMode||(wallet?.virtual_coin_cents||0)>0" class="coin-card wallet-accent-card">
      <view class="coin-watermark"><Icon name="game" :size="118"/></view>
      <view class="wallet-heading"><Icon name="wallet" :size="20"/><text class="balance-caption">金币余额</text></view>
      <view class="balance-main coin-main"><view class="coin-copy"><text class="balance-value">{{coinAmount()}}<text class="coin-unit">金币</text></text><text class="wallet-note">仅用于小程序消费</text></view><button v-if="miniCoinMode" class="btn btn-small coin-buy" :disabled="busy" @click="open('balance')">购买金币</button></view>
    </view>
    <view class="balance-section" :class="{'wallet-accent-card':!miniCoinMode&&!(wallet?.virtual_coin_cents||0)}">
      <view class="wallet-heading balance-heading"><view class="wallet-heading"><Icon name="wallet" :size="19"/><text class="balance-caption">账户余额</text></view><WithdrawalRules :minimum="wallet?.withdrawal_min_cents"/></view>
      <view class="balance-main"><text class="balance-value">{{walletAmount('spendable_cents')}}</text><view class="wallet-actions"><button v-if="!miniCoinMode" class="btn btn-secondary btn-small" :disabled="busy" @click="open('balance')">充值</button><button class="btn btn-secondary btn-small" :disabled="busy" @click="open('withdrawal')">提现</button></view></view>
      <text class="wallet-note balance-availability">可提现 {{walletAmount('available_cents')}}</text><text v-if="!state.user" class="wallet-note">登录后查看</text>
      <view class="balance-breakdown"><view class="balance-part"><text class="wallet-note">进行中</text><text>{{walletAmount('ongoing_cents')}}</text></view><view class="balance-part"><text class="wallet-note">结算中</text><text>{{walletAmount('settling_cents')}}</text></view><view class="balance-part"><text class="wallet-note">提现中</text><text>{{walletAmount('withdrawal_frozen_cents')}}</text></view></view>
    </view>
    <view class="deposit-section"><view class="wallet-heading"><Icon name="shield-filled" :size="21"/><text class="section-title">平台押金</text><text class="voluntary">自愿</text></view><view class="deposit-main"><text class="deposit-value">{{walletAmount('deposit_cents')}}</text><button class="text-btn deposit-pay" :disabled="busy||!!state.user&&!wallet" @click="open('deposit')">{{wallet?.deposit_shortfall_cents?'补缴押金':'自愿缴纳'}}<Icon name="arrow" :size="14"/></button></view><text v-if="wallet?.deposit_qualified" class="wallet-note deposit-qualified">押金已达标 · 展示保障标识</text><text v-else class="wallet-note">目标 {{walletAmount('deposit_required_cents')}}<text v-if="wallet?.deposit_shortfall_cents"> · 还需补缴 {{walletAmount('deposit_shortfall_cents')}}</text></text><view v-if="wallet?.deposit_cents" class="refund-entry"><button class="text-btn" :disabled="busy" @click="open('refund')">申请退还押金</button></view><text v-if="wallet?.deposit_refunding_cents" class="wallet-note">退还处理中 {{walletAmount('deposit_refunding_cents')}}</text></view>
    <view v-if="activeTopups.length||activeWithdrawals.length||activeRefunds.length" class="progress-section"><text class="section-title">资金进度</text>
      <view v-for="record in activeTopups" :key="record.id" class="progress-row"><view class="record-copy"><text>{{ paymentPurposeName(record.purpose) }} ¥{{ fixed(record.amount_cents) }}</text><text class="wallet-note">{{ walletStatus(record.status) }} · {{ time(record.created_at) }}</text></view><view class="record-actions"><button class="text-btn" :disabled="busy" @click="resumeTopup(record)">继续支付</button><button class="text-btn" :disabled="busy" @click="syncRecord('topup',record.id)">同步</button></view></view>
      <view v-for="record in activeWithdrawals" :key="record.id" class="progress-row"><view class="record-copy"><text>提现 ¥{{ fixed(record.amount_cents) }}</text><text class="wallet-note">{{ walletStatus(record.status) }}</text><text v-if="record.failure_reason" class="wallet-note">{{ record.failure_reason }}</text></view><button class="text-btn" :disabled="busy" @click="syncRecord('withdrawal',record.id,record.status==='wait_user_confirm')">{{ record.status==='wait_user_confirm'?'确认收款':'同步' }}</button></view>
      <view v-for="record in activeRefunds" :key="record.id" class="progress-row"><view class="record-copy"><text>退还押金 ¥{{ fixed(record.amount_cents) }}</text><text class="wallet-note">{{ walletStatus(record.status) }}</text></view><button class="text-btn" :disabled="busy" @click="syncRecord('refund',record.id)">同步</button></view>
    </view>
    <view class="ledger-section"><view class="wallet-heading ledger-heading"><Icon name="receipt" :size="20"/><text class="section-title">资金明细</text></view><view class="ledger-tabs"><button v-for="tab in tabs" :key="tab.id" :class="['ledger-tab',{selected:filter===tab.id}]" @click="filter=tab.id">{{ tab.label }}</button></view>
      <view v-if="loading&&!entries.length" class="ledger-empty">正在读取资金记录…</view><view v-else-if="!filteredEntries.length" class="ledger-empty"><Icon name="receipt" :size="28"/><text>{{ state.user?'暂无资金记录':'请登录后查看资金记录' }}</text><button v-if="!state.user" class="text-btn" @click="open('balance')">去登录</button></view>
      <view v-for="entry in filteredEntries" :key="entry.id" class="ledger-row"><view class="record-copy"><text class="entry-title">{{ entry.note || entryName(entry) }}</text><text class="wallet-note">{{ time(entry.created_at) }} · {{ entryName(entry) }}</text></view><text class="entry-amount">{{entry.virtual_delta?fixed(entry.virtual_delta)+' 金币':'¥'+fixed(entry.amount_cents)}}</text></view><ListLoadState v-if="state.user&&(entries.length||topups.length||withdrawals.length||refunds.length||loading||error)" :loading="loading" :has-more="hasMore" :error="error" @retry="retryFailed"/>
    </view>
    <text v-if="wallet?.dev_mode" class="wallet-note dev-note">本地测试钱包，充值、支付和提现均为模拟资金。</text>
    <FundsForm :amount-prefix="operation==='balance'&&miniCoinMode?'金币':'¥'" :opened="!!operation" :title="operation?titles[operation]:''" :description="operation?descriptions[operation]:''" v-model:amount="amount" v-model:user-name="userName" :name-required="operation==='withdrawal'&&Number(amount)>=2000" :error="formError" :busy="busy" @close="close" @submit="submit"/>
  </AppShell>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import FinanceGuide from '../../components/FinanceGuide.vue'
import WithdrawalRules from '../../components/WithdrawalRules.vue'
import FundsForm from '../../components/FundsForm.vue'
import ListLoadState from '../../components/ListLoadState.vue'
import { useNativePageRefresh } from '../../lib/native-page-refresh'
import { paymentPurposeName } from '../../lib/finance'
import { state,sessionRevision } from '../../lib/store'
import { time } from '../../lib/format'
import { useWallet } from '../../lib/use-wallet'
import type { Wallet,WalletEntry } from '../../types/wallet'
import { walletStatus,walletKind } from '../../lib/wallet'
const {wallet,entries,topups,withdrawals,refunds,filteredEntries,loading,busy,error,operation,amount,userName,formError,filter,hasMore,load,retryFailed,open,close,submit,syncRecord,resumeTopup}=useWallet()
useNativePageRefresh({refresh:()=>load(),more:()=>error.value?retryFailed():load(true),blocked:()=>busy.value||!!operation.value,context:()=>[state.user?.id,state.user?.role,sessionRevision(),filter.value]})
/** 本地模拟钱包保持人民币记账，真实小程序金币独立展示。 */
const miniCoinMode=computed(()=>!state.devMode&&(state.commerce?.payment_method?state.commerce.payment_method==='virtual':state.commerce?.mode==='mini_program'))
const tabs=[{id:'all',label:'明细'},{id:'recharge',label:'充值'},{id:'withdrawal',label:'提现'},{id:'deposit',label:'押金'}]
const titles=computed(()=>({balance:miniCoinMode.value?'购买金币':'充值',withdrawal:'提现',deposit:wallet.value?.deposit_shortfall_cents?'补缴平台押金':'缴纳平台押金',refund:'退还押金'}))
const descriptions=computed(()=>({balance:miniCoinMode.value?'1金币=1元，本次输入为购买金币数，仅供小程序消费，不可提现。购买金币不重复收取渠道手续费。':'充值余额仅用于平台消费，不可提现。',withdrawal:'仅可提现服务收入。申请后冻结金额，平台审核后由微信转账。',deposit:'平台押金完全自愿，不影响自由接单。当前最低¥'+fixed(wallet.value?.deposit_required_cents)+'，已缴¥'+fixed(wallet.value?.deposit_cents)+(wallet.value?.deposit_shortfall_cents?'，还需补缴¥'+fixed(wallet.value.deposit_shortfall_cents)+'。补齐后恢复标识，原押金继续保留。':'。达到最低金额后展示保障标识。'),refund:'退至押金原支付渠道；双方均无待付款、进行中或售后订单才能退还。'}))
const activeTopups=computed(()=>topups.value.filter(record=>record.status==='pending_payment'))
const activeWithdrawals=computed(()=>withdrawals.value.filter(record=>record.status!=='success').sort((a,b)=>Number(['failed','rejected'].includes(a.status))-Number(['failed','rejected'].includes(b.status))))
const activeRefunds=computed(()=>refunds.value.filter(record=>(!record.scope||record.scope==='personal')&&record.status==='processing'))
/** 同类充值分录根据真实分区显示押金名称，避免把押金误标为消费充值。 */
function entryName(entry:WalletEntry):string{return entry.kind==='topup'&&entry.deposit_delta>0?'缴纳平台押金':walletKind(entry.kind)}
/** 金币未知时显示占位，不能把缺失或非法数值伪装为零余额。 */
function coinAmount():string{if(!state.user)return '0.00';const value=wallet.value?.virtual_coin_cents;return typeof value==='number'&&Number.isSafeInteger(value)&&value>=0?fixed(value):'—'}
/** 钱包金额保留两位小数，未取得余额时由页面单独显示占位。 */
function fixed(cents?:number):string{return ((cents||0)/100).toFixed(2)}
/**
 * 仅展示服务端确认的资金分区，账户合计不参与提现资格计算。
 * @param key - 服务端钱包金额字段。
 * @returns 访客显示零，已登录但金额未知显示占位，否则显示人民币金额。
 */
function walletAmount(key:keyof Wallet):string{if(!state.user)return '¥0.00';const value=wallet.value?.[key];return typeof value==='number'&&Number.isSafeInteger(value)?'¥'+fixed(value):'—'}
</script>
<style scoped>
.wallet-toolbar{display:flex;justify-content:flex-end;margin-top:-15.3846rpx;margin-bottom:15.3846rpx}.wallet-accent-card{padding:38.4615rpx;border:1px solid var(--accent-line);border-radius:23.0769rpx;background:var(--surface);background-image:linear-gradient(115deg,var(--accent-soft),transparent)}.coin-card{position:relative;overflow:hidden;margin-bottom:23.0769rpx}.coin-watermark{position:absolute;right:23.0769rpx;top:19.2308rpx;color:var(--accent);opacity:.07;pointer-events:none}.coin-main{position:relative;align-items:flex-end}.coin-copy{flex:1;min-width:0}.coin-buy{font-size:26.9231rpx;min-height:44px;flex-shrink:0;padding:0 26.9231rpx}.coin-unit{font-size:26.9231rpx;font-weight:500;margin-left:11.5385rpx;letter-spacing:0}.balance-section{padding:30.7692rpx 0 38.4615rpx;border-bottom:1px solid var(--line-subtle)}.balance-section.wallet-accent-card{padding:30.7692rpx 38.4615rpx 38.4615rpx}.balance-heading{justify-content:space-between}.balance-caption{font-size:30.7692rpx;line-height:46.1538rpx;font-weight:600;color:var(--text)}.balance-main{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:19.2308rpx;margin:15.3846rpx 0 7.6923rpx}.balance-value{display:block;font-size:80.7692rpx;line-height:100rpx;font-weight:700;letter-spacing:-1.9231rpx;min-width:0;overflow-wrap:anywhere}.wallet-actions{display:flex;gap:15.3846rpx;flex-shrink:0}.wallet-actions .btn{color:var(--accent);border:1px solid var(--accent);background:transparent;min-height:44px;padding:0 26.9231rpx}.balance-breakdown{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;margin-top:30.7692rpx;padding-top:30.7692rpx;border-top:1px solid var(--line-subtle);font-size:28.8462rpx;color:var(--text)}.balance-part{display:flex;flex-direction:column;gap:7.6923rpx;min-width:0;overflow-wrap:anywhere}.wallet-note{display:block;font-size:25rpx;line-height:42.3077rpx;color:var(--muted);overflow-wrap:anywhere}.wallet-heading{display:flex;align-items:center;gap:15.3846rpx;min-width:0}.wallet-heading .app-icon{color:var(--accent)}.voluntary{font-size:21.1538rpx;line-height:34.6154rpx;padding:0 11.5385rpx;border-radius:7.6923rpx;background:var(--accent-soft);color:var(--accent)}.deposit-section{padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle)}.deposit-main{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;min-width:0;gap:15.3846rpx;margin:15.3846rpx 0 3.8462rpx}.deposit-value{min-width:0;max-width:100%;overflow-wrap:anywhere;font-size:61.5385rpx;line-height:80.7692rpx;font-weight:650;letter-spacing:-.9615rpx}.deposit-pay{min-height:44px}.refund-entry{margin-top:15.3846rpx}.refund-entry .text-btn{min-height:44px}.deposit-qualified{color:var(--accent)}.progress-section{padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle)}.ledger-section{padding:38.4615rpx 0}.ledger-heading{justify-content:flex-start;margin-bottom:23.0769rpx}.ledger-tabs{display:flex;align-items:center;padding:7.6923rpx;gap:7.6923rpx;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--surface)}.ledger-tab{flex:1;min-width:0;display:flex;align-items:center;justify-content:center;min-height:44px;padding:0;background:transparent;border-radius:11.5385rpx;color:var(--muted);font-size:26.9231rpx}.ledger-tab.selected{color:var(--accent);background:var(--accent-soft);font-weight:600}.ledger-empty{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:23.0769rpx;min-height:230.7692rpx;color:var(--muted);font-size:25rpx}.progress-row,.ledger-row{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;padding:30.7692rpx 0;border-bottom:1px solid var(--line-subtle)}.record-copy{flex:1;min-width:0;display:flex;flex-direction:column;gap:7.6923rpx;font-size:26.9231rpx;overflow-wrap:anywhere}.record-actions{display:flex;flex-direction:column;gap:15.3846rpx}.entry-title{line-height:42.3077rpx}.entry-amount{flex-shrink:0;max-width:50%;overflow-wrap:anywhere;text-align:right;font-size:30.7692rpx;font-weight:600}.dev-note{padding-bottom:46.1538rpx}
@media(max-width:350px){.wallet-accent-card{padding:30.7692rpx}.balance-value{font-size:73.0769rpx}.coin-main{align-items:flex-start}.coin-buy{padding:0 23.0769rpx}.balance-breakdown{gap:11.5385rpx}.balance-section.wallet-accent-card{padding:23.0769rpx 30.7692rpx}}
</style>
