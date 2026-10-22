<!-- 文件作用：俱乐部负责人编辑封面、图文介绍、覆盖游戏及加入门槛，提交后重新进行平台审核。 -->
<template>
  <AppShell title="俱乐部装修" :game-id="form.game_ids[0]" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <view v-if="loading" class="loading-line">正在读取俱乐部资料…</view>
    <EmptyState v-else-if="!canManage" title="仅俱乐部负责人可装修" detail="请登录有管理权限的账号，普通成员可以查看公开主页。" icon="shield"><button class="text-btn" @click="go(state.user?'/pages/community/clubs/detail?id='+id:'/pages/account/login?next='+encodeURIComponent('/pages/community/clubs/edit?id='+id))">{{ state.user?'返回俱乐部':'登录开机电竞' }} <Icon name="arrow" :size="12"/></button></EmptyState>
    <template v-else>
      <text class="subtle-note review-note">修改资料后重新审核，认证在通过后恢复。</text>
      <view class="form-section identity-section">
        <text class="section-title">主页形象</text>
        <view class="cover-editor"><image v-if="previewCover" class="cover-image" :src="assetUrl(previewCover)" mode="aspectFill"/><view class="cover-caption"><text>16:9 · 自动裁剪压缩</text><button class="text-btn cover-change" :disabled="saving||uploading" @click="uploadCover"><Icon name="image" :size="14"/>{{ uploading?'处理中…':'更换封面' }}</button></view></view>
        <view class="field"><text class="field-label">俱乐部名称 · 2—30 个字</text><input v-model="form.name" class="input" maxlength="30" placeholder="给队友一个值得记住的名字"/></view>
        <view class="field"><text class="field-label">简介 · 10—1000 个字</text><textarea v-model="form.description" class="input intro-input" maxlength="1000" placeholder="俱乐部的特色、擅长内容、活跃时间…"/></view>
        <view class="field club-edit-game-field"><text class="field-label">覆盖游戏 · 可多选</text><GameSelector v-model:values="form.game_ids" multiple title="覆盖游戏" :disabled="saving||uploading"/></view>
      </view>
      <view class="form-section details-section">
        <view class="section-heading"><text class="section-title">图文详情</text><view class="add-actions"><button class="text-btn" :disabled="saving||uploading||form.detail_blocks.length>=20" @click="addText"><Icon name="edit" :size="15"/>添加文字</button><button class="text-btn" :disabled="saving||uploading||form.detail_blocks.length>=20||imageCount>=10" @click="addImage"><Icon name="image" :size="15"/>{{ uploading?'图片处理中…':'添加图片' }}</button></view></view>
        <text class="hint block-hint">最多 20 段、10 张图片，文字合计不超过 8000 字。</text>
        <view v-for="(block,index) in form.detail_blocks" :key="block.key" class="detail-editor">
          <view class="block-toolbar"><text>{{ index+1 }} · {{ block.type==='text'?'文字说明':'展示图片' }}</text><view class="block-actions"><button class="text-btn block-action" :disabled="index===0||saving||uploading" @click="moveBlock(index,-1)">上移</button><button class="text-btn block-action" :disabled="index===form.detail_blocks.length-1||saving||uploading" @click="moveBlock(index,1)">下移</button><button :disabled="saving||uploading" class="text-btn block-action remove-button" @click="removeBlock(index)">删除</button></view></view>
          <textarea v-if="block.type==='text'" v-model="block.text" class="input detail-textarea" maxlength="2000" placeholder="介绍俱乐部、服务流程或成员要求…"/>
          <image v-else :src="assetUrl(block.image_url)" mode="widthFix" class="detail-image"/>
        </view>
        <text v-if="!form.detail_blocks.length" class="details-empty">暂未添加图文内容。</text>
      </view>
      <view class="form-section rules-section">
        <text class="section-title">加入规则</text>
        <view class="field"><text class="field-label">押金门槛 · 元</text><input v-model="form.deposit" class="input" type="digit" maxlength="10" placeholder="0 表示无押金门槛"/><text class="hint">填写 0—100000 元，最多两位小数。平台不代收或核验押金。</text></view>
        <view class="field"><text class="field-label">考核要求 · 最多 1000 个字</text><textarea v-model="form.assessment" class="input assessment-input" maxlength="1000" placeholder="例如段位、试训内容和通过标准。无考核可留空。"/></view>
        <text class="hint">新成员须确认当前门槛；规则变化后，待审核申请须重新确认。</text>
      </view>
      <view class="action-footer save-actions"><button class="btn save-button" :disabled="saving||uploading" @click="save">{{ saving?'正在提交…':'保存并提交审核' }}</button><button class="text-btn cancel-button" :disabled="saving||uploading" @click="cancel">取消修改</button></view>
    </template>
  </AppShell>
