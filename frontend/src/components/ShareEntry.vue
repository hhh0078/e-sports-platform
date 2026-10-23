<!-- 文件作用：五类公开内容共享两种分享方式，访客可直接分享好友，仅生成个人海报时登录。 -->
<template><view class="share-entry-shell"><button class="share-entry" :disabled="!active" :aria-label="label" @click="open"><Icon name="upload" :size="14"/><text class="share-entry-label">{{ label }}</text></button>
<!-- #ifdef MP-WEIXIN -->
<root-portal>
<!-- #endif -->
<view class="share-popup-layer" :style="popupTheme"><UniPopup ref="popup" type="bottom" background-color="transparent" :safe-area="false" @change="popupChanged"><view class="share-choices"><view class="share-choices-heading"><text class="share-choices-title">分享</text><button class="share-close" aria-label="关闭分享" @click="close"><Icon name="close" :size="24"/></button></view><view class="share-choice-grid"><button class="share-choice" open-type="share" @click="friend"><Icon name="weixin" :size="32"/><text class="share-choice-title">微信好友</text><text class="share-choice-note">直接分享给好友</text></button><button class="share-choice" @click="poster"><Icon name="image" :size="32"/><text class="share-choice-title">生成分享海报</text><text class="share-choice-note">含头像与小程序码</text></button></view><text class="share-choice-footer">海报使用你的头像和昵称，登录后生成。</text></view></UniPopup></view>
<!-- #ifdef MP-WEIXIN -->
</root-portal>
<!-- #endif -->
</view></template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { onHide, onShow, onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { state, sessionRevision, pageTheme } from '../lib/store'
import { token } from '../api/client'
import { go, notify } from '../lib/format'
import { isTimelinePreview, shareCreatePath, shareInputTargetPath } from '../lib/sharing'
import type { ShareKind } from '../types/sharing'
/** 官方弹层实例，只访问已公开的打开/关闭状态。 */
interface Popup { open: (type?: string) => void; close: () => void; showPopup?: boolean }
const props=withDefaults(defineProps<{kind:ShareKind;targetId?:string;gameId?:string;label?:string;active?:boolean}>(),{targetId:'',gameId:'',label:'分享',active:true})
/** 原生portal不依赖封面祖先的CSS继承，显式保持当前内容对应的游戏主题。 */
const popupTheme=computed(()=>pageTheme(props.gameId))
const popup=ref<Popup|null>(null),opened=ref(false)
let visible=true,disposed=false,operation=0,pendingCloseEvents=0,closing=false
/** 当前业务目标来自父组件，分享人不从路由或目标玩家推断。 */
function input(){return{kind:props.kind,target_id:props.targetId,game_id:props.gameId}}
/** 只有当前可见的弹层可以触发原生分享或登录跳转。 */
function current():boolean{return opened.value&&visible&&!disposed&&props.active}
/** ref晚绑定和关闭动画完成后，仍只展示当前有效打开意图。 */
async function syncPopup():Promise<void>{await nextTick();const instance=popup.value;if(!instance)return;if(!instance.showPopup)closing=false;if(current()){if(!instance.showPopup&&!closing)instance.open('bottom')}else if(instance.showPopup!==false&&!closing){closing=true;pendingCloseEvents++;instance.close()}}
/** 打开方式选择不登录、不创建分享记录、不调用任何发送接口。 */
function open():void{if(!visible||disposed||!props.active)return;if(isTimelinePreview())return notify('请先从微信顶部打开小程序，再分享给朋友');operation++;opened.value=true;void syncPopup()}
/** 关闭立即撤销后续异步回调，迟到ref不能复活旧弹层。 */
function close():void{operation++;opened.value=false;void syncPopup()}
/** 旧动画只确认旧关闭，用户点击遮罩才撤销当前分享选择。 */
function popupChanged(event:{show:boolean}):void{if(event.show)return;if(pendingCloseEvents){pendingCloseEvents--;return}closing=true;operation++;opened.value=false}
/** 微信端由宿主onShareAppMessage原生处理；H5仅复制公共内容链接。 */
function friend():void{if(!current())return;const currentOperation=operation;const target=shareInputTargetPath(input());close();
  // #ifdef H5
  uni.setClipboardData({data:location.origin+'/#'+target,success:()=>{if(!disposed&&visible&&operation===currentOperation+1)notify('公开链接已复制，可发给好友')}})
  // #endif
}
/** 只有选择个人海报时才要求登录，登录回跳保留目标而不冒用目标用户。 */
function poster():void{if(!current())return;const path=shareCreatePath(input());close();go(state.user?path:'/pages/account/login?next='+encodeURIComponent(path))}
/** 页面隐藏和主视图切换都关闭当前临时分享选择。 */
function hide():void{visible=false;close()}
watch([opened,popup,()=>popup.value?.showPopup],syncPopup,{flush:'post'})
watch([()=>props.active,()=>props.kind,()=>props.targetId,()=>props.gameId,()=>state.user?.id,sessionRevision,token],()=>{if(opened.value)close()},{flush:'sync'})
onShow(()=>{visible=true})
onHide(hide)
onUnload(()=>{disposed=true;hide()})
onBeforeUnmount(()=>{disposed=true;hide()})
</script>
<style scoped>.share-entry-shell{display:inline-flex;min-width:0;max-width:100%}.share-entry{display:inline-flex;align-items:center;justify-content:center;gap:5px;min-height:32px;max-width:100%;padding:0 6px;background:transparent;color:var(--accent);font-size:12px;line-height:20px;white-space:nowrap}.share-entry-label{overflow:hidden;text-overflow:ellipsis}.share-popup-layer{position:relative;z-index:180}.share-choices{width:100vw;max-width:600px;box-sizing:border-box;margin:0 auto;padding:24px 20px calc(20px + env(safe-area-inset-bottom));border-radius:20px 20px 0 0;background:var(--bg);color:var(--text)}.share-choices-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px}.share-choices-title{font-size:21px;line-height:30px;font-weight:700}.share-close{display:flex;align-items:center;justify-content:center;width:32px;height:32px;padding:0;background:transparent;color:var(--muted,#91a299)}.share-choice-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;min-width:0}.share-choice{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:10px;min-width:0;min-height:116px;padding:14px 6px;border:1px solid var(--line,rgba(255,255,255,0.12));border-radius:12px;background:var(--surface);color:var(--accent);line-height:22px}.share-choice-title{font-size:14px;font-weight:650;white-space:nowrap;color:var(--text)}.share-choice-note{font-size:11px;line-height:18px;color:var(--muted,#91a299)}.share-choice-footer{display:block;text-align:center;margin-top:16px;color:var(--muted,#91a299);font-size:11px;line-height:20px}</style>
