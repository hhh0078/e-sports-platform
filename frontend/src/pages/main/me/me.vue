<!-- 文件作用：按个人中心设计图呈现身份、真实统计、订单与账户入口，访客完整显示零数据。 -->
<template>
  <view class="me-content">
    <view class="profile-hero"><image v-if="game.cover_url" class="profile-hero-image" :src="assetUrl(game.cover_url)" mode="aspectFill"/><view class="profile-hero-shade"/><view class="identity-row"><view class="identity-main" @click="openIdentity"><view class="profile-avatar"><image class="profile-avatar-image" v-if="state.user?.avatar" :src="assetUrl(state.user.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="32"/></view><view class="identity-copy"><view class="profile-name-row"><text class="profile-name">{{ state.user?.nickname||'登录后查看' }}</text><Icon name="arrow" :size="12"/></view><text class="profile-bio">{{ state.user?.bio||(state.user?'认真游戏，快乐同行':'登录后管理服务与订单') }}</text></view></view></view><view class="stats-row"><view class="profile-stat" v-for="item in statLinks" :key="item.key" @click="protectedGo(item.url)"><text class="profile-stat-value">{{ stat(item.key) }}</text><text class="profile-stat-label">{{ item.label }}</text></view></view></view>
    <view class="orders-section"><view class="panel-heading"><text>我的订单</text><button class="all-orders-entry" @click="protectedGo('/pages/main/index?tab=orders&role=buyer')">全部<Icon name="arrow" :size="12"/></button></view><view class="order-shortcuts"><button class="order-shortcut" v-for="item in orderLinks" :key="item.status" @click="protectedGo('/pages/main/index?tab=orders&role=buyer&status='+item.status)"><Icon :name="item.icon" :size="25"/><text>{{ item.label }}</text></button></view></view>
    <view class="space-grid"><button class="space-entry" @click="openIdentity"><Icon name="user" :size="25"/><view class="space-copy"><text class="space-title">我的主页</text><text class="space-detail">展示你的游戏生涯</text></view><Icon class="space-arrow" name="arrow" :size="13"/></button><button class="space-entry space-club" @click="protectedGo('/pages/community/clubs/index?mine=1')"><Icon name="club" :size="25"/><view class="space-copy"><text class="space-title">我的俱乐部</text><text class="space-detail">管理专属俱乐部</text></view><Icon class="space-arrow" name="arrow" :size="13"/></button></view>
    <view class="provider-switch"><Icon name="game" :size="25"/><view class="provider-copy"><text class="provider-title">接单身份</text><text class="provider-detail">{{ !state.user?'登录后开启服务发布':state.user.provider_enabled?'无需挂靠，自由接单':'完善介绍，随时开启' }}</text></view><NativeSwitch class="provider-native-switch" :key="switchVersion" :checked="!!state.user?.provider_enabled" :color="game.primary_color" :disabled="saving" @change="toggleProvider"/></view>
    <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load">重新加载</button></view>
    <view class="menu-list"><button class="menu-item" @click="protectedGo('/pages/account/profile')"><Icon name="user" :size="22"/><text class="menu-label">个人资料</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="verifyPhone"><Icon name="phone" :size="22"/><text class="menu-label">手机号验证</text><text class="menu-value">{{ state.user?.phone_verified?state.user.phone_masked||'已验证':'未验证' }}</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="go('/pages/account/agreements?type=user_agreement')"><Icon name="orders" :size="22"/><text class="menu-label">用户协议</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="go('/pages/account/agreements?type=privacy_policy')"><Icon name="shield" :size="22"/><text class="menu-label">隐私政策</text><Icon name="arrow" :size="14"/></button><button v-if="state.user?.role==='admin'" class="menu-item" @click="go('/pages/manage/index')"><Icon name="settings" :size="22"/><text class="menu-label">平台管理中心</text><Icon name="arrow" :size="14"/></button><button v-if="state.devMode" class="menu-item" @click="go('/pages/account/login')"><Icon name="refresh" :size="22"/><text class="menu-label">切换体验身份</text><Icon name="arrow" :size="14"/></button></view>
  </view>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import NativeSwitch from '../../../components/NativeSwitch.vue'
