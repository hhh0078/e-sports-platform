<!-- 文件作用：发布与编辑游戏服务，配置游戏封面、文字样式和俱乐部挂靠。 -->
<template>
  <AppShell :title="serviceId ? '编辑服务' : '发布服务'" :game-id="form.game_id" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <template>

      <view class="form-section game-selector"><view class="field service-game-field"><text class="field-label">选择游戏</text><GameSelector :value="form.game_id" :disabled="busy" @change="changeGame"/></view></view><view class="preview"><view class="preview-toolbar"><text>封面预览</text><button v-if="covers.length>1" class="text-btn" @click="changeCover">换一张底图</button></view><ServiceCard :service="previewService" :interactive="false"/></view>
      <view class="platform-deposit-note"><Icon name="wallet" :size="17"/><view class="platform-deposit-copy"><text class="platform-deposit-title">平台押金 · 自愿缴纳</text><text class="subtle-note platform-deposit-detail">缴纳后展示平台押金标识，不影响发布</text></view><button class="text-btn platform-wallet-entry" @click="go(state.user?'/pages/account/wallet':'/pages/account/login?next='+encodeURIComponent('/pages/account/wallet'))">我的钱包<Icon name="arrow" :size="11"/></button></view>
      <view class="form-section form-panel"><text class="section-title">封面文字</text><text class="hint block">默认左下依次排列，位置和样式可独立设置。</text>

        <text class="field-label">游戏底图 · 也可以自己选</text>
        <view class="cover-options"><view v-for="src in covers" :key="src" :class="['cover-option',{chosen:form.cover_url===src}]" @click="form.cover_url=src"><image :src="assetUrl(src)" mode="aspectFill"/><view v-if="form.cover_url===src" class="cover-check"><Icon name="check" :size="12"/></view></view></view>
        <view class="section-row"><text class="field-label">叠加文字 {{ form.labels.length }} / 6</text><button class="text-btn" :disabled="form.labels.length>=6" @click="addLabel"><Icon name="plus" :size="15"/>添加文字</button></view>
        <view v-if="!form.labels.length" class="subtle-note">也可以保持纯图片封面。添加文字可展示段位、服务特色或限时活动。</view>
        <view v-for="(label,index) in form.labels" :key="label.id" class="label-editor"><view class="row between"><text class="label-index">文案 {{ index+1 }}</text><button class="text-btn danger" @click="form.labels.splice(index,1)">删除</button></view><input v-model="label.text" class="input" maxlength="48" placeholder="最多12个字，表情完整保留" @blur="label.text=normalizeLabelText(label.text)"/><view class="colors"><view class="color-field"><text>文字颜色</text><view class="swatches"><button v-for="color in textColors" :key="color" :style="{background:color}" :class="{chosen:label.color===color}" :aria-label="'文字颜色'+color" @click="label.color=color"/></view><input v-model="label.color" class="color-code" maxlength="7" placeholder="#ffffff"/></view><view class="color-field"><text>背景颜色</text><view class="swatches"><button v-for="color in backgroundColors" :key="color" :style="{background:color}" :class="{chosen:label.background===color,transparent:color==='transparent'}" :aria-label="'背景颜色'+color" @click="label.background=color"/></view><input v-model="label.background" class="color-code" maxlength="11" placeholder="#000000"/></view></view><view class="label-bottom"><button class="position-picker" @click="positionEditor=positionEditor===label.id?'':label.id">位置 · {{ positionName(label.position) }} <Icon name="bottom" :size="11"/></button><view class="font-picker"><text>字号 {{ label.font_size }}</text><slider :value="label.font_size" :min="12" :max="24" :step="1" :activeColor="selectedGame?.primary_color" :block-size="sliderBlockSize" @change="label.font_size=$event.detail.value"/></view></view><view v-if="positionEditor===label.id" class="position-grid"><button v-for="position in POSITIONS" :key="position[0]" :class="{selected:label.position===position[0]}" @click="label.position=position[0];positionEditor=''">{{ position[1] }}</button></view></view>
      </view>
      <view class="form-section form-panel"><text class="section-title">服务信息</text><view class="field"><text class="field-label">服务标题</text><input v-model="form.title" class="input" maxlength="40" placeholder="例如：三角洲双人护航 · 耐心沟通"/></view><view class="field"><text class="field-label">服务类型</text><picker :range="selectedGame?.categories || []" :value="categoryIndex" @change="form.category=selectedGame.categories[Number($event.detail.value)]"><view class="input picker-value">{{ form.category || '请选择' }}<Icon name="arrow" :size="14"/></view></picker></view><view class="price-grid"><view class="field"><text class="field-label">价格（元）</text><input v-model="form.price" type="digit" class="input" placeholder="例如 29"/></view><view class="field"><text class="field-label">计价单位</text><picker :range="units" :value="units.indexOf(form.unit)" @change="form.unit=units[Number($event.detail.value)]"><view class="input">每 {{ form.unit }}</view></picker></view></view><view class="field"><text class="field-label">服务说明与交付标准</text><textarea v-model="form.description" class="input description" maxlength="1000" placeholder="明确服务包含什么、预约方式、服务时长、验收标准以及不包含的内容。请勿提供外挂或索取账号密码。"/></view><view class="field"><text class="field-label">服务标签（逗号分隔，最多4个）</text><input v-model="form.tagsText" class="input" maxlength="60" placeholder="耐心沟通,稳定在线,新手友好"/></view><view class="field"><text class="field-label">俱乐部挂靠（可选）</text><picker :range="clubOptions" range-key="name" :value="clubIndex" @change="form.club_id=clubOptions[Number($event.detail.value)].id"><view class="input picker-value">{{ clubOptions[clubIndex]?.name || '自由打手 · 无需挂靠' }}<Icon name="arrow" :size="14"/></view></picker><text class="hint">仅能选择已通过挂靠的俱乐部。认证由平台审核，不影响自由接单。</text></view></view>
      <view class="subtle-note mb">发布后进行图文审核；无AI时由管理员人工处理。审核未通过会通知修改，已有订单的约定保持不变。</view><button class="btn submit" :disabled="busy" @click="save">{{ busy ? '正在发布…' : '发布服务' }}</button>
    </template>
    <SubscribeReminder ref="subscriptionReminder" :scenes="['provider_order']" :entry="false" title="发布成功" description="有人购买你的服务时，及时收到微信提醒。拒绝订阅不影响接单。" button-label="接收新订单提醒"/>
  </AppShell>
