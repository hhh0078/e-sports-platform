<!-- 文件作用：按个人中心设计图呈现身份、真实统计、订单与账户入口，访客完整显示零数据。 -->
<template>
  <view class="me-content">
    <view class="profile-hero"><image v-if="game.cover_url" class="profile-hero-image" :src="assetUrl(game.cover_url)" mode="aspectFill"/><view class="profile-hero-shade"/><view class="identity-row"><view class="identity-main" @click="openIdentity"><view class="profile-avatar"><image class="profile-avatar-image" v-if="state.user?.avatar" :src="assetUrl(state.user.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="32"/></view><view class="identity-copy"><view class="profile-name-row"><text class="profile-name">{{ state.user?.nickname||'登录后查看' }}</text><Icon name="arrow" :size="12"/></view><text class="profile-bio">{{ state.user?.bio||(state.user?'认真游戏，快乐同行':'登录后管理服务与订单') }}</text></view></view></view></view>
    <view class="orders-section"><view class="panel-heading"><text>我的订单</text><button class="all-orders-entry" @click="protectedGo('/pages/main/index?tab=orders&role=buyer')">全部<Icon name="arrow" :size="12"/></button></view><view class="order-shortcuts"><button class="order-shortcut" v-for="item in orderLinks" :key="item.status" @click="protectedGo('/pages/main/index?tab=orders&role=buyer&status='+item.status)"><view class="order-icon-wrap"><Icon :name="item.icon" :size="25"/><text v-if="orderCount(item.countKey)" class="order-count">{{ orderCount(item.countKey) }}</text></view><text>{{ item.label }}</text></button></view></view>
    <view class="me-wallet" @click="protectedGo('/pages/account/wallet')"><view class="wallet-heading"><Icon name="wallet" :size="21"/><text class="wallet-heading-title">我的钱包</text><view class="wallet-available"><text class="wallet-caption">可提现</text><text class="wallet-main-value">{{ walletAmount('available_cents') }}</text></view><Icon name="arrow" :size="13"/></view><view class="wallet-amounts"><view class="wallet-item"><text>进行中</text><text class="wallet-item-value">{{ walletAmount('ongoing_cents') }}</text></view><view class="wallet-item"><text>结算中</text><text class="wallet-item-value">{{ walletAmount('settling_cents') }}</text></view><view class="wallet-item"><text>提现中</text><text class="wallet-item-value">{{ walletAmount('withdrawal_frozen_cents') }}</text></view></view><view class="wallet-recharge"><text class="wallet-recharge-label">充值余额（仅消费）</text><text class="wallet-recharge-value">{{ walletAmount('recharge_cents') }}</text><Icon name="arrow" :size="12"/></view><text v-if="walletError" class="wallet-error">{{ walletError }}</text></view>
    <view class="space-grid"><button class="space-entry" @click="openIdentity"><Icon name="user" :size="25"/><view class="space-copy"><text class="space-title">我的主页</text><text class="space-detail">展示你的游戏生涯</text></view><Icon class="space-arrow" name="arrow" :size="13"/></button><button class="space-entry space-club" @click="protectedGo('/pages/community/clubs/index?mine=1')"><Icon name="club" :size="25"/><view class="space-copy"><text class="space-title">我的俱乐部</text><text class="space-detail">管理专属俱乐部</text></view><Icon class="space-arrow" name="arrow" :size="13"/></button></view>

    <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load">重新加载</button></view>
    <view class="menu-list"><ContactSupport :active="props.active"/><button class="menu-item messages-entry" @click="go('/pages/account/messages/index')"><Icon name="bell" :size="22"/><text class="menu-label">消息中心</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="protectedGo('/pages/account/profile')"><Icon name="user" :size="22"/><text class="menu-label">个人资料</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="verifyPhone"><Icon name="phone" :size="22"/><text class="menu-label">手机号验证</text><text class="menu-value">{{ state.user?.phone_verified?state.user.phone_masked||'已验证':'未验证' }}</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="go('/pages/account/agreements?type=user_agreement')"><Icon name="orders" :size="22"/><text class="menu-label">用户协议</text><Icon name="arrow" :size="14"/></button><button class="menu-item" @click="go('/pages/account/agreements?type=privacy_policy')"><Icon name="shield" :size="22"/><text class="menu-label">隐私政策</text><Icon name="arrow" :size="14"/></button><button v-if="state.user?.role==='admin'" class="menu-item" @click="go('/pages/manage/index')"><Icon name="settings" :size="22"/><text class="menu-label">平台管理中心</text><Icon name="arrow" :size="14"/></button><button v-if="state.devMode" class="menu-item" @click="go('/pages/account/login')"><Icon name="refresh" :size="22"/><text class="menu-label">切换体验身份</text><Icon name="arrow" :size="14"/></button></view>
  </view>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import Icon from '../../../components/Icon.vue'
