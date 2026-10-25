<!-- 文件作用：按先行设计编辑两档平台抽成及两类平台最低押金，所有转换以整数基点和整数分提交。 -->
<template>
  <AppShell title="交易规则" back>
    <view v-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view>
    <EmptyState v-if="state.ready&&state.user?.role!=='admin'" title="仅平台管理员可访问" detail="交易费率与平台最低押金仅能由平台管理员配置。" icon="shield"/>
    <template v-else-if="state.user?.role==='admin'">
      <view v-if="loading&&!loaded" class="loading-line">正在读取交易规则…</view>
      <template v-if="loaded">
        <view class="form-section finance-rate-section"><text class="finance-section-title">平台抽成</text><view class="field finance-field"><text class="field-label finance-label">自由打手订单</text><view class="finance-input-row"><input v-model="form.freeRate" class="finance-input" type="digit" maxlength="6" :disabled="busy||loading" placeholder="0.00"/><text class="finance-unit">%</text></view></view><view class="field finance-field"><text class="field-label finance-label">俱乐部挂靠订单</text><view class="finance-input-row"><input v-model="form.clubRate" class="finance-input" type="digit" maxlength="6" :disabled="busy||loading" placeholder="0.00"/><text class="finance-unit">%</text></view></view><text class="finance-note">两档费率分别生效。俱乐部自己的抽成由负责人另行设置，单笔订单两项抽成合计不得超过100%。</text></view>
        <view class="form-section finance-deposit-section"><view class="field finance-field"><text class="field-label finance-label">个人平台最低押金 · 自愿缴纳</text><view class="finance-input-row"><input v-model="form.personalDeposit" class="finance-input" type="digit" maxlength="9" :disabled="busy||loading" placeholder="0.00"/><text class="finance-unit">元</text></view></view><view class="field finance-field"><text class="field-label finance-label">俱乐部平台最低押金</text><view class="finance-input-row"><input v-model="form.clubDeposit" class="finance-input" type="digit" maxlength="9" :disabled="busy||loading" placeholder="0.00"/><text class="finance-unit">元</text></view></view></view>
        <view class="finance-rule-notes"><text class="finance-note">抽成配置仅影响新订单，旧订单保留成交时的比例和金额。</text><text class="finance-note">最低额提高后，原押金继续保留；未达标暂停展示保障标识，补齐后恢复。</text><text class="finance-note">个人押金始终自愿。最低金额为0时不要求缴纳，未缴押金不展示标识；最高可设置100000元。</text></view>
        <text v-if="formError" class="finance-form-error">{{ formError }}</text>
        <view class="action-footer finance-settings-actions"><button class="btn finance-settings-save" :disabled="busy||loading||!!error" @click="save">{{ busy?'正在保存…':'保存交易规则' }}</button><button class="text-btn" :disabled="busy||loading" @click="restore">取消修改</button></view>
      </template>
    </template>
    <view v-else-if="!error" class="loading-line">正在读取管理员身份…</view>
  </AppShell>
