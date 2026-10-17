<!-- 文件作用：本地测试身份切换与正式微信授权登录入口。 -->
<template><AppShell title="欢迎加入集结" subtitle="每个人都是玩家，也可以成为值得信赖的队友。" back>
  <view class="login-brand"><view class="login-mark">集</view><view>找队友，也成为队友</view><text>PLAY TOGETHER. GROW TOGETHER.</text></view>
  <view v-if="error" class="error-box stack"><text>{{ error }}</text><button class="btn btn-small btn-secondary" @click="load">重试</button></view>
  <view v-else-if="loading" class="loading-line">正在加载登录方式…</view>
  <template v-else><view v-if="state.devMode" class="stack"><view class="notice"><view class="local-title"><Icon name="info" :size="16"/><text>本地测试环境</text></view><view>选择一个体验身份开始测试。所有支付均为测试支付，不会扣费。正式环境会关闭此入口。</view></view><view class="section-row"><text class="section-title">选择体验身份</text><text class="eyebrow">LOCAL ACCOUNTS</text></view><button v-for="account in accounts" :key="account.id" class="account-card" :disabled="!!submitting" @click="selectAccount(account)"><view class="avatar">{{ account.nickname.slice(0,1) }}</view><view class="account-copy"><view>{{ account.nickname }}<text v-if="state.user?.id === account.id" class="current">当前</text></view><text>{{ accountDescriptions[account.id] || '体验玩家与接单能力' }}</text></view><Icon name="arrow" :size="17"/></button></view><view v-else class="stack"><button class="btn" :disabled="!!submitting" @click="realLogin">{{ submitting ? '登录中…' : '微信授权登录' }}</button><view class="notice">使用微信身份创建账号，不会读取你的聊天记录。登录后可自由选择玩家与打手身份。</view></view><view class="login-agreement">继续使用即表示你愿意遵守平台交易规则：<view>真实沟通、如约交付、不发布违规内容。</view></view></template>
</AppShell></template>
<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state, init, login, wechatLogin } from '../../lib/store'
import { api } from '../../lib/api'
import { go, notify } from '../../lib/format'
const accounts = ref([]), loading = ref(true), error = ref(''), submitting = ref('')
const accountDescriptions = { player: '玩家 · 下单、验收、退款与评价', worker: '打手 · 发布服务、接单与交付', clubowner: '俱乐部管理员 · 审核挂靠与订单协作', admin: '平台管理员 · 审核、主题与纠纷处理', outsider: '独立玩家 · 权限隔离与自由接单体验' }
let next = '/pages/me/index'
/** 加载服务端允许的登录方式，开发账号仅在开发模式查询。 */
async function load() { loading.value = true; error.value = ''; try { await init(); if (state.devMode) accounts.value = await api('/dev/accounts') || [] } catch (err) { error.value = err.message } finally { loading.value = false } }
/** 切换本地体验身份，并返回用户原本想访问的页面。 */
async function selectAccount(account) { if (submitting.value) return; submitting.value = account.id; try { await login(account.id); notify('已切换为' + account.nickname); go(next) } catch (err) { notify(err) } finally { submitting.value = '' } }
/** 使用小程序官方登录能力获取 code，交给服务端换取真实身份。 */
async function realLogin() {
  // #ifdef MP-WEIXIN
  submitting.value = 'wechat'
  try { const result = await new Promise((resolve, reject) => uni.login({ provider: 'weixin', success: resolve, fail: reject })); await wechatLogin(result.code); go(next) } catch (err) { notify(err.message ? err : new Error('微信登录失败，请重试')) } finally { submitting.value = '' }
  // #endif
  // #ifndef MP-WEIXIN
  notify('请在微信小程序内使用微信登录')
  // #endif
}
onLoad(options => { let target = ''; try { target = options.next ? decodeURIComponent(options.next) : '' } catch { target = '' } next = target.startsWith('/pages/') && !target.startsWith('/pages/login/') ? target : '/pages/me/index'; load() })
</script>
<style scoped>.login-brand{text-align:center;margin:14px 0 38px}.login-mark{height:66px;width:66px;line-height:66px;border-radius:17px;background:var(--accent);color:#06180e;font-size:36px;font-weight:850;transform:skew(-5deg);margin:0 auto 19px}.login-brand>view:not(.login-mark){font-size:18px;font-weight:600}.login-brand>text{font-size:8px;letter-spacing:1.5px;color:#607d6a;display:block;margin-top:8px}.local-title{display:flex;gap:6px;align-items:center;color:var(--accent);font-weight:650;margin-bottom:6px}.section-row{margin:10px 0 0}.section-row .eyebrow{font-size:8px;letter-spacing:1px;color:#6b8977}.account-card{display:flex;align-items:center;text-align:left;width:100%;gap:12px;border:1px solid rgba(255,255,255,.09);border-radius:12px;background:var(--surface);padding:15px;color:var(--text);line-height:1.5}.account-card[disabled]{opacity:.5}.account-copy{flex:1}.account-copy>view{font-size:14px;font-weight:650}.account-copy>text{font-size:9px;color:#869e8f}.current{display:inline-block;margin-left:7px;color:var(--accent);font-size:9px;padding:1px 5px;border:1px solid var(--accent);border-radius:3px}.login-agreement{color:#587665;text-align:center;line-height:1.9;font-size:10px;margin-top:28px}</style>
