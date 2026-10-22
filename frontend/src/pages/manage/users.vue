<!-- 文件作用：独立管理用户搜索与冻结审核，复用统一列表和身份隔离，不暴露其他管理资源。 -->
<template>
  <AppShell title="用户管理" back>
    <ManageList v-model:query="query" placeholder="搜索昵称或用户ID" :loading="loading" :busy="busy || confirming" :error="error" :empty="!users.length" :has-more="hasMore" :count="users.length" @search="loadData()" @more="loadData(true)" @prepare="showPage">
      <view v-for="user in users" :key="user.id" class="user-row">
        <view class="avatar user-avatar"><image v-if="user.avatar" class="avatar-image" :src="assetUrl(user.avatar)" mode="aspectFill"/><text v-else>{{ user.nickname?.slice(0, 1) || '用' }}</text></view>
        <view class="user-copy"><view class="user-heading"><text class="user-name">{{ user.nickname || '未设置昵称' }}</text><text class="role-label">{{ roleName(user) }}</text></view><text class="user-meta">ID: {{ user.id }}</text><view class="user-meta-line"><text :class="['user-status', { frozen: user.status === 'frozen' }]">{{ user.status === 'frozen' ? '已冻结' : '正常' }}</text><text class="user-meta">{{ user.phone_verified ? '手机已验证' : '手机未验证' }}</text></view><text class="user-bio">{{ user.bio || '暂未填写简介' }}</text></view>
        <button v-if="user.id !== state.user?.id" :class="['btn btn-small user-action', user.status === 'frozen' ? 'btn-secondary' : 'btn-danger']" :disabled="busy || confirming" @click="toggleUser(user)">{{ user.status === 'frozen' ? '解冻' : '冻结' }}</button><text v-else class="self-note">当前账号</text>
      </view>
    </ManageList>
  </AppShell>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import { useManageResource } from '../../lib/manage-resource'
import { moderateResource } from '../../api/admin.api'
import { state } from '../../lib/store'
import { assetUrl, notify } from '../../lib/format'
import type { User } from '../../types/models'
const confirming = ref(false)
let confirmationVersion = 0
const { items: users, query, loading, busy, error, hasMore, loadData, showPage, mutate, currentSession, sameSession } = useManageResource('users', { clearDraft: clearConfirmation })
/** 页面离开或身份变化时作废未确认操作，不让旧弹框恢复后执行冻结。 */
function clearConfirmation(): void { confirmationVersion++; confirming.value = false }
/** 使用服务端真实角色展示账号身份，不从昵称或认证猜测。 */
function roleName(user: User): string { return user.role === 'admin' ? '平台管理员' : user.role === 'clubowner' ? '俱乐部管理员' : '玩家 / 打手' }
/** 官方确认后冻结或恢复指定账号，确认期间切换身份则不再发出旧操作。 */
async function toggleUser(user: User): Promise<void> {
  if (busy.value || confirming.value || user.id === state.user?.id || state.user?.role !== 'admin') return
  const session = currentSession(), frozen = user.status === 'frozen', version = ++confirmationVersion; confirming.value = true
  try { const result = await uni.showModal({ title: frozen ? '解除账户冻结' : '冻结账户', content: frozen ? `确认恢复“${user.nickname || user.id}”的账户使用？` : `确认冻结“${user.nickname || user.id}”？冻结后该用户将无法继续交易。`, confirmText: frozen ? '确认解冻' : '确认冻结', confirmColor: frozen ? '#18794e' : '#aa4a35' }); if (version !== confirmationVersion || !result.confirm || !sameSession(session)) return; await mutate(() => moderateResource('users', user.id, { status: frozen ? 'active' : 'frozen' })) }
  catch (err) { if (version === confirmationVersion && sameSession(session)) notify(err) }
  finally { if (version === confirmationVersion) confirming.value = false }
}
</script>
<style scoped>
.user-row{display:flex;align-items:flex-start;gap:12px;min-width:0;padding:24px 0;border-bottom:1px solid var(--line-subtle)}.user-avatar{width:48px;height:48px;margin-top:2px}.avatar-image{display:block;width:100%;height:100%;border-radius:50%}.user-copy{display:flex;flex-direction:column;gap:5px;flex:1;min-width:0}.user-heading{display:flex;align-items:center;flex-wrap:wrap;gap:8px;min-width:0}.user-name{font-size:16px;line-height:24px;font-weight:650;overflow-wrap:anywhere}.role-label{max-width:100%;font-size:11px;line-height:18px;padding:1px 7px;border-radius:12px;color:var(--muted);background:#ffffff07;overflow-wrap:anywhere}.user-meta{font-size:12px;line-height:20px;color:var(--muted);overflow-wrap:anywhere}.user-meta-line{display:flex;align-items:center;gap:10px;flex-wrap:wrap}.user-status{font-size:12px;color:var(--accent)}.frozen{color:#e9b69d}.user-bio{font-size:13px;line-height:22px;color:var(--muted);overflow-wrap:anywhere}.user-action{flex-shrink:0;min-width:60px;margin-top:8px}.self-note{flex-shrink:0;max-width:60px;margin-top:16px;font-size:11px;color:var(--muted);line-height:18px}@media(max-width:350px){.user-row{gap:8px}.user-avatar{width:40px;height:40px}.user-action{min-width:48px;padding:0 8px}.self-note{max-width:44px}}
</style>
