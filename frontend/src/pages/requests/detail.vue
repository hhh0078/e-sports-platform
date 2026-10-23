<!-- 文件作用：以公开白名单需求展示分享落地，接单仍经过微信登录、手机号与服务端资格校验。 -->
<template><AppShell title="需求详情" immersive :game-id="job?.game_id"><view v-if="job" class="request-content"><view class="request-cover"><image class="request-cover-image" :src="assetUrl(selectedGame?.cover_url)||'/static/share/default-cover.jpg'" mode="aspectFill"/><view class="request-shade"/><text class="request-cover-caption">{{ job.game_name||selectedGame?.name }} · 公开需求</text></view><view class="request-body"><text class="request-title">{{ job.title }}</text><view class="request-meta"><text class="pill">{{ statusLabel(job.status) }}</text><text>{{ time(job.created_at) }} 发布</text></view><view class="request-price">¥{{ money(job.amount_cents) }}<text class="request-price-note">服务预算</text></view><view class="request-description"><text class="section-title">需求说明</text><text class="request-text">{{ job.requirements }}</text></view><text v-if="timelinePreview" class="subtle-note">请从微信顶部打开小程序，查看更多或接单。</text><view v-else class="request-actions"><ShareEntry kind="request" :target-id="job.id" :game-id="job.game_id" label="分享需求"/><button class="btn request-claim" :disabled="claiming||job.status!=='open'" @click="claim">{{ claiming?'领取中…':'接下这个需求' }}</button></view><text class="subtle-note">接单后在订单聊天室确认安排，沟通与交付记录会保留。</text></view></view><view v-else-if="loading" class="request-state loading-line">正在读取需求…</view><view v-else class="request-state error-box stack"><text>{{ error }}</text><button class="btn-secondary" @click="load">重新加载</button></view></AppShell></template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onUnload, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ShareEntry from '../../components/ShareEntry.vue'
import { getPublicRequest } from '../../api/sharing.api'
import { claimJob } from '../../api/orders.api'
import { listClubs } from '../../api/clubs.api'
import { token, errorMessage } from '../../api/client'
import { init, state, sessionRevision } from '../../lib/store'
import { assetUrl, go, money, notify, statusLabel, time } from '../../lib/format'
import { requirePhone } from '../../lib/phone'
import { isTimelinePreview, enablePublicShareMenu } from '../../lib/sharing'
import { createPublicShareHandlers } from '../../lib/use-public-share'
import type { Club, PublicJob } from '../../types/models'
const job=ref<PublicJob|null>(null),loading=ref(true),error=ref(''),claiming=ref(false),timelinePreview=isTimelinePreview(),selectedGame=computed(()=>state.games.find(game=>game.id===job.value?.game_id))
let id='',disposed=false,version=0
/** 匿名读取白名单公开需求，关闭后的迟到响应不能回写。 */
async function load():Promise<void>{const current=++version;loading.value=true;error.value='';try{if(!id)throw new Error('需求不存在');await init(false,timelinePreview);const result=await getPublicRequest(id);if(!disposed&&current===version)job.value=result}catch(err){if(!disposed&&current===version){job.value=null;error.value=errorMessage(err)}}finally{if(!disposed&&current===version)loading.value=false}}
/** 读取所有可用挂靠，避免仅第一页可选；服务端仍重新验证接单资格。 */
async function availableClubs(gameID:string):Promise<Club[]>{const clubs:Club[]=[];while(clubs.length<1000){const batch=await listClubs({mine:1,limit:100,offset:clubs.length});clubs.push(...batch);if(batch.length<100)break}return clubs.filter(club=>club.status==='active'&&['owner','active'].includes(club.membership_status||'')&&club.game_ids.includes(gameID))}
/** 官方操作菜单按三家俱乐部分页，取消不认领需求。 */
function selectClub(clubs:Club[],page=0):Promise<string|null>{const options:Array<{title:string;id?:string;page?:number}>=[{title:'独立接单（不挂靠）',id:''},...clubs.slice(page*3,page*3+3).map(club=>({title:club.name,id:club.id}))];if(page>0)options.push({title:'上一页俱乐部',page:page-1});if((page+1)*3<clubs.length)options.push({title:'下一页俱乐部',page:page+1});return new Promise(resolve=>uni.showActionSheet({itemList:options.map(item=>item.title),success:result=>{const option=options[result.tapIndex];resolve(!option?null:option.page===undefined?option.id??null:selectClub(clubs,option.page))},fail:()=>resolve(null)}))}
/** 同一提交锁覆盖授权和挂靠选择，账号变化或取消不得误创建订单。 */
async function claim():Promise<void>{if(timelinePreview)return notify('请从微信顶部打开小程序后接单');if(!job.value||claiming.value)return;if(!state.user)return go('/pages/account/login?next='+encodeURIComponent('/pages/requests/detail?id='+id));const userID=state.user.id,requestToken=token(),revision=sessionRevision();const same=()=>!disposed&&state.user?.id===userID&&token()===requestToken&&sessionRevision()===revision;claiming.value=true;try{if(!(await requirePhone())||!same())return;const clubs=await availableClubs(job.value.game_id);if(!same())return;const clubID=clubs.length?await selectClub(clubs):'';if(clubID===null||!same())return;const order=await claimJob(id,clubID);if(same())go('/pages/orders/detail?id='+order.id)}catch(err){if(same()){notify(err);void load()}}finally{claiming.value=false}}
const publicShareHandlers=createPublicShareHandlers(()=>({title:job.value?.title||'集结公开需求',path:'/pages/requests/detail?id='+encodeURIComponent(id),image:selectedGame.value?.cover_url||''}))
onShareAppMessage(publicShareHandlers.friend)
onShareTimeline(publicShareHandlers.timeline)
onShow(enablePublicShareMenu)
onLoad(options=>{id=options?.id||'';void load()})
onUnload(()=>{disposed=true;version++})
</script>
<style scoped>.request-cover{position:relative;height:500rpx;overflow:hidden;background:var(--surface)}.request-cover-image{display:block;width:100%;height:100%}.request-shade{position:absolute;inset:0;background:linear-gradient(180deg,transparent 45%,var(--bg))}.request-cover-caption{position:absolute;left:30.7692rpx;right:30.7692rpx;bottom:30.7692rpx;font-size:40.3846rpx;line-height:57.6923rpx;font-weight:750;overflow-wrap:anywhere;color:var(--text)}.request-body{padding:30.7692rpx;min-width:0}.request-title{display:block;font-size:46.1538rpx;line-height:65.3846rpx;font-weight:750;overflow-wrap:anywhere}.request-meta{display:flex;align-items:center;flex-wrap:wrap;gap:23.0769rpx;margin:23.0769rpx 0;color:var(--muted);font-size:23.0769rpx;line-height:38.4615rpx}.request-price{display:flex;align-items:baseline;gap:23.0769rpx;min-width:0;padding:30.7692rpx 0;border-top:1px solid var(--line);border-bottom:1px solid var(--line);color:var(--accent);font-size:69.2308rpx;line-height:84.6154rpx;font-weight:750}.request-price-note{font-size:23.0769rpx;color:var(--muted);font-weight:400}.request-description{padding:46.1538rpx 0}.request-text{display:block;margin-top:23.0769rpx;white-space:pre-wrap;overflow-wrap:anywhere;font-size:26.9231rpx;line-height:48.0769rpx;color:var(--muted)}.request-actions{display:flex;align-items:center;gap:30.7692rpx;padding:23.0769rpx 0}.request-claim{flex:1;min-width:0}.request-state{margin:192.3077rpx 30.7692rpx 46.1538rpx}</style>
