<!-- 文件作用：player独立主导航页面，页面实例与滚动由tabBar缓存，业务逻辑复用现有视图。 -->
<template><MainPage tab="player" :view="content"><PlayerView ref="content" :active="true"/></MainPage></template>
<script setup lang="ts">
import {ref} from 'vue'
import {onLoad,onReady,onShow,onShareAppMessage,onShareTimeline} from '@dcloudio/uni-app'
import MainPage from '../../components/MainPage.vue'
import PlayerView from './player/player.vue'
import {applyTabQuery,navigate,normalizePageUrl} from '../../lib/navigation'
import {prepareEntryGame,game,state} from '../../lib/store'
import {isTimelinePreview,enablePublicShareMenu} from '../../lib/sharing'
import {createPublicShareHandlers} from '../../lib/use-public-share'
const content=ref<{refreshKey?:number;refresh:()=>Promise<void>;loadMore?:()=>Promise<void>}|null>(null)
let legacyTarget=''
const publicShareHandlers=createPublicShareHandlers(()=>({title:'开机电竞 · '+game.value.name,path:'/pages/main/index?game_id='+encodeURIComponent(state.gameId),image:game.value.cover_url}))
onShareAppMessage(publicShareHandlers.friend);onShareTimeline(publicShareHandlers.timeline)
onShow(()=>{enablePublicShareMenu()})
onLoad(query=>{applyTabQuery('player',query);prepareEntryGame(query?.game_id||'');if(!isTimelinePreview()&&query?.tab&&query.tab!=='player'){legacyTarget=normalizePageUrl('/pages/main/index?'+Object.entries(query).map(([key,value])=>encodeURIComponent(key)+'='+encodeURIComponent(String(value))).join('&'))}})
onReady(()=>{if(legacyTarget){navigate(legacyTarget);legacyTarget=''}})
</script>
