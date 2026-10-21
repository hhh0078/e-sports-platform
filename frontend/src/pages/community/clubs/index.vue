<!-- 文件作用：发现俱乐部、查看个人挂靠并申请创建新的电竞俱乐部。 -->
<template>
  <AppShell title="俱乐部" back>
    <view class="club-search"><Icon name="search" :size="18"/><input v-model="query" class="search-input" placeholder="搜索俱乐部" confirm-type="search" @confirm="load"/></view>
    <view class="club-toolbar"><view class="club-tabs"><button :class="['club-tab',{active:!mine}]" @click="mine=false;load()">发现俱乐部</button><button :class="['club-tab',{active:mine}]" @click="mine=true;load()">我的俱乐部</button></view><button class="text-btn create-entry" @click="openCreate">创建</button></view>
    <view v-if="error" class="list-error" @click="load">{{ error }} · 重试</view>
    <view v-if="loading&&!clubs.length" class="loading-line">正在读取俱乐部…</view>
    <view v-else-if="!clubs.length&&!error" class="club-empty"><text class="empty-title">{{ mine&&!state.user?'请登录后查看':mine?'暂未加入俱乐部':'暂未找到俱乐部' }}</text><text class="hint">{{ mine?'到发现页寻找合适的团队，或创建自己的俱乐部。':'试试其他名称，或成为第一位组织者。' }}</text><button v-if="mine&&!state.user" class="text-btn" @click="go('/pages/account/login?next='+encodeURIComponent('/pages/community/clubs/index?mine=1'))">登录集结 <Icon name="arrow" :size="12"/></button></view>
    <view v-for="club in clubs" :key="club.id" class="club-entry" @click="go('/pages/community/clubs/detail?id='+club.id)">
      <view class="club-card-cover"><image class="club-cover-image" :src="assetUrl(club.cover_url||state.games.find(item=>club.game_ids.includes(item.id))?.cover_url)" mode="aspectFill"/></view>
      <view class="club-identity"><view class="club-monogram">{{ club.name?.slice(0,1) }}</view><view class="club-heading"><view class="club-name-row"><text class="club-name">{{ club.name }}</text><Icon v-if="club.verified" name="shield" :size="15" class="accent"/></view><text class="club-meta">{{ club.member_count||0 }} 位成员 · {{ club.owner_name }} 主理</text></view></view>
      <text class="club-description">{{ club.description }}</text>
      <view class="game-tags"><text v-for="id in club.game_ids" :key="id" class="pill">{{ gameName(id) }}</text></view>
      <view class="club-footer"><view :class="['club-verification',club.status==='active'?'accent':'muted']"><Icon :name="club.verified?'shield':'club'" :size="13"/><text>{{ club.status==='active'?(club.verified?'平台认证':'开放加入'):statusLabel(club.status) }}</text></view><Icon name="arrow" :size="16" class="muted"/></view>
    </view>
    <view v-if="creating" class="modal-mask" @click="creating=false"><view class="modal" @click.stop>
      <scroll-view scroll-y class="modal-scroll"><view class="create-form form-section">
        <text class="section-title">创建俱乐部</text><text class="create-hint hint">提交平台审核后开放成员挂靠。</text>
        <view class="field"><text class="field-label">俱乐部名称</text><input v-model="form.name" class="input" maxlength="30" placeholder="俱乐部名称"/></view>
        <view class="field"><text class="field-label">团队介绍</text><textarea v-model="form.description" class="input intro-input" maxlength="500" placeholder="介绍团队擅长的游戏、招募要求与服务风格"/></view>
        <view class="field"><text class="field-label">主要游戏 · 可多选</text><view class="game-choices"><button v-for="game in state.games" :key="game.id" :class="['game-choice',{selected:form.game_ids.includes(game.id)}]" @click="toggleGame(game.id)">{{ game.short_name }}<Icon v-if="form.game_ids.includes(game.id)" name="check" :size="12"/></button></view></view>
      </view></scroll-view>
      <view class="action-footer modal-actions"><button class="text-btn" @click="creating=false">取消</button><button class="btn" :disabled="busy" @click="create">{{ busy?'正在提交…':'提交申请' }}</button></view>
    </view></view>
  </AppShell>
