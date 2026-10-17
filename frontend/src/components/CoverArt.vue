<!-- 文件作用：按共享布局算法渲染游戏封面和互不重叠的文字标签。 -->
<template><view class="cover-art"><image v-if="src" class="cover-image" :src="assetUrl(src)" mode="aspectFill" /><view class="cover-shade"/><view v-for="label in arranged" :key="label.id" class="cover-label" :style="labelStyle(label)">{{ label.text }}</view><slot /></view></template>
<script setup>
import { computed, getCurrentInstance, nextTick, onMounted, ref } from 'vue'
import { assetUrl } from '../lib/format'
import { layoutLabels } from '../lib/cover'
const props = defineProps({ src: String, labels: { type: Array, default: () => [] } })
const width = ref(180)
const instance = getCurrentInstance()
const arranged = computed(() => layoutLabels(props.labels || []))
/** 将画布坐标转换为当前卡片的相对位置与字号。 */
function labelStyle(label) { return { left: label.left + '%', top: label.top + '%', width: label.width + '%', height: label.height + '%', color: label.color || '#ffffff', background: label.background || 'transparent', fontSize: ((label.font_size || 18) * width.value / 360) + 'px' } }
/** 测量实际宽度以保持封面预览与卡片文字位置一致。 */
function measure() { nextTick(() => uni.createSelectorQuery().in(instance.proxy).select('.cover-art').boundingClientRect(rect => { if (rect?.width) width.value = rect.width }).exec()) }
onMounted(measure)
</script>
<style scoped>.cover-art{position:relative;width:100%;padding-top:62.5%;overflow:hidden;background:#1b2621}.cover-image{position:absolute;inset:0;width:100%;height:100%}.cover-shade{position:absolute;inset:0;background:linear-gradient(180deg,rgba(0,0,0,0) 40%,rgba(0,0,0,.38))}.cover-label{position:absolute;display:flex;align-items:center;padding:0 1.3%;line-height:1.1;font-weight:750;border-radius:3px;box-sizing:border-box;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}</style>
