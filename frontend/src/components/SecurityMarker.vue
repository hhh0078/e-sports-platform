<!-- 文件作用：仅展示真实押金或认证的小黑金盾牌，轮廓与文字独立渲染并统一尺寸。 -->
<template><view class="security-marker" :class="{'security-marker-wide':label.length>2}" :aria-label="description||label"><image class="security-marker-shape" src="/static/icons/security-pendant.svg" mode="scaleToFill"/><view class="security-marker-content"><Icon name="shield-filled" :size="10"/><text class="security-marker-text">{{label}}</text></view></view></template>
<script setup lang="ts">
import Icon from './Icon.vue'
defineProps<{label:string;description?:string}>()
</script>
<style scoped>
.security-marker{position:relative;display:inline-flex;vertical-align:top;align-items:flex-start;justify-content:center;flex-shrink:0;width:46.1538rpx;height:61.5385rpx;max-width:100%;color:#e5bd70;line-height:1}.security-marker-wide{width:53.8462rpx}.security-marker-shape{position:absolute;inset:0;width:100%;height:100%}.security-marker-content{position:relative;display:flex;flex-direction:column;align-items:center;justify-content:flex-start;gap:3.8462rpx;width:100%;padding-top:7.6923rpx}.security-marker-text{display:block;max-width:100%;font-size:15.3846rpx;font-weight:650;line-height:19.2308rpx;white-space:nowrap}
</style>
