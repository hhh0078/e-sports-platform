<!-- 文件作用：以游戏主题工作台展示真实打手统计、服务管理和可分页领取的公开需求。 -->
<template>
  <AppShell active="worker" immersive>
    <SectionHero eyebrow="打手工作台" title="把热爱，变成实力" subtitle="不必挂靠，也能凭本事接单。" art="worker">
      <view v-if="state.user" class="worker-identity"><text class="worker-name">{{ state.user.nickname }}</text><view class="worker-state" :class="{inactive:!state.user?.provider_enabled}"><text class="worker-dot"/>{{ state.user?.provider_enabled ? '接单身份已开启' : '接单身份未开启' }}</view></view>
    </SectionHero>
    <view class="worker-content">
        <view class="worker-stats">
          <view><text class="stat-number">{{ statValue('provider_working_count') }}</text><text class="stat-label">进行中订单</text></view>
          <view><text class="stat-number">{{ statValue('provider_completed_count') }}</text><text class="stat-label">已完成订单</text></view>
          <view><text class="stat-number"><text v-if="stats.earnings_cents !== undefined" class="currency">¥</text>{{ stats.earnings_cents === undefined ? '—' : money(stats.earnings_cents) }}</text><text class="stat-label">已完成收入</text></view>
        </view>
        <view v-if="!state.user?.provider_enabled" class="provider-notice"><text>{{ state.user ? "完善个人介绍，开启接单身份。" : "登录后开启服务发布" }}</text><button @click="go('/pages/me/index')">去完善 <Icon name="arrow" :size="12"/></button></view>
        <view class="worker-actions"><button class="publish-action" @click="openPublish"><Icon name="plus" :size="24"/>发布服务</button><button class="orders-action" @click="go('/pages/orders/index?role=provider&status=working')">进行中订单 <Icon name="arrow" :size="18"/></button></view>
        <view class="worker-tabs"><button :class="{selected:tab==='services'}" @click="changeTab('services')">我的服务 <text v-if="serviceCountLabel">{{ serviceCountLabel }}</text></button><button :class="{selected:tab==='jobs'}" @click="changeTab('jobs')">接单大厅</button></view>
        <view v-if="tab==='jobs'" class="jobs-games"><GamePicker/></view>
        <view class="list-caption">{{ tab==='services' ? '审核通过后展示给玩家。' : '选择擅长的游戏，找到合拍的队友。' }}<text v-if="tab==='jobs'&&!loading&&!error">已加载 {{ jobs.length }} 个需求</text></view>
        <view v-if="error" class="worker-error"><text>{{ error }}</text><button @click="load()">重新加载</button></view>
        <view v-else-if="loading" class="loading-line">{{ tab==='services' ? '正在同步服务…' : '正在寻找新的机会…' }}</view>
        <template v-else-if="tab==='services'">
          <view v-if="mine.length" class="services-panel">
            <view v-for="service in mine" :key="service.id" class="my-service">
              <view class="service-thumbnail" @click="go('/pages/service/index?id=' + service.id)"><CoverArt :src="service.cover_url" :labels="service.labels"/></view>
              <view class="service-body">
                <view class="service-heading"><text class="service-title" @click="go('/pages/service/index?id=' + service.id)">{{ service.title }}</text><text :class="['service-status',service.status]">{{ statusLabel(service.status) }}</text></view>
                <view class="service-price"><text class="currency">¥</text><text>{{ money(service.price_cents) }}</text><text class="service-unit"> / {{ service.unit || '单' }}</text></view>
                <view class="service-controls"><button @click="go('/pages/publish/index?id=' + service.id)"><Icon name="edit" :size="14"/>编辑服务</button><button v-if="service.status==='active'" :disabled="pausing===service.id" @click="pause(service)"><Icon name="minus" :size="14"/>{{ pausing===service.id ? '暂停中…' : '暂停接单' }}</button><text v-else class="service-next">{{ service.status==='pending' ? '等待平台审核' : '编辑后重新审核' }}</text></view>
              </view>
            </view>
          </view>
          <view v-else-if="!state.user" class="guest-note" @click="go('/pages/login/index')">请登录后查看已发布的服务</view><EmptyState v-else title="让你的实力被看见" detail="选择游戏、定好服务内容和价格，制作一张专属封面。" icon="photo"><button class="btn btn-small" @click="openPublish">发布第一项服务</button></EmptyState>
        </template>
        <template v-else>
          <view v-if="jobs.length" class="jobs-list"><view v-for="job in jobs" :key="job.id" class="job-card"><view class="job-heading"><text class="job-game">{{ game.short_name }} · 公开需求</text><text class="job-time">{{ time(job.created_at) }}</text></view><view class="job-title">{{ job.title }}</view><view v-if="job.requirements" class="job-description">{{ job.requirements }}</view><view class="job-bottom"><view><text class="job-price">¥{{ money(job.amount_cents) }}</text><text class="job-budget"> / 预算</text></view><button class="btn btn-small" :disabled="!!claiming" @click="claim(job)">{{ claiming===job.id ? '领取中…' : '立即接单' }}</button></view></view></view>
          <EmptyState v-else title="现在还没有待接需求" detail="试试切换其他游戏，或者发布自己的服务，让玩家找到你。" icon="game"/>
        </template>
        <view v-if="moreError" class="more-error">{{ moreError }}</view>
        <button v-if="hasMore&&!loading&&!error" class="load-more" :disabled="loadingMore" @click="load(true)">{{ loadingMore ? '正在加载…' : moreError ? '重试加载' : tab==='services' ? '加载更多服务' : '加载更多需求' }}</button>
        <button class="club-banner" @click="go('/pages/clubs/index')"><Icon name="club" :size="31"/><view><text>发现俱乐部</text><view>独立接单，也能找到自己的队伍。</view></view><Icon name="arrow" :size="19"/></button>
        <view class="worker-reminder"><Icon name="info" :size="16"/><text>约定与交付，请留在订单聊天室。</text></view>
    </view>
  </AppShell>
