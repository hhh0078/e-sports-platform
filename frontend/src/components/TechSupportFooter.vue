<!-- 文件作用：四个主视图复用的自然流技术支持入口，不占用固定底部导航。 -->
<template><view class="support-footer"><button class="support-footer-button" @click="emit('open')">该平台由开机电竞、瓜瓜提供技术支持</button></view></template>
<script setup lang="ts">
const emit=defineEmits<{open:[]}>()
</script>
<style scoped>
.support-footer{display:flex;align-items:center;justify-content:center;min-width:0;padding:46.1538rpx 30.7692rpx 38.4615rpx}.support-footer-button{max-width:100%;padding:0;background:transparent;color:var(--muted);opacity:.75;font-size:19.2308rpx;line-height:34.6154rpx;text-align:center;white-space:normal}
</style>
