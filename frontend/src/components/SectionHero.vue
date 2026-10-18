<!-- 文件作用：为打手、订单和个人页提供同一设计体系的游戏主视觉与微信胶囊安全品牌栏。 -->
<template>
  <view class="section-hero" :class="'art-' + art">
    <image v-if="background" class="section-image" :src="assetUrl(background)" mode="aspectFill" />
    <view class="section-shade" />
    <view class="section-brand" :style="nativeHeader">
      <view class="brand-lockup" @click="go('/pages/index/index')"><view class="brand-mark">集</view><text class="brand-name">{{ state.siteName }}</text><text class="section-name">/ {{ eyebrow }}</text></view>
      <button v-if="state.devMode" class="experience" @click="go('/pages/login/index')">本地体验 <Icon name="arrow" :size="12" /></button>
    </view>
    <view class="section-copy">
      <view v-if="title" class="section-title">{{ title }}</view>
      <view v-if="subtitle" class="section-subtitle">{{ subtitle }}</view>
      <slot />
    </view>
  </view>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import Icon from './Icon.vue'
import { state, game } from '../lib/store'
import { assetUrl, go } from '../lib/format'
const props = defineProps({ eyebrow: String, title: String, subtitle: String, art: { type: String, default: 'orders' } })
const nativeHeader = ref({})
const background = computed(() => props.art === 'worker' ? game.value.covers?.[3] || game.value.cover_url : game.value.cover_url)
/** 按微信真实胶囊位置留出顶部空间，避免设计标题和原生按钮叠在一起。 */
function fitNativeHeader() {
  // #ifdef MP-WEIXIN
  const capsule = uni.getMenuButtonBoundingClientRect(), info = uni.getWindowInfo()
  if (capsule?.width) nativeHeader.value = { paddingTop: capsule.top + 'px', paddingRight: (info.windowWidth - capsule.left + 10) + 'px' }
  // #endif
}
onMounted(fitNativeHeader)
</script>
<style scoped>
.section-hero{position:relative;isolation:isolate;overflow:hidden;background:var(--bg)}.section-image{position:absolute;left:0;top:0;width:132%;height:100%;object-position:center}.section-shade{position:absolute;inset:0;background:linear-gradient(90deg,rgba(0,0,0,.72),rgba(0,0,0,0) 75%),linear-gradient(0deg,var(--bg),rgba(0,0,0,0) 60%)}.section-brand{position:relative;display:flex;align-items:center;justify-content:space-between;gap:8px;padding:34px 20px 0;min-height:62px}.brand-lockup{display:flex;align-items:center;gap:7px;min-width:0}.brand-mark{width:28px;height:28px;line-height:28px;text-align:center;background:var(--accent);color:#05160e;font-size:17px;font-weight:850;border-radius:8px;flex-shrink:0}.brand-name{font-size:21px;font-weight:800;white-space:nowrap}.section-name{font-size:13px;font-weight:650;color:#c2c8c7;white-space:nowrap}.experience{display:flex;align-items:center;gap:3px;border:1px solid #ffffff80;border-radius:20px;padding:2px 10px;font-size:11px;line-height:22px;background:#0005;color:#f6f7f6;white-space:nowrap}.section-copy{position:relative;padding:24px 20px 20px}.section-title{font-size:26px;line-height:1.3;font-weight:850;letter-spacing:-.4px;color:#fff}.section-subtitle{font-size:13px;line-height:1.6;color:#bcc4c5;margin-top:7px;font-weight:500}.art-profile .section-copy{padding-top:29px;padding-bottom:16px}.art-worker .section-copy{padding-bottom:17px}.art-worker .section-image{width:132%}@media(max-width:350px){.section-brand{padding-left:14px;padding-right:14px}.brand-name{font-size:19px}.section-name{font-size:11px}.experience{padding:1px 7px;font-size:10px}.section-copy{padding-left:16px;padding-right:16px}.section-title{font-size:23px}}@media(min-width:500px){.section-brand{padding-top:36px}.section-copy{padding-top:36px}.section-title{font-size:32px}}
/* #ifdef MP-WEIXIN */
.experience{display:none}.section-brand{padding-top:44px}.section-name{font-size:12px}
/* #endif */
</style>
