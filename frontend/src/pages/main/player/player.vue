<!-- 文件作用：玩家常驻视图展示游戏选择、搜索排序和统一服务卡片，按设计图实现。 -->
<template>
  <view class="player-content">
    <view class="player-hero"><image v-if="game.cover_url" class="hero-image" :src="assetUrl(game.cover_url)" mode="aspectFill" /><view class="hero-mask" /><view class="hero-copy"><view class="hero-text"><view class="hero-brand-row"><text class="hero-brand">开机电竞</text><ShareEntry :active="props.active" kind="home" :game-id="state.gameId" label="分享"/></view><text class="hero-title">开机即组队，一起赢</text><text class="hero-sub">{{ game.name }} · {{ (game.categories || []).slice(0, 2).join(' · ') }}</text></view><button v-if="!timelinePreview" class="btn btn-small hero-publish" @click="go('/pages/requests/publish')">发布需求<Icon name="arrow" :size="16" /></button></view></view>
    <view class="game-section"><GamePicker :active="props.active" /></view>
    <view class="services-section">
      <scroll-view scroll-x :show-scrollbar="false" class="category-scroll"><view class="categories"><button v-for="item in ['全部', ...(game.categories || [])]" :key="item" :class="['category', { active: category === item }]" @click="chooseCategory(item)">{{ item }}</button></view></scroll-view>
      <view class="service-toolbar"><text class="section-title">精选服务</text><view class="service-tools"><button class="text-btn muted" @click="searchOpen = !searchOpen"><Icon name="search" :size="16" />搜索</button><picker :range="sortOptions" :value="sortIndex" @change="changeSort(Number($event.detail.value))"><view class="sort-control">{{ sortOptions[sortIndex] }}<Icon name="bottom" :size="12" /></view></picker></view></view>
      <view v-if="searchOpen" class="search-row"><Icon name="search" :size="17" /><input class="search-input" v-model="query" placeholder="搜索服务、陪练或任务" confirm-type="search" @confirm="loadServices()" /><button v-if="query" class="text-btn muted" @click="clearSearch"><Icon name="close" :size="14" /></button><button class="text-btn" @click="loadServices()">搜索</button></view>
      <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="reload">重新加载</button></view><view v-else-if="loading && !services.length" class="loading-line">正在寻找好队友…</view><view v-else-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service" /></view><EmptyState v-else title="暂时没有匹配的服务" detail="换个分类试试，或发布需求。" icon="game"><button v-if="!timelinePreview" class="btn btn-small" @click="go('/pages/requests/publish')">发布需求</button></EmptyState>
      <view v-if="moreError" class="hint danger mt">{{ moreError }}</view><button v-if="hasMore && !loading && !error" class="btn-secondary btn-small load-more" :disabled="loadingMore" @click="loadServices(true)">{{ loadingMore ? '正在加载…' : moreError ? '重试加载' : '加载更多服务' }}</button>
      <button class="club-banner" @click="go('/pages/community/clubs/index')"><Icon name="club" :size="23" /><view class="club-banner-copy"><text class="club-banner-title">发现俱乐部</text><text class="club-banner-detail">找到合拍的队伍</text></view><Icon name="arrow" :size="15" /></button><view v-if="state.announcement" class="announcement"><Icon name="bell" :size="13" /><text>{{ state.announcement }}</text></view>
    </view>
  </view>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import Icon from '../../../components/Icon.vue'
