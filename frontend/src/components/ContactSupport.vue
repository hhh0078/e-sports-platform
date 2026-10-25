<!-- 文件作用：共享平台与俱乐部业务客服入口，原生微信客服、二维码、电话和微信号均由用户主动选择。 -->
<template>
  <view :class="['contact-support',{'contact-compact':compact,'contact-menu':menu}]"><button class="contact-entry" :disabled="!active" @click="open"><Icon name="headphones" :size="20"/><text class="contact-entry-label">{{ label||(club?'联系俱乐部':'联系客服') }}</text><Icon v-if="!compact" name="arrow" :size="14"/></button>
    <view class="contact-overlay"><UniPopup ref="popup" type="bottom" background-color="transparent" :safe-area="false" @change="popupChanged"><scroll-view scroll-y class="contact-sheet-scroll" :style="{height:qrOpened&&qrURL?'76vh':club?designRpx(360):designRpx(400)}"><view class="contact-sheet"><view class="contact-heading"><text class="contact-title">{{ club?club.name+'客服':'平台客服' }}</text><button class="text-btn contact-close" aria-label="关闭客服" @click="close"><Icon name="close" :size="22"/></button></view>
      <template v-if="!club">
        <!-- #ifdef MP-WEIXIN -->
        <button class="contact-choice contact-official" open-type="contact" session-from="jijie-platform-support"><view class="contact-choice-icon"><Icon name="chat" :size="22"/></view><view class="contact-choice-copy"><text class="contact-choice-title">微信在线客服</text><text class="contact-choice-note">在微信中与平台客服沟通</text></view><Icon name="arrow" :size="16"/></button>
        <!-- #endif -->
        <!-- #ifndef MP-WEIXIN -->
        <button class="contact-choice" @click="wechatHint"><view class="contact-choice-icon"><Icon name="chat" :size="22"/></view><view class="contact-choice-copy"><text class="contact-choice-title">微信在线客服</text><text class="contact-choice-note">请在微信小程序中使用</text></view><Icon name="arrow" :size="16"/></button>
        <!-- #endif -->
      </template>
      <button class="contact-choice" @click="toggleQR"><view class="contact-choice-icon"><Icon name="scan" :size="22"/></view><view class="contact-choice-copy"><text class="contact-choice-title">{{ club?'俱乐部客服二维码':'客服二维码' }}</text><text class="contact-choice-note">{{ qrURL?'点击查看，可长按识别':'暂未配置二维码' }}</text></view><Icon :name="qrOpened?'top':'bottom'" :size="16"/></button>
      <view v-if="qrOpened&&qrURL" class="contact-qr-section"><image class="contact-qr" :src="qrURL" mode="aspectFit" :show-menu-by-longpress="true" @click="previewQR"/><text class="contact-qr-hint">点击放大 · 长按识别二维码</text></view>
      <view v-if="club||wechat" class="contact-wechat"><view class="contact-choice-copy"><text class="contact-choice-title">客服微信号</text><text class="contact-choice-note contact-wechat-name">{{ wechat||'暂未配置微信号' }}</text></view><button v-if="wechat" class="text-btn" @click="copyWechat">复制</button></view>
      <button class="contact-choice contact-phone" @click="callPhone"><view class="contact-choice-icon"><Icon name="phone" :size="22"/></view><view class="contact-choice-copy"><text class="contact-choice-title">{{ club?'俱乐部联系电话':'客服电话' }}</text><text class="contact-choice-note">{{ phone||'暂未配置联系电话' }}</text></view><Icon name="arrow" :size="16"/></button>
      <text v-if="!qrURL&&!phone&&!wechat" class="contact-empty">{{ club?'该俱乐部暂未配置联系方式。':'二维码和电话暂未配置，可在微信小程序内使用在线客服。' }}</text>
    </view></scroll-view></UniPopup></view>
  </view>
