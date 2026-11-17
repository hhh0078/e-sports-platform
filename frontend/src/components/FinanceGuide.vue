<!-- 文件作用：将钱包及押金的完整说明收纳到官方弹层，主页面只保留规则入口。 -->
<template>
  <view class="finance-guide-control">
  <button class="finance-guide-link" @click="opened=true">{{ label }}<Icon name="arrow" :size="12"/></button>
  <FinancePopup :opened="opened" :title="label" @close="opened=false">
    <view v-for="section in sections" :key="section.title" class="finance-guide-section"><text class="finance-guide-title">{{section.title}}</text><text v-for="line in section.lines" :key="line" class="finance-guide-copy">{{line}}</text></view>
    <button class="btn finance-guide-done" @click="opened=false">我知道了</button>
  </FinancePopup>
  </view>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue'
import Icon from './Icon.vue'
import FinancePopup from './FinancePopup.vue'
const props=withDefaults(defineProps<{kind:'wallet'|'club'|'member'|'settings';label:string;holdDays?:number}>(),{holdDays:7})
const opened=ref(false)
/** 根据资金场景保留原规则，动态售后天数始终取当前钱包配置。 */
const sections=computed(()=>{
  if(props.kind==='wallet')return [
    {title:'金币与人民币',lines:['1金币=1元，仅用于小程序消费，不能提现。金币与人民币分别记账。','订单退款退回金币；iOS现金退款由Apple处理，结果以官方为准。']},
    {title:'服务收入与提现',lines:['充值余额仅可消费，只有已结算的服务收入可以提现。','服务验收后保留'+props.holdDays+'天售后期，无退款争议后转为可提现。具体渠道门槛、频次与审核要求请查看账户余额旁的提现规则。']},
    {title:'自愿平台押金',lines:['接单无需缴纳平台押金。自愿缴纳且达到当前最低金额后，服务展示押金标识。','最低额提高后原押金继续保留；未达标时暂停标识，补齐后恢复。','双方均无待付款、进行中或售后订单时，可申请原路退还。退还处理中的金额不再展示押金标识。']},
  ]
  if(props.kind==='club')return [
    {title:'佣金收款',lines:['俱乐部可提现佣金独立记账，提现转入当前负责人实名微信，收款由负责人确认。','订单及付款来源需经平台核验，微信确认收款不等于到账，以资金记录最终状态为准。']},
    {title:'俱乐部平台押金',lines:['押金由平台托管，与个人押金、成员押金及可提现佣金分开记账。','补齐当前最低金额后在俱乐部主页展示保障标识；最低额提高后原押金保留，补齐后恢复标识。当前要求为0不代表已经缴纳押金。','存在认证成员，或有该俱乐部关联的在途、售后订单时不能退还；满足条件后按原付款渠道退还。']},
  ]
  if(props.kind==='member')return [
    {title:'平台独立托管',lines:['这笔押金仅用于你在本俱乐部的认证保障，与俱乐部佣金分开托管，俱乐部不能提现或自行扣罚。']},
    {title:'缴纳与退还',lines:['付款经微信核验后计入平台托管账户。','退出俱乐部认证且关联的在途、售后订单结束后，可申请退回原支付渠道。退款中的金额不再展示为已缴押金。']},
  ]
  return [
    {title:'费率生效',lines:['两档平台费率分别生效。俱乐部自己的抽成由负责人另行设置，单笔订单两项抽成合计不得超过100%。','抽成配置仅影响新订单，旧订单保留成交时的比例和金额。']},
    {title:'最低押金',lines:['最低额提高后，原押金继续保留；未达标暂停展示保障标识，补齐后恢复。','个人押金始终自愿，不影响自由接单。最低金额为0时不要求缴纳，未缴押金不展示标识；最高可设置100000元。']},
  ]
})
</script>
<style scoped>
.finance-guide-control{display:inline-flex;align-items:center;flex-shrink:0}
.finance-guide-link{display:inline-flex;align-items:center;justify-content:center;gap:11.5385rpx;min-height:44px;padding:0;background:transparent;color:var(--accent);font-size:25rpx;line-height:38.4615rpx}.finance-guide-section{padding:23.0769rpx 0;border-bottom:1px solid var(--line-subtle)}.finance-guide-title{display:block;font-size:28.8462rpx;line-height:46.1538rpx;font-weight:650}.finance-guide-copy{display:block;margin-top:15.3846rpx;color:var(--muted);font-size:26.9231rpx;line-height:46.1538rpx;overflow-wrap:anywhere}.finance-guide-done{width:100%;margin-top:38.4615rpx}
</style>
