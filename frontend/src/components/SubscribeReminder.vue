<!-- 文件作用：复用微信提醒紧凑入口与发布成功真实弹层，只在明确点击时申请本次官方订阅。 -->
<template>
  <view v-if="state.user" class="subscribe-reminder"><button v-if="entry" class="subscribe-entry text-btn" :disabled="busy" @click="open"><Icon name="bell" :size="16"/><text>{{ label }}</text><Icon name="arrow" :size="11"/></button><FinancePopup :opened="opened" :title="title" :busy="busy" @close="close"><view class="subscribe-popup-intro"><view class="subscribe-bell"><Icon name="bell" :size="25"/></view><text class="subscribe-description">{{ description }}</text></view><text v-if="error" class="subscribe-error">{{ error }}</text><text v-else-if="!available" class="subscribe-note">{{ loading?'正在读取可订阅消息…':overview?.enabled===false?'平台暂未启用微信提醒，站内消息仍会正常保留。':'当前暂无可订阅模板，站内消息仍会正常保留。' }}</text><button class="btn subscribe-confirm" :disabled="busy||loading||!available" @click="subscribe">{{ busy?'正在处理…':buttonLabel }}</button><button v-if="!loading&&!available" class="text-btn subscribe-reload" @click="load">重新读取</button><text class="subscribe-note">一次性订阅不等于永久提醒；是否送达以微信授权和发送结果为准。拒绝不影响使用。</text><button class="text-btn subscribe-later" :disabled="busy" @click="close">稍后再说</button></FinancePopup></view>
</template>
<script setup lang="ts">
import {computed,ref,watch,onMounted} from 'vue'
import {onShow,onHide,onUnload} from '@dcloudio/uni-app'
import FinancePopup from './FinancePopup.vue'
import Icon from './Icon.vue'
import {state,sessionRevision} from '../lib/store'
import {token,errorMessage} from '../api/client'
import {getSubscriptions,recordSubscriptionAuthorization} from '../api/subscriptions.api'
import {subscriptionCandidates,requestSubscriptionFromGesture,wechatSubscribeAPI} from '../lib/subscriptions'
import {notify} from '../lib/format'
import type {SubscriptionOverview,SubscriptionScene} from '../types/subscriptions'
const props=withDefaults(defineProps<{scenes:SubscriptionScene[];entry?:boolean;label?:string;title?:string;description?:string;buttonLabel?:string;promptKey?:number}>(),{entry:true,label:'接收微信提醒',title:'重要进展，微信及时提醒',description:'接收订单与申请处理结果，重要事项及时查看。',buttonLabel:'接收结果提醒',promptKey:0})
const overview=ref<SubscriptionOverview|null>(null),loading=ref(false),opened=ref(false),busy=ref(false),error=ref('')
const available=computed(()=>subscriptionCandidates(overview.value,props.scenes).length>0)
let visible=true,disposed=false,version=0,operation=0,done:{next:()=>void;owner:ReturnType<typeof session>}|undefined
/** 捕获登录身份，授权结果仅可记到原账号。 */
function session(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 原生授权暂时隐藏页面仍可上报原账号结果，换号后禁止旧回包写入。 */
function same(owner:ReturnType<typeof session>){return !disposed&&!!owner.id&&owner.id===state.user?.id&&owner.token===token()&&owner.revision===sessionRevision()}
/** 预先读取模板，点击授权时不能等待网络再调用原生接口。 */
async function load(){if(!state.user||!visible||disposed)return;const owner=session(),current=++version;loading.value=true;error.value='';try{const result=await getSubscriptions();if(visible&&same(owner)&&current===version)overview.value=result}catch(err){if(visible&&same(owner)&&current===version)error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 显式打开订阅说明，读取失败可重试，不影响业务操作。 */
function open(){if(!state.user||busy.value)return;opened.value=true;if(!overview.value&&!loading.value)void load()}
/** 成功业务的导航仅归原账号，先取走回调再执行保证最多一次；后台切回后安全继续。 */
function continueDone(){if(!done)return;if(!same(done.owner)){done=undefined;return}if(!visible)return;const next=done.next;done=undefined;next()}
/** 发布成功后只展示轻提示；缺配置或页面已隐藏也不会丢失已成功的导航意图。 */
function offer(next?:()=>void){if(!state.user)return;if(next)done={next,owner:session()};if(!visible||!available.value){continueDone();return}opened.value=true}
/** 关闭并继续已成功的业务导航，拒绝与稍后再说完全等价。 */
function close(){if(busy.value)return;opened.value=false;error.value='';continueDone()}
/** 在按钮点击调用栈同步唤起原生授权，授权记录网络状态绝不阻断已成功业务导航。 */
async function subscribe(){if(busy.value||loading.value||!available.value||!visible)return;const owner=session(),current=++operation;busy.value=true;error.value='';try{const pending=requestSubscriptionFromGesture(overview.value,props.scenes,wechatSubscribeAPI());const result=await pending;if(current!==operation||!same(owner))return;const recording=recordSubscriptionAuthorization(result);if(done){busy.value=false;opened.value=false;continueDone()}await recording;if(current!==operation||!same(owner))return;if(visible)notify(result.results.some(item=>item.status==='accept')?'本次订阅选择已记录':'未订阅成功，不影响正常使用');busy.value=false;close()}catch(err){if(current===operation&&same(owner)){if(visible)error.value=errorMessage(err);busy.value=false;if(done){opened.value=false;continueDone()}}}finally{if(current===operation){busy.value=false;if(done){opened.value=false;continueDone()}}}}
/** 切号立即清除个人场景和弹层，不让旧授权复活。 */
function clear(){version++;operation++;overview.value=null;loading.value=false;opened.value=false;busy.value=false;error.value='';done=undefined}
watch([()=>state.user?.id,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
watch(()=>props.promptKey,(value,previous)=>{if(value&&value!==previous)offer()})
onMounted(()=>{if(!loading.value&&!overview.value)void load()})
onShow(()=>{visible=true;continueDone();void load()})
onHide(()=>{visible=false;version++;loading.value=false;opened.value=false})
onUnload(()=>{disposed=true;clear()})
defineExpose({offer,open})
</script>
<style scoped>
.subscribe-reminder{min-width:0}.subscribe-entry{display:flex;align-items:center;justify-content:flex-start;gap:11.5385rpx;min-height:84.6154rpx;width:100%;min-width:0;font-size:25rpx;color:var(--muted);padding:0}.subscribe-popup-intro{display:flex;align-items:center;gap:23.0769rpx;min-width:0;margin:7.6923rpx 0 30.7692rpx}.subscribe-bell{display:flex;align-items:center;justify-content:center;width:84.6154rpx;height:84.6154rpx;border-radius:50%;flex-shrink:0;color:var(--accent);background:var(--bg)}.subscribe-description{flex:1;min-width:0;font-size:26.9231rpx;line-height:42.3077rpx;color:var(--text);overflow-wrap:anywhere}.subscribe-note,.subscribe-error{display:block;font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);margin:23.0769rpx 0;overflow-wrap:anywhere}.subscribe-error{color:#e4a18d}.subscribe-confirm{width:100%}.subscribe-later,.subscribe-reload{display:flex;align-items:center;justify-content:center;margin:0 auto;font-size:25rpx;min-height:84.6154rpx}
</style>
