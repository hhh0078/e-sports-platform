<!-- 文件作用：独立个人资料设置页，更新头像、昵称、简介与主页封面，并管理会话和手机号验证。 -->
<template>
  <AppShell title="个人资料" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <view v-if="!state.user" class="guest-note" @click="go('/pages/account/login?next='+encodeURIComponent('/pages/account/profile'))">登录后查看和完善个人资料<Icon name="arrow" :size="13"/></view>
    <view class="avatar-editor"><button class="profile-avatar" :disabled="!state.user||saving||uploading" aria-label="选择微信头像" open-type="chooseAvatar" @chooseavatar="chooseWechatAvatar" @click="pickAvatarForWeb"><image class="avatar-image" v-if="form.avatar" :src="assetUrl(form.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="28"/><view class="camera-badge"><Icon name="photo" :size="12"/></view></button><view class="avatar-copy"><button class="text-btn change-avatar" open-type="chooseAvatar" :disabled="!state.user||saving||uploading" @chooseavatar="chooseWechatAvatar" @click="pickAvatarForWeb">微信头像<Icon name="arrow" :size="12"/></button><button class="text-btn change-avatar" :disabled="!state.user||saving||uploading" @click="upload('avatar')">{{uploading?'图片处理中…':'选择图片'}}<Icon name="arrow" :size="12"/></button><text class="hint">头像1:1，自动裁剪压缩</text></view></view>
    <view class="form-section profile-fields"><view class="field"><text class="field-label">昵称</text><input v-model="form.nickname" :disabled="!state.user" type="nickname" class="input" maxlength="24" placeholder="2—24个字的玩家昵称"/></view><view class="field"><view class="field-heading"><text class="field-label">一句话介绍</text><text class="field-count">{{form.bio.length}} / 300</text></view><textarea v-model="form.bio" :disabled="!state.user" class="input bio-input" maxlength="300" placeholder="擅长什么游戏，哪些时段在线？"/></view></view>
    <view class="profile-setting"><text class="field-label">手机号验证</text><button class="settings-row" :disabled="!state.user" @click="verifyPhone"><Icon name="phone" :size="20"/><text class="settings-value">{{state.user?.phone_verified?'已验证 '+(state.user.phone_masked||'手机号'):'验证后可下单和发布'}}</text><Icon name="arrow" :size="13"/></button></view>
    <button class="settings-row wallet-entry" @click="go(state.user?'/pages/account/wallet':'/pages/account/login?next='+encodeURIComponent('/pages/account/wallet'))"><Icon name="wallet" :size="20"/><text class="settings-value">我的钱包</text><Icon name="arrow" :size="13"/></button>

    <view class="form-section cover-settings"><view class="field-heading"><text class="field-label">个人主页封面</text><button class="text-btn" :disabled="!state.user||uploading||saving" @click="upload('profile_cover')">{{form.profile_cover?'更换封面':'选择封面'}}<Icon name="arrow" :size="12"/></button></view><view v-if="form.profile_cover" class="profile-cover"><image class="profile-cover-image" :src="assetUrl(form.profile_cover)" mode="aspectFill"/></view><text v-else class="cover-empty">未设置个人主页封面 · 16:9</text></view>
    <view class="action-footer profile-actions"><button class="btn profile-save" :disabled="!state.user||saving||uploading" @click="saveProfile">{{saving?'保存中…':'保存资料'}}</button><button class="text-btn" :disabled="!state.user||saving||uploading" @click="cancel">取消修改</button></view>
    <button v-if="state.user" class="public-link text-btn" @click="go('/pages/community/players/detail?id='+state.user.id)">查看我的公开主页<Icon name="arrow" :size="12"/></button><button v-if="state.user" class="logout-button text-btn" :disabled="saving||uploading" @click="signOut">退出登录</button>
  </AppShell>
