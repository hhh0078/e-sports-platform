<!-- 文件作用：按共享布局算法渲染游戏封面和互不重叠的文字标签。 -->
<template>
	<view class="cover-art" :style="coverStyle">
		<image v-if="src" class="cover-image" :src="assetUrl(src)" mode="aspectFill" />
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
		onShow
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
	import { mainNavigation } from '../lib/navigation'
	const props = withDefaults(defineProps<{ src?: string; labels?: CoverLabel[]; shade?: boolean; reserveBottom?: number; reserveBottomPixels?: number; reserveTopPixels?: number; minLabelAreaPixels?: number; aspectRatio?: number;readable?:boolean;minFontPixels?:number }>(), { labels: () => [], shade: true, reserveBottom: 0, reserveBottomPixels: 0, reserveTopPixels: 0, minLabelAreaPixels: 0, aspectRatio: 16 / 9,readable:false,minFontPixels:10 })
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
	let active = false
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
	/** 测量实际宽度以保持封面预览与卡片文字位置一致。 */
	function measure() {
		nextTick(() => {
			if (active && instance?.proxy) uni.createSelectorQuery().in(instance.proxy).select('.cover-art').boundingClientRect(
				rect => {
					if (!Array.isArray(rect) && active && rect?.width) width.value = rect.width
				}).exec()
		})
	}
	/** 监听页面及容器尺寸变化，避免旋转屏幕后文字字号与坐标失配。 */
	function startMeasuring() {
		active = true
		measure()
		uni.onWindowResize?.(measure)
		// #ifdef H5
		if (typeof ResizeObserver !== 'undefined' && instance?.proxy?.$el?.nodeType === 1) {
			resizeObserver = new ResizeObserver(measure)
			resizeObserver.observe(instance.proxy.$el)
		}
		// #endif
	}
	/** 卸载时清理尺寸监听，防止隐藏旧组件继续更新。 */
	function stopMeasuring() {
		active = false;
		uni.offWindowResize?.(measure);
		resizeObserver?.disconnect()
	}
	onMounted(startMeasuring)
	watch(() => mainNavigation.revision, measure)
	onShow(measure)
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
		border-radius: 3px;
		box-sizing: border-box;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		pointer-events: none
	}
	.cover-label-readable{padding:3px 6px;white-space:normal;word-break:break-all;overflow:visible;text-overflow:clip;line-height:1.25}
</style>
