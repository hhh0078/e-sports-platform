<!-- 文件作用：按共享布局算法渲染游戏封面和互不重叠的文字标签。 -->
<template>
	<view class="cover-art" :style="coverStyle">
		<image v-if="src" class="cover-image" :src="assetUrl(src)" mode="aspectFill" :lazy-load="lazyLoad" />
		<view v-if="shade" class="cover-shade" />
		<view v-for="label in arranged" :key="label.id" :class="['cover-label',{'cover-label-readable':readable}]" :style="labelStyle(label)">{{ label.text }}
		</view>
		<slot />
	</view>
</template>
<script setup lang="ts">
	import {
		computed,
		getCurrentInstance,
		nextTick,
		onMounted,
		onBeforeUnmount,
		watch,
		ref
	} from 'vue'
	import {
		onShow,
		onHide
	} from '@dcloudio/uni-app'
	import {
		assetUrl
	} from '../lib/format'
	import {
		layoutLabels
	} from '../lib/cover'
	import type { CSSProperties } from 'vue'
	import type { CoverLabel } from '../types/models'
	import type { PlacedLabel } from '../lib/cover'
	import {layoutReadableCover} from '../lib/cover-readable'
	import { useViewportWidth } from '../lib/viewport'
	const props = withDefaults(defineProps<{ src?: string; labels?: CoverLabel[]; shade?: boolean; reserveBottom?: number; reserveBottomPixels?: number; reserveTopPixels?: number; minLabelAreaPixels?: number; aspectRatio?: number;readable?:boolean;minFontPixels?:number; active?: boolean; lazyLoad?: boolean }>(), { labels: () => [], shade: true, reserveBottom: 0, reserveBottomPixels: 0, reserveTopPixels: 0, minLabelAreaPixels: 0, aspectRatio: 16 / 9,readable:false,minFontPixels:10, active: true, lazyLoad: false })
	const viewportWidth = useViewportWidth()
	const width = ref(0)
	const instance = getCurrentInstance()
	const ratio = computed(() => Number.isFinite(props.aspectRatio) && props.aspectRatio >= .5 && props.aspectRatio <= 4 ? props.aspectRatio : 16 / 9)
	/** 实际像素按测量宽度转为360画布单位，旧reserveBottom继续保留原单位。 */
	const reserved = computed(() => Math.max(Number.isFinite(props.reserveBottom) ? Math.max(0, props.reserveBottom) : 0, width.value > 0 && Number.isFinite(props.reserveBottomPixels) ? Math.max(0, props.reserveBottomPixels) * 360 / width.value : 0))
	/** 沉浸封面中的用户文字避开状态栏、胶囊与悬浮返回按钮。 */
	const reservedTop = computed(() => width.value > 0 && Number.isFinite(props.reserveTopPixels) ? Math.max(0, props.reserveTopPixels) * 360 / width.value : 0)
	/** 详情明确要求可读文字带时才增高，默认0保持所有卡片及旧调用的原比例。 */
	const baseCanvasHeight = computed(() => width.value > 0 && Number.isFinite(props.minLabelAreaPixels) && props.minLabelAreaPixels > 0 ? Math.max(360 / ratio.value, reservedTop.value + reserved.value + (props.minLabelAreaPixels + 16) * 360 / width.value) : 360 / ratio.value)
	/** 可读模式以真实像素计算，拥挤时只增高图片，不缩小或隐藏用户文字。 */
	const readableLayout=computed(()=>layoutReadableCover(props.labels||[],width.value,baseCanvasHeight.value*width.value/360,reserved.value*width.value/360,reservedTop.value*width.value/360,props.minFontPixels))
	const canvasHeight=computed(()=>props.readable&&width.value>0?readableLayout.value.height*360/width.value:baseCanvasHeight.value)
	const coverStyle = computed<CSSProperties>(() => ({ paddingTop: canvasHeight.value / 360 * 100 + '%' }))
	const arranged = computed(() =>props.readable?readableLayout.value.labels:layoutLabels(props.labels || [], reserved.value, canvasHeight.value, reservedTop.value))
	let mounted = false, pageVisible = true, measureQueued = false, measurementVersion = 0
	let resizeObserver: ResizeObserver | undefined
	/** 将画布坐标转换为当前卡片的相对位置与字号。 */
	function labelStyle(label: PlacedLabel): CSSProperties {

		return {
			left: label.left + '%',
			top: label.top + '%',
			width: label.width + '%',
			height: label.height + '%',
			color: label.color || '#ffffff',
			background: label.background || 'transparent',
			fontSize: (label.font_size * width.value / 360) + 'px',
			visibility: width.value ? 'visible' : 'hidden'
		}
	}
	/** 仅可见且已挂载的封面允许测量，隐藏标签页不会跨层读取布局。 */
	function canMeasure(): boolean { return mounted && pageVisible && props.active }
	/** 同一轮尺寸变化合并为一次测量，隐藏和卸载后的迟到结果不能改变已有布局。 */
	function measure() {
		if (!canMeasure()) return
		measurementVersion++
		if (measureQueued) return
		measureQueued = true
		void nextTick(() => {
			measureQueued = false
			const version = measurementVersion
			if (canMeasure() && instance?.proxy) uni.createSelectorQuery().in(instance.proxy).select('.cover-art').boundingClientRect(
				rect => {
					if (!Array.isArray(rect) && canMeasure() && version === measurementVersion && rect?.width) width.value = rect.width
				}).exec()
		})
	}
	/** 显示时恢复容器监听并重测，隐藏时立即停止，不监听无关标签页修订号。 */
	function synchronizeMeasuring() {
		measurementVersion++
		if (!canMeasure()) { resizeObserver?.disconnect(); resizeObserver = undefined; return }
		// #ifdef H5
		if (!resizeObserver && typeof ResizeObserver !== 'undefined' && instance?.proxy?.$el?.nodeType === 1) {
			resizeObserver = new ResizeObserver(measure)
			resizeObserver.observe(instance.proxy.$el)
		}
		// #endif
		measure()
	}
	/** 卸载时清理尺寸监听，防止隐藏旧组件继续更新。 */
	function stopMeasuring() {
		mounted = false
		measurementVersion++
		resizeObserver?.disconnect()
	}
	onMounted(() => { mounted = true; synchronizeMeasuring() })
	watch(() => props.active, synchronizeMeasuring)
	watch(viewportWidth, measure)
	onShow(() => { pageVisible = true; synchronizeMeasuring() })
	onHide(() => { pageVisible = false; synchronizeMeasuring() })
	onBeforeUnmount(stopMeasuring)
</script>
<style scoped>
	.cover-art {
		position: relative;
		z-index: 0;
		width: 100%;
		padding-top: 56.25%;
		overflow: hidden;
		background: #1b2621
	}

	.cover-image {
		position: absolute;
		z-index: 0;
		inset: 0;
		width: 100%;
		height: 100%;
		pointer-events: none
	}

	.cover-shade {
		position: absolute;
		z-index: 1;
		inset: 0;
		pointer-events: none;
		background: linear-gradient(180deg, rgba(0, 0, 0, 0) 38%, var(--surface) 100%)
	}

	.cover-label {
		position: absolute;
		z-index: 2;
		display: flex;
		align-items: center;
		padding: 0 1.3%;
		line-height: 1.1;
		font-weight: 750;
		border-radius: 5.7692rpx;
		box-sizing: border-box;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		pointer-events: none
	}
	.cover-label-readable{padding:5.7692rpx 11.5385rpx;white-space:normal;word-break:break-all;overflow:visible;text-overflow:clip;line-height:1.25}
</style>
