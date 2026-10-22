<!-- 文件作用：统一三类实缴押金徽标，按服务端有效金额展示实心安全图标与可选金额。 -->
<template><view v-if="hasDeposit" :class="['deposit-badge',{'deposit-badge-amount':showAmount}]"><Icon name="shield-filled" :size="showAmount?14:12"/><text class="deposit-badge-label">{{ badgeText }}</text></view></template>
<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import { money } from '../lib/format'
const props=withDefaults(defineProps<{amountCents?:number;showAmount?:boolean;kind?:'user_platform'|'user_club'|'club_platform'}>(),{amountCents:0,showAmount:false,kind:'user_platform'})
/** 只有服务端返回的正整数分才显示，未知、零额及退款中的无效金额均隐藏。 */
const hasDeposit=computed(()=>Number.isSafeInteger(props.amountCents)&&props.amountCents>0)
/** 列表用短标识，详情使用同一有效金额的真实说明。 */
const badgeText=computed(()=>props.showAmount?(props.kind==='user_club'?'俱乐部押金':'平台押金')+money(props.amountCents)+'元':props.kind==='user_club'?'俱乐部':'押金')
</script>
<style scoped>
.deposit-badge{display:inline-flex;vertical-align:top;align-items:center;align-self:flex-start;gap:3px;min-width:0;max-width:100%;min-height:21px;padding:2px 6px;border:1px solid #6b532b;border-radius:0 0 0 6px;box-sizing:border-box;background:#18150e;color:#e4bd70;font-size:9px;font-weight:650;line-height:15px}.deposit-badge-label{min-width:0;overflow-wrap:anywhere}.deposit-badge-amount{min-height:24px;border-radius:5px;font-size:11px;line-height:18px}
</style>
