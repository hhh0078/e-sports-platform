<!-- 文件作用：唯一主页面常驻四个业务视图和一个底部导航，切换不重建页面。 -->
<template>
	<view class="main-shell" :style="[themeStyle, safeStyle]">
		<view class="main-viewport">
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'player' }" :refresher-enabled="mainNavigation.tab==='player'" :refresher-triggered="refreshing.player" refresher-background="transparent" refresher-default-style="white" @refresherrefresh="runGesture('player','refresh')" @scrolltolower="runGesture('player','more')">
				<PlayerView ref="playerView" :active="mainNavigation.tab === 'player'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll main-scroll-safe" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'worker' }" :refresher-enabled="mainNavigation.tab==='worker'" :refresher-triggered="refreshing.worker" refresher-background="transparent" refresher-default-style="white" @refresherrefresh="runGesture('worker','refresh')" @scrolltolower="runGesture('worker','more')">
				<ContentTip class="main-content-tip" :tip="publicContent?.page_tips.worker"/>
				<WorkerView ref="workerView" :active="mainNavigation.tab === 'worker'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll main-scroll-safe" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'orders' }" :refresher-enabled="mainNavigation.tab==='orders'" :refresher-triggered="refreshing.orders" refresher-background="transparent" refresher-default-style="white" @refresherrefresh="runGesture('orders','refresh')" @scrolltolower="runGesture('orders','more')">
				<ContentTip class="main-content-tip" :tip="publicContent?.page_tips.orders"/>
				<OrdersView ref="ordersView" :active="mainNavigation.tab === 'orders'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
			<scroll-view scroll-y :show-scrollbar="false" class="main-scroll" :class="{ 'main-scroll-inactive': mainNavigation.tab !== 'me' }" :refresher-enabled="mainNavigation.tab==='me'" :refresher-triggered="refreshing.me" refresher-background="transparent" refresher-default-style="white" @refresherrefresh="runGesture('me','refresh')" @scrolltolower="runGesture('me','more')">
				<MeView ref="meView" :active="mainNavigation.tab === 'me'" />
				<TechSupportFooter @open="supportOpen=true" />
			</scroll-view>
		</view>
		<BottomNav v-if="!timelinePreview" :active="mainNavigation.tab" @change="selectMainTab" />
		<PhoneVerification />
		<TechSupportDialog :open="supportOpen" @close="supportOpen=false" />
		<HomeAnnouncement v-if="!timelinePreview" :active="mainPageActive&&mainNavigation.tab==='player'" :announcement="publicContent?.announcement"/>
	</view>
</template>
<script setup lang="ts">
	import { ref, watch } from 'vue'
	import { onLoad, onShareAppMessage, onShareTimeline, onShow, onHide, onUnload } from '@dcloudio/uni-app'
	import BottomNav from '../../components/BottomNav.vue'
	import PhoneVerification from '../../components/PhoneVerification.vue'
	import TechSupportFooter from '../../components/TechSupportFooter.vue'
	import TechSupportDialog from '../../components/TechSupportDialog.vue'
	import ContentTip from '../../components/ContentTip.vue'
	import HomeAnnouncement from '../../components/HomeAnnouncement.vue'
	import { publicContent,refreshPublicContent,resetPublicContent } from '../../lib/public-content'
	import PlayerView from './player/player.vue'
	import WorkerView from './worker/worker.vue'
	import OrdersView from './orders/orders.vue'
	import MeView from './me/me.vue'
	import { themeStyle, game, state, init, sessionRevision } from '../../lib/store'
	import { isTimelinePreview, enablePublicShareMenu } from '../../lib/sharing'
	import { createPublicShareHandlers } from '../../lib/use-public-share'
	import { readSafeArea } from '../../lib/safe-area'
	import { applyMainQuery, mainNavigation, selectMainTab } from '../../lib/navigation'
	import type { MainTab } from '../../lib/navigation'
	import { notify } from '../../lib/format'

	/** 主滚动容器只调用当前视图声明的读取动作，不参与业务请求和资金操作。 */
	interface MainViewActions {refreshKey?:number;refresh:()=>Promise<void>;loadMore?:()=>Promise<void>}
	const playerView=ref<MainViewActions|null>(null),workerView=ref<MainViewActions|null>(null),ordersView=ref<MainViewActions|null>(null),meView=ref<MainViewActions|null>(null)
	const refreshing=ref<Record<MainTab,boolean>>({player:false,worker:false,orders:false,me:false})
	const gestureLocks=new Map<MainTab,number>()
	let pageVisible=false,gestureSequence=0
	const mainPageActive=ref(false)
	/** 清理原生刷新显示和旧请求锁，迟到finally不能关闭下一轮新刷新。 */
	function clearGestures():void{gestureSequence++;gestureLocks.clear();for(const tab of ['player','worker','orders','me'] as const)refreshing.value[tab]=false}
	/** 在原生事件处串行化当前容器动作，隐藏、换号、切主题后只允许最新finally恢复状态。 */
	async function runGesture(tab:MainTab,kind:'refresh'|'more'):Promise<void>{
		if(!pageVisible||mainNavigation.tab!==tab||gestureLocks.has(tab))return
		const view={player:playerView.value,worker:workerView.value,orders:ordersView.value,me:meView.value}[tab],action=kind==='refresh'?view?.refresh:view?.loadMore
		if(!action)return
		const sequence=++gestureSequence;gestureLocks.set(tab,sequence);if(kind==='refresh')refreshing.value[tab]=true
		try{await action()}catch(error){if(pageVisible&&mainNavigation.tab===tab&&gestureLocks.get(tab)===sequence)notify(error)}finally{if(gestureLocks.get(tab)===sequence){gestureLocks.delete(tab);refreshing.value[tab]=false}}
	}
	watch([()=>mainNavigation.tab,()=>state.user?.id,sessionRevision,()=>state.gameId,()=>playerView.value?.refreshKey,()=>workerView.value?.refreshKey,()=>ordersView.value?.refreshKey],clearGestures,{flush:'sync'})
	onShow(()=>{enablePublicShareMenu();pageVisible=true;mainPageActive.value=true;void init(false,isTimelinePreview()).then(refreshPublicContent).catch(()=>{})})
	onHide(()=>{pageVisible=false;mainPageActive.value=false;clearGestures()})
	onUnload(()=>{pageVisible=false;mainPageActive.value=false;clearGestures()})
	watch(()=>state.gameId,()=>{resetPublicContent();if(pageVisible)void refreshPublicContent().catch(()=>{})})

	const timelinePreview = isTimelinePreview()
	const safe = readSafeArea()
	const safeStyle = { '--status-bar-height': safe.statusBarHeight + 'px', '--capsule-space': safe.capsuleSpace + 'px', '--capsule-right-inset': safe.capsuleRightInset + 'px' }
	const supportOpen = ref(false)
	const publicShareHandlers = createPublicShareHandlers(() => ({ title: '集结 · ' + game.value.name + '好队友，即刻集结', path: '/pages/main/index?tab=player&game_id=' + encodeURIComponent(state.gameId), image: game.value.cover_url }))
	onShareAppMessage(publicShareHandlers.friend)
	onShareTimeline(publicShareHandlers.timeline)
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
	.main-content-tip{margin:0 30.7692rpx 23.0769rpx}
</style>
