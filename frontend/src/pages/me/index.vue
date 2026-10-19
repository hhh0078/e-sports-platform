<!-- 文件作用：保持访客和已登录用户一致的个人中心结构，提供订单、个人主页、资料与验证入口。 -->
<template>
  <AppShell immersive active="me">
    <view class="me-hero"><image class="me-background" :src="assetUrl(game.cover_url)" mode="aspectFill"/><view class="me-shade"/><view class="me-titlebar" :style="nativeHeader"><text>我的集结</text><button :style="{right:nativeHeader.paddingRight||'19px'}" @click="protectedGo('/pages/profile/index')"><Icon name="settings" :size="22"/></button></view><view class="profile-top" @click="openIdentity"><view class="profile-avatar"><image v-if="state.user?.avatar" :src="assetUrl(state.user.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="35"/></view><view class="profile-name"><view>{{ state.user?.nickname || '登录后查看' }}<Icon name="arrow" :size="19"/></view><text>{{ state.user?.bio || '找到队友，也成为队友' }}</text></view></view><view class="stats-row"><view @click="protectedGo('/pages/orders/index?role=buyer')"><text>{{ stat('buyer_order_count') }}</text><view>下单记录</view></view><view @click="protectedGo('/pages/orders/index?role=provider&status=working')"><text>{{ stat('provider_working_count') }}</text><view>进行中接单</view></view><view @click="protectedGo('/pages/orders/index?role=all&status=completed')"><text>{{ stat('completed_order_count') }}</text><view>完成订单</view></view></view></view>
    <view class="profile-content">
      <view class="profile-panel orders-panel"><view class="panel-heading"><text>我的订单</text><button @click="protectedGo('/pages/orders/index?role=buyer')">全部 <Icon name="arrow" :size="15"/></button></view><view class="order-shortcuts"><view v-for="item in orderLinks" :key="item.status" @click="protectedGo('/pages/orders/index?role=buyer&status='+item.status)"><Icon :name="item.icon" :size="26"/><text>{{ item.label }}</text></view></view></view>
      <view class="profile-panel space-panel"><text class="panel-title">我的空间</text><view class="space-grid"><button @click="openIdentity"><Icon name="user" :size="30"/><view><text>个人主页</text><view>查看服务与名片</view></view></button><button @click="protectedGo('/pages/clubs/index?mine=1')"><Icon name="club" :size="30"/><view><text>我的俱乐部</text><view>找到你的集结地</view></view></button></view></view>
      <view class="provider-switch profile-panel"><Icon name="send" :size="29"/><view class="switch-copy"><text>接单身份</text><view>{{ !state.user ? '登录后开启服务发布' : state.user.provider_enabled ? '已开启，自由接单与发布服务' : '无需挂靠，开启后自由接单' }}</view></view><switch :key="switchVersion" :checked="!!state.user?.provider_enabled" :color="game.primary_color" :disabled="saving" @change="toggleProvider"/></view>
      <view v-if="error" class="error-box"><text>{{ error }}</text><button class="text-btn" @click="load">重新加载</button></view>
      <view class="menu-list profile-panel"><view class="menu-item" @click="protectedGo('/pages/profile/index')"><Icon name="user" :size="23"/><text class="menu-label">个人资料</text><Icon name="arrow" :size="16"/></view><view class="menu-item" @click="verifyPhone"><Icon name="phone" :size="23"/><text class="menu-label">手机号验证</text><text v-if="state.user?.phone_verified" class="verified-phone">{{ state.user.phone_masked || '已验证' }}</text><Icon name="arrow" :size="16"/></view><view class="menu-item" @click="go('/pages/agreements/index?type=user_agreement')"><Icon name="orders" :size="23"/><text class="menu-label">用户协议</text><Icon name="arrow" :size="16"/></view><view class="menu-item" @click="go('/pages/agreements/index?type=privacy_policy')"><Icon name="shield" :size="23"/><text class="menu-label">隐私政策</text><Icon name="arrow" :size="16"/></view></view>
      <view v-if="state.user?.role==='admin'" class="menu-list profile-panel"><view class="menu-item admin-entry" @click="go('/pages/admin/index')"><Icon name="settings" :size="23"/><text class="menu-label">平台管理中心</text><Icon name="arrow" :size="16"/></view></view>
      <view class="profile-motto">热爱不设门槛，实力值得被看见</view>
    </view>
  </AppShell>
