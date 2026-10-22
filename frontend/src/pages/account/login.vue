<!-- 文件作用：以游戏爱好者自由接单为入口，明确取得协议同意后登录，并提供可跳过的头像昵称完善。 -->
<template>
	<AppShell title="登录开机电竞" back>
		<view class="login-page">

			<view class="login-copy">

				<view class="login-headline">热爱游戏</view>
				<view class="login-headline login-headline-accent">自由接单</view><text
					class="login-subtitle">无需挂靠，凭实力找到队友</text>
			</view>
			<view class="game-montage">
				<view v-for="cover in loginCovers" :key="cover" class="montage-part">
					<image class="montage-image" :src="assetUrl(cover)" mode="aspectFill" />
				</view>
				<view class="montage-shade" />
			</view>
			<view class="login-bottom">
				<view class="login-benefits">
					<view class="login-benefit">
						<Icon name="game" :size="24" /><text>自由接单</text>
					</view>
					<view class="login-benefit">
						<Icon name="orders" :size="24" /><text>订单留证</text>
					</view>
					<view class="login-benefit">
						<Icon name="shield" :size="24" /><text>俱乐部认证</text>
					</view>
				</view>
				<view v-if="error" class="login-error"><text class="login-error-copy">{{error}}</text><button
						v-if="!agreements" class="retry-login" @click="load">重新加载</button></view>
				<button class="btn login-primary" :disabled="!!submitting||loading" @click="realLogin">
					<Icon name="weixin" :size="23" /><text>{{submitting?'正在登录…':loading?'正在准备…':'微信一键登录'}}</text>
				</button>
				<view class="consent-line"><checkbox-group @change="changeConsent"><label
							class="consent-checkbox-label">
							<checkbox class="consent-checkbox" value="accepted" :checked="accepted" color="#7fe4ba"
								:disabled="!!submitting" />
						</label></checkbox-group>
					<view class="consent-copy">我已阅读并同意<text class="agreement-link"
							@click="openAgreement('user_agreement')">《用户协议》</text>和<text class="agreement-link"
							@click="openAgreement('privacy_policy')">《隐私政策》</text></view>
				</view>
				<button v-if="state.devMode" class="dev-toggle" :disabled="!!submitting" @click="devOpen=true">开发体验
					<Icon name="arrow" :size="11" />
				</button>
			</view>
		</view>
		<view v-if="devOpen" class="login-mask" @click="devOpen=false">
			<view class="login-sheet" @click.stop>
				<view class="sheet-heading"><text>选择体验身份</text><button class="sheet-close" @click="devOpen=false">
						<Icon name="close" :size="21" />
					</button></view><text class="sheet-hint">本地测试账号，登录前同样需要主动同意协议。</text><scroll-view scroll-y
					class="accounts-scroll"><button v-for="account in accounts" :key="account.id" class="account-card"
						:disabled="!!submitting" @click="selectAccount(account)">
						<view class="account-avatar">{{account.nickname?.slice(0,1)}}</view>
						<view class="account-copy"><text class="account-name">{{account.nickname}}</text><text
								class="account-detail">{{accountDescriptions[account.id]||'体验玩家与接单能力'}}</text></view>
						<Icon name="arrow" :size="15" />
					</button></scroll-view>
			</view>
		</view>
		<view v-if="profileOffer" class="login-mask" @click="finishLogin">
			<view class="login-sheet profile-offer" @click.stop>
				<view class="sheet-heading"><text>让队友认识你</text><button class="sheet-close" :disabled="profileBusy"
						@click="finishLogin">
						<Icon name="close" :size="21" />
					</button></view><text class="sheet-hint">头像和昵称随时可以修改，也可以稍后完善。</text><button class="offer-avatar"
					:disabled="profileBusy" @click="pickAvatar">
					<image class="offer-avatar-image" v-if="draft.avatar" :src="assetUrl(draft.avatar)"
						mode="aspectFill" />
					<Icon v-else name="user" :size="30" />
				</button>
				<view class="form-section offer-field"><text class="field-label">昵称</text><input
						v-model="draft.nickname" type="nickname" class="input" maxlength="24" placeholder="2—24个字的昵称" />
				</view><button class="btn login-primary" :disabled="profileBusy"
					@click="saveOptionalProfile">{{profileBusy?'正在保存…':'保存并进入'}}</button><button class="finish-later"
					:disabled="profileBusy" @click="finishLogin">先这样，稍后完善</button>
			</view>
		</view>
	</AppShell>
