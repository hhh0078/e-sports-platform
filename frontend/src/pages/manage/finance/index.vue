<!-- 文件作用：按先行设计分页查看个人及俱乐部提现，每笔申请进入独立资金来源审核页。 -->
<template>
  <AppShell title="资金审核" back>
    <view v-if="error" class="error-box" @click="load()">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="提现来源及审核结果仅向平台管理员开放。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view class="audit-intro"><text class="audit-title">资金来源审核</text><text class="audit-note">逐笔核验付款、订单和提现占用</text></view>
      <scroll-view scroll-x :show-scrollbar="false" class="audit-filter-scroll"><view class="audit-filters"><button v-for="filter in filters" :key="filter.value" :class="['audit-filter',{selected:status===filter.value}]" @click="chooseStatus(filter.value)">{{ filter.label }}</button></view></scroll-view>
      <view v-if="loading&&!withdrawals.length" class="loading-line">正在读取提现申请…</view>
      <EmptyState v-else-if="!withdrawals.length&&!error" title="暂无提现申请" :detail="status==='all'?'个人和俱乐部的提现申请会显示在这里。':'当前状态下暂无提现申请。'" icon="wallet"/>
      <button v-for="item in withdrawals" :key="item.id" class="audit-card" @click="openDetail(item.id)"><view class="audit-card-copy"><text class="audit-account">{{ item.account_type==='club'||item.club_id?'俱乐部佣金提现':'个人提现' }}</text><text class="audit-amount">¥{{ fixed(item.amount_cents) }}</text><text class="audit-meta">{{ walletStatus(item.status) }} · 查看逐笔来源</text><text class="audit-reference">{{ item.club_id?'俱乐部 '+item.club_id:'用户 '+item.user_id }}</text><text class="audit-time">{{ time(item.created_at) }}</text></view><Icon name="arrow" :size="18"/></button>
      <button v-if="hasMore" class="text-btn audit-more" :disabled="loading" @click="load(true)">{{ loading?'正在读取…':'加载更多' }}</button>
      <text class="audit-footnote">来源不足、已退款或重复占用将阻止转账。</text>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import { state,init,sessionRevision } from '../../../lib/store'
import { token,errorMessage } from '../../../api/client'
import { listAdminWithdrawals } from '../../../api/wallet.api'
import { go,time } from '../../../lib/format'
import { walletStatus } from '../../../lib/wallet'
import type { WalletWithdrawal } from '../../../types/wallet'

/** 筛选直接作用于服务端分页，不能只筛选已经读取的记录。 */
type AuditStatus=WalletWithdrawal['status']|'all'
const filters:Array<{value:AuditStatus;label:string}>=[{value:'pending_review',label:'待审核'},{value:'processing',label:'处理中'},{value:'wait_user_confirm',label:'待收款'},{value:'success',label:'已完成'},{value:'failed',label:'未成功'},{value:'rejected',label:'已拒绝'},{value:'all',label:'全部'}]
const status=ref<AuditStatus>('pending_review'),withdrawals=ref<WalletWithdrawal[]>([]),loading=ref(false),error=ref(''),hasMore=ref(false)
let visible=false,version=0,offset=0
/** 捕获管理员身份、令牌及认证版本，禁止跨账号回填资金数据。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 仅在页面可见且仍为同一管理员时接收响应。 */
function sameSession(session:ReturnType<typeof currentSession>){return visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 按整数分展示两位小数，避免提现金额省略分位。 */
function fixed(cents:number):string{return(cents/100).toFixed(2)}
/** 初始化身份后读取真实分页，刷新失败保留已加载的同账号数据。 */
async function load(append=false):Promise<void>{
  if(!visible||(append&&loading.value))return
  const current=++version,selected=status.value,start=append?offset:0
  let session:ReturnType<typeof currentSession>|null=null
  loading.value=true;error.value=''
  try{await init();if(!visible||current!==version)return;if(state.user?.role!=='admin')return;session=currentSession();const result=await listAdminWithdrawals({status:selected,limit:20,offset:start});if(current!==version||!sameSession(session)||status.value!==selected)return;const known=new Set(append?withdrawals.value.map(item=>item.id):[]);withdrawals.value=append?[...withdrawals.value,...result.filter(item=>!known.has(item.id))]:result;offset=start+result.length;hasMore.value=result.length===20}
  catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}
  finally{if(current===version)loading.value=false}
}
/** 切换筛选作废旧请求并从第一页开始，不混合不同状态的结果。 */
function chooseStatus(value:AuditStatus):void{if(value===status.value)return;status.value=value;version++;withdrawals.value=[];offset=0;hasMore.value=false;void load()}
/** 仅当前管理员可进入指定原申请的来源审核。 */
function openDetail(id:string):void{if(visible&&state.user?.role==='admin')go('/pages/manage/finance/detail?id='+encodeURIComponent(id))}
/** 账号变化立即清空上一管理员的列表，页面可见时重新验证身份。 */
function clear():void{version++;withdrawals.value=[];offset=0;hasMore.value=false;loading.value=false;error.value=''}
/** 返回列表时刷新当前筛选，及时展示详情页的真实处理结果。 */
function showPage():void{visible=true;void load()}
/** 隐藏作废未完成读取，旧响应不能覆盖返回后的列表。 */
function hidePage():void{visible=false;version++;loading.value=false}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{hidePage();clear()})
</script>
<style scoped>
.audit-intro{display:flex;flex-direction:column;gap:12px;padding:8px 4px 24px}.audit-title{font-size:23px;line-height:32px;font-weight:650}.audit-note,.audit-meta,.audit-reference,.audit-time,.audit-footnote{font-size:14px;line-height:22px;color:var(--muted)}.audit-filter-scroll{width:100%;max-width:100%;margin-bottom:20px}.audit-filters{display:inline-flex;min-width:100%;gap:20px;white-space:nowrap}.audit-filter{display:flex;align-items:center;justify-content:center;flex-shrink:0;min-height:40px;padding:0 2px;border:0;border-bottom:2px solid transparent;border-radius:0;background:transparent;color:var(--muted);font-size:14px}.audit-filter.selected{border-bottom-color:var(--accent);color:var(--accent)}.audit-card{display:flex;align-items:center;justify-content:space-between;gap:16px;width:100%;min-width:0;margin-bottom:20px;padding:16px 14px;border-radius:12px;background:var(--surface);color:var(--text);text-align:left;line-height:normal}.audit-card-copy{display:flex;flex:1;flex-direction:column;gap:8px;min-width:0}.audit-account{font-size:16px;line-height:24px}.audit-amount{font-size:28px;line-height:36px;font-weight:550;color:var(--accent);overflow-wrap:anywhere}.audit-reference{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.audit-time{font-size:12px;line-height:18px}.audit-more{width:100%;margin:12px 0}.audit-footnote{display:block;margin:32px 4px 16px;overflow-wrap:anywhere}
</style>
