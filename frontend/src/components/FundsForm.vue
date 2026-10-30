<!-- 文件作用：复用组织押金和佣金提现确认表单，金额和实名仅在当前操作生命周期保存。 -->
<template>
  <FinancePopup :opened="opened" :title="title" :busy="busy" @close="emit('close')"><view class="funds-form-sheet">
    <text class="funds-form-note">{{ description }}</text>
    <view class="funds-amount-field"><text>{{amountPrefix}}</text><input class="funds-amount-input" type="digit" maxlength="12" placeholder="0.00" :value="amount" :disabled="busy" @input="changeAmount"/></view>
    <view v-if="nameRequired" class="field"><text class="field-label">{{ nameLabel }}</text><input class="input" :value="userName" maxlength="40" :disabled="busy" placeholder="仅用于本次官方转账实名核验" @input="changeName"/></view>
    <text v-if="error" class="funds-form-error">{{ error }}</text>
    <view class="action-footer funds-form-actions"><button class="text-btn" :disabled="busy" @click="emit('close')">取消</button><button class="btn" :disabled="busy" @click="emit('submit')">{{ busy?'处理中…':'确认'+title }}</button></view>
  </view></FinancePopup>
</template>
<script setup lang="ts">
import FinancePopup from './FinancePopup.vue'
withDefaults(defineProps<{opened:boolean;title:string;amountPrefix?:string;description:string;amount:string;userName?:string;nameRequired?:boolean;nameLabel?:string;error?:string;busy?:boolean}>(),{amountPrefix:'¥',userName:'',nameRequired:false,nameLabel:'微信实名认证姓名',error:'',busy:false})
const emit=defineEmits<{'update:amount':[value:string];'update:userName':[value:string];close:[];submit:[]}>()
/** 从跨端原生输入事件中读取已缩窄的字符串，异常事件不清空草稿。 */
function inputValue(event:unknown):string|undefined{if(!event||typeof event!=='object'||!('detail' in event))return;const detail=event.detail;if(detail&&typeof detail==='object'&&'value' in detail&&typeof detail.value==='string')return detail.value}
/** 更新金额草稿，不直接发起资金操作。 */
function changeAmount(event:unknown):void{const value=inputValue(event);if(value!==undefined)emit('update:amount',value)}
/** 更新本次请求实名，离页清理由调用页负责。 */
function changeName(event:unknown):void{const value=inputValue(event);if(value!==undefined)emit('update:userName',value)}
</script>
<style scoped>
.funds-form-sheet{width:100%;box-sizing:border-box;min-width:0}.funds-form-note{display:block;margin:7.6923rpx 0 30.7692rpx;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted,#9aada2);overflow-wrap:anywhere;white-space:pre-wrap}.funds-amount-field{display:flex;align-items:center;gap:15.3846rpx;height:123.0769rpx;border:1px solid var(--line,rgba(255,255,255,0.12));border-radius:19.2308rpx;padding:0 23.0769rpx;font-size:50rpx;margin-bottom:38.4615rpx}.funds-amount-input{display:block;flex:1;min-width:0;height:92.3077rpx;font-size:57.6923rpx;color:var(--text,#edf4ef)}.funds-form-error{display:block;margin-top:23.0769rpx;font-size:26.9231rpx;line-height:42.3077rpx;color:#efb299;overflow-wrap:anywhere}.funds-form-actions{display:flex;align-items:center;justify-content:flex-end;gap:38.4615rpx;margin-top:46.1538rpx}.funds-form-actions .btn{flex:1;min-width:0}.funds-form-actions .text-btn{flex-shrink:0;min-width:100rpx}
</style>
