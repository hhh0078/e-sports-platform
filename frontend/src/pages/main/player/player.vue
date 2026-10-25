<!-- 文件作用：玩家常驻视图展示游戏选择、搜索排序和统一服务卡片，按设计图实现。 -->
<template>
  <view class="player-content">
    <view class="player-hero"><view class="hero-visual" :style="carouselHeight>0?{minHeight:carouselHeight+'px'}:{}"><ContentCarousel :slides="publicContent?.slides||[]" :fallback="publicContent?'':game.cover_url" :active="props.active" :geometry="carouselGeometry" @height-change="setCarouselHeight" @caption-change="hasSlideText=$event"/><view class="hero-mask"/><view class="hero-copy"><view class="hero-text"><view class="hero-brand-row"><text class="hero-brand">开机电竞</text><ShareEntry :active="props.active" kind="home" :game-id="state.gameId" label="分享"/></view><text class="hero-title" :class="{'hero-caption-hidden':hasSlideText}">开机即组队，一起赢</text><text class="hero-sub" :class="{'hero-caption-hidden':hasSlideText}">{{ game.name }} · {{ (game.categories || []).slice(0, 2).join(' · ') }}</text></view><button v-if="!timelinePreview" class="btn btn-small hero-publish" @click="go('/pages/requests/publish')">发布需求<Icon name="arrow" :size="16"/></button></view></view><ContentTip class="hero-public-tip" :tip="publicContent?.page_tips.player"/></view>
    <view class="game-section"><GamePicker :active="props.active" /></view>
    <view class="services-section">
      <scroll-view scroll-x :show-scrollbar="false" class="category-scroll"><view class="categories"><button v-for="item in ['全部', ...(game.categories || [])]" :key="item" :class="['category', { active: category === item }]" @click="chooseCategory(item)">{{ item }}</button></view></scroll-view>
      <view class="service-toolbar"><text class="section-title">精选服务</text><view class="service-tools"><button class="text-btn muted" @click="searchOpen = !searchOpen"><Icon name="search" :size="16" />搜索</button><picker :range="sortOptions" :value="sortIndex" @change="changeSort(Number($event.detail.value))"><view class="sort-control">{{ sortOptions[sortIndex] }}<Icon name="bottom" :size="12" /></view></picker></view></view>
      <view v-if="searchOpen" class="search-row"><Icon name="search" :size="17" /><input class="search-input" v-model="query" placeholder="搜索服务、陪练或任务" confirm-type="search" @confirm="loadServices()" /><button v-if="query" class="text-btn muted" @click="clearSearch"><Icon name="close" :size="14" /></button><button class="text-btn" @click="loadServices()">搜索</button></view>
      <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="reload">重新加载</button></view><view v-else-if="loading && !hasLoaded && !services.length" class="loading-line">正在寻找好队友…</view><view v-else-if="services.length" class="card-grid"><ServiceCard v-for="service in services" :key="service.id" :service="service" :active="props.active" /></view><EmptyState v-else title="暂时没有匹配的服务" detail="换个分类试试，或发布需求。" icon="game"><button v-if="!timelinePreview" class="btn btn-small" @click="go('/pages/requests/publish')">发布需求</button></EmptyState>
      <ListLoadState v-if="!error&&(services.length||moreError)" :loading="loadingMore" :has-more="hasMore" :error="moreError" @retry="retryList"/>
      <button class="club-banner" @click="go('/pages/community/clubs/index')"><Icon name="club" :size="23" /><view class="club-banner-copy"><text class="club-banner-title">发现俱乐部</text><text class="club-banner-detail">找到合拍的队伍</text></view><Icon name="arrow" :size="15" /></button><view v-if="state.announcement" class="announcement"><Icon name="bell" :size="13" /><text>{{ state.announcement }}</text></view>
    </view>
  </view>
