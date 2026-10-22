<!-- 文件作用：按先行设计独立配置平台业务客服，仅修改客服三字段并通过官方接口裁剪压缩二维码。 -->
<template>
  <AppShell title="客服设置" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="客服配置仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading&&!loaded" class="loading-line">正在读取客服配置…</view>
      <template v-if="loaded"><view class="support-intro"><text class="support-title">平台业务客服</text><text class="support-note">供首页、个人中心的客服入口使用</text></view>
        <view class="form-section support-qr-section"><text class="field-label support-label">客服二维码 · 1:1</text><view class="support-qr-editor"><image v-if="form.support_qr_url" class="support-qr-image" :src="assetUrl(form.support_qr_url)" mode="aspectFit" :show-menu-by-longpress="true" @click="previewQR"/><button v-else class="support-qr-upload" :disabled="busy||uploading||loading" @click="uploadQR"><Icon name="photo" :size="28"/><text>上传二维码</text></button><view class="support-qr-actions"><button class="text-btn" :disabled="busy||uploading||loading" @click="uploadQR">{{ uploading?'正在处理…':form.support_qr_url?'更换二维码':'选择图片' }}</button><button v-if="form.support_qr_url" class="text-btn support-clear" :disabled="busy||uploading||loading" @click="clearQR">清除</button></view></view><text class="support-note">官方裁剪为方形，目标800px，保留二维码和四周白边；通常约500KB以内，最大3MB。</text></view>
        <view class="form-section support-contact-section"><view class="field"><text class="field-label support-label">客服电话</text><input v-model="form.support_phone" class="input support-input" maxlength="32" :disabled="busy||uploading||loading" placeholder="填写业务客服电话"/></view><view class="field"><text class="field-label support-label">客服微信号</text><input v-model="form.support_wechat" class="input support-input" maxlength="64" :disabled="busy||uploading||loading" placeholder="填写可复制的业务客服微信号"/></view></view>
        <view class="support-notes"><text class="support-note">微信在线客服使用小程序官方客服能力，请在微信后台开通客服。</text><text class="support-note">空配置会显示暂未配置，不会使用技术支持号码替代业务客服。</text></view><text v-if="formError" class="support-error">{{ formError }}</text>
        <view class="action-footer support-actions"><button class="btn support-save" :disabled="busy||uploading||loading||!!error" @click="save">{{ busy?'正在保存…':'保存客服设置' }}</button><button class="text-btn" :disabled="busy||uploading||loading" @click="restore">取消修改</button></view>
      </template>
    </template><view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { reactive,ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state,init,sessionRevision } from '../../lib/store'