import ContactSupport from '../../../components/ContactSupport.vue'
import { state, game, init, clearSession, sessionRevision } from '../../../lib/store'
import { token, errorMessage, errorStatus } from '../../../api/client'
import { getMe, getMyStats } from '../../../api/users.api'
import { getWallet } from '../../../api/wallet.api'
import type { Wallet } from '../../../types/wallet'
import { requirePhone } from '../../../lib/phone'
import { go, notify, assetUrl } from '../../../lib/format'
import { useMainVisibility } from '../../../lib/navigation'
import type { UserStats } from '../../../types/models'
const props = defineProps<{ active: boolean }>()
const stats = ref<Partial<UserStats>>({}), error = ref(''),wallet=ref<Wallet|null>(null),walletError=ref('')
const orderLinks = [{ status: 'pending_payment', label: '待付款', icon: 'wallet',countKey:'buyer_pending_payment_count' }, { status: 'working', label: '进行中', icon: 'clock',countKey:'buyer_working_count' }, { status: 'delivered', label: '待验收', icon: 'checkbox',countKey:'buyer_delivered_count' }, { status: 'refund', label: '售后', icon: 'headphones',countKey:'buyer_refund_count' }] satisfies Array<{status:string;label:string;icon:string;countKey:keyof UserStats}>
/** 仅显示真实订单数量，最多展示99+，访客及未知数据不显示角标。 */
function orderCount(key:keyof UserStats):string {const value=state.user?Number(stats.value[key]||0):0;return Number.isSafeInteger(value)&&value>0?(value>99?'99+':String(value)):''}
/** 钱包分区只取服务端金额，读取中及故障不伪造零余额。 */
function walletAmount(key:keyof Wallet):string {if(!state.user)return '¥0.00';const value=wallet.value?.[key];return typeof value==='number'&&Number.isSafeInteger(value)?'¥'+(value/100).toFixed(2):'—'}

