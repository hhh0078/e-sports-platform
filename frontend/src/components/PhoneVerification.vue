<!-- 文件作用：使用官方 uni-popup 和微信手机号授权，统一交易前验证弹层。 -->
<template>
  <view class="phone-layer">
  <UniPopup ref="popup" type="bottom" :is-mask-click="!phoneGate.busy" background-color="transparent" @change="popupChanged">
    <view class="phone-sheet"><view class="phone-heading"><text>验证手机号</text><button class="icon-btn" aria-label="关闭手机号验证" :disabled="phoneGate.busy" @click="close"><Icon name="close" :size="21" /></button></view><text class="phone-copy">下单与发布前需要验证手机号，方便订单沟通与账号找回。号码不会展示在公开主页。</text><text v-if="phoneGate.error" class="phone-error">{{ phoneGate.error }}</text>
      <!-- #ifdef MP-WEIXIN -->
      <button class="btn" open-type="getPhoneNumber" :disabled="phoneGate.busy" @getphonenumber="verifyWechat">{{ phoneGate.busy ? '正在验证…' : '使用微信绑定的手机号' }}</button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <text class="phone-copy">请在微信小程序中完成手机号授权。</text>
      <!-- #endif -->
      <button v-if="state.devMode" class="btn-secondary btn-small phone-dev" :disabled="phoneGate.busy" @click="verifyDev">本地测试：绑定模拟手机号</button>
    </view>
  </UniPopup>
  </view>
</template>
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { phoneGate, finishPhone } from '../lib/phone'
import { state, refreshMe } from '../lib/store'
import { token, errorMessage } from '../api/client'
import { verifyDevPhone, verifyWechatPhone } from '../api/auth.api'
/** 官方弹层开放的实例方法。 */
interface PopupController { open: (type?: string) => void; close: () => void }
const popup = ref<PopupController | null>(null), active = ref(true)
/** 未提交时允许取消，取消不继续原交易。 */
function close(): void { if (!phoneGate.busy) finishPhone(false) }
/** 关闭官方遮罩后取消当前手机号操作。 */
function popupChanged(event: { show: boolean }): void { if (!event.show && phoneGate.visible && !phoneGate.busy) finishPhone(false) }
/** 兑换官方一次性手机号凭证。 */
async function verifyWechat(event: { detail: { code?: string } }): Promise<void> { if (!event.detail?.code) { phoneGate.error = '未授权手机号，可以稍后重试。'; return } await bind(() => verifyWechatPhone(event.detail.code!)) }
/** 显式开发环境使用模拟手机号。 */
async function verifyDev(): Promise<void> { await bind(() => verifyDevPhone('19900000000')) }
/** 绑定后同步身份，跨会话响应不得更改新账号。 */
async function bind(action: () => Promise<unknown>): Promise<void> {
  if (phoneGate.busy) return
  const requestToken = token(); phoneGate.busy = true; phoneGate.error = ''
  try { await action(); if (requestToken !== token()) { finishPhone(false); return } await refreshMe(); finishPhone(true) }
  catch (err) { if (requestToken === token()) phoneGate.error = errorMessage(err) }
  finally { phoneGate.busy = false }
}
watch(() => phoneGate.visible && active.value, async visible => { await nextTick(); if (visible) popup.value?.open('bottom'); else popup.value?.close() })
onShow(() => { active.value = true })
onHide(() => { active.value = false; if (phoneGate.visible) finishPhone(false) })
</script>
<style scoped>
.phone-layer{position:relative;z-index:150;height:0;overflow:visible}
.phone-sheet{display:flex;flex-direction:column;gap:16px;width:100vw;max-width:600px;margin:0 auto;padding:12px 20px calc(24px + env(safe-area-inset-bottom));border-radius:18px 18px 0 0;border:1px solid var(--line);background:var(--surface);color:var(--text)}.phone-heading{display:flex;align-items:center;justify-content:space-between;font-size:19px;font-weight:700}.phone-copy{font-size:13px;line-height:22px;color:var(--muted)}.phone-error{font-size:12px;line-height:20px;color:#f1b39d}
</style>
