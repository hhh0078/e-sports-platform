<!-- 文件作用：发现俱乐部、查看个人挂靠并申请创建新的电竞俱乐部。 -->
<template><AppShell title="找到你的集结地" subtitle="自由接单，也能与团队一起成长" back><view class="club-hero"><text class="eyebrow">BETTER TOGETHER</text><text class="hero-line">实力被看见，<text class="accent">合作有归属。</text></text><text class="muted">挂靠认证是加分项，每个人都能自由接单。</text></view><view class="row between mb"><view class="tabs"><button :class="['tab',{active:!mine}]" @click="mine=false;load()">发现俱乐部</button><button :class="['tab',{active:mine}]" @click="mine=true;load()">我的俱乐部</button></view><button class="text-btn" @click="openCreate">创建</button></view><view v-if="error" class="error-box mb" @click="load">{{ error }} · 重试</view><EmptyState v-if="!clubs.length && !loading" title="还没有俱乐部" detail="成为第一位组织者，或到发现页选择团队" icon="club"/><view v-for="club in clubs" :key="club.id" class="club-card panel" @click="go('/pages/club/index?id='+club.id)"><view class="row"><view class="club-monogram">{{ club.name?.slice(0,1) }}</view><view class="club-heading"><view class="row"><text class="club-name">{{ club.name }}</text><Icon v-if="club.verified" name="shield" :size="16" class="accent"/></view><text class="hint">{{ club.member_count || 0 }} 位成员 · {{ club.owner_name }} 主理</text></view><Icon name="arrow" :size="16"/></view><text class="club-description">{{ club.description }}</text><view class="row between"><view class="game-tags"><text v-for="id in club.game_ids" :key="id" class="pill">{{ gameName(id) }}</text></view><text :class="club.status==='active'?'accent':'muted'" class="small">{{ club.status==='active'?(club.verified?'平台认证':'开放加入'):statusLabel(club.status) }}</text></view></view>
<view v-if="creating" class="modal-mask" @click="creating=false"><view class="modal panel" @click.stop><text class="section-title">创建俱乐部</text><text class="muted">提交平台审核后开放成员挂靠。</text><input v-model="form.name" class="input" maxlength="30" placeholder="俱乐部名称"/><textarea v-model="form.description" class="input" maxlength="500" placeholder="介绍团队擅长的游戏、招募要求与服务风格"/><text class="field-label">主要游戏（可多选）</text><view class="game-choices"><button v-for="game in state.games" :key="game.id" :class="['tab',{active:form.game_ids.includes(game.id)}]" @click="toggleGame(game.id)">{{ game.short_name }}</button></view><view class="row"><button class="btn-secondary" @click="creating=false">取消</button><button class="btn" :disabled="busy" @click="create">提交申请</button></view></view></view></AppShell></template>
<script setup>
/** 文件作用：加载公开与个人俱乐部，验证创建资料后提交审批。 */
import { ref,reactive } from 'vue'
import { onLoad,onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state,init } from '../../lib/store'
import { api } from '../../lib/api'
import { go,notify,statusLabel } from '../../lib/format'
const clubs=ref([]),mine=ref(false),loading=ref(false),error=ref(''),creating=ref(false),busy=ref(false),form=reactive({name:'',description:'',game_ids:[]})
/** 根据入口参数选择个人俱乐部列表，首次加载即可读取本人挂靠。 */
function applyRoute(options){mine.value=options?.mine==='1'}
/** 刷新俱乐部列表，仅登录后才能读取个人挂靠。 */
async function load(){loading.value=true;error.value='';try{await init();if(mine.value&&!state.user){clubs.value=[];return}clubs.value=await api('/clubs'+(mine.value?'?mine=1':''))}catch(e){error.value=e.message}finally{loading.value=false}}
/** 显示游戏短名称。 */
function gameName(id){return state.games.find(item=>item.id===id)?.short_name||id}
/** 打开创建表单，默认选择当前游戏。 */
function openCreate(){if(!state.user)return go('/pages/login/index');form.game_ids=[state.gameId];creating.value=true}
/** 切换俱乐部覆盖的游戏。 */
function toggleGame(id){const index=form.game_ids.indexOf(id);if(index>=0)form.game_ids.splice(index,1);else form.game_ids.push(id)}
/** 验证资料并创建待审核俱乐部。 */
async function create(){if(busy.value)return;if(!form.name.trim()||!form.description.trim()||!form.game_ids.length)return notify('请填写名称、介绍并选择游戏');busy.value=true;try{const club=await api('/clubs','POST',{name:form.name.trim(),description:form.description.trim(),game_ids:form.game_ids});creating.value=false;notify('俱乐部申请已提交');go('/pages/club/index?id='+club.id)}catch(e){notify(e)}finally{busy.value=false}}
onLoad(applyRoute)
onShow(load)
</script>
<style scoped>.club-hero{padding:12px 0 28px}.hero-line{display:block;font-size:28px;line-height:1.5;font-weight:800;letter-spacing:-1px;margin:12px 0}.hero-line>text{display:block}.club-card{margin-bottom:14px}.club-monogram{width:45px;height:45px;flex-shrink:0;border-radius:12px;background:#ffffff08;color:var(--accent);display:flex;align-items:center;justify-content:center;font-size:22px;font-weight:750}.club-heading{flex:1;min-width:0}.club-name{font-size:16px;font-weight:700}.club-description{font-size:12px;line-height:1.8;display:block;color:#a0b1a6;margin:16px 0}.game-tags{display:flex;flex-wrap:wrap;gap:5px}.modal-mask{position:fixed;inset:0;background:#000b;z-index:110;display:flex;align-items:center;padding:22px;justify-content:center}.modal{width:100%;max-width:440px;display:flex;flex-direction:column;gap:15px;max-height:85vh;overflow:auto}.modal>.row>button{flex:1}.game-choices{display:flex;flex-wrap:wrap;gap:7px}</style>