</template>
<script setup>
import { ref, onMounted } from 'vue'
import { onShow, onHide } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state, game, init, clearSession, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { requirePhone } from '../../lib/phone'
import { go, notify, assetUrl } from '../../lib/format'
const stats=ref({}),saving=ref(false),error=ref(''),switchVersion=ref(0),nativeHeader=ref({})
const orderLinks=[{status:'pending_payment',label:'待付款',icon:'wallet'},{status:'working',label:'进行中',icon:'clock'},{status:'delivered',label:'待验收',icon:'checkbox'},{status:'refund',label:'售后',icon:'headphones'}]
let loadVersion=0,visible=true
/** 记录请求发起时的账号与会话。 */
function currentSession(){return{token:token(),userId:state.user?.id,revision:sessionRevision()}}
/** 检查账号切换和令牌变化，禁止提交旧账号响应。 */
function sameSession(session){return session.token===token()&&session.userId===state.user?.id&&session.revision===sessionRevision()}
/** 清空不再属于当前版本的统计。 */
function invalidateLoad(){stats.value={};return ++loadVersion}
/** 仅允许当前可见页面及原会话更新数据。 */
function currentRequest(version,session){return visible&&version===loadVersion&&sameSession(session)}
/** 访客显示空数据，登录后的未完成请求使用等待占位符。 */
function stat(key){return state.user?(stats.value[key]??'—'):0}
/** 需要身份的入口保留原目标，访客可以继续查看整个个人中心。 */
function protectedGo(target){if(!state.user){go('/pages/login/index?next='+encodeURIComponent(target));return false}go(target);return true}
/** 已登录用户打开公开名片，访客从头像和昵称区域进入登录。 */
function openIdentity(){protectedGo(state.user?'/pages/player/index?id='+encodeURIComponent(state.user.id):'/pages/me/index')}
/** 显式打开手机号验证，已有验证也展示当前安全状态。 */
async function verifyPhone(){if(!state.user)return protectedGo('/pages/me/index');if(state.user.phone_verified)return notify('已验证 '+(state.user.phone_masked||'手机号'));await requirePhone()}
/** 刷新真实资料与统计，旧会话和隐藏页面的响应全部丢弃。 */
async function load(){const version=invalidateLoad();let session;error.value='';try{await init();if(!visible||version!==loadVersion||!state.user)return;session=currentSession();const user=await api('/me');if(!currentRequest(version,session))return;state.user=user;const summary=await api('/me/stats');if(currentRequest(version,session))stats.value=summary||{}}catch(err){if(!visible||version!==loadVersion||(session&&!sameSession(session)))return;if(err.status===401){clearSession();return}error.value=err.message}}
/** 访客先登录；已登录用户切换接单身份，失败时重建原开关。 */
async function toggleProvider(event){
  if(saving.value||!state.user){switchVersion.value++;if(!state.user)protectedGo('/pages/me/index');return}
  const enabled=event.detail.value
  if(enabled&&!state.user.bio?.trim()){switchVersion.value++;notify('请先完善个人介绍');go('/pages/profile/index');return}
  const version=invalidateLoad(),session=currentSession();saving.value=true
  try{const user=await api('/me','PATCH',{provider_enabled:enabled});if(!currentRequest(version,session))return;state.user=user;notify(enabled?'接单身份已开启':'接单身份已关闭')}
  catch(err){if(currentRequest(version,session))notify(err)}finally{saving.value=false;switchVersion.value++;if(visible&&sameSession(session))load()}
}
/** 根据微信原生胶囊保留顶部标题与设置按钮空间。 */
function fitHeader(){
  // #ifdef MP-WEIXIN
  const capsule=uni.getMenuButtonBoundingClientRect(),info=uni.getWindowInfo();if(capsule?.width)nativeHeader.value={paddingTop:capsule.top+'px',paddingRight:info.windowWidth-capsule.left+12+'px'}
  // #endif
}
/** 页面可见时重新查询。 */
function showPage(){visible=true;load()}
/** 页面离开后不允许迟到响应恢复身份或统计。 */
function hidePage(){visible=false;invalidateLoad()}
onMounted(fitHeader)
onShow(showPage)
onHide(hidePage)
</script>
<style scoped>
.me-hero{position:relative;overflow:hidden;padding-bottom:12px}.me-background{position:absolute;inset:0;width:100%;height:100%;opacity:.33}.me-shade{position:absolute;inset:0;background:linear-gradient(0deg,var(--bg),#07100d33 85%)}.me-titlebar{position:relative;display:flex;align-items:center;justify-content:center;min-height:45px;padding:18px 20px 0;box-sizing:content-box;font-size:21px;font-weight:750}.me-titlebar>button{position:absolute;right:19px;bottom:6px;margin:0;padding:0;line-height:29px;background:none;color:#dce5df}.profile-top{position:relative;display:flex;align-items:center;gap:15px;padding:22px 20px 18px}.profile-avatar{width:58px;height:58px;flex-shrink:0;border:1px solid #c4cfca77;background:#d2dbd733;border-radius:50%;overflow:hidden;display:flex;align-items:center;justify-content:center;color:#c8d1cc}.profile-avatar image{width:100%;height:100%}.profile-name{flex:1;min-width:0}.profile-name>view{display:flex;align-items:center;gap:5px;font-size:22px;font-weight:750;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.profile-name>text{display:block;font-size:12px;line-height:19px;color:#aab8b0;margin-top:4px;max-height:38px;overflow:hidden}.stats-row{position:relative;display:flex;margin:9px 0 0;padding:2px 0 4px}.stats-row>view{flex:1;text-align:center;border-right:1px solid #ffffff30}.stats-row>view:last-child{border:0}.stats-row>view>text{font-size:24px;line-height:27px;font-weight:750}.stats-row>view>view{margin-top:4px;color:#aebcb3;font-size:12px}.profile-content{display:flex;flex-direction:column;gap:11px;padding:0 14px}.profile-panel{border:1px solid #ffffff16;border-radius:11px;background:#ffffff02}.panel-heading{display:flex;justify-content:space-between;align-items:center;padding:10px 12px;border-bottom:1px solid #ffffff12}.panel-heading>text,.panel-title{font-size:16px;font-weight:750}.panel-heading>button{display:flex;align-items:center;gap:3px;padding:0;margin:0;background:none;font-size:12px;color:#a3b0a7;line-height:23px}.order-shortcuts{display:flex;justify-content:space-around;padding:12px 0}.order-shortcuts>view{flex:1;display:flex;flex-direction:column;align-items:center;gap:8px;color:#c6d5cd}.order-shortcuts text{font-size:12px;color:#e1e9e4}.space-panel{padding:12px 10px 10px}.panel-title{display:block;margin:0 2px 10px}.space-grid{display:grid;grid-template-columns:1fr 1fr;gap:9px}.space-grid>button{display:flex;align-items:center;gap:9px;margin:0;padding:10px 10px;border:1px solid #ffffff20;border-radius:8px;background:#ffffff03;color:#d0ded5;text-align:left;line-height:20px}.space-grid>button>view{min-width:0}.space-grid>button text{display:flex;align-items:center;gap:4px;font-size:13px;font-weight:700;color:#e9eee9;white-space:nowrap}.space-grid>button>view>view{font-size:10px;color:#9eafa3;white-space:nowrap}.provider-switch{display:flex;align-items:center;gap:13px;min-height:61px;padding:11px 14px;color:#7fc9a8}.switch-copy{flex:1;min-width:0}.switch-copy>text{font-size:15px;font-weight:700;color:#e4ebe5}.switch-copy>view{font-size:11px;line-height:18px;margin-top:3px;color:#a3b3a8}.provider-switch switch{width:45px;height:32px;flex-shrink:0;transform:scale(.76);transform-origin:right center}.menu-list{overflow:hidden}.menu-item{display:flex;align-items:center;gap:16px;height:44px;padding:0 15px;border-bottom:1px solid #ffffff12;color:#cbd9d0}.menu-item:last-child{border-bottom:0}.menu-item .menu-label{flex:1;font-size:14px;font-weight:600;color:#dce6de}.menu-item .verified-phone{flex:0;font-size:10px;font-weight:400;white-space:nowrap;color:#90b6a0}.admin-entry{color:var(--accent)}.profile-motto{font-size:11px;color:#8da093;text-align:center;padding:12px 0 18px}.error-box{font-size:12px;padding:12px}.error-box .text-btn{margin-top:5px}@media(max-width:350px){.profile-content{padding:0 12px}.profile-top{padding-left:16px;padding-right:16px;gap:12px}.profile-name>view{font-size:20px}.space-grid>button{padding:11px 7px;gap:6px}.space-grid>button text{font-size:11px}.space-grid>button>view>view{font-size:9px}.menu-item{gap:12px}}
</style>