</template>
<script setup lang="ts">
import { ref, reactive } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state, init, logout, clearSession, sessionRevision } from '../../lib/store'
import { getMe, updateProfile } from '../../api/users.api'
import { token, errorMessage, errorStatus, isCancelled } from '../../api/client'
import { chooseUploadImage } from '../../lib/media'
import { requirePhone } from '../../lib/phone'
import { go, notify, assetUrl } from '../../lib/format'
const form=reactive({nickname:'',bio:'',avatar:'',profile_cover:''}),error=ref(''),saving=ref(false),uploading=ref(false)
let version=0,visible=true,disposed=false,savedForm='',loadedSession: ReturnType<typeof currentSession> | null=null
/** 捕获资料操作所属账号和会话。 */
function currentSession(){return{token:token(),userId:state.user?.id,revision:sessionRevision()}}
/** 拒绝切换身份后的旧请求更新当前资料。 */
function sameSession(session: ReturnType<typeof currentSession>){return session.token===token()&&session.userId===state.user?.id&&session.revision===sessionRevision()}
/** 仅允许当前可见资料页接收对应版本的响应。 */
function currentRequest(current: number, session: ReturnType<typeof currentSession>){return visible&&current===version&&sameSession(session)}
/** 恢复用户已保存的资料，取消不发送写请求。 */
function resetForm(){Object.assign(form,{nickname:state.user?.nickname||'',bio:state.user?.bio||'',avatar:state.user?.avatar||'',profile_cover:state.user?.profile_cover||''});savedForm=JSON.stringify(form);loadedSession=currentSession()}
/** 加载最新资料，同时保护随后发生的保存及身份切换。 */
async function load(){const current=++version;let session: ReturnType<typeof currentSession> | undefined;error.value='';try{await init();if(!visible||current!==version||!state.user)return;session=currentSession();const user=await getMe();if(!currentRequest(current,session))return;state.user=user;resetForm()}catch(err){if(!visible||current!==version||(session&&!sameSession(session)))return;if(errorStatus(err)===401)clearSession();error.value=errorMessage(err)}}
/** 官方裁剪压缩后上传图片，账号或页面已改变时不回填草稿。 */
async function upload(purpose: 'avatar' | 'profile_cover',filePath?:string){if(uploading.value||saving.value)return;const session=currentSession();version++;uploading.value=true;try{const url=await chooseUploadImage(purpose,{filePath});if(!disposed&&sameSession(session)&&url)form[purpose==='avatar'?'avatar':'profile_cover']=url}catch(err){if(!disposed&&sameSession(session)&&!isCancelled(err))notify(err)}finally{uploading.value=false}}
/** 微信头像使用官方临时文件，统一裁剪压缩上传后才更新草稿。 */
async function chooseWechatAvatar(event:{detail:{avatarUrl?:string}}):Promise<void>{const filePath=event.detail?.avatarUrl;if(filePath)await upload('avatar',filePath)}
/** 网页预览头像走现有相册入口，小程序由chooseAvatar原生弹层处理。 */
function pickAvatarForWeb():void{
  // #ifdef H5
  void upload('avatar')
  // #endif
}
/** 检查昵称和简介，只保存显式选择的四项公开资料。 */
async function saveProfile(){if(saving.value||uploading.value||!state.user)return;const nickname=form.nickname.trim(),bio=form.bio.trim();if(Array.from(nickname).length<2||Array.from(nickname).length>24)return notify('昵称需要 2—24 个字');if(Array.from(bio).length>300)return notify('个人介绍不能超过 300 个字');const current=++version,session=currentSession();saving.value=true;try{const user=await updateProfile({nickname,bio,avatar:form.avatar,profile_cover:form.profile_cover});if(!currentRequest(current,session))return;state.user=user;resetForm();notify('资料已保存')}catch(err){if(currentRequest(current,session))notify(err)}finally{saving.value=false}}
/** 放弃未保存修改并回填服务端资料。 */
function cancel(){resetForm();notify('未保存的修改已取消')}
/** 请求手机号验证或显示已完成验证的脱敏号码。 */
async function verifyPhone(){if(state.user?.phone_verified)return notify('已验证 '+(state.user.phone_masked||'手机号'));await requirePhone()}
/** 撤销当前会话，迟到回调不能退出后来登录的账号。 */
async function signOut(){if(saving.value)return;version++;saving.value=true;try{await logout();if(!state.user&&!token()){resetForm();go('/pages/main/index?tab=me')}}catch(err){notify(err)}finally{saving.value=false}}
/** 原生选图返回时保留当前草稿，仅在身份变化或没有未保存内容时刷新。 */
function showPage(){visible=true;if(loadedSession&&sameSession(loadedSession)&&(uploading.value||JSON.stringify(form)!==savedForm))return;load()}
/** 页面隐藏后使读取与保存回调失效。 */
function hidePage(){visible=false;version++}
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.guest-note{display:flex;align-items:center;justify-content:center;gap:7.6923rpx;margin-bottom:30.7692rpx;color:var(--muted);font-size:23.0769rpx}.avatar-editor{display:flex;align-items:center;gap:30.7692rpx;padding:15.3846rpx 0 46.1538rpx;border-bottom:1px solid var(--line-subtle)}.profile-avatar{position:relative;display:flex;align-items:center;justify-content:center;overflow:visible;flex-shrink:0;width:107.6923rpx;height:107.6923rpx;padding:0;border:1px solid var(--line);border-radius:50%;background:var(--surface);color:var(--accent)}.avatar-image{display:block;width:100%;height:100%;border-radius:50%}.camera-badge{position:absolute;right:-3.8462rpx;bottom:-3.8462rpx;display:flex;align-items:center;justify-content:center;width:42.3077rpx;height:42.3077rpx;background:var(--accent);color:var(--bg);border-radius:50%;border:3.8462rpx solid var(--bg)}.avatar-copy{display:flex;flex-direction:column;gap:7.6923rpx;min-width:0}.change-avatar{justify-content:flex-start;padding:0}.profile-fields{padding:46.1538rpx 0}.field-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.field-count{font-size:19.2308rpx;color:var(--muted);white-space:nowrap}.bio-input{height:184.6154rpx}.profile-setting{padding:30.7692rpx 0;border-bottom:1px solid var(--line-subtle)}.settings-row{display:flex;align-items:center;gap:23.0769rpx;min-height:44px;width:100%;padding:15.3846rpx 0;border:0;border-radius:0;background:transparent;color:var(--text);font-size:25rpx;text-align:left}.settings-value{flex:1;min-width:0}.wallet-entry{padding:23.0769rpx 0;border-bottom:1px solid var(--line-subtle)}.provider-row{display:flex;align-items:center;gap:23.0769rpx;min-height:130.7692rpx;padding:30.7692rpx 0;border-bottom:1px solid var(--line-subtle);color:var(--text)}.provider-copy{display:flex;flex:1;flex-direction:column;gap:5.7692rpx;min-width:0}.provider-title{font-size:25rpx;font-weight:600}.provider-default{flex-shrink:0;color:var(--accent);font-size:21.1538rpx;line-height:34.6154rpx;white-space:nowrap}.cover-settings{border-bottom:0;padding-bottom:0}.profile-cover{position:relative;width:100%;height:0;padding-top:56.25%;border-radius:15.3846rpx;margin-top:23.0769rpx;overflow:hidden}.profile-cover-image{position:absolute;inset:0;display:block;width:100%;height:100%}.cover-empty{display:block;padding:23.0769rpx 0 0;color:var(--muted);font-size:23.0769rpx;line-height:38.4615rpx}.profile-actions{flex-direction:column;align-items:stretch;gap:15.3846rpx;padding-top:46.1538rpx}.profile-save{flex:none;width:100%}.public-link,.logout-button{display:flex;width:100%;margin-top:15.3846rpx;font-size:21.1538rpx}.logout-button{color:var(--muted)}
</style>
