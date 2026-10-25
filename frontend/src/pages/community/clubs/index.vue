<!-- 文件作用：发现俱乐部、查看个人挂靠并申请创建新的电竞俱乐部。 -->
<template>
  <AppShell title="俱乐部中心" back>
    <view class="club-search"><Icon name="search" :size="18"/><input v-model="query" class="search-input" placeholder="搜索俱乐部" confirm-type="search" @confirm="load()"/></view>
    <view class="club-toolbar"><view class="club-tabs"><button :class="['club-tab',{active:!mine}]" @click="mine=false;load()">发现俱乐部</button><button :class="['club-tab',{active:mine}]" @click="mine=true;load()">我的俱乐部</button></view><view class="club-toolbar-actions"><button class="text-btn create-entry" @click="openCreate">创建</button><button v-if="managedClubs.length" class="text-btn manage-entry" @click="openManagement">管理俱乐部</button></view></view>
    <text v-if="managementError" class="hint" @click="loadManaged()">管理权限暂未同步，点击重试</text>
    <view v-if="error" class="list-error" @click="retryFailed">{{ error }} · 重试</view>
    <view v-if="loading&&!clubs.length" class="loading-line">正在读取俱乐部…</view>
    <view v-else-if="!clubs.length&&!error" class="club-empty"><text class="empty-title">{{ mine&&!state.user?'请登录后查看':mine?'暂未加入俱乐部':'暂未找到俱乐部' }}</text><text class="hint">{{ mine?'到发现页寻找合适的团队，或创建自己的俱乐部。':'试试其他名称，或成为第一位组织者。' }}</text><button v-if="mine&&!state.user" class="text-btn" @click="go('/pages/account/login?next='+encodeURIComponent('/pages/community/clubs/index?mine=1'))">登录开机电竞 <Icon name="arrow" :size="12"/></button></view>
    <ClubCard v-for="club in clubs" :key="club.id" :club="club"/><ListLoadState v-if="clubs.length" :loading="loading" :has-more="hasMore" :error="error" @retry="retryFailed"/>
    <view v-if="creating" class="modal-mask" @click="creating=false"><view class="modal" @click.stop>
      <scroll-view scroll-y class="modal-scroll"><view class="create-form form-section">
        <text class="section-title">创建俱乐部</text><text class="create-hint hint">先保存草稿，按平台要求缴纳押金后提交审核。</text>
        <view class="field"><view class="field-counter-row"><text class="field-label">俱乐部名称 · 2—30字</text><text class="hint">{{ Array.from(form.name.trim()).length }}/30</text></view><input v-model="form.name" class="input" :maxlength="-1" placeholder="2—30字的俱乐部名称"/></view>
        <view class="field"><view class="field-counter-row"><text class="field-label">团队介绍 · 10—1000字</text><text class="hint">{{ Array.from(form.description.trim()).length }}/1000</text></view><textarea v-model="form.description" class="input intro-input" :maxlength="-1" placeholder="至少10字，介绍擅长游戏、招募要求与服务风格"/></view>
        <text class="create-deposit-note">{{ creationDeposit===null?'平台押金金额将在缴纳前明确展示，保存草稿不会扣款。':creationDeposit>0?'创建需缴平台押金 ¥'+money(creationDeposit)+'，由平台独立托管。':'当前平台配置免缴押金，保存后提交审核。' }}</text><view class="field club-create-game-field"><text class="field-label">主要游戏 · 可多选</text><button class="input selector-trigger club-games-trigger" :disabled="busy" @click="clubGameSelector?.open()"><Icon name="game" :size="24"/><text class="club-games-summary">{{ clubGameSelector?.summary || '请选择游戏' }}</text><Icon name="arrow" :size="14"/></button></view>
      </view></scroll-view>
      <view class="action-footer modal-actions"><button class="text-btn" @click="creating=false">取消</button><button class="btn" :disabled="busy" @click="create">{{ busy?'正在保存…':'保存草稿并继续' }}</button></view>
    </view></view>
    <UniPopup ref="managementPopup" type="bottom" background-color="transparent" :safe-area="false"><view class="management-sheet"><view class="management-heading"><text class="section-title">选择管理的俱乐部</text><button class="text-btn" aria-label="关闭俱乐部选择" @click="closeManagement"><Icon name="close" :size="22"/></button></view><scroll-view scroll-y class="management-scroll" @scrolltolower="loadManaged(true)"><button v-for="item in managedClubs" :key="item.id" class="management-club" @click="goManagement(item)"><image v-if="item.avatar_url" class="management-avatar" :src="assetUrl(item.avatar_url)" mode="aspectFill"/><view class="management-copy"><text class="management-name">{{item.name}}</text><text class="hint">{{item.management_role==='owner'?'负责人':'副管理员'}} · {{item.member_count}}名成员{{item.status==='pending_deposit'?' · 待缴押金':''}}</text></view><Icon name="arrow" :size="14"/></button><ListLoadState :loading="managementLoading" :has-more="managementMore" :error="managementError" @retry="loadManaged(managementFailedAppend)"/></scroll-view></view></UniPopup>
    <GameSelector ref="clubGameSelector" v-model:values="form.game_ids" multiple title="主要游戏" :field="false" :active="creating" :disabled="busy"/>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：加载公开与个人俱乐部，验证创建资料后提交审批。 */
