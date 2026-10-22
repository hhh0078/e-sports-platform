<!-- 文件作用：按先行设计供平台管理员定向或广播纯文本消息，确认范围后发送并隔离旧会话回调。 -->
<template>
  <AppShell title="发送平台消息" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="平台消息仅能由平台管理员发送。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading" class="loading-line">正在确认管理员身份…</view>
      <template v-else-if="authorized&&!error">
        <view class="form-section notice-section"><view class="notice-heading"><Icon name="user" :size="20"/><text>接收对象</text></view><view class="field"><text class="field-label">用户ID（选填）</text><input v-model="form.recipient_id" class="input notice-recipient" maxlength="128" :disabled="sending" placeholder="留空发送给全部用户"/></view><text class="subtle-note notice-hint">留空为全站广播，请确认内容后发送。</text></view>
        <view class="form-section notice-section notice-content"><view class="notice-heading"><Icon name="bell" :size="20"/><text>消息内容</text></view><view class="field"><view class="notice-field-line"><text class="field-label">标题</text><text class="notice-counter">{{ charCount(form.title) }}/60</text></view><input v-model="form.title" class="input notice-title" maxlength="60" :disabled="sending" placeholder="输入消息标题"/></view><view class="field"><view class="notice-field-line"><text class="field-label">正文</text><text class="notice-counter">{{ charCount(form.body) }}/5000</text></view><textarea v-model="form.body" class="input notice-body" maxlength="5000" :disabled="sending" placeholder="输入完整消息内容"/></view></view>
        <view class="action-footer notice-actions"><button class="btn notice-send" :disabled="sending" @click="send">{{ sending?'正在发送…':'发送消息' }}</button><button class="text-btn" :disabled="sending" @click="reset">取消修改</button></view>
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
import Icon from '../../components/Icon.vue'
import { state,init,sessionRevision } from '../../lib/store'
import { token,errorMessage } from '../../api/client'
import { sendAdminMessage } from '../../api/messages.api'
import { notify } from '../../lib/format'

const form=reactive({recipient_id:'',title:'',body:''}),loading=ref(false),authorized=ref(false),sending=ref(false),error=ref('')
let visible=false,version=0,loadedSession:ReturnType<typeof currentSession>|null=null
/** 捕获当前管理员会话，发送确认也绑定原始身份。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 只有相同平台管理员可发送或接收回调，客户端角色不替代服务端校验。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 中文与扩展字符按真实字符个数校验。 */
function charCount(value:string){return Array.from(value).length}
/** 用户ID采用UTF-8字节上限，拒绝多字节输入超过后端限制。 */
function byteCount(value:string){return Array.from(value).reduce((total,char)=>{const code=char.codePointAt(0)||0;return total+(code<=0x7f?1:code<=0x7ff?2:code<=0xffff?3:4)},0)}
/** 身份变化及离页清空消息草稿，避免跨管理员恢复待发送内容。 */
function clear(){version++;Object.assign(form,{recipient_id:'',title:'',body:''});authorized.value=false;loading.value=false;sending.value=false;error.value='';loadedSession=null}
/** 首次进入与错误重试均先完成真实身份初始化。 */
async function load(){const current=++version;loading.value=true;error.value='';try{await init();if(!visible||current!==version)return;authorized.value=state.user?.role==='admin';loadedSession=authorized.value?currentSession():null}catch(err){if(visible&&current===version)error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 校验输入并固定发送快照，广播不会误带空ID。 */
function messageBody(){const recipient=form.recipient_id.trim(),title=form.title.trim(),body=form.body.trim();if(byteCount(recipient)>128)throw new Error('用户ID最多128字节');if(charCount(title)<1||charCount(title)>60)throw new Error('消息标题需为1—60字');if(charCount(body)<1||charCount(body)>5000)throw new Error('消息正文需为1—5000字');return{...(recipient?{recipient_id:recipient}:{}),title,body}}
/** 官方确认框明确接收范围，取消、隐藏或换号后绝不继续发送。 */
async function send(){if(sending.value||!authorized.value||!sameSession(loadedSession))return;let body:ReturnType<typeof messageBody>;try{body=messageBody()}catch(err){notify(err);return}const current=++version,session=currentSession();sending.value=true;try{const confirmation=await uni.showModal({title:body.recipient_id?'确认发送消息':'确认全站广播',content:body.recipient_id?`向用户“${body.recipient_id}”发送“${body.title}”？`:`向全部有效用户广播“${body.title}”？请确认正文内容。`,confirmText:body.recipient_id?'确认发送':'确认广播',confirmColor:'#35865c'});if(!confirmation.confirm||current!==version||!sameSession(session))return;const result=await sendAdminMessage(body);if(current!==version||!sameSession(session))return;Object.assign(form,{recipient_id:'',title:'',body:''});notify(`已发送给${result.recipient_count}位用户，送达不代表已读`)}catch(err){if(current===version&&sameSession(session))notify(err)}finally{if(current===version)sending.value=false}}
/** 取消修改仅清空本次消息，不发送网络请求。 */
function reset(){if(sending.value||!sameSession(loadedSession))return;Object.assign(form,{recipient_id:'',title:'',body:''});notify('未发送的修改已取消')}
/** 同账号返回若仍有可用授权则显示空表单，否则重新初始化。 */
function showPage(){visible=true;if(authorized.value&&sameSession(loadedSession))return;void load()}
/** 离页立即撤销未完成确认和发送回调并清空本次内容。 */
function hidePage(){visible=false;clear()}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()})
onShow(showPage)
onHide(hidePage)
onUnload(hidePage)
</script>
<style scoped>
.notice-section{padding:24px 0}.notice-heading{display:flex;align-items:center;gap:12px;margin-bottom:24px;color:var(--text);font-size:17px;font-weight:650;line-height:24px}.notice-field-line{display:flex;align-items:center;justify-content:space-between;gap:8px;min-width:0}.notice-counter{flex-shrink:0;font-size:12px;color:var(--muted)}.notice-recipient,.notice-title{font-size:16px}.notice-body{height:180px;min-height:180px;font-size:16px;line-height:24px}.notice-content{border-bottom:0}.notice-hint{padding:0;font-size:12px}.notice-actions{flex-direction:column;gap:16px;padding-top:8px}.notice-send{flex:0 0 auto;width:100%}
</style>
