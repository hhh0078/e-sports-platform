<!-- 文件作用：管理员查看提现列表并进入来源审核，独立维护不含密钥的商家转账公开配置。 -->
<template>
  <AppShell title="钱包管理" back>
    <view v-if="bootstrapError" class="error-box bootstrap-error"><text>{{ bootstrapError }}</text><button class="text-btn" :disabled="initializing" @click="showPage">重新加载</button></view>
    <view v-if="state.ready && state.user?.role !== 'admin'" class="access-note"><text>仅平台管理员可访问</text><button class="text-btn" @click="go('/pages/account/login')">切换账号</button></view>
    <template v-else-if="state.user?.role === 'admin'">
      <view class="finance-links"><button class="finance-link" @click="go('/pages/manage/finance/index')"><view class="finance-link-copy"><text class="finance-link-title">资金来源审核</text><text class="hint finance-link-note">逐笔核验个人与俱乐部提现来源</text></view><Icon name="arrow" :size="16"/></button><button class="finance-link" @click="go('/pages/manage/finance/settings')"><view class="finance-link-copy"><text class="finance-link-title">交易规则</text><text class="hint finance-link-note">平台抽成与创建俱乐部押金</text></view><Icon name="arrow" :size="16"/></button></view>
      <view class="section-heading"><text class="section-title">提现申请</text><button class="text-btn" :disabled="listLoading" @click="loadList()"><Icon name="refresh" :size="14"/>刷新</button></view>
      <scroll-view scroll-x :show-scrollbar="false" class="status-scroll"><view class="status-filters"><button v-for="item in filters" :key="item.value" :class="['status-filter', { selected: status === item.value }]" @click="chooseStatus(item.value)">{{ item.label }}</button></view></scroll-view>
      <view v-if="listError" class="subtle-note error-copy">{{ listError }}<button class="text-btn" :disabled="listLoading" @click="loadList()">重试</button></view>
      <view v-if="listLoading && !withdrawals.length" class="loading-line">正在读取提现申请…</view>
      <view v-else-if="!withdrawals.length && !listError" class="empty-list"><Icon name="orders" :size="28"/><text>暂无{{ statusName(status) }}提现</text><text class="hint">用户提交申请后在此审核</text></view>
      <view v-for="item in withdrawals" :key="item.id" class="withdrawal-row">
        <view class="withdrawal-heading"><text class="amount">¥{{ money(item.amount_cents) }}</text><text class="state-label">{{ statusName(item.status) }}</text></view>
        <text class="user-id">{{ item.club_id?'俱乐部 '+item.club_id:'用户 '+item.user_id }}</text><text class="hint">申请于 {{ time(item.created_at) }}</text>
        <text v-if="item.failure_reason" class="failure-reason">{{ item.failure_reason }}</text>
        <view class="withdrawal-actions"><button class="text-btn" @click="openAudit(item.id)">{{ item.status==='pending_review'?'查看来源并审核':'来源与处理记录' }}<Icon name="arrow" :size="13"/></button></view>
      </view>
      <button v-if="hasMore" class="text-btn load-more" :disabled="listLoading" @click="loadList(true)">加载更多</button>
      <text v-if="loaded && !transferReady && !state.devMode" class="subtle-note">提现通道尚未就绪，请先核对下方商家转账及支付配置。</text>
      <view class="form-section">
        <view class="section-heading"><text class="section-title">商家转账配置</text><text class="ready-label">{{ transferReady ? '已就绪' : '未就绪' }}</text></view>
        <text class="subtle-note">{{ configurationNotice || '保存配置不代表微信商家转账场景已经获批。' }}</text>
        <view v-if="settingsError" class="subtle-note error-copy">{{ settingsError }}<button class="text-btn" :disabled="settingsLoading" @click="loadSettings()">重试</button></view>
        <text v-if="settingsLoading" class="hint">正在读取公开配置…</text>
        <view class="enabled-row"><text>启用真实提现</text><NativeSwitch :checked="form.withdrawal_enabled" :disabled="!loaded || saving" :color="game.primary_color" @change="changeEnabled"/></view>
        <view class="field"><text class="field-label">商家转账场景编号</text><input v-model="form.transfer_scene_id" class="input" maxlength="32" :disabled="!loaded || saving" placeholder="填写商户已获批的场景编号"/></view>
        <view class="field"><view class="section-heading"><text class="field-label">转账场景报备字段</text><button class="text-btn" :disabled="!loaded || saving || form.transfer_scene_report_infos.length >= 10" @click="addReport">添加字段</button></view><text class="hint">按获批场景填写，最多10项，每项名称和内容1—32字。</text><view v-for="(report, index) in form.transfer_scene_report_infos" :key="index" class="report-row"><input v-model="report.info_type" class="input" maxlength="32" :disabled="saving" placeholder="字段名"/><input v-model="report.info_content" class="input" maxlength="32" :disabled="saving" placeholder="字段内容"/><button class="text-btn" :disabled="saving" aria-label="删除报备字段" @click="removeReport(index)"><Icon name="close" :size="16"/></button></view></view>
        <view class="field"><text class="field-label">转账通知HTTPS地址</text><input v-model="form.transfer_notify_url" class="input" maxlength="2048" :disabled="!loaded || saving" placeholder="无查询参数的HTTPS回调地址"/></view>
        <view class="field"><text class="field-label">转账备注 · 最多32字</text><input v-model="form.transfer_remark" class="input" maxlength="32" :disabled="!loaded || saving"/></view>
        <view class="field"><text class="field-label">收款感知描述 · 最多10字</text><input v-model="form.user_recv_perception" class="input" maxlength="10" :disabled="!loaded || saving"/></view>
        <button class="text-btn payment-link" @click="go('/pages/manage/settings')">商户密钥与证书沿用支付配置<Icon name="arrow" :size="12"/></button>
        <view class="action-footer"><button class="text-btn" :disabled="!loaded || saving" @click="restoreSettings">取消修改</button><button class="btn" :disabled="!loaded || saving" @click="saveSettings">{{ saving ? '正在保存…' : '保存转账配置' }}</button></view>
      </view>
    </template>
    <view v-else-if="!bootstrapError" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import NativeSwitch from '../../components/NativeSwitch.vue'
