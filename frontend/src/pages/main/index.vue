<!-- 文件作用：唯一主页面常驻四个业务视图和一个底部导航，切换不重建页面。 -->
<template>
	<view class="main-shell" :style="[themeStyle, safeStyle]">
		<view class="main-viewport">
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'player' }">
				<PlayerView :active="mainNavigation.tab === 'player'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll main-scroll-safe" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'worker' }">
				<WorkerView :active="mainNavigation.tab === 'worker'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll main-scroll-safe" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'orders' }">
				<OrdersView :active="mainNavigation.tab === 'orders'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'me' }">
				<MeView :active="mainNavigation.tab === 'me'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
		</view>
		<BottomNav v-if="!timelinePreview" :active="mainNavigation.tab" @change="selectMainTab" />
		<PhoneVerification />
		<TechSupportDialog :open="supportOpen" @close="supportOpen=false" />
	</view>
</template>
<script setup lang="ts">
	import { ref } from 'vue'
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
	import { applyMainQuery, mainNavigation, selectMainTab } from '../../lib/navigation'

	const timelinePreview = isTimelinePreview()
	const safe = readSafeArea()
	const safeStyle = { '--status-bar-height': safe.statusBarHeight + 'px', '--capsule-space': safe.capsuleSpace + 'px', '--capsule-right-inset': safe.capsuleRightInset + 'px' }
	const supportOpen = ref(false)
	const publicShareHandlers = createPublicShareHandlers(() => ({ title: '集结 · ' + game.value.name + '好队友，即刻集结', path: '/pages/main/index?tab=player&game_id=' + encodeURIComponent(state.gameId), image: game.value.cover_url }))
	onShareAppMessage(publicShareHandlers.friend)
	onShareTimeline(publicShareHandlers.timeline)
	onShow(enablePublicShareMenu)
	onLoad(async query => { applyMainQuery(isTimelinePreview() ? { ...query, tab: 'player' } : query); if (query?.game_id) { await init(false, isTimelinePreview()); if (state.games.some(item => item.id === query.game_id)) state.gameId = query.game_id } })
</script>
<style scoped>
	.main-shell {
		display: flex;
		flex-direction: column;
		height: 100vh;
		height: 100dvh;
		min-width: 0;
		overflow: hidden;
		background: var(--bg);
		color: var(--text)
	}

	.main-viewport {
		position: relative;
		flex: 1;
		min-height: 0;
		min-width: 0;
		width: 100%;
		max-width: 600px;
		margin: 0 auto
	}

	.main-scroll {
		position: absolute;
		top: 0;
		left: 0;
		right: 0;
		height: 100%;
		width: 100%;
		min-width: 0;
		z-index: 1;
		visibility: visible;
		pointer-events: auto
	}

	/* 不可见页面仍保留原生尺寸和滚动范围，仅关闭绘制与触摸。 */
	.main-scroll-inactive {
		visibility: hidden;
		pointer-events: none;
		z-index: 0
	}

	/* 状态栏避让属于各自固定滚动容器，切标签不能改变父视口高度并截断隐藏列表位置。 */
	.main-scroll-safe {
		top: var(--status-bar-height, 0px);
		height: calc(100% - var(--status-bar-height, 0px))
	}
</style>