<!-- 文件作用：首页真实公告浮层，展示成功后才确认版本已读，按账号隔离并与强制更新提示互斥。 -->
<template>
  <!-- #ifdef MP-WEIXIN -->
  <root-portal>
  <!-- #endif -->
  <view class="announcement-root" :style="pageTheme()"><UniPopup :key="popupInstance" ref="popup" type="center" background-color="transparent" mask-background-color="rgba(0,0,0,.68)" :safe-area="false" @change="changed"><view class="home-announcement" :class="{'home-announcement-image':presented?.mode==='image'}"><scroll-view scroll-y class="announcement-scroll"><image v-if="presented?.mode==='image'&&presented.image_url" class="announcement-image" :src="assetUrl(presented.image_url)" mode="widthFix" @click="navigate"/><view v-else class="announcement-text-panel" @click="navigate"><text class="announcement-title">{{presented?.title||'平台公告'}}</text><text class="announcement-body">{{presented?.body}}</text><view v-if="presented?.target" class="announcement-link">查看详情 <Icon name="arrow" :size="14"/></view></view></scroll-view><button class="announcement-close" aria-label="关闭公告" @click="close"><Icon name="close" :size="22"/><text>关闭</text></button></view></UniPopup></view>
  <!-- #ifdef MP-WEIXIN -->
  </root-portal>
  <!-- #endif -->
