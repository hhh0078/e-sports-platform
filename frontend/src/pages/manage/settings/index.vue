<!-- 文件作用：按九类组织平台设置，多项分类提供二级列表，不预读配置秘密。 -->
<template>
  <AppShell :title="selectedGroup?.label||'平台设置'" back>
    <view v-if="error" class="error-box"><text>{{error}}</text><button class="text-btn" @click="load">重新加载</button></view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="设置和操作权限仍由服务端核验。" icon="shield"/>
    <template v-else-if="ready">
      <template v-if="groupID&&!selectedGroup"><text class="subtle-note">设置分类不存在，请返回选择。</text><button class="text-btn" @click="back">返回设置</button></template>
      <template v-else>
        <text class="subtle-note settings-intro">{{selectedGroup?'选择需要维护的配置':'按业务选择设置，审核与交易待办在平台管理处理。'}}</text>
        <view class="settings-list"><button v-for="item in items" :key="item.label" class="settings-entry" @click="open(item)"><view class="settings-entry-icon"><Icon :name="item.icon" :size="21"/></view><view class="settings-entry-copy"><text class="settings-entry-title">{{item.label}}</text><text v-if="item.description" class="settings-entry-description">{{item.description}}</text></view><Icon name="arrow" :size="13"/></button></view>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue'
import {onLoad,onShow,onHide} from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import {ADMIN_SETTINGS_GROUPS} from '../../../lib/admin-navigation'
import type {AdminEntry} from '../../../lib/admin-navigation'
import {state,init,sessionRevision} from '../../../lib/store'
import {token,errorMessage} from '../../../api/client'
import {go} from '../../../lib/format'
import {back} from '../../../lib/navigation'
const groupID=ref(''),ready=ref(false),error=ref('')
const selectedGroup=computed(()=>ADMIN_SETTINGS_GROUPS.find(group=>group.id===groupID.value))
const items=computed<AdminEntry[]>(()=>selectedGroup.value?.entries||ADMIN_SETTINGS_GROUPS.map(group=>({label:group.label,description:group.description,icon:group.icon,route:group.entries.length===1?group.entries[0].route:'/pages/manage/settings/index?group='+group.id})))
let visible=false,version=0
/** 只初始化当前身份，离页和切号的迟到结果不开放设置列表。 */
async function load():Promise<void>{const current=++version,revision=sessionRevision(),entryToken=token();ready.value=false;error.value='';try{await init();if(visible&&current===version&&revision===sessionRevision()&&entryToken===token())ready.value=state.user?.role==='admin'}catch(reason){if(visible&&current===version)error.value=errorMessage(reason)}}
/** 所有跳转来自固定白名单，不信任路由查询中的目标地址。 */
function open(item:AdminEntry):void{if(ready.value&&state.user?.role==='admin')go(item.route)}
onLoad(options=>{groupID.value=options?.group||''})
onShow(()=>{visible=true;void load()})
onHide(()=>{visible=false;version++;ready.value=false})
</script>
<style scoped>
.settings-intro{padding:0 0 15.3846rpx}.settings-list{min-width:0}.settings-entry{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-width:0;min-height:123.0769rpx;padding:23.0769rpx 0;border:0;border-bottom:1px solid var(--line-subtle);border-radius:0;background:transparent;color:var(--muted);text-align:left}.settings-entry-icon{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:61.5385rpx;color:var(--accent)}.settings-entry-copy{flex:1;min-width:0}.settings-entry-title{display:block;color:var(--text);font-size:28.8462rpx;font-weight:600;line-height:42.3077rpx}.settings-entry-description{display:block;margin-top:7.6923rpx;color:var(--muted);font-size:23.0769rpx;line-height:34.6154rpx;overflow-wrap:anywhere}
</style>
