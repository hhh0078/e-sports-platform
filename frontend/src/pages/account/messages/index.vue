<!-- 文件作用：按先行设计显示当前账号本机消息标题和未读点，完整收取分页但不删除未读消息。 -->
<template>
  <AppShell title="消息中心" back>
    <view class="message-toolbar"><button class="text-btn subscription-settings-entry" @click="go('/pages/account/subscriptions')"><Icon name="bell" :size="16"/>微信提醒设置</button><button class="text-btn refresh-messages" :disabled="loading" @click="showPage"><Icon name="refresh" :size="16"/>{{ loading ? '同步中…' : '刷新' }}</button></view>
    <view v-if="error" class="message-error"><text class="message-error-copy">{{ error }}</text><button class="text-btn" :disabled="loading" @click="showPage">重试</button></view>
    <view v-if="pendingCount" class="pending-notice"><text class="pending-copy">{{ pendingCount }} 条已读状态待同步</text><button class="text-btn" :disabled="loading" @click="retryPending">重新同步</button></view>
    <view v-if="loading && !items.length" class="loading-line">正在读取消息…</view>
    <EmptyState v-else-if="!state.user && !error" title="暂无消息" detail="请登录后查看" icon="bell"><button class="text-btn" @click="go('/pages/account/login?next=' + encodeURIComponent('/pages/account/messages/index'))">登录</button></EmptyState>
    <EmptyState v-else-if="!items.length && !error" title="暂无消息" detail="平台通知和审核结果会显示在这里" icon="bell"/>
    <view v-for="item in items" :key="item.id" :class="['message-row', { 'message-read': item.read }]" @click="openMessage(item.id)"><view class="message-mark"><view v-if="!item.read" class="unread-dot"/></view><text class="message-title">{{ item.title }}</text><Icon name="arrow" :size="15"/></view>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import { state } from '../../../lib/store'
import { go } from '../../../lib/format'
import { useMessages } from '../../../lib/use-messages'
const { items, loading, error, pendingCount, showPage, retryPending } = useMessages()
/** 进入已有本机全文详情，由详情持久标记已读后再确认远端删除。 */
function openMessage(id: string): void { go('/pages/account/messages/detail?id=' + encodeURIComponent(id)) }
</script>
<style scoped>
.message-toolbar{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-height:69.2308rpx;padding-bottom:23.0769rpx;border-bottom:1px solid var(--line-subtle)}.subscription-settings-entry{font-size:25rpx;min-width:0}.refresh-messages{color:var(--muted);font-size:25rpx}.message-row{display:flex;align-items:center;gap:23.0769rpx;min-width:0;min-height:130.7692rpx;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle);color:var(--text)}.message-mark{display:flex;align-items:center;justify-content:center;width:30.7692rpx;height:38.4615rpx;flex-shrink:0}.unread-dot{width:13.4615rpx;height:13.4615rpx;border-radius:50%;background:var(--accent)}.message-title{display:-webkit-box;flex:1;min-width:0;font-size:30.7692rpx;line-height:50rpx;font-weight:500;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow-wrap:anywhere}.message-read{color:var(--muted)}.message-error{display:flex;align-items:flex-start;justify-content:space-between;gap:23.0769rpx;padding:30.7692rpx 0;color:#e9b69d}.message-error-copy{min-width:0;flex:1;font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.pending-notice{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;padding:23.0769rpx 0;border-bottom:1px solid var(--line-subtle)}.pending-copy{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}
</style>