</template>
<script setup lang="ts">
import {ref,watch,nextTick,onBeforeUnmount} from 'vue'
import {onHide,onShow} from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import {acknowledgeAnnouncement,getAnnouncementState} from '../api/content.api'
import {state,sessionRevision,pageTheme,init} from '../lib/store'
import {assetUrl} from '../lib/format'
import {openContentTarget} from '../lib/content'
import {acquireSystemPrompt,getUpdatePromptState,onUpdatePromptStateChange} from '../lib/update-manager'
import type {ContentAnnouncement} from '../types/content'
/** 官方弹层只使用公开打开关闭方法。 */
interface Popup{open:(type?:string)=>void;close:()=>void}
const props=defineProps<{active:boolean;announcement?:ContentAnnouncement}>()
const popup=ref<Popup|null>(null),popupInstance=ref(0),presented=ref<ContentAnnouncement>()
let version=0,checking=0,pending=false,opened=false,hidden=false,disposed=false,release:(()=>void)|null=null,handledEntry='',shownUser='',shownRevision=0,shownAttempt=0,acknowledged=false
/** 访客仅记录本机公告版本，存储失败允许下一次进入重试。 */
function guestSeen(value:number):boolean{try{return uni.getStorageSync('jijie_announcement_guest_version')===value}catch{return false}}
/** 关闭本次展示并先清租约引用，避免释放锁的同步通知重入当前弹层。 */
function closePresentation():void{const wasOpen=opened;opened=false;const lease=release;release=null;try{if(wasOpen)popup.value?.close()}catch{/* 原生组件关闭异常不能占住系统提示槽。 */}finally{lease?.()}}
/** 主动关闭只结束本次进入，不把未展示公告伪装成服务端已读。 */
function close():void{version++;checking=0;pending=false;closePresentation()}
/** 当前代次、账号与版本不变时才允许展示或确认已读。 */
function current(attempt:number,user:string,revision:number,value:number):boolean{return !disposed&&!hidden&&props.active&&attempt===version&&user===(state.user?.id||'')&&revision===sessionRevision()&&value===props.announcement?.version&&!!props.announcement.enabled}
/** 只有官方弹层真正show且仍为当前账号版本，才提交一次已读。 */
function changed(event:{show:boolean}):void{
  if(!event.show){pending=false;closePresentation();return}
  const value=presented.value
  if(!opened||!value||!current(shownAttempt,shownUser,shownRevision,value.version)){close();return}
  if(acknowledged)return
  acknowledged=true;handledEntry=shownUser+':'+value.version
  if(shownUser){void acknowledgeAnnouncement(value.version).catch(()=>{/* 失败不缓存已读，下次进入重新向服务器确认。 */})}
  else{try{uni.setStorageSync('jijie_announcement_guest_version',value.version)}catch{/* 本机存储失败仅本次已展示，下一次进入可重新提醒。 */}}
}
/** 查询本次账号的未读状态；资金或更新占用时保留pending，空闲后再申请同一展示。 */
async function consider():Promise<void>{
  if(disposed||hidden||!props.active||!pending||opened||checking===version)return
  const attempt=version,value=props.announcement;if(!value?.enabled||!value.version)return
  checking=attempt
  try{
    await init();const user=state.user?.id||'',revision=sessionRevision(),entry=user+':'+value.version
    if(!current(attempt,user,revision,value.version))return
    if(handledEntry===entry){pending=false;return}
    if(value.frequency==='once'){
      const seen=user?(await getAnnouncementState(value.version)).seen:guestSeen(value.version)
      if(!current(attempt,user,revision,value.version))return
      if(seen){pending=false;handledEntry=entry;return}
    }
    const lease=acquireSystemPrompt('announcement');if(!lease)return
    release=lease;pending=false;opened=true;shownUser=user;shownRevision=revision;shownAttempt=attempt;acknowledged=false;presented.value={...value,target:value.target?{...value.target}:null};popupInstance.value++
    // 官方close保留300ms动画实例；换key后下一帧使用新实例，快速切号不会吞掉open。
    await nextTick();if(!current(attempt,user,revision,value.version)){if(release===lease)closePresentation();return}
    if(!popup.value){closePresentation();return}try{popup.value.open('center')}catch{if(release===lease)closePresentation()}
  }finally{if(checking===attempt)checking=0}
}
/** 新进入、切账号或公告换版撤销旧查询；关闭期间先关pending，避免同步释放触发自重开。 */
function reset():void{version++;checking=0;pending=false;closePresentation();handledEntry='';if(!disposed&&!hidden&&props.active){pending=true;void consider().catch(()=>{})}}
/** 仅空闲槽唤醒被延后的公告，checking/opened阻止自身抢锁通知再次查询或关闭。 */
const unsubscribe=onUpdatePromptStateChange(()=>{const status=getUpdatePromptState();if(pending&&!opened&&checking!==version&&!status.background&&!status.financialBusy&&!status.owner&&!status.applying&&!status.prompting)void consider().catch(()=>{})})
/** 点击公告先关闭再进入白名单目标，普通背景点击不执行外部URL。 */
function navigate():void{const value=presented.value,target=value?.target,live=props.announcement?.target;if(!opened||!value||!current(shownAttempt,shownUser,shownRevision,value.version)||!target||!live||target.kind!==live.kind||target.id!==live.id)return;close();openContentTarget(target)}
watch([()=>props.active,()=>props.announcement?.version,()=>props.announcement?.enabled,()=>state.user?.id,sessionRevision],reset,{immediate:true,flush:'sync'})
onHide(()=>{hidden=true;reset()});onShow(()=>{hidden=false;reset()});onBeforeUnmount(()=>{disposed=true;unsubscribe();reset()})
</script>
<style scoped>
.announcement-root{position:relative;z-index:185}.home-announcement{width:calc(100vw - 76.9231rpx);max-width:440px;max-height:86vh;min-width:0;color:var(--text)}.announcement-scroll{max-height:calc(86vh - 107.6923rpx)}.announcement-image{display:block;width:100%;height:auto}.announcement-text-panel{padding:46.1538rpx 38.4615rpx;border:1px solid var(--line);border-radius:30.7692rpx;background:var(--surface)}.announcement-title{display:block;font-size:36.5385rpx;line-height:50rpx;font-weight:700;white-space:pre-wrap;overflow-wrap:anywhere}.announcement-body{display:block;margin-top:26.9231rpx;font-size:26.9231rpx;line-height:46.1538rpx;white-space:pre-wrap;overflow-wrap:anywhere;color:var(--muted)}.announcement-link{display:flex;align-items:center;justify-content:flex-end;gap:7.6923rpx;margin-top:30.7692rpx;color:var(--accent);font-size:25rpx}.announcement-close{display:flex;align-items:center;justify-content:center;gap:7.6923rpx;height:84.6154rpx;min-height:44px;padding:0 23.0769rpx;margin:15.3846rpx auto 0;border-radius:46.1538rpx;color:#fff;background:rgba(15,19,17,.9);border:1px solid rgba(255,255,255,.6);font-size:25rpx}
</style>
