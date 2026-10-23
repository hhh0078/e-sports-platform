<!-- 文件作用：复用官方加载更多状态行；下一页由触底触发，只有失败时保留轻量重试。 -->
<template>
  <view class="list-load-state">
    <button v-if="error" class="list-load-retry" :disabled="loading" @click="emit('retry')">{{ error }} · 轻点重试</button>
    <uni-load-more v-else :status="loading ? 'loading' : hasMore ? 'more' : 'noMore'" :content-text="contentText" color="var(--muted)" :icon-size="loadingIconSize" :show-icon="loading" />
  </view>
</template>
<script setup lang="ts">
withDefaults(defineProps<{ loading: boolean; hasMore: boolean; error?: string }>(), { error: '' })
const emit = defineEmits<{ retry: [] }>()
const contentText = { contentdown: '继续上拉加载', contentrefresh: '正在加载…', contentnomore: '已经到底了' }
const loadingIconSize = uni.upx2px(36)
</script>
<style scoped>
.list-load-state { width: 100%; min-width: 0; padding: 8rpx 0 16rpx; color: var(--muted) }
.list-load-state :deep(.uni-load-more) { height: 76rpx; min-height: 60rpx }
.list-load-state :deep(.uni-load-more__text) { margin-left: 12rpx; font-size: 23rpx; line-height: 1.5 }
.list-load-retry { display: flex; align-items: center; justify-content: center; width: 100%; min-height: 44px; height: auto; padding: 12rpx 24rpx; white-space: normal; overflow-wrap: anywhere; color: var(--muted); background: transparent; border: 0; font-size: 23rpx; line-height: 1.6 }
.list-load-retry::after { border: 0 }
</style>