import { token,errorMessage,isCancelled } from '../../api/client'
import { getAdminSettings,updateSettings } from '../../api/admin.api'
import { chooseUploadImage } from '../../lib/media'
import { assetUrl,notify } from '../../lib/format'
import type { Settings } from '../../types/models'
/** 客服编辑草稿不包含站名、公告或任何商户秘密。 */
type SupportSettings=Required<Pick<Settings,'support_qr_url'|'support_phone'|'support_wechat'>>
const form=reactive<SupportSettings>({support_qr_url:'',support_phone:'',support_wechat:''}),loading=ref(false),loaded=ref(false),busy=ref(false),uploading=ref(false),error=ref(''),formError=ref('')
let visible=false,disposed=false,version=0,mediaVersion=0,saved:SupportSettings|null=null,loadedSession:ReturnType<typeof currentSession>|null=null
/** 捕获管理员账号、令牌及认证修订，公开客服配置也不能跨账号回填。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 原生选图暂时隐藏页面时允许同一管理员继续，离页销毁和换账号均失效。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&!disposed&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 只应用已约定的客服字段，绝不回写同时被其他管理员修改的站名和公告。 */
function apply(value:Partial<Settings>):void{Object.assign(form,{support_qr_url:value.support_qr_url||'',support_phone:value.support_phone||'',support_wechat:value.support_wechat||''});saved={...form};loaded.value=true;loadedSession=currentSession()}
/** 独立读取客服配置，冷启动失败可以重新初始化身份和重试。 */
async function load():Promise<void>{if(!visible||disposed||busy.value||uploading.value)return;const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||disposed||current!==version)return;if(state.user?.role!=='admin')return;session=currentSession();const result=await getAdminSettings();if(visible&&current===version&&sameSession(session))apply(result)}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 选择官方裁剪压缩二维码，取消或失败保持原图，原生返回后再次核对账号。 */
async function uploadQR():Promise<void>{if(!loaded.value||busy.value||uploading.value||loading.value||!visible||!sameSession(loadedSession))return;const current=++mediaVersion,session=currentSession();uploading.value=true;formError.value='';try{const url=await chooseUploadImage('support_qr');if(current!==mediaVersion||!sameSession(session)||!url)return;form.support_qr_url=url}catch(err){if(current===mediaVersion&&sameSession(session)&&!isCancelled(err))formError.value=errorMessage(err)}finally{if(current===mediaVersion)uploading.value=false}}
/** 清除仅修改本地草稿，保存后才影响平台公开客服入口。 */
function clearQR():void{if(!busy.value&&!uploading.value&&!loading.value&&sameSession(loadedSession))form.support_qr_url=''}
/** 预览已托管二维码，不自动发送图片或消息。 */
function previewQR():void{if(visible&&sameSession(loadedSession)&&form.support_qr_url)uni.previewImage({urls:[assetUrl(form.support_qr_url)],current:assetUrl(form.support_qr_url)})}
/** 提交服务端审核，只发送三项客服字段，避免并发覆盖站名公告。 */
async function save():Promise<void>{if(!visible||!loaded.value||busy.value||uploading.value||loading.value||error.value||!sameSession(loadedSession))return;const body:SupportSettings={support_qr_url:form.support_qr_url,support_phone:form.support_phone.trim(),support_wechat:form.support_wechat.trim()};if(!/^[0-9+() -]{0,32}$/.test(body.support_phone)||(body.support_phone&&!/\d/.test(body.support_phone))){formError.value='客服电话最多32字符，仅填写数字、加号、括号、连字符和空格';return}if(!/^[A-Za-z0-9_-]{0,64}$/.test(body.support_wechat)){formError.value='客服微信号最多64字符，仅填写英文字母、数字、下划线和连字符';return}const current=++version,session=currentSession();busy.value=true;formError.value='';try{const result=await updateSettings(body);if(!visible||current!==version||!sameSession(session))return;apply(result);try{await init(true);if(visible&&current===version&&sameSession(session))notify('客服设置已保存')}catch(err){if(visible&&current===version&&sameSession(session))formError.value='设置已保存，公开配置刷新失败：'+errorMessage(err)}}catch(err){if(visible&&current===version&&sameSession(session))formError.value=errorMessage(err)}finally{if(current===version)busy.value=false}}
/** 撤销草稿只恢复本会话已读的客服配置。 */
function restore():void{if(!busy.value&&!uploading.value&&!loading.value&&saved&&sameSession(loadedSession)){apply(saved);formError.value=''}}
/** 身份变化清空所有草稿及选图任务，旧上传不能污染新管理员。 */
function clear():void{version++;mediaVersion++;Object.assign(form,{support_qr_url:'',support_phone:'',support_wechat:''});saved=null;loadedSession=null;loaded.value=false;loading.value=false;busy.value=false;uploading.value=false;error.value='';formError.value=''}
/** 原生选图返回保留草稿，未确认保存则重新读服务器。 */
function showPage():void{visible=true;if(loaded.value&&sameSession(loadedSession))return;void load()}
/** 普通读取保存隐藏后作废，官方选图的同账号回填通过独立媒体版本保护。 */
function hidePage():void{visible=false;version++;if(loading.value||busy.value)loaded.value=false;loading.value=false;busy.value=false}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage();clear()})
</script>
<style scoped>
.support-intro{display:flex;flex-direction:column;gap:12px;padding:8px 4px 24px}.support-title{font-size:22px;line-height:30px;font-weight:650}.support-note{display:block;color:var(--muted);font-size:14px;line-height:24px;overflow-wrap:anywhere}.support-qr-section{padding:20px 4px 24px;border:0}.support-label{font-size:14px;line-height:22px}.support-qr-editor{display:flex;align-items:center;gap:20px;min-width:0;margin:20px 0}.support-qr-image,.support-qr-upload{display:flex;align-items:center;justify-content:center;flex-direction:column;gap:12px;flex-shrink:0;width:124px;height:124px;padding:12px;box-sizing:border-box;border-radius:8px;background:#fff;color:#17251d;font-size:14px}.support-qr-image{display:block}.support-qr-actions{display:flex;flex-direction:column;align-items:flex-start;gap:8px}.support-clear{color:var(--muted)}.support-contact-section{padding:8px 4px 0;border:0}.support-input{font-size:16px}.support-notes{display:flex;flex-direction:column;gap:12px;padding:12px 4px 0}.support-error{display:block;margin:20px 4px 0;color:#e9b69d;font-size:14px;line-height:24px;overflow-wrap:anywhere}.support-actions{flex-direction:column;gap:16px;margin-top:40px}.support-save{flex:0 0 auto;width:100%}
</style>
