<!-- 文件作用：唯一主页面常驻四个业务视图和一个底部导航，切换不重建页面。 -->
<template>
  <view class="main-shell" :style="[themeStyle, safeStyle]">
    <view v-if="mainNavigation.tab !== 'player' && mainNavigation.tab !== 'me'" class="main-status-space" />
    <view class="main-viewport">
      <scroll-view v-show="mainNavigation.tab === 'player'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.player" @scroll="rememberScroll('player', $event)"><PlayerView :active="mainNavigation.tab === 'player'" /></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'worker'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.worker" @scroll="rememberScroll('worker', $event)"><WorkerView :active="mainNavigation.tab === 'worker'" /></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'orders'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.orders" @scroll="rememberScroll('orders', $event)"><OrdersView :active="mainNavigation.tab === 'orders'" /></scroll-view>
      <scroll-view v-show="mainNavigation.tab === 'me'" scroll-y :show-scrollbar="false" class="main-scroll" :scroll-top="positions.me" @scroll="rememberScroll('me', $event)"><MeView :active="mainNavigation.tab === 'me'" /></scroll-view>
    </view>
    <BottomNav :active="mainNavigation.tab" @change="selectMainTab" />
    <PhoneVerification />
  </view>
</template>
<script setup lang="ts">
import { reactive } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import BottomNav from '../../components/BottomNav.vue'
import PhoneVerification from '../../components/PhoneVerification.vue'
import PlayerView from './player/player.vue'
import WorkerView from './worker/worker.vue'
import OrdersView from './orders/orders.vue'
import MeView from './me/me.vue'
import { themeStyle } from '../../lib/store'
import { readSafeArea } from '../../lib/safe-area'
import { applyMainQuery, mainNavigation, selectMainTab, type MainTab } from '../../lib/navigation'

const safe = readSafeArea()
const safeStyle = { '--status-bar-height': safe.statusBarHeight + 'px', '--capsule-space': safe.capsuleSpace + 'px', '--capsule-right-inset': safe.capsuleRightInset + 'px' }
const positions = reactive<Record<MainTab, number>>({ player: 0, worker: 0, orders: 0, me: 0 })
/** 保存各标签独立滚动位置，切换与详情返回时继续原浏览位置。 */
function rememberScroll(tab: MainTab, event: { detail: { scrollTop: number } }): void { if (mainNavigation.tab === tab) positions[tab] = event.detail.scrollTop }
onLoad(query => applyMainQuery(query))
</script>
<style scoped>
.main-shell{display:flex;flex-direction:column;height:100vh;height:100dvh;min-width:0;overflow:hidden;background:var(--bg);color:var(--text)}
.main-viewport{position:relative;flex:1;min-height:0;min-width:0;width:100%;max-width:600px;margin:0 auto}
.main-scroll{height:100%;width:100%;min-width:0}
.main-status-space{height:var(--status-bar-height);flex-shrink:0}
</style>
