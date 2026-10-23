<!-- 文件作用：独立处理俱乐部审核与平台认证，并提供公开主页、成员和团队订单入口。 -->
<template>
  <AppShell title="俱乐部管理" back>
    <ManageList v-model:query="query" placeholder="搜索俱乐部名称、管理员" :loading="loading" :busy="busy" :error="error" :empty="!clubs.length" :has-more="hasMore" :count="clubs.length" @search="loadData()" @more="loadData(true)" @prepare="showPage">
      <template #filters><scroll-view scroll-x :show-scrollbar="false" class="filter-scroll"><view class="status-filters"><button v-for="item in filters" :key="item.value" :class="['status-filter', { selected: status === item.value }]" :disabled="busy" @click="chooseStatus(item.value)">{{ item.label }}</button></view></scroll-view></template>
      <view v-for="club in clubs" :key="club.id" class="club-row"><view class="club-summary"><view class="club-thumbnail"><image v-if="club.cover_url" class="club-image" :src="assetUrl(club.cover_url)" mode="aspectFill"/><Icon v-else name="club" :size="30"/></view><view class="club-copy"><view class="club-heading"><text class="club-name">{{ club.name }}</text><text class="club-label">{{ clubStatus(club.status) }}</text><text v-if="club.verified" class="club-label verified">已认证</text></view><text class="club-meta">管理员 {{ club.owner_name }} · {{ club.member_count || 0 }} 名成员</text><text class="club-description">{{ club.description }}</text></view></view><view class="club-actions"><button v-if="club.status === 'pending'" class="btn btn-secondary btn-small" :disabled="busy" @click="review(club, 'rejected', false)">拒绝</button><button v-if="club.status !== 'active'" class="btn btn-small" :disabled="busy" @click="review(club, 'active', true)">通过并认证</button><button v-else class="btn btn-secondary btn-small" :disabled="busy" @click="review(club, 'active', !club.verified)"><Icon name="shield" :size="14"/>{{ club.verified ? '取消认证' : '授予认证' }}</button></view><view class="club-links"><button class="text-btn club-link" @click="openClub(club.id)"><Icon name="home" :size="16"/>主页<Icon name="arrow" :size="12"/></button><button class="text-btn club-link" @click="openClub(club.id, 'members')"><Icon name="club" :size="16"/>成员<Icon name="arrow" :size="12"/></button><button class="text-btn club-link" @click="openClub(club.id, 'orders')"><Icon name="orders" :size="16"/>订单<Icon name="arrow" :size="12"/></button></view></view>
    </ManageList>
  </AppShell>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import Icon from '../../components/Icon.vue'
import { useManageResource } from '../../lib/manage-resource'
import { moderateResource } from '../../api/admin.api'
import { assetUrl, go, statusLabel } from '../../lib/format'
import type { Club } from '../../types/models'
const filters = [{ value: '', label: '全部' }, { value: 'pending', label: '待审核' }, { value: 'active', label: '已通过' }, { value: 'rejected', label: '已拒绝' }]
const status = ref('')
const { items: clubs, query, loading, busy, error, hasMore, loadData, showPage, mutate } = useManageResource('clubs', { params: () => ({ status: status.value || undefined }) })
/** 俱乐部审核显示通过状态，不沿用商品的已上架措辞。 */
function clubStatus(value: string): string { return filters.find(item => item.value === value)?.label || statusLabel(value) }
/** 服务端筛选完整俱乐部资源，不在单页记录内做假筛选。 */
function chooseStatus(value: string): void { if (busy.value || !filters.some(item => item.value === value) || value === status.value) return; status.value = value; clubs.value = []; hasMore.value = false; void loadData() }
/** 保留审核与认证两项真实字段，由服务端独立验证管理员权限。 */
async function review(club: Club, value: 'active' | 'rejected', verified: boolean): Promise<void> { await mutate(() => moderateResource('clubs', club.id, { status: value, verified })) }
/** 跳转已有俱乐部主页的安全页签，不复制成员或团队订单界面。 */
function openClub(id: string, tab: 'info' | 'members' | 'orders' = 'info'): void { go('/pages/community/clubs/detail?id=' + encodeURIComponent(id) + '&tab=' + tab) }
onLoad((options: Record<string, string | undefined> = {}) => { if (filters.some(item => item.value === options.status)) status.value = options.status || '' })
</script>
<style scoped>
.filter-scroll{width:100%;max-width:100%;margin-bottom:7.6923rpx;border-bottom:1px solid var(--line-subtle)}.status-filters{display:inline-flex;min-width:100%;gap:46.1538rpx;white-space:nowrap}.status-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:76.9231rpx;padding:0 3.8462rpx;border:0;border-bottom:3.8462rpx solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:25rpx}.status-filter.selected{color:var(--accent);border-bottom-color:var(--accent)}.club-row{display:flex;flex-direction:column;gap:23.0769rpx;min-width:0;padding:38.4615rpx 0 0;border-bottom:1px solid var(--line-subtle)}.club-summary{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0}.club-thumbnail{display:flex;align-items:center;justify-content:center;width:123.0769rpx;height:123.0769rpx;flex-shrink:0;border:1px solid var(--line);border-radius:9.6154rpx;overflow:hidden;color:var(--accent);background:var(--surface)}.club-image{display:block;width:100%;height:100%}.club-copy{display:flex;flex-direction:column;gap:11.5385rpx;flex:1;min-width:0}.club-heading{display:flex;align-items:center;flex-wrap:wrap;gap:11.5385rpx;min-width:0}.club-name{font-size:30.7692rpx;line-height:46.1538rpx;font-weight:650;overflow-wrap:anywhere}.club-label{font-size:19.2308rpx;line-height:30.7692rpx;padding:0 9.6154rpx;border:1px solid var(--line);border-radius:5.7692rpx;color:var(--muted)}.verified{color:var(--accent);background:#83e7b70d}.club-meta,.club-description{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.club-description{display:-webkit-box;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical}.club-actions{display:flex;align-items:center;justify-content:flex-end;flex-wrap:wrap;gap:15.3846rpx}.club-links{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));min-width:0;border-top:1px solid var(--line-subtle);padding:19.2308rpx 0;gap:15.3846rpx}.club-link{color:var(--muted);min-width:0;font-size:23.0769rpx;gap:11.5385rpx}@media(max-width:350px){.status-filters{gap:38.4615rpx}.club-summary{gap:19.2308rpx}.club-thumbnail{width:107.6923rpx;height:107.6923rpx}.club-link{gap:7.6923rpx}}
</style>
