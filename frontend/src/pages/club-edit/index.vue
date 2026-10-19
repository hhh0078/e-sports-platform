<!-- 文件作用：俱乐部负责人编辑封面、图文介绍、覆盖游戏及加入门槛，提交后重新进行平台审核。 -->
<template>
  <AppShell title="装修俱乐部" subtitle="把俱乐部的风格和规则，认真介绍给新队友" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <view v-if="loading" class="loading-line">正在读取俱乐部资料…</view>
    <EmptyState v-else-if="!canManage" title="仅俱乐部负责人可装修" detail="请登录有管理权限的账号，普通成员可以查看公开主页。" icon="shield"><button class="btn" @click="go(state.user?'/pages/club/index?id='+id:'/pages/login/index?next='+encodeURIComponent('/pages/club-edit/index?id='+id))">{{ state.user?'返回俱乐部':'登录集结' }}</button></EmptyState>
    <template v-else>
      <view class="notice mb">修改资料后将重新提交平台审核，认证会在通过后恢复。</view>
      <view class="edit-panel panel">
        <view class="section-heading"><text>主页形象</text><text>01</text></view>
        <view class="cover-editor"><image v-if="previewCover" :src="assetUrl(previewCover)" mode="aspectFill"/><view class="cover-caption"><text>俱乐部封面 · 16:9</text><button class="btn-secondary btn-small" :disabled="saving||uploading" @click="uploadCover">{{ uploading?'处理中…':'更换封面' }}</button></view></view>
        <view class="field"><text class="field-label">俱乐部名称 · 2—30 个字</text><input v-model="form.name" class="input" maxlength="30" placeholder="给队友一个值得记住的名字"/></view>
        <view class="field"><text class="field-label">简介 · 10—1000 个字</text><textarea v-model="form.description" class="input intro-input" maxlength="1000" placeholder="俱乐部的特色、擅长内容、活跃时间…"/></view>
        <view class="field"><text class="field-label">覆盖游戏 · 可多选</text><view class="game-choices"><button v-for="item in state.games" :key="item.id" :class="{selected:form.game_ids.includes(item.id)}" :disabled="saving" @click="toggleGame(item.id)">{{ item.short_name||item.name }}<Icon v-if="form.game_ids.includes(item.id)" name="check" :size="12"/></button></view></view>
      </view>
      <view class="edit-panel panel">
        <view class="section-heading"><text>图文详情</text><text>02</text></view><text class="hint block-hint">依次展示文字与图片，最多 20 段、10 张图片。文字合计不超过 8000 字。</text>
        <view v-for="(block,index) in form.detail_blocks" :key="block.key" class="detail-editor">
          <view class="block-toolbar"><text>{{ index+1 }} · {{ block.type==='text'?'文字说明':'展示图片' }}</text><view><button :disabled="index===0||saving||uploading" @click="moveBlock(index,-1)">上移</button><button :disabled="index===form.detail_blocks.length-1||saving||uploading" @click="moveBlock(index,1)">下移</button><button :disabled="saving||uploading" class="remove-button" @click="removeBlock(index)">删除</button></view></view>
          <textarea v-if="block.type==='text'" v-model="block.text" class="input detail-textarea" maxlength="2000" placeholder="介绍俱乐部、服务流程或成员要求…"/>
          <image v-else :src="assetUrl(block.image_url)" mode="widthFix" class="detail-image"/>
        </view>
        <view v-if="!form.detail_blocks.length" class="details-empty">添加招募介绍、环境图片或服务说明，让主页更完整。</view>
        <view class="add-actions"><button class="btn-secondary" :disabled="saving||uploading||form.detail_blocks.length>=20" @click="addText"><Icon name="edit" :size="16"/>添加文字</button><button class="btn-secondary" :disabled="saving||uploading||form.detail_blocks.length>=20||imageCount>=10" @click="addImage"><Icon name="image" :size="17"/>{{ uploading?'图片处理中…':'添加图片' }}</button></view>
      </view>
      <view class="edit-panel panel">
        <view class="section-heading"><text>加入规则</text><text>03</text></view>
        <view class="field"><text class="field-label">押金门槛 · 元</text><input v-model="form.deposit" class="input" type="digit" maxlength="10" placeholder="0 表示无押金门槛"/><text class="hint">仅用于展示规则，平台不代收或核验押金。填写 0—100000，可保留两位小数。</text></view>
        <view class="field"><text class="field-label">考核要求 · 最多 1000 个字</text><textarea v-model="form.assessment" class="input assessment-input" maxlength="1000" placeholder="例如段位、试训内容和通过标准。无考核可留空。"/></view>
        <text class="hint">新成员需主动确认当前门槛；规则变化后，原待审核申请须重新确认。</text>
      </view>
      <view class="save-actions"><button class="btn-secondary" :disabled="saving||uploading" @click="cancel">取消修改</button><button class="btn" :disabled="saving||uploading" @click="save">{{ saving?'正在提交…':'保存并提交审核' }}</button></view>
    </template>
  </AppShell>
