<!-- 文件作用：使用官方 uni-popup 和微信手机号授权，统一交易前验证弹层。 -->
<template>
  <!-- #ifdef MP-WEIXIN -->
  <root-portal>
  <!-- #endif -->
  <view class="phone-layer" :style="popupTheme">
  <UniPopup ref="popup" type="bottom" :is-mask-click="!phoneGate.busy" background-color="transparent" @change="popupChanged">
    <view class="phone-sheet"><view class="phone-heading"><text>验证手机号</text><button class="icon-btn phone-close" aria-label="关闭手机号验证" :disabled="phoneGate.busy" @click="close"><Icon name="close" :size="21" /></button></view><text class="phone-copy">下单与发布前需要验证手机号，方便订单沟通与账号找回。号码不会展示在公开主页。</text><text v-if="phoneGate.error" class="phone-error">{{ phoneGate.error }}</text>
      <!-- #ifdef MP-WEIXIN -->
      <button class="btn" open-type="getPhoneNumber" :disabled="phoneGate.busy||nativePending" @tap="prepareNative" @getphonenumber="verifyWechat">{{ phoneGate.busy ? '正在验证…' : '使用微信绑定的手机号' }}</button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <template v-if="state.commerce?.mode==='official_account'">
        <text v-if="state.commerce.sms_configured===false" class="phone-error">公众号短信验证尚未开通，请联系平台管理员配置短信服务。</text>
        <view class="field"><text class="field-label">本人手机号</text><input v-model="smsPhone" class="input" type="number" maxlength="11" :disabled="phoneGate.busy||smsSending" placeholder="请输入11位手机号"/></view>
        <view class="field"><text class="field-label">短信验证码</text><view class="phone-sms-row"><input v-model="smsCode" class="input phone-code-input" type="number" maxlength="6" :disabled="phoneGate.busy" placeholder="请输入验证码"/><button class="text-btn phone-send" :disabled="phoneGate.busy||smsSending||smsCooldown>0||state.commerce.sms_configured===false" @click="sendSMS">{{smsSending?'正在发送…':smsCooldown>0?smsCooldown+'秒后重发':'获取验证码'}}</button></view></view>
        <button class="btn" :disabled="phoneGate.busy||smsSending||state.commerce.sms_configured===false" @click="verifySMS">{{phoneGate.busy?'正在验证…':'确认验证'}}</button>
      </template>
      <text v-else class="phone-copy">请在微信小程序中完成手机号授权。</text>
      <!-- #endif -->
      <button v-if="state.devMode" class="btn-secondary btn-small phone-dev" :disabled="phoneGate.busy||nativePending" @click="verifyDev">本地测试：绑定模拟手机号</button>
    </view>
  </UniPopup>
  </view>
  <!-- #ifdef MP-WEIXIN -->
  </root-portal>
  <!-- #endif -->
