<!-- 文件作用：公开玩家名片与可售服务主页，访客可浏览，用户可分享自己的真实服务。 -->
<template><AppShell immersive active="me"><view v-if="loading" class="loading-line">正在读取玩家主页…</view><view v-else-if="error" class="error-box public-error" @click="load">{{ error }} · 点击重试</view><template v-else-if="profile"><view class="player-hero"><image :src="assetUrl(profile.profile_cover||game.cover_url)" mode="aspectFill"/><view class="hero-shade"/><view class="public-toolbar" :style="nativeHeader"><button @click="back"><Icon name="back" :size="24"/></button><view><button open-type="share" @click="sharePage"><Icon name="upload" :size="21"/></button><button v-if="state.user?.id===profile.id" @click="go('/pages/profile/index')">编辑资料</button></view></view></view><view class="player-content"><view class="player-heading"><view class="public-avatar"><image v-if="profile.avatar" :src="assetUrl(profile.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="38"/></view><view><text class="player-name">{{ profile.nickname }}</text><text class="player-tag">{{ profile.provider_enabled ? '玩家 · 自由打手' : '游戏玩家' }}</text></view></view><text class="player-bio">{{ profile.bio || '热爱游戏，期待下一次并肩。' }}</text><view v-if="clubs.length" class="player-clubs"><button v-for="club in clubs" :key="club.id" @click="go('/pages/club/index?id='+club.id)"><Icon name="shield" :size="13"/>{{ club.name }}<Icon name="arrow" :size="12"/></button></view><view class="public-section"><view><text>正在提供的服务</text><text class="service-total">{{ services.length }} 项</text></view><button open-type="share" @click="sharePage"><Icon name="upload" :size="14"/>分享主页</button></view><view v-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service"/></view><EmptyState v-else title="暂未上架服务" detail="这位玩家正在准备下一次集结。" icon="game"/><view class="public-note"><Icon name="info" :size="14"/>服务约定与交付，请留在订单聊天室。</view></view></template></AppShell></template>
<script setup>
import { ref, onMounted } from 'vue'
import { onLoad, onShow, onHide, onShareAppMessage } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ServiceCard from '../../components/ServiceCard.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, game, init } from '../../lib/store'
import { api } from '../../lib/api'
import { go, assetUrl, notify } from '../../lib/format'
const id=ref(''),profile=ref(null),services=ref([]),clubs=ref([]),loading=ref(true),error=ref(''),nativeHeader=ref({})
let version=0
/** 获取只包含公开资料和当前可售服务的名片。 */
async function load(){if(!id.value)return;const current=++version;loading.value=true;error.value='';try{await init();const data=await api('/players/'+encodeURIComponent(id.value));if(current!==version)return;profile.value=data.profile;services.value=(data.services||[]).filter(item=>item.status==='active');clubs.value=data.clubs||[]}catch(err){if(current===version)error.value=err.message}finally{if(current===version)loading.value=false}}
/** 返回上一页面，没有浏览历史则返回玩家大厅。 */
function back(){if(getCurrentPages().length>1)uni.navigateBack();else go('/pages/index/index')}
/** 在微信里使用原生分享，在网页中复制当前公开主页链接。 */
function sharePage(){
  // #ifdef H5
  uni.setClipboardData({data:location.origin+'/#/pages/player/index?id='+encodeURIComponent(id.value),success:()=>notify('主页链接已复制')})
  // #endif
}
/** 为微信系统胶囊预留顶部操作空间。 */
function fitHeader(){
  // #ifdef MP-WEIXIN
  const capsule=uni.getMenuButtonBoundingClientRect(),info=uni.getWindowInfo();if(capsule?.width)nativeHeader.value={paddingTop:capsule.top+'px',paddingRight:info.windowWidth-capsule.left+12+'px'}
  // #endif
}
/** 读取被分享的公开用户编号。 */
function applyRoute(options){id.value=options.id||state.user?.id||'';if(!id.value){loading.value=false;error.value='玩家主页不存在'}else load()}
onLoad(applyRoute)
onShow(()=>{if(id.value)load()})
onHide(()=>{version++})
onMounted(fitHeader)
onShareAppMessage(()=>({title:(profile.value?.nickname||'玩家')+'的集结主页',path:'/pages/player/index?id='+encodeURIComponent(id.value),imageUrl:assetUrl(profile.value?.profile_cover||game.value.cover_url)}))
</script>
<style scoped>.player-hero{position:relative;height:205px;background:var(--surface);overflow:hidden}.player-hero>image{width:100%;height:100%;position:absolute;inset:0}.hero-shade{position:absolute;inset:0;background:linear-gradient(0deg,var(--bg),#00000015 90%)}.public-toolbar{position:relative;display:flex;align-items:center;justify-content:space-between;padding:28px 18px 0;min-height:34px;box-sizing:content-box}.public-toolbar>view{display:flex;align-items:center;gap:16px}.public-toolbar button{margin:0;padding:0;background:none;color:#eef5ef;font-size:12px;line-height:30px}.player-content{position:relative;padding:0 16px 18px;margin-top:-44px}.player-heading{display:flex;align-items:center;gap:15px}.public-avatar{width:70px;height:70px;border:1px solid #adc5b955;border-radius:50%;overflow:hidden;background:#172b20;display:flex;align-items:center;justify-content:center;color:#b5d1c0}.public-avatar image{width:100%;height:100%}.player-name{display:block;color:#f1f5f2;font-size:25px;line-height:32px;font-weight:750}.player-tag{display:block;color:var(--accent);font-size:11px;margin-top:4px}.player-bio{display:block;white-space:pre-wrap;color:#a8bbb0;font-size:13px;line-height:21px;margin-top:18px}.player-clubs{display:flex;gap:7px;flex-wrap:wrap;margin-top:14px}.player-clubs button{display:flex;align-items:center;gap:4px;margin:0;padding:3px 8px;line-height:20px;border:1px solid #ffffff21;border-radius:14px;background:#ffffff04;color:#b7d2c3;font-size:11px}.public-section{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:27px 0 13px}.public-section>view{display:flex;align-items:center;gap:8px;font-size:18px;font-weight:750}.public-section .service-total{font-size:12px;font-weight:400;color:#80988a}.public-section button{display:flex;align-items:center;gap:4px;font-size:11px;background:none;color:var(--accent);padding:0;margin:0;line-height:26px}.public-note{display:flex;justify-content:center;gap:5px;color:#829a8a;font-size:10px;margin-top:24px}.public-error{margin:25px 16px}</style>
