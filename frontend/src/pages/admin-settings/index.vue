<!-- 文件作用：小程序内的管理员协议版本及微信支付配置入口，秘密仅在本次填写时保留，保存或离开后清空。 -->
<template>
  <AppShell title="协议与支付设置" subtitle="清楚的约定，可信的交易" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="协议发布和支付配置均由服务端再次验证管理权限。" icon="shield"><button class="btn" @click="go('/pages/login/index')">切换账号</button></EmptyState>
    <template v-else-if="state.user?.role==='admin'">
      <view class="settings-tabs"><button :class="{active:tab==='agreements'}" @click="tab='agreements'">用户协议</button><button :class="{active:tab==='payment'}" @click="tab='payment'">微信支付</button></view>
      <view v-if="loading" class="loading-line">正在读取配置…</view>
      <template v-else-if="tab==='agreements'">
        <view class="notice mb">发布后的协议可在登录页阅读。标题或正文发生变化时，必须填写新的版本号。</view>
        <view v-for="document in documents" :key="document.key" class="document-panel panel">
          <view class="panel-heading"><Icon :name="document.icon" :size="21"/><text>{{ document.label }}</text><text class="updated-date">{{ date(agreements[document.key].updated_at) }}</text></view>
          <view class="field"><text class="field-label">公开标题 · 2—80 个字</text><input v-model="agreements[document.key].title" class="input" maxlength="80"/></view>
          <view class="field"><text class="field-label">版本号 · 最多 64 个字</text><input v-model="agreements[document.key].version" class="input version-input" maxlength="64" placeholder="例如 2026-10-18.2"/></view>
          <view class="field"><view class="field-line"><text class="field-label">完整正文 · 30—8000 个字</text><text>{{ charCount(agreements[document.key].content) }} / 8000</text></view><textarea v-model="agreements[document.key].content" class="input agreement-content" maxlength="8000" :auto-height="false" placeholder="请填写完整且可阅读的协议内容"/></view>
          <button class="text-btn" @click="go('/pages/agreements/index?type='+document.key)">查看当前已发布版本 <Icon name="arrow" :size="12"/></button>
        </view>
        <view class="settings-actions"><button class="btn-secondary" :disabled="busy" @click="restoreAgreements">取消修改</button><button class="btn" :disabled="busy||!loaded" @click="saveAgreements">{{ busy?'正在发布…':'发布协议版本' }}</button></view>
      </template>
      <template v-else>
        <view class="payment-status panel"><view class="status-icon"><Icon :name="paymentStatus.configured?'shield':'settings'" :size="26"/></view><view><text>{{ paymentStatus.configured?'支付配置已就绪':'支付配置尚未完整' }}</text><text>{{ paymentStatus.configured?'真实交易仍需微信 AppID 与商户权限匹配':'可以先保存已准备好的资料，缺项时不会发起真实扣款' }}</text></view></view>
        <view v-if="!paymentStatus.storage_available" class="notice mb">服务器尚未启用支付资料的加密保存，请按项目部署说明完成配置后再保存。当前页面仍可查看已配置状态。</view>
        <view v-if="state.devMode" class="notice mb">本地开发环境仍使用测试支付；填写配置不会把测试订单变成真实扣款。</view>
        <view class="payment-panel panel">
          <view class="panel-heading"><Icon name="wallet" :size="21"/><text>商户信息</text><text class="updated-date">{{ paymentStatus.source==='admin'?'后台配置':'服务器配置' }}</text></view>
          <view v-for="field in publicFields" :key="field.key" class="field"><text class="field-label">{{ field.label }}</text><input v-model="payment[field.key]" class="input" :maxlength="field.url?2048:128" :placeholder="field.placeholder"/><text v-if="field.hint" class="hint input-hint">{{ field.hint }}</text></view>
        </view>
        <view class="payment-panel panel">
          <view class="panel-heading"><Icon name="shield" :size="21"/><text>密钥与证书</text></view><text class="hint secret-intro">已保存的秘密不会回显。留空表示保留当前值；输入新内容才会替换。</text>
          <view class="field"><view class="field-line"><text class="field-label">APIv3 密钥 · 32 字节</text><text :class="{configured:paymentStatus.api_v3_key_set}">{{ paymentStatus.api_v3_key_set?'已配置':'未配置' }}</text></view><input v-model="secrets.api_v3_key" class="input" password maxlength="64" :placeholder="paymentStatus.api_v3_key_set?'留空保留已保存的密钥':'填写微信支付商户 APIv3 密钥'"/></view>
          <view class="field"><view class="field-line"><text class="field-label">商户私钥 · PEM</text><text :class="{configured:paymentStatus.merchant_private_key_set}">{{ paymentStatus.merchant_private_key_set?'已配置':'未配置' }}</text></view><textarea v-model="secrets.merchant_private_key_pem" class="input key-content" maxlength="16384" :placeholder="paymentStatus.merchant_private_key_set?'已保存，若要更换请粘贴完整新私钥':'粘贴完整商户私钥，包含 BEGIN / END 行'"/></view>
          <view class="field"><view class="field-line"><text class="field-label">微信支付平台公钥 · PEM</text><text :class="{configured:paymentStatus.platform_public_key_set}">{{ paymentStatus.platform_public_key_set?'已配置':'未配置' }}</text></view><textarea v-model="secrets.platform_public_key_pem" class="input key-content" maxlength="16384" :placeholder="paymentStatus.platform_public_key_set?'已保存，若要更换请粘贴完整新公钥':'粘贴完整微信支付公钥或平台证书'"/></view>
          <text class="hint">只在本次填写期间保留输入，保存成功或离开页面即清空。服务端加密存储并保留管理审计。</text>
        </view>
        <view class="settings-actions"><button class="btn-secondary" :disabled="busy" @click="restorePayment">取消修改</button><button class="btn" :disabled="busy||!loaded||!paymentStatus.storage_available" @click="savePayment">{{ busy?'正在保存…':'保存支付配置' }}</button></view>
      </template>
    </template>
  </AppShell>