</template>
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { phoneGate, finishPhone, isCurrentPhoneGate } from '../lib/phone'
import { state, refreshMe, sessionRevision, pageTheme } from '../lib/store'
import { errorMessage } from '../api/client'
import { verifyDevPhone, verifyWechatPhone, sendPhoneSMS, verifyPhoneSMS } from '../api/auth.api'
import { designRpx } from '../lib/design-units'
/** 官方弹层开放的实例方法。 */
interface PopupController { open: (type?: string) => void; close: () => void; showPopup?: boolean }
const popup = ref<PopupController | null>(null), active = ref(true), nativePending = ref(false)
const smsPhone=ref(''),smsCode=ref(''),smsSending=ref(false),smsCooldown=ref(0)
let smsTimer:ReturnType<typeof setInterval>|undefined
/** 离开或切号时清除手机号、验证码和本地倒计时。 */
function clearSMS():void{smsPhone.value='';smsCode.value='';smsSending.value=false;smsCooldown.value=0;if(smsTimer)clearInterval(smsTimer);smsTimer=undefined}
/** 只向当前账号本人手机号发送真实短信，保留服务端错误提示。 */
async function sendSMS():Promise<void>{const epoch=displayedEpoch;if(!ownsGate(epoch)||smsSending.value||smsCooldown.value>0||phoneGate.busy)return;if(!/^1[3-9]\d{9}$/.test(smsPhone.value)){phoneGate.error='请输入有效的11位手机号';return}smsSending.value=true;phoneGate.error='';try{const result=await sendPhoneSMS(smsPhone.value);if(!ownsGate(epoch))return;smsCooldown.value=Math.max(1,result.retry_after);smsTimer=setInterval(()=>{smsCooldown.value=Math.max(0,smsCooldown.value-1);if(!smsCooldown.value&&smsTimer){clearInterval(smsTimer);smsTimer=undefined}},1000)}catch(err){if(ownsGate(epoch))phoneGate.error=errorMessage(err)}finally{if(ownsGate(epoch))smsSending.value=false}}
/** 手机号与验证码一起交服务端验证，成功后复用原门槛回跳。 */
async function verifySMS():Promise<void>{if(!/^1[3-9]\d{9}$/.test(smsPhone.value)||!/^\d{4,6}$/.test(smsCode.value)){phoneGate.error='请输入有效手机号和短信验证码';return}const phone=smsPhone.value,code=smsCode.value;await bind(()=>verifyPhoneSMS(phone,code))}
/** 原生根层不继承页面主题，显式补齐文字、边框与当前游戏色。 */
const popupTheme=computed(()=>({'--control-height':designRpx(44),'--compact-height':designRpx(36),'--radius':designRpx(12),'--muted':'#9aada2','--line':'rgba(255,255,255,0.12)',...pageTheme()}))
let displayedEpoch = -1, nativeEpoch: number | null = null, disposed = false, closing = false
/** 当前组件只处理自己实际展示的有效验证弹层。 */
function ownsGate(epoch = displayedEpoch): boolean { return !disposed && active.value && epoch === displayedEpoch && isCurrentPhoneGate(epoch) }
/** 未提交时允许取消，取消不继续原交易。 */
function close(): void { if (!phoneGate.busy && ownsGate()) finishPhone(false, displayedEpoch) }
/** 关闭官方遮罩后取消当前手机号操作。 */
function popupChanged(event: { show: boolean }): void { if (!event.show) { closing = true; if (!phoneGate.busy && ownsGate()) finishPhone(false, displayedEpoch); displayedEpoch = -1 } }
/** 原生手机号面板弹出前固定本次代次，同一组件不允许并发打开两个原生授权。 */
function prepareNative(): void { if (nativePending.value || phoneGate.busy || !ownsGate()) return; nativeEpoch = displayedEpoch; nativePending.value = true }
/** 兑换当前原生交互的一次性凭证，旧账号迟到事件不能绑定到新账号。 */
async function verifyWechat(event: { detail: { code?: string } }): Promise<void> { const epoch = nativeEpoch; nativeEpoch = null; nativePending.value = false; if (epoch === null || !ownsGate(epoch)) return; if (!event.detail?.code) { phoneGate.error = '未授权手机号，可以稍后重试。'; return } await bind(() => verifyWechatPhone(event.detail.code!), epoch) }
/** 显式开发环境使用模拟手机号。 */
async function verifyDev(): Promise<void> { await bind(() => verifyDevPhone('19900000000')) }
/** 绑定后同步身份，跨会话响应不得更改新账号。 */
async function bind(action: () => Promise<unknown>, epoch = displayedEpoch): Promise<void> {
  if (phoneGate.busy || !ownsGate(epoch)) return
  phoneGate.busy = true; phoneGate.error = ''
  try { await action(); if (!ownsGate(epoch)) return; await refreshMe(); if (ownsGate(epoch)) finishPhone(true, epoch) }
  catch (err) { if (ownsGate(epoch)) phoneGate.error = errorMessage(err) }
  finally { if (isCurrentPhoneGate(epoch)) phoneGate.busy = false }
}
/** 等官方旧关闭动画结束后再展示当前代次，迟到nextTick不会重新打开已取消的验证。 */
async function syncPopup(): Promise<void> { await nextTick(); const instance = popup.value; if (!instance) return; if (!instance.showPopup) closing = false; if (!disposed && active.value && isCurrentPhoneGate(phoneGate.epoch)) { if (instance.showPopup || closing) return; displayedEpoch = phoneGate.epoch; instance.open('bottom') } else if (instance.showPopup !== false && !closing) { displayedEpoch = -1; closing = true; instance.close() } }
/** 当前身份变更立即取消旧验证，旧网络请求只会被其代次守卫忽略。 */
function identityChanged(): void { if (phoneGate.visible && !isCurrentPhoneGate(phoneGate.epoch)) finishPhone(false, phoneGate.epoch) }
/** 隐藏只取消本组件实际持有的验证，不能关闭其他页面刚打开的新验证。 */
function hide(): void { const epoch = displayedEpoch; active.value = false; displayedEpoch = -1;clearSMS(); if (isCurrentPhoneGate(epoch)) finishPhone(false, epoch) }
watch(()=>phoneGate.epoch,clearSMS)
watch([() => phoneGate.visible, () => phoneGate.epoch, active, popup, () => popup.value?.showPopup], syncPopup, { flush: 'post' })
watch([() => state.user?.id, sessionRevision], identityChanged, { flush: 'sync' })
onShow(() => { active.value = true })
onHide(hide)
onUnload(() => { hide(); disposed = true })
</script>
<style scoped>
.phone-layer{position:relative;z-index:220;height:0;overflow:visible}
.phone-sms-row{display:flex;align-items:center;gap:23.0769rpx;min-width:0}.phone-code-input{flex:1;min-width:0}.phone-send{flex-shrink:0;min-height:44px}.phone-close{min-width:44px;min-height:44px}
.phone-sheet{display:flex;flex-direction:column;gap:30.7692rpx;box-sizing:border-box;width:100vw;max-width:600px;margin:0 auto;padding:23.0769rpx 38.4615rpx calc(46.1538rpx + env(safe-area-inset-bottom));border-radius:34.6154rpx 34.6154rpx 0 0;border:1px solid var(--line,rgba(255,255,255,0.12));background:var(--surface,#14201b);color:var(--text,#edf4ef)}.phone-heading{display:flex;align-items:center;justify-content:space-between;font-size:36.5385rpx;font-weight:700}.phone-copy{font-size:25rpx;line-height:42.3077rpx;color:var(--muted,#9aada2)}.phone-error{font-size:23.0769rpx;line-height:38.4615rpx;color:#f1b39d}
</style>
