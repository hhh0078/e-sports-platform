<!-- 文件作用：二级页面的统一主题和顶部返回框架；主导航仅由主页面承载。 -->
<template>
  <view class="app-frame secondary-shell" :class="{ immersive }" :style="themeStyle">
    <FloatingBack v-if="immersive" />
    <PageHeader v-else :title="title || '开机电竞'" back />
    <view class="app-body"><slot name="header" /><slot /></view>
    <PhoneVerification />
  </view>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { pageTheme } from '../lib/store'
import PageHeader from './PageHeader.vue'
import FloatingBack from './FloatingBack.vue'
import PhoneVerification from './PhoneVerification.vue'
const props = withDefaults(defineProps<{ title?: string; subtitle?: string; active?: string; back?: boolean; immersive?: boolean; hideNav?: boolean; gameId?: string }>(), { title: '', immersive: false })
const themeStyle = computed(() => pageTheme(props.gameId))
</script>
<style scoped>
.secondary-shell{min-height:100vh}.app-body{max-width:600px;margin:0 auto;padding:30.7692rpx 30.7692rpx calc(46.1538rpx + env(safe-area-inset-bottom));min-width:0}.immersive .app-body{padding:0 0 calc(46.1538rpx + env(safe-area-inset-bottom))}
</style>
