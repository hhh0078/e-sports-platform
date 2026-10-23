<!-- 文件作用：基于官方 uni-nav-bar 统一顶栏、微信安全区与所有二级页面返回。 -->
<template>
  <view class="page-navigation" :class="{ 'main-navigation': main }">
    <UniNavBar :title="title" :left-icon="back ? 'left' : ''" :left-width="main && title === '开机电竞' ? 0 : sideWidth" :right-width="sideWidth" :height="44" :status-bar="native" :border="false" color="var(--text)" background-color="var(--bg)" @click-left="goBack">
      <template #left><Icon v-if="back" name="back" :size="20" /></template>
      <view v-if="main && title === '开机电竞'" class="brand-title"><text class="brand-square">开</text><text>开机电竞</text></view><view v-else class="navigation-title">{{ title }}</view>
    </UniNavBar>
  </view>
</template>
<script setup lang="ts">
import UniNavBar from '@dcloudio/uni-ui/lib/uni-nav-bar/uni-nav-bar.vue'
import Icon from './Icon.vue'
import { back as goBack } from '../lib/navigation'
import { readSafeArea } from '../lib/safe-area'
withDefaults(defineProps<{ title?: string; subtitle?: string; eyebrow?: string; back?: boolean; main?: boolean }>(), { title: '', back: true, main: false })
const sideWidth = Math.max(44, readSafeArea().capsuleRightInset)
// 顶栏44px高度及胶囊避让为原生点击区域，保留真实像素；内部标题与图标按rpx适配。
let native = false
// #ifdef MP-WEIXIN
native = true
// #endif
</script>
<style scoped>
.page-navigation{position:sticky;top:0;z-index:30;flex-shrink:0;background:var(--bg);border-bottom:1px solid var(--line-subtle)}
.main-navigation{border-bottom:0}.navigation-title{display:flex;align-items:center;justify-content:center;width:100%;font-size:30.7692rpx;font-weight:650;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.brand-title{display:flex;align-items:center;gap:15.3846rpx;font-size:40.3846rpx;font-weight:800;letter-spacing:1.9231rpx}.brand-square{display:flex;align-items:center;justify-content:center;width:50rpx;height:50rpx;border-radius:13.4615rpx;background:var(--accent);color:#071b13;font-size:32.6923rpx}
.page-navigation :deep(.uni-navbar__header),.page-navigation :deep(.uni-navbar__header-container){padding-left:19.2308rpx;padding-right:19.2308rpx}
</style>
