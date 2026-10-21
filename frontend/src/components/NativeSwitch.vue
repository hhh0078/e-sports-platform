<!-- 文件作用：复用 uni-app 官方 switch，只统一真实 change 事件类型，不另绘开关界面。 -->
<template><switch :checked="checked" :disabled="disabled" :color="color" @change="nativeChange" /></template>
<script setup lang="ts">
import type { SwitchChangeEvent } from '../types/uni-components'
withDefaults(defineProps<{ checked?: boolean; disabled?: boolean; color?: string }>(), { checked: false, disabled: false, color: '#00e18a' })
const emit = defineEmits<{ change: [event: SwitchChangeEvent] }>()
/** 仅转发符合官方平台结构的 change，忽略缺少布尔值的无效事件。 */
function nativeChange(event: Event | SwitchChangeEvent): void {
  if (!('detail' in event) || typeof event.detail !== 'object' || event.detail === null || !('value' in event.detail) || typeof event.detail.value !== 'boolean') return
  emit('change', { detail: { value: event.detail.value } })
}
</script>
