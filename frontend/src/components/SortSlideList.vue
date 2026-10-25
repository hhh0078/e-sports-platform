<!-- 文件作用：使用uni-app官方movable-area/view长按拖动轮播排序，保留上移下移辅助入口。 -->
<template><view><text v-if="!modelValue.length" class="subtle-note">暂无轮播，新增图片后可长按右侧手柄排序。</text><movable-area v-else class="sort-area" :style="{height:rowRpx*modelValue.length+'rpx'}"><movable-view v-for="(slide,index) in modelValue" :key="slide.id+':'+layoutVersion" class="sort-row" :class="{'sort-row-dragging':dragIndex===index}" direction="vertical" :disabled="dragIndex!==index" :x="0" :y="index*rowPixels" :animation="true" :damping="35" @change="moved($event)" @touchend="finish" @touchcancel="cancel"><view class="sort-row-body"><image v-if="slide.kind==='image'&&slide.image_url" class="sort-thumb" :src="assetUrl(slide.image_url)" mode="aspectFill"/><view v-else class="sort-thumb sort-placeholder"><Icon name="game" :size="26"/></view><view class="sort-copy"><text class="sort-title">{{slide.kind==='game_slot'?'当前游戏轮播':slide.labels?.[0]?.text||'图片轮播'}}</text><text class="sort-subtitle">{{slide.kind==='game_slot'?'此位置展开所选游戏的轮播':slide.target?'点击跳转'+targetKind(slide):'未设置跳转'}}</text><view class="sort-actions"><button class="text-btn" :disabled="index===0" @click="step(index,-1)">上移</button><button class="text-btn" :disabled="index===modelValue.length-1" @click="step(index,1)">下移</button><button v-if="slide.kind==='image'" class="text-btn" @click="emit('edit',index)">编辑</button><button class="text-btn danger" @click="emit('remove',index)">移除</button></view></view><view class="sort-handle" @longpress="begin(index)"><Icon name="bars" :size="22"/></view></view></movable-view></movable-area></view></template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import Icon from './Icon.vue'
import {assetUrl} from '../lib/format'
import {useViewportWidth} from '../lib/viewport'
import type {ContentSlide} from '../types/content'
const props=defineProps<{modelValue:ContentSlide[]}>(),emit=defineEmits<{'update:modelValue':[rows:ContentSlide[]];edit:[index:number];remove:[index:number]}>()
const rowRpx=211.5385,width=useViewportWidth(),rowPixels=computed(()=>rowRpx*width.value/750),dragIndex=ref(-1),layoutVersion=ref(0);let dropIndex=-1,dragRows:ContentSlide[]|null=null
/** 仅允许当前数组内的整数索引，防止负数splice误移动末项。 */
function validIndex(index:number):boolean{return Number.isInteger(index)&&index>=0&&index<props.modelValue.length}
/** 长按后只解锁当前官方拖动项，普通滚动不会重排。 */
function begin(index:number):void{if(!validIndex(index))return;dragIndex.value=index;dropIndex=index;dragRows=props.modelValue}
/** 官方y坐标为真实像素，仅将用户touch来源换算目标行。 */
function moved(event:{detail:{y:number;source?:string}}):void{if(dragIndex.value<0||event.detail.source!=='touch'||!Number.isFinite(event.detail.y)||!Number.isFinite(rowPixels.value)||rowPixels.value<=0)return;dropIndex=Math.max(0,Math.min(props.modelValue.length-1,Math.round(event.detail.y/rowPixels.value)))}
/** 松手才提交一次新顺序并恢复原生位置，避免拖动中循环回写。 */
function finish():void{if(dragIndex.value<0)return;const from=dragIndex.value,to=dropIndex,current=dragRows===props.modelValue;dragIndex.value=-1;dropIndex=-1;dragRows=null;if(current&&validIndex(from)&&validIndex(to)&&from!==to){const rows=props.modelValue.slice(),item=rows.splice(from,1)[0];if(item){rows.splice(to,0,item);emit('update:modelValue',rows)}}layoutVersion.value++}
/** 系统取消拖动不改变数组。 */
function cancel():void{dragIndex.value=-1;dropIndex=-1;dragRows=null;layoutVersion.value++}
/** 无障碍辅助按钮与拖动使用相同的唯一顺序。 */
function step(index:number,delta:number):void{if(!validIndex(index)||delta!==-1&&delta!==1)return;const to=index+delta;if(!validIndex(to))return;if(dragIndex.value>=0)cancel();const rows=props.modelValue.slice(),item=rows.splice(index,1)[0];if(item){rows.splice(to,0,item);emit('update:modelValue',rows)}}
/** 显示公开目标种类，不把ID当标题写死。 */
function targetKind(slide:ContentSlide):string{return slide.target?{service:'服务',request:'需求',article:'文章'}[slide.target.kind]:''}
watch(()=>props.modelValue,()=>{if(dragIndex.value>=0)cancel()})
</script>
<style scoped>
.sort-area{position:relative;width:100%;min-width:0}.sort-row{position:absolute;left:0;top:0;width:100%;height:200rpx;min-width:0;z-index:1}.sort-row-dragging{z-index:5}.sort-row-body{display:flex;align-items:center;gap:19.2308rpx;width:100%;height:100%;padding:19.2308rpx;border:1px solid var(--line-subtle);border-radius:19.2308rpx;background:var(--surface);box-sizing:border-box}.sort-row-dragging .sort-row-body{border-color:var(--accent);box-shadow:0 7.6923rpx 23.0769rpx rgba(0,0,0,.4)}.sort-thumb{width:153.8462rpx;height:86.5385rpx;border-radius:11.5385rpx;flex-shrink:0}.sort-placeholder{display:flex;align-items:center;justify-content:center;background:rgba(131,231,183,.08);color:var(--accent)}.sort-copy{flex:1;min-width:0}.sort-title{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:26.9231rpx;font-weight:650}.sort-subtitle{display:block;margin-top:7.6923rpx;font-size:21.1538rpx;line-height:30.7692rpx;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.sort-actions{display:flex;flex-wrap:wrap;gap:0 11.5385rpx}.sort-actions .text-btn{font-size:19.2308rpx;min-height:46.1538rpx;padding:0}.sort-handle{display:flex;align-items:center;justify-content:center;width:69.2308rpx;height:107.6923rpx;flex-shrink:0;color:var(--muted)}
</style>