</template>
<script setup lang="ts">
import { computed,nextTick,ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { state,sessionRevision } from '../lib/store'
import { assetUrl,notify } from '../lib/format'
import { errorMessage } from '../api/client'
import type { Club } from '../types/models'
import { designRpx } from '../lib/design-units'
const props=withDefaults(defineProps<{club?:Club|null;label?:string;compact?:boolean;menu?:boolean;active?:boolean}>(),{club:null,label:'',compact:false,menu:false,active:true})
const popup=ref<{open:(type?:string)=>void;close:()=>void;showPopup?:boolean}|null>(null),opened=ref(false),qrOpened=ref(false)
let visible=true,disposed=false,closing=false,operation=0,pendingCloseEvents=0
/** 俱乐部客服只读本组织配置，不回退到平台或技术支持联系方式。 */
const phone=computed(()=>String(props.club?props.club.contact_phone||'':state.support_phone||'').trim())
/** 微信号用于用户主动复制，不自动添加好友或发送消息。 */
const wechat=computed(()=>String(props.club?props.club.contact_wechat||'':state.support_wechat||'').trim())
/** 二维码始终使用后端托管的公开素材地址。 */
const qrURL=computed(()=>assetUrl(props.club?props.club.contact_qr_url||'':state.support_qr_url||''))
/** 仅当前可见、已启用的客服弹层允许调用原生能力。 */
function current():boolean{return opened.value&&visible&&!disposed&&props.active}
/** 官方弹层常驻，旧关闭动画结束后重新打开，延迟回调只读当前有效状态。 */
async function syncPopup():Promise<void>{await nextTick();const instance=popup.value;if(!instance)return;if(!instance.showPopup)closing=false;if(current()){if(!instance.showPopup&&!closing)instance.open('bottom')}else if(instance.showPopup!==false&&!closing){closing=true;pendingCloseEvents++;instance.close()}}
/** 用户主动打开，不因组件挂载而弹出或触发联系动作。 */
function open():void{if(!visible||disposed||!props.active)return;operation++;qrOpened.value=false;opened.value=true;void syncPopup()}
/** 关闭后立即作废未完成的复制或原生提示回调。 */
function close():void{operation++;opened.value=false;qrOpened.value=false;void syncPopup()}
/** 程序关闭事件只确认旧操作；遮罩主动关闭才撤销当前意图，避免迟到事件误关新弹层。 */
function popupChanged(event:{show:boolean}):void{if(event.show)return;if(pendingCloseEvents){pendingCloseEvents--;return}closing=true;operation++;opened.value=false;qrOpened.value=false}
/** 只展开真实已配置二维码，空配置明确提示。 */
function toggleQR():void{if(!current())return;if(!qrURL.value){notify('暂未配置客服二维码');return}qrOpened.value=!qrOpened.value}
/** 点击后使用官方图片预览，二维码可在微信中长按识别。 */
function previewQR():void{if(!current()||!qrURL.value)return;const currentOperation=operation;uni.previewImage({urls:[qrURL.value],current:qrURL.value,fail(error){if(current()&&currentOperation===operation)notify(errorMessage(error))}})}
/** 只有用户点击电话入口才调用官方拨号，不使用技术支持号码填充空值。 */
function callPhone():void{if(!current())return;const number=phone.value.replace(/[\s()\-]/g,'');if(!/^\+?\d{3,20}$/.test(number)){notify(phone.value?'客服电话格式无效，请使用其他联系方式':'暂未配置客服电话');return}const currentOperation=operation;uni.makePhoneCall({phoneNumber:number,fail(error){if(current()&&currentOperation===operation&&!/cancel/i.test(error.errMsg||''))notify(errorMessage(error))}})}
/** 仅复制本次弹层显示的客服微信号，旧回调不向其他页面弹提示。 */
function copyWechat():void{if(!current()||!wechat.value)return;const currentOperation=operation;uni.setClipboardData({data:wechat.value,success(){if(current()&&currentOperation===operation)notify('客服微信号已复制')},fail(error){if(current()&&currentOperation===operation)notify(errorMessage(error))}})}
/** H5明确说明官方客服能力的使用位置，不跳转自建聊天室。 */
function wechatHint():void{if(current())notify('请在微信小程序中使用在线客服')}
/** 页面隐藏关闭公开联系弹框，避免返回后自动拨号或恢复原生动作。 */
function hide():void{visible=false;close()}
watch([opened,popup,()=>popup.value?.showPopup],syncPopup,{flush:'post'})
watch([()=>props.active,()=>props.club?.id,phone,wechat,qrURL,sessionRevision],()=>{if(opened.value)close()},{flush:'sync'})
onShow(()=>{visible=true})
onHide(hide)
onUnload(()=>{disposed=true;hide()})
defineExpose({open,close})
</script>
<style scoped>
.contact-support{min-width:0}.contact-entry{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;width:100%;min-width:0;min-height:44px;padding:15.3846rpx 0;border:0;border-radius:0;background:transparent;color:var(--text);font-size:28.8462rpx;text-align:left}.contact-entry-label{flex:1;min-width:0;line-height:46.1538rpx}.contact-compact .contact-entry{width:auto;min-height:69.2308rpx;gap:11.5385rpx;padding:0;color:var(--accent);font-size:25rpx}.contact-overlay{position:relative;z-index:170}.contact-sheet-scroll{max-height:85vh;width:100%;overflow:hidden}.contact-sheet{width:100vw;max-width:600px;min-width:0;box-sizing:border-box;margin:0 auto;padding:46.1538rpx 38.4615rpx calc(46.1538rpx + env(safe-area-inset-bottom));border-radius:42.3077rpx 42.3077rpx 0 0;background:var(--bg);color:var(--text)}.contact-heading{display:flex;align-items:center;justify-content:space-between;gap:30.7692rpx;min-width:0;margin-bottom:46.1538rpx}.contact-title{flex:1;min-width:0;font-size:42.3077rpx;line-height:57.6923rpx;font-weight:650;overflow-wrap:anywhere}.contact-close{flex-shrink:0;min-width:61.5385rpx}.contact-choice{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:123.0769rpx;min-width:0;padding:23.0769rpx 26.9231rpx;margin:0 0 23.0769rpx;border:0;border-radius:19.2308rpx;background:var(--surface);color:var(--text);text-align:left;line-height:normal}.contact-choice-icon{display:flex;align-items:center;justify-content:center;width:53.8462rpx;height:61.5385rpx;flex-shrink:0;color:var(--accent)}.contact-choice-copy{display:flex;flex:1;flex-direction:column;gap:7.6923rpx;min-width:0}.contact-choice-title{font-size:30.7692rpx;line-height:46.1538rpx}.contact-choice-note{font-size:25rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.contact-qr-section{display:flex;flex-direction:column;align-items:center;gap:23.0769rpx;margin:38.4615rpx 0 46.1538rpx}.contact-qr{display:block;width:384.6154rpx;height:384.6154rpx;box-sizing:content-box;padding:23.0769rpx;border-radius:15.3846rpx;background:#fff}.contact-qr-hint{font-size:25rpx;line-height:42.3077rpx;color:var(--muted)}.contact-wechat{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;padding:15.3846rpx 7.6923rpx 38.4615rpx}.contact-wechat-name{color:var(--accent)}.contact-phone{margin-bottom:0}.contact-empty{display:block;margin-top:38.4615rpx;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted);overflow-wrap:anywhere}
.contact-menu .contact-entry{height:92.3077rpx;min-height:44px;padding:0;border-bottom:1px solid var(--line-subtle);font-size:28.8462rpx}.contact-menu .contact-entry-label{line-height:42.3077rpx}
</style>
