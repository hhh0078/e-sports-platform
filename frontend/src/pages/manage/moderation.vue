<!-- 文件作用：按先行设计提供平台图文人工审核，独立展示待审任务、内容详情和可追溯处理结果。 -->
<template>
  <AppShell title="图文审核" back>
    <view v-if="error" class="error-box moderation-error"><text>{{ error }}</text><button class="text-btn" :disabled="loading" @click="retryFailed">重新加载</button></view>
    <view v-if="loading&&!authorized" class="loading-line">正在核验管理员身份…</view>
    <EmptyState v-else-if="!authorized&&!error" title="仅平台管理员可访问" detail="审核原文和操作记录仅对平台管理员开放。" icon="shield"><button class="text-btn" @click="go('/pages/account/login')">切换账号</button></EmptyState>
    <template v-if="authorized">
      <text class="moderation-note">发布成功后进入审核，人工与 AI 可接续处理。</text>
      <view class="moderation-metrics"><view v-for="item in metrics" :key="item.key" class="moderation-metric"><text class="metric-label">{{ item.label }}</text><text :class="['metric-value',{'metric-rejected':item.key==='rejected'}]">{{ counts?counts[item.key]:'—' }}</text></view></view>
      <scroll-view scroll-x class="moderation-filters" :show-scrollbar="false"><view class="moderation-filter-row"><button v-for="item in filters" :key="item.value" :class="['moderation-filter',{active:status===item.value}]" :disabled="busy" @click="chooseStatus(item.value)">{{ item.label }}</button></view></scroll-view>
      <view v-if="loading&&!tasks.length" class="loading-line">正在读取审核队列…</view>
      <button v-for="item in tasks" :key="item.id" class="moderation-row" :disabled="busy" @click="openTask(item.id)"><view class="moderation-kind"><Icon :name="item.kind==='image'?'image':'receipt'" :size="21"/></view><view class="moderation-row-copy"><view class="moderation-row-heading"><text class="moderation-field">{{ item.field_label }}</text><text :class="['moderation-status',statusClass(item.status)]">{{ statusLabel(item.status) }}</text></view><text class="moderation-owner">{{ item.target_type }} · {{ item.target_id }}</text><text class="moderation-preview">{{ item.kind==='image'?'图片内容 · 点击安全查看':item.preview||'无文字内容' }}</text><text class="moderation-time">{{ time(item.created_at) }}</text></view><Icon name="arrow" :size="13"/></button>
      <EmptyState v-if="!loading&&!tasks.length&&!error" title="当前没有审核任务" detail="新的用户图文提交后会出现在这里。" icon="shield"/>
      <view v-if="tasks.length" class="moderation-pagination"><text class="hint">已展示 {{ tasks.length }} 条</text><ListLoadState :loading="loading" :has-more="!!nextCursor" :error="error" @retry="retryFailed"/></view>
    </template>
    <FinancePopup :opened="detailOpened" title="审核详情" :busy="busy" @close="closeDetail">
      <view v-if="detailError" class="error-box"><text>{{ detailError }}</text><button class="text-btn" @click="selectedId&&openTask(selectedId)">重新读取</button></view>
      <view v-if="detailLoading" class="loading-line">正在读取审核内容…</view>
      <template v-else-if="selected">
        <view class="detail-heading"><text class="detail-field">{{ selected.field_label }}</text><text :class="['moderation-status',statusClass(selected.status)]">{{ statusLabel(selected.status) }}</text></view>
        <view class="detail-metadata"><text class="detail-meta">对象 {{ selected.target_type }} · {{ selected.target_id }}</text><text class="detail-meta">内容版本 {{ selected.content_version }} · {{ time(selected.created_at) }}</text><text class="detail-meta">审核来源 {{ decisionSource(selected.decision_source) }}</text></view>
        <text v-if="selected.kind==='text'" class="detail-content">{{ selected.preview||'无文字内容' }}</text>
        <image v-else-if="imagePath" class="detail-image" :src="imagePath" mode="aspectFit" @click="previewImage"/>
        <text v-else-if="imageLoading" class="subtle-note">正在安全读取图片…</text>
        <view v-else-if="imageError" class="image-error"><text>{{ imageError }}</text><button class="text-btn" @click="loadImage(selected)">重试图片</button></view>
        <text v-if="selected.reason" class="detail-reason">处理说明：{{ selected.reason }}</text>
        <text v-if="selected.last_error" class="detail-ai-error">AI 暂未完成：{{ selected.last_error }}。可继续人工处理。</text>
        <template v-if="canDecide"><view class="reason-label"><text>拒绝原因 · 拒绝时必填</text><text>{{ Array.from(reason.trim()).length }}/500</text></view><textarea v-model="reason" class="input reason-input" :maxlength="-1" placeholder="说明具体问题，1—500字，用户会收到此原因" :disabled="busy"/><view class="decision-actions"><button class="btn btn-secondary reject-action" :disabled="busy" @click="decide('reject')">拒绝</button><button class="btn allow-action" :disabled="busy" @click="decide('allow')">通过</button></view><button v-if="selected.status==='manual_pending'" class="text-btn retry-ai" :disabled="busy" @click="retryAI">请求 AI 重试</button></template>
        <text v-else class="detail-readonly">{{ selected.status==='superseded'?'原内容已经更新，本任务仅供追溯。':selected.status==='ai_processing'?'AI 正在处理，请稍后重新读取任务。':'本任务已处理，仅供追溯。' }}</text>
      </template>
    </FinancePopup>
  </AppShell>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import ListLoadState from '../../components/ListLoadState.vue'
