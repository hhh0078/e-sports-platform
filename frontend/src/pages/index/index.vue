<!-- 文件作用：玩家首页，提供游戏切换、服务筛选和下单入口。 -->
<template>
  <AppShell active="index">
    <view class="hero"><image v-if="game.cover_url" class="hero-image" :src="assetUrl(game.cover_url)" mode="aspectFill"/><view class="hero-mask"/><view class="hero-copy"><view class="eyebrow"><text class="status-dot"/> {{ game.short_name || game.name }} · 一起上场</view><view class="hero-title"><view>你的下一局，</view><text>有人一起赢。</text></view><view class="hero-sub">{{ game.subtitle || '找到默契队友，让每一局都值得。' }}</view><button class="hero-button" @click="go('/pages/request/index')">发布我的需求 <Icon name="arrow" :size="13" color="#05180e" /></button></view><view class="hero-foot"><text>自由接单 · 不限俱乐部</text><view class="hero-dots"><view class="dot current"/><view class="dot"/><view class="dot"/></view></view></view>
    <view class="promise-row"><view><Icon name="shield" :size="13"/><text>订单全程留证</text></view><view><Icon name="chat" :size="13"/><text>四方沟通保障</text></view><view><Icon name="wallet" :size="13"/><text>售后平台介入</text></view></view>
    <view class="section-row game-heading"><view class="section-title">选择你的战场</view><text class="eyebrow">PICK YOUR GAME</text></view>
    <GamePicker />
    <view class="search-row"><Icon name="search" :size="17" color="#6d8275"/><input v-model="query" placeholder="搜服务、陪练、任务…" placeholder-style="color:#697c70" confirm-type="search" @confirm="loadServices"/><view v-if="query" @click="clearSearch"><Icon name="close" :size="14" /></view><button class="search-go" @click="loadServices">搜索</button></view>
    <scroll-view scroll-x :show-scrollbar="false" class="category-scroll"><view class="categories"><view v-for="item in ['全部', ...(game.categories || [])]" :key="item" class="category" :class="{active:category === item}" @click="chooseCategory(item)">{{ item }}</view></view></scroll-view>
    <view class="section-row services-heading"><view><text class="section-title">精选服务</text><text class="service-total">{{ services.length }} 个选择</text></view><picker :range="sortOptions" @change="changeSort"><view class="sort-control">{{ sortOptions[sortIndex] }} <Icon name="bottom" :size="10" /></view></picker></view>
    <view v-if="error" class="error-box stack"><text>{{ error }}</text><button class="btn btn-small btn-secondary" @click="reload">重新加载</button></view>
    <view v-else-if="loading" class="loading-line">正在集结好队友…</view>
    <view v-else-if="sorted.length" class="card-grid"><ServiceCard v-for="service in sorted" :key="service.id" :service="service" /></view>
    <EmptyState v-else title="这个战场，等你先开局" detail="暂时没有匹配的服务。发布需求，让合适的队友找到你。" icon="game"><button class="btn btn-small" @click="go('/pages/request/index')">发布需求</button></EmptyState>
    <view class="club-banner" @click="go('/pages/clubs/index')"><view class="club-banner-icon"><Icon name="club" :size="26" /></view><view class="club-banner-copy"><view>个人出发，也能找到组织</view><text>发现俱乐部 · 让实力多一份认证</text></view><Icon name="arrow" :size="16" /></view>
    <view v-if="state.announcement" class="announcement"><Icon name="bell" :size="13"/><text>{{ state.announcement }}</text></view>
    <view class="brand-footer"><text>JIJIE · BETTER TOGETHER</text><view>找队友，也成为队友</view></view>
  </AppShell>
