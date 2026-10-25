<!-- 文件作用：管理员按ID或名称搜索三类公开跳转目标，复用真实弹层并隔离账号和迟到回包。 -->
<template><view><button class="target-trigger" @click="begin"><view class="target-trigger-copy"><text>{{modelValue?kindName(modelValue.kind):'无跳转'}}</text><text v-if="modelValue" class="target-id">{{modelValue.id}}</text></view><Icon name="arrow" :size="14"/></button><FinancePopup :opened="opened" title="选择跳转目标" @close="close"><view class="tabs"><button v-for="item in kinds" :key="item" class="tab" :class="{active:kind===item}" @click="chooseKind(item)">{{kindName(item)}}</button></view><view class="target-search"><input v-model="query" class="target-search-input" placeholder="输入ID或名称搜索" confirm-type="search" @confirm="load()"/><button class="text-btn" @click="load()">搜索</button></view><scroll-view scroll-y class="target-results" @scrolltolower="load(true)"><button v-for="item in results" :key="item.kind+item.id" class="target-option" @click="select(item)"><image v-if="item.image_url" class="target-thumb" :src="assetUrl(item.image_url)" mode="aspectFill"/><view class="target-copy"><text class="target-title">{{item.title}}</text><text class="target-id">{{item.subtitle||item.id}}</text></view><Icon v-if="modelValue?.kind===item.kind&&modelValue.id===item.id" name="checkmarkempty" :size="16"/></button><ListLoadState :loading="loading" :has-more="offset<total" :error="error" @retry="retryFailed"/></scroll-view><button class="text-btn" @click="clear">清除跳转</button></FinancePopup></view></template>
<script setup lang="ts">
import {ref,watch,onBeforeUnmount} from 'vue'
import {onShow,onHide} from '@dcloudio/uni-app'
import FinancePopup from './FinancePopup.vue'
import ListLoadState from './ListLoadState.vue'
import Icon from './Icon.vue'
import {searchContentTargets} from '../api/content.api'
import {state,sessionRevision} from '../lib/store'
import {errorMessage,errorStatus,token} from '../api/client'
import {assetUrl} from '../lib/format'
import {appendUniqueRows} from '../lib/list-page'
import type {ContentTarget,ContentTargetOption} from '../types/content'
const props=withDefaults(defineProps<{modelValue:ContentTarget|null;kinds?:ContentTarget['kind'][]}>(),{kinds:()=>['service','request','article']})
const emit=defineEmits<{'update:modelValue':[value:ContentTarget|null]}>()
const opened=ref(false),kind=ref<ContentTarget['kind']>('service'),query=ref(''),results=ref<ContentTargetOption[]>([]),offset=ref(0),total=ref(0),loading=ref(false),error=ref('')
let version=0,submitted='',visible=true,disposed=false,owner:ReturnType<typeof session>|null=null,failed:{more:boolean;offset:number;query:string;kind:ContentTarget['kind']}|null=null
/** 只在有效平台管理员上下文内读取候选，不用昵称或客户端目标猜权限。 */
function allowed():boolean{return state.user?.role==='admin'&&state.user.status!=='frozen'}
/** 捕获发起账号与权限，旧原生弹层回调不得作用于新账号。 */
function session(){return{id:state.user?.id,role:state.user?.role,adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(),revision:sessionRevision()}}
/** 检查弹层归属、页面可见和完整认证修订。 */
function currentOwner():boolean{return !!owner&&!disposed&&visible&&opened.value&&allowed()&&owner.id===state.user?.id&&owner.role===state.user?.role&&owner.adminRole===state.user?.platform_admin_role&&owner.status===state.user?.status&&owner.token===token()&&owner.revision===sessionRevision()}
/** 展示三类站内目标，不允许任意URL。 */
function kindName(value:ContentTarget['kind']):string{return {service:'服务',request:'需求',article:'文章'}[value]}
/** 打开时读取实时公开候选，账号不是有效管理员时不调用私有搜索。 */
function begin():void{if(!visible||disposed||!allowed()||!props.kinds.length)return;close();kind.value=props.modelValue?.kind&&props.kinds.includes(props.modelValue.kind)?props.modelValue.kind:props.kinds[0];query.value='';owner=session();opened.value=true;void load()}
/** 关闭、隐藏、撤权后清理候选并废弃请求，旧finally不能释放下一次搜索。 */
function close():void{opened.value=false;version++;loading.value=false;results.value=[];offset.value=0;total.value=0;error.value='';failed=null;owner=null}
/** 切换种类清空旧候选，文章专用弹层不能选择其它业务目标。 */
function chooseKind(value:ContentTarget['kind']):void{if(!currentOwner()||!props.kinds.includes(value)||kind.value===value)return;kind.value=value;query.value='';results.value=[];offset.value=0;total.value=0;failed=null;void load()}
/** 真实分页保存失败方向与偏移，不用未提交文字或旧总数跳过失败页。 */
async function load(more=false,retry=false):Promise<void>{
  if(!currentOwner())return
  if(more&&!retry&&failed){await retryFailed();return}
  if(more&&(loading.value||offset.value>=total.value))return
  const previous=retry?failed:null,append=previous?previous.more:more,start=previous?previous.offset:append?offset.value:0
  if(!append&&!previous)submitted=query.value.trim()
  const search=previous?previous.query:submitted,selected=previous?previous.kind:kind.value,current=++version;loading.value=true;error.value=''
  try{const page=await searchContentTargets(selected,search,start);if(current===version&&currentOwner()&&selected===kind.value){results.value=appendUniqueRows(append?results.value:[],page.items);offset.value=start+page.items.length;total.value=page.total;failed=null}}
  catch(err){if(current===version&&currentOwner()){if([401,403].includes(errorStatus(err)||0)){close();return}failed={more:append,offset:start,query:search,kind:selected};error.value=errorMessage(err)}}
  finally{if(current===version)loading.value=false}
}
/** 轻点失败提示只重试原动作，新增搜索必须由搜索按钮显式提交。 */
async function retryFailed():Promise<void>{if(!loading.value)await load(failed?.more||false,true)}
/** 只接受当前公开候选的类型与ID，关闭后的旧点击不能跨弹层或账号提交。 */
function select(value:ContentTarget):void{if(!currentOwner()||!props.kinds.includes(value.kind)||value.kind!==kind.value||!results.value.some(item=>item.kind===value.kind&&item.id===value.id))return;emit('update:modelValue',{kind:value.kind,id:value.id});close()}
/** 明确清除跳转仍保留父表单图片和文字，旧弹层回调不清新草稿。 */
function clear():void{if(currentOwner()){emit('update:modelValue',null);close()}}
watch([()=>state.user?.id,()=>state.user?.role,()=>state.user?.platform_admin_role,()=>state.user?.status,sessionRevision,()=>props.kinds.join('|')],close,{flush:'sync'})
onShow(()=>{visible=true});onHide(()=>{visible=false;close()});onBeforeUnmount(()=>{disposed=true;close()})
</script>
<style scoped>
.target-trigger{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;width:100%;min-height:84.6154rpx;padding:15.3846rpx 0;background:transparent;color:var(--text);text-align:left;border-bottom:1px solid var(--line);font-size:26.9231rpx}.target-trigger-copy,.target-copy{display:flex;flex-direction:column;gap:7.6923rpx;flex:1;min-width:0}.target-id{font-size:19.2308rpx;line-height:30.7692rpx;color:var(--muted);overflow-wrap:anywhere}.target-search{display:flex;align-items:center;gap:15.3846rpx;margin:23.0769rpx 0;padding:7.6923rpx 19.2308rpx;border:1px solid var(--line);border-radius:15.3846rpx}.target-search-input{flex:1;min-width:0;font-size:25rpx}.target-results{height:36vh;min-width:0}.target-option{display:flex;align-items:center;gap:19.2308rpx;width:100%;padding:23.0769rpx 0;background:transparent;color:var(--text);text-align:left;border-bottom:1px solid var(--line-subtle)}.target-thumb{width:115.3846rpx;height:65.3846rpx;border-radius:11.5385rpx}.target-title{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;font-size:26.9231rpx;line-height:38.4615rpx}
</style>
