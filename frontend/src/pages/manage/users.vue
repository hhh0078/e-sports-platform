<!-- 文件作用：用户管理安全摘要与手机号服务端搜索，完整操作进入按权限控制的用户详情。 -->
<template><AppShell title="用户管理" back><ManageList v-model:query="query" placeholder="搜索昵称、用户ID或手机号" :loading="loading" :busy="busy" :error="error" :empty="!users.length" :has-more="hasMore" :count="users.length" @search="loadData()" @more="loadData(true)" @retry="retryFailed" @prepare="showPage"><button v-for="user in users" :key="user.id" class="user-row" @click="openUser(user)"><view class="avatar user-avatar"><image v-if="user.avatar" class="avatar-image" :src="assetUrl(user.avatar)" mode="aspectFill"/><text v-else>{{user.nickname?.slice(0,1)||'用'}}</text></view><view class="user-copy"><view class="user-heading"><text class="user-name">{{user.nickname||'未设置昵称'}}</text><text class="role-label">{{platformRoleName(user)}}</text></view><text class="user-meta">ID: {{user.id}}</text><text class="user-meta">{{user.phone_masked||(user.phone_verified?'手机已验证':'手机未验证')}}</text></view><text :class="['user-status',{frozen:user.status==='frozen'}]">{{user.status==='frozen'?'已冻结':'正常'}}</text><Icon name="arrow" :size="14"/></button></ManageList></AppShell></template>
<script setup lang="ts">
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import Icon from '../../components/Icon.vue'
import {useManageResource} from '../../lib/manage-resource'
import {assetUrl,go} from '../../lib/format'
import {platformRoleName} from '../../lib/management-permissions'
import type {User} from '../../types/models'
const {items:users,query,loading,busy,error,hasMore,loadData,retryFailed,showPage,currentSession,sameSession}=useManageResource('users')
/** 详情只传公开用户ID，不能从列表携带或修改管理员权限。 */
function openUser(user:User):void{if(!sameSession(currentSession())||!users.value.some(item=>item.id===user.id))return;go('/pages/manage/users/detail?id='+encodeURIComponent(user.id))}
</script>
<style scoped>
.user-row{display:flex;align-items:center;gap:23.0769rpx;min-width:0;width:100%;background:transparent;border-radius:0;text-align:left;color:var(--text);padding:30.7692rpx 0;border-bottom:1px solid var(--line-subtle)}.user-avatar{width:92.3077rpx;height:92.3077rpx;margin-top:3.8462rpx}.avatar-image{display:block;width:100%;height:100%;border-radius:50%}.user-copy{display:flex;flex-direction:column;gap:9.6154rpx;flex:1;min-width:0}.user-heading{display:flex;align-items:center;flex-wrap:wrap;gap:15.3846rpx;min-width:0}.user-name{font-size:30.7692rpx;line-height:46.1538rpx;font-weight:650;overflow-wrap:anywhere}.role-label{max-width:100%;font-size:21.1538rpx;line-height:34.6154rpx;padding:1.9231rpx 13.4615rpx;border-radius:23.0769rpx;color:var(--muted);background:#ffffff07;overflow-wrap:anywhere}.user-meta{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.user-status{font-size:23.0769rpx;color:var(--accent)}.frozen{color:#e9b69d}@media(max-width:350px){.user-row{gap:15.3846rpx}.user-avatar{width:76.9231rpx;height:76.9231rpx}}
</style>
