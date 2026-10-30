<!-- 文件作用：管理员配置支付模式、渠道手续费和服务端加密保存的微信接入资料。 -->
<template>
  <view class="commerce-config">
    <view class="form-section"><view class="commerce-heading">登录与支付模式</view><text class="subtle-note">小程序使用微信虚拟支付；公众号通过微信授权登录并使用普通微信支付。</text>
      <view class="commerce-choices"><button v-for="item in modes" :key="item.value" :class="['commerce-choice',{selected:draft.mode===item.value}]" :disabled="busy" @click="draft.mode=item.value">{{draft.mode===item.value?'●':'○'}} {{item.label}}</button></view>
      <view class="field"><text class="field-label">手续费承担方</text><view class="commerce-choices"><button v-for="item in bearers" :key="item.value" :class="['commerce-choice',{selected:draft.fee_bearer===item.value}]" :disabled="busy" @click="draft.fee_bearer=item.value">{{draft.fee_bearer===item.value?'●':'○'}} {{item.label}}</button></view></view>
      <view class="commerce-rates"><view class="field"><text class="field-label">安卓手续费（%）</text><input v-model="androidRate" class="input" type="digit" maxlength="6" :disabled="busy" placeholder="0.00"/></view><view class="field"><text class="field-label">iOS手续费（%）</text><input v-model="iosRate" class="input" type="digit" maxlength="6" :disabled="busy" placeholder="0.00"/></view></view>
      <text class="subtle-note">1金币=1元。费率只影响新付款，买家承担时付款前展示费用；修改模式不改变历史资金归属。</text>
    </view>
    <view class="form-section"><view class="commerce-heading">小程序虚拟支付 <text class="commerce-status">{{status.virtual_configured?'已配置':'待配置'}}</text></view>
      <text v-if="loaded&&!status.callback_configured" class="subtle-note">微信消息推送尚未配置，请先完成金币充值和退款通知设置。</text>
      <view class="field"><text class="field-label">Offer ID</text><input v-model="draft.offer_id" class="input" maxlength="128" :disabled="busy" placeholder="已开通的虚拟支付Offer ID"/></view>
      <view class="field"><text class="field-label">支付环境</text><view class="commerce-choices"><button v-for="item in environments" :key="item.value" :class="['commerce-choice',{selected:draft.virtual_env===item.value}]" :disabled="busy" @click="draft.virtual_env=item.value">{{draft.virtual_env===item.value?'●':'○'}} {{item.label}}</button></view></view>
      <view v-for="field in virtualSecrets" :key="field.key" class="field"><text class="field-label">{{field.label}} · {{status[field.flag]?'已保存':'未保存'}}</text><input v-model="secrets[field.key]" class="input" password maxlength="256" :disabled="busy" placeholder="留空保留已保存密钥"/></view>
    </view>
    <view class="form-section"><view class="commerce-heading">公众号登录 <text class="commerce-status">{{status.official_configured?'已配置':'待配置'}}</text></view>
      <view class="field"><text class="field-label">公众号AppID</text><input v-model="draft.official_app_id" class="input" maxlength="128" :disabled="busy" placeholder="公众号AppID，与小程序分开配置"/></view>
      <view class="field"><text class="field-label">网页授权回跳地址</text><input v-model="draft.official_redirect_url" class="input" maxlength="2048" :disabled="busy" placeholder="https://你的域名/"/><text class="subtle-note">填写已配置网页授权域名的HTTPS根地址，不带 #，授权后自动返回原页面。</text></view>
      <view class="field"><text class="field-label">公众号Secret · {{status.official_secret_set?'已保存':'未保存'}}</text><input v-model="secrets.official_secret" class="input" password maxlength="256" :disabled="busy" placeholder="留空保留已保存密钥"/></view>
      <text class="subtle-note">普通微信支付沿用下方商户配置。网页授权域名、JSAPI支付授权目录和商户AppID绑定须与微信后台一致。手机号使用真实短信验证；未配置时明确提示。</text>
    </view>
    <text v-if="error" class="commerce-error">{{error}}</text><button v-if="!loaded" class="text-btn" :disabled="loading" @click="load">{{loading?'正在读取模式配置…':'重新读取模式配置'}}</button><text class="subtle-note">秘密不回显，保存成功或离开页面即清空。</text>
    <view class="action-footer commerce-actions"><button class="text-btn" :disabled="busy||!loaded" @click="restore">取消修改</button><button class="btn" :disabled="busy||!loaded||!status.storage_available" @click="save">{{busy?'正在保存…':'保存模式配置'}}</button></view>
  </view>
