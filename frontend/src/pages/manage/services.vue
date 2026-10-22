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
.filter-scroll{width:100%;max-width:100%;margin-bottom:4px;border-bottom:1px solid var(--line-subtle)}.status-filters{display:inline-flex;min-width:100%;gap:20px;white-space:nowrap}.status-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:40px;padding:0 2px;border:0;border-bottom:2px solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:13px}.status-filter.selected{color:var(--accent);border-bottom-color:var(--accent)}.service-row{display:flex;flex-direction:column;gap:12px;min-width:0;padding:20px 0;border-bottom:1px solid var(--line-subtle)}.service-summary{display:flex;align-items:flex-start;gap:12px;min-width:0}.service-image{display:block;flex-shrink:0;width:112px;height:63px;border-radius:5px}.service-copy{display:flex;flex-direction:column;gap:5px;min-width:0;flex:1}.service-heading{display:flex;align-items:flex-start;gap:8px;min-width:0;flex-wrap:wrap}.service-title{font-size:15px;line-height:23px;font-weight:650;overflow-wrap:anywhere}.service-status{max-width:100%;font-size:10px;line-height:16px;padding:1px 6px;color:var(--accent);border:1px solid var(--line);border-radius:4px}.service-meta{font-size:12px;line-height:20px;color:var(--muted);overflow-wrap:anywhere}.service-price{font-size:16px;font-weight:650;line-height:22px;color:var(--accent)}.service-unit{font-size:11px;font-weight:400;color:var(--muted)}.service-description{display:-webkit-box;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical;font-size:13px;line-height:22px;color:var(--muted);overflow-wrap:anywhere}.service-actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px}.service-detail{margin-right:auto;justify-content:flex-start;color:var(--muted)}@media(max-width:350px){.status-filters{gap:16px}.service-image{width:96px;height:54px}.service-summary{gap:10px}}
</style>