</template>
<script setup lang="ts">
import { ref,watch,nextTick,getCurrentInstance,onMounted,onBeforeUnmount } from 'vue'
import Icon from '../../../components/Icon.vue'
import ContentCarousel from '../../../components/ContentCarousel.vue'
import { useViewportWidth } from '../../../lib/viewport'
import { readSafeArea } from '../../../lib/safe-area'
import { designPixels } from '../../../lib/design-units'
import type { CarouselGeometry } from '../../../lib/carousel-layout'
import ContentTip from '../../../components/ContentTip.vue'
import { publicContent } from '../../../lib/public-content'
import ShareEntry from '../../../components/ShareEntry.vue'
import { isTimelinePreview } from '../../../lib/sharing'
import GamePicker from '../../../components/GamePicker.vue'
import ServiceCard from '../../../components/ServiceCard.vue'
import EmptyState from '../../../components/EmptyState.vue'
import ListLoadState from '../../../components/ListLoadState.vue'
import { appendUniqueRows } from '../../../lib/list-page'
import { state, game, init } from '../../../lib/store'
import { listServices } from '../../../api/services.api'
import { errorMessage } from '../../../api/client'
import { go } from '../../../lib/format'
import { useMainVisibility, refreshMainList } from '../../../lib/navigation'
import type { Service } from '../../../types/models'
const hasSlideText=ref(false),carouselHeight=ref(0),carouselGeometry=ref<CarouselGeometry|null>(null)
const carouselViewport=useViewportWidth(),carouselInstance=getCurrentInstance()
let carouselMounted=false,carouselMeasureQueued=false,carouselMeasureVersion=0
const timelinePreview=isTimelinePreview()
const props = defineProps<{ active: boolean }>()
/** 查询上下文变化使父容器撤销旧刷新显示，不等待被替代的网络请求。 */
const refreshKey=ref(0)
const hasMore = ref(false), loadingMore = ref(false), moreError = ref(''), services = ref<Service[]>([]), query = ref(''), category = ref('全部'), loading = ref(false), error = ref(''), sortIndex = ref(0), searchOpen = ref(false)
/** 已显示空态在返回刷新时保留，只有新游戏、分类或搜索才重新显示首次加载行。 */
const hasLoaded = ref(false)
const sortOptions = ['综合排序', '价格从低到高', '成交优先']
const sortValues = ['recommended', 'price', 'completed']
let requestVersion = 0, submittedQuery = '', visible = false, nextOffset = 0, failedAppend = false
/** 查询公开服务并分页，切换游戏或隐藏视图后丢弃旧请求。 */
async function loadServices(append = false, preserve = false): Promise<void> {
  if (!visible) return
  const adding = append === true
  if (adding && (loading.value || loadingMore.value || !hasMore.value)) return
  const version = ++requestVersion
  if (adding) loadingMore.value = true
  else { loading.value = true; error.value = ''; if (!preserve) { refreshKey.value++; nextOffset = 0; hasMore.value = false; hasLoaded.value = false; submittedQuery = query.value.trim() } }
  moreError.value = ''
  const current = () => visible && version === requestVersion
  /** 使用同一筛选读取指定分页，返回时可以恢复此前全部已加载数据。 */
  const read = (limit: number, offset: number) => listServices({ game_id: state.gameId, category: category.value === '全部' ? '' : category.value, q: submittedQuery, sort: sortValues[sortIndex.value], limit, offset })
  try {
    if (preserve && services.value.length) { const result = await refreshMainList(Math.max(nextOffset, services.value.length), 24, read, current); if (result) { nextOffset = result.items.length; services.value = appendUniqueRows([], result.items); hasMore.value = result.hasMore } }
    else { const offset = adding ? nextOffset : 0, items = await read(24, offset); if (current()) { nextOffset = offset + items.length; services.value = appendUniqueRows(adding ? services.value : [], items); hasMore.value = items.length === 24 } }
    if (current()) hasLoaded.value = true
  }
  catch (err) { if (version === requestVersion) { if (adding || (preserve && (hasLoaded.value || services.value.length))) {failedAppend=adding;moreError.value = errorMessage(err);} else error.value = errorMessage(err) } }
  finally { if (version === requestVersion) { loading.value = false; loadingMore.value = false } }
}
/** 返回主视图同步可配置主题及服务列表。 */
async function reload(): Promise<void> { const version = ++requestVersion; try { await init(true,isTimelinePreview()); if (!visible || version !== requestVersion) return; if (category.value !== '全部' && !game.value.categories.includes(category.value)) category.value = '全部'; await loadServices(false, true) } catch (err) { if (visible && version === requestVersion) { if (hasLoaded.value || services.value.length) {failedAppend=false;moreError.value = errorMessage(err);} else error.value = errorMessage(err); loading.value = false } } }
/** 选择分类时从第一页重新查询。 */
function chooseCategory(value: string): void { category.value = value; void loadServices() }
/** 清除搜索并恢复当前游戏全部服务。 */
function clearSearch(): void { query.value = ''; void loadServices() }
/** 使用官方选择器更新排序。 */
function changeSort(index: number): void { sortIndex.value = Math.max(0, Math.min(index, sortOptions.length - 1)); void loadServices() }
/** 隐藏后停止在途列表更新。 */
function hidePage(): void { carouselMeasureVersion++;visible = false; requestVersion++; loading.value = false; loadingMore.value = false }
/** 激活时刷新已加载范围，保留当前筛选及主容器记录的滚动位置。 */
function showPage(): void { visible = true; void reload();measureCarousel() }
watch(() => state.gameId, () => { category.value = '全部'; query.value = ''; services.value = []; nextOffset = 0; hasMore.value = false; hasLoaded.value = false; if (visible) void loadServices() })
/** 下拉仅刷新当前筛选，保留已加载范围与原生滚动几何。 */
async function refresh():Promise<void>{if(!visible||loading.value||loadingMore.value)return;await reload()}
/** 触底只追加下一页，已有请求或无更多时由列表方法直接忽略。 */
async function loadMore():Promise<void>{if(!visible||loading.value||loadingMore.value)return;if(moreError.value&&!failedAppend)await reload();else await loadServices(true)}
/** 失败重试区分首屏范围刷新与追加失败，恢复后不清空已有卡片。 */
async function retryList():Promise<void>{await loadMore()}