</template>
<script setup lang="ts">
	import { ref, reactive } from 'vue'
	import { onLoad, onHide, onUnload } from '@dcloudio/uni-app'
	import AppShell from '../../components/AppShell.vue'
	import Icon from '../../components/Icon.vue'
	import { state, init, login, prepareWechatLogin, wechatLogin, sessionRevision } from '../../lib/store'
	import { getAgreements, listDevAccounts } from '../../api/auth.api'
	import { updateProfile } from '../../api/users.api'
	import type { User, Agreements, Consent } from '../../types/models'
	import { token, errorMessage, errorStatus, isCancelled } from '../../api/client'
	import { chooseUploadImage } from '../../lib/media'
	import { go, notify, assetUrl } from '../../lib/format'
	const accounts = ref<User[]>([]), agreements = ref<Agreements | null>(null), loading = ref(true), error = ref(''), submitting = ref(''), accepted = ref(false), devOpen = ref(false), profileOffer = ref(false), profileBusy = ref(false)
	const draft = reactive({ nickname: '', avatar: '' })
	/** 复用已核验的三款官方游戏图片，仅作为独立主视觉。 */
	const loginCovers = ['/static/covers/official/delta-3.jpg', '/static/covers/official/honor-1.jpg', '/static/covers/official/peace-1.jpg']
	const accountDescriptions : Record<string, string> = { player: '玩家 · 下单与验收', worker: '打手 · 发布服务与交付', clubowner: '俱乐部管理员', admin: '平台管理与纠纷处理', outsider: '独立玩家 · 自由接单' }
	let next = '/pages/main/index?tab=me', operation = 0, disposed = false
	/** 加载当前协议版本，任何重新加载都要求再次主动勾选。 */
	async function load() { loading.value = true; error.value = ''; accepted.value = false; try { await init(); agreements.value = await getAgreements(); if (state.devMode) accounts.value = await listDevAccounts() || [] } catch (err) { error.value = errorMessage(err) } finally { loading.value = false } }
	/** 只接受复选框的明确勾选事件，不因点击登录按钮默认同意。 */
	function changeConsent(event : { detail : { value : string[] } }) { accepted.value = (event.detail.value || []).includes('accepted') }
	/** 打开公开协议全文，阅读后返回仍需用户自行选择。 */
	function openAgreement(type : keyof Agreements) { if (!submitting.value) go('/pages/account/agreements?type=' + type) }
	/** 创建已明确同意的协议版本记录。 */
	function consent() : Consent | null { if (!accepted.value || !agreements.value) { notify('请先阅读并勾选用户协议和隐私政策'); return null } return { agreed: true, user_agreement_version: agreements.value.user_agreement.version, privacy_policy_version: agreements.value.privacy_policy.version } }
	/** 处理协议过期，将新版内容交给用户重新阅读并勾选。 */
	async function loginFailed(err : unknown) { if (errorStatus(err) === 409) { await load(); error.value = '协议已更新，请阅读后重新勾选。' } else { const detail = errorMessage(err); error.value = /INVALID_LOGIN.*access_token expired/.test(detail) ? '微信开发者工具登录态已过期，请重新登录开发者工具后重试' : detail || '暂时无法登录，请重试'; notify(error.value) } }
	/** 登录成功后展示可关闭的头像昵称弹层，不强制提供额外资料。 */
	function offerProfile(user : User | null) { if (!user) return; devOpen.value = false; draft.nickname = user.nickname || ''; draft.avatar = user.avatar || ''; profileOffer.value = true }
	/** 本地账号也提交用户主动同意的协议版本，保持正式流程一致。 */
	async function selectAccount(account : User) { if (submitting.value) return; const agreed = consent(); if (!agreed) { devOpen.value = false; return } const current = ++operation; submitting.value = account.id; error.value = ''; try { const user = await login(account.id, agreed); if (current === operation) offerProfile(user) } catch (err) { if (current === operation) await loginFailed(err) } finally { if (current === operation) submitting.value = '' } }
	/** 先用微信 code 换取短期票据，核对协议后才显式提交登录。 */
	async function realLogin() {
		if (submitting.value) return
		const agreed = consent(); if (!agreed) return
		const agreementSnapshot : Consent = { ...agreed }
		// #ifdef H5
		if (state.devMode) { devOpen.value = true; return }
		notify('请在微信小程序内登录'); return
		// #endif
		// #ifdef MP-WEIXIN
		const current = ++operation, revision = sessionRevision(); submitting.value = 'wechat'; error.value = ''
		try {
			const result = await new Promise<UniApp.LoginRes>((resolve, reject) => uni.login({ provider: 'weixin', success: resolve, fail: reject }))
			const prepared = await prepareWechatLogin(result.code)
			if (current !== operation || revision !== sessionRevision()) return
			if (prepared.agreements.user_agreement.version !== agreementSnapshot.user_agreement_version || prepared.agreements.privacy_policy.version !== agreementSnapshot.privacy_policy_version) { agreements.value = prepared.agreements; accepted.value = false; error.value = '协议已更新，请阅读后重新勾选。'; return }
			const user = await wechatLogin(prepared.ticket, agreementSnapshot)
			if (current === operation) offerProfile(user)
		} catch (err) { if (current === operation) await loginFailed(err) } finally { if (current === operation) submitting.value = '' }
		// #endif
	}
	/** 通过统一裁剪压缩上传流程选择头像，取消时保持现有草稿。 */
	async function pickAvatar() { if (profileBusy.value) return; const revision = sessionRevision(), savedToken = token(); profileBusy.value = true; try { const url = await chooseUploadImage('avatar'); if (!disposed && revision === sessionRevision() && savedToken === token() && url) draft.avatar = url } catch (err) { if (!disposed && revision === sessionRevision() && !isCancelled(err)) notify(err) } finally { profileBusy.value = false } }
	/** 保存可选资料前检查长度，并丢弃切换会话后的返回结果。 */
	async function saveOptionalProfile() { if (profileBusy.value || !state.user) return; const nickname = draft.nickname.trim(); if (Array.from(nickname).length < 2 || Array.from(nickname).length > 24) return notify('昵称需要 2—24 个字'); const revision = sessionRevision(), savedToken = token(); profileBusy.value = true; try { const user = await updateProfile({ nickname, avatar: draft.avatar }); if (!disposed && revision === sessionRevision() && savedToken === token()) { state.user = user; profileOffer.value = false; go(next) } } catch (err) { if (revision === sessionRevision()) notify(err) } finally { profileBusy.value = false } }
	/** 关闭可选资料弹层并返回原本想访问的页面。 */
	function finishLogin() { if (profileBusy.value) return; profileOffer.value = false; go(next) }
	/** 只允许返回站内页面，避免登录后的跳转离开应用。 */
	function applyRoute(options : Record<string, string | undefined> = {}) { let target = ''; try { target = options.next ? decodeURIComponent(options.next) : '' } catch { } next = target.startsWith('/pages/') && !target.startsWith('/pages/account/login') ? target : '/pages/main/index?tab=me'; load() }
	onLoad(applyRoute)
	onHide(() => { operation++; submitting.value = '' })
	onUnload(() => { disposed = true; operation++ })
