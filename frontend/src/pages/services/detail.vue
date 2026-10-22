<!-- 文件作用：以沉浸封面展示服务，紧凑呈现作者、约定和真实下单，文字保持独立图层。 -->
<template>
  <AppShell title="服务详情" immersive :game-id="service?.game_id">
    <view v-if="error" class="detail-state error-box stack"><text>{{ error }}</text><button class="btn-secondary" @click="load">重新加载</button></view>
    <view v-else-if="loading" class="detail-state loading-line">正在读取服务…</view>
    <view v-else-if="service" class="service-content" :class="{'with-checkout':service.status==='active'&&service.provider_id!==state.user?.id}">
      <CoverArt :src="service.cover_url" :labels="service.labels" :aspect-ratio="4/3" :reserve-bottom-pixels="coverReserveBottom" :reserve-top-pixels="coverReserveTop" :min-label-area-pixels="coverMinLabelArea" :shade="false" readable :min-font-pixels="14">
        <view class="detail-gradient" />
        <view class="detail-cover-info"><text class="detail-title">{{ service.title }}</text><view class="detail-labels"><text class="detail-label">{{ service.category }}</text><view v-if="service.club_verified" class="detail-cert"><Icon name="shield" :size="12"/><text>俱乐部认证</text></view></view><view v-if="providerDepositCents||service.provider_club_deposit_cents" class="detail-deposits"><DepositBadge :amount-cents="providerDepositCents" show-amount/><DepositBadge kind="user_club" :amount-cents="service.provider_club_deposit_cents" show-amount/></view></view>
      </CoverArt>
      <view class="detail-body">
        <view class="price-row"><view class="price-main"><text class="currency">¥</text><text class="detail-price">{{ money(service.price_cents) }}</text><text class="price-unit">/{{ service.unit || '单' }}</text></view><view class="sales"><text>已成交 {{ service.completed_count || 0 }}</text><view class="rating"><Icon name="star" :size="11"/><text>{{ Number(service.rating || 0).toFixed(1) }}</text></view></view></view>
        <view class="provider-strip"><button class="provider-link" @click="go('/pages/community/players/detail?id='+service.provider_id)"><view class="provider-avatar"><image v-if="service.provider_avatar" :src="assetUrl(service.provider_avatar)" mode="aspectFill"/><Icon v-else name="user" :size="18"/></view><text class="provider-name">{{ service.provider_name }}</text><Icon v-if="service.club_verified" name="shield" :size="13"/><text class="provider-view">查看主页</text><Icon name="arrow" :size="12"/></button><button v-if="service.club_id" class="club-link text-btn" @click="go('/pages/community/clubs/detail?id='+service.club_id)">俱乐部</button></view>
        <view class="content-section"><text class="section-title">服务说明</text><text class="description">{{ service.description }}</text><view class="service-traits"><text v-for="tag in service.tags||[]" :key="tag" class="trait">{{ tag }}</text></view></view>
        <template v-if="service.status==='active' && service.provider_id!==state.user?.id">
          <view class="content-section order-form"><text class="section-title">下单信息</text><view class="quantity-row"><text class="field-label">数量（{{ service.unit||'单' }}）</text><view class="quantity"><button aria-label="减少数量" @click="changeQuantity(-1)" :disabled="quantity<=1">−</button><text>{{ quantity }}</text><button aria-label="增加数量" @click="changeQuantity(1)" :disabled="quantity>=99">＋</button></view></view><view class="field"><text class="field-label">需求与验收标准</text><textarea v-model="requirements" class="input" maxlength="1000" placeholder="填写区服、预约时间、服务目标与验收标准"/><text class="hint">{{ requirements.length }}/1000 · 请在订单聊天室确认细节</text></view><text class="transaction-hint">订单内保留沟通与交付记录。请勿提供游戏账号密码。</text></view>
          <view class="checkout"><view class="checkout-total"><text class="checkout-caption">合计</text><text class="total">¥{{ money(service.price_cents*quantity) }}</text></view><button class="btn" :disabled="submitting" @click="createOrder">{{ submitting?'正在创建…':state.user?'确认下单':'登录后下单' }}<Icon name="arrow" :size="14"/></button></view>
        </template>
        <view v-else class="notice">{{ service.provider_id===state.user?.id?'这是你发布的服务，可前往打手工作台管理。':'该服务暂未上架，无法下单。' }}</view>
        <view class="section-row"><text class="section-title">玩家评价</text><text class="muted">{{ reviews.length }} 条</text></view><view v-if="reviews.length" class="stack"><view v-for="review in reviews" :key="review.id" class="review"><view class="row between"><text>{{ review.buyer_name||'玩家' }}</text><text class="accent">{{ review.rating }} / 5</text></view><text class="review-content">{{ review.content }}</text><text class="hint">{{ time(review.created_at) }}</text></view></view><EmptyState v-else title="还没有评价" detail="完成订单后，玩家可留下真实体验。" icon="star"/>
      </view>
    </view>
  </AppShell>
