<!-- 文件作用：唯一主页面常驻四个业务视图和一个底部导航，切换不重建页面。 -->
<template>
  <view class="main-shell" :style="[themeStyle, safeStyle]">
    <view v-if="mainNavigation.tab !== 'player' && mainNavigation.tab !== 'me'" class="main-status-space" />
    <view class="main-viewport">
      <scroll-view v-show="mainNavigation.tab === 'player'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.player" @scroll="rememberScroll('player', $event)"><PlayerView :active="mainNavigation.tab === 'player'" /><TechSupportFooter @open="supportOpen=true"/></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'worker'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.worker" @scroll="rememberScroll('worker', $event)"><WorkerView :active="mainNavigation.tab === 'worker'" /><TechSupportFooter @open="supportOpen=true"/></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'orders'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.orders" @scroll="rememberScroll('orders', $event)"><OrdersView :active="mainNavigation.tab === 'orders'" /><TechSupportFooter @open="supportOpen=true"/></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'me'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.me" @scroll="rememberScroll('me', $event)"><MeView :active="mainNavigation.tab === 'me'" /><TechSupportFooter @open="supportOpen=true"/></scroll-view>
    </view>
    <BottomNav v-if="!timelinePreview" :active="mainNavigation.tab" @change="selectMainTab" />
    <PhoneVerification />
    <TechSupportDialog :open="supportOpen" @close="supportOpen=false"/>
  </view>
</template>
<script setup lang="ts">
import { reactive,ref } from 'vue'
import { onLoad, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import BottomNav from '../../components/BottomNav.vue'
import PhoneVerification from '../../components/PhoneVerification.vue'
import TechSupportFooter from '../../components/TechSupportFooter.vue'
import TechSupportDialog from '../../components/TechSupportDialog.vue'
import PlayerView from './player/player.vue'
import WorkerView from './worker/worker.vue'
import OrdersView from './orders/orders.vue'
import MeView from './me/me.vue'
import { themeStyle, game, state, init } from '../../lib/store'
import { isTimelinePreview, enablePublicShareMenu } from '../../lib/sharing'
import { createPublicShareHandlers } from '../../lib/use-public-share'
import { readSafeArea } from '../../lib/safe-area'
import { applyMainQuery, mainNavigation, selectMainTab, type MainTab } from '../../lib/navigation'

const timelinePreview=isTimelinePreview()
const safe = readSafeArea()
const safeStyle = { '--status-bar-height': safe.statusBarHeight + 'px', '--capsule-space': safe.capsuleSpace + 'px', '--capsule-right-inset': safe.capsuleRightInset + 'px' }
const positions = reactive<Record<MainTab, number>>({ player: 0, worker: 0, orders: 0, me: 0 })
const supportOpen=ref(false)
/** 保存各标签独立滚动位置，切换与详情返回时继续原浏览位置。 */
function rememberScroll(tab: MainTab, event: { detail: { scrollTop: number } }): void { if (mainNavigation.tab === tab) positions[tab] = event.detail.scrollTop }
const publicShareHandlers=createPublicShareHandlers(()=>({title:'集结 · '+game.value.name+'好队友，即刻集结',path:'/pages/main/index?tab=player&game_id='+encodeURIComponent(state.gameId),image:game.value.cover_url}))
onShareAppMessage(publicShareHandlers.friend)
onShareTimeline(publicShareHandlers.timeline)
onShow(enablePublicShareMenu)
onLoad(async query => { applyMainQuery(isTimelinePreview()?{...query,tab:'player'}:query); if(query?.game_id){await init(false,isTimelinePreview()); if(state.games.some(item=>item.id===query.game_id))state.gameId=query.game_id} })
</script>
<style scoped>
.main-shell{display:flex;flex-direction:column;height:100vh;height:100dvh;min-width:0;overflow:hidden;background:var(--bg);color:var(--text)}
.main-viewport{position:relative;flex:1;min-height:0;min-width:0;width:100%;max-width:600px;margin:0 auto}
.main-scroll{height:100%;width:100%;min-width:0}
.main-status-space{height:var(--status-bar-height);flex-shrink:0}
</style>
