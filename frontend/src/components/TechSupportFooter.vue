<!-- 文件作用：四个主视图复用的自然流技术支持入口，不占用固定底部导航。 -->
<template><view class="support-footer"><button class="support-footer-button" @click="emit('open')">该平台由开机电竞、瓜瓜提供技术支持</button></view></template>
<script setup lang="ts">
const emit=defineEmits<{open:[]}>()
</script>
<style scoped>
.support-footer{display:flex;align-items:center;justify-content:center;min-width:0;padding:24px 16px 20px}.support-footer-button{max-width:100%;padding:0;background:transparent;color:var(--muted);opacity:.75;font-size:10px;line-height:18px;text-align:center;white-space:normal}
</style>