import Icon from '../../components/Icon.vue'
import { state, game, init, sessionRevision } from '../../lib/store'
import { token, errorMessage } from '../../api/client'
import { listAdminWithdrawals, getWalletSettings, saveWalletSettings } from '../../api/wallet.api'
import type { WalletSettings, WalletSettingsResult, WalletWithdrawal } from '../../types/wallet'
import type { SwitchChangeEvent } from '../../types/uni-components'
import { go, notify, money, time } from '../../lib/format'

/** 提现过滤仅使用服务端已支持的六种状态。 */
type WithdrawalStatus = WalletWithdrawal['status']
const filters: Array<{ value: WithdrawalStatus; label: string }> = [{ value: 'pending_review', label: '待审核' }, { value: 'processing', label: '处理中' }, { value: 'wait_user_confirm', label: '待确认' }, { value: 'success', label: '成功' }, { value: 'failed', label: '失败' }, { value: 'rejected', label: '已拒绝' }]
const status = ref<WithdrawalStatus>('pending_review'), withdrawals = ref<WalletWithdrawal[]>([]), listLoading = ref(false), hasMore = ref(false), listError = ref('')
const form = reactive<WalletSettings>(emptySettings()), settingsLoading = ref(false), settingsError = ref(''), saving = ref(false), loaded = ref(false), transferReady = ref(false), configurationNotice = ref('')
const bootstrapError = ref(''), initializing = ref(false)
let visible = false, pageRevision = 0, listVersion = 0, listOffset = 0, settingsVersion = 0, saveVersion = 0, savedSettings = ''
let loadedSession: ReturnType<typeof currentSession> | null = null

