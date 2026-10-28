<!-- 文件作用：提供平台管理员身份证核验密钥配置页，秘密仅在本次输入期间保留且从不回显。 -->
<template>
  <AppShell title="身份核验配置" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="身份核验密钥仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading" class="loading-line">正在读取核验配置…</view>
      <template v-else-if="loaded&&!error">
        <view class="identity-config-status"><text class="identity-config-tag">{{ status.has_api_key?'已配置密钥':'未配置密钥' }}</text><text class="identity-config-provider">小小API</text></view>
        <text class="subtle-note">配置完成后，成年用户可提交姓名和身份证进行一致性核验。</text>
        <view v-if="!status.storage_available" class="error-box">服务器尚未配置密钥加密能力，请先配置服务端加密主密钥。</view>
        <view class="form-section">
          <text class="section-title">核验服务</text>
          <view class="field"><text class="field-label">接口地址</text><text class="identity-config-url">{{ status.api_url }}</text></view>
          <view class="field"><view class="identity-config-field"><text class="field-label">接口密钥</text><text class="identity-config-key-state">{{ status.has_api_key?'已配置':'未配置' }}</text></view><input v-model="apiKey" class="input identity-config-key" password maxlength="4096" :disabled="busy||!status.storage_available" placeholder="留空保留已保存密钥"/></view>
          <text class="subtle-note">密钥加密保存在服务端，保存后不回显。</text>
        </view>
        <view class="form-section">
          <text class="section-title">核验规则</text>
          <text class="identity-config-rule">仅满18周岁且姓名、身份证一致时通过。用户默认未通过核验，每次支付前检查。</text>
          <text class="subtle-note">保存配置不调用真实身份证核验。</text>
        </view>
        <text v-if="saveError" class="identity-config-error">{{ saveError }}</text>
        <view class="action-footer identity-config-actions"><button class="btn identity-config-save" :disabled="busy||!status.storage_available" @click="save">{{ busy?'正在保存…':'保存核验配置' }}</button><button class="text-btn" :disabled="busy" @click="cancel">取消修改</button></view>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>

<script setup lang="ts">
import { reactive,ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import { state,init,sessionRevision } from '../../lib/store'
import { token,errorMessage } from '../../api/client'
import { getIdentitySettings,saveIdentitySettings } from '../../api/admin.api'
import { notify } from '../../lib/format'
import type { IdentitySettings,IdentitySettingsInput } from '../../types/models'

const apiURL='https://v2.xxapi.cn/api/idCardCheck'
const status=reactive<IdentitySettings>({has_api_key:false,configured:false,storage_available:false,api_url:apiURL})
const apiKey=ref(''),loading=ref(false),loaded=ref(false),busy=ref(false),error=ref(''),saveError=ref('')
let visible=false,version=0,loadedSession:ReturnType<typeof currentSession>|null=null

/** 捕获管理员账号、令牌及认证修订，异步响应绑定同一会话。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 只有可见页面且仍属于同一管理员会话才接收配置。 */
function sameSession(session:ReturnType<typeof currentSession>|null):boolean{return !!session&&visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 页面与会话版本共同拦截迟到读取和保存响应。 */
function currentRequest(current:number,session:ReturnType<typeof currentSession>):boolean{return current===version&&sameSession(session)}
/** 只提取脱敏配置状态，额外响应字段不进入表单或缓存。 */
function apply(result:IdentitySettings):void{Object.assign(status,{has_api_key:result.has_api_key===true,configured:result.configured===true,storage_available:result.storage_available===true,api_url:apiURL});apiKey.value='';loaded.value=true;loadedSession=currentSession()}
/** 切号后移除旧配置、错误及临时密钥。 */
function clear():void{version++;Object.assign(status,{has_api_key:false,configured:false,storage_available:false,api_url:apiURL});apiKey.value='';loaded.value=false;loading.value=false;busy.value=false;error.value='';saveError.value='';loadedSession=null}
/** 冷启动失败可以重试初始化身份，权限确认后读取配置。 */
async function load():Promise<void>{const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==version)return;if(state.user?.role!=='admin'){clear();return}session=currentSession();const result=await getIdentitySettings();if(currentRequest(current,session))apply(result)}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 保存仅提交临时密钥，空值由服务端保留原密钥，结束后擦除请求秘密。 */
async function save():Promise<void>{
  if(busy.value||!loaded.value||!status.storage_available||!sameSession(loadedSession))return
  saveError.value=''
  if(/[\r\n]/.test(apiKey.value)||apiKey.value.length>4096){saveError.value='接口密钥最多4096字且不能包含换行';return}
  const body:IdentitySettingsInput={api_key:apiKey.value.trim()},current=++version,session=currentSession();busy.value=true
  try{const result=await saveIdentitySettings(body);if(currentRequest(current,session)){apply(result);notify('身份核验配置已保存')}}
  catch(err){if(currentRequest(current,session))saveError.value=errorMessage(err)}
  finally{body.api_key='';if(current===version){apiKey.value='';busy.value=false}}
}
/** 撤销本次输入，不修改服务端已保存密钥。 */
function cancel():void{if(busy.value||!sameSession(loadedSession))return;apiKey.value='';saveError.value='';notify('未保存的修改已取消')}
/** 页面重新显示时读取当前会话配置，秘密不恢复。 */
function showPage():void{visible=true;if(loaded.value&&sameSession(loadedSession))return;void load()}
/** 隐藏立即擦除输入，并作废未完成读取和保存回调。 */
function hidePage():void{visible=false;version++;if(loading.value||busy.value)loaded.value=false;apiKey.value='';saveError.value='';loading.value=false;busy.value=false}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{hidePage();clear()})
</script>

<style scoped>
.identity-config-status{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0;padding-top:30.7692rpx}.identity-config-tag{border:1px solid var(--line);border-radius:38.4615rpx;padding:3.8462rpx 19.2308rpx;color:var(--muted);font-size:23.0769rpx;line-height:38.4615rpx}.identity-config-provider,.identity-config-key-state{color:var(--muted);font-size:23.0769rpx}.identity-config-field{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.identity-config-url{display:block;padding:23.0769rpx 0;border-bottom:1px solid var(--line);font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere;word-break:break-all}.identity-config-key{font-size:26.9231rpx}.identity-config-rule{display:block;color:var(--muted);font-size:26.9231rpx;line-height:46.1538rpx}.identity-config-actions{flex-direction:column;gap:15.3846rpx}.identity-config-save{width:100%;flex:0 0 auto}.identity-config-error{display:block;margin-top:23.0769rpx;font-size:25rpx;line-height:42.3077rpx;color:#f1b39d;overflow-wrap:anywhere}
</style>