</template>
<script setup>
/** 文件作用：按当前身份和游戏分页加载工作台，保留挂靠选择、取消和服务暂停的真实流程。 */
import { computed, ref, watch } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import SectionHero from '../../components/SectionHero.vue'
import CoverArt from '../../components/CoverArt.vue'
import Icon from '../../components/Icon.vue'
import GamePicker from '../../components/GamePicker.vue'
import EmptyState from '../../components/EmptyState.vue'
import { state, game, init } from '../../lib/store'
import { api } from '../../lib/api'
import { money, time, statusLabel, go, notify } from '../../lib/format'
const pageSize = 20
const claimClubs = ref([]), tab = ref('services'), jobs = ref([]), mine = ref([]), stats = ref({})
const error = ref(''), moreError = ref(''), loading = ref(false), loadingMore = ref(false), hasMore = ref(false), claiming = ref(''), pausing = ref('')
const serviceCountLabel = computed(() => Number.isInteger(stats.value.services_count) ? String(stats.value.services_count) : mine.value.length ? '已加载 ' + mine.value.length : '')
let version = 0
/**
 * 显示服务端提供的打手统计，未加载时不伪装为零。
 * @param {string} key - 打手统计字段。
 * @returns {number|string} 真实统计或等待占位符。
 */
function statValue(key) { return !state.user ? 0 : Number.isFinite(stats.value[key]) ? stats.value[key] : '—' }
/** 访客点击发布时进入登录，浏览工作台不要求登录。 */
function openPublish() { go(state.user ? '/pages/publish/index' : '/pages/login/index?next=' + encodeURIComponent('/pages/publish/index')) }
/**
 * 分页读取当前标签的数据，只有最新身份、标签和游戏的请求可以更新页面。
 * @param {boolean} append - 是否追加下一页，按钮事件对象不视为追加。
 * @returns {Promise<void>} 完成当前列表与统计加载。
 */