</template>
<script setup>
import { ref, computed, watch } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import GamePicker from '../../components/GamePicker.vue'
import ServiceCard from '../../components/ServiceCard.vue'
import EmptyState from '../../components/EmptyState.vue'
import { state, game, init } from '../../lib/store'
import { api } from '../../lib/api'
import { go, assetUrl } from '../../lib/format'
const services = ref([]), query = ref(''), category = ref('全部'), loading = ref(true), error = ref(''), sortIndex = ref(0)
const sortOptions = ['综合推荐', '价格从低到高', '成交优先']
let requestVersion = 0
const sorted = computed(() => { const result = [...services.value]; if (sortIndex.value === 1) result.sort((a,b) => a.price_cents - b.price_cents); if (sortIndex.value === 2) result.sort((a,b) => b.completed_count - a.completed_count); return result })
/** 根据当前游戏和筛选读取公开服务，忽略过时响应。 */
async function loadServices() { const version = ++requestVersion; loading.value = true; error.value = ''; try { const data = await api('/services?game_id=' + encodeURIComponent(state.gameId) + '&category=' + encodeURIComponent(category.value === '全部' ? '' : category.value) + '&q=' + encodeURIComponent(query.value.trim())); if (version === requestVersion) services.value = data || [] } catch (err) { if (version === requestVersion) error.value = err.message } finally { if (version === requestVersion) loading.value = false } }
/** 初始化数据并刷新当前可购买服务。 */
async function reload() { try { await init(); await loadServices() } catch (err) { error.value = err.message; loading.value = false } }
/** 切换服务类型并重新查询。 */
function chooseCategory(value) { category.value = value; loadServices() }
/** 清除搜索词并恢复列表。 */
function clearSearch() { query.value = ''; loadServices() }
/** 应用用户选择的展示排序。 */
function changeSort(event) { sortIndex.value = Number(event.detail.value) }
watch(() => state.gameId, () => { category.value = '全部'; query.value = ''; loadServices() })
onShow(reload)
</script>
<style scoped>
.hero{position:relative;height:263px;overflow:hidden;border-radius:16px;border:1px solid rgba(255,255,255,.09);background:#162b22}.hero-image{position:absolute;inset:0;width:100%;height:100%;object-position:60% center}.hero-mask{position:absolute;inset:0;background:linear-gradient(90deg,rgba(4,15,11,.94) 0%,rgba(4,15,11,.7) 40%,rgba(4,15,11,.12) 85%),linear-gradient(0deg,rgba(4,15,11,.85),transparent 40%)}.hero-copy{position:relative;padding:28px 21px;z-index:1}.hero-copy .eyebrow{font-size:9px;letter-spacing:1px;display:flex;align-items:center;gap:5px}.hero-title{font-size:29px;line-height:1.3;font-weight:850;letter-spacing:-1.3px;margin:13px 0 10px}.hero-title text{color:var(--accent)}.hero-sub{font-size:10px;color:#a7bbae;max-width:230px}.hero-button{display:flex;align-items:center;justify-content:space-between;gap:17px;background:var(--accent);color:#06180e;margin:19px 0 0;font-size:11px;line-height:20px;font-weight:750;border-radius:6px;padding:8px 13px;width:142px}.hero-foot{position:absolute;left:21px;right:18px;bottom:13px;display:flex;justify-content:space-between;align-items:center;font-size:8px;color:#708b7b;letter-spacing:.4px}.hero-dots{display:flex;align-items:center;gap:4px}.dot{width:4px;height:4px;background:#77817a;border-radius:50%}.dot.current{background:var(--accent);width:14px;border-radius:3px}.promise-row{display:flex;justify-content:space-between;padding:15px 2px 0;color:#71897b;font-size:8px}.promise-row>view{display:flex;align-items:center;gap:4px}.game-heading{margin-top:28px;margin-bottom:0}.game-heading .eyebrow{font-size:8px;letter-spacing:1px;color:#647f6d}.search-row{display:flex;align-items:center;gap:9px;background:var(--surface);border:1px solid rgba(255,255,255,.06);border-radius:9px;padding:10px 12px;margin:16px 0}.search-row input{flex:1;font-size:11px;height:22px;min-width:0}.search-go{border:0;background:none;color:#a1b7a8;font-size:10px;line-height:20px;padding:0 0 0 7px}.categories{display:flex;gap:9px;padding:2px 0 4px;white-space:nowrap}.category{padding:7px 13px;color:#8a9e91;background:rgba(255,255,255,.03);border:1px solid rgba(255,255,255,.07);border-radius:7px;font-size:11px;flex-shrink:0}.category.active{background:var(--accent);border-color:var(--accent);color:#06170e;font-weight:750}.services-heading{margin-top:23px;margin-bottom:14px}.service-total{font-size:9px;color:#5f7869;margin-left:8px}.sort-control{display:flex;align-items:center;gap:4px;font-size:9px;color:#91a296}.club-banner{display:flex;align-items:center;gap:13px;margin-top:25px;background:var(--surface);border:1px solid rgba(255,255,255,.1);border-radius:13px;padding:18px 14px;color:var(--accent)}.club-banner-icon{width:38px;height:38px;border-radius:10px;display:flex;align-items:center;justify-content:center;background:rgba(255,255,255,.04)}.club-banner-copy{flex:1}.club-banner-copy view{font-size:12px;font-weight:650;color:#d7e2d9}.club-banner-copy text{font-size:9px;color:#809789;margin-top:5px;display:block}.announcement{display:flex;align-items:flex-start;gap:6px;font-size:10px;color:#819589;margin-top:19px}.brand-footer{text-align:center;margin-top:34px;color:#405e4c;font-size:8px;letter-spacing:1.4px}.brand-footer view{font-size:9px;letter-spacing:2px;margin-top:5px;color:#536e5e}@media(min-width:600px){.hero{height:320px}.hero-copy{padding:38px 32px}.hero-title{font-size:40px}.hero-sub{font-size:13px}.hero-button{margin-top:25px;width:165px;font-size:13px}.hero-foot{left:32px}.hero-image{object-position:center}}
</style>
