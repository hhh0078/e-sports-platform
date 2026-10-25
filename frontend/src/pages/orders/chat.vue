<!-- 文件作用：承接原订单四方证据聊天，独立展示历史、增量消息、图片上传与受控签名续期。 -->
<template>
  <AppShell title="订单聊天" :game-id="order?.game_id" class="order-chat-shell" back>
    <view v-if="error" class="chat-error"><text>{{ error }}</text><button class="text-btn" @click="load()">重新加载</button></view>
    <view v-if="!order&&!error" class="chat-loading">正在读取沟通记录…</view>
    <view v-if="order" class="chat-layout" :class="{'keyboard-open':keyboardHeight>0}" :style="{height:chatHeight}">
      <button class="chat-order" @click="go('/pages/orders/detail?id='+encodeURIComponent(id))"><image v-if="orderCover" class="chat-order-image" :src="assetUrl(orderCover)" mode="aspectFill"/><view class="chat-order-copy"><text class="chat-order-title">{{ order.title }}</text><text class="chat-order-meta">{{ statusLabel(order.status) }} · ¥{{ money(order.amount_cents) }}</text></view><text class="chat-order-link">查看订单</text><Icon name="arrow" :size="13"/></button>
      <view class="chat-evidence-note"><Icon name="shield-check" :size="15"/><text>沟通记录用于交易核查，仅订单相关成员可见</text></view>
      <scroll-view scroll-y class="chat-scroll" :scroll-into-view="scrollTarget" :scroll-with-animation="false" @scrolltoupper="loadEarlier" @scrolltolower="followChatBottom=true" @scroll="chatScrolled">
        <button v-if="hasEarlier" class="history-button" :disabled="loadingEarlier" @click="loadEarlier">{{ loadingEarlier?'加载中…':historyError||'上滑加载更早的沟通记录' }}</button><text v-if="messages.length&&!hasEarlier" class="chat-history-end">已显示全部沟通记录</text><view v-if="!messages.length" class="chat-empty"><Icon name="chat" :size="30"/><text>说清需求，合作从这里开始</text></view>
        <view v-for="message in messages" :key="message.id" :id="'msg-'+message.id" :class="['message',{'mine':message.sender_id===state.user?.id,'system':message.kind==='system'||message.sender_role==='system'}]">
          <view v-if="message.kind==='system'||message.sender_role==='system'" class="system-message"><Icon name="shield-check" :size="12"/><text>{{ message.content }} · {{ time(message.created_at) }}</text></view>
          <template v-else><view class="message-meta"><text class="message-name">{{ message.sender_name }}</text><text class="role-badge">{{ roleName(message.sender_role) }}</text></view><view class="bubble"><text v-if="message.content" class="message-text" selectable>{{ message.content }}</text><image v-if="message.image_url" class="evidence-image" :src="evidenceUrl(message.image_url)" mode="widthFix" @click="previewImage(message)" @error="refreshEvidence(message)"/></view><text class="message-time">{{ time(message.created_at) }}</text></template>
        </view><view id="chat-bottom" class="chat-bottom"/>
      </scroll-view>
      <view class="composer"><button class="image-button" :disabled="sending" aria-label="上传图片证据" @click="sendImage"><Icon name="image" :size="22"/></button><input v-model="draft" class="composer-input" maxlength="1000" confirm-type="send" :adjust-position="false" placeholder="输入消息…" @keyboardheightchange="keyboardChanged" @confirm="sendMessage"/><button class="send-button" :disabled="sending||!draft.trim()" @click="sendMessage">{{ sending?'发送中':'发送' }}</button></view>
    </view>
  </AppShell>
