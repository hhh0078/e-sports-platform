<!-- 文件作用：发布与编辑游戏服务，配置游戏封面、文字样式和俱乐部挂靠。 -->
<template>
  <AppShell :title="serviceId ? '编辑服务' : '发布服务'" :game-id="form.game_id" back>
    <view v-if="error" class="error-box mb"><text>{{ error }}</text><button v-if="!formReady" class="text-btn" @click="load">重新加载</button></view>
    <text v-if="draftIssue" class="subtle-note">{{draftIssue}}</text>
    <view v-if="!formReady&&!error" class="loading-line">正在准备发布表单…</view>
    <template v-if="formReady">

      <view class="form-section game-selector"><view class="field service-game-field"><text class="field-label">选择游戏</text><GameSelector :value="form.game_id" :disabled="draftPending" @change="changeGame"/></view></view>
      <view class="platform-deposit-note"><Icon name="wallet" :size="17"/><view class="platform-deposit-copy"><text class="platform-deposit-title">平台押金 · 自愿缴纳</text><text class="subtle-note platform-deposit-detail">缴纳后展示平台押金标识，不影响发布</text></view><button class="text-btn platform-wallet-entry" @click="go(state.user?'/pages/account/wallet':'/pages/account/login?next='+encodeURIComponent('/pages/account/wallet'))">我的钱包<Icon name="arrow" :size="11"/></button></view>
      <view class="form-section cover-section"><text class="section-title">封面底图</text>
        <view class="cover-options"><button v-for="src in covers" :key="src" :class="['cover-option',{chosen:form.cover_url===src}]" :aria-pressed="form.cover_url===src" aria-label="选择封面底图" @click="form.cover_url=src"><image class="cover-image" :src="assetUrl(src)" mode="aspectFill"/><view v-if="form.cover_url===src" class="cover-check"><Icon name="check" :size="12"/></view></button></view>
      </view>
      <view class="form-section cover-text-section">
        <view class="cover-text-heading"><view class="cover-text-title"><text class="section-title">封面文字</text><text class="label-count">{{form.labels.length}} / 6</text></view><button class="text-btn add-label" :disabled="draftPending||form.labels.length>=6" @click="addLabel"><Icon name="plus" :size="15"/>添加文字</button></view>
        <text v-if="!form.labels.length" class="subtle-note">可添加最多6条文字，也可保留纯图片封面。</text>
        <CoverLabelEditor v-for="(label,index) in form.labels" :key="label.id??index" :label="label" :index="index" :disabled="draftPending" :accent="selectedGame?.primary_color" :slider-block-size="sliderBlockSize" @remove="form.labels.splice(index,1)"/>
      </view>
      <view class="form-section form-panel"><text class="section-title">服务信息</text><view class="field"><text class="field-label">服务标题</text><input v-model="form.title" class="input" maxlength="40" placeholder="例如：三角洲双人护航 · 耐心沟通"/></view><view class="field"><text class="field-label">服务类型</text><picker :range="selectedGame?.categories || []" :value="categoryIndex" @change="form.category=selectedGame.categories[Number($event.detail.value)]"><view class="input picker-value">{{ form.category || '请选择' }}<Icon name="arrow" :size="14"/></view></picker></view><view class="price-grid"><view class="field"><text class="field-label">价格（元）</text><input v-model="form.price" type="digit" class="input" placeholder="例如 29"/></view><view class="field"><text class="field-label">计价单位</text><picker :range="units" :value="units.indexOf(form.unit)" @change="form.unit=units[Number($event.detail.value)]"><view class="input">每 {{ form.unit }}</view></picker></view></view><view class="field"><text class="field-label">服务说明与交付标准</text><textarea v-model="form.description" class="input description" maxlength="1000" placeholder="明确服务包含什么、预约方式、服务时长、验收标准以及不包含的内容。请勿提供外挂或索取账号密码。"/></view><view class="field"><text class="field-label">服务标签（逗号分隔，最多4个）</text><input v-model="form.tagsText" class="input" maxlength="60" placeholder="耐心沟通,稳定在线,新手友好"/></view><view class="field"><text class="field-label">俱乐部挂靠（可选）</text><picker :range="clubOptions" range-key="name" :value="clubIndex" @change="form.club_id=clubOptions[Number($event.detail.value)].id"><view class="input picker-value">{{ clubOptions[clubIndex]?.name || '自由打手 · 无需挂靠' }}<Icon name="arrow" :size="14"/></view></picker><text class="hint">仅能选择已通过挂靠的俱乐部。认证由平台审核，不影响自由接单。</text></view></view>
      <view class="action-footer editor-actions"><button class="btn next-preview" :disabled="draftPending" @click="nextPreview">下一步预览<Icon name="arrow" :size="14"/></button></view>
    </template>
    <PublishDraftDialog v-for="context in draftDialogs" :key="context" :opened="true" :context="context" @restore="draft.restore" @discard="draft.discard" @dismiss="draft.dismiss"/>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：维护可保存的封面配置，检查金额与服务内容后发布并进入后置图文审核。 */
