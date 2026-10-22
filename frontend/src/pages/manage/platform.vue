<!-- 文件作用：按先行设计独立编辑平台名称与公告，保护管理员会话并刷新公共配置。 -->
<template>
  <AppShell title="平台设置" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="平台设置仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading" class="loading-line">正在读取平台设置…</view>
      <template v-else-if="loaded&&!error">
        <view class="form-section platform-section">
          <view class="platform-heading"><Icon name="settings" :size="20"/><text>基础信息</text></view>
          <view class="field"><view class="platform-field-line"><text class="field-label">平台名称</text><text class="platform-counter">{{ charCount(form.site_name) }}/24</text></view><input v-model="form.site_name" class="input platform-name" maxlength="24" :disabled="busy" placeholder="请输入平台名称"/></view>
        </view>
        <view class="form-section platform-section platform-announcement">
          <view class="platform-heading"><Icon name="bell" :size="20"/><text>平台公告</text></view>
          <view class="field"><view class="platform-field-line"><text class="field-label">公告内容</text><text class="platform-counter">{{ charCount(form.announcement) }}/300</text></view><textarea v-model="form.announcement" class="input platform-content" maxlength="300" :disabled="busy" placeholder="输入要向玩家展示的平台公告"/></view>
          <text class="subtle-note platform-note">公告为空时，玩家页不显示公告。</text>
        </view>
        <view class="action-footer platform-actions"><button class="btn platform-save" :disabled="busy||!loaded" @click="save">{{ busy?'正在保存…':'保存设置' }}</button><button class="text-btn" :disabled="busy" @click="restore">取消修改</button></view>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, init, sessionRevision } from '../../lib/store'
import { token, errorMessage } from '../../api/client'
import { getAdminSettings, updateSettings } from '../../api/admin.api'
import { notify } from '../../lib/format'
import type { Settings } from '../../types/models'

const form=reactive<Settings>({site_name:'',announcement:''})
const loading=ref(false),busy=ref(false),loaded=ref(false),error=ref('')
let visible=false,version=0,saved:Settings|null=null,loadedSession:ReturnType<typeof currentSession>|null=null

/** 捕获真实令牌、身份和会话修订，防止旧管理员草稿写入新会话。 */
function currentSession(){return{token:token(),revision:sessionRevision(),userId:state.user?.id}}
/** 只有仍拥有平台管理权限的同一会话可以接收结果。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&state.user?.role==='admin'&&session.token===token()&&session.revision===sessionRevision()&&session.userId===state.user?.id}
/** 页面隐藏、请求替换或身份变化均使未完成的结果失效。 */
function currentRequest(current:number,session:ReturnType<typeof currentSession>){return visible&&current===version&&sameSession(session)}
/** 按Unicode字符计算前后端一致的中文长度。 */
function charCount(value:string){return Array.from(value||'').length}
/** 清除上一个管理员的草稿和加载标志，不保留不可确认的设置。 */
function clearDraft(){version++;Object.assign(form,{site_name:'',announcement:''});saved=null;loadedSession=null;loaded.value=false;loading.value=false;busy.value=false;error.value=''}
/** 只通过具名管理员接口读取设置，迟到响应不覆盖新账号。 */
async function load(){const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==version)return;session=currentSession();if(state.user?.role!=='admin'){clearDraft();return}const result=await getAdminSettings();if(!currentRequest(current,session))return;Object.assign(form,{site_name:result.site_name||'',announcement:result.announcement||''});saved={...form};loadedSession=session;loaded.value=true}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 取消当前公开草稿，恢复本次已确认的服务端设置。 */
function restore(){if(busy.value||!sameSession(loadedSession)||!saved)return;Object.assign(form,saved);notify('未保存的修改已取消')}
/** 校验、保存平台文字并刷新全局公开配置，保存和刷新失败分别提示。 */
async function save(){if(busy.value||!loaded.value||!visible||!sameSession(loadedSession))return;const body:Settings={site_name:form.site_name.trim(),announcement:form.announcement.trim()};if(charCount(body.site_name)<2||charCount(body.site_name)>24)return notify('平台名称需为2—24字');if(charCount(body.announcement)>300)return notify('平台公告最多300字');const current=++version,session=currentSession();busy.value=true;try{const result=await updateSettings(body);if(!currentRequest(current,session))return;Object.assign(form,{site_name:result.site_name||'',announcement:result.announcement||''});saved={...form};try{await init(true);if(currentRequest(current,session))notify('平台设置已保存')}catch(err){if(currentRequest(current,session))notify('设置已保存，全局配置刷新失败：'+errorMessage(err))}}catch(err){if(currentRequest(current,session))notify(err)}finally{if(current===version)busy.value=false}}
/** 返回相同管理员页面时保留草稿，否则重新读取真实配置。 */
function showPage(){visible=true;if(loaded.value&&sameSession(loadedSession))return;void load()}
/** 离开时作废未完成请求，下次进入后重新核实未确认的保存结果。 */
function hidePage(){visible=false;version++;if(loading.value||busy.value)loaded.value=false;loading.value=false;busy.value=false}
/** 账号变化立即清空旧草稿，并在页面可见时重新授权读取。 */
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clearDraft();if(visible)void load()})
onShow(showPage)
onHide(hidePage)
onUnload(hidePage)
</script>
<style scoped>
.platform-section{padding:24px 0}.platform-heading{display:flex;align-items:center;gap:12px;margin-bottom:24px;color:var(--text);font-size:17px;font-weight:650;line-height:24px}.platform-field-line{display:flex;align-items:center;justify-content:space-between;gap:8px;min-width:0}.platform-field-line .field-label{font-size:14px}.platform-counter{flex-shrink:0;font-size:12px;color:var(--muted)}.platform-name{font-size:16px}.platform-content{height:156px;min-height:156px;font-size:14px;line-height:22px}.platform-announcement{border-bottom:0}.platform-note{padding:4px 0 0;font-size:12px}.platform-actions{flex-direction:column;gap:16px;padding-top:8px}.platform-save{flex:0 0 auto;width:100%}
</style>
