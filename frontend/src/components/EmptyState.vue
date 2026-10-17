<!-- 文件作用：统一空内容、未登录和错误后重试的占位状态。 -->
<template><view class="empty-state"><view class="empty-icon"><Icon :name="icon" :size="30" /></view><text class="empty-title">{{ title }}</text><text v-if="detail" class="empty-detail">{{ detail }}</text><slot /></view></template>
<script setup>
import Icon from './Icon.vue'
defineProps({ title: { type: String, default: '暂时还没有内容' }, detail: String, icon: { type: String, default: 'orders' } })
</script>
<style scoped>.empty-state{display:flex;flex-direction:column;align-items:center;gap:12px;padding:48px 20px;text-align:center;border:1px dashed rgba(255,255,255,.1);border-radius:16px;background:rgba(255,255,255,.015)}.empty-icon{color:var(--accent);opacity:.8;margin-bottom:3px}.empty-title{font-size:15px;font-weight:600}.empty-detail{font-size:12px;color:#87988c;line-height:1.7;max-width:270px}</style>
