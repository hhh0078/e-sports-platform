<!-- 文件作用：所有页面共用的可横向滚动游戏切换器。 -->
<template><scroll-view class="games-scroll" scroll-x :show-scrollbar="false" :scroll-into-view="'game-' + state.gameId"><view class="games-list"><view v-for="item in state.games" :key="item.id" :id="'game-' + item.id" class="game-choice" :class="{chosen: state.gameId === item.id}" @click="selectGame(item.id)"><view class="game-image"><image class="game-cover" :src="assetUrl(item.cover_url || item.covers?.[0])" mode="aspectFill"/><view v-if="state.gameId === item.id" class="game-check"><Icon name="check" :size="10" color="#07170d" /></view></view><text>{{ item.short_name || item.name }}</text></view></view></scroll-view></template>
<script setup>
import { state, selectGame } from '../lib/store'
import { assetUrl } from '../lib/format'
import Icon from './Icon.vue'
</script>
<style scoped>.games-scroll{width:100%;white-space:nowrap;margin:16px 0}.games-list{display:flex;gap:17px;padding:3px 1px 6px}.game-choice{flex-shrink:0;text-align:center;width:53px;color:#89978f;font-size:10px;cursor:pointer}.game-image{width:53px;height:53px;border-radius:14px;overflow:hidden;border:1px solid #3b433d;position:relative;margin-bottom:7px;opacity:.65}.game-cover{width:100%;height:100%;filter:saturate(.6)}.game-choice.chosen{color:var(--accent);font-weight:700}.chosen .game-image{opacity:1;border:2px solid var(--accent)}.chosen .game-cover{filter:none}.game-check{position:absolute;right:2px;bottom:2px;width:13px;height:13px;background:var(--accent);border-radius:50%;display:flex;align-items:center;justify-content:center}</style>