</template>
<script setup>
import { ref, reactive } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, init, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { go, notify } from '../../lib/format'

const documents=[{key:'user_agreement',label:'用户服务协议',icon:'orders'},{key:'privacy_policy',label:'隐私政策',icon:'shield'}]
const publicFields=[{key:'merchant_id',label:'微信支付商户号',placeholder:'微信支付商户平台中的商户号'},{key:'certificate_serial',label:'商户 API 证书序列号',placeholder:'与商户私钥对应的证书序列号'},{key:'platform_public_key_id',label:'微信支付公钥 ID',placeholder:'例如 PUB_KEY_ID_…'},{key:'notify_url',label:'支付结果通知地址',placeholder:'https://你的域名/api/payments/wechat/notify',url:true,hint:'需要公网可访问的 HTTPS 地址，与服务端支付回调路由一致。'},{key:'refund_notify_url',label:'退款结果通知地址',placeholder:'https://你的域名/api/payments/wechat/refund-notify',url:true}]
const tab=ref('agreements'),loading=ref(true),busy=ref(false),error=ref(''),loaded=ref(false)
const agreements=reactive({user_agreement:{title:'',version:'',content:'',updated_at:''},privacy_policy:{title:'',version:'',content:'',updated_at:''}})
const payment=reactive({merchant_id:'',certificate_serial:'',platform_public_key_id:'',notify_url:'',refund_notify_url:''})
const secrets=reactive({api_v3_key:'',merchant_private_key_pem:'',platform_public_key_pem:''})
const paymentStatus=reactive({configured:false,storage_available:false,source:'',api_v3_key_set:false,merchant_private_key_set:false,platform_public_key_set:false})
let version=0,visible=true,savedAgreements='',savedPayment='',loadedSession=null