</template>
<script setup lang="ts">
import { requirePhone } from '../../lib/phone'
/** 文件作用：维护可保存的封面配置，检查金额与服务内容后发布并进入后置图文审核。 */
import { ref, reactive, computed, watch } from 'vue'
import { onLoad, onShow, onHide, onUnload, onResize } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ServiceCard from '../../components/ServiceCard.vue'
import GameSelector from '../../components/GameSelector.vue'
import Icon from '../../components/Icon.vue'
import SubscribeReminder from '../../components/SubscribeReminder.vue'
import { state, init, sessionRevision } from '../../lib/store'
import { getService, createService, updateService } from '../../api/services.api'
import { listClubs } from '../../api/clubs.api'
import {getWallet} from '../../api/wallet.api'
import {getClubDeposits} from '../../api/finance.api'
import type { Club, CoverLabel, Service } from '../../types/models'
import { token, errorMessage } from '../../api/client'
import { go, notify, assetUrl } from '../../lib/format'
import { newLabel, POSITIONS, gameCovers, randomGameCover, normalizeLabelText } from '../../lib/cover'
import { designPixels } from '../../lib/design-units'
const positionEditor = ref('')
const subscriptionReminder=ref<{offer:(next?:()=>void)=>void}|null>(null)
/** 原生slider旋钮接口使用实际像素，并限制在官方允许的12至28像素范围。 */
const sliderBlockSize=ref(16)
/** 按窗口宽度同步旋钮，避免表单缩放后仍保留固定像素。 */
function refreshSliderBlockSize():void{try{const width=uni.getWindowInfo().windowWidth;if(Number.isFinite(width)&&width>0)sliderBlockSize.value=Math.max(12,Math.min(28,Math.round(designPixels(16,width))))}catch{/* 原生窗口暂不可读时保留可用旋钮尺寸。 */}}
onResize(refreshSliderBlockSize)
/** 发布预览的两类押金只来自本账号服务端记录，不属于可保存的草稿字段。 */
const serverProviderDeposit=ref(0),serverProviderClubDeposit=ref(0)
const serviceId = ref(''), clubs = ref<Club[]>([]), error = ref(''), busy = ref(false)
const form = reactive({ game_id:'delta', title:'', description:'', category:'', price:'', unit:'局', cover_url:'', labels:[] as CoverLabel[], tagsText:'', club_id:'' })
const selectedGame = computed(() => state.games.find(item => item.id===form.game_id) || state.games[0])
const covers = computed(() => gameCovers(selectedGame.value))
const categoryIndex = computed(() => Math.max(0,(selectedGame.value?.categories || []).indexOf(form.category)))
const clubOptions = computed(() => [{id:'',name:'自由打手 · 无需挂靠'},...clubs.value.filter(club=>club.status==='active'&&['owner','active'].includes(club.membership_status||'')&&(club.game_ids||[]).includes(form.game_id))])
const clubIndex = computed(() => Math.max(0,clubOptions.value.findIndex(item=>item.id===form.club_id)))
/** 使用正式服务卡片展示草稿，确保文字和价格区与列表共用同一布局。 */
const previewService = computed<Service>(() => ({ provider_deposit_cents:serverProviderDeposit.value,provider_club_deposit_cents:serverProviderClubDeposit.value, id:serviceId.value, game_id:form.game_id, provider_id:state.user?.id||'', provider_name:state.user?.nickname||'', provider_avatar:state.user?.avatar||'', club_id:form.club_id, club_name:clubs.value.find(club=>club.id===form.club_id)?.name||'', club_verified:clubs.value.some(club=>club.id===form.club_id&&club.verified), title:form.title||'你的服务标题', description:form.description, category:form.category||'服务类型', price_cents:Number.isFinite(Number(form.price))?Math.max(0,Math.round(Number(form.price)*100)):0, unit:form.unit, cover_url:form.cover_url, labels:form.labels, tags:form.tagsText.split(/[,，]/).map(item=>item.trim()).filter(Boolean).slice(0,4), status:'draft', rating:0, completed_count:0, created_at:'' }))
const units = ['局','小时','次','任务'], textColors = ['#ffffff','#00e18a','#f4c36c','#ff6b6b','#101714'], backgroundColors = ['transparent','#07130f','#00e18a','#b98132','#a43c3c']
let disposed=false,visible=true,loadVersion=0,loading=false,platformDepositVersion=0,clubDepositVersion=0,loadedSession: ReturnType<typeof currentSession> | null=null
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
  const current=++loadVersion,entryToken=token(),revision=sessionRevision()
  let session: ReturnType<typeof currentSession> | undefined
  loading=true;loadedSession=null;error.value='';serverProviderDeposit.value=0;serverProviderClubDeposit.value=0;platformDepositVersion++;clubDepositVersion++
  try {
    await init()
    if(!visible||disposed||current!==loadVersion||entryToken!==token()||revision!==sessionRevision())return
    session=currentSession()
    if(serviceId.value){const service=await getService(serviceId.value);if(!currentLoad(current,session))return;serverProviderDeposit.value=service.provider_id===session.id?service.provider_deposit_cents||0:0;Object.assign(form,service,{price:(service.price_cents/100).toFixed(2),tagsText:(service.tags||[]).join(','),labels:service.labels||[]})}
    else{Object.assign(form,{game_id:state.gameId,title:'',description:'',category:'',price:'',unit:'局',cover_url:'',labels:[],tagsText:'',club_id:''});chooseDefaults()}
    const affiliations=state.user?await listClubs({mine:1}):[]
    if(!currentLoad(current,session))return
    clubs.value=affiliations
    if(form.club_id&&!clubOptions.value.some(club=>club.id===form.club_id))form.club_id=''
    loadedSession=session
    void refreshPlatformDeposit();void refreshClubDeposit()
  }catch(e){if(visible&&!disposed&&current===loadVersion&&(session?sameSession(session):entryToken===token()&&revision===sessionRevision()))error.value=errorMessage(e)}
  finally{if(current===loadVersion)loading=false}
}
/** 会话轮换立即移除原身份挂靠；下次显示时按新身份读取，不改其他草稿。 */
function invalidateSession(){loadVersion++;platformDepositVersion++;clubDepositVersion++;loading=false;loadedSession=null;clubs.value=[];form.club_id='';serverProviderDeposit.value=0;serverProviderClubDeposit.value=0}
watch(sessionRevision,invalidateSession,{flush:'sync'})
/** 同一身份已完成加载时保留草稿，取消的读取则在重显后重新开始。 */
function showPage(){refreshSliderBlockSize();visible=true;if(!loading&&(!loadedSession||!sameSession(loadedSession)))load();else if(!loading){void refreshPlatformDeposit();void refreshClubDeposit()}}
/** 隐藏后使旧服务、挂靠和错误响应失效，保留已完成的同账号草稿。 */
function hidePage(){visible=false;loadVersion++;platformDepositVersion++;clubDepositVersion++;loading=false}
/** 为新建服务或新选游戏随机设置底图，并使用当前游戏默认分类。 */
function chooseDefaults() { form.cover_url=randomGameCover(selectedGame.value);form.category=selectedGame.value?.categories?.[0] || '' }
/** 仅更换底图，保留所有文字、颜色、位置和服务信息。 */
function changeCover() { form.cover_url=randomGameCover(selectedGame.value,form.cover_url) }
/** 切换游戏，避免带入不适用的分类或封面。 */
function changeGame(id:string) { const selected=state.games.find(item=>item.id===id&&item.enabled!==false);if(!selected||selected.id===form.game_id)return;form.game_id=selected.id;chooseDefaults();if(!clubOptions.value.some(club=>club.id===form.club_id))form.club_id='' }
/** 加入一条默认左下角文案。 */
function addLabel() { if(form.labels.length<6) form.labels.push(newLabel(selectedGame.value?.primary_color)) }
/** 获取当前九宫格位置索引。 */
function positionIndex(value: string | undefined) { return Math.max(0,POSITIONS.findIndex(item=>item[0]===value)) }
/** 获取中文位置名称。 */
function positionName(value: string | undefined) { return POSITIONS[positionIndex(value)][1] }
/** 单次提交锁覆盖手机号授权与发布，默认接单身份无需另行启用，取消或失败保留草稿。 */
async function save() {
  if(busy.value)return
  if(!state.user)return go('/pages/account/login?next='+encodeURIComponent('/pages/services/publish'+(serviceId.value?'?id='+serviceId.value:'')))
  const session=currentSession();busy.value=true;error.value=''
  try {
    if(!(await requirePhone())||!sameSession(session))return
    form.labels.forEach(label=>{label.text=normalizeLabelText(label.text)})
    const price=Math.round(Number(form.price)*100)
    if(!form.title.trim()||!form.description.trim()||!form.category||!form.cover_url)return notify('请完整填写标题、说明、类型和封面')
    if(!/^\d+(\.\d{1,2})?$/.test(form.price)||price<100||price>1000000)return notify('价格请填写1至10000元，最多两位小数')
    if(form.labels.some(label=>!label.text.trim()||!/^#[0-9a-f]{6}$/i.test(label.color||'')||!(label.background==='transparent'||/^#[0-9a-f]{6}$/i.test(label.background||''))))return notify('请填写文案，并使用正确的六位颜色值')
    const body={game_id:form.game_id,title:form.title.trim(),description:form.description.trim(),category:form.category,price_cents:price,unit:form.unit,cover_url:form.cover_url,labels:form.labels,tags:form.tagsText.split(/[,，]/).map(item=>item.trim()).filter(Boolean).slice(0,4),club_id:form.club_id}
    await (serviceId.value?updateService(serviceId.value,body):createService(body))
    if(!sameSession(session))return
    notify('服务已发布，图文审核中');if(subscriptionReminder.value)subscriptionReminder.value.offer(()=>go('/pages/main/index?tab=worker'));else go('/pages/main/index?tab=worker')
  }catch(e){if(sameSession(session)){error.value=errorMessage(e);notify(e)}}finally{busy.value=false}
}
onLoad(options=>{serviceId.value=options?.id||'';load()})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.game-selector{margin-bottom:30.7692rpx;padding:0 0 23.0769rpx}.game-selector .field{margin:0}.picker-value{justify-content:space-between;gap:23.0769rpx}.preview{min-width:0;margin-bottom:30.7692rpx}.preview-toolbar{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;padding:0 0 23.0769rpx;font-size:28.8462rpx;font-weight:650}.form-panel{margin-bottom:0}.form-panel>.section-title{display:block;margin-bottom:30.7692rpx}.block{display:block;margin:15.3846rpx 0 30.7692rpx}.cover-options{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;margin-top:23.0769rpx}.cover-option{position:relative;height:123.0769rpx;border:3.8462rpx solid transparent;border-radius:15.3846rpx;overflow:hidden}.cover-option.chosen{border-color:var(--accent)}.cover-option image{width:100%;height:100%}.cover-check{position:absolute;right:3.8462rpx;bottom:3.8462rpx;display:flex;align-items:center;justify-content:center;width:38.4615rpx;height:38.4615rpx;border-radius:50%;background:var(--accent);color:var(--bg)}.label-editor{border:0;border-bottom:1px solid var(--line-subtle);border-radius:0;padding:30.7692rpx 0;margin-top:0}.label-index{font-size:23.0769rpx;color:var(--muted)}.colors{display:flex;flex-direction:column;gap:23.0769rpx;margin-top:23.0769rpx}.color-field{display:flex;align-items:center;flex-wrap:wrap;gap:15.3846rpx;min-width:0}.color-field>text{font-size:23.0769rpx;min-width:92.3077rpx;color:var(--muted)}.swatches{display:flex;gap:15.3846rpx;min-width:0}.swatches button{display:flex;width:53.8462rpx;height:53.8462rpx;min-height:53.8462rpx;padding:0;border:3.8462rpx solid transparent;border-radius:9.6154rpx}.swatches button.chosen{border-color:var(--accent);box-shadow:0 0 0 1px var(--bg)}.swatches button.transparent{background:repeating-linear-gradient(45deg,#ffffff10 0 7.6923rpx,transparent 7.6923rpx 15.3846rpx);border-color:var(--line)}.color-code{height:53.8462rpx;font-family:monospace;font-size:21.1538rpx;color:var(--muted);width:146.1538rpx;margin-left:auto;padding:5.7692rpx 11.5385rpx;border:1px solid var(--line);border-radius:9.6154rpx}.label-bottom{display:flex;flex-direction:column;gap:23.0769rpx;margin-top:30.7692rpx}.position-picker{display:flex;align-items:center;justify-content:space-between;width:100%;height:69.2308rpx;padding:0 23.0769rpx;border:0;border-bottom:1px solid var(--line);border-radius:0;background:transparent;color:var(--text);font-size:23.0769rpx}.font-picker{display:flex;align-items:center;gap:23.0769rpx;font-size:23.0769rpx;color:var(--muted)}.font-picker slider{flex:1;margin:0}.position-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;margin-top:23.0769rpx}.position-grid button{height:69.2308rpx;padding:0;border:1px solid var(--line);border-radius:11.5385rpx;background:transparent;color:var(--muted);font-size:23.0769rpx;line-height:65.3846rpx}.position-grid button.selected{border-color:var(--accent);color:var(--accent)}.price-grid{display:grid;grid-template-columns:1fr 1fr;gap:23.0769rpx}.description{height:269.2308rpx}.submit{position:sticky;bottom:23.0769rpx;width:100%;z-index:3;box-shadow:0 0 0 15.3846rpx var(--bg);margin-top:46.1538rpx}.label-editor .input{font-size:25rpx}
.platform-deposit-note{display:flex;align-items:flex-start;gap:15.3846rpx;min-width:0;padding:0 0 30.7692rpx;border-bottom:1px solid var(--line-subtle);color:var(--accent)}.platform-deposit-copy{display:flex;flex:1;flex-direction:column;gap:5.7692rpx;min-width:0}.platform-deposit-title{font-size:23.0769rpx;line-height:34.6154rpx;color:var(--text)}.platform-deposit-detail{padding:0;font-size:19.2308rpx;line-height:32.6923rpx;overflow-wrap:anywhere}.platform-wallet-entry{flex-shrink:0;min-height:53.8462rpx;padding:0;font-size:19.2308rpx;gap:5.7692rpx}
</style>