</script>
<style scoped>
	.login-page {
		min-height: calc(100vh - 120px);
		display: flex;
		flex-direction: column;
		padding-bottom: 12px
	}

	.login-top {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		min-height: 28px
	}

	.skip-login {
		color: var(--muted);
		font-size: 11px
	}

	.login-copy {
		padding: 12px 8px 16px
	}

	.login-brand {
		display: flex;
		align-items: center;
		gap: 7px;
		margin-bottom: 12px;
		color: #bdd0c3;
		font-size: 14px;
		font-weight: 650
	}

	.login-headline {
		font-size: 32px;
		line-height: 40px;
		font-weight: 800;
		letter-spacing: -.8px
	}

	.login-headline-accent {
		color: var(--accent)
	}

	.login-subtitle {
		display: block;
		font-size: 12px;
		line-height: 20px;
		color: var(--muted);
		margin-top: 8px
	}

	.game-montage {
		position: relative;
		display: flex;
		flex-shrink: 0;
		height: 190px;
		margin: 0 -16px 20px;
		overflow: hidden
	}

	.montage-part {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		transform: skew(-8deg);
		border-right: 3px solid var(--bg)
	}

	.montage-image {
		display: block;
		width: 150%;
		height: 100%;
		margin-left: -25%;
		transform: skew(8deg)
	}

	.montage-shade {
		position: absolute;
		inset: 0;
		background: linear-gradient(180deg, var(--bg), transparent 18%, transparent 78%, var(--bg))
	}

	.login-benefits {
		display: flex;
		gap: 12px;
		margin: 0 0 24px;
		color: var(--accent)
	}

	.login-benefit {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-width: 0;
		font-size: 11px;
		color: var(--accent)
	}

	.login-benefit+.login-benefit {
		border-left: 1px solid var(--line-subtle)
	}

	.login-primary {
		width: 100%;
		gap: 8px
	}

	.consent-line {
		display: flex;
		align-items: flex-start;
		justify-content: center;
		gap: 0;
		margin-top: 16px;
		font-size: 10px;
		line-height: 22px;
		color: var(--muted)
	}

	.consent-checkbox-label {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px
	}

	.consent-checkbox {
		transform: scale(.7);
		transform-origin: center center
	}

	.consent-copy {
		min-width: 0;
		max-width: calc(100% - 24px)
	}

	.agreement-link {
		color: var(--accent)
	}

	.dev-toggle {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 4px;
		min-height: 28px;
		margin: 16px auto 0;
		padding: 0 8px;
		border: 0;
		border-radius: 0;
		background: transparent;
		color: var(--muted);
		font-size: 11px
	}

	.login-error {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		color: #efb299;
		font-size: 12px;
		line-height: 20px;
		margin-bottom: 16px
	}

	.login-error-copy {
		flex: 1;
		min-width: 0;
		overflow-wrap: anywhere;
		word-break: break-word
	}

	.retry-login {
		flex-shrink: 0;
		min-height: 28px;
		padding: 0;
		color: var(--accent);
		background: transparent;
		font-size: 11px
	}

	.login-mask {
		position: fixed;
		inset: 0;
		z-index: 110;
		background: #000b;
		display: flex;
		align-items: flex-end
	}

	.login-sheet {
		width: 100%;
		max-width: 600px;
		margin: 0 auto;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 18px 18px 0 0;
		padding: 20px 16px calc(24px + env(safe-area-inset-bottom));
		box-sizing: border-box
	}

	.sheet-heading {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
		font-size: 18px;
		font-weight: 650
	}

	.sheet-close {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		padding: 0;
		background: transparent;
		color: var(--muted)
	}

	.sheet-hint {
		display: block;
		font-size: 12px;
		color: var(--muted);
		line-height: 20px;
		margin: 8px 0 16px
	}

	.accounts-scroll {
		height: 44vh;
		max-height: 400px
	}

	.account-card {
		display: flex;
		align-items: center;
		gap: 12px;
		width: 100%;
		padding: 12px 0;
		background: transparent;
		border: 0;
		border-bottom: 1px solid var(--line-subtle);
		border-radius: 0;
		text-align: left
	}

	.account-copy {
		flex: 1;
		min-width: 0
	}

	.account-avatar {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		width: 36px;
		height: 36px;
		border-radius: 50%;
		background: #ffffff08;
		color: var(--accent);
		font-size: 18px
	}

	.account-name {
		display: block;
		min-width: 0;
		max-width: 100%;
		overflow-wrap: anywhere;
		word-break: break-word;
		font-size: 14px;
		font-weight: 650
	}

	.account-detail {
		display: block;
		min-width: 0;
		max-width: 100%;
		overflow-wrap: anywhere;
		word-break: break-word;
		font-size: 11px;
		color: var(--muted);
		margin-top: 4px
	}

	.offer-avatar {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 64px;
		height: 64px;
		padding: 0;
		border: 1px solid var(--line);
		border-radius: 50%;
		background: #ffffff04;
		color: var(--accent);
		margin: 20px auto;
		overflow: hidden
	}

	.offer-avatar-image {
		display: block;
		width: 100%;
		height: 100%
	}

	.offer-field {
		padding: 8px 0 20px;
		margin-bottom: 20px
	}

	.finish-later {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 100%;
		min-height: 36px;
		margin-top: 8px;
		background: transparent;
		color: var(--muted);
		font-size: 12px
	}

	@media(max-width:350px) {
		.login-headline {
			font-size: 29px;
			line-height: 37px
		}

		.game-montage {
			height: 170px
		}

		.login-copy {
			padding-top: 8px
		}

		.login-benefits {
			gap: 8px
		}

		.consent-line {
			font-size: 10px
		}

		.login-brand {
			margin-bottom: 8px
		}
	}
</style>