import { ref,reactive,watch } from 'vue'
import { onLoad,onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import GameSelector from '../../../components/GameSelector.vue'
import Icon from '../../../components/Icon.vue'
import ClubCard from '../../../components/ClubCard.vue'
import ListLoadState from '../../../components/ListLoadState.vue'
import {useNativePageRefresh} from '../../../lib/native-page-refresh'
import {appendUniqueRows} from '../../../lib/list-page'
import { state,init,sessionRevision } from '../../../lib/store'
import { listClubs, createClub } from '../../../api/clubs.api'
import { getFinancialRules } from '../../../api/finance.api'
import { newWalletRequestID } from '../../../lib/wallet'
import { requirePhone } from '../../../lib/phone'
import type { Club } from '../../../types/models'
import { token,errorMessage } from '../../../api/client'
import { go,notify,money,assetUrl } from '../../../lib/format'
const clubs=ref<Club[]>([]),query=ref(''),mine=ref(false),loading=ref(false),error=ref(''),creating=ref(false),busy=ref(false),form=reactive({name:'',description:'',game_ids:[] as string[]})
const managedClubs=ref<Club[]>([]),managementLoading=ref(false),managementMore=ref(false),managementError=ref('')
const managementPopup=ref<{open:(type?:string)=>void;close:()=>void}|null>(null)
let managementVersion=0,managementOffset=0,managementFailedAppend=false
const hasMore=ref(false)
let nextOffset=0,appliedQuery=''
/** 失败查询独立于输入草稿保存，重试沿用原方向、关键词与偏移。 */
let failedRead:{append:boolean;onlyMine:boolean;search:string;key:string;offset:number}|undefined
const creationDeposit=ref<number|null>(null)
const clubGameSelector=ref<InstanceType<typeof GameSelector>|null>(null)
let listVersion=0,pageVersion=0,createVersion=0,visible=true,disposed=false
/** 捕获当前身份和会话，防止旧身份的列表或创建结果回填。 */
function currentSession(){return{userId:state.user?.id,token:token(),revision:sessionRevision()}}
/** 校验请求期间的账号、令牌与会话版本均未改变。 */
function sameSession(session: ReturnType<typeof currentSession>){return session.userId===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 仅接收当前可见页面发出的最新列表请求。 */
function currentList(current: number,session: ReturnType<typeof currentSession>){return visible&&!disposed&&current===listVersion&&sameSession(session)}
/** 账号变化立即清除私有列表和创建草稿，再按新身份刷新。 */
function resetIdentity(){clearManagement();listVersion++;pageVersion++;createVersion++;clubs.value=[];hasMore.value=false;nextOffset=0;appliedQuery='';failedRead=undefined;error.value='';loading.value=false;creating.value=false;busy.value=false;Object.assign(form,{name:'',description:'',game_ids:[]});if(visible&&!disposed){load();loadManaged()}}
watch(()=>state.user?.id,resetIdentity)
/** 根据入口参数选择个人俱乐部列表，首次加载即可读取本人挂靠。 */
function applyRoute(options: Record<string,string | undefined> = {}){mine.value=options?.mine==='1'}
/** 刷新俱乐部列表，仅登录后才能读取个人挂靠。 */
async function load(append=false,retrying=false){
  const adding=append===true
  if(!visible||disposed||adding&&(loading.value||!hasMore.value))return
  const draftKey=JSON.stringify([mine.value,query.value.trim()]);if(adding&&!retrying&&draftKey!==appliedQuery)return
  const read=retrying&&failedRead?failedRead:{append:adding,onlyMine:mine.value,search:query.value.trim(),key:draftKey,offset:adding?nextOffset:0}
  const current=++listVersion,{onlyMine,search,key,offset}=read,extending=read.append
  let session:ReturnType<typeof currentSession>|undefined
  loading.value=true;error.value=''
  try{
    await init();if(!visible||disposed||current!==listVersion)return
    session=currentSession();if(onlyMine&&!state.user){clubs.value=[];hasMore.value=false;return}
    const result=await listClubs({...onlyMine?{mine:1}:{},q:search,limit:20,offset})
    if(!currentList(current,session)||!retrying&&(onlyMine!==mine.value||search!==query.value.trim()))return
    clubs.value=appendUniqueRows(extending?clubs.value:[],result)
    failedRead=undefined;nextOffset=offset+result.length;appliedQuery=key;hasMore.value=result.length===20
  }catch(e){if(visible&&!disposed&&current===listVersion&&(!session||sameSession(session))){failedRead=read;error.value=errorMessage(e)}}finally{if(current===listVersion)loading.value=false}
}
/** 目录的两处错误入口重试原失败查询，不提交未确认的搜索文字。 */
async function retryFailed(){if(loading.value||busy.value)return;await load(failedRead?.append||false,!!failedRead)}
/** 打开创建表单，默认选择当前游戏。 */
function openCreate(){if(!state.user)return go('/pages/account/login');form.game_ids=[state.gameId];creating.value=true;void loadCreationRule()}
/** 只读当前创建门槛；最终支付前以草稿快照和资金页金额为准。 */
async function loadCreationRule(){const session=currentSession();try{const rules=await getFinancialRules();if(!disposed&&sameSession(session))creationDeposit.value=rules.club_creation_deposit_cents}catch{/* 读取失败不伪造价格，私有草稿保存不扣款。 */}}
/** 校验公开资料，以同一请求号保存草稿，缴纳成功后才提交审核。 */
async function create(){
  if(busy.value||!state.user||!visible||disposed)return
  if(Array.from(form.name.trim()).length<2||Array.from(form.name.trim()).length>30)return notify('俱乐部名称需要2—30字')
  if(Array.from(form.description.trim()).length<10||Array.from(form.description.trim()).length>1000)return notify('团队介绍需要10—1000字')
  if(!form.game_ids.length||form.game_ids.length>32)return notify('请选择1—32个游戏')
  const current=pageVersion,operation=++createVersion,session=currentSession()
  const body={name:form.name.trim(),description:form.description.trim(),game_ids:[...form.game_ids]}
  const fingerprint=JSON.stringify(body),key='jijie_club_creation:'+session.userId
  const stored=uni.getStorageSync(key) as {fingerprint?:string;request_id?:string}|undefined
  const requestID=stored?.fingerprint===fingerprint&&stored.request_id?stored.request_id:newWalletRequestID()
  uni.setStorageSync(key,{fingerprint,request_id:requestID});busy.value=true
  try{
    if(!(await requirePhone())||!visible||disposed||current!==pageVersion||!sameSession(session))return
    const club=await createClub({...body,request_id:requestID})
    if(!visible||disposed||current!==pageVersion||!sameSession(session))return
    const cached=uni.getStorageSync(key) as {request_id?:string}|undefined
    if(cached?.request_id===requestID)uni.removeStorageSync(key)
    creating.value=false
    notify(club.status==='pending_deposit'?'草稿已保存，请缴纳平台押金':'俱乐部申请已提交')
    go('/pages/community/clubs/'+(club.status==='pending_deposit'?'funds':'detail')+'?id='+club.id)
  }catch(e){if(visible&&!disposed&&current===pageVersion&&sameSession(session))notify(e)}finally{if(operation===createVersion)busy.value=false}
}
/** 回到列表时刷新数据，同一身份的创建草稿仍保留。 */
function showPage(){visible=true;load();loadManaged()}
/** 离开页面后使在途列表和创建结果失效。 */
function hidePage(){closeManagement();managementVersion++;managementLoading.value=false;visible=false;listVersion++;pageVersion++;loading.value=false}
useNativePageRefresh({refresh:async()=>{await Promise.all([load(),loadManaged()])},more:()=>error.value?retryFailed():load(true),context:()=>[state.user?.id,state.user?.role,sessionRevision(),mine.value,query.value],blocked:()=>busy.value||creating.value})

/** 清除旧账号组织权限与选择浮层，公开列表筛选不影响管理范围。 */
function clearManagement():void{managementVersion++;managementOffset=0;managedClubs.value=[];managementMore.value=false;managementError.value='';managementLoading.value=false;closeManagement()}
/** 独立读取本人有管理权的组织，使用服务端筛选后分页而非过滤公开列表。 */
async function loadManaged(append=false):Promise<void>{if(!visible||disposed||append&&(managementLoading.value||!managementMore.value))return;const version=++managementVersion;managementLoading.value=true;const offset=append?managementOffset:0;let session:ReturnType<typeof currentSession>|undefined;try{await init();if(!visible||disposed||version!==managementVersion)return;if(!state.user){clearManagement();return}session=currentSession();const rows=await listClubs({managed:1,limit:20,offset});if(!visible||version!==managementVersion||!sameSession(session))return;managedClubs.value=appendUniqueRows(append?managedClubs.value:[],rows);managementOffset=offset+rows.length;managementMore.value=rows.length===20;managementError.value='';managementFailedAppend=false}catch(err){if(visible&&version===managementVersion&&(!session||sameSession(session))){managementError.value=errorMessage(err);managementFailedAppend=append}}finally{if(version===managementVersion)managementLoading.value=false}}
/** 单组织直达控制台，多组织使用官方真实浮层。 */
function openManagement():void{if(!state.user||!managedClubs.value.length)return;if(managedClubs.value.length===1&&!managementMore.value)return goManagement(managedClubs.value[0]);managementPopup.value?.open('bottom')}
/** 仅从服务端本人管理列表打开已有控制台。 */
function goManagement(club:Club):void{if(!state.user||!managedClubs.value.some(item=>item.id===club.id))return;closeManagement();go('/pages/community/clubs/console?id='+encodeURIComponent(club.id))}
/** 关闭选择器，不改变当前公开搜索与滚动状态。 */
function closeManagement():void{managementPopup.value?.close()}

onLoad(applyRoute)
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.field-counter-row{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.club-search{display:flex;align-items:center;gap:19.2308rpx;min-width:0;border-bottom:1px solid var(--line);color:var(--muted)}.search-input{flex:1;min-width:0;height:84.6154rpx;padding:0;background:transparent;font-size:26.9231rpx}.club-toolbar{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;margin:15.3846rpx 0 30.7692rpx}.club-tabs{display:flex;gap:38.4615rpx;min-width:0;max-width:calc(100% - 84.6154rpx)}.club-tab{display:flex;align-items:center;justify-content:center;min-width:0;height:84.6154rpx;padding:0;border-radius:0;border-bottom:3.8462rpx solid transparent;background:transparent;font-size:26.9231rpx;color:var(--muted);white-space:nowrap}.club-tab.active{border-bottom-color:var(--accent);color:var(--text);font-weight:650}.create-deposit-note{display:block;font-size:26.9231rpx;line-height:42.3077rpx;color:var(--accent);margin:0 0 38.4615rpx;overflow-wrap:anywhere}.create-entry{padding:0;min-width:61.5385rpx;font-size:26.9231rpx}.list-error{padding:23.0769rpx 0;color:#efb299;font-size:23.0769rpx;overflow-wrap:anywhere}.club-empty{display:flex;flex-direction:column;align-items:center;gap:23.0769rpx;padding:92.3077rpx 0;text-align:center;min-width:0}.empty-title{font-size:28.8462rpx;color:var(--text)}
.modal-mask{position:fixed;inset:0;z-index:110;display:flex;align-items:center;justify-content:center;padding:38.4615rpx;background:#000b}.modal{width:100%;max-width:460px;overflow:hidden;background:var(--surface);border-radius:23.0769rpx;min-width:0}.modal-scroll{height:60vh;max-height:1076.9231rpx;padding:38.4615rpx 38.4615rpx 0;box-sizing:border-box}.create-form{padding:0;border:0}.create-hint{display:block;margin:-15.3846rpx 0 38.4615rpx}.intro-input{height:215.3846rpx}.club-games-trigger{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:44px;margin:0;text-align:left}.club-games-summary{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:26.9231rpx;color:var(--text)}.modal-actions{padding:38.4615rpx;gap:23.0769rpx}
@media(max-width:350px){.club-tabs{gap:30.7692rpx}.club-tab{font-size:25rpx}.modal-mask{padding:30.7692rpx}.modal-scroll{padding:30.7692rpx 30.7692rpx 0}.modal-actions{padding:30.7692rpx}}
.club-toolbar{flex-wrap:wrap}.club-tabs{max-width:100%;flex:1 1 auto}.club-toolbar-actions{display:flex;align-items:center;gap:23.0769rpx;flex-shrink:0;margin-left:auto}.manage-entry{font-size:25rpx;min-height:44px;white-space:nowrap}.management-sheet{width:100vw;max-width:600px;box-sizing:border-box;padding:30.7692rpx 30.7692rpx calc(30.7692rpx + env(safe-area-inset-bottom));border-radius:38.4615rpx 38.4615rpx 0 0;background:var(--bg);color:var(--text)}.management-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;margin-bottom:15.3846rpx}.management-scroll{height:55vh;min-width:0}.management-club{display:flex;align-items:center;gap:23.0769rpx;width:100%;padding:23.0769rpx 0;border-bottom:1px solid var(--line-subtle);border-radius:0;background:transparent;color:var(--text);text-align:left}.management-avatar{width:92.3077rpx;height:92.3077rpx;border-radius:23.0769rpx;flex-shrink:0}.management-copy{flex:1;min-width:0}.management-name{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:30.7692rpx}
</style>
