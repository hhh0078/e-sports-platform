<!-- 文件作用：独立审核服务，按服务端状态搜索分页，保留通过、拒绝、下架及详情入口。 -->
<template>
  <AppShell title="服务审核" back>
    <ManageList v-model:query="query" placeholder="搜索服务标题、发布者" :loading="loading" :busy="busy" :error="error" :empty="!services.length" :has-more="hasMore" :count="services.length" @search="loadData()" @more="loadData(true)" @prepare="showPage">
      <template #filters><scroll-view scroll-x :show-scrollbar="false" class="filter-scroll"><view class="status-filters"><button v-for="item in filters" :key="item.value" :class="['status-filter', { selected: status === item.value }]" :disabled="busy" @click="chooseStatus(item.value)">{{ item.label }}</button></view></scroll-view></template>
      <view v-for="service in services" :key="service.id" class="service-row"><view class="service-summary"><image class="service-image" :src="assetUrl(service.cover_url)" mode="aspectFill"/><view class="service-copy"><view class="service-heading"><text class="service-title">{{ service.title }}</text><text class="service-status">{{ serviceStatus(service.status) }}</text></view><text class="service-meta">{{ service.provider_name }} · {{ service.category }}</text><text class="service-price">¥{{ money(service.price_cents) }}<text class="service-unit"> / {{ service.unit }}</text></text></view></view><text class="service-description">{{ service.description }}</text><view class="service-actions"><button class="text-btn service-detail" @click="go('/pages/services/detail?id=' + encodeURIComponent(service.id))">查看详情<Icon name="arrow" :size="13"/></button><button v-if="service.status === 'pending'" class="btn btn-secondary btn-small" :disabled="busy" @click="review(service.id, 'rejected')">拒绝</button><button v-if="service.status !== 'active'" class="btn btn-small" :disabled="busy" @click="review(service.id, 'active')"><Icon name="check" :size="14"/>通过</button><button v-else class="btn btn-secondary btn-small" :disabled="busy" @click="review(service.id, 'paused')">暂停</button></view></view>
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
import { go, assetUrl, money, statusLabel } from '../../lib/format'
const filters = [{ value: '', label: '全部' }, { value: 'pending', label: '待审核' }, { value: 'active', label: '已通过' }, { value: 'paused', label: '已暂停' }, { value: 'rejected', label: '已拒绝' }]
const status = ref('')
const { items: services, query, loading, busy, error, hasMore, loadData, showPage, mutate } = useManageResource('services', { params: () => ({ status: status.value || undefined }) })
/** 管理审核状态与筛选文案一致，未知状态仍保留共享可辨识文案。 */
function serviceStatus(value: string): string { return filters.find(item => item.value === value)?.label || statusLabel(value) }
/** 切换服务端状态筛选，重新从第一页读取，禁止只过滤已加载记录。 */
function chooseStatus(value: string): void { if (busy.value || !filters.some(item => item.value === value) || value === status.value) return; status.value = value; services.value = []; hasMore.value = false; void loadData() }
/** 通过具名审核接口更新服务状态，并刷新当前筛选下的真实结果。 */
async function review(id: string, value: 'active' | 'rejected' | 'paused'): Promise<void> { await mutate(() => moderateResource('services', id, { status: value })) }
onLoad((options: Record<string, string | undefined> = {}) => { if (filters.some(item => item.value === options.status)) status.value = options.status || '' })
</script>
<style scoped>
.filter-scroll{width:100%;max-width:100%;margin-bottom:7.6923rpx;border-bottom:1px solid var(--line-subtle)}.status-filters{display:inline-flex;min-width:100%;gap:38.4615rpx;white-space:nowrap}.status-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:76.9231rpx;padding:0 3.8462rpx;border:0;border-bottom:3.8462rpx solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:25rpx}.status-filter.selected{color:var(--accent);border-bottom-color:var(--accent)}.service-row{display:flex;flex-direction:column;gap:23.0769rpx;min-width:0;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle)}.service-summary{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0}.service-image{display:block;flex-shrink:0;width:215.3846rpx;height:121.1538rpx;border-radius:9.6154rpx}.service-copy{display:flex;flex-direction:column;gap:9.6154rpx;min-width:0;flex:1}.service-heading{display:flex;align-items:flex-start;gap:15.3846rpx;min-width:0;flex-wrap:wrap}.service-title{font-size:28.8462rpx;line-height:44.2308rpx;font-weight:650;overflow-wrap:anywhere}.service-status{max-width:100%;font-size:19.2308rpx;line-height:30.7692rpx;padding:1.9231rpx 11.5385rpx;color:var(--accent);border:1px solid var(--line);border-radius:7.6923rpx}.service-meta{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.service-price{font-size:30.7692rpx;font-weight:650;line-height:42.3077rpx;color:var(--accent)}.service-unit{font-size:21.1538rpx;font-weight:400;color:var(--muted)}.service-description{display:-webkit-box;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical;font-size:25rpx;line-height:42.3077rpx;color:var(--muted);overflow-wrap:anywhere}.service-actions{display:flex;align-items:center;flex-wrap:wrap;gap:15.3846rpx}.service-detail{margin-right:auto;justify-content:flex-start;color:var(--muted)}@media(max-width:350px){.status-filters{gap:30.7692rpx}.service-image{width:184.6154rpx;height:103.8462rpx}.service-summary{gap:19.2308rpx}}
</style>
