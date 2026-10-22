<!-- 文件作用：按工作台设计图展示真实统计、统一服务卡片、接单大厅及挂靠选择。 -->
<template>
  <view class="worker-content">
    <view class="worker-identity"><view class="avatar"><image class="worker-avatar-image" v-if="state.user?.avatar" :src="assetUrl(state.user.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="22"/></view><text class="worker-name">{{ state.user?.nickname || '登录后查看' }}</text><text v-if="state.user" class="provider-status">● 接单中</text><button class="worker-settings" aria-label="个人设置" @click="go('/pages/account/profile')"><Icon name="settings" :size="18"/></button></view>
    <view class="worker-stats"><view class="worker-stat"><text class="worker-stat-value">{{ statValue('provider_working_count') }}</text><text class="worker-stat-label">进行中</text></view><view class="worker-stat"><text class="worker-stat-value">{{ statValue('provider_completed_count') }}</text><text class="worker-stat-label">已完成</text></view><view class="worker-stat"><text class="worker-stat-value">{{ !state.user ? '¥0' : stats.earnings_cents === undefined ? '—' : '¥'+money(stats.earnings_cents) }}</text><text class="worker-stat-label">已完成收入</text></view></view>
    <view class="workbench-intro"><view class="intro-copy"><view class="intro-title"><text class="intro-accent">把擅长</text>，变成收入</view><text class="intro-detail">在集结，让游戏技能被看见</text><view class="intro-actions"><button class="publish-entry" @click="openPublish"><Icon name="plus" :size="17"/>发布服务</button><button class="orders-entry" @click="go('/pages/main/index?tab=orders&role=provider&status=working')">查看订单<Icon name="arrow" :size="11"/></button></view></view><image class="workbench-art" src="/static/illustrations/empty-services.png" mode="aspectFit"/></view>
    <view class="worker-tabs"><button class="worker-tab" :class="{selected:tab==='services'}" @click="changeTab('services')">我的服务<text class="worker-tab-count" v-if="serviceCountLabel">{{ serviceCountLabel }}</text></button><button class="worker-tab" :class="{selected:tab==='jobs'}" @click="changeTab('jobs')">接单大厅</button></view>
    <GamePicker v-if="tab==='jobs'" :active="props.active"/>
    <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load()">重新加载</button></view><view v-else-if="loading&&!(tab==='services'?mine.length:jobs.length)" class="loading-line">正在同步{{ tab==='services'?'服务':'需求' }}…</view>
    <template v-else-if="tab==='services'"><view v-if="mine.length" class="card-grid"><ServiceCard v-for="service in mine" :key="service.id" :service="service"><template #actions><view class="service-controls"><button class="service-control" @click="go('/pages/services/publish?id='+service.id)"><Icon name="edit" :size="13"/>编辑服务</button><button v-if="service.status==='active'" class="service-control status-active" :disabled="pausing===service.id" @click="pause(service)">{{ pausing===service.id?'暂停中':'● 接单中' }}</button><text v-else class="service-status">{{ statusLabel(service.status) }}</text></view></template></ServiceCard></view><EmptyState v-else title="还没有上架的服务" :detail="state.user?'选择擅长的游戏，发布你的第一项服务。':'登录后即可发布服务和接单'" icon=""><button class="empty-link" @click="changeTab('jobs')">先逛接单大厅<Icon name="arrow" :size="13"/></button></EmptyState></template>
    <template v-else><view v-if="jobs.length" class="jobs-list"><view v-for="job in jobs" :key="job.id" class="job-card"><view class="job-heading"><text>{{ game.short_name }} · 公开需求</text><text>{{ time(job.created_at) }}</text></view><text class="job-title">{{ job.title }}</text><text v-if="job.requirements" class="job-description">{{ job.requirements }}</text><view class="job-bottom"><text class="job-price">¥{{ money(job.amount_cents) }}</text><button class="job-claim" :disabled="!!claiming" @click="claim(job)">{{ claiming===job.id?'领取中…':'立即接单' }}</button></view></view></view><EmptyState v-else title="暂时没有待接需求" detail="试试其他游戏，下一位队友也许正在路上。" illustration="/static/illustrations/empty-orders.png"/></template>
    <view v-if="moreError" class="hint danger">{{ moreError }}</view><button v-if="hasMore&&!loading&&!error" class="load-more" :disabled="loadingMore" @click="load(true)">{{ loadingMore?'正在加载…':moreError?'重试加载':'加载更多' }}<Icon name="bottom" :size="12"/></button>
    <button class="club-banner" @click="go('/pages/community/clubs/index')"><Icon name="club" :size="22"/><view class="club-banner-copy"><text class="club-banner-title">发现俱乐部</text><text class="club-banner-detail">找到合拍的团队，一起认真游戏</text></view><Icon name="arrow" :size="14"/></button>
  </view>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '../../../components/Icon.vue'
