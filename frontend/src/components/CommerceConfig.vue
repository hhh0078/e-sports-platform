<!-- 文件作用：按前端模式、支付方式、虚拟支付和微信登录分别展示与保存配置，秘密只留在本次表单。 -->
<template>
 <view class="commerce-config">
  <view v-if="section==='frontend'" class="form-section">
   <view class="commerce-heading">选择前端入口</view><text class="subtle-note">决定用户登录入口，支付方式单独设置。</text>
   <view class="commerce-options"><button v-for="item in modes" :key="item.value" :class="['commerce-option',{selected:draft.mode===item.value}]" :disabled="busy" @click="draft.mode=item.value"><text class="commerce-radio">{{draft.mode===item.value?'◉':'○'}}</text><view class="commerce-option-copy"><text class="commerce-option-title">{{item.label}}</text><text class="commerce-option-hint">{{item.value==='mini_program'?'在微信小程序中登录与使用':'在微信内通过网页授权登录'}}</text></view></button></view>
   <text v-if="draft.mode==='official_account'&&draft.payment_method==='virtual'" class="subtle-note">公众号目前仅支持微信支付，请先在支付方式中切换为微信支付。</text>
  </view>
  <view v-else-if="section==='methods'" class="form-section">
   <view class="commerce-heading">选择新付款方式</view><text class="subtle-note">已发起的付款与退款沿用原单渠道。</text>
   <view class="commerce-options"><button v-for="item in methods" :key="item.value" :class="['commerce-option',{selected:draft.payment_method===item.value}]" :disabled="busy||item.value==='virtual'&&draft.mode==='official_account'" @click="draft.payment_method=item.value"><text class="commerce-radio">{{draft.payment_method===item.value?'◉':'○'}}</text><view class="commerce-option-copy"><text class="commerce-option-title">{{item.label}}</text><text class="commerce-option-hint">{{item.value==='virtual'&&draft.mode==='official_account'?'请先切换为小程序模式':item.hint}}</text></view></button></view>
  </view>
  <template v-else-if="section==='virtual'">
   <view class="form-section"><view class="commerce-heading">接入资料 <text class="commerce-status">{{status.virtual_configured?'已配置':'待配置'}}</text></view>
    <text v-if="loaded&&!status.callback_configured" class="subtle-note">消息验签待配置，请先在服务器和微信后台接通充值与退款通知。</text>
    <view class="field"><text class="field-label">Offer ID</text><input v-model="draft.offer_id" class="input" maxlength="32" :disabled="busy" placeholder="填写虚拟支付 Offer ID"/></view>
    <view class="field"><text class="field-label">支付环境</text><view class="commerce-choices"><button v-for="item in environments" :key="item.value" :class="['commerce-choice',{selected:draft.virtual_env===item.value}]" :disabled="busy" @click="draft.virtual_env=item.value">{{draft.virtual_env===item.value?'●':'○'}} {{item.label}}</button></view></view>
    <view v-for="field in virtualSecrets" :key="field.key" class="field"><text class="field-label">{{field.label}} · {{status[field.flag]?'已保存':'未保存'}}</text><input v-model="secrets[field.key]" class="input" password maxlength="256" :disabled="busy" placeholder="留空保留已保存密钥"/></view>
   </view>
   <view class="form-section"><view class="commerce-heading">渠道手续费</view><view class="commerce-choices"><button v-for="item in bearers" :key="item.value" :class="['commerce-choice',{selected:draft.fee_bearer===item.value}]" :disabled="busy" @click="draft.fee_bearer=item.value">{{draft.fee_bearer===item.value?'●':'○'}} {{item.label}}</button></view>
    <view class="commerce-rates"><view class="field"><text class="field-label">安卓手续费（%）</text><input v-model="androidRate" class="input" type="digit" maxlength="6" :disabled="busy" placeholder="0.00"/></view><view class="field"><text class="field-label">iOS手续费（%）</text><input v-model="iosRate" class="input" type="digit" maxlength="6" :disabled="busy" placeholder="0.00"/></view></view>
    <text class="subtle-note">只影响新服务付款的报价；购买金币不重复加收渠道手续费。</text>
   </view>
  </template>
  <template v-else-if="section==='login'">
   <view class="form-section"><view class="commerce-heading">小程序登录</view><text class="subtle-note">AppID 与密钥由服务器部署配置维护。</text></view>
   <view class="form-section"><view class="commerce-heading">公众号登录 <text class="commerce-status">{{status.official_configured?'已配置':'待配置'}}</text></view>
    <view class="field"><text class="field-label">公众号 AppID</text><input v-model="draft.official_app_id" class="input" maxlength="128" :disabled="busy" placeholder="公众号 AppID"/></view>
    <view class="field"><text class="field-label">网页授权回跳地址</text><input v-model="draft.official_redirect_url" class="input" maxlength="2048" :disabled="busy" placeholder="https://你的域名/"/><text class="subtle-note">填写授权域名下不带 # 的 HTTPS 地址。</text></view>
    <view class="field"><text class="field-label">公众号 Secret · {{status.official_secret_set?'已保存':'未保存'}}</text><input v-model="secrets.official_secret" class="input" password maxlength="256" :disabled="busy" placeholder="留空保留已保存密钥"/></view>
   </view>
  </template>
  <text v-if="error" class="commerce-error">{{error}}</text><button v-if="!loaded" class="text-btn" :disabled="loading" @click="load">{{loading?'正在读取配置…':'重新读取配置'}}</button>
  <text v-if="section==='virtual'||section==='login'" class="subtle-note">秘密不回显，保存成功或离开页面即清空。</text>
  <view class="action-footer commerce-actions"><button class="text-btn" :disabled="busy||!loaded" @click="restore">取消修改</button><button class="btn" :disabled="busy||!loaded||!status.storage_available" @click="save">{{busy?'正在保存…':'保存'+title}}</button></view>
 </view>
