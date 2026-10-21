<!-- 文件作用：单行游戏目录、官方选择弹层与本地游戏偏好，滚动区域不挤压“全部”入口。 -->
<template>
  <view class="game-picker">
    <view class="game-picker-row"><scroll-view class="games-scroll" scroll-x :show-scrollbar="false" :scroll-into-view="'game-option-' + state.gameId" scroll-with-animation><view class="games-list"><button v-for="item in state.games" :id="'game-option-' + item.id" :key="item.id" :class="['game-choice', { chosen: state.gameId === item.id }]" @click="selectGame(item.id)"><view class="game-image"><image class="game-icon" :src="assetUrl(item.icon_url || item.cover_url)" mode="aspectFill" /></view><text class="game-label">{{ item.short_name || item.name }}</text></button></view></scroll-view><button class="catalog-trigger" aria-label="查看全部游戏" @click="openCatalog"><text>全部</text><Icon name="bottom" :size="13" /></button></view>
    <UniPopup ref="popup" type="bottom" background-color="transparent"><view class="catalog-sheet"><view class="catalog-heading"><text>全部游戏</text><button class="icon-btn" aria-label="关闭游戏目录" @click="popup?.close()"><Icon name="close" :size="22" /></button></view><view class="catalog-search"><Icon name="search" :size="17" /><input class="catalog-input" v-model="search" placeholder="搜索游戏名称" /></view><scroll-view scroll-y class="catalog-scroll"><view class="catalog-grid"><button v-for="item in filteredGames" :key="item.id" :class="['catalog-game', { selected: state.gameId === item.id }]" @click="chooseFromCatalog(item.id)"><image class="catalog-game-icon" :src="assetUrl(item.icon_url || item.cover_url)" mode="aspectFill" /><text>{{ item.name }}</text></button></view><text v-if="!filteredGames.length" class="loading-line">没有找到这个游戏</text></scroll-view></view></UniPopup>
  </view>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import { state, selectGame } from '../lib/store'
import { assetUrl } from '../lib/format'
import Icon from './Icon.vue'
defineProps<{ catalog?: boolean }>()
const popup = ref<{ open: (type?: string) => void; close: () => void } | null>(null), search = ref('')
const filteredGames = computed(() => state.games.filter(item => (item.name + item.short_name + item.id).toLowerCase().includes(search.value.trim().toLowerCase())))
/** 打开官方目录弹层并重置查询。 */
function openCatalog(): void { search.value = ''; popup.value?.open('bottom') }
/** 选择游戏后回到原业务视图。 */
function chooseFromCatalog(id: string): void { selectGame(id); popup.value?.close() }
</script>
<style scoped>
.game-picker{width:100%;min-width:0;max-width:100%}.game-picker-row{display:flex;align-items:stretch;gap:4px;padding:10px 6px;min-width:0}.games-scroll{flex:1;width:0;min-width:0;white-space:nowrap}.games-list{display:inline-flex;gap:12px;padding:0 4px;min-width:100%}.game-choice{display:flex;flex-direction:column;align-items:center;gap:5px;width:48px;flex-shrink:0;min-width:0;margin:0;padding:0;background:transparent;border-radius:0;font-size:10px;line-height:14px;color:#b7c6bd}.game-label{display:block;width:100%;text-align:center;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.game-image{display:flex;align-items:center;justify-content:center;width:40px;height:40px;flex-shrink:0;overflow:hidden;border-radius:9px;border:1px solid var(--line)}.game-icon{display:block;width:100%;height:100%}.chosen{color:var(--accent);font-weight:650}.chosen .game-image{border:2px solid var(--accent)}.catalog-trigger{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:6px;flex-shrink:0;width:38px;padding:0;border-left:1px solid var(--line-subtle);border-radius:0;background:transparent;color:#b6c8bd;font-size:11px;line-height:16px}.catalog-sheet{width:100vw;max-width:600px;margin:0 auto;background:var(--surface);border:1px solid var(--line);border-radius:18px 18px 0 0;padding:12px 16px calc(20px + env(safe-area-inset-bottom));color:var(--text)}.catalog-heading{display:flex;align-items:center;justify-content:space-between;font-size:19px;font-weight:700}.catalog-search{display:flex;align-items:center;gap:8px;min-height:44px;padding:8px 12px;margin:12px 0;border:1px solid var(--line);border-radius:10px}.catalog-input{flex:1;min-width:0}.catalog-scroll{height:360px;max-height:55vh}.catalog-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px}.catalog-game{display:flex;flex-direction:column;align-items:center;gap:8px;padding:10px 2px;background:transparent;font-size:11px;border:1px solid transparent;border-radius:10px}.catalog-game-icon{width:54px;height:54px;border-radius:10px}.catalog-game.selected{color:var(--accent);border-color:var(--accent)}
@media(max-width:350px){.games-list{gap:9px}.game-image{width:36px;height:36px}.game-choice{width:44px;font-size:9px}.catalog-trigger{width:34px}}
</style>
