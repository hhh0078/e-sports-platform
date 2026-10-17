<!-- 文件作用：统一官方 uni-icons 图标名称与尺寸。 -->
<template><uni-icons :type="types[name] || name" :size="size" :color="color" /></template>
<script setup>
import UniIcons from '@dcloudio/uni-ui/lib/uni-icons/uni-icons.vue'
defineProps({ name: { type: String, default: 'circle' }, size: { type: [Number, String], default: 20 }, color: { type: String, default: 'currentColor' } })
const types = { 'receipt': 'list', 'chevron-right': 'right', 'shield-check': 'auth', 'check-circle': 'checkbox', 'shield-alert': 'info', 'image': 'image', home: 'home', game: 'flag', work: 'staff', orders: 'list', user: 'person', arrow: 'right', back: 'left', search: 'search', shield: 'auth', check: 'checkmarkempty', plus: 'plusempty', close: 'closeempty', chat: 'chatbubble', settings: 'gear', club: 'staff', bell: 'notification', star: 'star-filled', clock: 'clock', edit: 'compose', logout: 'undo', wallet: 'wallet', photo: 'image', refresh: 'refresh', heart: 'heart', info: 'info', send: 'paperplane', download: 'download' }
</script>