</template>
<script setup lang="ts">
import { onMounted,onBeforeUnmount,reactive,ref,computed } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import { getCommerceSettings,updateCommerceSettings } from '../api/commerce.api'
import type { CommerceSettings,CommerceSettingsInput,CommerceMode,FeeBearer,PaymentMethod } from '../types/commerce'
import { state,sessionRevision } from '../lib/store'
import { token,errorMessage } from '../api/client'
import { notify } from '../lib/format'
const props=defineProps<{section:'frontend'|'methods'|'virtual'|'login'}>()
const labels={frontend:'前端模式',methods:'支付方式',virtual:'虚拟支付',login:'微信登录'}
const methods:Array<{value:PaymentMethod;label:string;hint:string}>=[{value:'wechat',label:'微信支付',hint:'人民币付款，使用微信支付商户'},{value:'virtual',label:'虚拟支付',hint:'小程序金币付款，1金币=1元'}]
const title=computed(()=>labels[props.section])
const modes:Array<{value:CommerceMode;label:string}>=[{value:'mini_program',label:'小程序模式'},{value:'official_account',label:'公众号模式'}],bearers:Array<{value:FeeBearer;label:string}>=[{value:'platform',label:'平台承担'},{value:'buyer',label:'买家承担'}],environments:Array<{value:0|1;label:string}>=[{value:0,label:'正式环境'},{value:1,label:'沙箱环境'}]
const virtualSecrets=[{key:'app_key',flag:'app_key_set',label:'正式AppKey'},{key:'sandbox_app_key',flag:'sandbox_app_key_set',label:'沙箱AppKey'}] as const
const draft=reactive({mode:'mini_program' as CommerceMode,payment_method:'virtual' as PaymentMethod,fee_bearer:'platform',android_fee_bps:0,ios_fee_bps:0,offer_id:'',virtual_env:0 as 0|1,official_app_id:'',official_redirect_url:''}),secrets=reactive({app_key:'',sandbox_app_key:'',official_secret:''}),status=reactive({app_key_set:false,sandbox_app_key_set:false,official_secret_set:false,virtual_configured:false,official_configured:false,callback_configured:false,storage_available:false})
const androidRate=ref('0.00'),iosRate=ref('0.00'),loading=ref(false),busy=ref(false),loaded=ref(false),error=ref('')
let version=0,visible=true,saved:CommerceSettings|null=null,loadedOwner=''
/** 配置回包必须与当前管理员及会话匹配。 */
function owner():string{return state.user?.role==='admin'?[state.user.id,token(),sessionRevision()].join(':'):''}
/** 清理本次输入，不持久保存微信秘密。 */
function clearSecrets():void{secrets.app_key='';secrets.sandbox_app_key='';secrets.official_secret=''}
/** 只应用公开字段和保存状态，忽略任何秘密回显。 */
function apply(value:CommerceSettings):void{Object.assign(draft,{mode:value.mode,payment_method:value.payment_method||(value.mode==='official_account'?'wechat':'virtual'),fee_bearer:value.fee_bearer,android_fee_bps:value.android_fee_bps,ios_fee_bps:value.ios_fee_bps,offer_id:value.offer_id||'',virtual_env:value.virtual_env,official_app_id:value.official_app_id||'',official_redirect_url:value.official_redirect_url||''});for(const key of Object.keys(status) as Array<keyof typeof status>)status[key]=!!value[key];androidRate.value=(value.android_fee_bps/100).toFixed(2);iosRate.value=(value.ios_fee_bps/100).toFixed(2);saved=value;clearSecrets()}
/** 只读取当前管理员配置，丢弃隐藏或切号后的响应。 */
async function load():Promise<void>{if(loading.value||!owner())return;const account=owner(),current=++version;loading.value=true;error.value='';try{const value=await getCommerceSettings();if(visible&&current===version&&account===owner()){apply(value);loadedOwner=account;loaded.value=true}}catch(err){if(visible&&current===version&&account===owner())error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 撤销未保存的公开草稿并清空秘密。 */
function restore():void{if(saved)apply(saved)}
/** 百分比精确转换为整数万分比，拒绝越界与多余小数。 */
function rate(value:string):number{if(!/^\d{1,3}(?:\.\d{1,2})?$/.test(value.trim()))throw new Error('手续费请输入0至99.99%的数字，最多两位小数');const result=Math.round(Number(value)*100);if(result<0||result>=10000)throw new Error('手续费须在0至99.99%之间');return result}
/** 校验草稿并交由服务端加密保存，不在客户端记录秘密。 */
async function save():Promise<void>{
 if(busy.value||!loaded.value||!owner()||loadedOwner!==owner())return
 let body:CommerceSettingsInput|undefined
 const account=owner(),current=++version
 busy.value=true;error.value=''
 try{
  if(props.section==='frontend')body={frontend_mode:draft.mode}
  else if(props.section==='methods')body={payment_method:draft.payment_method}
  else if(props.section==='virtual')body={fee_bearer:draft.fee_bearer as FeeBearer,android_fee_bps:rate(androidRate.value),ios_fee_bps:rate(iosRate.value),offer_id:draft.offer_id.trim(),virtual_env:draft.virtual_env,app_key:secrets.app_key,sandbox_app_key:secrets.sandbox_app_key}
  else{
   body={official_app_id:draft.official_app_id.trim(),official_redirect_url:draft.official_redirect_url.trim(),official_secret:secrets.official_secret}
   if(body.official_redirect_url&&(!/^https:\/\/[^\s]+$/.test(body.official_redirect_url)||body.official_redirect_url.includes('#')))throw new Error('授权回跳地址必须为不带 # 的有效HTTPS地址')
  }
  const value=await updateCommerceSettings(body)
  if(visible&&current===version&&account===owner()){
   apply(value)
   state.commerce={mode:value.mode,payment_method:value.payment_method,fee_bearer:value.fee_bearer,android_fee_bps:value.android_fee_bps,ios_fee_bps:value.ios_fee_bps,virtual_configured:value.virtual_configured,callback_configured:value.callback_configured,official_configured:value.official_configured,sms_configured:value.sms_configured}
   notify(title.value+'已保存')
  }
 }catch(err){if(visible&&current===version&&account===owner())error.value=errorMessage(err)}
 finally{if(body){body.app_key='';body.sandbox_app_key='';body.official_secret=''}if(current===version)busy.value=false}
}
/** 离开作废回包并清空秘密。 */
function hide():void{visible=false;version++;loading.value=false;busy.value=false;clearSecrets()}
/** 同一管理员返回保留公开草稿，身份变化重新读取。 */
function show():void{visible=true;if(!loaded.value||loadedOwner!==owner()){loaded.value=false;saved=null;void load()}}
onMounted(show);onShow(show);onHide(hide);onUnload(hide);onBeforeUnmount(hide)
</script>
<style scoped>
.commerce-heading{display:flex;align-items:center;gap:15.3846rpx;font-size:32.6923rpx;font-weight:700;margin-bottom:30.7692rpx}.commerce-status{margin-left:auto;font-size:23.0769rpx;color:var(--muted);font-weight:400}.commerce-choices{display:flex;flex-wrap:wrap;gap:15.3846rpx;margin:23.0769rpx 0}.commerce-choice{display:flex;align-items:center;justify-content:flex-start;min-height:44px;flex:1;min-width:0;padding:0 15.3846rpx;background:transparent;color:var(--muted);border:1px solid var(--line);font-size:26.9231rpx}.commerce-choice.selected{border-color:var(--accent);color:var(--accent)}.commerce-rates{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:30.7692rpx}.commerce-error{display:block;color:#efb299;line-height:42.3077rpx;overflow-wrap:anywhere}.commerce-actions{display:flex;flex-direction:column-reverse;gap:15.3846rpx;margin:38.4615rpx 0}.commerce-actions .btn{width:100%}
.commerce-options{display:flex;flex-direction:column;gap:23.0769rpx;margin-top:30.7692rpx}.commerce-option{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:138.4615rpx;padding:30.7692rpx;text-align:left;border:1px solid var(--line);border-radius:15.3846rpx;background:transparent;color:var(--text)}.commerce-option.selected{border-color:var(--accent);background:var(--accent-soft)}.commerce-radio{flex-shrink:0;color:var(--accent);font-size:32.6923rpx}.commerce-option-copy{flex:1;min-width:0}.commerce-option-title{display:block;font-size:30.7692rpx;line-height:46.1538rpx;font-weight:600}.commerce-option-hint{display:block;color:var(--muted);font-size:25rpx;line-height:38.4615rpx;margin-top:7.6923rpx;white-space:normal}
</style>
