<!-- 文件作用：展示型二级页复用的圆形悬浮返回，统一避让原生状态栏与胶囊。 -->
<template><button class="floating-back" :style="{ top: Math.max(12, safe.floatingTop) + 'px' }" aria-label="返回" @click="back"><Icon name="back" :size="21" /></button></template>
<script setup lang="ts">
import Icon from './Icon.vue'
import { back } from '../lib/navigation'
import { readSafeArea } from '../lib/safe-area'
const safe = readSafeArea()
</script>
<style scoped>
.floating-back{position:fixed;left:16px;z-index:45;display:flex;align-items:center;justify-content:center;width:36px;height:36px;padding:0;border:1px solid #ffffff70;border-radius:50%;background:#07100c80;color:#fff;line-height:1}
</style>