/** 原生选择器返回值按真实矩形安全缩窄，不信任缺失或隐藏时的零宽数据。 */
function carouselRect(value:unknown):{width:number;top:number;bottom:number}|null{if(!value||typeof value!=='object')return null;const row=value as Record<string,unknown>;return typeof row.width==='number'&&Number.isFinite(row.width)&&row.width>0&&typeof row.top==='number'&&Number.isFinite(row.top)&&typeof row.bottom==='number'&&Number.isFinite(row.bottom)?{width:row.width,top:row.top,bottom:row.bottom}:null}
/** 只增加原图区域的稳定高度，三秒切换不会重排下面分类、搜索或服务列表。 */
function setCarouselHeight(height:number):void{if(Number.isFinite(height)&&height>=0&&Math.abs(carouselHeight.value-height)>.1)carouselHeight.value=height}
/** 显示及配置/视口变化合并一次测量，不订阅滚动或轮播索引。 */
function measureCarousel():void{carouselMeasureVersion++;if(!carouselMounted||!visible||!props.active||!carouselInstance?.proxy||carouselMeasureQueued)return;carouselMeasureQueued=true;void nextTick(()=>{carouselMeasureQueued=false;const current=carouselMeasureVersion;if(!carouselMounted||!visible||!props.active||!carouselInstance?.proxy)return;uni.createSelectorQuery().in(carouselInstance.proxy).select('.hero-visual').boundingClientRect().select('.hero-brand-row').boundingClientRect().select('.hero-publish').boundingClientRect().exec((values:unknown[])=>{if(!carouselMounted||!visible||!props.active||current!==carouselMeasureVersion)return;const hero=carouselRect(values[0]);if(!hero)return;const brand=carouselRect(values[1]),button=carouselRect(values[2]),windowWidth=carouselViewport.value,safe=readSafeArea(),baseHeight=designPixels(144,windowWidth)+safe.statusBarHeight+safe.capsuleSpace,reserveTop=Math.max(0,(brand?.bottom??hero.top)-hero.top,(button?.bottom??hero.top)-hero.top)+designPixels(8,windowWidth);carouselGeometry.value={width:hero.width,baseHeight,reserveTop,reserveBottom:designPixels(20,windowWidth)}})})}
watch([()=>props.active,()=>state.gameId,()=>game.value,()=>publicContent.value?.revision,carouselViewport],measureCarousel,{flush:'post'})
onMounted(()=>{carouselMounted=true;measureCarousel()})
onBeforeUnmount(()=>{carouselMounted=false;carouselMeasureVersion++})

