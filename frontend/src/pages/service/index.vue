<!-- 文件作用：服务详情、真实评价与创建订单。 -->
<template><AppShell active="index" title="遇见你的默契队友" subtitle="明确需求，再一起出发。" back>
  <view v-if="error" class="error-box stack"><text>{{ error }}</text><button class="btn btn-secondary" @click="load">重试</button></view>
  <view v-else-if="loading" class="loading-line">正在读取服务…</view>
  <template v-else-if="service">
    <view class="detail-cover"><CoverArt :src="service.cover_url" :labels="service.labels" /></view>
    <view class="detail-main"><view class="row between"><view class="row"><text class="pill pill-accent">{{ service.category }}</text><text v-if="service.club_verified" class="pill">俱乐部认证</text></view><text class="muted">已完成 {{ service.completed_count || 0 }} 单</text></view><view class="detail-title">{{ service.title }}</view><view class="price-row"><text class="detail-price">¥{{ money(service.price_cents) }}</text><text class="muted">/ {{ service.unit || '单' }}</text><view class="rating"><Icon name="star" :size="12" color="#d5ba7e"/> {{ Number(service.rating || 0).toFixed(1) }}</view></view></view>
    <view class="panel provider-panel"><view class="avatar">{{ (service.provider_name || '队').slice(0,1) }}</view><view class="provider-info"><view>{{ service.provider_name }}</view><text>{{ service.club_name || '独立接单 · 自由队友' }}</text></view><button v-if="service.club_id" class="btn btn-small btn-secondary" @click="go('/pages/club/index?id=' + service.club_id)">俱乐部</button><Icon v-else name="shield" :size="21" /></view>
    <view class="section-row"><text class="section-title">服务说明</text><text class="eyebrow">SERVICE</text></view><view class="description panel">{{ service.description }}</view><view class="row tags"><text v-for="tag in service.tags || []" :key="tag" class="pill">{{ tag }}</text></view>
    <view class="section-row"><text class="section-title">下单约定</text></view><view class="notice">请写清区服、目标、时间和验收标准。不要提供游戏账号密码。订单内沟通与交付记录会保留，发生争议可申请平台介入。</view>
    <template v-if="service.status === 'active' && service.provider_id !== state.user?.id"><view class="panel order-form"><view class="field"><text class="field-label">购买数量</text><view class="quantity"><button @click="changeQuantity(-1)" :disabled="quantity <= 1">−</button><text>{{ quantity }}</text><button @click="changeQuantity(1)" :disabled="quantity >= 99">＋</button><text class="muted">{{ service.unit || '单' }}</text></view></view><view class="field"><text class="field-label">我的需求与验收标准</text><textarea v-model="requirements" class="input" maxlength="1000" placeholder="例如：今晚 8 点，安卓 QQ 区，双排陪练 1 小时，希望有语音讲解。"/><text class="hint">{{ requirements.length }}/1000 · 请在订单聊天室确认细节</text></view><view class="row between checkout"><view><text class="muted">合计</text><text class="total">¥{{ money(service.price_cents * quantity) }}</text></view><button class="btn" :disabled="submitting" @click="createOrder">{{ submitting ? '正在创建…' : state.user ? '确认下单' : '登录后下单' }}<Icon name="arrow" :size="14" color="#07170e" /></button></view></view></template>
    <view v-else class="notice mt">{{ service.provider_id === state.user?.id ? '这是你发布的服务，可以前往打手工作台管理。' : '该服务暂未上架，无法下单。' }}</view>
    <view class="section-row"><text class="section-title">玩家评价</text><text class="muted">{{ reviews.length }} 条</text></view><view v-if="reviews.length" class="stack"><view v-for="review in reviews" :key="review.id" class="panel"><view class="row between"><text>{{ review.buyer_name || '玩家' }}</text><text class="accent">{{ review.rating }} / 5</text></view><view class="review-content">{{ review.content }}</view><text class="hint">{{ time(review.created_at) }}</text></view></view><EmptyState v-else title="期待第一份真实评价" detail="仅完成订单的玩家可以评价。" icon="star"/>
  </template>
</AppShell></template>
<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import CoverArt from '../../components/CoverArt.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, init, selectGame } from '../../lib/store'
import { api } from '../../lib/api'
import { go, money, notify, time } from '../../lib/format'
const service = ref(null), reviews = ref([]), loading = ref(true), error = ref(''), quantity = ref(1), requirements = ref(''), submitting = ref(false)
let id = ''
/** 读取服务详情，并切到该服务的游戏主题。 */
async function load() { loading.value = true; error.value = ''; try { await init(); service.value = await api('/services/' + encodeURIComponent(id)); reviews.value = service.value.reviews || []; selectGame(service.value.game_id) } catch (err) { error.value = err.message } finally { loading.value = false } }
/** 保持购买数量在服务端允许的范围内。 */
function changeQuantity(amount) { quantity.value = Math.max(1, Math.min(99, quantity.value + amount)) }
/** 从服务端创建真实订单，金额以服务端计算结果为准。 */
async function createOrder() { if (!state.user) return go('/pages/login/index?next=' + encodeURIComponent('/pages/service/index?id=' + id)); if (!requirements.value.trim()) return notify('请填写需求与验收标准'); if (submitting.value) return; submitting.value = true; try { const order = await api('/orders', 'POST', { service_id: id, quantity: quantity.value, requirements: requirements.value.trim() }); go('/pages/order/index?id=' + order.id) } catch (err) { notify(err) } finally { submitting.value = false } }
onLoad(options => { id = options.id || ''; load() })
</script>
<style scoped>.detail-cover{overflow:hidden;border-radius:16px;border:1px solid rgba(255,255,255,.1)}.detail-main{padding:22px 0}.detail-title{font-size:25px;line-height:1.4;font-weight:750;margin-top:14px}.price-row{display:flex;align-items:baseline;gap:8px;margin-top:15px}.detail-price{font-size:34px;font-weight:800;color:var(--accent);letter-spacing:-1px}.rating{margin-left:auto;color:#d5ba7e;font-size:12px}.provider-panel{display:flex;gap:12px;align-items:center}.provider-info{flex:1}.provider-info view{font-weight:650;font-size:14px}.provider-info text{color:#94a599;font-size:11px}.description{white-space:pre-wrap;line-height:1.9;font-size:13px;color:#b6c6bb}.tags{margin-top:12px;flex-wrap:wrap}.order-form{margin-top:17px}.quantity{display:flex;align-items:center;gap:22px}.quantity button{width:34px;height:34px;border:1px solid rgba(255,255,255,.1);border-radius:8px;background:transparent;color:var(--text);line-height:30px;padding:0}.quantity button[disabled]{opacity:.3}.quantity>text:not(.muted){font-weight:700}.checkout{border-top:1px solid rgba(255,255,255,.08);padding-top:18px;gap:15px}.checkout>.btn{font-size:12px;padding:12px 16px}.total{font-size:25px;color:var(--accent);font-weight:750;margin-left:6px}.review-content{margin:12px 0;line-height:1.8;color:#b8c5bd;font-size:13px}</style>
