<!-- 文件作用：原图区域内的独立图片/文字轮播，实测安全带与稳定最大高度保证所有字幕可读且不覆盖品牌按钮。 -->
<template>
  <view class="content-carousel">
    <swiper class="carousel-images" :autoplay="canAnimate&&items.length>1" :interval="3000" :duration="350" :circular="items.length>1" :current="imageIndex" @change="imageChanged"><swiper-item v-for="slide in items" :key="slide.id"><image class="carousel-image" :src="assetUrl(slide.image_url)" mode="aspectFill" @click="open(slide)"/></swiper-item></swiper>
    <swiper class="carousel-labels" :current="textIndex" :duration="350" :circular="items.length>1" :disable-touch="true"><swiper-item v-for="(slide,index) in items" :key="slide.id"><view class="carousel-label-plane"><view v-for="label in arrangement.slides[index]||[]" :key="label.id" class="carousel-label" :style="labelStyle(label)">{{label.text}}</view></view></swiper-item></swiper>
    <view v-if="items.length>1" class="carousel-dots"><view v-for="(slide,index) in items" :key="slide.id" class="carousel-dot" :class="{'carousel-dot-active':index===imageIndex}"/></view>
  </view>
</template>
<script setup lang="ts">
import {computed,ref,watch,onBeforeUnmount} from 'vue'
import {onHide,onShow} from '@dcloudio/uni-app'
import {assetUrl} from '../lib/format'
import {openContentTarget} from '../lib/content'
import {layoutCarouselSlides} from '../lib/carousel-layout'
import type {CarouselGeometry} from '../lib/carousel-layout'
import type {PlacedLabel} from '../lib/cover'
import type {ContentSlide} from '../types/content'
import type {CSSProperties} from 'vue'
const props=withDefaults(defineProps<{slides:ContentSlide[];fallback?:string;active:boolean;geometry?:CarouselGeometry|null}>(),{fallback:'',geometry:null})
const emit=defineEmits<{'height-change':[height:number];'caption-change':[hasText:boolean]}>()
const items=computed<ContentSlide[]>(()=>props.slides.length?props.slides:props.fallback?[{id:'current-game-fallback',kind:'image',image_url:props.fallback,labels:[],target:null}]:[])
const arrangement=computed(()=>layoutCarouselSlides(items.value,props.geometry))
const imageIndex=ref(0),textIndex=ref(0),pageVisible=ref(true)
const canAnimate=computed(()=>props.active&&pageVisible.value)
const textHasLabels=computed(()=>!!arrangement.value.slides[textIndex.value]?.length)
let timer:ReturnType<typeof setTimeout>|undefined,disposed=false
/** 撤销过时文字切换，隐藏后不继续计时。 */
function clearTimer():void{if(timer!==undefined)clearTimeout(timer);timer=undefined}
/** 图片先切，文字晚300毫秒跟随，快速滑动只采纳最后一个有效索引。 */
function imageChanged(event:{detail:{current:number}}):void{const next=event.detail.current;if(!canAnimate.value||!Number.isInteger(next)||next<0||next>=items.value.length)return;clearTimer();imageIndex.value=next;timer=setTimeout(()=>{if(!disposed&&canAnimate.value&&next<items.value.length)textIndex.value=next;timer=undefined},300)}
/** 布局值来自真实像素；与共享CoverArt一致，保证320宽也不少于10实际像素。 */
function labelStyle(label:PlacedLabel):CSSProperties{return {left:label.left+'%',top:label.top+'%',width:label.width+'%',height:label.height+'%',color:label.color||'#fff',background:label.background||'transparent',fontSize:label.font_size*(props.geometry?.width||0)/360+'px'}}
/** 文字层不截获手势或点击；仅实际点击图片才按该图目标跳转。 */
function open(slide:ContentSlide):void{if(canAnimate.value)openContentTarget(slide.target)}
/** 换图组立即清理旧文字和timer，不把上一组字幕带到新游戏。 */
function resetSlides():void{clearTimer();imageIndex.value=0;textIndex.value=0}
/** 隐藏时停止原生自动轮播和计时，返回同步当前图片字幕。 */
function hide():void{pageVisible.value=false;clearTimer()}
/** 重新显示只恢复已选图片字幕，不延续隐藏前的迟到计时器。 */
function show():void{if(disposed)return;pageVisible.value=true;clearTimer();textIndex.value=imageIndex.value}
watch([()=>props.slides,()=>props.fallback],resetSlides,{flush:'sync'})
watch(()=>props.active,value=>{clearTimer();if(value&&pageVisible.value)textIndex.value=imageIndex.value},{flush:'sync'})
watch(()=>arrangement.value.height,value=>emit('height-change',value),{immediate:true,flush:'post'})
watch(textHasLabels,value=>emit('caption-change',value),{immediate:true,flush:'post'})
onHide(hide);onShow(show);onBeforeUnmount(()=>{disposed=true;hide()})
</script>
<style scoped>
.content-carousel,.carousel-images{position:absolute;inset:0;width:100%;height:100%;overflow:hidden}.carousel-image{display:block;width:100%;height:100%}.carousel-labels{position:absolute;inset:0;width:100%;height:100%;z-index:2;pointer-events:none}.carousel-label-plane{position:relative;width:100%;height:100%;pointer-events:none}.carousel-label{position:absolute;display:flex;align-items:center;justify-content:center;line-height:1.25;font-weight:700;padding:5.7692rpx 11.5385rpx;border-radius:5.7692rpx;box-sizing:border-box;white-space:normal;overflow-wrap:anywhere;pointer-events:none}.carousel-dots{position:absolute;right:23.0769rpx;bottom:13.4615rpx;z-index:4;display:flex;gap:7.6923rpx;pointer-events:none}.carousel-dot{width:9.6154rpx;height:9.6154rpx;border-radius:50%;background:rgba(255,255,255,.3)}.carousel-dot-active{width:26.9231rpx;border-radius:7.6923rpx;background:var(--accent)}
</style>