</template>
<script setup>
import { ref, reactive, computed } from 'vue'
import { onLoad, onShow, onHide, onUnload } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import EmptyState from '../../components/EmptyState.vue'
import Icon from '../../components/Icon.vue'
import { state, game, init, sessionRevision } from '../../lib/store'
import { api, token } from '../../lib/api'
import { chooseUploadImage } from '../../lib/media'
import { go, notify, assetUrl } from '../../lib/format'

const id=ref(''),loading=ref(true),canManage=ref(false),saving=ref(false),uploading=ref(false),error=ref('')
const form=reactive({name:'',description:'',cover_url:'',game_ids:[],detail_blocks:[],deposit:'0',assessment:''})
const previewCover=computed(()=>form.cover_url||state.games.find(item=>item.id===form.game_ids[0])?.cover_url||game.value.cover_url)
const imageCount=computed(()=>form.detail_blocks.filter(block=>block.type==='image').length)
let version=0,blockKey=0,visible=true,disposed=false,savedForm='',loadedSession=null

/** 捕获编辑者会话，防止切换账号后回填旧俱乐部内容。 */
function currentSession(){return{token:token(),revision:sessionRevision(),userId:state.user?.id}}
/** 同时校验令牌、用户编号和会话版本。 */
function sameSession(session){return session&&session.token===token()&&session.revision===sessionRevision()&&session.userId===state.user?.id}
/** 接受当前可见页面发出的最新响应。 */
function currentRequest(current,session){return visible&&!disposed&&current===version&&sameSession(session)}
/** 为编辑段落附上稳定本地编号，编号不会发送到服务端。 */
function editableBlock(block){return{...block,key:++blockKey}}
/** 读取服务端管理权限并建立可取消的独立资料草稿。 */
async function load(){if(!id.value)return;const current=++version;loading.value=true;error.value='';let session;try{await init();if(current!==version||!visible)return;session=currentSession();if(!state.user){canManage.value=false;return}const result=await api('/clubs/'+encodeURIComponent(id.value));if(!currentRequest(current,session))return;canManage.value=!!result.can_manage;if(!canManage.value)return;const club=result.club;Object.assign(form,{name:club.name||'',description:club.description||'',cover_url:club.cover_url||'',game_ids:[...(club.game_ids||[])],detail_blocks:(club.detail_blocks||[]).map(editableBlock),deposit:String((club.deposit_cents||0)/100),assessment:club.assessment||''});savedForm=JSON.stringify(form);loadedSession=session}catch(err){if(current===version&&(!session||sameSession(session)))error.value=err.message}finally{if(current===version)loading.value=false}}
/** 切换游戏选择，至少一个游戏的要求在保存时统一校验。 */
function toggleGame(gameId){const index=form.game_ids.indexOf(gameId);if(index>=0)form.game_ids.splice(index,1);else form.game_ids.push(gameId)}
/** 添加一段文字，保留现有顺序与草稿内容。 */
function addText(){if(form.detail_blocks.length<20)form.detail_blocks.push(editableBlock({type:'text',text:''}))}
/** 交换相邻段落，越界时不改变内容。 */
function moveBlock(index,offset){const target=index+offset;if(target<0||target>=form.detail_blocks.length)return;const block=form.detail_blocks.splice(index,1)[0];form.detail_blocks.splice(target,0,block)}
/** 删除当前尚未保存的图文段落。 */
function removeBlock(index){form.detail_blocks.splice(index,1)}
/** 上传俱乐部封面，原生选图返回后保留其他未保存内容。 */
async function uploadCover(){await upload('club_cover')}
/** 上传详情插图，数量限制与服务器保持一致。 */
async function addImage(){if(form.detail_blocks.length>=20||imageCount.value>=10)return;await upload('club_detail')}
/** 统一上传入口仅接收本账号、本页仍存活的结果。 */
async function upload(purpose){if(saving.value||uploading.value)return;const session=currentSession();version++;uploading.value=true;try{const url=await chooseUploadImage(purpose,{clubId:id.value});if(disposed||!sameSession(session)||!url)return;if(purpose==='club_cover')form.cover_url=url;else form.detail_blocks.push(editableBlock({type:'image',image_url:url}))}catch(err){if(!disposed&&sameSession(session)&&!err.cancelled)notify(err)}finally{uploading.value=false}}
/** 使用十进制字符计算分，拒绝负数、三位小数与非金额输入。 */
function depositCents(value){const raw=String(value).trim()||'0';if(!/^\d{1,6}(\.\d{1,2})?$/.test(raw))throw new Error('押金门槛请填写有效金额，最多两位小数');const[whole,fraction='']=raw.split('.'),cents=Number(whole)*100+Number(fraction.padEnd(2,'0'));if(cents>10000000)throw new Error('押金门槛不能超过 100000 元');return cents}
/** 校验必要资料与完整图文，再提交待审核版本。 */
async function save(){if(saving.value||uploading.value||!canManage.value)return;const name=form.name.trim(),description=form.description.trim(),assessment=form.assessment.trim();let deposit;try{deposit=depositCents(form.deposit)}catch(err){return notify(err)}if(Array.from(name).length<2||Array.from(name).length>30)return notify('俱乐部名称需要 2—30 个字');if(Array.from(description).length<10||Array.from(description).length>1000)return notify('简介需要 10—1000 个字');if(!form.game_ids.length||form.game_ids.length>32)return notify('请选择 1—32 个覆盖游戏');if(Array.from(assessment).length>1000)return notify('考核要求不能超过 1000 个字');const blocks=form.detail_blocks.map(block=>block.type==='text'?{type:'text',text:block.text.trim()}:{type:'image',image_url:block.image_url});if(blocks.length>20||imageCount.value>10)return notify('最多添加 20 段内容和 10 张图片');const textBlocks=blocks.filter(block=>block.type==='text');if(textBlocks.some(block=>!block.text||Array.from(block.text).length>2000))return notify('文字段落需填写 1—2000 个字，空段落可以删除');if(textBlocks.reduce((sum,block)=>sum+Array.from(block.text).length,0)>8000)return notify('图文详情中的文字合计不能超过 8000 字');const current=++version,session=currentSession();saving.value=true;try{await api('/clubs/'+encodeURIComponent(id.value),'PATCH',{name,description,cover_url:form.cover_url,game_ids:[...form.game_ids],detail_blocks:blocks,deposit_cents:deposit,assessment});if(!currentRequest(current,session))return;savedForm=JSON.stringify(form);notify('已保存，等待平台审核');go('/pages/club/index?id='+id.value)}catch(err){if(currentRequest(current,session))notify(err)}finally{saving.value=false}}
/** 取消只恢复本页草稿，不修改已保存俱乐部资料。 */
function cancel(){if(savedForm)Object.assign(form,JSON.parse(savedForm));notify('未保存的修改已取消')}
/** 返回编辑器时保留未保存内容，账号变化则重新校验权限。 */
function showPage(){visible=true;if(loadedSession&&sameSession(loadedSession)&&(uploading.value||JSON.stringify(form)!==savedForm))return;if(id.value)load()}
/** 页面隐藏后使已发出的读取和保存回调失效。 */
function hidePage(){visible=false;version++}
/** 设置俱乐部路由，首屏由显示生命周期统一加载。 */
function applyRoute(options){id.value=options.id||'';if(!id.value){loading.value=false;error.value='缺少俱乐部编号'}}
onLoad(applyRoute)
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage()})
</script>
<style scoped>
.edit-panel{padding:17px;margin-bottom:15px}.section-heading{display:flex;align-items:center;justify-content:space-between;margin-bottom:16px;font-size:18px;font-weight:750}.section-heading>text:last-child{font-size:12px;color:#688877;font-weight:500;letter-spacing:2px}.cover-editor{position:relative;height:170px;border-radius:12px;overflow:hidden;margin-bottom:22px;background:#0b1510}.cover-editor>image{width:100%;height:100%}.cover-caption{position:absolute;left:0;right:0;bottom:0;display:flex;align-items:center;justify-content:space-between;padding:13px;background:#08100cc9;font-size:12px}.cover-caption button{margin:0;font-size:11px}.intro-input{height:120px}.assessment-input{height:135px}.game-choices{display:flex;flex-wrap:wrap;gap:8px;margin-top:10px}.game-choices>button{display:flex;align-items:center;gap:6px;margin:0;padding:5px 12px;background:#ffffff04;border:1px solid #ffffff18;border-radius:18px;color:#9faf9f;font-size:12px;line-height:22px}.game-choices>button.selected{color:var(--accent);border-color:var(--accent);background:#80bd9a0a}.block-hint{display:block;line-height:21px;margin-bottom:13px}.detail-editor{border:1px solid #ffffff16;border-radius:10px;padding:12px;margin:14px 0;background:#ffffff02}.block-toolbar{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:12px;font-size:12px;color:#a7bcb0}.block-toolbar>view{display:flex;gap:11px}.block-toolbar button{margin:0;padding:0;background:none;color:#9fbdab;font-size:11px;line-height:20px}.block-toolbar .remove-button{color:#d2aa96}.block-toolbar button[disabled]{opacity:.35}.detail-textarea{height:150px;font-size:13px}.detail-image{display:block;width:100%;border-radius:6px}.details-empty{padding:25px 12px;border:1px dashed #ffffff20;border-radius:9px;color:#7f9c8a;font-size:12px;line-height:22px;text-align:center}.add-actions{display:flex;gap:9px;margin-top:15px}.add-actions>button{flex:1;display:flex;align-items:center;justify-content:center;gap:6px;font-size:12px;padding:9px 5px}.save-actions{display:flex;gap:10px;margin-top:22px}.save-actions>button{flex:1;font-size:13px}.save-actions>.btn{flex:1.6}.field>.hint{display:block;line-height:21px;margin-top:8px}
</style>