async function load(append = false) {
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const current = ++version, selectedTab = tab.value
  if (adding) loadingMore.value = true
  else { loading.value = true; loadingMore.value = false; error.value = ''; hasMore.value = false }
  moreError.value = ''
  try {
    await init()
    if (current !== version) return
    if (!state.user) { mine.value = []; stats.value = { earnings_cents: 0 }; claimClubs.value = []; if (selectedTab !== 'jobs') { jobs.value = []; return } }
    const userId = state.user?.id, gameId = state.gameId
    const list = selectedTab === 'jobs' ? jobs : mine
    const endpoint = selectedTab === 'jobs' ? '/jobs?game_id=' + encodeURIComponent(gameId) : '/services?mine=1'
    const [items, summary, clubs] = await Promise.all([
      api(endpoint + '&limit=' + pageSize + '&offset=' + (adding ? list.value.length : 0)),
      adding || !state.user ? null : api('/me/stats'),
      adding || !state.user ? null : loadClaimClubs(current)
    ])
    if (current !== version || state.user?.id !== userId || tab.value !== selectedTab || (selectedTab === 'jobs' && state.gameId !== gameId)) return
    const next = Array.isArray(items) ? items : []
    list.value = adding ? [...list.value, ...next] : next
    hasMore.value = next.length === pageSize
    if (!adding) {
      stats.value = summary || {}
      claimClubs.value = (clubs || []).filter(club => club.status === 'active' && ['owner', 'active'].includes(club.membership_status))
    }
  } catch (err) {
    if (current === version) { if (adding) moreError.value = err.message; else error.value = err.message }
  } finally {
    if (current === version) { loading.value = false; loadingMore.value = false }
  }
}
/**
 * 读取全部挂靠分页，避免俱乐部数量超过接口默认页长时无法选择后续组织。
 * @param {number} current - 所属工作台加载版本。
 * @returns {Promise<object[]>} 当前身份全部俱乐部，过期请求提前结束。
 */
async function loadClaimClubs(current) {
  const clubs = []
  while (current === version) {
    const batch = await api('/clubs?mine=1&limit=100&offset=' + clubs.length)
    const items = Array.isArray(batch) ? batch : []
    clubs.push(...items)
    if (items.length < 100) break
  }
  return clubs
}
/**
 * 切换工作台标签，从第一页重新同步以避免跨标签分页。
 * @param {string} value - 服务或接单大厅标签。
 */
function changeTab(value) { if (tab.value !== value) { tab.value = value; load() } }
/**
 * 明确选择挂靠后原子领取公开需求，取消菜单不修改订单。
 * @param {object} job - 当前公开需求。
 * @returns {Promise<void>} 领取成功后打开订单沟通。
 */
async function claim(job) {
  if (!state.user) return go('/pages/login/index?next=' + encodeURIComponent('/pages/worker/index'))
  if (!state.user.provider_enabled) return notify('请先在个人中心开启接单身份')
  if (claiming.value) return
  claiming.value = job.id
  try {
    const clubId = await chooseClaimClub(job.game_id)
    if (clubId === null) return
    const order = await api('/orders/' + job.id + '/actions', 'POST', { action: 'claim', note: '', club_id: clubId })
    go('/pages/order/index?id=' + order.id)
  } catch (err) { notify(err); await load() }
  finally { claiming.value = '' }
}
/**
 * 仅向打手提供当前游戏的有效挂靠，允许独立接单。
 * @param {string} gameId - 公开需求所属游戏。
 * @returns {Promise<string|null>} 挂靠编号、独立接单空串或取消标记。
 */
function chooseClaimClub(gameId) {
  const clubs = claimClubs.value.filter(club => (club.game_ids || []).includes(gameId))
  return clubs.length ? showClaimClubPage(clubs, 0) : Promise.resolve('')
}
/**
 * 将俱乐部选择分成每页最多六项，保留前后翻页和取消能力。
 * @param {object[]} clubs - 该游戏已通过挂靠的俱乐部。
 * @param {number} page - 当前菜单页码。
 * @returns {Promise<string|null>} 用户明确选择的挂靠或取消标记。
 */
function showClaimClubPage(clubs, page) {
  const options = [{ name: '独立接单（不挂靠）', id: '' }, ...clubs.slice(page * 3, page * 3 + 3)]
  if (page > 0) options.push({ name: '上一页俱乐部', page: page - 1 })
  if ((page + 1) * 3 < clubs.length) options.push({ name: '下一页俱乐部', page: page + 1 })
  return new Promise(resolve => uni.showActionSheet({
    itemList: options.map(option => option.name),
    success: result => { const option = options[result.tapIndex]; resolve(!option ? null : option.page === undefined ? option.id : showClaimClubPage(clubs, option.page)) },
    fail: () => resolve(null)
  }))
}
/**
 * 暂停本人已上架服务并刷新真实状态，不隐藏暂停或待审核记录。
 * @param {object} service - 当前打手的已上架服务。
 * @returns {Promise<void>} 完成暂停及列表更新。
 */
