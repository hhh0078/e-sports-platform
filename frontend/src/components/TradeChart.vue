<!-- 文件作用：使用uni-app官方canvas绘制真实交易与退款双曲线，适配微信和H5，不引入图表依赖。 -->
<template><view class="trade-chart"><canvas :id="canvasID" :canvas-id="canvasID" class="trade-canvas"/><view class="chart-legend"><view class="legend-item"><view class="legend-dot paid"/><text>服务成交</text></view><view class="legend-item"><view class="legend-dot refund"/><text>成功退款</text></view></view></view></template>
<script setup lang="ts">
import { getCurrentInstance,onMounted,onBeforeUnmount,watch,nextTick } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import type { AdminTradeDay } from '../types/admin-analytics'
import { tradeChartGeometry,legacyCanvasColor } from '../lib/admin-chart'
const props=withDefaults(defineProps<{days:AdminTradeDay[];color?:string}>(),{color:'#8de8b6'})
const instance=getCurrentInstance(),canvasID='trade-'+instance?.uid
let disposed=false,version=0
/** 交易金额只用于坐标标注，按元或万元显示，避免高金额挤窄图形。 */
function axisAmount(cents:number):string{const yuan=cents/100;return yuan>=10000?(yuan/10000).toFixed(1).replace(/\.0$/,'')+'万':yuan>=100?String(Math.round(yuan)):yuan.toFixed(2).replace(/\.00$/,'').replace(/(\.\d)0$/,'$1')}
/** 每次重绘先测量真实容器，隐藏/销毁时的迟到测量不覆盖新图。 */
async function draw(){
  const revision=++version;await nextTick();if(disposed||!instance?.proxy)return
  uni.createSelectorQuery().in(instance.proxy as ComponentPublicInstance).select('.trade-canvas').boundingClientRect(rect=>{
    if(disposed||revision!==version||!rect||Array.isArray(rect)||typeof rect.width!=='number'||!rect.width)return
    const width=rect.width,height=196,geometry=tradeChartGeometry(props.days,width,height),context=uni.createCanvasContext(canvasID,instance.proxy as ComponentPublicInstance)
    context.clearRect(0,0,width,height);context.setFontSize(11);context.setTextBaseline('middle')
    for(let tick=0;tick<=2;tick++){const y=geometry.bottom-tick/2*(geometry.bottom-geometry.top);context.setStrokeStyle(legacyCanvasColor('#ffffff12'));context.setLineWidth(1);context.beginPath();context.moveTo(geometry.left,y);context.lineTo(geometry.right,y);context.stroke();context.setFillStyle('#91a299');context.setTextAlign('right');context.fillText(axisAmount(geometry.ceiling*tick/2),geometry.left-8,y)}
    /** 每个系列使用同一纵轴，退款不能叠加到成交额。 */
    function line(key:'paidY'|'refundY',color:string){context.setStrokeStyle(legacyCanvasColor(color));context.setLineWidth(2);context.beginPath();geometry.points.forEach((point,index)=>{if(index===0)context.moveTo(point.x,point[key]);else context.lineTo(point.x,point[key])});context.stroke();if(geometry.points.length===1){const point=geometry.points[0];context.setFillStyle(legacyCanvasColor(color));context.beginPath();context.arc(point.x,point[key],3,0,2*Math.PI);context.fill()}}
    line('paidY',props.color);line('refundY','#d7b276')
    context.setFillStyle('#91a299');context.setTextBaseline('top')
    for(const index of new Set([0,Math.floor((props.days.length-1)/2),props.days.length-1])){const point=geometry.points[index];if(!point)continue;context.setTextAlign(index===0?'left':index===props.days.length-1?'right':'center');context.fillText(point.date.slice(5).replace('-','/'),point.x,geometry.bottom+10)}
    context.draw()
  }).exec()
}
watch(()=>[props.days,props.color],()=>{void draw()},{deep:true})
onMounted(()=>{void draw();uni.onWindowResize(draw)})
onBeforeUnmount(()=>{disposed=true;version++;uni.offWindowResize(draw)})
</script>
<style scoped>
.trade-chart{width:100%;min-width:0}.trade-canvas{display:block;width:100%;height:196px}.chart-legend{display:flex;align-items:center;justify-content:center;gap:24px;margin-top:8px}.legend-item{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--muted)}.legend-dot{width:8px;height:8px;border-radius:50%}.paid{background:var(--accent)}.refund{background:#d7b276}
</style>
