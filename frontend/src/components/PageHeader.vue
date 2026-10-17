<!-- 文件作用：二级页面的标题、说明和返回导航。 -->
<template><view class="page-header"><view v-if="back" class="back" @click="returnPage"><Icon name="back" :size="19" /><text>返回</text></view><text v-if="eyebrow" class="eyebrow">{{ eyebrow }}</text><view class="page-heading">{{ title }}</view><view v-if="subtitle" class="muted intro">{{ subtitle }}</view></view></template>
<script setup>
import Icon from './Icon.vue'
import { go } from '../lib/format'
defineProps({ title: String, subtitle: String, eyebrow: String, back: Boolean })
/** 优先返回上一页，无历史时跳转首页。 */
function returnPage() { if (getCurrentPages().length > 1) uni.navigateBack(); else go('/pages/index/index') }
</script>
<style scoped>.page-header{padding:8px 0 24px}.back{display:flex;align-items:center;gap:3px;color:#9baaa0;font-size:11px;margin-bottom:16px}.eyebrow{display:block;margin-bottom:8px}.intro{margin-top:10px;line-height:1.7;max-width:400px}</style>
