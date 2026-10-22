<!-- 文件作用：仅展示当前账号本机保存的消息全文，已读落盘成功才确认删除远端，正文不执行HTML。 -->
<template>
  <AppShell title="消息详情" back>
    <view v-if="error" class="message-error"><text class="message-error-copy">{{ error }}</text><button class="text-btn" :disabled="loading" @click="showPage">重试</button></view>
    <template v-if="selected && cacheOwner === state.user?.id"><view class="message-article"><text class="message-title">{{ selected.title }}</text><view class="message-meta"><text>{{ time(selected.created_at) }}</text><text>平台通知</text></view><text class="message-body" selectable>{{ selected.body }}</text></view><button v-if="selected.ack_pending" class="text-btn pending-retry" :disabled="loading" @click="retryPending">{{ loading ? '正在同步已读状态…' : '重新同步已读状态' }}</button></template>
    <view v-else-if="loading" class="loading-line">正在读取消息…</view>
    <EmptyState v-else-if="!state.user && !error" title="暂无消息" detail="请登录后查看" icon="bell"><button class="text-btn" @click="go('/pages/account/login?next=' + encodeURIComponent('/pages/account/messages/detail?id=' + id))">登录</button></EmptyState>
    <EmptyState v-else-if="!error" title="该消息暂不可用" detail="请返回消息中心查看" icon="bell"><button class="text-btn" @click="go('/pages/account/messages/index')">返回消息中心</button></EmptyState>
  </AppShell>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import { state } from '../../../lib/store'
import { go, time } from '../../../lib/format'
import { useMessages } from '../../../lib/use-messages'
const id = ref('')
const { selected, cacheOwner, loading, error, showPage, retryPending } = useMessages({ detailId: () => id.value })
onLoad((options: Record<string, string | undefined> = {}) => { id.value = typeof options.id === 'string' ? options.id : '' })
</script>
<style scoped>
.message-article{min-width:0;padding-top:8px}.message-title{display:block;font-size:22px;line-height:32px;font-weight:700;overflow-wrap:anywhere}.message-meta{display:flex;align-items:center;flex-wrap:wrap;gap:8px 12px;padding:8px 0 20px;border-bottom:1px solid var(--line-subtle);font-size:13px;line-height:22px;color:var(--muted)}.message-body{display:block;padding-top:24px;font-size:16px;line-height:29px;white-space:pre-wrap;overflow-wrap:anywhere;color:var(--text)}.message-error{display:flex;align-items:flex-start;justify-content:space-between;gap:12px;padding:12px 0 20px;color:#e9b69d}.message-error-copy{flex:1;min-width:0;font-size:13px;line-height:22px;overflow-wrap:anywhere}.pending-retry{margin-top:24px;font-size:13px}
</style>
