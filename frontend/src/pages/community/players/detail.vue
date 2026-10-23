<!-- 文件作用：公开玩家名片与可售服务主页，访客可浏览，用户可分享自己的真实服务。 -->
<template><AppShell title="玩家主页" :immersive="!!profile?.profile_cover" back><view v-if="loading&&!profile" class="loading-line">正在读取玩家主页…</view><view v-if="error" class="error-box public-error" @click="retryFailed">{{ error }} · 点击重试</view><template v-if="profile"><view v-if="profile.profile_cover" class="player-cover"><image :src="assetUrl(profile.profile_cover)" mode="aspectFill" /></view><view class="player-content" :class="{'has-cover':!!profile.profile_cover}"><view class="player-heading"><view class="public-avatar"><image v-if="profile.avatar" :src="assetUrl(profile.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="38"/></view><view><text class="player-name">{{ profile.nickname }}</text><text class="player-tag">{{ clubs.length ? clubs.map(club=>club.name).join(' · ')+' · 认证成员' : '玩家 · 自由打手' }}</text></view></view><view v-if="state.user?.id===profile.id" class="own-actions"><button class="text-btn" @click="go('/pages/account/profile')">编辑资料 <Icon name="edit" :size="13"/></button></view><view v-if="profile.platform_deposit_cents||clubDeposits.length" class="player-deposits"><DepositBadge :amount-cents="profile.platform_deposit_cents" show-amount/><button v-for="deposit in clubDeposits" :key="deposit.club_id" class="player-club-deposit" @click="go('/pages/community/clubs/detail?id='+deposit.club_id)"><DepositBadge kind="user_club" :amount-cents="deposit.amount_cents" show-amount/><text class="deposit-club-name">{{ deposit.club_name }}</text></button></view><text class="player-bio">{{ profile.bio || '热爱游戏，期待下一次并肩。' }}</text><view v-if="clubs.length" class="player-clubs"><button v-for="club in clubs" :key="club.id" @click="go('/pages/community/clubs/detail?id='+club.id)"><Icon name="shield" :size="13"/>{{ club.name }}<Icon name="arrow" :size="12"/></button></view><view class="public-section"><view><text>正在提供的服务</text><text class="service-total">{{ services.length }} 项</text></view><ShareEntry kind="player" :target-id="profile.id" label="分享主页"/></view><view v-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service"/></view><EmptyState v-else-if="!loading" title="暂未上架服务" detail="这位玩家正在准备下一次组队。" icon="game"/><ListLoadState v-if="services.length" :loading="loading" :has-more="hasMore" :error="error" @retry="retryFailed"/></view></template></AppShell></template>
<script setup lang="ts">
import { ref,computed } from 'vue'
import { onLoad, onShow, onHide, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import ServiceCard from '../../../components/ServiceCard.vue'
import DepositBadge from '../../../components/DepositBadge.vue'
import EmptyState from '../../../components/EmptyState.vue'
import ListLoadState from '../../../components/ListLoadState.vue'
import {useNativePageRefresh} from '../../../lib/native-page-refresh'
import {appendUniqueRows} from '../../../lib/list-page'
import Icon from '../../../components/Icon.vue'
import ShareEntry from '../../../components/ShareEntry.vue'
import { isTimelinePreview, enablePublicShareMenu } from '../../../lib/sharing'
import { createPublicShareHandlers } from '../../../lib/use-public-share'
import { state, game, init, sessionRevision } from '../../../lib/store'
import { getPlayerProfile } from '../../../api/users.api'
import type { PublicProfile, Service, Club } from '../../../types/models'
import { errorMessage,errorStatus } from '../../../api/client'
import { go, assetUrl, notify } from '../../../lib/format'
const id=ref(''),profile=ref<PublicProfile | null>(null),services=ref<Service[]>([]),clubs=ref<Club[]>([]),loading=ref(true),error=ref('')
/** 公开主页只呈现仍有效的实缴俱乐部押金，规则金额和退款中金额不参与。 */
const clubDeposits=computed(()=>(profile.value?.club_deposits||[]).filter(item=>Number.isSafeInteger(item.amount_cents)&&item.amount_cents>0))
const hasMore=ref(false)
let version=0,nextOffset=0,visible=true
let lastFailedAppend=false
/** 获取只包含公开资料和当前可售服务的名片。 */
async function load(append=false){
  const adding=append===true;if(!id.value||!visible||adding&&(loading.value||!hasMore.value))return
  const current=++version,target=id.value,offset=adding?nextOffset:0;loading.value=true;error.value=''
  try{await init(false,isTimelinePreview());if(current!==version||!visible)return;const data=await getPlayerProfile(target,isTimelinePreview(),{limit:20,offset});if(current!==version||!visible||target!==id.value)return
    const batch=data.services||[]
    services.value=appendUniqueRows(adding?services.value:[],batch.filter(item=>item.status==='active'))
    if(!adding){profile.value=data.profile;clubs.value=data.clubs||[]}
    nextOffset=offset+batch.length;hasMore.value=batch.length===20
  }catch(err){if(current===version&&visible){if(errorStatus(err)===404){profile.value=null;services.value=[];clubs.value=[];hasMore.value=false}lastFailedAppend=adding;error.value=errorMessage(err)}}finally{if(current===version)loading.value=false}
}
/** 重试失败时的首读或追加页，目标失效后重新查询首屏。 */
async function retryFailed(){if(loading.value)return;await load(lastFailedAppend&&!!profile.value)}
/** 读取被分享的公开用户编号。 */
function applyRoute(options: Record<string,string | undefined> = {}){id.value=options.id||state.user?.id||'';if(!id.value){loading.value=false;error.value='玩家主页不存在'}else load()}
onLoad(applyRoute)
onShow(()=>{visible=true;enablePublicShareMenu();if(id.value&&!loading.value)void load()})
onHide(()=>{visible=false;version++;loading.value=false})
useNativePageRefresh({refresh:()=>load(),more:()=>error.value?retryFailed():load(true),context:()=>[state.user?.id,state.user?.role,sessionRevision(),id.value]})
const publicShareHandlers=createPublicShareHandlers(()=>({title:(profile.value?.nickname||'玩家')+'的主页 · 开机电竞',path:'/pages/community/players/detail?id='+encodeURIComponent(id.value),image:profile.value?.profile_cover||game.value.cover_url}))
onShareAppMessage(publicShareHandlers.friend)
onShareTimeline(publicShareHandlers.timeline)
</script>
<style scoped>
.player-content{min-width:0;padding:15.3846rpx 0 30.7692rpx}.player-content.has-cover{padding:38.4615rpx 30.7692rpx 46.1538rpx}.player-cover{position:relative;width:100%;padding-top:56.25%;overflow:hidden;background:var(--surface)}.player-cover image{position:absolute;inset:0;width:100%;height:100%}.player-heading{display:flex;align-items:center;gap:30.7692rpx}.public-avatar{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:123.0769rpx;height:123.0769rpx;overflow:hidden;border:1px solid var(--line);border-radius:50%;background:var(--surface);color:var(--accent)}.public-avatar image{width:100%;height:100%}.player-heading>view:last-child{flex:1;min-width:0}.player-name{display:block;font-size:46.1538rpx;line-height:61.5385rpx;font-weight:750;overflow-wrap:anywhere}.player-tag{display:block;color:var(--accent);font-size:21.1538rpx;margin-top:7.6923rpx}.player-bio{display:block;white-space:pre-wrap;color:#b6c8bd;font-size:25rpx;line-height:42.3077rpx;margin:30.7692rpx 0}.own-actions{display:flex;justify-content:flex-end;margin-top:15.3846rpx}.player-clubs{display:flex;flex-wrap:wrap;gap:15.3846rpx;margin-top:30.7692rpx}.player-clubs button{display:flex;align-items:center;gap:11.5385rpx;padding:11.5385rpx 19.2308rpx;min-height:53.8462rpx;max-width:100%;background:#ffffff04;border:1px solid var(--line);border-radius:15.3846rpx;font-size:21.1538rpx;color:#bfd1c4}.public-section{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;margin:46.1538rpx 0 23.0769rpx}.public-section>view{display:flex;align-items:center;gap:15.3846rpx;font-size:32.6923rpx;font-weight:700;min-width:0}.service-total{font-size:23.0769rpx;font-weight:400;color:var(--muted)}.public-section>button{display:flex;align-items:center;gap:7.6923rpx;padding:0;min-height:53.8462rpx;color:var(--accent);background:transparent;font-size:21.1538rpx}.public-error{margin:30.7692rpx 0}
.player-deposits{display:flex;flex-wrap:wrap;align-items:flex-start;gap:15.3846rpx;margin-top:26.9231rpx;min-width:0}.player-club-deposit{display:flex;flex-direction:column;gap:5.7692rpx;align-items:flex-start;min-width:0;max-width:100%;padding:0;background:transparent;text-align:left}.deposit-club-name{display:block;max-width:346.1538rpx;color:var(--muted);font-size:19.2308rpx;line-height:30.7692rpx;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
</style>