import { ref, reactive, computed, watch,nextTick } from 'vue'
import { onLoad, onShow, onHide, onUnload, onResize } from '@dcloudio/uni-app'
import CoverLabelEditor from '../../components/CoverLabelEditor.vue'
import AppShell from '../../components/AppShell.vue'
import GameSelector from '../../components/GameSelector.vue'
import Icon from '../../components/Icon.vue'
import PublishDraftDialog from '../../components/PublishDraftDialog.vue'
import {createPublishDraft,publishDraftOwner,serviceDraftFields} from '../../lib/publish-draft'
import { state, init, sessionRevision } from '../../lib/store'
import { getService } from '../../api/services.api'
import { listClubs } from '../../api/clubs.api'
import {getWallet} from '../../api/wallet.api'
import {getClubDeposits} from '../../api/finance.api'
import type { Club, CoverLabel, Service } from '../../types/models'
import { token, errorMessage } from '../../api/client'
import { go, notify, assetUrl } from '../../lib/format'
import { newLabel, gameCovers, randomGameCover, normalizeLabelText } from '../../lib/cover'
import { designPixels } from '../../lib/design-units'
import {setServicePreview,serviceFormIssue} from '../../lib/service-preview'
/** 原生slider旋钮接口使用实际像素，并限制在官方允许的12至28像素范围。 */
const sliderBlockSize=ref(16)
/** 按窗口宽度同步旋钮，避免表单缩放后仍保留固定像素。 */
function refreshSliderBlockSize():void{try{const width=uni.getWindowInfo().windowWidth;if(Number.isFinite(width)&&width>0)sliderBlockSize.value=Math.max(12,Math.min(28,Math.round(designPixels(16,width))))}catch{/* 原生窗口暂不可读时保留可用旋钮尺寸。 */}}
onResize(refreshSliderBlockSize)
/** 发布预览的两类押金只来自本账号服务端记录，不属于可保存的草稿字段。 */
const serverProviderDeposit=ref(0),serverProviderClubDeposit=ref(0)
const serviceId = ref(''), clubs = ref<Club[]>([]), error = ref(''), formReady=ref(false)
const form = reactive({ game_id:'delta', title:'', description:'', category:'', price:'', unit:'局', cover_url:'', labels:[] as CoverLabel[], tagsText:'', club_id:'' })
const draft=createPublishDraft({kind:'service',owner:()=>publishDraftOwner(state.user?.id),object:()=>serviceId.value,read:()=>form,parse:serviceDraftFields,apply:fields=>{Object.assign(form,fields);if(form.club_id&&!clubOptions.value.some(club=>club.id===form.club_id))form.club_id=''}})
const draftPending=draft.pending,draftIssue=draft.issue
// 微信单个组件上的key不会重建实例；真实keyed列表只在存在非零恢复上下文时创建弹层。
const draftDialogs=computed(()=>draftPending.value?[draft.contextID()]:[])
const selectedGame = computed(() => state.games.find(item => item.id===form.game_id) || state.games[0])
const covers = computed(() => gameCovers(selectedGame.value))
const categoryIndex = computed(() => Math.max(0,(selectedGame.value?.categories || []).indexOf(form.category)))
const clubOptions = computed(() => [{id:'',name:'自由打手 · 无需挂靠'},...clubs.value.filter(club=>club.status==='active'&&['owner','active'].includes(club.membership_status||'')&&(club.game_ids||[]).includes(form.game_id))])
const clubIndex = computed(() => Math.max(0,clubOptions.value.findIndex(item=>item.id===form.club_id)))
/** 使用正式服务卡片展示草稿，确保文字和价格区与列表共用同一布局。 */
const previewService = computed<Service>(() => ({ provider_deposit_cents:serverProviderDeposit.value,provider_club_deposit_cents:serverProviderClubDeposit.value, id:serviceId.value, game_id:form.game_id, provider_id:state.user?.id||'', provider_name:state.user?.nickname||'', provider_avatar:state.user?.avatar||'', club_id:form.club_id, club_name:clubs.value.find(club=>club.id===form.club_id)?.name||'', club_verified:clubs.value.some(club=>club.id===form.club_id&&club.verified), title:form.title||'你的服务标题', description:form.description, category:form.category||'服务类型', price_cents:Number.isFinite(Number(form.price))?Math.max(0,Math.round(Number(form.price)*100)):0, unit:form.unit, cover_url:form.cover_url, labels:form.labels, tags:form.tagsText.split(/[,，]/).map(item=>item.trim()).filter(Boolean).slice(0,4), status:'draft', rating:0, completed_count:0, created_at:'' }))
const units = ['局','小时','次','任务']
let disposed=false,visible=true,loadVersion=0,loading=false,sessionLoadQueued=false,platformDepositVersion=0,clubDepositVersion=0,loadedSession: ReturnType<typeof currentSession> | null=null
/** 捕获服务发布所属账号，防止手机号授权后切换会话继续写入。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 仅接收本页存活期间、同一用户令牌和会话版本的结果。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean {return !disposed&&session.id===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 只允许当前可见页面、最新加载与原会话回填服务和私人挂靠。 */
function currentLoad(version: number,session: ReturnType<typeof currentSession>){return visible&&version===loadVersion&&sameSession(session)}
/** 刷新本人平台实缴押金，旧页面、旧账号和旧请求不能恢复徽标。 */
async function refreshPlatformDeposit(){const version=++platformDepositVersion,session=currentSession();if(!visible||!session.id)return;try{const wallet=await getWallet();if(!visible||version!==platformDepositVersion||!sameSession(session))return;serverProviderDeposit.value=wallet.deposit_qualified===true&&Number.isSafeInteger(wallet.deposit_cents)&&wallet.deposit_cents>0?wallet.deposit_cents:0}catch{if(visible&&version===platformDepositVersion&&sameSession(session))serverProviderDeposit.value=0}}
/** 按草稿当前挂靠读取本人实缴成员押金，不能拿俱乐部平台押金或门槛金额显示。 */
async function refreshClubDeposit(){const version=++clubDepositVersion,session=currentSession(),clubId=form.club_id;serverProviderClubDeposit.value=0;if(!visible||!session.id||!clubId)return;try{const result=await getClubDeposits(clubId);if(!visible||version!==clubDepositVersion||!sameSession(session)||form.club_id!==clubId)return;const amount=result.membership.amount_cents;serverProviderClubDeposit.value=Number.isSafeInteger(amount)&&amount>0?amount:0}catch{if(visible&&version===clubDepositVersion&&sameSession(session)&&form.club_id===clubId)serverProviderClubDeposit.value=0}}
/** 切换挂靠立即移除旧俱乐部徽标，只有当前账号新响应可回填。 */
watch(()=>form.club_id,()=>{clubDepositVersion++;serverProviderClubDeposit.value=0;if(loadedSession&&sameSession(loadedSession))void refreshClubDeposit()},{flush:'sync'})
/** 加载游戏和可挂靠俱乐部，每个异步边界都检查身份与页面版本。 */
async function load() {
  if(!visible||disposed)return
  draft.leave()
  const current=++loadVersion,entryToken=token(),revision=sessionRevision()
  let session: ReturnType<typeof currentSession> | undefined
  loading=true;formReady.value=false;loadedSession=null;error.value='';serverProviderDeposit.value=0;serverProviderClubDeposit.value=0;platformDepositVersion++;clubDepositVersion++
  try {
    await init()
    if(!visible||disposed||current!==loadVersion||entryToken!==token()||revision!==sessionRevision())return
    session=currentSession()
    if(serviceId.value){const service=await getService(serviceId.value);if(!currentLoad(current,session))return;serverProviderDeposit.value=service.provider_id===session.id?service.provider_deposit_cents||0:0;Object.assign(form,{game_id:service.game_id,title:service.title,description:service.description,category:service.category,price:(service.price_cents/100).toFixed(2),unit:service.unit,cover_url:service.cover_url,labels:(service.labels||[]).map(label=>({...label})),tagsText:(service.tags||[]).join(','),club_id:service.club_id})}
    else{Object.assign(form,{game_id:state.gameId,title:'',description:'',category:'',price:'',unit:'局',cover_url:'',labels:[],tagsText:'',club_id:''});chooseDefaults()}
    const affiliations=state.user?await listClubs({mine:1}):[]
    if(!currentLoad(current,session))return
    clubs.value=affiliations
    if(form.club_id&&!clubOptions.value.some(club=>club.id===form.club_id))form.club_id=''
    loadedSession=session
    draft.start()
    if(!draft.contextID())throw new Error('发布表单初始化失败，请重新加载')
    formReady.value=true
    void refreshPlatformDeposit();void refreshClubDeposit()
  }catch(e){if(visible&&!disposed&&current===loadVersion&&(session?sameSession(session):entryToken===token()&&revision===sessionRevision()))error.value=errorMessage(e)}
  finally{if(current===loadVersion)loading=false}
}
/** 会话轮换立即移除原身份挂靠；下次显示时按新身份读取，不改其他草稿。 */
function invalidateSession(){draft.leave();formReady.value=false;loadVersion++;platformDepositVersion++;clubDepositVersion++;loading=false;loadedSession=null;clubs.value=[];Object.assign(form,{game_id:state.gameId,title:'',description:'',category:'',price:'',unit:'局',cover_url:'',labels:[],tagsText:'',club_id:''});serverProviderDeposit.value=0;serverProviderClubDeposit.value=0}
/** 清会话会先变更修订号再移除token；合并到下一帧读取稳定身份，不能同步捕获旧token。 */
function scheduleSessionLoad():void{if(sessionLoadQueued||disposed)return;sessionLoadQueued=true;void nextTick(()=>{sessionLoadQueued=false;if(visible&&!disposed&&!loading&&(!loadedSession||!sameSession(loadedSession)))void load()})}
watch([sessionRevision,()=>state.user?.id],()=>{invalidateSession();scheduleSessionLoad()},{flush:'sync'})
/** 同一身份已完成加载时保留草稿，取消的读取则在重显后重新开始。 */
function showPage(){refreshSliderBlockSize();visible=true;if(!loading&&(!loadedSession||!sameSession(loadedSession)))load();else if(!loading){void refreshPlatformDeposit();void refreshClubDeposit()}}
/** 隐藏后使旧服务、挂靠和错误响应失效，保留已完成的同账号草稿。 */
function hidePage(){draft.flush();visible=false;loadVersion++;platformDepositVersion++;clubDepositVersion++;loading=false}
/** 为新建服务或新选游戏随机设置底图，并使用当前游戏默认分类。 */
function chooseDefaults() { form.cover_url=randomGameCover(selectedGame.value);form.category=selectedGame.value?.categories?.[0] || '' }
/** 切换游戏，避免带入不适用的分类或封面。 */
function changeGame(id:string) { const selected=state.games.find(item=>item.id===id&&item.enabled!==false);if(!selected||selected.id===form.game_id)return;form.game_id=selected.id;chooseDefaults();if(!clubOptions.value.some(club=>club.id===form.club_id))form.club_id='' }
/** 加入一条默认左下角文案。 */
function addLabel() { if(form.labels.length<6) form.labels.push(newLabel(selectedGame.value?.primary_color)) }
/** 校验并补存完整表单后打开独立预览，返回保持本页实例与原生滚动位置。 */
function nextPreview():void {
  if(!formReady.value||draftPending.value)return
  form.labels.forEach(label=>{label.text=normalizeLabelText(label.text)})
  const issue=serviceFormIssue(form)
  if(issue)return notify(issue)
  const submitted=draft.submitting(),session=currentSession()
  if(!submitted)return notify('草稿尚未准备好，请稍后重试')
  setServicePreview({owner:publishDraftOwner(session.id),revision:session.revision,serviceID:serviceId.value,fields:form,service:previewService.value,valid:()=>sameSession(session)&&draft.contextID()===submitted.context,complete:()=>{
    draft.complete(submitted)
    // 已发布的原稿返回时重新初始化，不能再次发布同一份内容；提交期间的新输入继续保留。
    if(sameSession(session)&&JSON.stringify(serviceDraftFields(form))===submitted.signature)loadedSession=null
  }})
  go('/pages/services/preview'+(serviceId.value?'?id='+encodeURIComponent(serviceId.value):''))
}
onLoad(options=>{serviceId.value=options?.id||'';load()})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage();draft.dispose()})
</script>
<style scoped>
.game-selector{margin-bottom:30.7692rpx;padding:0 0 23.0769rpx}.game-selector .field{margin:0}.picker-value{justify-content:space-between;gap:23.0769rpx}.form-panel{margin-bottom:0}.cover-options{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx}.cover-option{position:relative;min-width:0;height:123.0769rpx;min-height:44px;padding:0;border:3.8462rpx solid transparent;border-radius:15.3846rpx;overflow:hidden;background:transparent}.cover-option.chosen{border-color:var(--accent)}.cover-image{width:100%;height:100%}.cover-check{position:absolute;right:3.8462rpx;bottom:3.8462rpx;display:flex;align-items:center;justify-content:center;width:38.4615rpx;height:38.4615rpx;border-radius:50%;background:var(--accent);color:var(--bg)}.cover-text-heading{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.cover-text-title{display:flex;align-items:baseline;gap:15.3846rpx;min-width:0}.label-count{font-size:23.0769rpx;color:var(--muted)}.add-label{min-height:44px;padding:0}.price-grid{display:grid;grid-template-columns:1fr 1fr;gap:23.0769rpx}.description{height:269.2308rpx}.editor-actions{position:sticky;bottom:0;z-index:3;background:var(--bg);padding:23.0769rpx 0 calc(15.3846rpx + env(safe-area-inset-bottom))}.next-preview{width:100%}
.platform-deposit-note{display:flex;align-items:flex-start;gap:15.3846rpx;min-width:0;padding:0 0 30.7692rpx;border-bottom:1px solid var(--line-subtle);color:var(--accent)}.platform-deposit-copy{display:flex;flex:1;flex-direction:column;gap:5.7692rpx;min-width:0}.platform-deposit-title{font-size:23.0769rpx;line-height:34.6154rpx;color:var(--text)}.platform-deposit-detail{padding:0;font-size:19.2308rpx;line-height:32.6923rpx;overflow-wrap:anywhere}.platform-wallet-entry{flex-shrink:0;min-height:44px;padding:0;font-size:19.2308rpx;gap:5.7692rpx}
</style>
