<!-- 文件作用：按先行设计管理AI兼容接口与审核提示词，密钥仅本次输入保留，连接测试完全由后端执行。 -->
<template>
  <AppShell title="AI 设置" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="AI配置与连接测试仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading" class="loading-line">正在读取AI配置…</view>
      <template v-else-if="loaded&&!error">
        <view class="ai-status"><text class="ai-mode">{{ modeLabel }}</text><Icon name="info" :size="16"/><text class="ai-status-note">{{ status.configuration_notice }}</text></view>
        <text v-if="status.mode==='ai'" class="subtle-note">配置完整不代表已连通或具备图像审核能力，请测试真实兼容接口。</text>
        <view class="form-section ai-section"><text class="section-title">兼容接口</text>
          <view class="field"><text class="field-label">Base URL</text><input v-model="form.base_url" class="input ai-base-url" maxlength="2048" :disabled="busy||testing" placeholder="https://兼容接口域名/v1"/></view>
          <view class="field"><text class="field-label">模型</text><input v-model="form.model" class="input ai-model" maxlength="120" :disabled="busy||testing" placeholder="qwen3.8-flash"/></view>
          <view class="field"><view class="ai-field-line"><text class="field-label">API Key</text><text class="ai-key-state">{{ status.has_api_key?'已配置':'未配置' }}</text></view><input v-model="apiKey" class="input ai-key" password maxlength="4096" :disabled="busy||testing" placeholder="留空保留已保存密钥"/></view>
          <view class="ai-test-actions"><button class="btn-secondary btn-small ai-test" :disabled="busy||testing" @click="testConnection">{{ testing?'正在测试…':'测试连接' }}</button><text class="ai-test-note">测试仅使用已保存配置</text></view>
          <view v-if="connection" class="ai-test-result"><text>已收到真实响应 · {{ connection.decision==='allow'?'测试通过':'测试内容被拒绝' }}</text><text class="ai-result-reason">{{ connection.reason }}</text></view>
          <text v-if="testError" class="ai-test-error">{{ testError }}</text>
        </view>
        <view class="form-section ai-section ai-prompt-section"><view class="ai-field-line"><text class="section-title">审核提示词</text><Icon name="info" :size="16"/></view><textarea v-model="form.prompt" class="input ai-prompt" maxlength="12000" :disabled="busy||testing" placeholder="填写20—12000字的审核规则和要求"/><view class="ai-prompt-counter"><text>20—12000字</text><text>{{ charCount(form.prompt) }}/12000</text></view></view>
        <view class="action-footer ai-actions"><button class="btn ai-save" :disabled="busy||testing" @click="save">{{ busy?'正在保存…':'保存AI配置' }}</button><button class="text-btn" :disabled="busy||testing" @click="restore">取消修改</button></view>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { reactive,ref,computed,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state,init,sessionRevision } from '../../lib/store'
import { token,errorMessage } from '../../api/client'
import { getAISettings,saveAISettings,testAIConnection } from '../../api/ai.api'
import { notify } from '../../lib/format'
import type { AISettings,AISettingsInput,AIConnectionResult } from '../../types/ai'

