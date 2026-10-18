<!-- 文件作用：提供全局游戏主题、品牌栏和四项主导航。 -->
<template>
  <view class="app-frame" :class="{immersive}" :style="themeStyle">
    <view v-if="!immersive" class="shell-top" :style="headerStyle"><view class="brand" @click="go('/pages/index/index')"><view class="brand-mark">集</view><view><text class="brand-name">集结</text><text class="brand-en">JIJIE</text></view></view><view class="top-right"><text v-if="state.devMode" class="local-badge">本地体验</text><view class="account-circle" @click="go(state.user ? '/pages/me/index' : '/pages/login/index')"><Icon :name="state.user ? 'user' : 'arrow'" :size="17" /></view></view></view>
    <view class="app-body">
      <slot name="header" />
      <PageHeader v-if="title" :title="title" :subtitle="subtitle" :back="back" />
      <slot />
    </view>
    <view class="bottom-nav"><view class="nav-inner"><view v-for="item in items" :key="item.key" class="nav-item" :class="{selected: active === item.key}" @click="go(item.url)"><view class="nav-icon"><Icon :name="item.icon" :size="23" /></view><text>{{ item.label }}</text><view v-if="active === item.key" class="nav-dot" /></view></view></view>
  </view>
</template>
<script setup>
import { ref, onMounted } from 'vue'
import { state, themeStyle } from '../lib/store'
import { go } from '../lib/format'
import Icon from './Icon.vue'
import PageHeader from './PageHeader.vue'
defineProps({ title: String, subtitle: String, active: String, back: Boolean, immersive: Boolean })
const items = [{ key: 'index', label: '玩家', icon: 'home', url: '/pages/index/index' }, { key: 'worker', label: '打手', icon: 'work', url: '/pages/worker/index' }, { key: 'orders', label: '订单', icon: 'orders', url: '/pages/orders/index' }, { key: 'me', label: '我的', icon: 'user', url: '/pages/me/index' }]
const headerStyle = ref({})
/** 根据微信原生胶囊边界保留安全空间，避免头像与系统按钮重叠。 */
function fitNativeHeader() {
  // #ifdef MP-WEIXIN
  const capsule = uni.getMenuButtonBoundingClientRect()
  const info = uni.getWindowInfo()
  if (capsule?.width) headerStyle.value = { paddingTop: capsule.top + 'px', paddingRight: (info.windowWidth - capsule.left + 10) + 'px', paddingBottom: '18px' }
  // #endif
}
onMounted(fitNativeHeader)
</script>
<style scoped>
.immersive .app-body{max-width:600px;padding:0 0 82px}.shell-top{max-width:720px;margin:0 auto;display:flex;justify-content:space-between;align-items:center;padding:22px 20px 20px;padding-top:calc(22px + var(--status-bar-height,0px))}.brand{display:flex;gap:8px;align-items:center}.brand-mark{font-size:17px;font-weight:900;background:var(--accent);color:#06140d;width:32px;height:32px;line-height:32px;text-align:center;border-radius:8px;transform:skew(-5deg)}.brand-name{font-size:22px;font-weight:800;letter-spacing:2px}.brand-en{font-size:9px;letter-spacing:2.4px;margin-left:9px;font-weight:700;color:#9cad9e}.top-right{display:flex;gap:12px;align-items:center}.local-badge{font-size:9px;border:1px solid #34413a;color:#879b8e;padding:3px 7px;border-radius:4px}.account-circle{border:1px solid rgba(255,255,255,.13);border-radius:50%;height:30px;width:30px;display:flex;align-items:center;justify-content:center;color:#c5d0c9}.bottom-nav{position:fixed;z-index:20;bottom:0;left:0;right:0;background:var(--bg);border-top:1px solid rgba(255,255,255,.09);padding-bottom:env(safe-area-inset-bottom)}.nav-inner{display:flex;max-width:720px;margin:0 auto;height:60px}.nav-item{display:flex;flex:1;flex-direction:column;justify-content:center;align-items:center;gap:4px;font-size:11px;color:#9aa69f;position:relative;cursor:pointer}.nav-icon{height:24px}.nav-item.selected{color:var(--accent);font-weight:700}.nav-dot{position:absolute;top:0;width:22px;height:2px;background:var(--accent);border-radius:0 0 3px 3px}@media(min-width:760px){.shell-top{padding:24px 30px}}
/* #ifdef MP-WEIXIN */
.shell-top{padding-right:110px}.account-circle{display:none}.brand-en{font-size:8px;margin-left:5px;letter-spacing:1.6px}.local-badge{font-size:8px;padding:2px 5px;white-space:nowrap}.top-right{gap:0}.brand{flex-shrink:0}
/* #endif */
</style>
