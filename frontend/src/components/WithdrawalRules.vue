<!-- 文件作用：个人及俱乐部钱包复用提现规则入口和弹框，明确平台门槛、渠道限制与来源审核。 -->
<template>
  <button class="withdrawal-rules-link" @click="opened=true">提现规则 <Icon name="info" :size="13"/></button>
  <FinancePopup :opened="opened" title="提现规则" @close="opened=false">
    <text class="withdrawal-rule">1. 平台0元起提现，不额外设置平台最低门槛。实际申请金额必须大于0，并满足微信渠道最低金额{{ minimum ? '（当前¥'+(minimum/100).toFixed(2)+'）' : '' }}。</text>
    <text class="withdrawal-rule">2. 每天可提现一次，按北京时间自然日计算。处理中和成功申请占用当日次数；最终失败或拒绝后可重新申请，重复查询原单不新增次数。</text>
    <text class="withdrawal-rule">3. {{ club ? '仅可提现本俱乐部已结算佣金，转入当前负责人的实名微信；需核对收款人实名。' : '仅可提现已结算的服务收入。充值余额仅用于消费，押金需另行原路退还。' }}</text>
    <text class="withdrawal-rule">4. 服务验收后保留7天售后期，无争议后结算。平台会逐笔核验订单、付款来源、退款和资金占用，来源异常不能出款。</text>
    <text class="withdrawal-rule">5. 申请后冻结对应金额；微信确认收款不等于已到账，以资金记录显示的最终状态为准。未知结果请同步原申请。</text>
    <button class="btn withdrawal-rules-close" @click="opened=false">我知道了</button>
  </FinancePopup>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import Icon from './Icon.vue'
import FinancePopup from './FinancePopup.vue'
withDefaults(defineProps<{ club?: boolean; minimum?: number }>(), { club: false, minimum: 0 })
const opened=ref(false)
</script>
<style scoped>
.withdrawal-rules-link{display:inline-flex;align-items:center;justify-content:center;gap:9.6154rpx;min-height:53.8462rpx;padding:0;background:transparent;color:var(--accent);font-size:25rpx}.withdrawal-rule{display:block;margin-top:23.0769rpx;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted,#9aada2);overflow-wrap:anywhere}.withdrawal-rules-close{width:100%;margin-top:46.1538rpx}
</style>
