<!-- 文件作用：按服务端有效金额复用平台押金小标识和金额说明，不表示担保或赔付承诺。 -->
<template><view v-if="hasDeposit" :class="['deposit-badge',{'deposit-badge-amount':showAmount}]"><Icon name="wallet" :size="showAmount?14:10"/><text class="deposit-badge-label">{{ badgeText }}</text></view></template>
<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import { money } from '../lib/format'
const props=withDefaults(defineProps<{amountCents?:number;showAmount?:boolean}>(),{amountCents:0,showAmount:false})
/** 只有服务端返回的正整数分才显示，未知、零额及退款中的无效金额均隐藏。 */
const hasDeposit=computed(()=>Number.isSafeInteger(props.amountCents)&&props.amountCents>0)
/** 列表用短标识，详情使用同一有效金额的真实说明。 */
const badgeText=computed(()=>props.showAmount?'已缴平台押金 ¥'+money(props.amountCents):'平台押金')
</script>
<style scoped>
.deposit-badge{display:inline-flex;align-items:center;align-self:flex-start;gap:4px;min-width:0;max-width:100%;min-height:18px;color:var(--accent);font-size:10px;line-height:16px}.deposit-badge-label{min-width:0;overflow-wrap:anywhere}.deposit-badge-amount{font-size:12px;line-height:18px}
</style>