/** 捕获当前管理员的令牌与会话序号。 */
function currentSession(){return{token:token(),revision:sessionRevision(),userId:state.user?.id}}
/** 管理权限或会话变化后，不接受此前发出的响应。 */
function sameSession(session){return session&&state.user?.role==='admin'&&session.token===token()&&session.revision===sessionRevision()&&session.userId===state.user?.id}
/** 只有当前可见且版本匹配的页面可以接收配置。 */
function currentRequest(current,session){return visible&&current===version&&sameSession(session)}
/** 统计 Unicode 字符数量，使中文和扩展字符按实际长度校验。 */
function charCount(value){return Array.from(value||'').length}
/** 日期仅作版本信息展示，无有效日期时显示尚未发布。 */
function date(value){return value?String(value).slice(0,10):'尚未发布'}
/** 清空本次输入的所有秘密，绝不写入持久缓存。 */
function clearSecrets(){Object.keys(secrets).forEach(key=>{secrets[key]=''})}
/** 只复制公开字段和是否配置标志，不接收任何服务端秘密回显。 */
function applyPayment(result){publicFields.forEach(field=>{payment[field.key]=result[field.key]||''});Object.keys(paymentStatus).forEach(key=>{paymentStatus[key]=key==='source'?result[key]||'':!!result[key]});savedPayment=JSON.stringify(payment);clearSecrets()}
/** 并行读取两类管理员配置，并校验身份后一起展示。 */
async function load(){const current=++version;loading.value=true;error.value='';let session;try{await init();if(current!==version||!visible)return;session=currentSession();if(state.user?.role!=='admin'){loaded.value=false;clearSecrets();return}const[agreementData,paymentData]=await Promise.all([api('/admin/agreements'),api('/admin/payment-settings')]);if(!currentRequest(current,session))return;documents.forEach(document=>{agreements[document.key]={...agreementData[document.key]}});savedAgreements=JSON.stringify(agreements);applyPayment(paymentData);loadedSession=session;loaded.value=true}catch(err){if(current===version&&(!session||sameSession(session)))error.value=err.message}finally{if(current===version)loading.value=false}}
/** 撤销当前协议草稿，保持已发布内容不变。 */
function restoreAgreements(){if(savedAgreements)Object.assign(agreements,JSON.parse(savedAgreements));notify('未发布的修改已取消')}
/** 撤销公开配置草稿并清空本次输入的秘密。 */
function restorePayment(){if(savedPayment)Object.assign(payment,JSON.parse(savedPayment));clearSecrets();notify('未保存的修改已取消')}
/** 发布完整两份协议，正文变化沿用旧版本号时提前提示。 */
async function saveAgreements(){if(busy.value||!loaded.value||state.user?.role!=='admin')return;const body={},previous=JSON.parse(savedAgreements||'{}');for(const document of documents){const draft=agreements[document.key],item={title:draft.title.trim(),version:draft.version.trim(),content:draft.content.trim()};if(charCount(item.title)<2||charCount(item.title)>80||charCount(item.version)<1||charCount(item.version)>64||charCount(item.content)<30||charCount(item.content)>8000)return notify(document.label+'需填写有效标题、版本号和 30—8000 字的完整正文');const old=previous[document.key];if(old&&old.version===item.version&&(old.title!==item.title||old.content!==item.content))return notify(document.label+'的标题或正文已变更，请使用新的版本号');body[document.key]=item}const current=++version,session=currentSession();busy.value=true;try{const result=await api('/admin/agreements','PATCH',body);if(!currentRequest(current,session))return;documents.forEach(document=>{agreements[document.key]={...result[document.key]}});savedAgreements=JSON.stringify(agreements);notify('协议新版本已发布')}catch(err){if(currentRequest(current,session))notify(err)}finally{busy.value=false}}
/** 校验公开字段及密钥字节格式，允许合法缺项草稿交由服务器加密保存。 */
async function savePayment(){if(busy.value||!loaded.value||!paymentStatus.storage_available||state.user?.role!=='admin')return;const body={};for(const field of publicFields){const value=String(payment[field.key]||'').trim();if(value&&field.url&&!/^https:\/\/[^\s/]+(?:\/[^\s]*)?$/.test(value))return notify(field.label+'需要有效的 HTTPS 地址');body[field.key]=value}body.api_v3_key=secrets.api_v3_key.trim();body.merchant_private_key_pem=secrets.merchant_private_key_pem.trim();body.platform_public_key_pem=secrets.platform_public_key_pem.trim();if(body.api_v3_key&&!/^[\x21-\x7e]{32}$/.test(body.api_v3_key))return notify('APIv3 密钥必须是 32 位英文、数字或符号');const current=++version,session=currentSession();busy.value=true;try{const result=await api('/admin/payment-settings','PATCH',body);if(!currentRequest(current,session))return;applyPayment(result);notify(result.configured?'支付配置已保存':'配置草稿已保存，请按已配置状态补齐资料')}catch(err){if(currentRequest(current,session))notify(err)}finally{Object.keys(body).filter(key=>key in secrets).forEach(key=>{body[key]=''});busy.value=false}}
/** 返回同一管理员的编辑页面时保留公开草稿，秘密不会被恢复。 */
function showPage(){visible=true;if(loaded.value&&sameSession(loadedSession))return;load()}
/** 离开即清空秘密并作废未完成回调，避免账号切换泄露管理状态。 */
function hidePage(){visible=false;version++;if(busy.value||loading.value)loaded.value=false;clearSecrets()}
onShow(showPage)
onHide(hidePage)
onUnload(hidePage)
</script>
<style scoped>
.settings-tabs{display:flex;gap:8px;background:#ffffff05;padding:5px;border:1px solid #ffffff10;border-radius:11px;margin:12px 0 19px}.settings-tabs>button{flex:1;margin:0;padding:9px;background:none;border-radius:8px;font-size:14px;line-height:24px;color:#91a496}.settings-tabs>button.active{background:#ffffff0b;color:var(--accent);font-weight:700}.document-panel,.payment-panel{padding:17px;margin-bottom:16px}.panel-heading{display:flex;align-items:center;gap:8px;font-size:17px;font-weight:750;margin-bottom:20px;color:#dce8df}.panel-heading>.updated-date{margin-left:auto;color:#789381;font-size:10px;font-weight:400}.field-line{display:flex;align-items:center;justify-content:space-between;gap:8px}.field-line>text:last-child{font-size:10px;color:#819b89;white-space:nowrap}.field-line>text.configured{color:var(--accent)}.version-input{font-family:monospace}.agreement-content{height:350px;font-size:13px;line-height:23px;box-sizing:border-box}.document-panel>.text-btn{display:flex;align-items:center;justify-content:flex-end;gap:4px;width:100%;font-size:12px;margin-top:10px;padding:0}.settings-actions{display:flex;gap:10px;margin:22px 0}.settings-actions>button{flex:1;font-size:13px}.settings-actions>.btn{flex:1.5}.payment-status{display:flex;align-items:center;gap:13px;padding:17px;margin-bottom:16px}.status-icon{display:flex;align-items:center;justify-content:center;width:48px;height:48px;border-radius:12px;background:#ffffff08;color:var(--accent);flex-shrink:0}.payment-status>view:last-child>text:first-child{display:block;font-size:16px;font-weight:700;line-height:24px}.payment-status>view:last-child>text:last-child{display:block;font-size:11px;color:#8ba392;line-height:19px;margin-top:4px}.input-hint{display:block;margin-top:8px;line-height:20px}.secret-intro{display:block;line-height:21px;margin:-4px 0 19px}.key-content{height:125px;font-family:monospace;font-size:11px;line-height:19px}.payment-panel>.hint{display:block;line-height:21px}.field-label{line-height:23px}
</style>