/** 创建不含秘密的空配置，不预设商户获批场景。 */
function emptySettings(): WalletSettings { return { withdrawal_enabled: false, transfer_scene_id: '', transfer_scene_report_infos: [], transfer_notify_url: '', transfer_remark: '', user_recv_perception: '' } }
/** 捕获管理员、令牌、认证及页面生命周期版本。 */
function currentSession() { return { id: state.user?.id, token: token(), revision: sessionRevision(), page: pageRevision } }
/** 仅接受当前页面同一管理员会话的读取或写入结果。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean { return visible && state.user?.role === 'admin' && session.id === state.user.id && session.token === token() && session.revision === sessionRevision() && session.page === pageRevision }
/** 显示真实状态文案，不把处理中当作转账成功。 */
function statusName(value: WithdrawalStatus): string { return filters.find(item => item.value === value)?.label || value }
/** 只回填约定的公开配置，服务端额外字段不进入表单。 */
function setDraft(value: WalletSettings): void { Object.assign(form, { withdrawal_enabled: value.withdrawal_enabled, transfer_scene_id: value.transfer_scene_id, transfer_scene_report_infos: value.transfer_scene_report_infos.map(item => ({ info_type: item.info_type, info_content: item.info_content })), transfer_notify_url: value.transfer_notify_url, transfer_remark: value.transfer_remark, user_recv_perception: value.user_recv_perception }) }
/** 保存公开快照和服务端就绪状态，留给取消操作恢复。 */
function applySettings(value: WalletSettingsResult): void { setDraft(value.settings); savedSettings = JSON.stringify(form); transferReady.value = value.transfer_ready; configurationNotice.value = value.configuration_notice; loaded.value = true; loadedSession = currentSession() }
/** 分页读取当前状态申请，筛选或账号切换后的旧响应不会回填。 */
async function loadList(append = false): Promise<void> {
  if (!visible || state.user?.role !== 'admin' || (append && listLoading.value)) return
  const current = ++listVersion, session = currentSession(), selected = status.value, offset = append ? listOffset : 0
  listLoading.value = true; listError.value = ''
  try { const result = await listAdminWithdrawals({ status: selected, limit: 20, offset }); if (current !== listVersion || !sameSession(session) || selected !== status.value) return; const known = new Set(append ? withdrawals.value.map(item => item.id) : []); withdrawals.value = append ? [...withdrawals.value, ...result.filter(item => !known.has(item.id))] : result; listOffset = offset + result.length; hasMore.value = result.length === 20 }
  catch (error) { if (current === listVersion && sameSession(session)) listError.value = errorMessage(error) }
  finally { if (current === listVersion) listLoading.value = false }
}
/** 读取公开转账配置，不请求密钥或实名信息。 */
async function loadSettings(): Promise<void> {
  if (!visible || state.user?.role !== 'admin' || saving.value) return
  const current = ++settingsVersion, session = currentSession(); settingsLoading.value = true; settingsError.value = ''
  try { const value = await getWalletSettings(); if (current === settingsVersion && sameSession(session)) applySettings(value) }
  catch (error) { if (current === settingsVersion && sameSession(session)) settingsError.value = errorMessage(error) }
  finally { if (current === settingsVersion) settingsLoading.value = false }
}
/** 切换服务端状态分页，不混入上一状态的数据。 */
function chooseStatus(value: WithdrawalStatus): void { if (value === status.value) return; status.value = value; withdrawals.value = []; hasMore.value = false; void loadList() }
/** 所有提现操作必须先进入独立来源明细，旧列表不再绕过核验直接执行。 */
function openAudit(id:string):void{if(visible&&state.user?.role==='admin')go('/pages/manage/finance/detail?id='+encodeURIComponent(id))}
/** 只修改公开草稿开关，保存前不调用配置接口。 */
function changeEnabled(event: SwitchChangeEvent): void { form.withdrawal_enabled = event.detail.value }
/** 增加一条待填写的场景报备，真实字段不预填假数据。 */
function addReport(): void { if (loaded.value && !saving.value && form.transfer_scene_report_infos.length < 10) form.transfer_scene_report_infos.push({ info_type: '', info_content: '' }) }
/** 删除指定报备草稿，保存前不影响服务端配置。 */
function removeReport(index: number): void { if (!saving.value) form.transfer_scene_report_infos.splice(index, 1) }
/** 统一校验后端约束，生成仅含公开字段的保存对象。 */
function settingsBody(): WalletSettings {
  const body: WalletSettings = { withdrawal_enabled: form.withdrawal_enabled, transfer_scene_id: form.transfer_scene_id.trim(), transfer_scene_report_infos: form.transfer_scene_report_infos.map(item => ({ info_type: item.info_type.trim(), info_content: item.info_content.trim() })), transfer_notify_url: form.transfer_notify_url.trim(), transfer_remark: form.transfer_remark.trim(), user_recv_perception: form.user_recv_perception.trim() }
  if (body.transfer_scene_id.length > 32 || /[\r\n\t /?&]/.test(body.transfer_scene_id)) throw new Error('转账场景编号格式无效')
  if (body.transfer_notify_url && !/^https:\/\/[^\s/?#]+(?:\/[^\s?#]*)?$/.test(body.transfer_notify_url)) throw new Error('转账通知地址需要无查询参数的HTTPS地址')
  if (Array.from(body.transfer_remark).length > 32 || Array.from(body.user_recv_perception).length > 10 || body.transfer_scene_report_infos.length > 10) throw new Error('转账备注最多32字，收款感知最多10字，报备字段最多10项')
  if (body.transfer_scene_report_infos.some(item => [item.info_type, item.info_content].some(value => Array.from(value).length < 1 || Array.from(value).length > 32))) throw new Error('报备字段名称和内容均需1—32字')
  return body
}
/** 显式保存配置草稿，不把保存成功解释为微信转账资格通过。 */
async function saveSettings(): Promise<void> {
  if (!visible || !loaded.value || saving.value || state.user?.role !== 'admin') return
  let body: WalletSettings; try { body = settingsBody() } catch (error) { notify(error); return }
  const current = ++saveVersion, session = currentSession(); saving.value = true; settingsError.value = ''; settingsVersion++
  try { const value = await saveWalletSettings(body); if (current !== saveVersion || !sameSession(session)) return; applySettings(value); notify('公开配置已保存，实际转账资格以微信及服务端状态为准') }
  catch (error) { if (current === saveVersion && sameSession(session)) settingsError.value = errorMessage(error) }
  finally { if (current === saveVersion) saving.value = false }
}
/** 取消仅恢复同一管理员已经读取的公开配置，不发起写请求。 */
function restoreSettings(): void { if (!saving.value && savedSettings) { setDraft(JSON.parse(savedSettings) as WalletSettings); notify('未保存的修改已取消') } }
/** 清除旧管理员的资金列表、公开草稿和未确认操作。 */
function clearData(): void { withdrawals.value = []; listOffset = 0; hasMore.value = false; loaded.value = false; loadedSession = null; savedSettings = ''; setDraft(emptySettings()); transferReady.value = false; configurationNotice.value = ''; listError.value = ''; settingsError.value = ''; bootstrapError.value = '' }
/** 页面显示时核对身份；同管理员返回保留未保存的公开草稿。 */
async function showPage(): Promise<void> { visible = true; if (initializing.value) return; const revision = pageRevision; initializing.value = true; bootstrapError.value = ''; try { await init(); if (!visible || revision !== pageRevision) return; if (state.user?.role !== 'admin') { clearData(); return }; if (loadedSession && loadedSession.id === state.user.id && loadedSession.token === token() && loadedSession.revision === sessionRevision() && loaded.value) { await loadList(); return }; await Promise.all([loadList(), loadSettings()]) } catch (error) { if (visible && revision === pageRevision) bootstrapError.value = errorMessage(error) } finally { if (revision === pageRevision) initializing.value = false } }
/** 隐藏即作废回调和确认操作，不在客户端缓存资金信息。 */
function hidePage(): void { visible = false; pageRevision++; listVersion++; settingsVersion++; saveVersion++; if(settingsLoading.value||saving.value)loaded.value=false; listLoading.value = false; settingsLoading.value = false; initializing.value = false; saving.value = false }
watch([() => state.user?.id, () => state.user?.role, sessionRevision], () => { const wasVisible = visible; hidePage(); clearData(); if (wasVisible) void showPage() }, { flush: 'sync' })
onShow(showPage)
onHide(hidePage)
onUnload(() => { hidePage(); clearData() })
</script>
<style scoped>
.finance-links{display:flex;flex-direction:column;gap:16px;margin:0 0 32px}.finance-link{display:flex;align-items:center;justify-content:space-between;gap:16px;width:100%;min-height:80px;min-width:0;padding:16px;border-radius:10px;background:var(--surface);color:var(--text);text-align:left}.finance-link-copy{display:flex;flex:1;flex-direction:column;gap:6px;min-width:0}.finance-link-title{font-size:16px;line-height:24px}.finance-link-note{font-size:14px;line-height:22px;overflow-wrap:anywhere}
.section-heading{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0}.status-scroll{width:100%;max-width:100%;margin:12px 0}.status-filters{display:inline-flex;min-width:100%;gap:6px;white-space:nowrap}.status-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:28px;padding:0 12px;border:1px solid var(--line-subtle);border-radius:20px;background:transparent;color:var(--muted);font-size:11px}.status-filter.selected{background:var(--accent);color:var(--bg);border-color:var(--accent)}.empty-list{display:flex;flex-direction:column;align-items:center;gap:10px;padding:32px 0;color:var(--muted);font-size:13px}.withdrawal-row{display:flex;flex-direction:column;gap:6px;padding:20px 0;border-bottom:1px solid var(--line-subtle);min-width:0}.withdrawal-heading{display:flex;align-items:center;justify-content:space-between;gap:12px}.amount{display:block;font-size:23px;line-height:32px;font-weight:700;color:var(--accent);overflow-wrap:anywhere}.state-label,.ready-label{font-size:11px;color:var(--muted);flex-shrink:0}.user-id,.failure-reason{display:block;overflow-wrap:anywhere;font-size:12px;line-height:20px}.failure-reason,.error-copy{color:#e9b69d}.withdrawal-actions{display:flex;align-items:center;flex-wrap:wrap;gap:12px;margin-top:8px}.load-more{width:100%;margin-top:12px}.enabled-row{display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:52px;margin-bottom:20px;font-size:13px}.report-row{display:grid;grid-template-columns:repeat(2,minmax(0,1fr)) 28px;align-items:center;gap:8px;min-width:0}.report-row .input{font-size:12px}.payment-link{justify-content:flex-start;white-space:normal;text-align:left}.access-note{display:flex;flex-direction:column;align-items:center;gap:12px;padding:40px 0;color:var(--muted)}.action-footer{flex-wrap:wrap}.action-footer .btn{min-width:130px}@media(max-width:350px){.status-filter{padding:0 10px}.report-row{gap:6px}.report-row .input{font-size:11px}.withdrawal-actions{gap:8px}}
</style>