import ShareEntry from '../../../components/ShareEntry.vue'
import { isTimelinePreview } from '../../../lib/sharing'
import GamePicker from '../../../components/GamePicker.vue'
import ServiceCard from '../../../components/ServiceCard.vue'
import EmptyState from '../../../components/EmptyState.vue'
import { state, game, init } from '../../../lib/store'
import { listServices } from '../../../api/services.api'
import { errorMessage } from '../../../api/client'
import { go, assetUrl } from '../../../lib/format'
import { useMainVisibility, refreshMainList } from '../../../lib/navigation'
import type { Service } from '../../../types/models'
const timelinePreview=isTimelinePreview()
const props = defineProps<{ active: boolean }>()
const hasMore = ref(false), loadingMore = ref(false), moreError = ref(''), services = ref<Service[]>([]), query = ref(''), category = ref('全部'), loading = ref(false), error = ref(''), sortIndex = ref(0), searchOpen = ref(false)
const sortOptions = ['综合排序', '价格从低到高', '成交优先']
const sortValues = ['recommended', 'price', 'completed']
let requestVersion = 0, submittedQuery = '', visible = false
/** 查询公开服务并分页，切换游戏或隐藏视图后丢弃旧请求。 */
async function loadServices(append = false, preserve = false): Promise<void> {
  if (!visible) return
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const version = ++requestVersion
  if (adding) loadingMore.value = true
  else { loading.value = true; error.value = ''; if (!preserve) { hasMore.value = false; submittedQuery = query.value.trim() } }
  moreError.value = ''
  const current = () => visible && version === requestVersion
  /** 使用同一筛选读取指定分页，返回时可以恢复此前全部已加载数据。 */
  const read = (limit: number, offset: number) => listServices({ game_id: state.gameId, category: category.value === '全部' ? '' : category.value, q: submittedQuery, sort: sortValues[sortIndex.value], limit, offset })
  try {
    if (preserve && services.value.length) { const result = await refreshMainList(services.value.length, 24, read, current); if (result) { services.value = result.items; hasMore.value = result.hasMore } }
    else { const items = await read(24, adding ? services.value.length : 0); if (current()) { services.value = adding ? [...services.value, ...items] : items; hasMore.value = items.length === 24 } }
  }
  catch (err) { if (version === requestVersion) { if (adding || (preserve && services.value.length)) moreError.value = errorMessage(err); else error.value = errorMessage(err) } }
  finally { if (version === requestVersion) { loading.value = false; loadingMore.value = false } }
}
/** 返回主视图同步可配置主题及服务列表。 */
async function reload(): Promise<void> { const version = ++requestVersion; try { await init(true,isTimelinePreview()); if (!visible || version !== requestVersion) return; if (category.value !== '全部' && !game.value.categories.includes(category.value)) category.value = '全部'; await loadServices(false, true) } catch (err) { if (visible && version === requestVersion) { if (services.value.length) moreError.value = errorMessage(err); else error.value = errorMessage(err); loading.value = false } } }
/** 选择分类时从第一页重新查询。 */
function chooseCategory(value: string): void { category.value = value; void loadServices() }
/** 清除搜索并恢复当前游戏全部服务。 */
function clearSearch(): void { query.value = ''; void loadServices() }
/** 使用官方选择器更新排序。 */
function changeSort(index: number): void { sortIndex.value = Math.max(0, Math.min(index, sortOptions.length - 1)); void loadServices() }
/** 隐藏后停止在途列表更新。 */
function hidePage(): void { visible = false; requestVersion++; loading.value = false; loadingMore.value = false }
/** 激活时刷新已加载范围，保留当前筛选及主容器记录的滚动位置。 */
function showPage(): void { visible = true; void reload() }
watch(() => state.gameId, () => { category.value = '全部'; query.value = ''; services.value = []; hasMore.value = false; if (visible) void loadServices() })
useMainVisibility(() => props.active, showPage, hidePage)
</script>
<style scoped>
.hero-brand-row{display:flex;align-items:center;gap:23.0769rpx;min-width:0}.player-content{padding-bottom:46.1538rpx}.player-hero{position:relative;isolation:isolate;min-height:calc(276.9231rpx + var(--status-bar-height, 0px) + var(--capsule-space, 0px));overflow:hidden}.hero-image{position:absolute;inset:0;width:100%;height:100%;z-index:-2}.hero-mask{position:absolute;inset:0;z-index:-1;background:linear-gradient(90deg,#07100dd9,#07100d18),linear-gradient(0deg,var(--bg),transparent 50%)}.hero-copy{display:flex;align-items:center;gap:23.0769rpx;padding:calc(var(--status-bar-height, 0px) + var(--capsule-space, 0px) + 15.3846rpx) 30.7692rpx 30.7692rpx;min-width:0}.hero-text{display:flex;flex:1;flex-direction:column;gap:7.6923rpx;min-width:0}.hero-brand{font-size:42.3077rpx;line-height:46.1538rpx;font-weight:800;letter-spacing:1.9231rpx;color:#fff}.hero-title{font-size:46.1538rpx;line-height:57.6923rpx;font-weight:800;letter-spacing:-0.9615rpx;color:#fff}.hero-sub{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;overflow-wrap:anywhere;color:#c3d0c8;font-size:21.1538rpx;line-height:34.6154rpx}.hero-publish{flex-shrink:0;width:196.1538rpx;min-height:69.2308rpx;height:69.2308rpx;padding:0 23.0769rpx;border-radius:38.4615rpx;font-size:23.0769rpx;gap:7.6923rpx;white-space:nowrap}.game-section{margin:0 23.0769rpx;border:1px solid var(--line-subtle);background:var(--surface);border-radius:23.0769rpx;overflow:hidden}.services-section{padding:30.7692rpx 30.7692rpx 0}.category-scroll{width:100%;white-space:nowrap}.categories{display:inline-flex;gap:15.3846rpx;padding-bottom:7.6923rpx;min-width:100%}.category{display:flex;align-items:center;justify-content:center;min-height:57.6923rpx;flex-shrink:0;padding:0 23.0769rpx;border:1px solid var(--line);border-radius:34.6154rpx;background:transparent;font-size:21.1538rpx;color:var(--muted)}.category.active{background:var(--accent);border-color:var(--accent);color:#092416;font-weight:650}.service-toolbar{display:flex;justify-content:space-between;gap:15.3846rpx;align-items:center;min-width:0;margin:30.7692rpx 0 23.0769rpx}.service-tools{display:flex;align-items:center;gap:15.3846rpx;min-width:0}.sort-control{display:flex;align-items:center;gap:7.6923rpx;font-size:19.2308rpx;color:var(--muted);min-height:53.8462rpx;white-space:nowrap}.search-row{display:flex;align-items:center;gap:15.3846rpx;background:var(--surface);border:1px solid var(--line);border-radius:19.2308rpx;padding:11.5385rpx 19.2308rpx;margin-bottom:23.0769rpx;min-width:0}.search-input{flex:1;font-size:23.0769rpx;min-width:0}.load-more{width:100%;margin-top:23.0769rpx}.club-banner{display:flex;align-items:center;gap:23.0769rpx;width:100%;padding:23.0769rpx;margin-top:30.7692rpx;border:1px solid var(--line-subtle);border-radius:19.2308rpx;background:var(--surface);text-align:left;color:#bdd4c6}.club-banner-copy{display:flex;flex:1;flex-direction:column;gap:5.7692rpx;min-width:0}.club-banner-title{font-size:25rpx;color:var(--text);font-weight:650}.club-banner-detail{font-size:19.2308rpx;color:var(--muted)}.announcement{display:flex;align-items:flex-start;gap:11.5385rpx;color:var(--muted);font-size:19.2308rpx;line-height:32.6923rpx;padding:30.7692rpx 7.6923rpx 0}.announcement>.app-icon{margin-top:3.8462rpx}
@media(max-width:350px){.hero-title{font-size:38.4615rpx;line-height:50rpx}.hero-brand{font-size:38.4615rpx}.hero-publish{width:184.6154rpx;padding:0 19.2308rpx}}
</style>
