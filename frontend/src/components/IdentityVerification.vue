<!-- 文件作用：按先行设计承载首次身份提醒与成年身份核验，临时身份资料离开即清除。 -->
<template>
  <!-- #ifdef MP-WEIXIN -->
  <root-portal>
  <!-- #endif -->
  <view class="identity-layer" :style="popupTheme">
    <UniPopup ref="popup" type="bottom" :is-mask-click="!identityGate.busy" background-color="transparent" :safe-area="false" @change="popupChanged">
      <scroll-view scroll-y class="identity-scroll">
        <view class="identity-sheet">
          <view class="identity-heading"><text>身份核验</text><button class="identity-close icon-btn" aria-label="关闭身份核验" :disabled="identityGate.busy" @click="close"><Icon name="close" :size="21"/></button></view>
          <text class="identity-copy">根据青少年防沉迷政策，我们需要核验您的身份信息。您可以继续浏览，但下单支付前必须完成身份核验。</text>
          <template v-if="formOpen">
            <view class="identity-field"><text class="identity-label">真实姓名</text><input v-model="name" class="input identity-input" :disabled="identityGate.busy" maxlength="50" placeholder="请输入身份证上的姓名" :adjust-position="true" aria-label="真实姓名"/></view>
            <view class="identity-field"><text class="identity-label">身份证号码</text><input v-model="idcard" class="input identity-input" :disabled="identityGate.busy" maxlength="18" type="idcard" placeholder="请输入18位居民身份证号码" :adjust-position="true" aria-label="身份证号码"/></view>
            <checkbox-group @change="consentChanged"><label class="identity-consent"><checkbox class="identity-checkbox" value="accepted" :checked="consent" :disabled="identityGate.busy" :color="popupTheme['--accent']"/><text>同意将姓名和身份证号码提交至小小API（山东省云鹊网络科技有限公司）用于身份核验，核验信息不对其他用户公开。</text></label></checkbox-group>
            <text v-if="identityGate.error" class="identity-error" role="alert">{{identityGate.error}}</text>
            <button class="btn identity-primary" :disabled="identityGate.busy" @click="submit">{{identityGate.busy?'正在核验…':'同意并核验'}}</button>
            <button class="text-btn identity-secondary" :disabled="identityGate.busy" @click="close">暂不核验</button>
          </template>
          <template v-else><button class="btn identity-primary" @click="begin">立即核验</button><button class="text-btn identity-secondary" @click="close">继续浏览</button></template>
        </view>
      </scroll-view>
    </UniPopup>
  </view>
  <!-- #ifdef MP-WEIXIN -->
  </root-portal>
  <!-- #endif -->
</template>
<script setup lang="ts">
import {computed,nextTick,ref,watch} from 'vue'
import {onShow,onHide,onUnload} from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import {state,sessionRevision,pageTheme} from '../lib/store'
import {errorMessage} from '../api/client'
import {verifyIdentity} from '../api/users.api'
import {identityGate,isCurrentIdentityGate,finishIdentity,acceptIdentityResult,requestIdentityReminder,markIdentityPresented} from '../lib/identity'
import {validateIdentityInput} from '../lib/identity-card'
import {acquireSystemPrompt,getUpdatePromptState,onUpdatePromptStateChange} from '../lib/update-manager'
import {isTimelinePreview} from '../lib/sharing'
import {designRpx} from '../lib/design-units'
import {go} from '../lib/format'

