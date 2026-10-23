<!-- 文件作用：发现俱乐部、查看个人挂靠并申请创建新的电竞俱乐部。 -->
<template>
  <AppShell title="俱乐部" back>
    <view class="club-search"><Icon name="search" :size="18"/><input v-model="query" class="search-input" placeholder="搜索俱乐部" confirm-type="search" @confirm="load"/></view>
    <view class="club-toolbar"><view class="club-tabs"><button :class="['club-tab',{active:!mine}]" @click="mine=false;load()">发现俱乐部</button><button :class="['club-tab',{active:mine}]" @click="mine=true;load()">我的俱乐部</button></view><button class="text-btn create-entry" @click="openCreate">创建</button></view>
    <view v-if="error" class="list-error" @click="load">{{ error }} · 重试</view>
    <view v-if="loading&&!clubs.length" class="loading-line">正在读取俱乐部…</view>
    <view v-else-if="!clubs.length&&!error" class="club-empty"><text class="empty-title">{{ mine&&!state.user?'请登录后查看':mine?'暂未加入俱乐部':'暂未找到俱乐部' }}</text><text class="hint">{{ mine?'到发现页寻找合适的团队，或创建自己的俱乐部。':'试试其他名称，或成为第一位组织者。' }}</text><button v-if="mine&&!state.user" class="text-btn" @click="go('/pages/account/login?next='+encodeURIComponent('/pages/community/clubs/index?mine=1'))">登录开机电竞 <Icon name="arrow" :size="12"/></button></view>
    <ClubCard v-for="club in clubs" :key="club.id" :club="club"/>
    <view v-if="creating" class="modal-mask" @click="creating=false"><view class="modal" @click.stop>
      <scroll-view scroll-y class="modal-scroll"><view class="create-form form-section">
        <text class="section-title">创建俱乐部</text><text class="create-hint hint">先保存草稿，按平台要求缴纳押金后提交审核。</text>
        <view class="field"><text class="field-label">俱乐部名称</text><input v-model="form.name" class="input" maxlength="30" placeholder="俱乐部名称"/></view>
        <view class="field"><text class="field-label">团队介绍</text><textarea v-model="form.description" class="input intro-input" maxlength="500" placeholder="介绍团队擅长的游戏、招募要求与服务风格"/></view>
        <text class="create-deposit-note">{{ creationDeposit===null?'平台押金金额将在缴纳前明确展示，保存草稿不会扣款。':creationDeposit>0?'创建需缴平台押金 ¥'+money(creationDeposit)+'，由平台独立托管。':'当前平台配置免缴押金，保存后提交审核。' }}</text><view class="field club-create-game-field"><text class="field-label">主要游戏 · 可多选</text><button class="input selector-trigger club-games-trigger" :disabled="busy" @click="clubGameSelector?.open()"><Icon name="game" :size="24"/><text class="club-games-summary">{{ clubGameSelector?.summary || '请选择游戏' }}</text><Icon name="arrow" :size="14"/></button></view>
      </view></scroll-view>
      <view class="action-footer modal-actions"><button class="text-btn" @click="creating=false">取消</button><button class="btn" :disabled="busy" @click="create">{{ busy?'正在保存…':'保存草稿并继续' }}</button></view>
    </view></view>
    <GameSelector ref="clubGameSelector" v-model:values="form.game_ids" multiple title="主要游戏" :field="false" :active="creating" :disabled="busy"/>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：加载公开与个人俱乐部，验证创建资料后提交审批。 */