import Icon from '../../../components/Icon.vue'
import { state, game, init, clearSession, sessionRevision } from '../../../lib/store'
import { token, errorMessage, errorStatus } from '../../../api/client'
import { getMe, getMyStats, setProviderEnabled } from '../../../api/users.api'
import { requirePhone } from '../../../lib/phone'
import { go, notify, assetUrl } from '../../../lib/format'
import { useMainVisibility } from '../../../lib/navigation'
import type { UserStats } from '../../../types/models'
const props = defineProps<{ active: boolean }>()
const stats = ref<Partial<UserStats>>({}), saving = ref(false), error = ref(''), switchVersion = ref(0)
const orderLinks = [{ status: 'pending_payment', label: '待付款', icon: 'wallet' }, { status: 'working', label: '进行中', icon: 'clock' }, { status: 'delivered', label: '待验收', icon: 'checkbox' }, { status: 'refund', label: '售后', icon: 'headphones' }]
const statLinks = [{ key: 'buyer_order_count', label: '下单记录', url: '/pages/main/index?tab=orders&role=buyer' }, { key: 'provider_working_count', label: '进行中接单', url: '/pages/main/index?tab=orders&role=provider&status=working' }, { key: 'completed_order_count', label: '完成订单', url: '/pages/main/index?tab=orders&role=all&status=completed' }] satisfies Array<{ key: keyof UserStats; label: string; url: string }>
let loadVersion = 0, visible = false
/** 记录身份及版本，阻止迟到请求覆盖其他会话。 */
function currentSession() { return { token: token(), userId: state.user?.id, revision: sessionRevision() } }
/** 比较会话是否仍属于当前账号。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return session.token === token() && session.userId === state.user?.id && session.revision === sessionRevision() }
/** 使旧统计读取失效。 */
function invalidateLoad(): number { stats.value = {}; return ++loadVersion }
/** 检查视图可见且请求仍有效。 */
function currentRequest(version: number, session: ReturnType<typeof currentSession>): boolean { return visible && version === loadVersion && sameSession(session) }
/** 访客显示零值，登录用户等待真实统计。 */
function stat(key: keyof UserStats): number | string { return state.user ? stats.value[key] ?? '—' : 0 }
/** 需要身份的操作保留登录后目标。 */
function protectedGo(target: string): boolean { if (!state.user) { go('/pages/account/login?next=' + encodeURIComponent(target)); return false } go(target); return true }
/** 点击头像查看本人公开名片。 */
function openIdentity(): void { protectedGo(state.user ? '/pages/community/players/detail?id=' + encodeURIComponent(state.user.id) : '/pages/main/index?tab=me') }
/** 主动验证手机号，授权拒绝不绕过交易门槛。 */
async function verifyPhone(): Promise<void> { if (!state.user) { protectedGo('/pages/main/index?tab=me'); return } if (state.user.phone_verified) { notify('已验证 ' + (state.user.phone_masked || '手机号')); return } const session=currentSession();try { await requirePhone() } catch (err) { if (visible&&sameSession(session)) notify(err) } }
/** 刷新个人资料和真实统计，保留隐藏及跨账号竞态保护。 */
async function load(): Promise<void> {
  const version = invalidateLoad(); let session: ReturnType<typeof currentSession> | undefined; error.value = ''
  try { await init(); if (!visible || version !== loadVersion || !state.user) return; session = currentSession(); const user = await getMe(); if (!currentRequest(version, session)) return; state.user = user; const summary = await getMyStats(); if (currentRequest(version, session)) stats.value = summary || {} }
  catch (err) { if (!visible || version !== loadVersion || (session && !sameSession(session))) return; if (errorStatus(err) === 401) { clearSession(); return } error.value = errorMessage(err) }
}
/** 接单开关保留简介门槛与失败回滚。 */
async function toggleProvider(event: { detail: { value: boolean } }): Promise<void> {
  if (saving.value || !state.user) { switchVersion.value++; if (!state.user) protectedGo('/pages/main/index?tab=me'); return }
  const enabled = event.detail.value
  if (enabled && !state.user.bio?.trim()) { switchVersion.value++; notify('请先完善个人介绍'); go('/pages/account/profile'); return }
  const version = invalidateLoad(), session = currentSession(); saving.value = true
  try { const user = await setProviderEnabled(enabled); if (!currentRequest(version, session)) return; state.user = user; notify(enabled ? '接单身份已开启' : '接单身份已关闭') }
  catch (err) { if (currentRequest(version, session)) notify(err) }
  finally { saving.value = false; switchVersion.value++; if (visible && sameSession(session)) void load() }
}
/** 激活视图时读取数据。 */
function showPage(): void { visible = true; void load() }
/** 隐藏视图时丢弃所有未完成的数据读取。 */
function hidePage(): void { visible = false; invalidateLoad() }
useMainVisibility(() => props.active, showPage, hidePage)
/** 隐藏时也清除旧身份统计，并在同账号会话轮换后拒绝迟到回调。 */
watch([() => state.user?.id, sessionRevision], () => { invalidateLoad(); error.value = ''; switchVersion.value++; if (visible) void load() }, { flush: 'sync' })
</script>
<style scoped>
.me-content{padding:0 16px 24px;min-width:0}.profile-hero{position:relative;min-height:240px;box-sizing:border-box;margin:0 -16px;padding:calc(var(--status-bar-height,0px) + var(--capsule-space,44px)) 16px 16px;overflow:hidden;background:var(--bg)}.profile-hero-image{position:absolute;inset:0;z-index:0;width:100%;height:100%}.profile-hero-shade{position:absolute;inset:0;z-index:1;background:linear-gradient(180deg,#20232180 0%,#191c1bd6 68%,var(--bg) 100%)}.identity-row{position:relative;z-index:2;display:flex;align-items:center;min-width:0;min-height:64px}.identity-main{display:flex;flex:1;min-width:0;align-items:center;gap:16px}.profile-avatar{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:64px;height:64px;border:1px solid #ffffff55;border-radius:50%;overflow:hidden;background:#26312cc9;color:#d3e3d9}.profile-avatar-image{width:100%;height:100%}.identity-copy{display:flex;flex:1;flex-direction:column;gap:6px;min-width:0}.profile-name-row{display:flex;align-items:center;gap:5px;min-width:0;color:#d6ded8}.profile-name{font-size:21px;font-weight:700;line-height:29px;color:#f1f5f2;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.profile-bio{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;max-height:36px;overflow:hidden;font-size:12px;line-height:18px;color:#c1cdc5;overflow-wrap:anywhere}.stats-row{position:relative;z-index:2;display:flex;margin-top:20px;padding:0}.profile-stat{display:flex;flex:1;flex-direction:column;align-items:center;gap:4px;min-width:0;border-right:1px solid #ffffff16}.profile-stat:last-child{border-right:0}.profile-stat-value{color:var(--accent);font-size:22px;font-weight:650;line-height:28px;font-variant-numeric:tabular-nums}.profile-stat-label{font-size:11px;line-height:17px;color:#c1cdc5;white-space:nowrap}.orders-section{padding:12px 0 16px;border-bottom:1px solid var(--line-subtle)}.panel-heading{display:flex;justify-content:space-between;align-items:center;gap:12px;min-height:36px;font-size:15px;font-weight:650}.all-orders-entry{display:flex;align-items:center;justify-content:center;gap:3px;min-height:28px;padding:0;background:transparent;color:var(--muted);font-size:10px;font-weight:400}.order-shortcuts{display:flex;padding:16px 0 4px}.order-shortcut{display:flex;flex:1;flex-direction:column;align-items:center;justify-content:center;gap:9px;min-width:0;padding:0;background:transparent;color:#d4e0d8;font-size:11px;line-height:18px}.space-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));padding:16px 0;border-bottom:1px solid var(--line-subtle)}.space-entry{display:flex;align-items:center;gap:8px;min-width:0;min-height:44px;padding:0 12px 0 0;border:0;border-radius:0;background:transparent;color:var(--accent)}.space-club{padding:0 0 0 16px;border-left:1px solid var(--line-subtle)}.space-copy{display:flex;flex:1;flex-direction:column;gap:4px;text-align:left;min-width:0}.space-title{font-size:13px;font-weight:650;line-height:20px;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.space-detail{font-size:10px;line-height:16px;color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.space-arrow{color:var(--muted)}.provider-switch{display:flex;align-items:center;gap:12px;min-height:68px;padding:12px 0;border-bottom:1px solid var(--line-subtle);color:#d0ded4}.provider-copy{display:flex;flex:1;flex-direction:column;gap:3px;min-width:0}.provider-title{font-size:13px;font-weight:650;line-height:20px;color:var(--text)}.provider-detail{font-size:10px;line-height:17px;color:var(--muted)}.provider-native-switch{flex-shrink:0;transform:scale(.8);transform-origin:right center;width:44px}.menu-list{min-width:0}.menu-item{display:flex;align-items:center;gap:12px;width:100%;height:48px;padding:0;border:0;border-bottom:1px solid var(--line-subtle);border-radius:0;background:transparent;color:#c8dacd;text-align:left}.menu-item:last-child{border-bottom:0}.menu-label{flex:1;min-width:0;font-size:13px;color:var(--text)}.menu-value{flex-shrink:0;font-size:10px;color:var(--muted)}.error-box{margin:12px 0}
@media(max-width:350px){.identity-main{gap:10px}.profile-avatar{width:56px;height:56px}.profile-name{font-size:17px}.profile-bio{font-size:11px}.space-entry{gap:7px;padding-right:8px}.space-club{padding:0 0 0 10px}.space-title{font-size:12px}.space-detail{font-size:9px}.space-arrow{display:none}.profile-stat-label{font-size:10px}}
</style>
