<!-- 文件作用：资金表单与规则共用官方遮罩弹层，微信端挂到原生根层，隔离页面布局和主题继承。 -->
<template>
  <!-- #ifdef MP-WEIXIN -->
  <root-portal>
  <!-- #endif -->
  <view class="finance-popup-layer" :style="popupTheme"><UniPopup ref="popup" type="center" background-color="transparent" mask-background-color="rgba(0,0,0,0.64)" :safe-area="false" :is-mask-click="!busy" @change="popupChanged"><view class="finance-popup-sheet"><view class="finance-popup-heading"><text class="finance-popup-title">{{ title }}</text><button class="finance-popup-close" :disabled="busy" aria-label="关闭弹框" @click="requestClose"><Icon name="close" :size="22"/></button></view><scroll-view class="finance-popup-scroll" scroll-y><view class="finance-popup-content"><slot/></view></scroll-view></view></UniPopup></view>
  <!-- #ifdef MP-WEIXIN -->
  </root-portal>
  <!-- #endif -->
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { onHide, onShow, onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { pageTheme, sessionRevision, state } from '../lib/store'
/** 官方弹层的公开方法与显示状态。 */
interface Popup { open:(type?:string)=>void;close:()=>void;showPopup?:boolean }
const props=withDefaults(defineProps<{opened:boolean;title:string;busy?:boolean}>(),{busy:false})
const emit=defineEmits<{close:[]}>()
const popup=ref<Popup|null>(null),intent=ref(props.opened)
const popupTheme=computed(()=>({'--control-height':'44px','--compact-height':'36px','--radius':'12px','--muted':'#9aada2','--line':'rgba(255,255,255,0.12)','--line-subtle':'rgba(255,255,255,0.07)',...pageTheme()}))
let visible=true,disposed=false,closing=false
/** 仅同步当前显示意图；延迟绑定和关闭动画结束不能复活已撤销的弹框。 */
async function syncPopup():Promise<void>{await nextTick();const instance=popup.value;if(!instance)return;if(!instance.showPopup)closing=false;const show=intent.value&&visible&&!disposed;if(show){if(!instance.showPopup&&!closing)instance.open('center')}else if(instance.showPopup&&!closing){closing=true;instance.close()}}
/** 用户关闭时保留资金提交锁，处理中不能把交易误当成已取消。 */
function requestClose():void{if(props.busy)return;intent.value=false;emit('close');void syncPopup()}
/** 官方遮罩关闭只撤销当前意图，程序关闭的同步事件不反向影响新表单。 */
function popupChanged(event:{show:boolean}):void{if(event.show){if(!intent.value||!visible||disposed)void syncPopup();return}if(closing)return;closing=true;if(!props.busy){intent.value=false;emit('close')}}
/** 隐藏和切换身份时强制撤销临时界面，资金原单仍由组合函数查询恢复。 */
function hide():void{visible=false;intent.value=false;emit('close');void syncPopup()}
watch(()=>props.opened,value=>{intent.value=value;void syncPopup()},{flush:'sync'})
watch([popup,()=>popup.value?.showPopup],syncPopup,{flush:'post'})
watch([()=>state.user?.id,sessionRevision],()=>{intent.value=false;emit('close');void syncPopup()},{flush:'sync'})
onShow(()=>{visible=true})
onHide(hide)
onUnload(()=>{disposed=true;hide()})
onBeforeUnmount(()=>{disposed=true;hide()})
</script>
<style scoped>
.finance-popup-layer{position:relative;z-index:190}.finance-popup-sheet{width:calc(100vw - 32px);max-width:440px;max-height:82vh;box-sizing:border-box;overflow:hidden;border:1px solid var(--line,rgba(255,255,255,0.12));border-radius:16px;background:var(--surface,#14201b);color:var(--text,#edf4ef)}.finance-popup-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:20px 20px 12px}.finance-popup-title{flex:1;min-width:0;font-size:19px;line-height:28px;font-weight:700;overflow-wrap:anywhere}.finance-popup-close{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:28px;height:28px;padding:0;background:transparent;color:var(--muted,#9aada2)}.finance-popup-scroll{max-height:calc(82vh - 64px)}.finance-popup-content{padding:0 20px 20px;min-width:0;box-sizing:border-box}
</style>
