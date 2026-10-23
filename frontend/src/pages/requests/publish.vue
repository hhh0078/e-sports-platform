<!-- 文件作用：老板发布自定义游戏服务需求，确认预算后进入订单支付与公开接单流程。 -->
<template>
  <AppShell title="发布需求" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <view class="form-section request-fields"><view class="field request-game-field"><text class="field-label">选择游戏</text><GameSelector :value="gameId" :disabled="busy" @change="gameId=$event"/></view><view class="field"><view class="field-heading"><text class="field-label">需求标题</text><text class="field-count">{{title.length}} / 40</text></view><input v-model="title" class="input" maxlength="40" placeholder="例如：今晚找位耐心的三角洲陪练"/></view><view class="field"><view class="field-heading"><text class="field-label">目标与验收标准</text><text class="field-count">{{requirements.length}} / 1000</text></view><textarea v-model="requirements" class="input request-requirements" maxlength="1000" placeholder="说明区服、时段、任务目标、时长，以及怎样算完成，请不要填写密码。"/></view><view class="field"><text class="field-label">总预算（元）</text><input v-model="price" class="input" type="digit" placeholder="1～10000，最多两位小数"/></view></view>
    <view class="request-note"><Icon class="note-icon" name="info" :size="15"/><text class="subtle-note request-note-copy">支付后进入接单大厅，未接单可取消；协作约定与沟通随订单保存。</text></view><view class="request-note rules-note"><Icon class="note-icon" name="shield" :size="15"/><text class="subtle-note request-note-copy">遵守游戏官方规则，请勿填写账号密码或提出外挂要求。</text></view>
    <SubscribeReminder :scenes="['buyer_accepted']" label="需求被接单时微信提醒"/>
    <view class="action-footer"><button class="btn request-submit" :disabled="busy" @click="submit">{{busy?'正在创建…':'确认需求，去支付'}}</button></view>
  </AppShell>
</template>
<script setup lang="ts">
import { requirePhone } from '../../lib/phone'
/** 文件作用：校验自定义需求与整数分预算并创建待支付订单。 */
import { ref } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import GameSelector from '../../components/GameSelector.vue'
import Icon from '../../components/Icon.vue'
import SubscribeReminder from '../../components/SubscribeReminder.vue'
import { state,init,sessionRevision } from '../../lib/store'
import { createRequest } from '../../api/orders.api'
import { token,errorMessage } from '../../api/client'
import { go,notify } from '../../lib/format'
const gameId=ref('delta'),title=ref(''),requirements=ref(''),price=ref(''),busy=ref(false),error=ref('')
let disposed=false
/** 捕获发布操作的用户和会话，授权中切换账号不能继续提交原草稿。 */
function currentSession(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 仅允许当前存活页面的同一账号接收授权和创建结果。 */
function sameSession(session: ReturnType<typeof currentSession>): boolean {return !disposed&&session.id===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 用单次提交锁覆盖手机号授权和创建订单，拒绝连点、迟到会话及页面销毁后的继续发布。 */
async function submit(){
  if(busy.value)return
  if(!state.user)return go('/pages/account/login?next='+encodeURIComponent('/pages/requests/publish'))
  const session=currentSession()
  busy.value=true;error.value=''
  try{
    if(!(await requirePhone())||!sameSession(session))return
    const cents=Math.round(Number(price.value)*100)
    if(!title.value.trim()||!requirements.value.trim())return notify('请写明需求标题和验收标准')
    if(!/^\d+(\.\d{1,2})?$/.test(price.value)||cents<100||cents>1000000)return notify('预算应为1至10000元，最多两位小数')
    const order=await createRequest({game_id:gameId.value,title:title.value.trim(),requirements:requirements.value.trim(),amount_cents:cents})
    if(sameSession(session))go('/pages/orders/detail?id='+order.id)
  }catch(e){if(sameSession(session)){error.value=errorMessage(e);notify(e)}}finally{busy.value=false}
}
onLoad(async()=>{try{await init();gameId.value=state.gameId}catch(e){error.value=errorMessage(e)}})
onUnload(()=>{disposed=true})
</script>
<style scoped>
.request-fields{padding-top:15.3846rpx;padding-bottom:30.7692rpx}.field-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.field-count{font-size:19.2308rpx;color:var(--muted);white-space:nowrap}.request-requirements{height:269.2308rpx}.request-note{display:flex;align-items:flex-start;gap:19.2308rpx;padding:23.0769rpx 0;color:var(--muted)}.request-note-copy{flex:1;min-width:0;padding:0;font-size:21.1538rpx;line-height:38.4615rpx}.note-icon{margin-top:3.8462rpx}.rules-note{border-top:1px solid var(--line-subtle)}.request-submit{width:100%}
</style>