import {useNativePageRefresh} from '../../lib/native-page-refresh'
import {appendUniqueRows} from '../../lib/list-page'
import FinancePopup from '../../components/FinancePopup.vue'
import Icon from '../../components/Icon.vue'
import { state, init, sessionRevision } from '../../lib/store'
import { token, errorMessage, errorStatus } from '../../api/client'
import { listModerationTasks, getModerationTask, decideModerationTask, retryModerationTask, getModerationImage } from '../../api/moderation.api'
import { go, notify, time } from '../../lib/format'
import type { ModerationCounts, ModerationStatus, ModerationTask } from '../../types/moderation'

const filters: Array<{value: ''|ModerationStatus;label:string}>=[{value:'',label:'全部'},{value:'manual_pending',label:'待人工'},{value:'ai_pending',label:'AI待审'},{value:'ai_processing',label:'AI处理中'},{value:'allowed',label:'已通过'},{value:'rejected',label:'已拒绝'},{value:'superseded',label:'已更新'}]
const metrics:Array<{key:ModerationStatus;label:string}>=[{key:'manual_pending',label:'待人工'},{key:'ai_processing',label:'AI处理中'},{key:'rejected',label:'已拒绝'}]
const tasks=ref<ModerationTask[]>([]),counts=ref<ModerationCounts|null>(null),nextCursor=ref(''),status=ref<''|ModerationStatus>(''),loading=ref(false),authorized=ref(false),error=ref('')
const selected=ref<ModerationTask|null>(null),selectedId=ref(''),detailOpened=ref(false),detailLoading=ref(false),detailError=ref(''),busy=ref(false),reason=ref(''),imagePath=ref(''),imageLoading=ref(false),imageError=ref('')
const canDecide=computed(()=>!!selected.value&&['ai_pending','ai_processing','manual_pending'].includes(selected.value.status))
let visible=false,listVersion=0,detailVersion=0,imageVersion=0,operationVersion=0,lastFailedAppend=false
/** 捕获完整管理员会话，异步原文和确认框不能跨身份继续。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 页面仍可见且当前账号仍为相同管理员时才允许使用结果。 */
function sameSession(session:ReturnType<typeof currentSession>){return visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 把服务端固定枚举转为用户可读状态，未知值也不开放操作。 */
function statusLabel(value:ModerationStatus){return filters.find(item=>item.value===value)?.label||'未知状态'}
/** 区分真实人工/AI、可信图片沿用、源图片撤销和系统过期，不能把已完成误标为未处理。 */
function decisionSource(value:ModerationTask['decision_source']){return{manual:'人工',ai:'AI',inherited:'沿用可信图片审核',source:'关联图片审核结果',system:'系统处理','':'尚未处理'}[value]||'未知来源'}
/** 仅返回固定样式类，不把用户内容拼入样式。 */
function statusClass(value:ModerationStatus){return value==='rejected'?'status-rejected':value==='allowed'?'status-allowed':value==='superseded'?'status-archived':'status-pending'}
/** 释放仅管理员可读的临时图片，路径从不写入持久存储。 */
function removeImage(path:string){if(!path)return;try{const manager=uni.getFileSystemManager?.();manager?.unlink({filePath:path,fail:()=>{}})}catch{/* 临时文件可能已被系统清理，不影响撤销界面。 */}}
/** 清除详情和在途回调，关闭后不继续保留用户原文。 */
function closeDetail(){if(busy.value)return;detailVersion++;imageVersion++;detailOpened.value=false;selected.value=null;selectedId.value='';reason.value='';detailError.value='';detailLoading.value=false;imageLoading.value=false;imageError.value='';removeImage(imagePath.value);imagePath.value=''}
/** 请求队列前先核验身份；状态切换使旧分页请求失效。 */
async function loadList(more=false){if(!visible||busy.value||more&&(loading.value||!nextCursor.value))return;const current=++listVersion;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|undefined;try{await init();if(!visible||current!==listVersion)return;authorized.value=state.user?.role==='admin';if(!authorized.value){tasks.value=[];counts.value=null;return}session=currentSession();const data=await listModerationTasks({...(status.value?{status:status.value}:{}),...(more?{cursor:nextCursor.value}:{}),limit:20});if(current!==listVersion||!sameSession(session))return;tasks.value=appendUniqueRows(more?tasks.value:[],data.items);nextCursor.value=data.next_cursor||'';counts.value=data.counts}catch(err){if(visible&&current===listVersion&&(!session||sameSession(session))){lastFailedAppend=more;error.value=errorMessage(err)}}finally{if(current===listVersion)loading.value=false}}
/** 所有队列错误入口恢复失败的首读或原游标追加，不重放人工审核。 */
async function retryFailed(){if(loading.value||busy.value)return;await loadList(lastFailedAppend)}
/** 切换状态保留服务端过滤语义，不对已加载页做局部假筛选。 */
function chooseStatus(value:''|ModerationStatus){if(busy.value||status.value===value)return;closeDetail();status.value=value;tasks.value=[];nextCursor.value='';void loadList()}
/** 从固定同源管理员端点下载图片，账号变化后立即丢弃临时文件。 */
async function loadImage(task:ModerationTask){const current=++imageVersion,detail=detailVersion,session=currentSession();imageLoading.value=true;imageError.value='';try{const path=await getModerationImage(task.id);if(current!==imageVersion||detail!==detailVersion||!sameSession(session)||selected.value?.id!==task.id){removeImage(path);return}removeImage(imagePath.value);imagePath.value=path}catch(err){if(current===imageVersion&&detail===detailVersion&&sameSession(session))imageError.value=errorMessage(err)}finally{if(current===imageVersion)imageLoading.value=false}}
/** 读取完整内容而非用列表摘要作审核依据。 */
async function openTask(id:string){if(busy.value||!authorized.value||!sameSession(currentSession()))return;closeDetail();const current=++detailVersion,session=currentSession();selectedId.value=id;detailOpened.value=true;detailLoading.value=true;try{const task=await getModerationTask(id);if(current!==detailVersion||!sameSession(session))return;selected.value=task;if(task.kind==='image')void loadImage(task)}catch(err){if(current===detailVersion&&sameSession(session))detailError.value=errorMessage(err)}finally{if(current===detailVersion)detailLoading.value=false}}
/** 原生预览只使用本次已鉴权读取的临时文件。 */
function previewImage(){if(imagePath.value&&authorized.value)uni.previewImage({urls:[imagePath.value],current:imagePath.value})}
/** 人工决定前要求明确确认；过期任务由服务端409拒绝并重新读取。 */
async function decide(decision:'allow'|'reject'){if(busy.value||!canDecide.value||!selected.value||!sameSession(currentSession()))return;const text=reason.value.trim(),size=Array.from(text).length;if(size>500||decision==='reject'&&size<1)return notify('拒绝原因需要1—500字');const task=selected.value,session=currentSession(),operation=++operationVersion,detail=detailVersion;busy.value=true;try{const confirmation=await uni.showModal({title:decision==='reject'?'确认拒绝内容':'确认通过内容',content:(task.status==='ai_processing'?'AI正在处理中，先完成的有效审核结果生效；若状态已更新，请重新读取。\n':'')+(decision==='reject'?`将替换违规字段并通知用户。原因：${text}`:'确认已经查看完整内容，允许保留该字段？'),confirmText:decision==='reject'?'确认拒绝':'确认通过',confirmColor:decision==='reject'?'#b65e49':'#35865c'});if(!confirmation.confirm||operation!==operationVersion||detail!==detailVersion||!sameSession(session))return;const result=await decideModerationTask(task.id,decision,decision==='reject'?text:'');if(operation!==operationVersion||detail!==detailVersion||!sameSession(session))return;selected.value=result;reason.value='';notify(decision==='reject'?'已拒绝并通知用户':'已通过')}catch(err){if(operation===operationVersion&&sameSession(session)){detailError.value=errorMessage(err);if(errorStatus(err)===409){selected.value=null;detailError.value+='，请重新读取最新内容。'}}}finally{if(operation===operationVersion){busy.value=false;if(sameSession(session))void loadList()}}}
/** AI重试只作用于待人工任务，无配置时仍保留人工操作。 */
async function retryAI(){if(busy.value||selected.value?.status!=='manual_pending'||!sameSession(currentSession()))return;const task=selected.value,session=currentSession(),operation=++operationVersion,detail=detailVersion;busy.value=true;try{const result=await retryModerationTask(task.id);if(operation!==operationVersion||detail!==detailVersion||!sameSession(session))return;selected.value=result;notify(result.status==='manual_pending'?'AI暂不可用，任务继续等待人工审核':'已加入AI审核队列')}catch(err){if(operation===operationVersion&&sameSession(session)){detailError.value=errorMessage(err);if(errorStatus(err)===409)selected.value=null}}finally{if(operation===operationVersion){busy.value=false;if(sameSession(session))void loadList()}}}
/** 隐藏或换号清空原文与临时文件，使旧确认和回调失效。 */
function clearPage(){lastFailedAppend=false;listVersion++;operationVersion++;busy.value=false;closeDetail();tasks.value=[];counts.value=null;nextCursor.value='';authorized.value=false;loading.value=false;error.value=''}
/** 显示时重新核验真实权限，不使用缓存的管理员身份访问内容。 */
function showPage(){visible=true;void loadList()}
/** 隐藏后撤销所有在途UI结果；已提交人工决定仍由服务端原任务保证唯一性。 */
function hidePage(){visible=false;clearPage()}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clearPage();if(visible)void loadList()})
useNativePageRefresh({refresh:()=>loadList(),more:()=>error.value?retryFailed():loadList(true),context:()=>[state.user?.id,state.user?.role,sessionRevision(),status.value],blocked:()=>busy.value||detailOpened.value})
onShow(showPage)
onHide(hidePage)
onUnload(hidePage)
</script>
<style scoped>
.moderation-error{display:flex;flex-direction:column;gap:15.3846rpx}.moderation-note{display:block;color:var(--muted);font-size:25rpx;line-height:42.3077rpx;margin:7.6923rpx 0 30.7692rpx}.moderation-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;padding:23.0769rpx;background:var(--surface);border-radius:15.3846rpx}.moderation-metric{display:flex;flex-direction:column;align-items:center;gap:11.5385rpx;min-width:0}.metric-label{font-size:23.0769rpx;color:var(--muted)}.metric-value{font-size:42.3077rpx;line-height:57.6923rpx;color:var(--accent);overflow-wrap:anywhere}.metric-rejected{color:#e4a18d}.moderation-filters{width:100%;max-width:100%;margin:23.0769rpx 0 0;white-space:nowrap}.moderation-filter-row{display:inline-flex;gap:30.7692rpx;min-width:100%;border-bottom:1px solid var(--line-subtle)}.moderation-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:44px;padding:0;color:var(--muted);background:transparent;border-radius:0;font-size:25rpx;line-height:42.3077rpx;border-bottom:3.8462rpx solid transparent}.moderation-filter.active{color:var(--accent);border-bottom-color:var(--accent)}.moderation-row{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-width:0;padding:30.7692rpx 0;text-align:left;background:transparent;border-radius:0;border-bottom:1px solid var(--line-subtle);color:var(--muted)}.moderation-kind{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:69.2308rpx;height:76.9231rpx;border-radius:15.3846rpx;background:var(--surface);color:var(--text)}.moderation-row-copy{flex:1;min-width:0}.moderation-row-heading{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.moderation-field{font-size:28.8462rpx;line-height:42.3077rpx;color:var(--text);min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.moderation-status{flex-shrink:0;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted)}.status-rejected{color:#e4a18d}.status-allowed,.status-pending{color:var(--accent)}.status-archived{color:var(--muted)}.moderation-owner,.moderation-preview,.moderation-time{display:block;max-width:100%;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:21.1538rpx;line-height:34.6154rpx;color:var(--muted)}.moderation-pagination{display:flex;align-items:center;justify-content:center;gap:23.0769rpx;padding:30.7692rpx 0;flex-wrap:wrap}.detail-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.detail-field{font-size:30.7692rpx;line-height:46.1538rpx;color:var(--text);overflow-wrap:anywhere}.detail-metadata{display:flex;flex-direction:column;gap:7.6923rpx;padding:23.0769rpx 0;margin-bottom:23.0769rpx;border-bottom:1px solid var(--line-subtle)}.detail-meta{font-size:21.1538rpx;line-height:34.6154rpx;color:var(--muted);overflow-wrap:anywhere}.detail-content{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:26.9231rpx;line-height:46.1538rpx}.detail-image{display:block;width:100%;height:461.5385rpx;background:var(--bg);border-radius:15.3846rpx}.detail-reason,.detail-ai-error,.detail-readonly{display:block;margin:23.0769rpx 0;font-size:23.0769rpx;line-height:42.3077rpx;white-space:pre-wrap;overflow-wrap:anywhere;color:var(--muted)}.detail-reason,.detail-ai-error,.image-error{color:#e4a18d}.reason-label{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;margin-top:30.7692rpx;color:var(--muted);font-size:23.0769rpx;line-height:42.3077rpx}.reason-input{height:153.8462rpx;min-height:153.8462rpx;margin-top:15.3846rpx;font-size:26.9231rpx;line-height:42.3077rpx}.decision-actions{display:flex;gap:23.0769rpx;margin-top:23.0769rpx}.reject-action,.allow-action{flex:1;min-width:0}.reject-action{border:1px solid #9d6250;color:#e4a18d}.retry-ai{margin:15.3846rpx auto 0;font-size:25rpx}.image-error{display:flex;flex-direction:column;gap:15.3846rpx;min-width:0;font-size:25rpx;line-height:42.3077rpx}
</style>