let loadVersion = 0, visible = false
/** 记录身份及版本，阻止迟到请求覆盖其他会话。 */
function currentSession() { return { token: token(), userId: state.user?.id, revision: sessionRevision() } }
/** 比较会话是否仍属于当前账号。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return session.token === token() && session.userId === state.user?.id && session.revision === sessionRevision() }
/** 使旧统计读取失效。 */
function invalidateLoad(): number { stats.value = {};wallet.value=null;walletError.value=''; return ++loadVersion }
/** 检查视图可见且请求仍有效。 */
function currentRequest(version: number, session: ReturnType<typeof currentSession>): boolean { return visible && version === loadVersion && sameSession(session) }
/** 需要身份的操作保留登录后目标。 */
function protectedGo(target: string): boolean { if (!state.user) { go('/pages/account/login?next=' + encodeURIComponent(target)); return false } go(target); return true }
/** 点击头像查看本人公开名片。 */
function openIdentity(): void { protectedGo(state.user ? '/pages/community/players/detail?id=' + encodeURIComponent(state.user.id) : '/pages/main/index?tab=me') }
/** 主动验证手机号，授权拒绝不绕过交易门槛。 */
async function verifyPhone(): Promise<void> { if (!state.user) { protectedGo('/pages/main/index?tab=me'); return } if (state.user.phone_verified) { notify('已验证 ' + (state.user.phone_masked || '手机号')); return } const session=currentSession();try { await requirePhone() } catch (err) { if (visible&&sameSession(session)) notify(err) } }
/** 刷新个人资料和真实统计，保留隐藏及跨账号竞态保护。 */
async function load(): Promise<void> {
  const version = invalidateLoad(); let session: ReturnType<typeof currentSession> | undefined; error.value = ''
  try { await init(); if (!visible || version !== loadVersion || !state.user) return; session = currentSession(); const user = await getMe(); if (!currentRequest(version, session)) return; state.user = user; const [summary,funds] = await Promise.allSettled([getMyStats(),getWallet()]);if(!currentRequest(version,session))return;if(summary.status==='rejected'&&errorStatus(summary.reason)===401||funds.status==='rejected'&&errorStatus(funds.reason)===401){clearSession();return}if(summary.status==='fulfilled')stats.value=summary.value||{};else error.value=errorMessage(summary.reason);if(funds.status==='fulfilled')wallet.value=funds.value;else walletError.value='钱包暂未同步，点击进入重试'  }
  catch (err) { if (!visible || version !== loadVersion || (session && !sameSession(session))) return; if (errorStatus(err) === 401) { clearSession(); return } error.value = errorMessage(err) }
}
/** 激活视图时读取数据。 */
function showPage(): void { visible = true; void load() }
/** 隐藏视图时丢弃所有未完成的数据读取。 */
function hidePage(): void { visible = false; invalidateLoad() }
useMainVisibility(() => props.active, showPage, hidePage)
/** 隐藏时也清除旧身份统计，并在同账号会话轮换后拒绝迟到回调。 */
watch([() => state.user?.id, sessionRevision], () => { invalidateLoad(); error.value = ''; if (visible) void load() }, { flush: 'sync' })
</script>
<style scoped>
	.me-content {
		padding: 0 30.7692rpx 46.1538rpx;
		min-width: 0
	}

	.profile-hero {
		position: relative;
		min-height: 323.0769rpx;
		box-sizing: border-box;
		margin: 0 -30.7692rpx;
		padding: max(30.7692rpx, calc(var(--status-bar-height, 0px) + var(--capsule-space, 44px) - 23.0769rpx)) 30.7692rpx 23.0769rpx;
		overflow: hidden;
		background: var(--bg)
	}

	.profile-hero-image {
		position: absolute;
		inset: 0;
		z-index: 0;
		width: 100%;
		height: 100%
	}

	.profile-hero-shade {
		position: absolute;
		inset: 0;
		z-index: 1;
		background: linear-gradient(180deg, rgba(32, 35, 33, 0.5) 0%, rgba(25, 28, 27, 0.84) 55%, var(--bg) 100%)
	}

	.identity-row {
		position: relative;
		z-index: 2;
		display: flex;
		align-items: center;
		min-width: 0;
		min-height: 123.0769rpx
	}

	.identity-main {
		display: flex;
		flex: 1;
		min-width: 0;
		align-items: center;
		gap: 30.7692rpx
	}

	.profile-avatar {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 123.0769rpx;
		height: 123.0769rpx;
		border: 1px solid #ffffff55;
		border-radius: 50%;
		overflow: hidden;
		background: #26312cc9;
		color: #d3e3d9
	}

	.profile-avatar-image {
		width: 100%;
		height: 100%
	}

	.identity-copy {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 11.5385rpx;
		min-width: 0
	}

	.profile-name-row {
		display: flex;
		align-items: center;
		gap: 9.6154rpx;
		min-width: 0;
		color: #d6ded8
	}

	.profile-name {
		font-size: 40.3846rpx;
		font-weight: 700;
		line-height: 55.7692rpx;
		color: #f1f5f2;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap
	}

	.profile-bio {
		display: -webkit-box;
		-webkit-line-clamp: 2;
		-webkit-box-orient: vertical;
		max-height: 69.2308rpx;
		overflow: hidden;
		font-size: 23.0769rpx;
		line-height: 34.6154rpx;
		color: #c1cdc5;
		overflow-wrap: anywhere
	}

	.stats-row {
		position: relative;
		z-index: 2;
		display: flex;
		margin-top: 38.4615rpx;
		padding: 0
	}

	.profile-stat {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		gap: 7.6923rpx;
		min-width: 0;
		border-right: 1px solid #ffffff16
	}

	.profile-stat:last-child {
		border-right: 0
	}

	.profile-stat-value {
		color: var(--accent);
		font-size: 42.3077rpx;
		font-weight: 650;
		line-height: 53.8462rpx;
		font-variant-numeric: tabular-nums
	}

	.profile-stat-label {
		font-size: 21.1538rpx;
		line-height: 32.6923rpx;
		color: #c1cdc5;
		white-space: nowrap
	}

	.orders-section {
		padding: 15.3846rpx 0 30.7692rpx;
		border-bottom: 1px solid var(--line-subtle)
	}

	.panel-heading {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 23.0769rpx;
		min-height: 69.2308rpx;
		font-size: 28.8462rpx;
		font-weight: 650
	}

	.all-orders-entry {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 5.7692rpx;
		min-height: 53.8462rpx;
		padding: 0;
		background: transparent;
		color: var(--muted);
		font-size: 19.2308rpx;
		font-weight: 400
	}

	.order-shortcuts {
		display: flex;
		padding: 30.7692rpx 0 7.6923rpx
	}

	.order-shortcut {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 17.3077rpx;
		min-width: 0;
		padding: 0;
		background: transparent;
		color: #d4e0d8;
		font-size: 21.1538rpx;
		line-height: 34.6154rpx
	}

	.space-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		padding: 30.7692rpx 0;
		border-bottom: 1px solid var(--line-subtle)
	}

	.space-entry {
		display: flex;
		align-items: center;
		gap: 15.3846rpx;
		min-width: 0;
		min-height: 44px;
		padding: 0 23.0769rpx 0 0;
		border: 0;
		border-radius: 0;
		background: transparent;
		color: var(--accent)
	}

	.space-club {
		padding: 0 0 0 30.7692rpx;
		border-left: 1px solid var(--line-subtle)
	}

	.space-copy {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 7.6923rpx;
		text-align: left;
		min-width: 0
	}

	.space-title {
		font-size: 25rpx;
		font-weight: 650;
		line-height: 38.4615rpx;
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap
	}

	.space-detail {
		font-size: 19.2308rpx;
		line-height: 30.7692rpx;
		color: var(--muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap
	}

	.space-arrow {
		color: var(--muted)
	}

	.provider-row {
		display: flex;
		align-items: center;
		gap: 23.0769rpx;
		min-height: 130.7692rpx;
		padding: 23.0769rpx 0;
		border-bottom: 1px solid var(--line-subtle);
		color: #d0ded4
	}

	.provider-copy {
		display: flex;
		flex: 1;
		flex-direction: column;
		gap: 5.7692rpx;
		min-width: 0
	}

	.provider-title {
		font-size: 25rpx;
		font-weight: 650;
		line-height: 38.4615rpx;
		color: var(--text)
	}

	.provider-detail {
		font-size: 19.2308rpx;
		line-height: 32.6923rpx;
		color: var(--muted)
	}

	.provider-default {
		display: flex;
		align-items: center;
		gap: 9.6154rpx;
		flex-shrink: 0;
		color: var(--accent);
		font-size: 21.1538rpx;
		line-height: 34.6154rpx;
		white-space: nowrap
	}

	.menu-list {
		min-width: 0
	}

	.menu-item {
		display: flex;
		align-items: center;
		gap: 23.0769rpx;
		width: 100%;
		height: 92.3077rpx;
		padding: 0;
		border: 0;
		border-bottom: 1px solid var(--line-subtle);
		border-radius: 0;
		background: transparent;
		color: #c8dacd;
		text-align: left
	}

	.menu-item:last-child {
		border-bottom: 0
	}

	.menu-label {
		flex: 1;
		min-width: 0;
		font-size: 25rpx;
		color: var(--text)
	}

	.menu-value {
		flex-shrink: 0;
		font-size: 19.2308rpx;
		color: var(--muted)
	}

	.error-box {
		margin: 23.0769rpx 0
	}

	.order-icon-wrap {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 61.5385rpx;
		height: 57.6923rpx
	}

	.order-count {
		position: absolute;
		top: -11.5385rpx;
		right: -17.3077rpx;
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 30.7692rpx;
		height: 30.7692rpx;
		padding: 0 7.6923rpx;
		box-sizing: border-box;
		border: 1px solid var(--bg);
		border-radius: 19.2308rpx;
		background: var(--accent);
		color: var(--bg);
		font-size: 17.3077rpx;
		line-height: 26.9231rpx;
		font-weight: 700
	}

	.me-wallet {
		min-width: 0;
		margin: 23.0769rpx 0 0;
		padding: 19.2308rpx 23.0769rpx 0;
		border: 1px solid var(--line);
		border-radius: 23.0769rpx;
		background: var(--surface)
	}

	.wallet-heading {
		display: flex;
		align-items: center;
		gap: 13.4615rpx;
		min-width: 0;
		min-height: 61.5385rpx;
		font-size: 25rpx;
		font-weight: 650
	}

	.wallet-heading-title {
		flex: 1;
		min-width: 0;
		white-space: nowrap
	}

	.wallet-available {
		display: flex;
		align-items: baseline;
		justify-content: flex-end;
		flex-wrap: wrap;
		gap: 9.6154rpx;
		min-width: 0;
		max-width: 65%
	}

	.wallet-caption {
		color: var(--muted);
		font-size: 19.2308rpx;
		line-height: 34.6154rpx;
		font-weight: 400
	}

	.wallet-main-value {
		max-width: 100%;
		color: var(--accent);
		font-size: 38.4615rpx;
		line-height: 53.8462rpx;
		font-weight: 700;
		overflow-wrap: anywhere
	}

	.wallet-amounts {
		display: flex;
		gap: 15.3846rpx;
		margin: 19.2308rpx 0
	}

	.wallet-item {
		display: flex;
		flex: 1;
		min-width: 0;
		flex-direction: column;
		gap: 3.8462rpx;
		align-items: center;
		color: var(--muted);
		font-size: 19.2308rpx;
		line-height: 30.7692rpx;
		border-right: 1px solid var(--line-subtle)
	}

	.wallet-item:last-child {
		border-right: 0
	}

	.wallet-item-value {
		max-width: 100%;
		color: var(--text);
		font-size: 23.0769rpx;
		line-height: 34.6154rpx;
		overflow-wrap: anywhere
	}

	.wallet-recharge {
		display: flex;
		align-items: center;
		gap: 15.3846rpx;
		padding: 13.4615rpx 0;
		border-top: 1px solid var(--line-subtle);
		color: var(--muted);
		font-size: 19.2308rpx;
		line-height: 34.6154rpx
	}

	.wallet-recharge-label {
		flex: 1;
		min-width: 0
	}

	.wallet-recharge-value {
		color: var(--accent)
	}

	.wallet-error {
		display: block;
		padding: 0 0 19.2308rpx;
		color: #dfbb71;
		font-size: 21.1538rpx;
		line-height: 34.6154rpx
	}

	@media(max-width:350px) {
		.profile-hero {
			min-height: 328.125rpx
		}

		.identity-main {
			gap: 19.2308rpx
		}

		.profile-avatar {
			width: 107.6923rpx;
			height: 107.6923rpx
		}

		.profile-name {
			font-size: 32.6923rpx
		}

		.profile-bio {
			font-size: 21.1538rpx
		}

		.space-entry {
			gap: 13.4615rpx;
			padding-right: 15.3846rpx
		}

		.space-club {
			padding: 0 0 0 19.2308rpx
		}

		.space-title {
			font-size: 23.0769rpx
		}

		.space-detail {
			font-size: 17.3077rpx
		}

		.space-arrow {
			display: none
		}

		.profile-stat-label {
			font-size: 19.2308rpx
		}
	}
</style>
