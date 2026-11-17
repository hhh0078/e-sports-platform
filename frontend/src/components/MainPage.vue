<!-- 文件作用：四个独立主页面共用外观、单个滚动容器和资金身份弹层；页面内容由各路由独立持有。 -->
<template>
 <view class="main-shell" :style="[themeStyle,safeStyle]">
  <view class="main-viewport"><scroll-view scroll-y :show-scrollbar="false" class="main-scroll" :class="{'main-scroll-safe':tab==='worker'||tab==='orders'}" :refresher-enabled="true" :refresher-triggered="refreshing[tab]" refresher-background="transparent" refresher-default-style="white" @refresherrefresh="runGesture(tab,'refresh')" @scrolltolower="runGesture(tab,'more')">
   <ContentTip v-if="tab==='worker'||tab==='orders'" class="main-content-tip" :tip="publicContent?.page_tips[tab]"/>
   <slot/><TechSupportFooter @open="supportOpen=true"/>
  </scroll-view></view>
  <BottomNav v-if="!timelinePreview" :active="tab" @change="select"/>
  <PhoneVerification/><IdentityVerification :remind="!timelinePreview"/>
  <TechSupportDialog :open="supportOpen" @close="supportOpen=false"/>
  <HomeAnnouncement v-if="!timelinePreview&&tab==='player'" :active="mainPageActive" :announcement="publicContent?.announcement"/>
 </view>
</template>
<script setup lang="ts">
	import { ref, watch } from 'vue'
	import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
	import BottomNav from './BottomNav.vue'
	import PhoneVerification from './PhoneVerification.vue'
	import IdentityVerification from './IdentityVerification.vue'
	import TechSupportFooter from './TechSupportFooter.vue'
	import TechSupportDialog from './TechSupportDialog.vue'
	import ContentTip from './ContentTip.vue'
	import HomeAnnouncement from './HomeAnnouncement.vue'
	import { publicContent,refreshPublicContent,resetPublicContent } from '../lib/public-content'
	import { themeStyle, state, init, sessionRevision } from '../lib/store'
	import { isTimelinePreview } from '../lib/sharing'
	import { readSafeArea } from '../lib/safe-area'
	import { mainNavigation, navigate, MAIN_PAGES } from '../lib/navigation'
	import type { MainTab } from '../lib/navigation'
	import { notify } from '../lib/format'

	/** 主滚动容器只调用当前视图声明的读取动作，不参与业务请求和资金操作。 */
	interface MainViewActions {refreshKey?:number;refresh:()=>Promise<void>;loadMore?:()=>Promise<void>}
	const props=defineProps<{tab:MainTab;view:MainViewActions|null}>()
	const refreshing=ref<Record<MainTab,boolean>>({player:false,worker:false,orders:false,me:false})
	const gestureLocks=new Map<MainTab,number>()
	let pageVisible=false,gestureSequence=0
	const mainPageActive=ref(false)
	/** 清理原生刷新显示和旧请求锁，迟到finally不能关闭下一轮新刷新。 */
	function clearGestures():void{gestureSequence++;gestureLocks.clear();for(const tab of ['player','worker','orders','me'] as const)refreshing.value[tab]=false}
	/** 在原生事件处串行化当前容器动作，隐藏、换号、切主题后只允许最新finally恢复状态。 */
	async function runGesture(tab:MainTab,kind:'refresh'|'more'):Promise<void>{
		if(!pageVisible||mainNavigation.tab!==tab||gestureLocks.has(tab))return
		const view=props.view,action=kind==='refresh'?view?.refresh:view?.loadMore
		if(!action)return
		const sequence=++gestureSequence;gestureLocks.set(tab,sequence);if(kind==='refresh')refreshing.value[tab]=true
		try{await action()}catch(error){if(pageVisible&&mainNavigation.tab===tab&&gestureLocks.get(tab)===sequence)notify(error)}finally{if(gestureLocks.get(tab)===sequence){gestureLocks.delete(tab);refreshing.value[tab]=false}}
	}
	watch([()=>mainNavigation.tab,()=>state.user?.id,sessionRevision,()=>state.gameId,()=>props.view?.refreshKey],clearGestures,{flush:'sync'})
	onShow(()=>{uni.hideTabBar({animation:false,fail:()=>{}});mainNavigation.tab=props.tab;pageVisible=true;mainPageActive.value=true;void refreshPublicContent().catch(()=>{});void init(false,isTimelinePreview()).catch(()=>{})})
	onHide(()=>{pageVisible=false;mainPageActive.value=false;clearGestures()})
	onUnload(()=>{pageVisible=false;mainPageActive.value=false;clearGestures()})
	watch(()=>state.gameId,()=>{if(!state.ready)return;resetPublicContent();if(pageVisible)void refreshPublicContent().catch(()=>{})},{flush:'sync'})

	const timelinePreview = isTimelinePreview()
	const safe = readSafeArea()
	const safeStyle = { '--status-bar-height': safe.statusBarHeight + 'px', '--capsule-space': safe.capsuleSpace + 'px', '--capsule-right-inset': safe.capsuleRightInset + 'px' }
	const supportOpen = ref(false)
	/** 切换真实主页面，参数由共享导航交接，不能重启应用。 */
	function select(tab:MainTab):void{navigate(MAIN_PAGES[tab])}
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

	/* 状态栏避让属于各自固定滚动容器，切标签不能改变父视口高度并截断隐藏列表位置。 */
	.main-scroll-safe {
		top: var(--status-bar-height, 0px);
		height: calc(100% - var(--status-bar-height, 0px))
	}
	.main-content-tip{margin:0 30.7692rpx 23.0769rpx}
</style>
