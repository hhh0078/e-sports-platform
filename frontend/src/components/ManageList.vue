<!-- 文件作用：统一管理子页的权限、搜索、加载、错误、空态和分页，仅承载页面自己的业务内容。 -->
<template>
  <EmptyState v-if="state.ready && state.user?.role !== 'admin'" title="仅平台管理员可访问" detail="请使用平台管理员账号登录" icon="shield"><button class="text-btn" @click="go('/pages/account/login')">切换账号</button></EmptyState>
  <template v-else-if="state.user?.role === 'admin'">
    <slot name="before"/>
    <view class="manage-search"><Icon name="search" :size="18"/><input class="manage-search-input" :value="query" :placeholder="placeholder" maxlength="100" confirm-type="search" :disabled="busy" @input="changeQuery" @confirm="emit('search')"/><button class="text-btn manage-search-button" :disabled="loading || busy" @click="emit('search')">搜索</button></view>
    <slot name="filters"/>
    <view v-if="error" class="manage-error"><text class="manage-error-copy">{{ error }}</text><button class="text-btn" :disabled="loading || busy" @click="emit('search')">重试</button></view>
    <view v-if="loading && empty" class="loading-line">正在读取管理数据…</view>
    <slot/>
    <EmptyState v-if="!loading && empty && !error" title="没有匹配的数据" detail="可以更换关键词或筛选条件" icon="search"/>
    <view v-if="!empty" class="manage-pagination"><text class="manage-count">已展示 {{ count }} 条</text><button v-if="hasMore" class="text-btn" :disabled="loading || busy" @click="emit('more')">{{ loading ? '正在加载…' : '加载更多' }}</button><text v-else class="manage-count">已全部加载</text></view>
  </template>
  <view v-else class="identity-state"><template v-if="error"><text class="manage-error-copy">{{ error }}</text><button class="text-btn" @click="emit('prepare')">重新核验身份</button></template><text v-else class="identity-loading">正在核验管理员身份…</text></view>
</template>
<script setup lang="ts">
import EmptyState from './EmptyState.vue'
import Icon from './Icon.vue'
import { state } from '../lib/store'
import { go } from '../lib/format'
withDefaults(defineProps<{ query: string; placeholder?: string; loading: boolean; busy?: boolean; error: string; empty: boolean; hasMore: boolean; count: number }>(), { placeholder: '搜索管理数据', busy: false })
const emit = defineEmits<{ 'update:query': [value: string]; search: []; more: []; prepare: [] }>()
/** 原生输入只更新搜索草稿，确认后由具体页面重新读取服务端数据。 */
function changeQuery(event: Event): void { const detail = (event as Event & { detail?: { value?: unknown } }).detail; if (typeof detail?.value === 'string') emit('update:query', detail.value) }
</script>
<style scoped>
.manage-search{display:flex;align-items:center;gap:19.2308rpx;min-width:0;width:100%;min-height:44px;padding:0 23.0769rpx;margin:7.6923rpx 0 30.7692rpx;border-radius:15.3846rpx;background:#ffffff07;color:var(--muted)}.manage-search-input{flex:1;min-width:0;height:84.6154rpx;font-size:26.9231rpx}.manage-search-button{min-width:76.9231rpx;border-left:1px solid var(--line-subtle);border-radius:0;padding-left:23.0769rpx}.manage-error{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;padding:23.0769rpx 0;color:#e9b69d}.manage-error-copy{min-width:0;font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.manage-pagination{display:flex;align-items:center;justify-content:center;flex-wrap:wrap;gap:23.0769rpx;padding:46.1538rpx 0 15.3846rpx}.manage-count{font-size:23.0769rpx;color:var(--muted)}.identity-state{display:flex;flex-direction:column;align-items:center;gap:23.0769rpx;padding:76.9231rpx 30.7692rpx;min-width:0;text-align:center;color:#e9b69d}.identity-loading{font-size:25rpx;line-height:42.3077rpx;color:var(--muted)}
</style>
