<!-- 文件作用：统一官方 uni-icons 的语义名称、固定占位和双轴居中。 -->
<template><view class="app-icon" :style="{ width: edge, height: edge, color }"><image v-if="name === 'game'" class="gamepad-image" src="/static/illustrations/gamepad-icon.png" mode="aspectFit"/><UniIcons v-else :type="types[name] || name" :size="size" :color="color" /></view></template>
<script setup lang="ts">
import { computed } from 'vue'
import UniIcons from '@dcloudio/uni-ui/lib/uni-icons/uni-icons.vue'
const props = withDefaults(defineProps<{ name?: string; size?: number | string; color?: string }>(), { name: 'circle', size: 20, color: 'currentColor' })
const edge = computed(() => typeof props.size === 'number' || /^\d+(\.\d+)?$/.test(props.size) ? `${props.size}px` : props.size)
const types: Record<string, string> = { receipt: 'list', 'chevron-right': 'right', 'shield-check': 'auth', 'check-circle': 'checkbox', 'shield-alert': 'info', home: 'home', game: 'flag', work: 'staff', orders: 'list', user: 'person', arrow: 'right', back: 'left', search: 'search', shield: 'auth', check: 'checkmarkempty', plus: 'plusempty', minus: 'minus', close: 'closeempty', chat: 'chatbubble', settings: 'gear', club: 'staff', bell: 'notification', star: 'star-filled', clock: 'refreshempty', edit: 'compose', logout: 'undo', wallet: 'wallet', photo: 'image', refresh: 'refresh', heart: 'heart', info: 'info', send: 'paperplane', download: 'download', phone: 'phone', upload: 'redo', headphones: 'headphones' }
</script>
<style scoped>
.app-icon{display:inline-flex;align-items:center;justify-content:center;vertical-align:middle;line-height:1;flex-shrink:0}.app-icon :deep(.uni-icons){display:flex;align-items:center;justify-content:center;line-height:1}
/* 官方图标库没有手柄，使用独立透明素材，继续复用固定居中外盒。 */
.gamepad-image{display:block;width:100%;height:100%}
</style>
