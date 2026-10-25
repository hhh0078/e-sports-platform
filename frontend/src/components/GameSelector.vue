<!-- 文件作用：复用首页游戏网格的官方底部选择器，单选即时回填、多选确认回填，始终与主游戏偏好隔离。 -->
<template>
  <view class="game-selector-control">
    <button v-if="props.field" class="input selector-trigger" :disabled="props.disabled || !props.active" @click="open"><image v-if="selectedIcon" class="selector-game-icon" :src="assetUrl(selectedIcon)" mode="aspectFill"/><Icon v-else name="game" :size="24"/><text class="selector-summary">{{ summary }}</text><Icon name="arrow" :size="14"/></button>
    <view class="selector-overlay">
      <UniPopup ref="popup" type="bottom" background-color="transparent" mask-background-color="rgba(0,0,0,0.6)" :safe-area="false" @change="popupChanged">
        <view class="catalog-sheet">
          <view class="catalog-heading"><view class="catalog-title-group"><text class="catalog-title">{{ props.title }}</text><text v-if="props.multiple" class="catalog-count">已选 {{ draft.length }} 项</text></view><button class="icon-btn selector-close" aria-label="关闭游戏选择" @click="close"><Icon name="close" :size="22"/></button></view>
          <view class="catalog-search"><Icon name="search" :size="17"/><input v-model="search" class="catalog-input" placeholder="搜索游戏名称"/></view>
          <scroll-view scroll-y class="catalog-scroll" :show-scrollbar="false"><view class="catalog-grid"><button v-for="item in filteredGames" :key="item.id" :data-id="item.id" :class="['catalog-game',{selected:isSelected(item.id),unavailable:!item.available}]" :disabled="!item.available&&!props.multiple" @click="choose(item.id)"><view class="catalog-icon-wrap"><image v-if="item.icon_url||item.cover_url" class="catalog-game-icon" :src="assetUrl(item.icon_url||item.cover_url)" mode="aspectFill"/><Icon v-else name="game" :size="32"/><view v-if="props.multiple&&isSelected(item.id)" class="catalog-check"><Icon name="check" :size="11"/></view></view><text class="catalog-game-name">{{ item.name }}</text><text v-if="!item.available" class="catalog-game-hint">已停用{{props.multiple?' · 可取消':''}}</text></button></view><text v-if="!filteredGames.length" class="catalog-empty">{{search.trim()?'没有找到这个游戏':'暂无可选游戏'}}</text></scroll-view>
          <view v-if="props.multiple" class="selector-actions"><button class="text-btn selector-cancel" @click="close">取消</button><button class="btn selector-confirm" :disabled="!canConfirm" @click="confirm">确认选择</button></view>
        </view>
      </UniPopup>
    </view>
  </view>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import Icon from './Icon.vue'
import { state, sessionRevision } from '../lib/store'
import { token } from '../api/client'
import { assetUrl, notify } from '../lib/format'