defineExpose({refresh,loadMore,refreshKey})
useMainVisibility(() => props.active, showPage, hidePage)
</script>
<style scoped>
.hero-brand-row{display:flex;align-items:center;gap:23.0769rpx;min-width:0}.player-content{padding-bottom:46.1538rpx}.player-hero{position:relative;isolation:isolate;min-height:calc(276.9231rpx + var(--status-bar-height, 0px) + var(--capsule-space, 0px));overflow:hidden}.hero-image{position:absolute;inset:0;width:100%;height:100%;z-index:-2}.hero-mask{position:absolute;inset:0;z-index:-1;background:linear-gradient(90deg,#07100dd9,#07100d18),linear-gradient(0deg,var(--bg),transparent 50%)}.hero-copy{display:flex;align-items:center;gap:23.0769rpx;padding:calc(var(--status-bar-height, 0px) + var(--capsule-space, 0px) + 15.3846rpx) 30.7692rpx 30.7692rpx;min-width:0}.hero-text{display:flex;flex:1;flex-direction:column;gap:7.6923rpx;min-width:0}.hero-brand{font-size:42.3077rpx;line-height:46.1538rpx;font-weight:800;letter-spacing:1.9231rpx;color:#fff}.hero-title{font-size:46.1538rpx;line-height:57.6923rpx;font-weight:800;letter-spacing:-0.9615rpx;color:#fff}.hero-sub{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;overflow-wrap:anywhere;color:#c3d0c8;font-size:21.1538rpx;line-height:34.6154rpx}.hero-publish{flex-shrink:0;width:196.1538rpx;min-height:69.2308rpx;height:69.2308rpx;padding:0 23.0769rpx;border-radius:38.4615rpx;font-size:23.0769rpx;gap:7.6923rpx;white-space:nowrap}.game-section{margin:0 23.0769rpx;border:1px solid var(--line-subtle);background:var(--surface);border-radius:23.0769rpx;overflow:hidden}.services-section{padding:30.7692rpx 30.7692rpx 0}.category-scroll{width:100%;white-space:nowrap}.categories{display:inline-flex;gap:15.3846rpx;padding-bottom:7.6923rpx;min-width:100%}.category{display:flex;align-items:center;justify-content:center;min-height:57.6923rpx;flex-shrink:0;padding:0 23.0769rpx;border:1px solid var(--line);border-radius:34.6154rpx;background:transparent;font-size:21.1538rpx;color:var(--muted)}.category.active{background:var(--accent);border-color:var(--accent);color:#092416;font-weight:650}.service-toolbar{display:flex;justify-content:space-between;gap:15.3846rpx;align-items:center;min-width:0;margin:30.7692rpx 0 23.0769rpx}.service-tools{display:flex;align-items:center;gap:15.3846rpx;min-width:0}.sort-control{display:flex;align-items:center;gap:7.6923rpx;font-size:19.2308rpx;color:var(--muted);min-height:53.8462rpx;white-space:nowrap}.search-row{display:flex;align-items:center;gap:15.3846rpx;background:var(--surface);border:1px solid var(--line);border-radius:19.2308rpx;padding:11.5385rpx 19.2308rpx;margin-bottom:23.0769rpx;min-width:0}.search-input{flex:1;font-size:23.0769rpx;min-width:0}.load-more{width:100%;margin-top:23.0769rpx}.club-banner{display:flex;align-items:center;gap:23.0769rpx;width:100%;padding:23.0769rpx;margin-top:30.7692rpx;border:1px solid var(--line-subtle);border-radius:19.2308rpx;background:var(--surface);text-align:left;color:#bdd4c6}.club-banner-copy{display:flex;flex:1;flex-direction:column;gap:5.7692rpx;min-width:0}.club-banner-title{font-size:25rpx;color:var(--text);font-weight:650}.club-banner-detail{font-size:19.2308rpx;color:var(--muted)}.announcement{display:flex;align-items:flex-start;gap:11.5385rpx;color:var(--muted);font-size:19.2308rpx;line-height:32.6923rpx;padding:30.7692rpx 7.6923rpx 0}.announcement>.app-icon{margin-top:3.8462rpx}
@media(max-width:350px){.hero-title{font-size:38.4615rpx;line-height:50rpx}.hero-brand{font-size:38.4615rpx}.hero-publish{width:184.6154rpx;padding:0 19.2308rpx}}
.hero-visual{position:relative;isolation:isolate;min-height:calc(276.9231rpx + var(--status-bar-height,0px) + var(--capsule-space,0px))}.hero-mask{z-index:1;pointer-events:none}.hero-copy{position:relative;z-index:3;pointer-events:none}.hero-brand-row,.hero-publish{pointer-events:auto}.hero-caption-hidden{visibility:hidden}.hero-public-tip{position:relative;margin:0 23.0769rpx 15.3846rpx}
</style>