</template>
<script setup lang="ts">
/** 文件作用：加载公开与个人俱乐部，验证创建资料后提交审批。 */
import { ref,reactive,watch } from 'vue'
import { onLoad,onShow,onHide,onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import Icon from '../../../components/Icon.vue'
import { state,init,sessionRevision } from '../../../lib/store'
import { listClubs, createClub } from '../../../api/clubs.api'
import type { Club } from '../../../types/models'
import { token,errorMessage } from '../../../api/client'
import { go,notify,statusLabel,assetUrl } from '../../../lib/format'
const clubs=ref<Club[]>([]),query=ref(''),mine=ref(false),loading=ref(false),error=ref(''),creating=ref(false),busy=ref(false),form=reactive({name:'',description:'',game_ids:[] as string[]})
let listVersion=0,pageVersion=0,createVersion=0,visible=true,disposed=false
/** 捕获当前身份和会话，防止旧身份的列表或创建结果回填。 */
function currentSession(){return{userId:state.user?.id,token:token(),revision:sessionRevision()}}
/** 校验请求期间的账号、令牌与会话版本均未改变。 */
function sameSession(session: ReturnType<typeof currentSession>){return session.userId===state.user?.id&&session.token===token()&&session.revision===sessionRevision()}
/** 仅接收当前可见页面发出的最新列表请求。 */
function currentList(current: number,session: ReturnType<typeof currentSession>){return visible&&!disposed&&current===listVersion&&sameSession(session)}
/** 账号变化立即清除私有列表和创建草稿，再按新身份刷新。 */
function resetIdentity(){listVersion++;pageVersion++;createVersion++;clubs.value=[];error.value='';loading.value=false;creating.value=false;busy.value=false;Object.assign(form,{name:'',description:'',game_ids:[]});if(visible&&!disposed)load()}
watch(()=>state.user?.id,resetIdentity)
/** 根据入口参数选择个人俱乐部列表，首次加载即可读取本人挂靠。 */
function applyRoute(options: Record<string,string | undefined> = {}){mine.value=options?.mine==='1'}
/** 刷新俱乐部列表，仅登录后才能读取个人挂靠。 */
async function load(){if(!visible||disposed)return;const current=++listVersion,onlyMine=mine.value,search=query.value.trim();let session: ReturnType<typeof currentSession> | undefined;loading.value=true;error.value='';try{await init();if(!visible||disposed||current!==listVersion)return;session=currentSession();if(onlyMine&&!state.user){clubs.value=[];return}const result=await listClubs({...onlyMine?{mine:1}:{},q:search});if(currentList(current,session))clubs.value=result}catch(e){if(visible&&!disposed&&current===listVersion&&(!session||sameSession(session)))error.value=errorMessage(e)}finally{if(current===listVersion)loading.value=false}}
/** 显示游戏短名称。 */
function gameName(id: string){return state.games.find(item=>item.id===id)?.short_name||id}
/** 打开创建表单，默认选择当前游戏。 */
function openCreate(){if(!state.user)return go('/pages/account/login');form.game_ids=[state.gameId];creating.value=true}
/** 切换俱乐部覆盖的游戏。 */
function toggleGame(id: string){const index=form.game_ids.indexOf(id);if(index>=0)form.game_ids.splice(index,1);else form.game_ids.push(id)}
/** 验证资料并创建待审核俱乐部。 */
async function create(){if(busy.value||!state.user||!visible||disposed)return;if(!form.name.trim()||!form.description.trim()||!form.game_ids.length)return notify('请填写名称、介绍并选择游戏');const current=pageVersion,operation=++createVersion,session=currentSession();busy.value=true;try{const club=await createClub({name:form.name.trim(),description:form.description.trim(),game_ids:[...form.game_ids]});if(!visible||disposed||current!==pageVersion||!sameSession(session))return;creating.value=false;notify('俱乐部申请已提交');go('/pages/community/clubs/detail?id='+club.id)}catch(e){if(visible&&!disposed&&current===pageVersion&&sameSession(session))notify(e)}finally{if(operation===createVersion)busy.value=false}}
/** 回到列表时刷新数据，同一身份的创建草稿仍保留。 */
function showPage(){visible=true;load()}
/** 离开页面后使在途列表和创建结果失效。 */
function hidePage(){visible=false;listVersion++;pageVersion++;loading.value=false}
onLoad(applyRoute)
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.club-search{display:flex;align-items:center;gap:10px;min-width:0;border-bottom:1px solid var(--line);color:var(--muted)}.search-input{flex:1;min-width:0;height:44px;padding:0;background:transparent;font-size:14px}.club-toolbar{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0;margin:8px 0 16px}.club-tabs{display:flex;gap:20px;min-width:0;max-width:calc(100% - 44px)}.club-tab{display:flex;align-items:center;justify-content:center;min-width:0;height:44px;padding:0;border-radius:0;border-bottom:2px solid transparent;background:transparent;font-size:14px;color:var(--muted);white-space:nowrap}.club-tab.active{border-bottom-color:var(--accent);color:var(--text);font-weight:650}.create-entry{padding:0;min-width:32px;font-size:14px}.list-error{padding:12px 0;color:#efb299;font-size:12px;overflow-wrap:anywhere}.club-empty{display:flex;flex-direction:column;align-items:center;gap:12px;padding:48px 0;text-align:center;min-width:0}.empty-title{font-size:15px;color:var(--text)}
.club-entry{min-width:0;padding-bottom:16px;border-bottom:1px solid var(--line-subtle);margin-bottom:24px}.club-card-cover{position:relative;height:0;padding-top:56.25%;overflow:hidden;border-radius:8px;background:var(--surface);margin-bottom:12px}.club-cover-image{position:absolute;inset:0;width:100%;height:100%}.club-identity{display:flex;align-items:center;gap:12px;min-width:0}.club-monogram{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:44px;height:44px;border:1px solid var(--line);border-radius:8px;background:transparent;color:var(--text);font-size:22px;font-weight:650}.club-heading{flex:1;min-width:0}.club-name-row{display:flex;align-items:center;gap:6px;min-width:0}.club-name{min-width:0;font-size:18px;line-height:26px;font-weight:700;overflow-wrap:anywhere}.club-meta{display:block;font-size:11px;line-height:18px;color:var(--muted);overflow-wrap:anywhere}.club-description{display:block;margin:10px 0 12px;font-size:12px;line-height:21px;color:var(--muted);overflow:hidden;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow-wrap:anywhere}.game-tags{display:flex;flex-wrap:wrap;gap:6px;min-width:0;max-width:100%}.club-footer{display:flex;align-items:center;justify-content:space-between;gap:12px;min-width:0;margin-top:12px}.club-verification{display:flex;align-items:center;gap:6px;min-width:0;font-size:11px;line-height:20px;overflow-wrap:anywhere}
.modal-mask{position:fixed;inset:0;z-index:110;display:flex;align-items:center;justify-content:center;padding:20px;background:#000b}.modal{width:100%;max-width:460px;overflow:hidden;background:var(--surface);border-radius:12px;min-width:0}.modal-scroll{height:60vh;max-height:560px;padding:20px 20px 0;box-sizing:border-box}.create-form{padding:0;border:0}.create-hint{display:block;margin:-8px 0 20px}.intro-input{height:112px}.game-choices{display:flex;flex-wrap:wrap;gap:8px;min-width:0;max-width:100%;padding-bottom:12px}.game-choice{display:flex;align-items:center;justify-content:center;gap:6px;max-width:100%;min-height:28px;padding:4px 10px;background:transparent;border:1px solid var(--line);border-radius:6px;font-size:11px;line-height:18px;color:var(--muted)}.game-choice.selected{border-color:var(--accent);color:var(--accent)}.modal-actions{padding:20px;gap:12px}
@media(max-width:350px){.club-tabs{gap:16px}.club-tab{font-size:13px}.club-meta{font-size:10px}.modal-mask{padding:16px}.modal-scroll{padding:16px 16px 0}.modal-actions{padding:16px}}
</style>
