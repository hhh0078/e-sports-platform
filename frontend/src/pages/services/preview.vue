<!-- 文件作用：独立展示待发布服务，支持返回修改、手机号校验、确认发布及精确草稿清理。 -->
<template>
  <AppShell title="预览服务" :game-id="preview?.fields.game_id" back>
    <template v-if="preview">
      <text v-if="error" class="error-box">{{error}}</text>
      <view class="preview-heading"><text class="section-title">服务卡片预览</text></view>
      <ServiceCard :service="preview.service" :interactive="false"/>
      <view class="form-section"><text class="section-title">服务说明</text><text class="preview-description">{{preview.fields.description}}</text></view>
      <view class="form-section publish-notice"><text class="section-title">发布须知</text><text class="subtle-note">发布后进行图文审核；无AI时由管理员人工处理。审核未通过会通知修改，已有订单的约定保持不变。</text></view>
      <view class="preview-footer-space"/>
      <view class="preview-footer"><button class="text-btn preview-back" :disabled="busy" @click="back">返回修改</button><button class="btn preview-confirm" :disabled="busy||published" @click="save">{{busy?'正在提交…':published?'已提交':preview.serviceID?'确认保存':'确认发布'}}</button></view>
    </template>
    <view v-else class="expired-preview"><text class="subtle-note">预览已过期，请返回编辑后重新预览。</text><button class="btn" @click="returnToEditor">返回编辑</button></view>
    <SubscribeReminder ref="subscriptionReminder" :scenes="['provider_order']" :entry="false" title="发布成功" description="有人购买你的服务时，及时收到微信提醒。拒绝订阅不影响接单。" button-label="接收新订单提醒"/>
  </AppShell>
</template>
<script setup lang="ts">
import {ref,shallowRef,triggerRef,watch} from 'vue'
import {onLoad,onShow,onHide,onUnload} from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import ServiceCard from '../../components/ServiceCard.vue'
import SubscribeReminder from '../../components/SubscribeReminder.vue'
import {getServicePreview,clearServicePreview,serviceFormIssue} from '../../lib/service-preview'
import type {ServicePreview} from '../../lib/service-preview'
import {publishDraftOwner} from '../../lib/publish-draft'
import {state,sessionRevision} from '../../lib/store'
import {requirePhone} from '../../lib/phone'
import {back} from '../../lib/navigation'
import {go,notify} from '../../lib/format'
import {token,errorMessage} from '../../api/client'
import {createService,updateService} from '../../api/services.api'
import {getWallet} from '../../api/wallet.api'
import {getClubDeposits} from '../../api/finance.api'
const preview=shallowRef<ServicePreview|null>(null),busy=ref(false),published=ref(false),error=ref('')
const subscriptionReminder=ref<{offer:(next?:()=>void)=>void}|null>(null)
let disposed=false,visible=true,refreshVersion=0,editingID=''
/** 当前账号与编辑上下文必须仍对应此快照，失败或迟到响应不能跨账号写入。 */
function current(value:ServicePreview):boolean{return !disposed&&preview.value===value&&getServicePreview(publishDraftOwner(state.user?.id),sessionRevision())===value}
/** 从编辑页接收已校验快照，缺失时只显示返回入口。 */
function load():void{const value=getServicePreview(publishDraftOwner(state.user?.id),sessionRevision());preview.value=value?.serviceID===editingID?value:null}
/** 预览显示时更新真实押金，不把服务端字段写入草稿；旧账号响应无效。 */
async function refreshDeposits():Promise<void>{
  const value=preview.value,version=++refreshVersion
  if(!value||!state.user||!current(value))return
  const results=await Promise.allSettled([getWallet(),value.fields.club_id?getClubDeposits(value.fields.club_id):Promise.resolve(null)])
  if(!visible||version!==refreshVersion||!current(value))return
  const wallet=results[0].status==='fulfilled'?results[0].value:null,club=results[1].status==='fulfilled'?results[1].value:null
  const platform=wallet?.deposit_qualified===true?wallet.deposit_cents:0,member=club?.membership.amount_cents||0
  value.service={...value.service,provider_deposit_cents:Number.isSafeInteger(platform)&&platform>0?platform:0,provider_club_deposit_cents:Number.isSafeInteger(member)&&member>0?member:0}
  triggerRef(preview)
}
/** 无历史时回到编辑入口，让已有账号隔离草稿提供恢复选择。 */
function returnToEditor():void{if(getCurrentPages().length>1)back();else go('/pages/services/publish'+(editingID?'?id='+encodeURIComponent(editingID):''))}
/** 提交锁覆盖手机号授权、请求和订阅；失败保留草稿，成功只清精确提交版本。 */
async function save():Promise<void>{
  const value=preview.value
  if(busy.value||published.value||!value||!current(value))return
  if(!state.user)return go('/pages/account/login?next='+encodeURIComponent('/pages/services/publish'+(value.serviceID?'?id='+encodeURIComponent(value.serviceID):'')))
  const requestToken=token();busy.value=true;error.value=''
  try{
    if(!(await requirePhone())||!current(value)||requestToken!==token())return
    const form=value.fields,issue=serviceFormIssue(form)
    if(issue)return notify(issue)
    const body={game_id:form.game_id,title:form.title.trim(),description:form.description.trim(),category:form.category,price_cents:Math.round(Number(form.price)*100),unit:form.unit,cover_url:form.cover_url,labels:form.labels,tags:form.tagsText.split(/[,，]/).map(item=>item.trim()).filter(Boolean).slice(0,4),club_id:form.club_id}
    await(value.serviceID?updateService(value.serviceID,body):createService(body))
    value.complete()
    if(!current(value)||requestToken!==token())return
    published.value=true;clearServicePreview(value)
    notify('服务已发布，图文审核中')
    /** 订阅提示结束时仍核对原账号，迟到关闭不能导航新的会话。 */
    function finishPublish():void{if(!disposed&&requestToken===token()&&value?.owner===publishDraftOwner(state.user?.id)&&value?.revision===sessionRevision())go('/pages/main/index?tab=worker')}
    if(subscriptionReminder.value)subscriptionReminder.value.offer(finishPublish);else finishPublish()
  }catch(reason){if(current(value)){error.value=errorMessage(reason);notify(reason)}}finally{busy.value=false}
}
watch([sessionRevision,()=>state.user?.id],()=>{refreshVersion++;preview.value=null;error.value=''},{flush:'sync'})
onLoad(options=>{editingID=options?.id||'';load()})
onShow(()=>{visible=true;if(preview.value&&!current(preview.value))preview.value=null;void refreshDeposits()})
onHide(()=>{visible=false;refreshVersion++})
onUnload(()=>{disposed=true;visible=false;refreshVersion++})
</script>
<style scoped>
.preview-heading{padding:0 0 30.7692rpx}.preview-description{display:block;font-size:26.9231rpx;line-height:1.8;color:var(--text);white-space:pre-wrap;overflow-wrap:anywhere}.publish-notice .subtle-note{padding:0}.preview-footer-space{height:153.8462rpx}.preview-footer{position:fixed;left:0;right:0;bottom:0;display:flex;align-items:center;gap:30.7692rpx;width:100%;max-width:600px;margin:0 auto;padding:23.0769rpx 30.7692rpx calc(23.0769rpx + env(safe-area-inset-bottom));border-top:1px solid var(--line-subtle);background:var(--bg);z-index:5}.preview-back{min-height:44px;color:var(--muted)}.preview-confirm{flex:1;min-width:0}.expired-preview{padding:61.5385rpx 0}
</style>
