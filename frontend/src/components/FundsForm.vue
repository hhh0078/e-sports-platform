<!-- 文件作用：复用组织押金和佣金提现确认表单，金额和实名仅在当前操作生命周期保存。 -->
<template>
  <view v-if="opened" class="modal-mask funds-form-mask" @click="emit('close')"><view class="funds-form-sheet" @click.stop>
    <text class="section-title">{{ title }}</text><text class="funds-form-note">{{ description }}</text>
    <view class="funds-amount-field"><text>¥</text><input class="funds-amount-input" type="digit" maxlength="12" placeholder="0.00" :value="amount" :disabled="busy" @input="changeAmount"/></view>
    <view v-if="nameRequired" class="field"><text class="field-label">俱乐部负责人微信实名认证姓名</text><input class="input" :value="userName" maxlength="40" :disabled="busy" placeholder="仅用于本次官方转账实名核验" @input="changeName"/></view>
    <text v-if="error" class="funds-form-error">{{ error }}</text>
    <view class="action-footer funds-form-actions"><button class="text-btn" :disabled="busy" @click="emit('close')">取消</button><button class="btn" :disabled="busy" @click="emit('submit')">{{ busy?'处理中…':'确认'+title }}</button></view>
  </view></view>
</template>
<script setup lang="ts">
withDefaults(defineProps<{opened:boolean;title:string;description:string;amount:string;userName?:string;nameRequired?:boolean;error?:string;busy?:boolean}>(),{userName:'',nameRequired:false,error:'',busy:false})
const emit=defineEmits<{'update:amount':[value:string];'update:userName':[value:string];close:[];submit:[]}>()
/** 从跨端原生输入事件中读取已缩窄的字符串，异常事件不清空草稿。 */
function inputValue(event:unknown):string|undefined{if(!event||typeof event!=='object'||!('detail' in event))return;const detail=event.detail;if(detail&&typeof detail==='object'&&'value' in detail&&typeof detail.value==='string')return detail.value}
/** 更新金额草稿，不直接发起资金操作。 */
function changeAmount(event:unknown):void{const value=inputValue(event);if(value!==undefined)emit('update:amount',value)}
/** 更新本次请求实名，离页清理由调用页负责。 */
function changeName(event:unknown):void{const value=inputValue(event);if(value!==undefined)emit('update:userName',value)}
</script>
<style scoped>
.funds-form-mask{z-index:150}.funds-form-sheet{width:100%;max-width:460px;max-height:85vh;overflow-y:auto;padding:24px;background:var(--surface);border-radius:16px;box-sizing:border-box;min-width:0}.funds-form-note{display:block;margin:14px 0;font-size:14px;line-height:24px;color:var(--muted);overflow-wrap:anywhere}.funds-amount-field{display:flex;align-items:center;gap:8px;height:68px;border-bottom:1px solid var(--line);font-size:28px;margin-bottom:20px}.funds-amount-input{flex:1;min-width:0;height:48px;font-size:30px;color:var(--text)}.funds-form-error{display:block;margin-top:12px;font-size:14px;line-height:22px;color:#efb299;overflow-wrap:anywhere}.funds-form-actions{justify-content:flex-end;gap:24px;margin-top:24px}.funds-form-actions .btn{min-width:140px}
</style>
