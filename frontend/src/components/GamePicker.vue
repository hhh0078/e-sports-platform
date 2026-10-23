<!-- 文件作用：保留首页单行游戏目录，显式选择后更新主游戏偏好，完整目录复用共享选择器。 -->
<template>
  <view class="game-picker">
    <view class="game-picker-row"><scroll-view class="games-scroll" scroll-x :show-scrollbar="false" :scroll-into-view="'game-option-' + state.gameId" scroll-with-animation><view class="games-list"><button v-for="item in state.games" :id="'game-option-' + item.id" :key="item.id" :class="['game-choice', { chosen: state.gameId === item.id }]" :disabled="!props.active" @click="chooseGame(item.id)"><view class="game-image"><image class="game-icon" :src="assetUrl(item.icon_url || item.cover_url)" mode="aspectFill" /></view><text class="game-label">{{ item.short_name || item.name }}</text></button></view></scroll-view><button class="catalog-trigger" aria-label="查看全部游戏" :disabled="!props.active" @click="openCatalog"><text>全部</text><Icon name="bottom" :size="13" /></button></view>
    <GameSelector ref="selector" :field="false" :active="props.active" :value="state.gameId" title="全部游戏" @change="chooseGame"/>
  </view>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { state, selectGame } from '../lib/store'
import { assetUrl } from '../lib/format'
import Icon from './Icon.vue'
import GameSelector from './GameSelector.vue'
const props=withDefaults(defineProps<{active?:boolean}>(),{active:true})
const selector=ref<InstanceType<typeof GameSelector>|null>(null)
/** 主视图隐藏时拒绝残留点击，仅显式选择更新主题与本地偏好。 */
function chooseGame(id:string):void{if(props.active&&state.games.some(item=>item.id===id&&item.enabled!==false))selectGame(id)}
/** 打开共享目录，选择器自身不会修改全局偏好。 */
function openCatalog():void{if(props.active)selector.value?.open()}
</script>
<style scoped>
.game-picker{width:100%;min-width:0;max-width:100%}.game-picker-row{display:flex;align-items:stretch;gap:7.6923rpx;padding:19.2308rpx 11.5385rpx;min-width:0}.games-scroll{flex:1;width:0;min-width:0;white-space:nowrap}.games-list{display:inline-flex;gap:23.0769rpx;padding:0 7.6923rpx;min-width:100%}.game-choice{display:flex;flex-direction:column;align-items:center;gap:9.6154rpx;width:92.3077rpx;flex-shrink:0;min-width:0;margin:0;padding:0;background:transparent;border-radius:0;font-size:19.2308rpx;line-height:26.9231rpx;color:#b7c6bd}.game-label{display:block;width:100%;text-align:center;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.game-image{display:flex;align-items:center;justify-content:center;width:76.9231rpx;height:76.9231rpx;flex-shrink:0;overflow:hidden;border-radius:17.3077rpx;border:1px solid var(--line)}.game-icon{display:block;width:100%;height:100%}.chosen{color:var(--accent);font-weight:650}.chosen .game-image{border:3.8462rpx solid var(--accent)}.catalog-trigger{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:11.5385rpx;flex-shrink:0;width:73.0769rpx;padding:0;border-left:1px solid var(--line-subtle);border-radius:0;background:transparent;color:#b6c8bd;font-size:21.1538rpx;line-height:30.7692rpx}
@media(max-width:350px){.games-list{gap:17.3077rpx}.game-image{width:69.2308rpx;height:69.2308rpx}.game-choice{width:84.6154rpx;font-size:17.3077rpx}.catalog-trigger{width:65.3846rpx}}
</style>
