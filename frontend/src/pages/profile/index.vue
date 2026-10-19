<!-- 文件作用：独立个人资料设置页，更新头像、昵称、简介与主页封面，并管理会话和手机号验证。 -->
<template><AppShell title="个人资料" subtitle="认真介绍自己，让合拍的队友找到你" back><view v-if="error" class="error-box mb">{{ error }}</view><view v-if="!state.user" class="guest-note" @click="go('/pages/login/index?next='+encodeURIComponent('/pages/profile/index'))">登录后查看和完善个人资料 <Icon name="arrow" :size="12"/></view><view class="profile-form panel"><button class="avatar-row" :disabled="!state.user||saving||uploading" @click="upload('avatar')"><text class="row-label">头像</text><image v-if="form.avatar" :src="assetUrl(form.avatar)" mode="aspectFill"/><view v-else class="empty-avatar"><Icon name="user" :size="26"/></view><Icon name="arrow" :size="15"/></button><view class="field"><text class="field-label">昵称 · 2—24 个字</text><input v-model="form.nickname" :disabled="!state.user" type="nickname" class="input" maxlength="24" placeholder="你的玩家昵称"/></view><view class="field"><text class="field-label">个人介绍 · 最多 300 个字</text><textarea v-model="form.bio" :disabled="!state.user" class="input bio-input" maxlength="300" placeholder="擅长什么游戏，哪些时段在线？"/></view><view class="field"><view class="field-line"><text class="field-label">个人主页封面</text><button class="text-btn" :disabled="!state.user||uploading||saving" @click="upload('profile_cover')">{{ uploading ? '处理中…' : '更换封面' }}</button></view><image v-if="form.profile_cover" class="profile-cover" :src="assetUrl(form.profile_cover)" mode="aspectFill"/><view v-else class="cover-empty">使用游戏主题封面</view></view><view v-if="state.user" class="form-actions"><button class="btn-secondary" :disabled="!state.user||saving||uploading" @click="cancel">取消修改</button><button class="btn" :disabled="!state.user||saving||uploading" @click="saveProfile">{{ saving ? '保存中…' : '保存资料' }}</button></view></view><view v-if="state.user" class="settings-panel panel"><button @click="verifyPhone"><Icon name="phone" :size="20"/><text class="row-label">手机号验证</text><text class="setting-value">{{ state.user.phone_verified ? state.user.phone_masked || '已验证' : '未验证' }}</text><Icon name="arrow" :size="14"/></button><button @click="go('/pages/player/index?id='+state.user.id)"><Icon name="user" :size="20"/><text class="row-label">查看我的公开主页</text><Icon name="arrow" :size="14"/></button><button v-if="state.devMode" @click="go('/pages/login/index')"><Icon name="refresh" :size="20"/><text class="row-label">切换体验身份</text><Icon name="arrow" :size="14"/></button></view><button v-if="state.user" class="logout-button btn-secondary" :disabled="!state.user||saving||uploading" @click="signOut">退出登录</button></AppShell></template>
<script setup>
import { ref, reactive } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state, init, logout, clearSession, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { chooseUploadImage } from '../../lib/media'
import { requirePhone } from '../../lib/phone'
import { go, notify, assetUrl } from '../../lib/format'
const form=reactive({nickname:'',bio:'',avatar:'',profile_cover:''}),error=ref(''),saving=ref(false),uploading=ref(false)
let version=0,visible=true,disposed=false,savedForm='',loadedSession=null
/** 捕获资料操作所属账号和会话。 */
function currentSession(){return{token:token(),userId:state.user?.id,revision:sessionRevision()}}
/** 拒绝切换身份后的旧请求更新当前资料。 */
function sameSession(session){return session.token===token()&&session.userId===state.user?.id&&session.revision===sessionRevision()}
/** 仅允许当前可见资料页接收对应版本的响应。 */
function currentRequest(current,session){return visible&&current===version&&sameSession(session)}
/** 恢复用户已保存的资料，取消不发送写请求。 */
function resetForm(){Object.assign(form,{nickname:state.user?.nickname||'',bio:state.user?.bio||'',avatar:state.user?.avatar||'',profile_cover:state.user?.profile_cover||''});savedForm=JSON.stringify(form);loadedSession=currentSession()}
/** 加载最新资料，同时保护随后发生的保存及身份切换。 */
async function load(){const current=++version;let session;error.value='';try{await init();if(!visible||current!==version||!state.user)return;session=currentSession();const user=await api('/me');if(!currentRequest(current,session))return;state.user=user;resetForm()}catch(err){if(!visible||current!==version||(session&&!sameSession(session)))return;if(err.status===401)clearSession();error.value=err.message}}
/** 官方裁剪压缩后上传图片，账号或页面已改变时不回填草稿。 */
async function upload(purpose){if(uploading.value||saving.value)return;const session=currentSession();version++;uploading.value=true;try{const url=await chooseUploadImage(purpose);if(!disposed&&sameSession(session)&&url)form[purpose==='avatar'?'avatar':'profile_cover']=url}catch(err){if(!disposed&&sameSession(session)&&!err.cancelled)notify(err)}finally{uploading.value=false}}
/** 检查昵称和简介，只保存显式选择的四项公开资料。 */
async function saveProfile(){if(saving.value||uploading.value||!state.user)return;const nickname=form.nickname.trim(),bio=form.bio.trim();if(Array.from(nickname).length<2||Array.from(nickname).length>24)return notify('昵称需要 2—24 个字');if(Array.from(bio).length>300)return notify('个人介绍不能超过 300 个字');const current=++version,session=currentSession();saving.value=true;try{const user=await api('/me/profile','PATCH',{nickname,bio,avatar:form.avatar,profile_cover:form.profile_cover});if(!currentRequest(current,session))return;state.user=user;resetForm();notify('资料已保存')}catch(err){if(currentRequest(current,session))notify(err)}finally{saving.value=false}}
/** 放弃未保存修改并回填服务端资料。 */
function cancel(){resetForm();notify('未保存的修改已取消')}
/** 请求手机号验证或显示已完成验证的脱敏号码。 */
async function verifyPhone(){if(state.user?.phone_verified)return notify('已验证 '+(state.user.phone_masked||'手机号'));await requirePhone()}
/** 撤销当前会话，迟到回调不能退出后来登录的账号。 */
async function signOut(){if(saving.value)return;version++;saving.value=true;try{await logout();if(!state.user&&!token()){resetForm();go('/pages/me/index')}}catch(err){notify(err)}finally{saving.value=false}}
/** 原生选图返回时保留当前草稿，仅在身份变化或没有未保存内容时刷新。 */
function showPage(){visible=true;if(loadedSession&&sameSession(loadedSession)&&(uploading.value||JSON.stringify(form)!==savedForm))return;load()}
/** 页面隐藏后使读取与保存回调失效。 */
function hidePage(){visible=false;version++}
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.profile-form .avatar-row[disabled],.profile-form .text-btn[disabled]{background:transparent;color:var(--muted)}
</style>
<style scoped>.guest-note{display:flex;align-items:center;justify-content:center;gap:5px;margin:4px 0 18px;color:var(--muted);font-size:12px}.profile-form input[disabled],.profile-form textarea[disabled]{opacity:.65}.profile-form{padding:17px}.avatar-row{display:flex;align-items:center;gap:12px;width:100%;padding:0 0 17px;margin:0 0 19px;background:none;color:var(--text);border-bottom:1px solid #ffffff12;border-radius:0;text-align:left;font-size:14px;line-height:24px}.avatar-row .row-label{flex:1}.avatar-row image,.empty-avatar{width:52px;height:52px;border-radius:14px;flex-shrink:0}.empty-avatar{display:flex;align-items:center;justify-content:center;background:#ffffff08;color:var(--accent)}.bio-input{height:120px}.field-line{display:flex;align-items:center;justify-content:space-between;gap:10px}.field-line .text-btn{font-size:12px}.profile-cover{width:100%;height:175px;border-radius:9px;margin-top:7px}.cover-empty{padding:35px 12px;text-align:center;border:1px dashed #ffffff28;border-radius:9px;font-size:12px;color:#84998b}.form-actions{display:flex;gap:9px;margin-top:20px}.form-actions>button{flex:1;font-size:13px}.settings-panel{padding:0;margin-top:15px;overflow:hidden}.settings-panel>button{display:flex;align-items:center;gap:11px;width:100%;margin:0;padding:14px;color:#c6d4cb;background:none;border-radius:0;border-bottom:1px solid #ffffff12;text-align:left;font-size:13px;line-height:24px}.settings-panel>button:last-child{border-bottom:0}.settings-panel .row-label{flex:1;min-width:0}.settings-panel .setting-value{flex:0;font-size:11px;white-space:nowrap;color:#8aa897}.logout-button{width:100%;margin-top:20px;font-size:14px;color:#d0ada0}</style>
