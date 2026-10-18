<!-- 文件作用：以游戏主题展示个人资料、真实交易统计、接单身份及权限内的管理入口。 -->
<template>
  <AppShell immersive active="me">
    <SectionHero eyebrow="我的" art="profile" :title="state.user ? '' : '我的集结'" :subtitle="state.user ? '' : '你热爱的，都值得认真对待。'">
      <template v-if="state.user">
        <view class="profile-top">
          <view class="profile-avatar">{{ avatarLetter }}</view>
          <view class="profile-name"><view>{{ state.user.nickname }}</view><text>{{ state.user.role === 'admin' ? '平台管理员' : state.user.provider_enabled ? '玩家 · 打手' : '玩家' }}</text></view>
          <button class="profile-edit" :disabled="saving" @click="toggleEditor"><Icon name="edit" :size="18" /><text>{{ editing ? '收起编辑' : '编辑资料' }}</text></button>
        </view>
        <view class="profile-bio">{{ state.user.bio || '写下你的游戏经历，让队友更了解你。' }}</view>
      </template>
    </SectionHero>
    <view class="profile-content">
      <EmptyState v-if="!state.user" title="让每一次相遇，都有名字" detail="登录后查看订单、发布服务，建立你的玩家名片。" icon="user"><button class="btn" @click="go('/pages/login/index')">登录 / 开始体验</button></EmptyState>
      <template v-else>
        <view v-if="editing" class="panel edit-panel">
          <view class="field"><text class="field-label">昵称 · 2—24 个字</text><input v-model="form.nickname" class="input" maxlength="24" placeholder="你的玩家昵称" /></view>
          <view class="field"><text class="field-label">个人介绍 · 最多 300 个字</text><textarea v-model="form.bio" class="input" maxlength="300" placeholder="擅长什么游戏？哪些时段在线？你的服务风格是…" /></view>
          <view class="edit-actions"><button class="btn-secondary" :disabled="saving" @click="toggleEditor">取消</button><button class="btn" :disabled="saving" @click="saveProfile">{{ saving ? '保存中…' : '保存资料' }}</button></view>
        </view>
        <view class="stats-row">
          <view @click="go('/pages/orders/index?role=buyer')"><text>{{ stats.buyer_order_count ?? '—' }}</text><view>下单记录</view></view>
          <view @click="go('/pages/orders/index?role=provider&status=working')"><text>{{ stats.provider_working_count ?? '—' }}</text><view>进行中接单</view></view>
          <view @click="go('/pages/orders/index?role=all&status=completed')"><text>{{ stats.completed_order_count ?? '—' }}</text><view>完成订单</view></view>
        </view>
        <view class="provider-switch profile-panel">
          <view class="switch-copy"><view class="switch-label">接单身份</view><text class="hint">{{ state.user.provider_enabled ? '已开启，可以发布服务和领取需求。' : '无需俱乐部，开启后自由接单。' }}</text></view>
          <switch :key="switchVersion" :checked="state.user.provider_enabled" :color="game.primary_color" :disabled="saving" @change="toggleProvider" />
        </view>
        <view v-if="state.user.provider_enabled" class="earnings-panel profile-panel" @click="go('/pages/orders/index?role=provider&status=completed')">
          <view class="earnings-copy"><text class="earnings-label">已完成订单收入</text><view class="balance"><text class="currency">¥</text>{{ stats.earnings_cents === undefined ? '—' : money(stats.earnings_cents) }}</view><text class="hint">{{ state.devMode ? '测试环境统计，不是真实可提现余额。' : '按已完成订单统计，不是可提现余额。' }}</text></view><Icon name="wallet" :size="31" />
        </view>
        <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load">重新加载</button></view>
        <view class="menu-list profile-panel">
          <view class="menu-item" @click="go('/pages/orders/index')"><Icon name="orders" :size="25" /><view class="menu-copy"><text>我的订单</text><view>交易进度、沟通与售后</view></view><Icon name="arrow" :size="16" /></view>
          <view class="menu-item" @click="go('/pages/clubs/index?mine=1')"><Icon name="club" :size="26" /><view class="menu-copy"><text>我的俱乐部</text><view>查看我的俱乐部与挂靠</view></view><Icon name="arrow" :size="16" /></view>
          <view class="menu-item" @click="go('/pages/worker/index')"><Icon name="work" :size="26" /><view class="menu-copy"><text>打手工作台</text><view>管理服务，发现新需求</view></view><Icon name="arrow" :size="16" /></view>
          <view v-if="state.user.role === 'admin'" class="menu-item admin-entry" @click="go('/pages/admin/index')"><Icon name="settings" :size="25" /><view class="menu-copy"><text>平台管理中心</text><view>数据、审核、游戏主题与订单纠纷</view></view><Icon name="arrow" :size="16" /></view>
          <view v-if="state.devMode" class="menu-item" @click="go('/pages/login/index')"><Icon name="refresh" :size="26" /><view class="menu-copy"><text>切换体验身份</text><view>体验玩家、打手与管理角色</view></view><Icon name="arrow" :size="16" /></view>
        </view>
        <button class="logout-button profile-panel" :disabled="saving" @click="signOut"><Icon name="logout" :size="25" /><text>退出登录</text><Icon name="arrow" :size="16" /></button>
      </template>
    </view>
  </AppShell>