import { ref,reactive,watch } from 'vue'
import { onLoad,onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import GameSelector from '../../../components/GameSelector.vue'
import Icon from '../../../components/Icon.vue'
import ClubCard from '../../../components/ClubCard.vue'
import { state,init,sessionRevision } from '../../../lib/store'
import { listClubs, createClub } from '../../../api/clubs.api'
import { getFinancialRules } from '../../../api/finance.api'
import { newWalletRequestID } from '../../../lib/wallet'
import { requirePhone } from '../../../lib/phone'
import type { Club } from '../../../types/models'
import { token,errorMessage } from '../../../api/client'
import { go,notify,money } from '../../../lib/format'
const clubs=ref<Club[]>([]),query=ref(''),mine=ref(false),loading=ref(false),error=ref(''),creating=ref(false),busy=ref(false),form=reactive({name:'',description:'',game_ids:[] as string[]})
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
function resetIdentity(){listVersion++;pageVersion++;createVersion++;clubs.value=[];error.value='';loading.value=false;creating.value=false;busy.value=false;Object.assign(form,{name:'',description:'',game_ids:[]});if(visible&&!disposed)load()}
watch(()=>state.user?.id,resetIdentity)
/** 根据入口参数选择个人俱乐部列表，首次加载即可读取本人挂靠。 */
function applyRoute(options: Record<string,string | undefined> = {}){mine.value=options?.mine==='1'}
/** 刷新俱乐部列表，仅登录后才能读取个人挂靠。 */
async function load(){if(!visible||disposed)return;const current=++listVersion,onlyMine=mine.value,search=query.value.trim();let session: ReturnType<typeof currentSession> | undefined;loading.value=true;error.value='';try{await init();if(!visible||disposed||current!==listVersion)return;session=currentSession();if(onlyMine&&!state.user){clubs.value=[];return}const result=await listClubs({...onlyMine?{mine:1}:{},q:search});if(currentList(current,session))clubs.value=result}catch(e){if(visible&&!disposed&&current===listVersion&&(!session||sameSession(session)))error.value=errorMessage(e)}finally{if(current===listVersion)loading.value=false}}
/** 打开创建表单，默认选择当前游戏。 */
function openCreate(){if(!state.user)return go('/pages/account/login');form.game_ids=[state.gameId];creating.value=true;void loadCreationRule()}
/** 只读当前创建门槛；最终支付前以草稿快照和资金页金额为准。 */
async function loadCreationRule(){const session=currentSession();try{const rules=await getFinancialRules();if(!disposed&&sameSession(session))creationDeposit.value=rules.club_creation_deposit_cents}catch{/* 读取失败不伪造价格，私有草稿保存不扣款。 */}}
/** 校验公开资料，以同一请求号保存草稿，缴纳成功后才提交审核。 */
async function create(){
  if(busy.value||!state.user||!visible||disposed)return
  if(!form.name.trim()||!form.description.trim()||!form.game_ids.length)return notify('请填写名称、介绍并选择游戏')
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
function showPage(){visible=true;load()}
/** 离开页面后使在途列表和创建结果失效。 */
function hidePage(){visible=false;listVersion++;pageVersion++;loading.value=false}
onLoad(applyRoute)
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.club-search{display:flex;align-items:center;gap:19.2308rpx;min-width:0;border-bottom:1px solid var(--line);color:var(--muted)}.search-input{flex:1;min-width:0;height:84.6154rpx;padding:0;background:transparent;font-size:26.9231rpx}.club-toolbar{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;margin:15.3846rpx 0 30.7692rpx}.club-tabs{display:flex;gap:38.4615rpx;min-width:0;max-width:calc(100% - 84.6154rpx)}.club-tab{display:flex;align-items:center;justify-content:center;min-width:0;height:84.6154rpx;padding:0;border-radius:0;border-bottom:3.8462rpx solid transparent;background:transparent;font-size:26.9231rpx;color:var(--muted);white-space:nowrap}.club-tab.active{border-bottom-color:var(--accent);color:var(--text);font-weight:650}.create-deposit-note{display:block;font-size:26.9231rpx;line-height:42.3077rpx;color:var(--accent);margin:0 0 38.4615rpx;overflow-wrap:anywhere}.create-entry{padding:0;min-width:61.5385rpx;font-size:26.9231rpx}.list-error{padding:23.0769rpx 0;color:#efb299;font-size:23.0769rpx;overflow-wrap:anywhere}.club-empty{display:flex;flex-direction:column;align-items:center;gap:23.0769rpx;padding:92.3077rpx 0;text-align:center;min-width:0}.empty-title{font-size:28.8462rpx;color:var(--text)}
.modal-mask{position:fixed;inset:0;z-index:110;display:flex;align-items:center;justify-content:center;padding:38.4615rpx;background:#000b}.modal{width:100%;max-width:460px;overflow:hidden;background:var(--surface);border-radius:23.0769rpx;min-width:0}.modal-scroll{height:60vh;max-height:1076.9231rpx;padding:38.4615rpx 38.4615rpx 0;box-sizing:border-box}.create-form{padding:0;border:0}.create-hint{display:block;margin:-15.3846rpx 0 38.4615rpx}.intro-input{height:215.3846rpx}.club-games-trigger{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-height:44px;margin:0;text-align:left}.club-games-summary{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:26.9231rpx;color:var(--text)}.modal-actions{padding:38.4615rpx;gap:23.0769rpx}
@media(max-width:350px){.club-tabs{gap:30.7692rpx}.club-tab{font-size:25rpx}.modal-mask{padding:30.7692rpx}.modal-scroll{padding:30.7692rpx 30.7692rpx 0}.modal-actions{padding:30.7692rpx}}
</style>
