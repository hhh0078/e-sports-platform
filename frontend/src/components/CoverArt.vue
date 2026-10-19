<!-- 文件作用：按共享布局算法渲染游戏封面和互不重叠的文字标签。 -->
<template><view class="cover-art"><image v-if="src" class="cover-image" :src="assetUrl(src)" mode="aspectFill" /><view v-if="shade" class="cover-shade"/><view v-for="label in arranged" :key="label.id" class="cover-label" :style="labelStyle(label)">{{ label.text }}</view><slot /></view></template>
<script setup>
import { computed, getCurrentInstance, nextTick, onMounted, onBeforeUnmount, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { assetUrl } from '../lib/format'
import { layoutLabels } from '../lib/cover'
const props = defineProps({ src: String, labels: { type: Array, default: () => [] }, shade: { type: Boolean, default: true }, reserveBottom: { type: Number, default: 0 } })
const width = ref(0)
const instance = getCurrentInstance()
const arranged = computed(() => layoutLabels(props.labels || [], props.reserveBottom))
let active = false, resizeObserver
/** 将画布坐标转换为当前卡片的相对位置与字号。 */
function labelStyle(label) { return { left: label.left + '%', top: label.top + '%', width: label.width + '%', height: label.height + '%', color: label.color || '#ffffff', background: label.background || 'transparent', fontSize: (label.font_size * width.value / 360) + 'px', visibility: width.value ? 'visible' : 'hidden' } }
/** 测量实际宽度以保持封面预览与卡片文字位置一致。 */
function measure() { nextTick(() => { if (active) uni.createSelectorQuery().in(instance.proxy).select('.cover-art').boundingClientRect(rect => { if (active && rect?.width) width.value = rect.width }).exec() }) }
/** 监听页面及容器尺寸变化，避免旋转屏幕后文字字号与坐标失配。 */
function startMeasuring() {
  active = true
  measure()
  uni.onWindowResize?.(measure)
  // #ifdef H5
  if (typeof ResizeObserver !== 'undefined' && instance.proxy.$el?.nodeType === 1) {
    resizeObserver = new ResizeObserver(measure)
    resizeObserver.observe(instance.proxy.$el)
  }
  // #endif
}
/** 卸载时清理尺寸监听，防止隐藏旧组件继续更新。 */
function stopMeasuring() { active = false; uni.offWindowResize?.(measure); resizeObserver?.disconnect() }
onMounted(startMeasuring)
onShow(measure)
onBeforeUnmount(stopMeasuring)
</script>
<style scoped>.cover-art{position:relative;width:100%;padding-top:62.5%;overflow:hidden;background:#1b2621}.cover-image{position:absolute;inset:0;width:100%;height:100%;pointer-events:none}.cover-shade{position:absolute;inset:0;pointer-events:none;background:linear-gradient(180deg,rgba(0,0,0,0) 40%,rgba(0,0,0,.38))}.cover-label{position:absolute;display:flex;align-items:center;padding:0 1.3%;line-height:1.1;font-weight:750;border-radius:3px;box-sizing:border-box;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;pointer-events:none}</style>
