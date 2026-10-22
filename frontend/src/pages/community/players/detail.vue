<!-- 文件作用：公开玩家名片与可售服务主页，访客可浏览，用户可分享自己的真实服务。 -->
<template><AppShell title="玩家主页" :immersive="!!profile?.profile_cover" back><view v-if="loading" class="loading-line">正在读取玩家主页…</view><view v-else-if="error" class="error-box public-error" @click="load">{{ error }} · 点击重试</view><template v-else-if="profile"><view v-if="profile.profile_cover" class="player-cover"><image :src="assetUrl(profile.profile_cover)" mode="aspectFill" /></view><view class="player-content" :class="{'has-cover':!!profile.profile_cover}"><view class="player-heading"><view class="public-avatar"><image v-if="profile.avatar" :src="assetUrl(profile.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="38"/></view><view><text class="player-name">{{ profile.nickname }}</text><text class="player-tag">{{ profile.provider_enabled ? '玩家 · 自由打手' : '游戏玩家' }}</text></view></view><view v-if="state.user?.id===profile.id" class="own-actions"><button class="text-btn" @click="go('/pages/account/profile')">编辑资料 <Icon name="edit" :size="13"/></button></view><text class="player-bio">{{ profile.bio || '热爱游戏，期待下一次并肩。' }}</text><view v-if="clubs.length" class="player-clubs"><button v-for="club in clubs" :key="club.id" @click="go('/pages/community/clubs/detail?id='+club.id)"><Icon name="shield" :size="13"/>{{ club.name }}<Icon name="arrow" :size="12"/></button></view><view class="public-section"><view><text>正在提供的服务</text><text class="service-total">{{ services.length }} 项</text></view><button open-type="share" @click="sharePage"><Icon name="upload" :size="14"/>分享主页</button></view><view v-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service"/></view><EmptyState v-else title="暂未上架服务" detail="这位玩家正在准备下一次组队。" icon="game"/></view></template></AppShell></template>
<script setup lang="ts">
import { ref } from 'vue'
import { onLoad, onShow, onHide, onShareAppMessage } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import ServiceCard from '../../../components/ServiceCard.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import { state, game, init } from '../../../lib/store'
import { getPlayerProfile } from '../../../api/users.api'
import type { PublicProfile, Service, Club } from '../../../types/models'
import { errorMessage } from '../../../api/client'
import { go, assetUrl, notify } from '../../../lib/format'
const id=ref(''),profile=ref<PublicProfile | null>(null),services=ref<Service[]>([]),clubs=ref<Club[]>([]),loading=ref(true),error=ref('')
let version=0
/** 获取只包含公开资料和当前可售服务的名片。 */
async function load(){if(!id.value)return;const current=++version;loading.value=true;error.value='';try{await init();const data=await getPlayerProfile(id.value);if(current!==version)return;profile.value=data.profile;services.value=(data.services||[]).filter(item=>item.status==='active');clubs.value=data.clubs||[]}catch(err){if(current===version)error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 在微信里使用原生分享，在网页中复制当前公开主页链接。 */
function sharePage(){
  // #ifdef H5
  uni.setClipboardData({data:location.origin+'/#/pages/community/players/detail?id='+encodeURIComponent(id.value),success:()=>notify('主页链接已复制')})
  // #endif
}
/** 读取被分享的公开用户编号。 */
function applyRoute(options: Record<string,string | undefined> = {}){id.value=options.id||state.user?.id||'';if(!id.value){loading.value=false;error.value='玩家主页不存在'}else load()}
onLoad(applyRoute)
onShow(()=>{if(id.value)load()})
onHide(()=>{version++})
onShareAppMessage(()=>({title:(profile.value?.nickname||'玩家')+'的主页 · 开机电竞',path:'/pages/community/players/detail?id='+encodeURIComponent(id.value),imageUrl:assetUrl(profile.value?.profile_cover||game.value.cover_url)}))
</script>
<style scoped>
.player-content{min-width:0;padding:8px 0 16px}.player-content.has-cover{padding:20px 16px 24px}.player-cover{position:relative;width:100%;padding-top:56.25%;overflow:hidden;background:var(--surface)}.player-cover image{position:absolute;inset:0;width:100%;height:100%}.player-heading{display:flex;align-items:center;gap:16px}.public-avatar{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:64px;height:64px;overflow:hidden;border:1px solid var(--line);border-radius:50%;background:var(--surface);color:var(--accent)}.public-avatar image{width:100%;height:100%}.player-heading>view:last-child{flex:1;min-width:0}.player-name{display:block;font-size:24px;line-height:32px;font-weight:750;overflow-wrap:anywhere}.player-tag{display:block;color:var(--accent);font-size:11px;margin-top:4px}.player-bio{display:block;white-space:pre-wrap;color:#b6c8bd;font-size:13px;line-height:22px;margin:16px 0}.own-actions{display:flex;justify-content:flex-end;margin-top:8px}.player-clubs{display:flex;flex-wrap:wrap;gap:8px;margin-top:16px}.player-clubs button{display:flex;align-items:center;gap:6px;padding:6px 10px;min-height:28px;max-width:100%;background:#ffffff04;border:1px solid var(--line);border-radius:8px;font-size:11px;color:#bfd1c4}.public-section{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:24px 0 12px}.public-section>view{display:flex;align-items:center;gap:8px;font-size:17px;font-weight:700;min-width:0}.service-total{font-size:12px;font-weight:400;color:var(--muted)}.public-section>button{display:flex;align-items:center;gap:4px;padding:0;min-height:28px;color:var(--accent);background:transparent;font-size:11px}.public-error{margin:16px 0}
</style>
