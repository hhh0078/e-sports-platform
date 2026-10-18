<!-- 文件作用：严格按玩家页设计稿呈现游戏主视觉、热门游戏入口和两列服务，保留搜索、分类与排序。 -->
<template>
  <AppShell active="index" immersive>
    <view class="hero">
      <image v-if="game.cover_url" class="hero-image" :src="assetUrl(game.cover_url)" mode="aspectFill" />
      <view class="hero-mask" />
      <view class="hero-brand" :style="nativeHeader"><view class="brand-title">{{ state.siteName }}<text> · {{ game.name }}</text></view><button v-if="state.devMode" class="experience" @click="go('/pages/login/index')">本地体验 <Icon name="arrow" :size="11" /></button></view>
      <view class="hero-copy"><view class="hero-title">好队友，<text>即刻集结</text></view><view class="hero-sub">{{ (game.categories || []).slice(0,3).join(' · ') }}</view><button class="hero-button" @click="go('/pages/request/index')">发布需求 <Icon name="arrow" :size="17" color="#07140d" /></button><view class="hero-promise"><Icon name="shield" :size="15"/><text>四方沟通 · 订单留证</text></view></view>
    </view>
    <view class="game-section"><GamePicker catalog /></view>
    <view class="services-section">
      <scroll-view scroll-x :show-scrollbar="false" class="category-scroll"><view class="categories"><button v-for="item in ['全部', ...(game.categories || [])]" :key="item" class="category" :class="{active:category === item}" @click="chooseCategory(item)">{{ item }}</button></view></scroll-view>
      <view class="service-toolbar"><view class="section-title">精选服务</view><view class="service-tools"><button class="search-toggle" @click="searchOpen=!searchOpen"><Icon name="search" :size="16"/>搜索</button><view class="sort-wrap"><button class="sort-control" :aria-expanded="sortOpen" @click="sortOpen=!sortOpen">{{ sortOptions[sortIndex] }} <Icon name="bottom" :size="11" /></button><view v-if="sortOpen" class="sort-dismiss" @click="sortOpen=false"/><view v-if="sortOpen" class="sort-menu"><button v-for="(option,index) in sortOptions" :key="option" :class="{chosen:sortIndex===index}" @click="changeSort(index)">{{ option }}<Icon v-if="sortIndex===index" name="check" :size="14" /></button></view></view></view></view>
      <view v-if="searchOpen" class="search-row"><Icon name="search" :size="17"/><input v-model="query" placeholder="搜索服务、陪练或任务" confirm-type="search" @confirm="loadServices"/><button v-if="query" class="clear-query" @click="clearSearch"><Icon name="close" :size="14" /></button><button class="search-go" @click="loadServices">搜索</button></view>
      <view v-if="error" class="error-box stack"><text>{{ error }}</text><button class="btn btn-small btn-secondary" @click="reload">重新加载</button></view>
      <view v-else-if="loading" class="loading-line">正在寻找好队友…</view>
      <view v-else-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service" /></view>
      <EmptyState v-else title="暂时没有匹配的服务" detail="换个分类试试，或发布需求，让合适的队友找到你。" icon="game"><button class="btn btn-small" @click="go('/pages/request/index')">发布需求</button></EmptyState>
      <view v-if="moreError" class="more-error">{{ moreError }}</view><button v-if="hasMore && !loading && !error" class="load-more btn btn-secondary btn-small" :disabled="loadingMore" @click="loadServices(true)">{{ loadingMore ? '正在加载…' : moreError ? '重试加载' : '加载更多服务' }}</button>
      <button class="club-banner" @click="go('/pages/clubs/index')"><Icon name="club" :size="22"/><view><text>发现俱乐部</text><view>独立接单，也能找到自己的队伍</view></view><Icon name="arrow" :size="16" /></button>
      <view v-if="state.announcement" class="announcement"><Icon name="bell" :size="13"/><text>{{ state.announcement }}</text></view>
    </view>
  </AppShell>