</template>
<script setup lang="ts">
import {ref,computed,nextTick,watch} from 'vue'
import {onLoad,onShow,onHide,onUnload,onResize} from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import {useNativePageRefresh} from '../../lib/native-page-refresh'
import {prepareImage} from '../../lib/media'
import {readSafeArea} from '../../lib/safe-area'
import {designPixels} from '../../lib/design-units'
import {getOrder,listMessages,sendOrderMessage,uploadEvidence,refreshEvidenceLink} from '../../api/orders.api'
import {state,init,sessionRevision} from '../../lib/store'
import {token,errorMessage,errorStatus,isCancelled} from '../../api/client'
import {assetUrl,go,money,time,statusLabel,notify} from '../../lib/format'
import type {Order,Message} from '../../types/models'
const id=ref(''),order=ref<Order|null>(null),messages=ref<Message[]>([]),error=ref(''),draft=ref(''),sending=ref(false),scrollTarget=ref(''),hasEarlier=ref(false),loadingEarlier=ref(false),historyError=ref(''),followChatBottom=ref(true),keyboardHeight=ref(0),chatHeight=ref('70vh')
const orderCover=computed(()=>state.games?.find(game=>game.id===order.value?.game_id)?.cover_url||'')
let disposed=false,visible=false,polling=false,chatInitialized=false,viewerID='',pageVersion=0,lastChatScrollTop=0
let timer:ReturnType<typeof setInterval>|undefined
const refreshingImages=new Set<string>()
/** 捕获消息及图片操作所属账号和会话，原生选图回来后再次核对。 */
function currentSession(){return{id:state.user?.id,role:state.user?.role,adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(),revision:sessionRevision()}}
/** 页面销毁或账号变化后，任何旧发送或签名响应均不得回填。 */
function sameSession(session:ReturnType<typeof currentSession>):boolean{return !disposed&&session.id===state.user?.id&&session.role===state.user?.role&&session.adminRole===state.user?.platform_admin_role&&session.status===state.user?.status&&session.token===token()&&session.revision===sessionRevision()}
/** 只允许可见且同一请求版本的结果更新私密聊天。 */
function currentRequest(version:number,session:ReturnType<typeof currentSession>):boolean{return visible&&version===pageVersion&&sameSession(session)}
/** 换号、失权或销毁时清空私密订单、历史、草稿及证据签名。 */
function clearPrivateOrder():void{order.value=null;messages.value=[];draft.value='';scrollTarget.value='';hasEarlier.value=false;loadingEarlier.value=false;historyError.value='';followChatBottom.value=true;chatInitialized=false;pageVersion++;polling=false;lastChatScrollTop=0;refreshingImages.clear()}
watch(()=>[state.user?.id,state.user?.role,state.user?.platform_admin_role,state.user?.status,sessionRevision()].join('\n'),()=>{clearPrivateOrder();if(visible)void load()})
/** 读取受保护订单和原聊天接口，增量游标只取新消息，历史始终在本页内去重保留。 */
async function load(silent=false):Promise<void>{
  if(!id.value||polling||!visible)return
  const version=pageVersion;let session:ReturnType<typeof currentSession>|undefined;polling=true
  try{await init();if(!visible||version!==pageVersion)return;session=currentSession();if(viewerID!==(state.user?.id||'')){viewerID=state.user?.id||'';order.value=null;messages.value=[];chatInitialized=false;hasEarlier.value=false}
    const last=messages.value[messages.value.length-1],query={limit:100,...(chatInitialized&&last?{after_id:last.id}:{})}
    const data=await Promise.all([getOrder(id.value),listMessages(id.value,query)]);if(!currentRequest(version,session))return
    order.value=data[0];error.value='';if(!chatInitialized){hasEarlier.value=data[1].length===100;chatInitialized=true}
    const changed=data[1].some(item=>!messages.value.some(old=>old.id===item.id));mergeMessages(data[1]);if(changed&&followChatBottom.value&&!loadingEarlier.value)await scrollChat()
  }catch(err){if(!visible||version!==pageVersion||session&&!sameSession(session))return;if([401,403,404].includes(errorStatus(err)||0)){clearPrivateOrder();clearInterval(timer);error.value=errorMessage(err)}else if(!silent||!order.value)error.value=errorMessage(err)}finally{if(version===pageVersion)polling=false}
}
/** 订单失权后立即移除原证据与草稿，普通网络错误只提示并保留重试内容。 */
function chatFailure(err:unknown):void{if([401,403,404].includes(errorStatus(err)||0)){clearPrivateOrder();clearInterval(timer);error.value=errorMessage(err)}else notify(err)}
/** 原生下拉仅重新读取受保护沟通记录，保留未发送文字，不触发资金同步。 */
async function refreshOrder():Promise<void>{await load()}
/** 按后端时间与ID稳定去重，轮询和旧页并发不会丢弃已经加载的历史。 */
function mergeMessages(items:Message[]):void{const merged=new Map(messages.value.map(item=>[item.id,item]));items.forEach(item=>merged.set(item.id,item));messages.value=[...merged.values()].sort((a,b)=>a.created_at.localeCompare(b.created_at)||a.id.localeCompare(b.id))}
/** 触顶用原首条消息游标读取历史，追加后恢复该消息锚点，不跳到新消息末尾。 */
async function loadEarlier():Promise<void>{
  if(!visible||loadingEarlier.value||!hasEarlier.value||!messages.value.length)return
  const version=pageVersion,session=currentSession(),first=messages.value[0].id;loadingEarlier.value=true;historyError.value='';followChatBottom.value=false
  try{const items=await listMessages(id.value,{limit:100,before_id:first});if(!currentRequest(version,session))return;hasEarlier.value=items.length===100;mergeMessages(items);scrollTarget.value='';await nextTick();if(currentRequest(version,session))scrollTarget.value='msg-'+first}catch(err){if(currentRequest(version,session)){if([401,403,404].includes(errorStatus(err)||0))chatFailure(err);else historyError.value='历史记录加载失败，点击重试'}}finally{if(version===pageVersion)loadingEarlier.value=false}
}
/** 清空再设置底部锚点，避免重复值不触发原生滚动。 */
async function scrollChat():Promise<void>{const version=pageVersion;followChatBottom.value=true;scrollTarget.value='';await nextTick();if(visible&&version===pageVersion)scrollTarget.value='chat-bottom'}
/** 用户向上阅读时关闭自动跟随，新消息不会抢走历史阅读位置。 */
function chatScrolled(event:{detail:{scrollTop:number}}):void{const top=event.detail.scrollTop;if(top+2<lastChatScrollTop)followChatBottom.value=false;lastChatScrollTop=top}
/** 键盘高度是原生实际像素，仅压缩聊天视口，避免整页标题被自动上推。 */
function keyboardChanged(event:{detail:{height:number}}):void{const height=event.detail.height;keyboardHeight.value=Number.isFinite(height)?Math.max(0,height):0;measureChatViewport();if(followChatBottom.value)void scrollChat()}
/** 视口、原生44px导航和键盘使用实际坐标；设计内边距先按窗口换算，避免把rpx再次缩放。 */
function measureChatViewport():void{try{const info=uni.getWindowInfo(),status=readSafeArea().statusBarHeight,bottom=info.safeAreaInsets?.bottom||0;chatHeight.value=Math.max(88,info.windowHeight-44-status-bottom-designPixels(16,info.windowWidth)-keyboardHeight.value)+'px'}catch{/* 平台测量暂不可用时保留现有可读视口。 */}}
/** 文字发送失败保留草稿，成功后仅清空对应账号的输入。 */
async function sendMessage():Promise<void>{if(sending.value||!draft.value.trim())return;const session=currentSession();sending.value=true;try{const sent=await sendOrderMessage(id.value,{content:draft.value.trim()});if(!sameSession(session))return;mergeMessages([sent]);draft.value='';if(visible){await load();await scrollChat()}}catch(err){if(sameSession(session)&&visible)chatFailure(err)}finally{sending.value=false}}
/** 通过既有官方裁剪压缩与本订单证据上传发送图片，取消选择不误报错误。 */
async function sendImage():Promise<void>{if(sending.value)return;const session=currentSession();sending.value=true;try{const path=await prepareImage('evidence');if(!sameSession(session))return;const evidence=await uploadEvidence(id.value,path);if(!sameSession(session))return;const sent=await sendOrderMessage(id.value,{content:'',image_url:evidence.url});if(!sameSession(session))return;mergeMessages([sent]);if(visible){await load();await scrollChat()}}catch(err){if(sameSession(session)&&visible&&!isCancelled(err))chatFailure(err)}finally{sending.value=false}}
/** 只转换受控相对证据地址，不把私密签名保存到本机历史。 */
function evidenceUrl(url:string|undefined):string{return assetUrl(url)}
/** 阅读或图片加载失败时按原订单重新授权证据，失权与迟到回包不恢复旧图片。 */
async function refreshEvidence(message:Message):Promise<string|undefined>{if(!message.image_url||refreshingImages.has(message.id))return;const version=pageVersion,session=currentSession();refreshingImages.add(message.id);try{const evidenceID=message.image_url.split('?')[0].split('/').pop()||'',result=await refreshEvidenceLink(id.value,evidenceID);if(!currentRequest(version,session))return;message.image_url=result.url;return result.url}catch(err){if(currentRequest(version,session))chatFailure(err)}finally{refreshingImages.delete(message.id)}}
/** 打开实际图片前续签，避免过期链接导致无法查证。 */
async function previewImage(message:Message):Promise<void>{const url=await refreshEvidence(message);if(url)uni.previewImage({urls:[evidenceUrl(url)]})}
/** 发送人身份采用服务端角色，昵称不能伪造平台或组织管理标签。 */
function roleName(role:string):string{return({buyer:'老板',provider:'打手',club_admin:'俱乐部',admin:'平台',system:'系统'} as Record<string,string>)[role]||'成员'}
/** 隐藏立即停止轮询并作废在途回包，保留当前页尚未发送的草稿。 */
function stop():void{visible=false;pageVersion++;polling=false;loadingEarlier.value=false;keyboardHeight.value=0;clearInterval(timer)}
useNativePageRefresh({refresh:refreshOrder,blocked:()=>sending.value,context:()=>[state.user?.id,sessionRevision(),id.value]})
onLoad(options=>{id.value=options?.id||'';if(!id.value)error.value='订单不存在或缺少编号'})
onShow(()=>{visible=true;measureChatViewport();void load();clearInterval(timer);timer=setInterval(()=>{if(visible)void load(true)},4000)})
onResize(measureChatViewport)
onHide(stop)
onUnload(()=>{disposed=true;stop();clearPrivateOrder()})
</script>
<style scoped>
.order-chat-shell :deep(.app-body){padding-top:15.3846rpx;padding-bottom:calc(15.3846rpx + env(safe-area-inset-bottom))}.chat-layout{display:flex;flex-direction:column;min-width:0;min-height:0;overflow:hidden}.chat-order{display:flex;align-items:center;gap:15.3846rpx;min-width:0;width:100%;padding:15.3846rpx;border:1px solid var(--line);border-radius:15.3846rpx;background:var(--surface);color:var(--muted);text-align:left;flex-shrink:0}.chat-order-image{display:block;flex-shrink:0;width:84.6154rpx;height:84.6154rpx;border-radius:7.6923rpx}.chat-order-copy{flex:1;min-width:0}.chat-order-title{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:26.9231rpx;font-weight:650;color:var(--text);line-height:40.3846rpx}.chat-order-meta{display:block;margin-top:3.8462rpx;font-size:21.1538rpx;line-height:34.6154rpx}.chat-order-link{font-size:21.1538rpx;white-space:nowrap}.chat-evidence-note{display:flex;align-items:center;gap:11.5385rpx;flex-shrink:0;font-size:19.2308rpx;line-height:32.6923rpx;color:var(--muted);padding:15.3846rpx 0}.chat-scroll{flex:1;min-height:0;height:0;width:100%;box-sizing:border-box}.history-button{display:flex;align-items:center;justify-content:center;width:100%;min-height:44px;background:transparent;color:var(--accent);font-size:21.1538rpx}.chat-history-end{display:block;text-align:center;font-size:19.2308rpx;color:var(--muted);padding:15.3846rpx 0}.chat-empty{display:flex;flex-direction:column;align-items:center;gap:23.0769rpx;padding:115.3846rpx 0;color:var(--muted);font-size:25rpx}.message{display:flex;flex-direction:column;align-items:flex-start;min-width:0;margin:26.9231rpx 0}.mine{align-items:flex-end}.system{align-items:center}.message-meta{display:flex;align-items:center;flex-wrap:wrap;gap:11.5385rpx;max-width:100%;margin-bottom:9.6154rpx;font-size:19.2308rpx;line-height:30.7692rpx;color:var(--muted)}.message-name{overflow-wrap:anywhere}.role-badge{padding:0 9.6154rpx;border:1px solid var(--line);border-radius:7.6923rpx;color:var(--accent);font-size:17.3077rpx}.bubble{max-width:88%;padding:15.3846rpx 23.0769rpx;border-radius:3.8462rpx 15.3846rpx 15.3846rpx;background:var(--surface);color:var(--text);font-size:26.9231rpx;line-height:44.2308rpx;min-width:0}.mine .bubble{background:var(--accent);color:#092416;border-radius:15.3846rpx 3.8462rpx 15.3846rpx 15.3846rpx}.message-text{display:block;white-space:pre-wrap;overflow-wrap:anywhere}.message-time{font-size:17.3077rpx;line-height:30.7692rpx;color:var(--muted);margin-top:5.7692rpx}.system-message{display:flex;align-items:flex-start;justify-content:center;gap:7.6923rpx;max-width:100%;padding:7.6923rpx 19.2308rpx;font-size:19.2308rpx;line-height:32.6923rpx;color:var(--muted);text-align:center;overflow-wrap:anywhere}.evidence-image{display:block;width:346.1538rpx;max-width:100%;border-radius:9.6154rpx}.chat-bottom{height:7.6923rpx}.composer{display:flex;align-items:center;gap:11.5385rpx;flex-shrink:0;padding-top:15.3846rpx;border-top:1px solid var(--line-subtle);min-width:0}.composer-input{display:block;flex:1;min-width:0;height:84.6154rpx;min-height:44px;line-height:normal;padding:0 23.0769rpx;font-size:25rpx;border:1px solid var(--line);border-radius:42.3077rpx;background:var(--surface);color:var(--text)}.image-button{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:84.6154rpx;min-width:103.125rpx;height:84.6154rpx;min-height:44px;padding:0;background:var(--surface);border-radius:15.3846rpx;color:var(--text)}.send-button{display:flex;align-items:center;justify-content:center;min-height:44px;height:84.6154rpx;flex-shrink:0;padding:0 26.9231rpx;border-radius:38.4615rpx;background:var(--accent);color:#092416;font-size:25rpx}.chat-error{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;font-size:25rpx;line-height:42.3077rpx;color:#efb299;overflow-wrap:anywhere}.chat-loading{text-align:center;color:var(--muted);padding:76.9231rpx 0;font-size:25rpx}
.keyboard-open .chat-order,.keyboard-open .chat-evidence-note{display:none}
</style>