/** 编辑表单只保存公开配置，秘密使用独立的短期引用。 */
type AIForm=Pick<AISettings,'base_url'|'model'|'prompt'>
const defaults:AIForm={base_url:'https://maas.qianwenaiapi.com/compatible-mode/v1',model:'qwen3.8-flash',prompt:''}
const form=reactive<AIForm>({...defaults}),apiKey=ref(''),loading=ref(false),loaded=ref(false),busy=ref(false),testing=ref(false),error=ref(''),testError=ref(''),connection=ref<AIConnectionResult|null>(null)
const status=reactive<Pick<AISettings,'has_api_key'|'ready'|'mode'|'configuration_notice'>>({has_api_key:false,ready:false,mode:'unconfigured',configuration_notice:''})
const modeLabel=computed(()=>status.mode==='local_simulation'?'本地模拟':status.mode==='ai'?'已配置AI':'未配置密钥')
let visible=false,version=0,saved:AIForm|null=null,loadedSession:ReturnType<typeof currentSession>|null=null
/** 捕获管理员账号、令牌及认证修订，所有异步操作绑定本次会话。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 页面可见并保持同一平台管理员时才允许接收配置和结果。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 身份、页面和请求版本共同防止迟到结果回填。 */
function currentRequest(current:number,session:ReturnType<typeof currentSession>){return current===version&&sameSession(session)}
/** 字符数量与后端Unicode长度保持一致。 */
function charCount(value:string){return Array.from(value).length}
/** 计算UTF-8字节数，避免多字节密钥越过后端4096字节限制。 */
function byteCount(value:string){return Array.from(value).reduce((total,char)=>{const code=char.codePointAt(0)||0;return total+(code<=0x7f?1:code<=0x7ff?2:code<=0xffff?3:4)},0)}
/** 只接收公开字段及能力标记，防止任何额外秘密进入表单。 */
function apply(result:AISettings){Object.assign(form,{base_url:result.base_url,model:result.model,prompt:result.prompt});Object.assign(status,{has_api_key:result.has_api_key,ready:result.ready,mode:result.mode,configuration_notice:result.configuration_notice});saved={...form};apiKey.value='';loaded.value=true;loadedSession=currentSession()}
/** 账号变化即移除公开草稿、密钥和测试结果。 */
function clear(){version++;Object.assign(form,defaults);Object.assign(status,{has_api_key:false,ready:false,mode:'unconfigured',configuration_notice:''});apiKey.value='';saved=null;loadedSession=null;loaded.value=false;loading.value=false;busy.value=false;testing.value=false;error.value='';testError.value='';connection.value=null}
/** 冷启动错误可以重新初始化身份，成功后再读取专属领域配置。 */
async function load(){const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==version)return;if(state.user?.role!=='admin'){clear();return}session=currentSession();const result=await getAISettings();if(currentRequest(current,session))apply(result)}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 保存前校验接口、模型、提示词与秘密长度，最终URL安全仍由后端判断。 */
function settingsBody():AISettingsInput{const body={base_url:form.base_url.trim(),model:form.model.trim(),prompt:form.prompt.trim(),api_key:apiKey.value.trim()};const match=body.base_url.match(/^https:\/\/([^\s/?#@]+)(?:\/[^\s?#]*)?$/);if(body.base_url.length>2048||!match||(match[1].includes(':')&&!match[1].endsWith(':443')))throw new Error('兼容接口需要公网HTTPS标准443地址，不能包含用户名、查询参数或片段');if(charCount(body.model)<1||charCount(body.model)>120||/[\r\n]/.test(form.model))throw new Error('模型需为1—120字且不能换行');if(charCount(body.prompt)<20||charCount(body.prompt)>12000)throw new Error('审核提示词需为20—12000字');if(byteCount(body.api_key)>4096||/[\r\n]/.test(apiKey.value))throw new Error('API Key最多4096字节且不能换行');return body}
/** 保存结束擦除请求密钥，旧账号结果不会清空新账号正在输入的秘密。 */
async function save(){if(busy.value||testing.value||!loaded.value||!sameSession(loadedSession))return;let body:AISettingsInput;try{body=settingsBody()}catch(err){notify(err);return}const current=++version,session=currentSession();busy.value=true;connection.value=null;testError.value='';try{const result=await saveAISettings(body);if(currentRequest(current,session)){apply(result);notify('AI配置已保存，真实能力请通过连接测试核实')}}catch(err){if(currentRequest(current,session))notify(err)}finally{body.api_key='';if(current===version){apiKey.value='';busy.value=false}}}
/** 测试仅调用后端已保存配置，不把本地模拟或配置状态伪装成真实连通。 */
async function testConnection(){if(busy.value||testing.value||!loaded.value||!sameSession(loadedSession))return;if(apiKey.value.trim()||JSON.stringify(form)!==JSON.stringify(saved))return notify('请先保存当前修改，再测试已保存配置');if(!status.has_api_key||!status.ready)return notify('请先保存完整兼容接口配置和API Key');const current=++version,session=currentSession();testing.value=true;connection.value=null;testError.value='';try{const result=await testAIConnection();if(result.mode!=='ai')throw new Error('连接测试未返回真实AI响应');if(currentRequest(current,session))connection.value={decision:result.decision,reason:result.reason,mode:result.mode}}catch(err){if(currentRequest(current,session)){testError.value=errorMessage(err);notify(err)}}finally{if(current===version)testing.value=false}}
/** 撤销草稿同时清空本次秘密和测试结果。 */
function restore(){if(busy.value||testing.value||!sameSession(loadedSession)||!saved)return;Object.assign(form,saved);apiKey.value='';testError.value='';connection.value=null;notify('未保存的修改已取消')}
/** 相同账号返回时保留公开草稿，秘密永不恢复。 */
function showPage(){visible=true;if(loaded.value&&sameSession(loadedSession))return;void load()}
/** 隐藏立即擦除秘密并作废未完成的配置与测试回调。 */
function hidePage(){visible=false;version++;if(loading.value||busy.value||testing.value)loaded.value=false;apiKey.value='';connection.value=null;testError.value='';loading.value=false;busy.value=false;testing.value=false}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{hidePage();clear()})
</script>
<style scoped>
.ai-status{display:flex;align-items:flex-start;gap:15.3846rpx;min-width:0;padding:15.3846rpx 0 0;color:var(--muted)}.ai-mode{flex-shrink:0;border:1px solid var(--accent);border-radius:23.0769rpx;padding:1.9231rpx 15.3846rpx;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--accent)}.ai-status-note{min-width:0;font-size:23.0769rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.ai-section{padding:46.1538rpx 0}.ai-field-line{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.ai-key-state{font-size:23.0769rpx;color:var(--muted)}.ai-base-url,.ai-model,.ai-key{font-size:26.9231rpx}.ai-test-actions{display:flex;flex-direction:column;align-items:flex-end;gap:7.6923rpx}.ai-test-note{font-size:21.1538rpx;color:var(--muted)}.ai-test-result{display:flex;flex-direction:column;gap:15.3846rpx;margin-top:30.7692rpx;padding:23.0769rpx 0;color:var(--accent);font-size:26.9231rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.ai-result-reason{color:var(--text)}.ai-test-error{display:block;margin-top:30.7692rpx;font-size:26.9231rpx;line-height:42.3077rpx;color:#e9b69d;overflow-wrap:anywhere}.ai-prompt-section{border-bottom:0}.ai-prompt{height:346.1538rpx;margin-top:30.7692rpx;font-size:26.9231rpx;line-height:42.3077rpx}.ai-prompt-counter{display:flex;justify-content:space-between;gap:15.3846rpx;margin-top:15.3846rpx;font-size:23.0769rpx;color:var(--muted)}.ai-actions{flex-direction:column;gap:30.7692rpx}.ai-save{flex:0 0 auto;width:100%}
</style>