async function pause(service) {
  if (pausing.value) return
  pausing.value = service.id
  try { await api('/services/' + service.id + '/pause', 'POST', {}); notify('服务已暂停'); await load() }
  catch (err) { notify(err) }
  finally { pausing.value = '' }
}
/** 页面离开时使在途请求失效，避免旧身份返回覆盖之后的工作台。 */
function stopLoading() { version++; loading.value = false; loadingMore.value = false }
/** 当前游戏变化后刷新接单大厅，服务管理仍显示本人全部游戏的服务。 */
function gameChanged() { if (tab.value === 'jobs') load() }
watch(() => state.gameId, gameChanged)
onShow(load)
onHide(stopLoading)
</script>
<style scoped>
.guest-note{padding:65px 12px;text-align:center;color:#89968f;font-size:12px}.worker-content{padding:0 14px 12px}.worker-identity{display:flex;align-items:center;gap:12px;margin-top:24px}.worker-name{font-size:17px;font-weight:750;max-width:46%;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.worker-state{display:flex;align-items:center;gap:6px;font-size:12px;color:var(--accent)}.worker-state.inactive{color:#abb5ae}.worker-dot{width:8px;height:8px;border:2px solid currentColor;border-radius:50%;box-sizing:border-box}
.worker-stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));min-height:80px;align-items:center;border:1px solid #ffffff22;border-radius:12px;background:#ffffff03}.worker-stats>view{min-width:0;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:5px;border-right:1px solid #ffffff22;padding:3px 4px}.worker-stats>view:last-child{border-right:0}.stat-number{max-width:100%;font-size:28px;line-height:32px;font-weight:800;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;letter-spacing:-.5px}.stat-number .currency{font-size:15px;margin-right:2px}.stat-label{font-size:11px;color:#abb5af;white-space:nowrap}.provider-notice{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-top:12px;padding:10px 12px;border:1px solid #ffffff14;border-radius:8px;font-size:12px;color:#b5c2b9}.provider-notice button{display:flex;align-items:center;gap:4px;margin:0;padding:0;background:none;color:var(--accent);font-size:12px;line-height:24px;white-space:nowrap}
.worker-actions{display:grid;grid-template-columns:1.35fr 1fr;gap:10px;margin:10px 0 13px}.worker-actions button{min-height:46px;margin:0;display:flex;align-items:center;justify-content:center;gap:8px;padding:7px 8px;border-radius:10px;font-size:16px;font-weight:750;line-height:28px}.publish-action{background:#ffffff05;color:var(--accent);border:1px solid #ffffff28}.orders-action{background:none;color:var(--text);border:1px solid #9baaa277}.worker-actions button[disabled]{opacity:.5}.worker-tabs{display:flex;gap:30px;min-height:48px;margin:0 -14px;padding:0 18px;border-bottom:1px solid #ffffff22}.worker-tabs button{position:relative;display:flex;align-items:center;gap:7px;margin:0;padding:0 2px;min-height:48px;line-height:26px;border:0;border-radius:0;background:none;color:#9ca6a0;font-size:18px;font-weight:650;white-space:nowrap}.worker-tabs button.selected{color:var(--text)}.worker-tabs button.selected:after{top:auto;content:'';position:absolute;left:0;right:0;bottom:0;height:3px;background:var(--accent);border-radius:2px}.worker-tabs button text{font-size:14px;font-weight:500;color:#acb6af}.list-caption{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:5px;padding:10px 1px;color:#a6b1aa;font-size:12px;line-height:18px}.list-caption>text{font-size:10px;color:#8c9e91}.jobs-games{margin:11px -14px 0}
.services-panel{border:1px solid #ffffff22;border-radius:12px;overflow:hidden;background:#ffffff03}.my-service{display:flex;align-items:center;gap:12px;padding:12px 11px;min-height:120px;box-sizing:border-box;border-bottom:1px solid #ffffff18}.my-service:last-child{border-bottom:0}.service-thumbnail{width:130px;flex-shrink:0;border-radius:9px;overflow:hidden;background:#17221b}.service-body{flex:1;min-width:0}.service-heading{display:flex;align-items:center;justify-content:space-between;gap:6px}.service-title{font-size:14px;line-height:20px;font-weight:750;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.service-status{flex-shrink:0;white-space:nowrap;border:1px solid #b0bdb354;border-radius:7px;padding:2px 6px;font-size:10px;line-height:14px;color:#aeb9b1}.service-status.active{color:var(--accent);border-color:var(--accent);background:#00aa7010}.service-status.pending{color:#d6bb80;border-color:#d6bb8055}.service-status.rejected{color:#eaa89d;border-color:#eaa89d55}.service-price{display:flex;align-items:baseline;color:var(--accent);font-size:25px;line-height:29px;font-weight:800;margin-top:4px;white-space:nowrap}.service-price .currency{font-size:14px;margin-right:2px}.service-unit{font-size:12px;font-weight:400;color:#bac4be;margin-left:7px}.service-controls{display:flex;align-items:center;gap:8px;margin-top:9px}.service-controls button{display:flex;align-items:center;justify-content:center;gap:5px;flex:1;min-width:0;margin:0;padding:5px 3px;min-height:34px;line-height:22px;border:1px solid #ffffff2e;border-radius:8px;color:var(--text);background:none;font-size:12px;white-space:nowrap}.service-controls button[disabled]{opacity:.5}.service-next{flex:1;font-size:10px;line-height:16px;color:#8f9e94;text-align:center}
.jobs-list{display:flex;flex-direction:column;gap:10px}.job-card{padding:16px;border:1px solid #ffffff20;border-radius:12px;background:#ffffff03}.job-heading,.job-bottom{display:flex;align-items:center;justify-content:space-between;gap:10px}.job-game{color:var(--accent);font-size:11px}.job-time,.job-budget{color:#91a197;font-size:11px}.job-title{font-size:17px;font-weight:700;line-height:24px;margin-top:14px}.job-description{font-size:12px;color:#a0aea5;margin-top:8px;line-height:1.7;white-space:pre-wrap;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.job-bottom{margin-top:16px;padding-top:12px;border-top:1px solid #ffffff12}.job-price{font-size:24px;color:var(--accent);font-weight:750}.job-bottom .btn{font-size:12px;min-width:88px;margin:0}
.worker-error{display:flex;flex-direction:column;gap:10px;padding:16px;border:1px solid #c66f5255;border-radius:10px;color:#edb39e;font-size:13px;margin:10px 0}.worker-error button,.load-more{margin:0;border:1px solid #ffffff25;background:none;color:var(--text);font-size:13px;line-height:36px;border-radius:8px}.worker-error button{align-self:flex-start;padding:0 15px}.load-more{margin-top:12px;width:100%}.more-error{font-size:12px;color:#edb39e;margin-top:12px}.club-banner{display:flex;align-items:center;gap:12px;width:100%;min-height:70px;margin:11px 0 0;padding:13px 16px;border:1px solid #ffffff15;border-radius:12px;background:#ffffff02;color:var(--accent);text-align:left;line-height:normal}.club-banner>view{flex:1;min-width:0}.club-banner>view>text{font-size:16px;font-weight:750;color:var(--text)}.club-banner>view>view{font-size:12px;color:#9aa79e;line-height:18px;margin-top:5px}.worker-reminder{display:flex;align-items:center;justify-content:center;gap:6px;color:#8b9a90;font-size:11px;padding:13px 0 0;line-height:18px}
@media(max-width:350px){.guest-note{padding:65px 12px;text-align:center;color:#89968f;font-size:12px}.worker-content{padding-left:12px;padding-right:12px}.worker-tabs{margin-left:-12px;margin-right:-12px;gap:22px}.worker-tabs button{font-size:17px}.worker-identity{gap:8px}.worker-state{font-size:11px}.worker-actions button{font-size:14px;gap:5px}.stat-number{font-size:25px}.stat-label{font-size:10px}.my-service{gap:9px;padding:11px 9px}.service-thumbnail{width:91px}.service-title{font-size:12px}.service-status{font-size:9px;padding:2px 4px}.service-controls{gap:5px}.service-controls button{font-size:10px;gap:3px}.service-next{font-size:9px}.club-banner{padding-left:13px;padding-right:13px}.club-banner>view>view{font-size:11px}}
</style>