</template>
<script setup lang="ts">
import { requirePhone } from '../../lib/phone'
import { ref, computed } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import CoverArt from '../../components/CoverArt.vue'
import DepositBadge from '../../components/DepositBadge.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, init, sessionRevision } from '../../lib/store'
import { readSafeArea } from '../../lib/safe-area'
import { normalizeLabelText } from '../../lib/cover'
import { getService } from '../../api/services.api'
import { createOrder as submitServiceOrder } from '../../api/orders.api'
import type { Service, Review } from '../../types/models'
import { token, errorMessage } from '../../api/client'
import { go, money, notify, time, assetUrl } from '../../lib/format'
/** 仅展示服务端返回的有效平台押金，不从交易金额、认证或本地草稿推算。 */
const providerDepositCents=computed(()=>{const amount=service.value?.provider_deposit_cents||0;return Number.isSafeInteger(amount)&&amount>0?amount:0})
/** 双押金长金额在窄屏可能换成两行，按两行高度预留，避免金额与用户文案相撞。 */
const coverReserveBottom=computed(()=>{const platform=providerDepositCents.value>0,club=(service.value?.provider_club_deposit_cents||0)>0;return 124+(platform&&club?64:platform||club?32:0)})
const service = ref<Service | null>(null), reviews = ref<Review[]>([]), loading = ref(true), error = ref(''), quantity = ref(1), requirements = ref(''), submitting = ref(false)
const coverReserveTop = Math.max(56, readSafeArea().floatingTop + 36 + 8)
/** 根据实际有效标签数量保留可读文字带，六条最多采用两列三行，不用极小字号掩盖拥挤。 */
const coverMinLabelArea = computed(() => { const count = (service.value?.labels || []).slice(0, 6).filter(label => normalizeLabelText(label.text)).length; return count > 4 ? 72 : count > 2 ? 48 : count ? 24 : 0 })
let id = '', disposed = false
/** 捕获下单请求所属用户和会话，授权后不沿用已改变的身份。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 页面仍存在且账号、令牌和修订号均一致时才继续交易或跳转。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean {return !disposed&&session.id===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 读取服务详情，并切到该服务的游戏主题。 */
async function load() { loading.value = true; error.value = ''; try { await init(); service.value = await getService(id); reviews.value = service.value.reviews || [] } catch (err) { error.value = errorMessage(err) } finally { loading.value = false } }
/** 保持购买数量在服务端允许的范围内。 */
function changeQuantity(amount: number) { quantity.value = Math.max(1, Math.min(99, quantity.value + amount)) }
/** 同一提交锁覆盖手机号授权和下单，金额由服务端计算，取消及失败均保留需求草稿。 */
async function createOrder() {
  if(submitting.value)return
  if(!state.user)return go('/pages/account/login?next='+encodeURIComponent('/pages/services/detail?id='+id))
  const session=currentSession();submitting.value=true
  try{
    if(!(await requirePhone())||!sameSession(session))return
    if(!requirements.value.trim())return notify('请填写需求与验收标准')
    const order=await submitServiceOrder({service_id:id,quantity:quantity.value,requirements:requirements.value.trim()})
    if(sameSession(session))go('/pages/orders/detail?id='+order.id)
  }catch(err){if(sameSession(session))notify(err)}finally{submitting.value=false}
}
onLoad(options => { id = options?.id || ''; load() })
onUnload(()=>{disposed=true})
</script>
<style scoped>
.detail-state{margin:96px 16px 24px}.service-content.with-checkout{padding-bottom:calc(88px + env(safe-area-inset-bottom))}.detail-gradient{position:absolute;z-index:1;inset:0;background:linear-gradient(180deg,transparent 32%,var(--bg) 100%);pointer-events:none}.detail-cover-info{position:absolute;z-index:3;left:16px;right:16px;bottom:0;display:flex;flex-direction:column;gap:6px;padding-bottom:14px;pointer-events:none}.detail-labels{display:flex;align-items:center;gap:8px;min-width:0;overflow:hidden;white-space:nowrap}.detail-label{padding:2px 8px;border:1px solid #ffffff40;border-radius:5px;background:#07120c60;color:#eef6f0;font-size:11px;line-height:18px;max-width:60%;overflow:hidden;text-overflow:ellipsis}.detail-cert{display:flex;align-items:center;gap:3px;color:var(--accent);font-size:11px;line-height:18px}.detail-title{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden;font-size:22px;line-height:30px;font-weight:750;color:var(--text)}.detail-body{padding:0 16px 24px}.price-row{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0;padding:12px 0 16px;border-bottom:1px solid var(--line-subtle)}.price-main{display:flex;align-items:baseline;gap:2px;color:var(--accent)}.currency{font-size:17px;font-weight:700}.detail-price{font-size:30px;line-height:36px;font-weight:750}.price-unit{color:var(--muted);font-size:12px}.sales{display:flex;align-items:center;gap:12px;color:var(--muted);font-size:11px;white-space:nowrap}.rating{display:flex;align-items:center;gap:3px;color:#d5ba7e}.provider-strip{display:flex;align-items:center;gap:8px;min-height:52px;border-bottom:1px solid var(--line-subtle);margin-bottom:12px}.provider-link{display:flex;align-items:center;gap:8px;flex:1;min-width:0;padding:8px 0;background:transparent;text-align:left;color:var(--accent)}.provider-avatar{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:28px;height:28px;border-radius:50%;overflow:hidden;background:var(--surface)}.provider-avatar image{width:100%;height:100%}.provider-name{font-size:13px;font-weight:650;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;max-width:45%}.provider-view{margin-left:auto;font-size:10px;color:var(--muted);white-space:nowrap}.club-link{font-size:10px}.content-section{padding:16px 0;border-bottom:1px solid var(--line-subtle)}.description{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px;line-height:23px;color:#baccc0;margin-top:12px}.service-traits{display:flex;flex-wrap:wrap;gap:8px;margin-top:12px}.trait{padding:3px 8px;border:1px solid var(--line);border-radius:14px;color:var(--muted);font-size:10px;line-height:16px}.quantity-row{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:16px 0 20px}.quantity{display:flex;align-items:center;gap:14px}.quantity button{display:flex;align-items:center;justify-content:center;width:32px;height:32px;padding:0;border:1px solid var(--line);border-radius:7px;background:transparent;color:var(--accent);font-size:19px;line-height:1}.quantity>text{font-size:15px;font-weight:650}.transaction-hint{display:block;font-size:11px;line-height:20px;color:var(--muted)}.checkout{position:fixed;z-index:30;bottom:0;left:0;right:0;display:flex;align-items:center;gap:16px;max-width:600px;margin:0 auto;padding:12px 16px calc(12px + env(safe-area-inset-bottom));border-top:1px solid var(--line);background:var(--bg)}.checkout>.btn{flex:1;min-width:0}.checkout-total{display:flex;align-items:baseline;gap:6px}.checkout-caption{font-size:11px;color:var(--muted)}.total{font-size:23px;color:var(--accent);font-weight:750}.review{padding:12px 0;border-bottom:1px solid var(--line-subtle)}.review-content{display:block;margin:8px 0;font-size:13px;line-height:22px;color:#b8c5bd}
.detail-deposits{display:flex;flex-wrap:wrap;align-items:center;justify-content:flex-end;gap:6px;min-width:0;max-width:100%}
</style>