</template>
<script setup lang="ts">
import { reactive,ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import { state,init,sessionRevision } from '../../../lib/store'
import { token,errorMessage } from '../../../api/client'
import { getFinancialSettings,saveFinancialSettings } from '../../../api/finance.api'
import { notify } from '../../../lib/format'
import { parseRate } from '../../../lib/finance'
import type { FinancialSettings } from '../../../types/wallet'

const form=reactive({freeRate:'',clubRate:'',personalDeposit:'',clubDeposit:''}),loading=ref(false),loaded=ref(false),busy=ref(false),error=ref(''),formError=ref('')
let visible=false,version=0,saved:FinancialSettings|null=null,loadedSession:ReturnType<typeof currentSession>|null=null
/** 记录已授权管理员会话，隐藏和切换身份后的保存结果不得回填。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 判断当前页面是否仍属于同一有效平台管理员。 */
function sameSession(session:ReturnType<typeof currentSession>|null){return !!session&&visible&&state.user?.role==='admin'&&session.id===state.user.id&&session.token===token()&&session.revision===sessionRevision()}
/** 服务器整数基点和整数分以两位小数回填，不把未加载配置当作零费率。 */
function apply(value:FinancialSettings):void{saved={...value};Object.assign(form,{freeRate:(value.free_platform_rate_bps/100).toFixed(2),clubRate:(value.club_platform_rate_bps/100).toFixed(2),personalDeposit:((value.personal_platform_deposit_cents??0)/100).toFixed(2),clubDeposit:(value.club_creation_deposit_cents/100).toFixed(2)});loaded.value=true;loadedSession=currentSession()}
/** 从明确的十进制字符串转换整数百分之一，拒绝浮点误差、负数、指数及超过两位小数。 */
function hundredths(raw:string,max:number,label:string):number{const value=raw.trim();if(!/^\d+(\.\d{1,2})?$/.test(value))throw new Error(label+'最多填写两位小数');const[whole,fraction='']=value.split('.');const result=Number(whole)*100+Number(fraction.padEnd(2,'0'));if(!Number.isSafeInteger(result)||result<0||result>max)throw new Error(label+'须在0至'+(max/100)+'之间');return result}
/** 读取真实配置并保护加载前后的管理员身份，失败保留同账号已读数据。 */
async function load():Promise<void>{if(busy.value||!visible)return;const current=++version;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==version)return;if(state.user?.role!=='admin')return;session=currentSession();const result=await getFinancialSettings();if(current===version&&sameSession(session))apply(result)}catch(err){if(visible&&current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 完整校验后提交两档独立平台费率，绝不将两个互斥订单档位相加。 */
async function save():Promise<void>{if(busy.value||loading.value||!loaded.value||!sameSession(loadedSession)||error.value)return;let body:FinancialSettings;try{body={free_platform_rate_bps:parseRate(form.freeRate),club_platform_rate_bps:parseRate(form.clubRate),personal_platform_deposit_cents:hundredths(form.personalDeposit,10000000,'个人平台最低押金'),club_creation_deposit_cents:hundredths(form.clubDeposit,10000000,'俱乐部平台最低押金')}}catch(err){formError.value=errorMessage(err);return}const current=++version,session=currentSession();busy.value=true;formError.value='';try{const result=await saveFinancialSettings(body);if(current===version&&sameSession(session)){apply(result);notify('规则已保存，押金标识按新最低额判断')}}catch(err){if(current===version&&sameSession(session))formError.value=errorMessage(err)}finally{if(current===version)busy.value=false}}
/** 取消只恢复同一管理员已读取的公开配置。 */
function restore():void{if(!busy.value&&!loading.value&&saved&&sameSession(loadedSession)){apply(saved);formError.value=''}}
/** 账号变化时销毁旧草稿，避免跨管理员提交或展示。 */
function clear():void{version++;Object.assign(form,{freeRate:'',clubRate:'',personalDeposit:'',clubDeposit:''});saved=null;loadedSession=null;loaded.value=false;loading.value=false;busy.value=false;error.value='';formError.value=''}
/** 正常返回保留公开草稿，未确认的读取或保存需重新向服务端核实。 */
function showPage():void{visible=true;if(loaded.value&&sameSession(loadedSession))return;void load()}
/** 离开即作废异步回调，进行中的保存结果下次显示重新读取。 */
function hidePage():void{visible=false;version++;if(loading.value||busy.value)loaded.value=false;loading.value=false;busy.value=false}
watch([()=>state.user?.id,()=>state.user?.role,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{hidePage();clear()})
</script>
<style scoped>
.finance-rate-section{padding:15.3846rpx 7.6923rpx 0;border-bottom:0}.finance-section-title{display:block;margin-bottom:53.8462rpx;font-size:34.6154rpx;line-height:53.8462rpx}.finance-field{margin-bottom:46.1538rpx}.finance-label{font-size:26.9231rpx;line-height:42.3077rpx;color:var(--muted)}.finance-input-row{display:flex;align-items:center;gap:15.3846rpx;min-width:0;min-height:100rpx;border-bottom:1px solid var(--line)}.finance-input{display:block;flex:1;min-width:0;height:92.3077rpx;color:var(--text);font-size:48.0769rpx;line-height:normal}.finance-unit{flex-shrink:0;padding-right:15.3846rpx;font-size:42.3077rpx;color:var(--text)}.finance-note{display:block;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted);overflow-wrap:anywhere}.finance-deposit-section{padding:61.5385rpx 7.6923rpx 0;border:0}.finance-rule-notes{display:flex;flex-direction:column;gap:23.0769rpx;margin:46.1538rpx 7.6923rpx 0}.finance-form-error{display:block;margin:38.4615rpx 7.6923rpx 0;color:#e9b69d;font-size:26.9231rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.finance-settings-actions{flex-direction:column;gap:30.7692rpx;margin-top:61.5385rpx}.finance-settings-save{flex:0 0 auto;width:100%}
</style>
