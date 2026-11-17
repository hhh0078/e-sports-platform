<!-- 文件作用：orders独立主导航页面，页面实例与滚动由tabBar缓存，业务逻辑复用现有视图。 -->
<template><MainPage tab="orders" :view="content"><OrdersView ref="content" :active="true"/></MainPage></template>
<script setup lang="ts">
import {ref} from 'vue'
import {onLoad,onShow,onShareAppMessage,onShareTimeline} from '@dcloudio/uni-app'
import MainPage from '../../../components/MainPage.vue'
import OrdersView from './orders.vue'
import {applyTabQuery} from '../../../lib/navigation'
import {prepareEntryGame,game,state} from '../../../lib/store'
import {enablePublicShareMenu} from '../../../lib/sharing'
import {createPublicShareHandlers} from '../../../lib/use-public-share'
const content=ref<{refreshKey?:number;refresh:()=>Promise<void>;loadMore?:()=>Promise<void>}|null>(null)
const publicShareHandlers=createPublicShareHandlers(()=>({title:'开机电竞 · '+game.value.name,path:'/pages/main/index?game_id='+encodeURIComponent(state.gameId),image:game.value.cover_url}))
onShareAppMessage(publicShareHandlers.friend);onShareTimeline(publicShareHandlers.timeline)
onShow(()=>{enablePublicShareMenu()})
onLoad(query=>{applyTabQuery('orders',query);prepareEntryGame(query?.game_id||'');})
</script>
