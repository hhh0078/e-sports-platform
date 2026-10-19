<!-- 文件作用：以游戏爱好者自由接单为入口，明确取得协议同意后登录，并提供可跳过的头像昵称完善。 -->
<template>
  <AppShell immersive hide-nav>
    <view class="login-page">
      <view class="login-top" :style="nativeHeader"><view class="login-brand"><text class="brand-mark">集</text><view><text class="brand-name">集结</text><text class="brand-en">JIJIE</text></view></view><button class="skip-login" @click="go('/pages/index/index')">先逛逛 <Icon name="arrow" :size="15"/></button></view>
      <view class="login-copy"><view>把游戏热爱，</view><view>变成<text>你的收入。</text></view><text class="login-subtitle">自由接单 · 找到队友 · 一起成长</text></view>
      <view class="game-montage"><view v-for="(cover,index) in loginCovers" :key="cover" :class="['montage-part','montage-'+index]"><image :src="assetUrl(cover)" mode="aspectFill"/></view><view class="montage-shade"/></view>
      <view class="login-bottom">
        <view class="login-benefits"><view><Icon name="orders" :size="25"/><text>自由接单</text></view><view><Icon name="shield" :size="25"/><text>订单保障</text></view><view><Icon name="club" :size="25"/><text>俱乐部认证</text></view></view>
        <view v-if="error" class="login-error">{{ error }}<button v-if="!agreements" @click="load">重新加载</button></view>
        <button class="login-primary" :disabled="!!submitting||loading" @click="realLogin"><Icon name="weixin" :size="25"/><text>{{ submitting ? '正在登录…' : loading ? '正在准备…' : '登录集结' }}</text></button>
        <view class="consent-line"><checkbox-group @change="changeConsent"><label><checkbox value="accepted" :checked="accepted" color="#7fe4ba" :disabled="!!submitting"/></label></checkbox-group><view>我已阅读并同意 <text @click="openAgreement('user_agreement')">《用户协议》</text> 和 <text @click="openAgreement('privacy_policy')">《隐私政策》</text></view></view>
        <view class="optional-tip">登录后可选择完善头像和昵称</view>
        <button v-if="state.devMode" class="dev-toggle" :disabled="!!submitting" @click="devOpen=true">开发体验</button>
      </view>
    </view>
    <view v-if="devOpen" class="login-mask" @click="devOpen=false"><view class="login-sheet" @click.stop><view class="sheet-heading"><text>选择体验身份</text><button @click="devOpen=false"><Icon name="close" :size="22"/></button></view><text class="sheet-hint">本地测试账号，登录前同样需要主动同意协议。</text><scroll-view scroll-y class="accounts-scroll"><button v-for="account in accounts" :key="account.id" class="account-card" :disabled="!!submitting" @click="selectAccount(account)"><view class="account-avatar">{{ account.nickname?.slice(0,1) }}</view><view><text class="account-name">{{ account.nickname }}</text><text class="account-detail">{{ accountDescriptions[account.id] || '体验玩家与接单能力' }}</text></view><Icon name="arrow" :size="17"/></button></scroll-view></view></view>
    <view v-if="profileOffer" class="login-mask" @click="finishLogin"><view class="login-sheet profile-offer" @click.stop><view class="sheet-heading"><text>让队友认识你</text><button :disabled="profileBusy" @click="finishLogin"><Icon name="close" :size="22"/></button></view><text class="sheet-hint">头像和昵称随时可以修改，也可以稍后完善。</text><button class="offer-avatar" :disabled="profileBusy" @click="pickAvatar"><image v-if="draft.avatar" :src="assetUrl(draft.avatar)" mode="aspectFill"/><Icon v-else name="user" :size="32"/></button><input v-model="draft.nickname" type="nickname" class="input" maxlength="24" placeholder="2—24 个字的昵称"/><button class="login-primary" :disabled="profileBusy" @click="saveOptionalProfile">{{ profileBusy ? '正在保存…' : '保存并进入' }}</button><button class="finish-later" :disabled="profileBusy" @click="finishLogin">先这样，稍后完善</button></view></view>
  </AppShell>