</template>
<script setup>
import { ref, watch, onMounted } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import GamePicker from '../../components/GamePicker.vue'
import ServiceCard from '../../components/ServiceCard.vue'
import EmptyState from '../../components/EmptyState.vue'
import { state, game, init } from '../../lib/store'
import { api } from '../../lib/api'
import { go, assetUrl } from '../../lib/format'
const hasMore = ref(false), loadingMore = ref(false), moreError = ref('')
const services = ref([]), query = ref(''), category = ref('全部'), loading = ref(true), error = ref(''), sortIndex = ref(0), sortOpen = ref(false), searchOpen = ref(false), nativeHeader = ref({})
const sortOptions = ['综合排序', '价格从低到高', '成交优先']
let requestVersion = 0, submittedQuery = ''
/** 根据当前游戏和筛选读取公开服务，忽略过时响应。 */
async function loadServices(append = false) {
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const version = ++requestVersion
  if (adding) loadingMore.value = true
  else { loading.value = true; error.value = ''; hasMore.value = false; submittedQuery = query.value.trim() }
  moreError.value = ''
  try {
    const path = '/services?game_id=' + encodeURIComponent(state.gameId) + '&category=' + encodeURIComponent(category.value === '全部' ? '' : category.value) + '&q=' + encodeURIComponent(submittedQuery) + '&sort=' + ['recommended','price','completed'][sortIndex.value] + '&limit=24&offset=' + (adding ? services.value.length : 0)
    const data = await api(path)
    if (version === requestVersion) { const items = data || []; services.value = adding ? [...services.value,...items] : items; hasMore.value = items.length === 24 }
  } catch (err) { if (version === requestVersion) { if (adding) moreError.value = err.message; else error.value = err.message } }
  finally { if (version === requestVersion) { loading.value = false; loadingMore.value = false } }
}
/** 返回首页时刷新配置，以便管理员改图、主题或上下架立即反映。 */
async function reload() { try { await init(true); if (category.value !== '全部' && !(game.value.categories || []).includes(category.value)) category.value = '全部'; await loadServices() } catch (err) { error.value = err.message; loading.value = false } }
/** 切换服务类型并重新查询。 */
function chooseCategory(value) { category.value = value; loadServices() }
/** 清除搜索词并恢复列表。 */
function clearSearch() { query.value = ''; loadServices() }
/** 应用用户选择的展示排序。 */
function changeSort(index) { sortIndex.value = index; sortOpen.value = false; loadServices() }
/** 为微信系统胶囊预留空间，首页品牌行不绘制系统状态栏。 */
function fitNativeHeader() {
  // #ifdef MP-WEIXIN
  const capsule = uni.getMenuButtonBoundingClientRect(), info = uni.getWindowInfo()
  if (capsule?.width) nativeHeader.value = { paddingTop: capsule.top + 'px', paddingRight: (info.windowWidth - capsule.left + 8) + 'px' }
  // #endif
}
watch(() => state.gameId, () => { category.value = '全部'; query.value = ''; loadServices() })
onMounted(fitNativeHeader)
onShow(reload)
</script>
<style scoped>
.more-error{color:#ffb0a3;font-size:12px;margin-top:14px}.load-more{width:100%;margin-top:15px}.hero{position:relative;height:230px;overflow:hidden;background:var(--bg)}.hero-image{position:absolute;top:0;left:0;width:125%;height:100%}.hero-mask{position:absolute;inset:0;background:linear-gradient(90deg,rgba(0,0,0,.78),rgba(0,0,0,.2) 70%),linear-gradient(0deg,var(--bg),transparent 32%)}.hero-brand{position:relative;z-index:1;display:flex;align-items:center;justify-content:space-between;gap:8px;padding:22px 16px 0;min-height:44px}.brand-title{font-size:18px;font-weight:800;white-space:nowrap}.brand-title text{font-size:13px;letter-spacing:.5px}.experience{margin:0;display:flex;align-items:center;gap:3px;color:#f1f3f2;background:#0005;border:1px solid #ffffff55;border-radius:15px;padding:2px 7px;font-size:10px;line-height:20px;flex-shrink:0}.hero-copy{position:relative;z-index:1;padding:24px 16px 0}.hero-title{font-size:32px;line-height:1.22;font-weight:900;font-style:italic;letter-spacing:-1.6px;text-shadow:0 2px 12px #0006;white-space:nowrap}.hero-title text{color:var(--accent)}.hero-sub{font-size:12px;font-weight:650;letter-spacing:.5px;margin-top:9px;color:#f0f2f0;text-shadow:0 1px 4px #000}.hero-button{display:flex;justify-content:space-between;align-items:center;width:150px;margin:16px 0 0;padding:9px 17px;border-radius:8px;background:var(--accent);color:#07140d;font-size:17px;font-weight:800;line-height:24px}.hero-promise{display:flex;gap:6px;align-items:center;color:#bdc3bf;font-size:11px;margin-top:10px}.game-section{position:relative;border-top:1px solid #ffffff20;border-radius:17px 17px 0 0;background:var(--surface);margin-top:-1px}.services-section{padding:5px 10px 25px;border-top:1px solid #ffffff16;border-radius:17px 17px 0 0;background:var(--bg)}.category-scroll{width:100%}.categories{display:flex;gap:9px;padding:0 1px 2px;white-space:nowrap}.category{margin:0;min-width:77px;flex-shrink:0;padding:5px 13px;color:#bbc1be;background:#ffffff04;border:1px solid #ffffff15;border-radius:9px;font-size:13px;line-height:21px}.category.active{background:var(--accent);border-color:var(--accent);color:#07140d;font-weight:800}.service-toolbar{display:flex;justify-content:space-between;align-items:center;gap:8px;height:33px}.section-title{font-size:13px;font-weight:600;border-left:3px solid var(--accent);padding-left:6px}.service-tools{display:flex;align-items:center;gap:10px;color:#b9c1bc}.search-toggle{margin:0;display:flex;align-items:center;gap:4px;font-size:11px;line-height:24px;color:inherit;background:none;padding:0 9px 0 0;border:0;border-right:1px solid #ffffff22;border-radius:0}.sort-wrap{position:relative}.sort-control{margin:0;display:flex;align-items:center;gap:3px;font-size:11px;line-height:24px;padding:0;color:inherit;background:none;border:0;border-radius:0}.sort-dismiss{position:fixed;inset:0;z-index:25}.sort-menu{position:absolute;right:0;top:29px;z-index:26;width:156px;padding:5px;border:1px solid #ffffff26;border-radius:10px;background:var(--surface);box-shadow:0 8px 26px #0008}.sort-menu button{margin:0;display:flex;align-items:center;justify-content:space-between;width:100%;min-height:40px;padding:8px 10px;border:0;border-radius:6px;font-size:13px;line-height:24px;background:none;color:var(--text);text-align:left}.sort-menu button.chosen{color:var(--accent);background:#ffffff08}.search-row{display:flex;align-items:center;gap:8px;border:1px solid #ffffff26;border-radius:8px;padding:7px 10px;margin-bottom:10px;color:#a7b7ad}.search-row input{flex:1;font-size:13px;height:28px;min-width:0}.search-go,.clear-query{margin:0;border:0;background:none;color:var(--accent);font-size:12px;line-height:28px;padding:0 3px}.card-grid{gap:9px}.club-banner{width:100%;display:flex;align-items:center;gap:12px;margin-top:18px;text-align:left;color:var(--accent);background:var(--surface);border:1px solid #ffffff17;border-radius:12px;padding:14px}.club-banner>view{flex:1}.club-banner text{font-size:14px;font-weight:650;color:var(--text)}.club-banner view view{font-size:11px;color:#9aa89f;margin-top:3px}.announcement{display:flex;gap:6px;align-items:flex-start;font-size:11px;color:#9ca9a1;margin:16px 2px 0}@media(max-width:350px){.hero-title{font-size:27px}.hero-sub{font-size:10px}.hero-brand{padding-left:13px}.brand-title{font-size:16px}.brand-title text{font-size:11px}.hero-copy{padding-left:13px}.experience{margin:0;font-size:9px}}@media(min-width:500px){.hero{height:280px}.hero-copy{padding-top:40px}.hero-title{font-size:42px}.hero-sub{font-size:15px}}
/* #ifdef MP-WEIXIN */
.hero{height:272px}.hero-brand{padding-top:44px}.experience{margin:0;display:none}.hero-copy{padding-top:27px}
/* #endif */
</style>