import GamePicker from '../../../components/GamePicker.vue'
import ServiceCard from '../../../components/ServiceCard.vue'
import EmptyState from '../../../components/EmptyState.vue'
import { state, game, init, sessionRevision, clearSession } from '../../../lib/store'
import { listServices, pauseService } from '../../../api/services.api'
import { listJobs, claimJob } from '../../../api/orders.api'
import { getMyStats } from '../../../api/users.api'
import { listClubs } from '../../../api/clubs.api'
import { errorMessage, errorStatus, token } from '../../../api/client'
import { money, time, statusLabel, go, notify, assetUrl } from '../../../lib/format'
import { useMainVisibility, refreshMainList } from '../../../lib/navigation'
import type { Club, PublicJob, Service, UserStats } from '../../../types/models'
const props = defineProps<{ active: boolean }>()
const pageSize = 20
const claimClubs = ref<Club[]>([]), tab = ref<'services' | 'jobs'>('services'), jobs = ref<PublicJob[]>([]), mine = ref<Service[]>([]), stats = ref<Partial<UserStats>>({})
const error = ref(''), moreError = ref(''), loading = ref(false), loadingMore = ref(false), hasMore = ref(false), claiming = ref(''), pausing = ref('')
const serviceCountLabel = computed(() => stats.value.services_count === undefined ? '' : String(stats.value.services_count))
let version = 0, visible = false
/** 捕获当前私有数据及原生操作所属的账号与会话。 */
function currentSession() { return { id: state.user?.id, token: token(), revision: sessionRevision() } }
/** 会话轮换后不能提交旧请求、旧操作菜单或迟到的跳转。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return session.id === state.user?.id && session.token === token() && session.revision === sessionRevision() }
/** 未登录显示零，已登录等待真实数据。 */
function statValue(key: keyof UserStats): number | string { return !state.user ? 0 : stats.value[key] ?? '—' }
/** 访客进入登录后继续发布服务。 */
function openPublish(): void { go(state.user ? '/pages/services/publish' : '/pages/account/login?next=' + encodeURIComponent('/pages/services/publish')) }
/** 同步分页与统计，防止跨标签、跨游戏和跨身份请求覆盖。 */
async function load(append = false, preserve = false): Promise<void> {
  if (!visible) return
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const current = ++version, selectedTab = tab.value
  if (adding) loadingMore.value = true
  else { loading.value = true; loadingMore.value = false; error.value = ''; if (!preserve) hasMore.value = false }
  moreError.value = ''
  let session = currentSession()
  const currentRequest = () => visible && current === version && sameSession(session) && tab.value === selectedTab
  try {
    await init(); if (current !== version || !visible) return; session = currentSession()
    if (!state.user) { mine.value = []; stats.value = { earnings_cents: 0 }; claimClubs.value = []; if (selectedTab !== 'jobs') { jobs.value = []; return } }
    const gameId = state.gameId
    /** 使用选定页签和游戏读取服务或需求，刷新时保留此前加载的全部页。 */
    const read = (limit: number, offset: number): Promise<Array<Service | PublicJob>> => selectedTab === 'services' ? listServices({ mine: 1, limit, offset }) : listJobs({ game_id: gameId, limit, offset })
    const loadedCount = selectedTab === 'services' ? mine.value.length : jobs.value.length
    const retained = preserve && loadedCount > 0
    const [listing, summary, clubs] = await Promise.all([
      retained ? refreshMainList(loadedCount, pageSize, read, currentRequest) : read(pageSize, adding ? loadedCount : 0).then(items => ({ items, hasMore: items.length === pageSize })),
      adding || !state.user ? null : getMyStats(), adding || !state.user ? null : loadClaimClubs(current),
    ])
    if (!listing || !currentRequest() || (selectedTab === 'jobs' && state.gameId !== gameId)) return
    if (selectedTab === 'services') { const services = listing.items.filter((item): item is Service => 'provider_id' in item); mine.value = adding ? [...mine.value, ...services] : services }
    else { const requests = listing.items.filter((item): item is PublicJob => 'requirements' in item); jobs.value = adding ? [...jobs.value, ...requests] : requests }
    hasMore.value = listing.hasMore
    if (!adding) { stats.value = summary || {}; claimClubs.value = (clubs || []).filter(club => club.status === 'active' && ['owner', 'active'].includes(club.membership_status || '')) }
  } catch (err) { if (currentRequest()) { if (errorStatus(err) === 401) clearSession(); else if (adding || (preserve && (selectedTab === 'services' ? mine.value.length : jobs.value.length))) moreError.value = errorMessage(err); else error.value = errorMessage(err) } }
  finally { if (current === version) { loading.value = false; loadingMore.value = false } }
}
/** 读取全部挂靠分页，俱乐部多时仍可选择。 */
async function loadClaimClubs(current: number): Promise<Club[]> { const clubs: Club[] = []; while (current === version) { const batch = await listClubs({ mine: 1, limit: 100, offset: clubs.length }); clubs.push(...batch); if (batch.length < 100) break } return clubs }
/** 切换服务管理或接单大厅。 */
function changeTab(value: 'services' | 'jobs'): void { if (tab.value !== value) { tab.value = value; void load(false, true) } }
/** 领取需求前明确选择挂靠，取消选择不创建交易。 */
async function claim(job: PublicJob): Promise<void> {
  if (!state.user) return go('/pages/account/login?next=' + encodeURIComponent('/pages/main/index?tab=worker'))
  if (claiming.value) return
  const session = currentSession()
  claiming.value = job.id
  try { const clubId = await chooseClaimClub(job.game_id); if (clubId === null || !sameSession(session)) return; const order = await claimJob(job.id, clubId); if (sameSession(session)) go('/pages/orders/detail?id=' + order.id) }
  catch (err) { if (sameSession(session)) { notify(err); await load(false, true) } }
  finally { if (sameSession(session)) claiming.value = '' }
}
/** 筛选当前游戏允许使用的俱乐部。 */
function chooseClaimClub(gameId: string): Promise<string | null> { const clubs = claimClubs.value.filter(club => club.game_ids.includes(gameId)); return clubs.length ? showClaimClubPage(clubs, 0) : Promise.resolve('') }
/** 将俱乐部操作菜单分页，保留翻页和取消。 */
function showClaimClubPage(clubs: Club[], page: number): Promise<string | null> {
  const options: Array<{ name: string; id?: string; page?: number }> = [{ name: '独立接单（不挂靠）', id: '' }, ...clubs.slice(page * 3, page * 3 + 3).map(club => ({ name: club.name, id: club.id }))]
  if (page > 0) options.push({ name: '上一页俱乐部', page: page - 1 })
  if ((page + 1) * 3 < clubs.length) options.push({ name: '下一页俱乐部', page: page + 1 })
  return new Promise(resolve => uni.showActionSheet({ itemList: options.map(option => option.name), success: result => { const option = options[result.tapIndex]; resolve(!option ? null : option.page === undefined ? option.id ?? null : showClaimClubPage(clubs, option.page)) }, fail: () => resolve(null) }))
}
/** 暂停本人服务并刷新审核状态。 */
async function pause(service: Service): Promise<void> { if (pausing.value) return; const session = currentSession(); pausing.value = service.id; try { await pauseService(service.id); if (!sameSession(session)) return; notify('服务已暂停'); await load(false, true) } catch (err) { if (sameSession(session)) notify(err) } finally { if (sameSession(session)) pausing.value = '' } }
/** 隐藏视图后使未完成请求失效。 */
function stopLoading(): void { visible = false; version++; loading.value = false; loadingMore.value = false }
/** 主视图重显时刷新已加载范围，保持滚动依赖的列表高度。 */
function showPage(): void { visible = true; void load(false, true) }
watch(() => state.gameId, () => { jobs.value = []; if (visible && tab.value === 'jobs') void load() })
useMainVisibility(() => props.active, showPage, stopLoading)
/** 隐藏时也立即清理上一会话的服务、挂靠和统计，不等待新账号请求完成。 */
watch([() => state.user?.id, sessionRevision], () => { version++; mine.value = []; jobs.value = []; stats.value = {}; claimClubs.value = []; hasMore.value = false; error.value = ''; moreError.value = ''; claiming.value = ''; pausing.value = ''; loading.value = false; loadingMore.value = false; if (visible) void load() }, { flush: 'sync' })
</script>
<style scoped>
.worker-content{padding:8px 16px 24px;min-width:0}.worker-identity{display:flex;align-items:center;gap:8px;min-height:44px;padding-right:var(--capsule-right-inset,100px);min-width:0}.avatar{width:32px;height:32px}.worker-avatar-image{width:100%;height:100%}.worker-name{font-size:14px;font-weight:650;min-width:0;max-width:84px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.provider-status{flex-shrink:0;padding:2px 6px;border:1px solid #83e7b733;border-radius:12px;font-size:9px;line-height:16px;color:var(--accent);white-space:nowrap}.worker-settings{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:28px;height:28px;padding:0;margin-left:auto;background:transparent;color:var(--text)}.worker-stats{display:flex;padding:16px 0 20px;border-bottom:1px solid var(--line-subtle)}.worker-stat{display:flex;flex:1;flex-direction:column;align-items:center;gap:3px;min-width:0;border-right:1px solid var(--line-subtle)}.worker-stat:last-child{border-right:0}.worker-stat-value{font-size:20px;line-height:27px;font-weight:650;font-variant-numeric:tabular-nums;white-space:nowrap}.worker-stat-label{font-size:10px;color:var(--muted)}.workbench-intro{position:relative;display:flex;align-items:center;min-height:154px;padding:20px 0;min-width:0}.intro-copy{position:relative;z-index:1;width:calc(100% - 144px);min-width:0}.intro-title{font-size:21px;font-weight:750;line-height:30px;letter-spacing:-.7px;white-space:nowrap}.intro-accent{color:var(--accent)}.intro-detail{display:block;margin-top:6px;font-size:10px;line-height:18px;color:var(--muted)}.intro-actions{display:flex;align-items:center;flex-wrap:wrap;gap:8px 12px;margin-top:12px}.publish-entry{display:flex;align-items:center;justify-content:center;gap:5px;height:36px;padding:0 10px;border:1px solid var(--accent);border-radius:7px;background:transparent;color:var(--accent);font-size:11px;font-weight:650;white-space:nowrap}.orders-entry{display:flex;align-items:center;justify-content:center;gap:3px;min-height:28px;padding:0;background:transparent;color:var(--muted);font-size:10px;white-space:nowrap}.workbench-art{position:absolute;right:-8px;top:26px;width:164px;height:110px;pointer-events:none}.worker-tabs{display:flex;gap:24px;border-bottom:1px solid var(--line-subtle);margin-bottom:12px}.worker-tab{display:flex;align-items:center;gap:5px;height:40px;padding:0 4px;background:transparent;color:var(--muted);font-size:13px;border-radius:0;border-bottom:2px solid transparent}.worker-tab.selected{color:var(--text);border-bottom-color:var(--accent);font-weight:650}.worker-tab-count{font-size:10px;color:var(--muted)}.service-controls{display:flex;align-items:center;gap:4px;width:100%;min-width:0}.service-control{display:flex;align-items:center;justify-content:center;gap:3px;min-height:28px;flex:1;min-width:0;padding:0 3px;border:0;border-radius:0;background:transparent;color:#c0d2c6;font-size:10px;white-space:nowrap}.status-active{color:var(--accent)}.service-status{display:flex;align-items:center;justify-content:center;flex:1;font-size:10px;color:var(--muted)}.empty-link{display:flex;align-items:center;justify-content:center;gap:5px;min-height:28px;margin-top:4px;padding:0;background:transparent;color:var(--accent);font-size:12px}.jobs-list{display:flex;flex-direction:column;min-width:0}.job-card{display:flex;flex-direction:column;gap:10px;padding:16px 0;border-bottom:1px solid var(--line-subtle);min-width:0}.job-heading{display:flex;justify-content:space-between;gap:8px;font-size:10px;color:var(--muted)}.job-title{font-size:15px;font-weight:650;line-height:22px}.job-description{font-size:12px;line-height:20px;color:var(--muted);display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.job-bottom{display:flex;align-items:center;justify-content:space-between;gap:12px}.job-price{font-size:23px;font-weight:700;color:var(--accent)}.job-claim{display:flex;align-items:center;justify-content:center;min-width:88px;height:36px;padding:0 12px;border:1px solid var(--accent);border-radius:7px;background:transparent;color:var(--accent);font-size:12px}.load-more{display:flex;align-items:center;justify-content:center;gap:5px;width:100%;min-height:36px;margin-top:12px;background:transparent;color:var(--muted);font-size:12px}.club-banner{display:flex;align-items:center;gap:10px;width:100%;min-height:64px;padding:12px 0;margin-top:16px;border:0;border-top:1px solid var(--line-subtle);border-radius:0;background:transparent;text-align:left;color:#bfd2c6}.club-banner-copy{flex:1;display:flex;flex-direction:column;gap:3px;min-width:0}.club-banner-title{font-size:13px;font-weight:650;color:var(--text)}.club-banner-detail{font-size:10px;color:var(--muted)}
@media(max-width:350px){.worker-identity{gap:6px}.worker-name{max-width:52px;font-size:12px}.provider-status{font-size:8px;padding:1px 4px}.worker-settings{width:24px}.intro-copy{width:calc(100% - 112px)}.intro-title{font-size:19px;letter-spacing:-.8px}.workbench-art{width:134px;height:90px;right:-10px;top:30px}.intro-actions{gap:8px}.publish-entry{padding:0 8px}.orders-entry{font-size:9px}.worker-stat-value{font-size:18px}}
</style>
