<!-- 文件作用：只读查询真实管理员操作记录，提供服务端搜索分页而不允许编辑或删除审计。 -->
<template>
  <AppShell title="操作审计" back>
    <ManageList v-model:query="query" placeholder="搜索操作、用户或对象" :loading="loading" :error="error" :empty="!audits.length" :has-more="hasMore" :count="audits.length" @search="loadData()" @more="loadData(true)" @retry="retryFailed" @prepare="showPage">
      <view v-for="entry in audits" :key="entry.id" class="audit-row"><view class="audit-heading"><text class="audit-action">{{ entry.action }}</text><text class="audit-time">{{ time(entry.created_at) }}</text></view><view class="audit-field"><text class="audit-label">操作人</text><text class="audit-value">{{ entry.actor_id }}</text></view><view class="audit-field"><text class="audit-label">目标</text><text class="audit-value">{{ entry.target_id }}</text></view><text v-if="entry.note" class="audit-note">{{ noteText(entry.note) }}</text></view>
    </ManageList>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import { useManageResource } from '../../lib/manage-resource'
import { time } from '../../lib/format'
const { items: audits, query, loading, error, hasMore, loadData, retryFailed, showPage } = useManageResource('audit')
/** 审计说明作为纯文本显示，不解析可执行HTML或把说明编造成操作记录。 */
function noteText(value: unknown): string { return typeof value === 'string' ? value : JSON.stringify(value) || '' }
</script>
<style scoped>
.audit-row{display:flex;flex-direction:column;gap:15.3846rpx;min-width:0;padding:46.1538rpx 0;border-bottom:1px solid var(--line-subtle)}.audit-heading{display:flex;align-items:flex-start;justify-content:space-between;flex-wrap:wrap;gap:15.3846rpx 30.7692rpx;min-width:0}.audit-action{min-width:0;font-size:30.7692rpx;font-weight:650;line-height:46.1538rpx;overflow-wrap:anywhere}.audit-time{flex-shrink:0;font-size:23.0769rpx;line-height:46.1538rpx;color:var(--muted)}.audit-field{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0;font-size:26.9231rpx;line-height:42.3077rpx}.audit-label{width:84.6154rpx;flex-shrink:0;color:var(--muted)}.audit-value{flex:1;min-width:0;overflow-wrap:anywhere}.audit-note{display:block;font-size:26.9231rpx;line-height:46.1538rpx;overflow-wrap:anywhere;color:var(--muted)}
</style>
