<!-- 文件作用：老板发布自定义游戏服务需求，确认预算后进入订单支付与公开接单流程。 -->
<template><AppShell title="发布需求" subtitle="说清目标，让合适的人来找你" back><view class="request-banner"><Icon name="game" :size="34"/><text>你定目标，队友来响应。</text></view><view v-if="error" class="error-box mb">{{ error }}</view><view class="panel"><view class="field"><text class="field-label">选择游戏</text><picker :range="state.games" range-key="name" :value="gameIndex" @change="gameId=state.games[Number($event.detail.value)].id"><view class="input">{{ selectedGame?.name || '请选择' }}</view></picker></view><view class="field"><text class="field-label">需求标题</text><input v-model="title" class="input" maxlength="40" placeholder="例如：今晚找位耐心的三角洲陪练"/></view><view class="field"><text class="field-label">目标与验收标准</text><textarea v-model="requirements" class="input" maxlength="1000" placeholder="说明区服、时段、任务目标、时长与怎样算完成，请不要填写密码。"/></view><view class="field"><text class="field-label">总预算（元）</text><input v-model="price" class="input" type="digit" placeholder="例如 39"/></view><text class="hint">支付后进入接单大厅。同一订单只有一名打手可接单，未接单可取消。全程在订单聊天室沟通。</text></view><view class="notice mt mb">陪练、组队、任务协作均须遵守游戏官方规则。请使用本人账号，不接受外挂或账号共享要求。</view><button class="btn" :disabled="busy" @click="submit">{{ busy ? '正在创建…' : '确认需求，去支付' }}</button></AppShell></template>
<script setup>
import { requirePhone } from '../../lib/phone'
/** 文件作用：校验自定义需求与整数分预算并创建待支付订单。 */
import { ref, computed } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { state,init } from '../../lib/store'
import { api } from '../../lib/api'
import { go,notify } from '../../lib/format'
const gameId=ref('delta'),title=ref(''),requirements=ref(''),price=ref(''),busy=ref(false),error=ref('')
const selectedGame=computed(()=>state.games.find(item=>item.id===gameId.value)),gameIndex=computed(()=>Math.max(0,state.games.findIndex(item=>item.id===gameId.value)))
/** 创建需求订单；没有会话时先进入登录页面。 */
async function submit(){if(!state.user)return go('/pages/login/index');if(busy.value)return;if(!(await requirePhone()))return;const cents=Math.round(Number(price.value)*100);if(!title.value.trim()||!requirements.value.trim())return notify('请写明需求标题和验收标准');if(!/^\d+(\.\d{1,2})?$/.test(price.value)||cents<100||cents>1000000)return notify('预算应为1至10000元，最多两位小数');busy.value=true;try{const order=await api('/requests','POST',{game_id:gameId.value,title:title.value.trim(),requirements:requirements.value.trim(),amount_cents:cents});go('/pages/order/index?id='+order.id)}catch(e){error.value=e.message;notify(e)}finally{busy.value=false}}
onLoad(async()=>{try{await init();gameId.value=state.gameId}catch(e){error.value=e.message}})
</script>
<style scoped>.request-banner{border-left:3px solid var(--accent);padding:15px 18px;margin:8px 0 24px;color:var(--accent);display:flex;gap:14px;align-items:center;background:#ffffff03;font-size:17px;font-weight:650}.input{font-size:13px}</style>
