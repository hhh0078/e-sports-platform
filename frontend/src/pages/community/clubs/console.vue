<!-- 文件作用：提供俱乐部俱乐部控制台，将佣金押金、成员、团队订单和装修拆分为清晰入口。 -->
<template>
  <AppShell title="俱乐部管理" back>
    <view v-if="error" class="error-box" @click="retryFailed">{{ error }} · 点击重试</view>
    <view v-if="loading&&!club" class="loading-line">正在读取管理权限…</view>
    <template v-if="club&&canManage">
      <view class="club-console-identity"><image v-if="club.avatar_url" class="club-console-avatar" :src="assetUrl(club.avatar_url)" mode="aspectFill"/><view class="club-console-copy"><text class="club-console-title">{{ club.name }}</text><text class="subtle-note">俱乐部控制台</text></view></view>
      <view class="club-console-summary"><text>认证成员</text><text>{{ club.member_count }} 人</text></view>
      <SubscribeReminder :scenes="['club_member','club_dispute']" label="接收成员申请与争议提醒"/>
      <view class="club-console-links"><button v-for="item in links" :key="item.page" class="club-console-link" @click="go(item.page+encodeURIComponent(id))"><view class="club-console-icon"><Icon :name="item.icon" :size="22"/></view><view class="club-console-copy"><text class="club-console-label">{{ item.title }}</text><text class="subtle-note">{{ item.detail }}</text></view><Icon name="arrow" :size="14"/></button></view>
      <text class="subtle-note">押金、佣金和负责人的个人资产分别记账。资金操作须通过平台核验。</text>
    </template>
    <text v-else-if="club" class="subtle-note">仅俱乐部管理者可访问控制台。</text>
  </AppShell>
</template>
<script setup lang="ts">
import AppShell from '../../../components/AppShell.vue'
import SubscribeReminder from '../../../components/SubscribeReminder.vue'
import Icon from '../../../components/Icon.vue'
import {state,sessionRevision} from '../../../lib/store'
import { useNativePageRefresh } from '../../../lib/native-page-refresh'
import { useClubFunds } from '../../../lib/use-club-funds'
import { assetUrl, go } from '../../../lib/format'
const {id,club,canManage,loading,error,load,retryFailed}=useClubFunds('club_platform',{summaryOnly:true})
useNativePageRefresh({refresh:()=>load(),context:()=>[state.user?.id,state.user?.role,sessionRevision(),id.value]})
const links=[{title:'俱乐部资金',detail:'平台押金、佣金与提现',icon:'wallet',page:'/pages/community/clubs/funds?id='},{title:'加入规则与装修',detail:'抽成、备注、封面与图文',icon:'edit',page:'/pages/community/clubs/edit?id='},{title:'联系方式',detail:'客服二维码、电话与微信号',icon:'phone',page:'/pages/community/clubs/edit?section=contact&id='},{title:'成员管理',detail:'申请审核与认证',icon:'club',page:'/pages/community/clubs/detail?tab=manage&id='},{title:'团队订单',detail:'查看履约进度与售后',icon:'orders',page:'/pages/community/clubs/detail?tab=orders&id='}]
</script>
<style scoped>
.club-console-identity{display:flex;align-items:center;gap:30.7692rpx;min-width:0;padding:38.4615rpx 0}.club-console-avatar{width:107.6923rpx;height:107.6923rpx;border-radius:30.7692rpx;background:var(--surface);flex-shrink:0}.club-console-copy{flex:1;min-width:0}.club-console-title{display:block;font-size:44.2308rpx;line-height:61.5385rpx;font-weight:700;overflow-wrap:anywhere}.club-console-summary{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle);font-size:26.9231rpx;color:var(--muted)}.club-console-links{display:flex;flex-direction:column;gap:23.0769rpx;padding:53.8462rpx 0}.club-console-link{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-width:0;min-height:146.1538rpx;padding:23.0769rpx 30.7692rpx;border-radius:23.0769rpx;background:var(--surface);color:var(--muted);text-align:left}.club-console-icon{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:69.2308rpx;height:69.2308rpx;color:var(--accent)}.club-console-label{display:block;font-size:30.7692rpx;color:var(--text);line-height:46.1538rpx}
</style>
