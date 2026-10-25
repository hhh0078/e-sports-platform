<!-- 文件作用：统一三类实缴押金徽标，按服务端有效金额展示实心安全图标与可选金额。 -->
<template><view v-if="hasDeposit&&showAmount" class="deposit-badge deposit-badge-amount"><Icon name="shield-filled" :size="14"/><text class="deposit-badge-label">{{badgeText}}</text></view><SecurityMarker v-else-if="hasDeposit" :label="badgeText" :description="(kind==='user_club'?'俱乐部托管押金':'平台押金')+money(amountCents)+'元'"/></template>
<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import SecurityMarker from './SecurityMarker.vue'
import { money } from '../lib/format'
const props=withDefaults(defineProps<{amountCents?:number;showAmount?:boolean;kind?:'user_platform'|'user_club'|'club_platform'}>(),{amountCents:0,showAmount:false,kind:'user_platform'})
/** 只有服务端返回的正整数分才显示，未知、零额及退款中的无效金额均隐藏。 */
const hasDeposit=computed(()=>Number.isSafeInteger(props.amountCents)&&props.amountCents>0)
/** 列表用短标识，详情使用同一有效金额的真实说明。 */
const badgeText=computed(()=>props.showAmount?(props.kind==='user_club'?'俱乐部押金':'平台押金')+money(props.amountCents)+'元':props.kind==='user_club'?'俱乐部':'押金')
</script>
<style scoped>
.deposit-badge{display:inline-flex;vertical-align:top;align-items:center;align-self:flex-start;gap:3.8462rpx;min-width:0;max-width:100%;min-height:34.6154rpx;padding:1.9231rpx 9.6154rpx;border:1px solid rgba(143,111,55,0.6);border-radius:0 0 0 9.6154rpx;box-sizing:border-box;background:rgba(24,21,14,0.76);color:#e4bd70;font-size:15.3846rpx;font-weight:650;line-height:26.9231rpx}.deposit-badge-label{min-width:0;overflow-wrap:anywhere}.deposit-badge-amount{gap:5.7692rpx;min-height:46.1538rpx;padding:3.8462rpx 11.5385rpx;border-radius:9.6154rpx;font-size:21.1538rpx;line-height:34.6154rpx;background:rgba(24,21,14,0.88)}
</style>