</template>
<script setup>
import { computed, ref, reactive } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import SectionHero from '../../components/SectionHero.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, game, init, logout, clearSession, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { money, go, notify } from '../../lib/format'
const switchVersion = ref(0)
const stats = ref({}), editing = ref(false), saving = ref(false), error = ref(''), form = reactive({ nickname: '', bio: '' })
let loadVersion = 0, visible = true
/** 记录发起操作时的会话、账号及版本，避免旧响应进入新身份页面。 */
function currentSession() { return { token: token(), userId: state.user?.id, revision: sessionRevision() } }
/** 确认当前账号仍是操作发起时的会话。 */
function sameSession(session) { return session.token === token() && session.userId === state.user?.id && session.revision === sessionRevision() }
/** 让早先读取失效，并清除不再保证属于当前界面的统计。 */
function invalidateLoad() { stats.value = {}; return ++loadVersion }
/** 只有当前页面版本与会话都未变化，才允许提交异步返回值。 */
function currentRequest(version, session) { return visible && version === loadVersion && sameSession(session) }
/** 按完整 Unicode 字符生成头像，避免表情昵称被截断。 */
const avatarLetter = computed(() => Array.from(state.user?.nickname || '集')[0])
/** 将服务端资料复制到编辑表单，避免取消后留下未保存内容。 */
function resetForm() {
  form.nickname = state.user?.nickname || ''
  form.bio = state.user?.bio || ''
}
/** 切换资料编辑状态，每次打开都使用当前已保存资料。 */
function toggleEditor() {
  if (saving.value) return
  resetForm()
  editing.value = !editing.value
}
/** 读取最新个人资料与订单汇总，页面返回时同步变化。 */
async function load() {
  const version = invalidateLoad()
  let session
  error.value = ''
  try {
    await init()
    if (!visible || version !== loadVersion) return
    if (!state.user) { editing.value = false; return }
    session = currentSession()
    const user = await api('/me')
    if (!currentRequest(version, session)) return
    state.user = user
    if (!editing.value) resetForm()
    const summary = await api('/me/stats')
    if (currentRequest(version, session)) stats.value = summary || {}
  } catch (err) {
    if (!visible || version !== loadVersion || (session && !sameSession(session))) return
    if (err.status === 401) { clearSession(); editing.value = false }
    error.value = err.message
  }
}
/** 按后端字数限制保存个人资料，仅提交昵称与介绍。 */
async function saveProfile() {
  if (saving.value || !state.user) return
  const nickname = form.nickname.trim(), bio = form.bio.trim(), nicknameLength = Array.from(nickname).length
  if (nicknameLength < 2 || nicknameLength > 24) return notify('昵称需要 2—24 个字')
  if (Array.from(bio).length > 300) return notify('个人介绍不能超过 300 个字')
  const version = invalidateLoad(), session = currentSession()
  saving.value = true
  try {
    const user = await api('/me', 'PATCH', { nickname, bio })
    if (!currentRequest(version, session)) return
    state.user = user
    editing.value = false
    resetForm()
    notify('资料已保存')
  } catch (err) { if (currentRequest(version, session)) notify(err) }
  finally { saving.value = false; if (visible && sameSession(session)) load() }
}
/** 开关接单身份；请求失败时按服务端原值重新渲染开关。 */
async function toggleProvider(event) {
  if (saving.value || !state.user) { switchVersion.value++; return }
  const enabled = event.detail.value
  if (enabled && !state.user.bio?.trim()) {
    resetForm()
    editing.value = true
    notify('请先填写个人介绍并保存')
    switchVersion.value++
    return
  }
  const version = invalidateLoad(), session = currentSession()
  saving.value = true
  try {
    const user = await api('/me', 'PATCH', { provider_enabled: enabled })
    if (!currentRequest(version, session)) return
    state.user = user
    notify(enabled ? '接单身份已开启' : '接单身份已关闭')
  } catch (err) { if (currentRequest(version, session)) notify(err) }
  finally { saving.value = false; switchVersion.value++; if (visible && sameSession(session)) load() }
}
/** 撤销登录会话并恢复访客界面。 */
async function signOut() {
  if (saving.value) return
  const version = invalidateLoad()
  saving.value = true
  const task = logout(), session = currentSession()
  try {
    await task
    if (!visible || version !== loadVersion || token() || state.user) return
    editing.value = false
    notify('已退出登录')
  } catch (err) { if (currentRequest(version, session)) notify(err) } finally { saving.value = false }
}
/** 页面重新可见后刷新资料；隐藏时禁止在途请求恢复旧界面。 */
function showPage() { visible = true; load() }
/** 离开页面时清空统计并使旧读取和操作回调失效。 */
function hidePage() { visible = false; invalidateLoad() }
onShow(showPage)
onHide(hidePage)
</script>
<style scoped>
.profile-top{display:flex;align-items:center;gap:14px}.profile-avatar{width:58px;height:58px;flex-shrink:0;display:flex;align-items:center;justify-content:center;border-radius:15px;background:var(--accent);color:#032118;font-size:29px;font-weight:800}.profile-name{flex:1;min-width:0}.profile-name>view{font-size:25px;line-height:1.25;font-weight:800;color:#fff;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.profile-name>text{display:block;margin-top:5px;font-size:12px;font-weight:600;color:var(--accent)}.profile-edit{display:flex;align-items:center;justify-content:center;gap:6px;flex-shrink:0;border:1px solid #ffffff12;border-radius:18px;padding:6px 8px;line-height:20px;font-size:12px;color:#e0e6e5;background:#07120f55}.profile-edit[disabled]{opacity:.5}.profile-bio{margin-top:10px;font-size:13px;line-height:20px;color:#b4bfc0;white-space:pre-wrap;overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;min-height:40px}.profile-content{display:flex;flex-direction:column;gap:11px;padding:0 14px}.profile-panel,.stats-row{border:1px solid #ffffff16;border-radius:11px;background:#ffffff01}.stats-row{display:flex;align-items:center;min-height:74px;padding:11px 0}.stats-row>view{flex:1;min-width:0;text-align:center;border-right:1px solid #ffffff14;cursor:pointer}.stats-row>view:last-child{border:0}.stats-row>view>text{font-size:27px;line-height:28px;font-weight:800;color:var(--text)}.stats-row>view>view{margin-top:3px;font-size:12px;line-height:18px;color:#aab8b9}.provider-switch{display:flex;align-items:center;justify-content:space-between;gap:6px;min-height:67px;padding:12px 14px}.switch-copy{flex:1;min-width:0}.switch-label{font-size:15px;line-height:22px;font-weight:750;margin-bottom:2px}.provider-switch .hint{display:block;font-size:12px;line-height:18px;color:#aab8b9}.provider-switch switch{width:49px;flex-shrink:0;transform:scale(.76);transform-origin:right center}.earnings-panel{display:flex;align-items:center;justify-content:space-between;gap:9px;min-height:104px;padding:12px 14px;color:var(--accent);cursor:pointer}.earnings-copy{flex:1;min-width:0}.earnings-label{display:block;font-size:13px;line-height:18px;color:#aab8b9}.balance{margin:3px 0 5px;font-size:31px;line-height:34px;font-weight:800;letter-spacing:-.7px}.currency{font-size:24px;margin-right:1px}.earnings-panel .hint{display:block;font-size:12px;line-height:18px;color:#aab8b9}.menu-list{padding:0;overflow:hidden}.menu-item{display:flex;align-items:center;gap:19px;min-height:61px;padding:10px 17px;border-bottom:1px solid #ffffff12;color:#aab8b9;cursor:pointer}.menu-item:last-child{border:0}.menu-copy{flex:1;min-width:0}.menu-copy>text{font-size:15px;line-height:21px;font-weight:750;color:var(--text)}.menu-copy>view{font-size:12px;line-height:18px;color:#aab8b9;margin-top:1px}.admin-entry{color:var(--accent)}.logout-button{display:flex;align-items:center;gap:19px;width:100%;min-height:44px;padding:9px 17px;line-height:24px;font-size:14px;color:#aab8b9;text-align:left}.logout-button>text{flex:1}.logout-button[disabled]{opacity:.5}.edit-panel{margin-bottom:1px;padding:16px}.edit-actions{display:flex;gap:9px}.edit-actions>button{flex:1}.error-box{font-size:12px;padding:12px}.error-box .text-btn{margin-top:5px}@media(max-width:350px){.profile-top{gap:9px}.profile-avatar{width:50px;height:50px;font-size:26px}.profile-name>view{font-size:22px}.profile-edit{font-size:10px;gap:3px;padding:5px}.profile-content{padding:0 12px}.provider-switch{padding:11px}.provider-switch .hint{font-size:11px}.menu-item{gap:14px;padding-left:14px;padding-right:14px}.earnings-panel .hint{font-size:11px}}
</style>