/** 受控选择值只由父页面提供，组件不写全局筛选与缓存。 */
interface Props { value?: string; values?: string[]; multiple?: boolean; disabled?: boolean; active?: boolean; field?: boolean; title?: string; placeholder?: string; minSelections?: number; maxSelections?: number }
/** 游戏选项保留停用或历史未知项，避免编辑俱乐部时无声丢失。 */
interface GameOption { id: string; name: string; short_name: string; icon_url: string; cover_url: string; available: boolean }
const props=withDefaults(defineProps<Props>(),{value:'',values:()=>[],multiple:false,disabled:false,active:true,field:true,title:'选择游戏',placeholder:'请选择游戏',minSelections:1,maxSelections:32})
const emit=defineEmits<{change:[id:string];'update:values':[ids:string[]]}>()
const popup=ref<{open:(type?:string)=>void;close:()=>void;showPopup?:boolean}|null>(null),opened=ref(false),search=ref(''),draft=ref<string[]>([])
let visible=true,disposed=false,operation=0,presentedOperation=-1,session:ReturnType<typeof currentSession>|null=null
/** 去重但保留父表单顺序，不直接修改传入数组。 */
function unique(ids:readonly string[]):string[]{return [...new Set(ids.filter(id=>typeof id==='string'&&!!id))]}
const selectedIds=computed(()=>props.multiple?unique(props.values):props.value?[props.value]:[])
const selectedIcon=computed(()=>props.multiple?'':state.games.find(item=>item.id===props.value)?.icon_url||'')
const summary=computed(()=>{const ids=selectedIds.value;if(!ids.length)return props.placeholder;const names=ids.slice(0,2).map(id=>state.games.find(item=>item.id===id)?.name||'已停用游戏');return names.join('、')+(ids.length>2?' 等'+ids.length+'项':'')})
const options=computed<GameOption[]>(()=>{const games:GameOption[]=state.games.filter(item=>item.enabled!==false).map(item=>({...item,available:true}));for(const id of opened.value?draft.value:selectedIds.value){if(games.some(item=>item.id===id))continue;const old=state.games.find(item=>item.id===id);games.push({id,name:old?.name||id,short_name:old?.short_name||'',icon_url:old?.icon_url||'',cover_url:old?.cover_url||'',available:false})}return games})
const filteredGames=computed(()=>{const query=search.value.trim().toLowerCase();return options.value.filter(item=>(item.name+item.short_name+item.id).toLowerCase().includes(query))})
const canConfirm=computed(()=>draft.value.length>=props.minSelections&&draft.value.length<=props.maxSelections)
/** 记录打开时的身份，迟到选择不能回填切换后的账号。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 仅当前可见、启用且身份不变的弹框允许提交。 */
function current():boolean{return opened.value&&visible&&!disposed&&props.active&&!props.disabled&&!!session&&session.id===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 官方实例常驻；微信延迟绑定ref或旧关闭动画结束后，仍可展示当前有效操作。 */
function presentPopup():void{const instance=popup.value;if(!instance||typeof instance.open!=='function'||instance.showPopup||!current()||presentedOperation===operation)return;presentedOperation=operation;instance.open('bottom')}
/** 每次打开复制父值并清空搜索，不通过v-if重新创建原生官方组件。 */
function open():void{if(opened.value||!visible||disposed||props.disabled||!props.active)return;session=currentSession();draft.value=[...selectedIds.value];search.value='';opened.value=true;const revision=++operation;void nextTick(()=>{if(revision===operation)presentPopup()})}
/** 已关闭的官方实例不再close，避免其300ms旧定时器误关重显后的新选择。 */
function close():void{operation++;opened.value=false;session=null;draft.value=[];const instance=popup.value;if(instance&&instance.showPopup!==false)instance.close()}
/** 官方遮罩或关闭事件同步作废选择，阻止动画期间的迟到点击。 */
function popupChanged(event:{show:boolean}):void{if(!event.show&&opened.value)close()}
/** 当前临时值决定选中态，单选与多选共用同一网格。 */
function isSelected(id:string):boolean{return draft.value.includes(id)}
/** 选择有效游戏；多选仅改临时副本，单选立即发给父页面。 */
function choose(id:string):void{if(!current()){close();return}const item=options.value.find(game=>game.id===id);if(!item)return;if(props.multiple){const index=draft.value.indexOf(id);if(index>=0)draft.value.splice(index,1);else if(item.available){if(draft.value.length>=props.maxSelections)return notify('最多选择'+props.maxSelections+'个游戏');draft.value.push(id)}return}if(!item.available)return;emit('change',id);close()}
/** 确认多选后才回填独立数组，最少与最多数量继续与业务约束一致。 */
function confirm():void{if(!current()||!props.multiple){close();return}if(!canConfirm.value)return notify('请选择'+props.minSelections+'—'+props.maxSelections+'个游戏');emit('update:values',[...draft.value]);close()}
/** 页面隐藏即撤销临时选择，重显后允许重新打开。 */
function hide():void{visible=false;close()}
/** 组件销毁后禁止异步挂载或选择回写。 */
function dispose():void{disposed=true;hide()}
watch([()=>props.active,()=>props.disabled,()=>state.user?.id,sessionRevision],()=>{if(opened.value)close()},{flush:'sync'})
watch(()=>JSON.stringify([props.value,props.values]),()=>{if(opened.value)close()},{flush:'sync'})
watch([popup,()=>popup.value?.showPopup],presentPopup,{flush:'post'})
onShow(()=>{visible=true})
onHide(hide)
onUnload(dispose)
onBeforeUnmount(dispose)
defineExpose({open,close,summary})
</script>
<style scoped>
.game-selector-control{width:100%;min-width:0;max-width:100%}.selector-trigger{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:44px;margin:0;text-align:left}.selector-game-icon{display:block;flex-shrink:0;width:53.8462rpx;height:53.8462rpx;border-radius:11.5385rpx}.selector-summary{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--text);font-size:26.9231rpx;line-height:42.3077rpx}.selector-overlay{position:relative;z-index:180}.catalog-sheet{display:flex;flex-direction:column;width:100vw;max-width:600px;max-height:85vh;margin:0 auto;box-sizing:border-box;background:var(--surface);border:1px solid var(--line);border-radius:34.6154rpx 34.6154rpx 0 0;padding:23.0769rpx 30.7692rpx calc(38.4615rpx + env(safe-area-inset-bottom));color:var(--text);overflow:hidden}.catalog-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;flex-shrink:0}.catalog-title-group{display:flex;flex-direction:column;gap:7.6923rpx;min-width:0}.catalog-title{font-size:36.5385rpx;line-height:51.9231rpx;font-weight:700}.catalog-count{font-size:23.0769rpx;line-height:34.6154rpx;color:var(--muted)}.selector-close{flex-shrink:0;width:84.6154rpx;height:84.6154rpx;color:var(--text)}.catalog-search{display:flex;align-items:center;gap:15.3846rpx;flex-shrink:0;min-height:44px;box-sizing:border-box;padding:15.3846rpx 23.0769rpx;margin:23.0769rpx 0;border:1px solid var(--line);border-radius:19.2308rpx;color:var(--muted)}.catalog-input{flex:1;min-width:0;width:0;color:var(--text);font-size:26.9231rpx;line-height:normal}.catalog-scroll{flex-shrink:1;min-height:0;height:692.3077rpx;max-height:55vh}.catalog-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:23.0769rpx}.catalog-game{display:flex;flex-direction:column;align-items:center;gap:15.3846rpx;min-width:0;margin:0;padding:19.2308rpx 3.8462rpx;background:transparent;color:var(--text);font-size:26.9231rpx;line-height:38.4615rpx;border:1px solid transparent;border-radius:19.2308rpx}.catalog-icon-wrap{position:relative;display:flex;align-items:center;justify-content:center;width:103.8462rpx;height:103.8462rpx;flex-shrink:0}.catalog-game-icon{display:block;width:103.8462rpx;height:103.8462rpx;border-radius:19.2308rpx}.catalog-game-name{display:-webkit-box;max-width:100%;min-width:0;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow-wrap:anywhere;text-align:center}.catalog-game.selected{color:var(--accent);border-color:var(--accent)}.catalog-check{position:absolute;top:-5.7692rpx;right:-5.7692rpx;display:flex;align-items:center;justify-content:center;width:32.6923rpx;height:32.6923rpx;border-radius:9.6154rpx;background:var(--accent);color:#0a2116}.catalog-game.unavailable{color:var(--muted)}.catalog-game-hint{font-size:19.2308rpx;line-height:30.7692rpx}.catalog-empty{display:block;padding:61.5385rpx 0;text-align:center;color:var(--muted);font-size:26.9231rpx;line-height:42.3077rpx}.selector-actions{display:flex;align-items:center;gap:30.7692rpx;flex-shrink:0;padding-top:30.7692rpx;border-top:1px solid var(--line-subtle);margin-top:23.0769rpx}.selector-cancel{flex:1;min-height:44px;color:var(--muted);font-size:26.9231rpx}.selector-confirm{flex:2;min-width:0;font-size:26.9231rpx}
</style>