</template>
<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import GameSelector from '../../../components/GameSelector.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import { state, game, init, sessionRevision } from '../../../lib/store'
import { getClub, updateClub } from '../../../api/clubs.api'
import type { CommunityBlock } from '../../../types/models'
import { token, errorMessage, isCancelled } from '../../../api/client'
import { chooseUploadImage } from '../../../lib/media'
import { go, notify, assetUrl } from '../../../lib/format'

/** 带本地稳定编号的装修段落；编号不传给后端。 */
type EditableBlock = CommunityBlock & {key: number}
const id=ref(''),loading=ref(true),canManage=ref(false),saving=ref(false),uploading=ref(false),error=ref('')
const form=reactive({name:'',description:'',cover_url:'',game_ids:[] as string[],detail_blocks:[] as EditableBlock[],deposit:'0',assessment:''})
const previewCover=computed(()=>form.cover_url||state.games.find(item=>item.id===form.game_ids[0])?.cover_url||game.value.cover_url)
const imageCount=computed(()=>form.detail_blocks.filter(block=>block.type==='image').length)
let version=0,blockKey=0,visible=true,disposed=false,savedForm='',loadedSession: ReturnType<typeof currentSession> | null=null

/** 捕获编辑者会话，防止切换账号后回填旧俱乐部内容。 */
function currentSession(){return{token:token(),revision:sessionRevision(),userId:state.user?.id}}
/** 同时校验令牌、用户编号和会话版本。 */
function sameSession(session: ReturnType<typeof currentSession> | null | undefined){return session&&session.token===token()&&session.revision===sessionRevision()&&session.userId===state.user?.id}
/** 接受当前可见页面发出的最新响应。 */
function currentRequest(current: number,session: ReturnType<typeof currentSession>){return visible&&!disposed&&current===version&&sameSession(session)}
/** 为编辑段落附上稳定本地编号，编号不会发送到服务端。 */
function editableBlock(block: CommunityBlock): EditableBlock{return{...block,key:++blockKey}}
/** 读取服务端管理权限并建立可取消的独立资料草稿。 */
async function load(){if(!id.value)return;const current=++version;loading.value=true;error.value='';let session: ReturnType<typeof currentSession> | undefined;try{await init();if(current!==version||!visible)return;session=currentSession();if(!state.user){canManage.value=false;return}const result=await getClub(id.value);if(!currentRequest(current,session))return;canManage.value=!!result.can_manage;if(!canManage.value)return;const club=result.club;Object.assign(form,{name:club.name||'',description:club.description||'',cover_url:club.cover_url||'',game_ids:[...(club.game_ids||[])],detail_blocks:(club.detail_blocks||[]).map(editableBlock),deposit:String((club.deposit_cents||0)/100),assessment:club.assessment||''});savedForm=JSON.stringify(form);loadedSession=session}catch(err){if(current===version&&(!session||sameSession(session)))error.value=errorMessage(err)}finally{if(current===version)loading.value=false}}
/** 添加一段文字，保留现有顺序与草稿内容。 */
function addText(){if(form.detail_blocks.length<20)form.detail_blocks.push(editableBlock({type:'text',text:''}))}
/** 交换相邻段落，越界时不改变内容。 */
function moveBlock(index: number,offset: number){const target=index+offset;if(target<0||target>=form.detail_blocks.length)return;const block=form.detail_blocks.splice(index,1)[0];form.detail_blocks.splice(target,0,block)}
/** 删除当前尚未保存的图文段落。 */
function removeBlock(index: number){form.detail_blocks.splice(index,1)}
/** 上传俱乐部封面，原生选图返回后保留其他未保存内容。 */
async function uploadCover(){await upload('club_cover')}
/** 上传详情插图，数量限制与服务器保持一致。 */
async function addImage(){if(form.detail_blocks.length>=20||imageCount.value>=10)return;await upload('club_detail')}
/** 统一上传入口仅接收本账号、本页仍存活的结果。 */
async function upload(purpose: 'club_cover' | 'club_detail'){if(saving.value||uploading.value)return;const session=currentSession();version++;uploading.value=true;try{const url=await chooseUploadImage(purpose,{clubId:id.value});if(disposed||!sameSession(session)||!url)return;if(purpose==='club_cover')form.cover_url=url;else form.detail_blocks.push(editableBlock({type:'image',image_url:url}))}catch(err){if(!disposed&&sameSession(session)&&!isCancelled(err))notify(err)}finally{uploading.value=false}}
/** 使用十进制字符计算分，拒绝负数、三位小数与非金额输入。 */
function depositCents(value: string){const raw=String(value).trim()||'0';if(!/^\d{1,6}(\.\d{1,2})?$/.test(raw))throw new Error('押金门槛请填写有效金额，最多两位小数');const[whole,fraction='']=raw.split('.'),cents=Number(whole)*100+Number(fraction.padEnd(2,'0'));if(cents>10000000)throw new Error('押金门槛不能超过 100000 元');return cents}
/** 校验必要资料与完整图文，再提交待审核版本。 */
async function save(){if(saving.value||uploading.value||!canManage.value)return;const name=form.name.trim(),description=form.description.trim(),assessment=form.assessment.trim();let deposit;try{deposit=depositCents(form.deposit)}catch(err){return notify(err)}if(Array.from(name).length<2||Array.from(name).length>30)return notify('俱乐部名称需要 2—30 个字');if(Array.from(description).length<10||Array.from(description).length>1000)return notify('简介需要 10—1000 个字');if(!form.game_ids.length||form.game_ids.length>32)return notify('请选择 1—32 个覆盖游戏');if(Array.from(assessment).length>1000)return notify('考核要求不能超过 1000 个字');const blocks: CommunityBlock[]=form.detail_blocks.map(block=>block.type==='text'?{type:'text',text:(block.text||'').trim()}:{type:'image',image_url:block.image_url});if(blocks.length>20||imageCount.value>10)return notify('最多添加 20 段内容和 10 张图片');const textBlocks=blocks.filter(block=>block.type==='text');if(textBlocks.some(block=>!block.text||Array.from(block.text||'').length>2000))return notify('文字段落需填写 1—2000 个字，空段落可以删除');if(textBlocks.reduce((sum,block)=>sum+Array.from(block.text||'').length,0)>8000)return notify('图文详情中的文字合计不能超过 8000 字');const current=++version,session=currentSession();saving.value=true;try{await updateClub(id.value,{name,description,cover_url:form.cover_url,game_ids:[...form.game_ids],detail_blocks:blocks,deposit_cents:deposit,assessment});if(!currentRequest(current,session))return;savedForm=JSON.stringify(form);notify('已保存，等待平台审核');go('/pages/community/clubs/detail?id='+id.value)}catch(err){if(currentRequest(current,session))notify(err)}finally{saving.value=false}}
/** 取消只恢复本页草稿，不修改已保存俱乐部资料。 */
function cancel(){if(savedForm)Object.assign(form,JSON.parse(savedForm));notify('未保存的修改已取消')}
/** 返回编辑器时保留未保存内容，账号变化则重新校验权限。 */
function showPage(){visible=true;if(loadedSession&&sameSession(loadedSession)&&(uploading.value||JSON.stringify(form)!==savedForm))return;if(id.value)load()}
/** 页面隐藏后使已发出的读取和保存回调失效。 */
function hidePage(){visible=false;version++}
/** 设置俱乐部路由，首屏由显示生命周期统一加载。 */
function applyRoute(options: Record<string,string | undefined> = {}){id.value=options.id||'';if(!id.value){loading.value=false;error.value='缺少俱乐部编号'}}
onLoad(applyRoute)
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.review-note{padding:0 0 8px;font-size:11px}.identity-section{padding-top:12px}.cover-editor{position:relative;height:0;padding-top:56.25%;border-radius:8px;overflow:hidden;background:var(--surface);margin-bottom:20px}.cover-image{position:absolute;inset:0;width:100%;height:100%}.cover-caption{position:absolute;left:0;right:0;bottom:0;display:flex;align-items:center;justify-content:space-between;gap:8px;padding:4px 8px 4px 12px;background:#08100c9c;font-size:10px;line-height:18px;color:#dbe7df}.cover-change{min-height:36px;padding:0 8px;color:#edf4ef;font-size:11px}.intro-input{height:96px}.assessment-input{height:112px}.section-heading{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px 12px;min-width:0;margin-bottom:8px}.add-actions{display:flex;align-items:center;flex-wrap:wrap;gap:12px;min-width:0}.add-actions .text-btn{padding:0;font-size:11px;color:var(--text)}.block-hint{display:block;font-size:11px;line-height:20px}.detail-editor{min-width:0;padding:16px 0;border-bottom:1px solid var(--line-subtle)}.block-toolbar{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;min-width:0;margin-bottom:8px;font-size:11px;line-height:20px;color:var(--muted)}.block-actions{display:flex;align-items:center;gap:12px;min-width:0}.block-action{padding:0;color:var(--muted);font-size:11px}.block-action.remove-button{color:#e7ae96}.detail-textarea{height:112px}.detail-image{display:block;width:100%;border-radius:8px}.details-empty{display:block;padding:16px 0 0;font-size:12px;line-height:22px;color:var(--muted)}.save-actions{flex-direction:column;gap:12px;padding:24px 0 0}.save-actions .save-button{flex:none;width:100%}.cancel-button{color:var(--muted)}
@media(max-width:350px){.cover-caption{padding-left:8px;font-size:9px}.cover-change{padding:0 4px}.add-actions{gap:10px}}
</style>