/** 官方弹层提供的最小展示接口。 */
interface PopupController{open:(type?:string)=>void;close:()=>void;showPopup?:boolean}
const props=withDefaults(defineProps<{remind?:boolean}>(),{remind:false})
const popup=ref<PopupController|null>(null),active=ref(true),formOpen=ref(false),name=ref(''),idcard=ref(''),consent=ref(false)
const popupTheme=computed(()=>({'--control-height':designRpx(44),'--compact-height':designRpx(36),'--radius':designRpx(12),'--muted':'#a8bab0','--line':'rgba(255,255,255,.16)',...pageTheme()}))
let displayedEpoch=-1,disposed=false,closing=false,syncing=false,release:(()=>void)|null=null
/** 只允许本组件持有的当前账号弹层操作数据。 */
function owns(epoch=displayedEpoch):boolean{return !disposed&&active.value&&epoch===displayedEpoch&&isCurrentIdentityGate(epoch)}
/** 姓名和身份证只存组件内存，不写入本地草稿。 */
function clearForm():void{name.value='';idcard.value='';consent.value=false;formOpen.value=false}
/** 释放身份提示槽，旧句柄不会影响其它公告或更新。 */
function releasePrompt():void{const lease=release;release=null;lease?.()}
/** 用户取消只结束当前门禁，保留下单与资金原表单。 */
function close():void{if(!identityGate.busy&&owns())finishIdentity(false,displayedEpoch)}
/** 官方实际展示才记录首次提醒，遮罩关闭只取消本代次。 */
function popupChanged(event:{show:boolean}):void{
  if(event.show){if(owns())markIdentityPresented(displayedEpoch);return}
  closing=true
  if(owns()&&!identityGate.busy)finishIdentity(false,displayedEpoch)
  displayedEpoch=-1;clearForm();releasePrompt()
}
/** 首次提醒的主操作进入表单，访客先登录并回到主页面。 */
function begin():void{if(!owns())return;if(!state.user){finishIdentity(false,displayedEpoch);go('/pages/account/login');return}formOpen.value=true}
/** 同意必须由当前弹层的主动勾选产生。 */
function consentChanged(event:{detail:{value:string[]}}):void{if(owns()&&!identityGate.busy)consent.value=event.detail.value.includes('accepted')}
/** 先在客户端拒绝未成年，只有成年且主动同意才把必要字段交给服务端核验。 */
async function submit():Promise<void>{
  if(identityGate.busy||!owns())return
  const epoch=displayedEpoch
  let body:ReturnType<typeof validateIdentityInput>
  try{body=validateIdentityInput(name.value,idcard.value,consent.value)}catch(error){identityGate.error=errorMessage(error);return}
  identityGate.busy=true;identityGate.error=''
  try{
    const result=await verifyIdentity(body)
    if(!owns(epoch))return
    if(!acceptIdentityResult(result,epoch))throw new Error('姓名与身份证信息核验未通过，请检查后重试')
    finishIdentity(true,epoch)
  }catch(error){if(owns(epoch))identityGate.error=errorMessage(error)}
  finally{body.name='';body.idcard='';if(isCurrentIdentityGate(epoch))identityGate.busy=false}
}
/** 进入提醒等待资金结束；强制门禁可在自身资金任务中占空闲槽，避免互相等待。 */
async function syncPopup():Promise<void>{
  if(syncing)return
  syncing=true
  try{
    await nextTick();const instance=popup.value;if(!instance)return
    if(!instance.showPopup)closing=false
    if(!disposed&&active.value&&isCurrentIdentityGate(identityGate.epoch)){
      if(instance.showPopup&&displayedEpoch!==identityGate.epoch){displayedEpoch=-1;closing=true;clearForm();instance.close();releasePrompt();return}
      if(instance.showPopup||closing)return
      if(identityGate.mode==='entry'&&getUpdatePromptState().financialBusy)return
      const lease=acquireSystemPrompt('identity');if(!lease)return
      release=lease;displayedEpoch=identityGate.epoch;clearForm();formOpen.value=identityGate.mode==='required'
      try{instance.open('bottom')}catch{displayedEpoch=-1;releasePrompt()}
    }else if(instance.showPopup!==false&&!closing){displayedEpoch=-1;closing=true;clearForm();instance.close();releasePrompt()}
    else if(!isCurrentIdentityGate(displayedEpoch)){clearForm();releasePrompt()}
  }finally{syncing=false}
}
/** 自动提醒只由入口承载，公开朋友圈预览不请求身份也不展示私有弹层。 */
function considerReminder():void{if(props.remind&&active.value&&!disposed&&!isTimelinePreview())requestIdentityReminder()}
/** 切账号清除旧证件与请求，不能让迟到核验写入新账号。 */
function identityChanged():void{if(identityGate.visible&&!isCurrentIdentityGate(identityGate.epoch))finishIdentity(false,identityGate.epoch);clearForm();considerReminder()}
/** 隐藏取消本组件展示的门禁并擦除临时信息，排队提醒未展示不记已读。 */
function hide():void{const epoch=displayedEpoch;active.value=false;displayedEpoch=-1;if(isCurrentIdentityGate(epoch))finishIdentity(false,epoch);clearForm();releasePrompt();void syncPopup()}
const unsubscribe=onUpdatePromptStateChange(()=>{void syncPopup()})
watch([()=>identityGate.visible,()=>identityGate.epoch,()=>identityGate.mode,active,popup,()=>popup.value?.showPopup],syncPopup,{flush:'post'})
watch([()=>state.user?.id,sessionRevision],identityChanged,{flush:'sync'})
watch([()=>state.ready,()=>state.user?.is_minor,()=>props.remind],considerReminder,{immediate:true})
onShow(()=>{active.value=true;considerReminder();void syncPopup()})
onHide(hide)
onUnload(()=>{hide();disposed=true;unsubscribe()})
</script>
<style scoped>
.identity-layer{position:relative;z-index:230;height:0;overflow:visible}.identity-scroll{width:100vw;max-width:600px;max-height:86vh;margin:0 auto;box-sizing:border-box;border-radius:34.6154rpx 34.6154rpx 0 0;border:1px solid var(--line);background:var(--surface,#14201b);color:var(--text,#edf4ef)}.identity-sheet{display:flex;flex-direction:column;gap:30.7692rpx;box-sizing:border-box;min-width:0;padding:23.0769rpx 38.4615rpx calc(30.7692rpx + env(safe-area-inset-bottom))}.identity-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;font-size:36.5385rpx;font-weight:700}.identity-close{flex-shrink:0;min-width:44px;min-height:44px}.identity-copy{font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted);white-space:pre-wrap;overflow-wrap:anywhere}.identity-field{display:flex;flex-direction:column;gap:7.6923rpx;min-width:0}.identity-label{font-size:25rpx;line-height:38.4615rpx;color:var(--muted)}.identity-input{display:block;box-sizing:border-box;width:100%;min-width:0;font-size:26.9231rpx;border-bottom:1px solid var(--line);border-radius:0}.identity-consent{display:flex;align-items:flex-start;gap:11.5385rpx;min-height:44px;min-width:0;font-size:25rpx;line-height:38.4615rpx;color:var(--muted)}.identity-checkbox{flex-shrink:0}.identity-error{display:block;color:#f1b39d;font-size:25rpx;line-height:42.3077rpx;white-space:pre-wrap;overflow-wrap:anywhere}.identity-primary{flex:none;width:100%}.identity-secondary{min-height:44px;flex:none;width:100%;font-size:26.9231rpx}
</style>
