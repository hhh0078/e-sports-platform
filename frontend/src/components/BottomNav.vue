<!-- 文件作用：仅在主页面挂载一次的四项底栏，直接发出内容切换事件。 -->
<template>
  <view class="bottom-nav"><view class="nav-inner">
    <button v-for="item in items" :key="item.key" :class="['nav-item', { selected: active === item.key }]" :aria-label="item.label" @click="emit('change', item.key)"><view class="nav-icon"><Icon :name="item.icon" :size="23" /></view><text>{{ item.label }}</text></button>
  </view></view>
</template>
<script setup lang="ts">
import Icon from './Icon.vue'
import type { MainTab } from '../lib/navigation'
defineProps<{ active: MainTab }>()
const emit = defineEmits<{ change: [tab: MainTab] }>()
const items: Array<{ key: MainTab; label: string; icon: string }> = [{ key: 'player', label: '玩家', icon: 'game' }, { key: 'worker', label: '打手', icon: 'work' }, { key: 'orders', label: '订单', icon: 'orders' }, { key: 'me', label: '我的', icon: 'user' }]
</script>
<style scoped>
.bottom-nav{flex-shrink:0;width:100%;padding-bottom:env(safe-area-inset-bottom);background:var(--bg);border-top:1px solid var(--line-subtle)}.nav-inner{display:flex;height:115.3846rpx;max-width:600px;margin:0 auto;padding:0 23.0769rpx}.nav-item{position:relative;flex:1;min-width:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:7.6923rpx;padding:7.6923rpx 0;background:transparent;color:var(--muted);font-size:21.1538rpx;border-radius:0;line-height:1.2}.nav-icon{display:flex;align-items:center;justify-content:center;width:53.8462rpx;height:50rpx}.selected{color:var(--accent);font-weight:700}
</style>
