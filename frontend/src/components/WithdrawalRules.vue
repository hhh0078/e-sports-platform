<!-- 文件作用：个人及俱乐部钱包复用提现规则入口和弹框，明确平台门槛、渠道限制与来源审核。 -->
<template>
  <button class="withdrawal-rules-link" @click="opened=true">提现规则 <Icon name="info" :size="13"/></button>
  <view v-if="opened" class="modal-mask withdrawal-rules-mask" @click="opened=false"><view class="withdrawal-rules-sheet" @click.stop>
    <view class="withdrawal-rules-heading"><text class="section-title">提现规则</text><button class="text-btn" aria-label="关闭提现规则" @click="opened=false"><Icon name="close" :size="20"/></button></view>
    <text class="withdrawal-rule">1. 平台0元起提现，不额外设置平台最低门槛。实际申请金额必须大于0，并满足微信渠道最低金额{{ minimum ? '（当前¥'+(minimum/100).toFixed(2)+'）' : '' }}。</text>
    <text class="withdrawal-rule">2. 每天可提现一次，按北京时间自然日计算。处理中和成功申请占用当日次数；最终失败或拒绝后可重新申请，重复查询原单不新增次数。</text>
    <text class="withdrawal-rule">3. {{ club ? '仅可提现本俱乐部已结算佣金，转入当前负责人的实名微信；需核对收款人实名。' : '仅可提现已结算的服务收入。充值余额仅用于消费，押金需另行原路退还。' }}</text>
    <text class="withdrawal-rule">4. 服务验收后保留7天售后期，无争议后结算。平台会逐笔核验订单、付款来源、退款和资金占用，来源异常不能出款。</text>
    <text class="withdrawal-rule">5. 申请后冻结对应金额；微信确认收款不等于已到账，以资金记录显示的最终状态为准。未知结果请同步原申请。</text>
    <button class="btn withdrawal-rules-close" @click="opened=false">我知道了</button>
  </view></view>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import Icon from './Icon.vue'
withDefaults(defineProps<{ club?: boolean; minimum?: number }>(), { club: false, minimum: 0 })
const opened=ref(false)
</script>
<style scoped>
.withdrawal-rules-link{display:inline-flex;align-items:center;justify-content:center;gap:5px;min-height:28px;padding:0;background:transparent;color:var(--accent);font-size:13px}.withdrawal-rules-mask{z-index:180}.withdrawal-rules-sheet{width:100%;max-width:500px;max-height:85vh;overflow-y:auto;padding:20px 20px calc(20px + env(safe-area-inset-bottom));background:var(--surface);border-radius:16px;box-sizing:border-box;min-width:0}.withdrawal-rules-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:16px}.withdrawal-rule{display:block;margin-top:12px;font-size:14px;line-height:24px;color:var(--muted);overflow-wrap:anywhere}.withdrawal-rules-close{width:100%;margin-top:24px}
</style>