</template>
<script setup>
import { ref, reactive, onMounted } from 'vue'
import { onLoad, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state, init, login, prepareWechatLogin, wechatLogin, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { chooseUploadImage } from '../../lib/media'
import { go, notify, assetUrl } from '../../lib/format'
const accounts=ref([]),agreements=ref(null),loading=ref(true),error=ref(''),submitting=ref(''),accepted=ref(false),devOpen=ref(false),profileOffer=ref(false),profileBusy=ref(false),nativeHeader=ref({})
const draft=reactive({nickname:'',avatar:''}),loginCovers=['/static/covers/official/delta-3.jpg','/static/covers/official/honor-1.jpg','/static/covers/official/peace-1.jpg']
const accountDescriptions={player:'玩家 · 下单与验收',worker:'打手 · 发布服务与交付',clubowner:'俱乐部管理员',admin:'平台管理与纠纷处理',outsider:'独立玩家 · 自由接单'}
let next='/pages/me/index',operation=0,disposed=false
/** 加载当前协议版本，任何重新加载都要求再次主动勾选。 */
async function load(){loading.value=true;error.value='';accepted.value=false;try{await init();agreements.value=await api('/agreements');if(state.devMode)accounts.value=await api('/dev/accounts')||[]}catch(err){error.value=err.message}finally{loading.value=false}}
/** 只接受复选框的明确勾选事件，不因点击登录按钮默认同意。 */
function changeConsent(event){accepted.value=(event.detail.value||[]).includes('accepted')}
/** 打开公开协议全文，阅读后返回仍需用户自行选择。 */
function openAgreement(type){if(!submitting.value)go('/pages/agreements/index?type='+type)}
/** 创建已明确同意的协议版本记录。 */
function consent(){if(!accepted.value||!agreements.value){notify('请先阅读并勾选用户协议和隐私政策');return null}return{agreed:true,user_agreement_version:agreements.value.user_agreement.version,privacy_policy_version:agreements.value.privacy_policy.version}}
/** 处理协议过期，将新版内容交给用户重新阅读并勾选。 */
async function loginFailed(err){if(err.status===409){await load();error.value='协议已更新，请阅读后重新勾选。'}else{const detail=err.message||err.errMsg||'';error.value=/INVALID_LOGIN.*access_token expired/.test(detail)?'微信开发者工具登录态已过期，请重新登录开发者工具后重试':detail||'暂时无法登录，请重试';notify(error.value)}}
/** 登录成功后展示可关闭的头像昵称弹层，不强制提供额外资料。 */
function offerProfile(user){if(!user)return;devOpen.value=false;draft.nickname=user.nickname||'';draft.avatar=user.avatar||'';profileOffer.value=true}
/** 本地账号也提交用户主动同意的协议版本，保持正式流程一致。 */
async function selectAccount(account){if(submitting.value)return;const agreed=consent();if(!agreed){devOpen.value=false;return}const current=++operation;submitting.value=account.id;error.value='';try{const user=await login(account.id,agreed);if(current===operation)offerProfile(user)}catch(err){if(current===operation)await loginFailed(err)}finally{if(current===operation)submitting.value=''}}
/** 先用微信 code 换取短期票据，核对协议后才显式提交登录。 */
async function realLogin(){
  if(submitting.value)return
  const agreed=consent();if(!agreed)return
  // #ifdef H5
  if(state.devMode){devOpen.value=true;return}
  notify('请在微信小程序内登录');return
  // #endif
  // #ifdef MP-WEIXIN
  const current=++operation,revision=sessionRevision();submitting.value='wechat';error.value=''
  try{
    const result=await new Promise((resolve,reject)=>uni.login({provider:'weixin',success:resolve,fail:reject}))
    const prepared=await prepareWechatLogin(result.code)
    if(current!==operation||revision!==sessionRevision())return
    if(prepared.agreements.user_agreement.version!==agreed.user_agreement_version||prepared.agreements.privacy_policy.version!==agreed.privacy_policy_version){agreements.value=prepared.agreements;accepted.value=false;error.value='协议已更新，请阅读后重新勾选。';return}
    const user=await wechatLogin(prepared.ticket,agreed)
    if(current===operation)offerProfile(user)
  }catch(err){if(current===operation)await loginFailed(err)}finally{if(current===operation)submitting.value=''}
  // #endif
}
/** 通过统一裁剪压缩上传流程选择头像，取消时保持现有草稿。 */
async function pickAvatar(){if(profileBusy.value)return;const revision=sessionRevision(),savedToken=token();profileBusy.value=true;try{const url=await chooseUploadImage('avatar');if(!disposed&&revision===sessionRevision()&&savedToken===token()&&url)draft.avatar=url}catch(err){if(!disposed&&revision===sessionRevision()&&!err.cancelled)notify(err)}finally{profileBusy.value=false}}
/** 保存可选资料前检查长度，并丢弃切换会话后的返回结果。 */
async function saveOptionalProfile(){if(profileBusy.value||!state.user)return;const nickname=draft.nickname.trim();if(Array.from(nickname).length<2||Array.from(nickname).length>24)return notify('昵称需要 2—24 个字');const revision=sessionRevision(),savedToken=token();profileBusy.value=true;try{const user=await api('/me/profile','PATCH',{nickname,avatar:draft.avatar});if(!disposed&&revision===sessionRevision()&&savedToken===token()){state.user=user;profileOffer.value=false;go(next)}}catch(err){if(revision===sessionRevision())notify(err)}finally{profileBusy.value=false}}
/** 关闭可选资料弹层并返回原本想访问的页面。 */
function finishLogin(){if(profileBusy.value)return;profileOffer.value=false;go(next)}
/** 为微信胶囊和状态栏留出真实空间。 */
function fitHeader(){
  // #ifdef MP-WEIXIN
  const capsule=uni.getMenuButtonBoundingClientRect(),window=uni.getWindowInfo();if(capsule?.width)nativeHeader.value={paddingTop:capsule.top+'px',paddingRight:window.windowWidth-capsule.left+12+'px'}
  // #endif
}
/** 只允许返回站内页面，避免登录后的跳转离开应用。 */
function applyRoute(options){let target='';try{target=options.next?decodeURIComponent(options.next):''}catch{}next=target.startsWith('/pages/')&&!target.startsWith('/pages/login/')?target:'/pages/me/index';load()}
onLoad(applyRoute)
onMounted(fitHeader)
onHide(()=>{operation++;submitting.value=''})
onUnload(()=>{disposed=true;operation++})
</script>
<style scoped>
.login-page{min-height:100vh;background:#07100e;color:#f3f6f4;padding-bottom:27px;overflow:hidden}.login-top{display:flex;justify-content:space-between;align-items:center;padding:30px 22px 0;min-height:64px;box-sizing:content-box}.login-brand{display:flex;align-items:center;gap:8px}.brand-mark{font-size:25px;font-weight:900;color:#03160d;background:#7fe4ba;line-height:36px;width:36px;text-align:center;border-radius:10px}.brand-name{display:block;font-size:23px;font-weight:800;line-height:27px}.brand-en{display:block;font-size:9px;font-weight:700;letter-spacing:4px;margin-top:2px}.skip-login{display:flex;align-items:center;gap:3px;padding:0;margin:0;font-size:13px;line-height:28px;color:#c3ccc7;background:none;white-space:nowrap}.login-copy{position:relative;z-index:2;padding:34px 22px 0;font-size:38px;font-weight:850;line-height:1.36;letter-spacing:-1px}.login-copy>view{white-space:nowrap}.login-copy>view>text{color:#7fe4ba}.login-subtitle{display:block;font-size:15px;font-weight:400;letter-spacing:1.3px;line-height:25px;color:#aebcb4;margin-top:14px}.game-montage{position:relative;display:flex;height:275px;margin-top:3px;margin-left:-26px;margin-right:-26px;overflow:hidden}.montage-part{flex:1;overflow:hidden;transform:skew(-12deg);border-right:6px solid #07100e;margin-right:0}.montage-part image{width:155%;height:100%;margin-left:-25%;transform:skew(12deg)}.montage-0 image{object-position:70%}.montage-shade{position:absolute;inset:0;background:linear-gradient(180deg,#07100e,transparent 23%,transparent 68%,#07100e)}.login-bottom{position:relative;margin-top:-7px;padding:0 22px}.login-benefits{display:flex;justify-content:space-between;margin-bottom:31px;color:#7fe4ba}.login-benefits>view{flex:1;display:flex;align-items:center;gap:7px;flex-direction:column;font-size:13px;position:relative}.login-benefits>view>text{color:#c0cac4}.login-benefits>view+view:before{content:'';position:absolute;width:1px;background:#53645b;height:22px;left:0;top:12px}.login-primary{display:flex;align-items:center;justify-content:center;gap:12px;width:100%;margin:0;padding:11px 10px;min-height:52px;line-height:28px;font-size:20px;font-weight:800;background:#7fe4ba;color:#03180f;border-radius:8px}.login-primary[disabled]{opacity:.55}.consent-line{display:flex;align-items:flex-start;justify-content:center;gap:4px;margin-top:16px;font-size:11px;line-height:21px;color:#adbab1}.consent-line checkbox{transform:scale(.72);transform-origin:center top;margin-right:-5px}.consent-line>view{padding-top:1px}.consent-line text{color:#7fe4ba}.optional-tip{font-size:11px;color:#8f9c94;text-align:center;margin-top:23px}.dev-toggle{margin:24px auto 0;padding:0 20px;font-size:11px;line-height:27px;background:none;border:1px solid #87988c80;border-radius:20px;color:#bbc7be}.login-error{display:flex;align-items:center;justify-content:space-between;gap:10px;font-size:12px;line-height:18px;color:#e7b19d;margin-bottom:14px}.login-error button{font-size:11px;color:#7fe4ba;line-height:24px;background:none;padding:0;margin:0}.login-mask{position:fixed;inset:0;z-index:110;background:#000b;display:flex;align-items:flex-end}.login-sheet{width:100%;max-width:600px;margin:0 auto;background:#101c16;border:1px solid #ffffff20;border-radius:20px 20px 0 0;padding:24px 20px calc(25px + env(safe-area-inset-bottom));box-sizing:border-box}.sheet-heading{display:flex;align-items:center;justify-content:space-between;color:#f0f5f2;font-size:22px;font-weight:750}.sheet-heading button{padding:0;background:none;color:#b7c6bd;margin:0;line-height:30px}.sheet-hint{display:block;color:#96a89c;font-size:12px;line-height:20px;margin:8px 0 18px}.accounts-scroll{height:48vh;max-height:430px}.account-card{width:100%;margin:0 0 10px;display:flex;align-items:center;gap:12px;color:#e5eee8;border:1px solid #ffffff16;background:#ffffff03;border-radius:10px;padding:13px;text-align:left;line-height:20px}.account-card>view:nth-child(2){flex:1}.account-avatar{height:39px;width:39px;border-radius:10px;display:flex;align-items:center;justify-content:center;color:#7fe4ba;background:#ffffff08;font-size:21px;font-weight:750}.account-name{display:block;font-size:15px;font-weight:650}.account-detail{display:block;font-size:11px;color:#8b9f91}.offer-avatar{width:72px;height:72px;border:1px solid #ffffff24;border-radius:18px;background:#ffffff08;color:#7fe4ba;padding:0;display:flex;align-items:center;justify-content:center;margin:20px auto}.offer-avatar image{width:100%;height:100%}.profile-offer .input{margin:16px 0}.profile-offer .login-primary{font-size:16px;margin-top:18px}.finish-later{margin:12px auto 0;background:none;color:#a5b5aa;font-size:13px;line-height:32px}@media(max-width:350px){.login-copy{font-size:32px;padding-left:18px;padding-right:18px}.login-subtitle{font-size:12px}.game-montage{height:235px}.login-bottom{padding-left:18px;padding-right:18px}.consent-line{font-size:10px}.login-top{padding-left:18px;padding-right:18px}.login-primary{font-size:18px}}
</style>
<style scoped>
.dev-toggle{width:max-content;min-width:92px}
</style>