</template>
<script setup lang="ts">
import { onMounted,onBeforeUnmount,reactive,ref } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import { getCommerceSettings,updateCommerceSettings } from '../api/commerce.api'
import type { CommerceSettings,CommerceSettingsInput,CommerceMode,FeeBearer } from '../types/commerce'
import { state,sessionRevision } from '../lib/store'
import { token,errorMessage } from '../api/client'
import { notify } from '../lib/format'
const modes:Array<{value:CommerceMode;label:string}>=[{value:'mini_program',label:'小程序模式'},{value:'official_account',label:'公众号模式'}],bearers:Array<{value:FeeBearer;label:string}>=[{value:'platform',label:'平台承担'},{value:'buyer',label:'买家承担'}],environments:Array<{value:0|1;label:string}>=[{value:0,label:'正式环境'},{value:1,label:'沙箱环境'}]
const virtualSecrets=[{key:'app_key',flag:'app_key_set',label:'正式AppKey'},{key:'sandbox_app_key',flag:'sandbox_app_key_set',label:'沙箱AppKey'}] as const
const draft=reactive<CommerceSettingsInput>({mode:'mini_program',fee_bearer:'platform',android_fee_bps:0,ios_fee_bps:0,offer_id:'',virtual_env:0,official_app_id:'',official_redirect_url:''}),secrets=reactive({app_key:'',sandbox_app_key:'',official_secret:''}),status=reactive({app_key_set:false,sandbox_app_key_set:false,official_secret_set:false,virtual_configured:false,official_configured:false,callback_configured:false,storage_available:false})
const androidRate=ref('0.00'),iosRate=ref('0.00'),loading=ref(false),busy=ref(false),loaded=ref(false),error=ref('')
let version=0,visible=true,saved:CommerceSettings|null=null,loadedOwner=''
/** 配置回包必须与当前管理员及会话匹配。 */
function owner():string{return state.user?.role==='admin'?[state.user.id,token(),sessionRevision()].join(':'):''}
/** 清理本次输入，不持久保存微信秘密。 */
function clearSecrets():void{secrets.app_key='';secrets.sandbox_app_key='';secrets.official_secret=''}
/** 只应用公开字段和保存状态，忽略任何秘密回显。 */
function apply(value:CommerceSettings):void{Object.assign(draft,{mode:value.mode,fee_bearer:value.fee_bearer,android_fee_bps:value.android_fee_bps,ios_fee_bps:value.ios_fee_bps,offer_id:value.offer_id||'',virtual_env:value.virtual_env,official_app_id:value.official_app_id||'',official_redirect_url:value.official_redirect_url||''});for(const key of Object.keys(status) as Array<keyof typeof status>)status[key]=!!value[key];androidRate.value=(value.android_fee_bps/100).toFixed(2);iosRate.value=(value.ios_fee_bps/100).toFixed(2);saved=value;clearSecrets()}
/** 只读取当前管理员配置，丢弃隐藏或切号后的响应。 */
async function load():Promise<void>{if(loading.value||!owner())return;const account=owner(),current=++version;loading.value=true;error.value='';try{const value=await getCommerceSettings();if(visible&&current===version&&account===owner()){apply(value);loadedOwner=account;loaded.value=true}}catch(err){if(visible&&current===version&&account===owner())error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 撤销未保存的公开草稿并清空秘密。 */
function restore():void{if(saved)apply(saved)}
/** 百分比精确转换为整数万分比，拒绝越界与多余小数。 */
function rate(value:string):number{if(!/^\d{1,3}(?:\.\d{1,2})?$/.test(value.trim()))throw new Error('手续费请输入0至99.99%的数字，最多两位小数');const result=Math.round(Number(value)*100);if(result<0||result>=10000)throw new Error('手续费须在0至99.99%之间');return result}
/** 校验草稿并交由服务端加密保存，不在客户端记录秘密。 */
async function save():Promise<void>{if(busy.value||!loaded.value||!owner())return;let body:CommerceSettingsInput|undefined;const account=owner(),current=++version;busy.value=true;error.value='';try{body={...draft,android_fee_bps:rate(androidRate.value),ios_fee_bps:rate(iosRate.value),offer_id:draft.offer_id.trim(),official_app_id:draft.official_app_id.trim(),official_redirect_url:draft.official_redirect_url.trim(),...secrets};if(body.official_redirect_url&&(!/^https:\/\/[^\s]+$/.test(body.official_redirect_url)||body.official_redirect_url.includes('#')))throw new Error('授权回跳地址必须为不带 # 的有效HTTPS地址');const value=await updateCommerceSettings(body);if(visible&&current===version&&account===owner()){apply(value);state.commerce={mode:value.mode,fee_bearer:value.fee_bearer,android_fee_bps:value.android_fee_bps,ios_fee_bps:value.ios_fee_bps,virtual_configured:value.virtual_configured,official_configured:value.official_configured,sms_configured:value.sms_configured};notify('模式配置已保存')}}catch(err){if(visible&&current===version&&account===owner())error.value=errorMessage(err)}finally{if(body){body.app_key='';body.sandbox_app_key='';body.official_secret=''}if(current===version)busy.value=false}}
/** 离开作废回包并清空秘密。 */
function hide():void{visible=false;version++;loading.value=false;busy.value=false;clearSecrets()}
/** 同一管理员返回保留公开草稿，身份变化重新读取。 */
function show():void{visible=true;if(!loaded.value||loadedOwner!==owner()){loaded.value=false;saved=null;void load()}}
onMounted(show);onShow(show);onHide(hide);onUnload(hide);onBeforeUnmount(hide)
</script>
<style scoped>
.commerce-heading{display:flex;align-items:center;gap:15.3846rpx;font-size:32.6923rpx;font-weight:700;margin-bottom:30.7692rpx}.commerce-status{margin-left:auto;font-size:23.0769rpx;color:var(--muted);font-weight:400}.commerce-choices{display:flex;flex-wrap:wrap;gap:15.3846rpx;margin:23.0769rpx 0}.commerce-choice{display:flex;align-items:center;justify-content:flex-start;min-height:44px;flex:1;min-width:0;padding:0 15.3846rpx;background:transparent;color:var(--muted);border:1px solid var(--line);font-size:26.9231rpx}.commerce-choice.selected{border-color:var(--accent);color:var(--accent)}.commerce-rates{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:30.7692rpx}.commerce-error{display:block;color:#efb299;line-height:42.3077rpx;overflow-wrap:anywhere}.commerce-actions{display:flex;flex-direction:column-reverse;gap:15.3846rpx;margin:38.4615rpx 0}.commerce-actions .btn{width:100%}
</style>
