<!-- 文件作用：使用微信官方手机号授权进行服务端核验，并保留清晰的开发测试入口。 -->
<template>
  <view v-if="phoneGate.visible && active" class="phone-mask" @click="close">
    <view class="phone-sheet" @click.stop>
      <view class="phone-heading"><text>验证手机号</text><button class="text-btn" @click="close">稍后再说</button></view>
      <text class="phone-copy">下单与发布前，需要验证手机号，方便订单沟通与账号找回。号码不会展示在公开主页。</text>
      <text v-if="phoneGate.error" class="phone-error">{{ phoneGate.error }}</text>
      <!-- #ifdef MP-WEIXIN -->
      <button class="phone-confirm" open-type="getPhoneNumber" :disabled="phoneGate.busy" @getphonenumber="verifyWechat">{{ phoneGate.busy ? '正在验证…' : '使用微信绑定的手机号' }}</button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <text class="phone-copy">请在微信小程序中完成手机号授权。</text>
      <!-- #endif -->
      <button v-if="state.devMode" class="phone-dev" :disabled="phoneGate.busy" @click="verifyDev">本地测试：绑定模拟手机号</button>
      <text v-if="state.devMode" class="phone-footnote">仅本地开发有效，正式环境不开放此入口。</text>
    </view>
  </view>
</template>
<script setup>
import { ref } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import { phoneGate, finishPhone } from '../lib/phone'
import { state, refreshMe } from '../lib/store'
import { api, token } from '../lib/api'
const active = ref(true)
/** 未提交请求时允许取消，取消不执行此前的下单或发布。 */
function close() { if (!phoneGate.busy) finishPhone(false) }
/** 兑换微信返回的一次性手机号凭证，不接受客户端自行填写真实号码。 */
async function verifyWechat(event) {
  if (!event.detail?.code) { phoneGate.error = '未授权手机号，可以稍后重试。'; return }
  await bind('/me/phone/wechat', { code: event.detail.code })
}
/** 在显式开发环境绑定无真实短信行为的测试号码。 */
async function verifyDev() { await bind('/me/phone/dev', { phone_number: '19900000000' }) }
/** 提交验证并刷新最新会话，迟到的授权不能改变另一个账号。 */
async function bind(path, data) {
  if (phoneGate.busy) return
  const requestToken = token()
  phoneGate.busy = true; phoneGate.error = ''
  try { await api(path, 'POST', data); if (requestToken !== token()) return finishPhone(false); await refreshMe(); finishPhone(true) }
  catch (error) { if (requestToken === token()) phoneGate.error = error.message }
  finally { phoneGate.busy = false }
}
onShow(() => { active.value = true })
onHide(() => { active.value = false; if (phoneGate.visible) finishPhone(false) })
</script>
<style scoped>
.phone-mask{position:fixed;inset:0;z-index:150;background:#0009;display:flex;align-items:flex-end;justify-content:center}.phone-sheet{width:100%;max-width:600px;background:#14201b;border:1px solid #ffffff18;border-radius:20px 20px 0 0;padding:25px 22px calc(26px + env(safe-area-inset-bottom));display:flex;flex-direction:column;gap:18px}.phone-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;font-size:20px;font-weight:700}.phone-copy{color:#abb9b1;font-size:13px;line-height:1.8}.phone-error{color:#efb59e;font-size:12px;line-height:1.7}.phone-confirm{background:#78dbae;color:#102019;font-weight:650;border:0;border-radius:10px;font-size:14px;line-height:44px;width:100%}.phone-dev{background:transparent;border:1px solid #ffffff22;border-radius:9px;color:#a9b8af;font-size:12px;line-height:38px}.phone-footnote{color:#83958a;font-size:10px;text-align:center}
</style>
