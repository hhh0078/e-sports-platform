<!-- 文件作用：管理员分栏配置平台/游戏轮播、首页公告、四页提示及多行注意事项，按修订号保存并保留冲突草稿。 -->
<template><AppShell title="公共信息" back><view v-if="error&&!loaded" class="error-box" @click="load">{{error}} · 点击重试</view><view v-if="loading&&!loaded" class="loading-line">正在读取公共信息…</view><view v-else-if="!allowed" class="subtle-note">仅平台管理员可以配置公共信息。</view><template v-else-if="loaded"><view v-if="error" class="error-box"><text>{{error}}</text><view class="form-actions"><button class="text-btn" @click="copyDraft">复制当前草稿</button><button class="text-btn" @click="reload">重新读取配置</button></view></view><view class="tabs content-tabs"><button v-for="item in tabs" :key="item.id" class="tab" :class="{active:tab===item.id}" @click="tab=item.id">{{item.label}}</button></view><view :class="{'content-disabled':busy}"><view v-if="tab==='slides'"><view class="scope-switch"><button class="text-btn" :class="{muted:!!selectedGame}" @click="selectedGame=''">平台轮播</button><GameSelector :value="selectedGame" placeholder="配置游戏轮播" :disabled="busy" @change="selectedGame=$event"/></view><text class="subtle-note">{{selectedGame?gameName+'的独立轮播，按当前顺序展示。':'平台图片与一个游戏轮播位置混排。游戏位置会展开当前游戏的多张轮播。'}}长按手柄拖动排序。</text><SortSlideList :model-value="slides" @update:model-value="setSlides" @edit="editSlide" @remove="removeSlide"/><view class="form-actions content-add"><button class="btn-secondary btn-small" :disabled="busy||slides.length>=12" @click="editSlide()"><Icon name="plusempty" :size="17"/>新增{{selectedGame?'游戏':'平台'}}轮播</button><button v-if="!selectedGame&&!slides.some(item=>item.kind==='game_slot')" class="text-btn" :disabled="busy||slides.length>=12" @click="addGameSlot">加入游戏轮播位置</button></view></view><view v-else-if="tab==='announcement'" class="form-section"><view class="section-row"><text class="section-title">首页公告</text><NativeSwitch :checked="draft.announcement.enabled" :disabled="busy" color="#83e7b7" @change="draft.announcement.enabled=$event.detail.value"/></view><view class="tabs"><button class="tab" :class="{active:draft.announcement.frequency==='always'}" @click="draft.announcement.frequency='always'">每次进入首页</button><button class="tab" :class="{active:draft.announcement.frequency==='once'}" @click="draft.announcement.frequency='once'">每个版本仅一次</button></view><text class="subtle-note">修改内容会更新公告版本。登录按账号记录，访客按本机记录。</text><view class="tabs"><button class="tab" :class="{active:draft.announcement.mode==='image'}" @click="draft.announcement.mode='image'">图片模式</button><button class="tab" :class="{active:draft.announcement.mode==='text'}" @click="draft.announcement.mode='text'">文字模式</button></view><view v-if="draft.announcement.mode==='image'" class="announcement-editor-image"><image v-if="draft.announcement.image_url" class="announcement-preview" :src="assetUrl(draft.announcement.image_url)" mode="widthFix"/><button class="btn-secondary btn-small" :disabled="busy" @click="uploadAnnouncement">{{draft.announcement.image_url?'更换图片':'上传公告图片'}}</button><text class="subtle-note">保留原比例，不裁剪。会压缩图片并验证大小；展示时图片不加底色，带清晰关闭入口。</text></view><view class="field"><text class="field-label">公告标题 · 最多60字</text><input v-model="draft.announcement.title" class="input" maxlength="60" :disabled="busy" placeholder="平台公告"/></view><view v-if="draft.announcement.mode==='text'" class="field"><text class="field-label">公告正文 · 最多3000字，支持换行</text><textarea v-model="draft.announcement.body" class="input content-textarea" maxlength="3000" :disabled="busy" placeholder="填写公告内容，可换行"/></view><view class="field"><text class="field-label">点击公告跳转</text><ContentTargetPicker v-model="draft.announcement.target"/></view></view><view v-else><view v-for="item in tipTabs" :key="item.id" class="form-section"><text class="section-title">{{item.label}}页顶部提示</text><textarea v-model="draft.page_tips[item.id].text" class="input" maxlength="300" :disabled="busy" placeholder="最多300字，可换行；留空则不显示"/><ContentTargetPicker :model-value="draft.page_tips[item.id].article_id?{kind:'article',id:draft.page_tips[item.id].article_id}:null" :kinds="['article']" @update:model-value="setTipArticle(item.id,$event)"/></view><view class="form-section"><text class="section-title">服务注意事项</text><textarea v-model="draft.service_notice" class="input content-textarea" maxlength="2000" :disabled="busy" placeholder="展示在服务详情，最多2000字，支持换行"/></view><view class="form-section"><text class="section-title">需求注意事项</text><textarea v-model="draft.request_notice" class="input content-textarea" maxlength="2000" :disabled="busy" placeholder="展示在需求详情，最多2000字，支持换行"/></view></view></view><view class="action-footer"><button class="btn" :disabled="busy||!loaded" @click="save">{{busy?'正在保存…':'保存公共信息'}}</button></view></template></AppShell></template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {onLoad,onShow,onHide,onUnload} from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import Icon from '../../../components/Icon.vue'
import NativeSwitch from '../../../components/NativeSwitch.vue'
import GameSelector from '../../../components/GameSelector.vue'
import SortSlideList from '../../../components/SortSlideList.vue'
import ContentTargetPicker from '../../../components/ContentTargetPicker.vue'
import {getAdminPublicContent,savePublicContent} from '../../../api/content.api'
import {errorMessage,errorStatus,isCancelled,token} from '../../../api/client'
import {state,init,sessionRevision} from '../../../lib/store'
import {emptyPublicContent} from '../../../lib/content'
import {assetUrl,go,notify} from '../../../lib/format'
import {chooseUploadImage} from '../../../lib/media'
import type {AdminPublicContent,ContentSlide,ContentTarget,PublicContentPatch} from '../../../types/content'
import type {MainTab} from '../../../lib/navigation'
const draft=ref(emptyPublicContent()),tab=ref<'slides'|'announcement'|'tips'>('slides'),selectedGame=ref(''),loading=ref(false),loaded=ref(false),busy=ref(false),error=ref(''),dirty=ref(false)
const allowed=computed(()=>state.user?.role==='admin'&&state.user.status!=='frozen'),gameName=computed(()=>state.games.find(item=>item.id===selectedGame.value)?.name||selectedGame.value),slides=computed(()=>selectedGame.value?draft.value.game_slides[selectedGame.value]||[]:draft.value.global_slides)
const tabs=[{id:'slides' as const,label:'轮播图'},{id:'announcement' as const,label:'公告'},{id:'tips' as const,label:'顶部提示'}],tipTabs:Array<{id:MainTab;label:string}>=[{id:'player',label:'玩家'},{id:'worker',label:'打手'},{id:'orders',label:'订单'},{id:'me',label:'我的'}]
let visible=false,disposed=false,readVersion=0,operationVersion=0,dialogVersion=0,confirming=false,loadedSession:ReturnType<typeof session>|null=null
watch(draft,()=>{dirty.value=true},{deep:true,flush:'sync'})
/** 捕获实际管理员、令牌与认证修订，原生弹框和保存结果都绑定发起人。 */
function session(){return{id:state.user?.id,role:state.user?.role,adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(),revision:sessionRevision()}}
/** 原生选图临时隐藏不改变合法任务归属，切号和撤权立即失效。 */
function sameSession(value:ReturnType<typeof session>|null):boolean{return !!value&&!disposed&&allowed.value&&value.id===state.user?.id&&value.role===state.user?.role&&value.adminRole===state.user?.platform_admin_role&&value.status===state.user?.status&&value.token===token()&&value.revision===sessionRevision()}
/** 清空私有配置并撤销所有旧操作租约，旧finally不能解锁新管理员。 */
function clear():void{readVersion++;operationVersion++;dialogVersion++;confirming=false;draft.value=emptyPublicContent();dirty.value=false;loaded.value=false;loading.value=false;busy.value=false;error.value='';loadedSession=null}
/** 回填服务端确认的修订号，不在冲突时自动覆盖草稿。 */
function apply(value:AdminPublicContent):void{draft.value=value;dirty.value=false;loaded.value=true;loadedSession=session()}
/** 读取完整编辑稿；初始化结束先核验页面与代次，失权清空私有内容。 */
async function load():Promise<void>{
  if(!visible||disposed||loading.value||busy.value)return
  const sequence=++readVersion;loading.value=true;error.value='';let actor:ReturnType<typeof session>|null=null
  try{await init();if(!visible||disposed||sequence!==readVersion)return;if(!allowed.value){clear();return}actor=session();const value=await getAdminPublicContent();if(visible&&sequence===readVersion&&sameSession(actor))apply(value)}
  catch(err){if(visible&&sequence===readVersion&&(!actor||sameSession(actor))){if([401,403].includes(errorStatus(err)||0))clear();error.value=errorMessage(err)}}
  finally{if(sequence===readVersion)loading.value=false}
}
/** 丢弃草稿需要当前管理员明确确认，旧账号或隐藏期间的确认不得读取新草稿。 */
async function reload():Promise<void>{
  if(!visible||busy.value||loading.value||confirming||!allowed.value)return
  const actor=session(),sequence=++dialogVersion,draftAtStart=draft.value
  if(dirty.value){confirming=true;try{const answer=await uni.showModal({title:'重新读取配置',content:'当前未保存的修改会被替换。可先复制草稿。',confirmText:'重新读取'});if(!answer.confirm)return}finally{if(sequence===dialogVersion)confirming=false}}
  if(!visible||sequence!==dialogVersion||busy.value||!sameSession(actor)||draft.value!==draftAtStart)return
  await load()
}
/** 复制仅含本管理员公共信息草稿的内容，不复制其它账号或支付登录资料。 */
function copyDraft():void{if(loaded.value&&sameSession(loadedSession))uni.setClipboardData({data:JSON.stringify(draft.value,null,2)})}
/** 同一编辑页全栏目一起保存，独立写代次保护隐藏返回和切号后的新任务。 */
async function save():Promise<boolean>{
  if(!visible||!loaded.value||busy.value||!sameSession(loadedSession))return false
  const sequence=++operationVersion,actor=session();readVersion++;loading.value=false;busy.value=true;error.value=''
  const value=JSON.parse(JSON.stringify(draft.value)) as AdminPublicContent,payload:PublicContentPatch={revision:value.revision,global_slides:value.global_slides,game_slides:value.game_slides,announcement:value.announcement,page_tips:value.page_tips,service_notice:value.service_notice,request_notice:value.request_notice}
  try{const saved=await savePublicContent(payload);if(sequence!==operationVersion||!sameSession(actor))return false;apply(saved);if(visible)notify('已保存，图文进入审核');return true}
  catch(err){if(sequence===operationVersion&&sameSession(actor)){if([401,403].includes(errorStatus(err)||0))clear();error.value=errorStatus(err)===409?'配置已被其他管理员更新，当前草稿已保留。请复制草稿后重新读取核对。':errorMessage(err)}return false}
  finally{if(sequence===operationVersion)busy.value=false}
}
/** 修改当前数组不影响平台顺序或其它游戏。 */
function setSlides(value:ContentSlide[]):void{if(!visible||busy.value||!loaded.value||!sameSession(loadedSession))return;if(selectedGame.value)draft.value.game_slides={...draft.value.game_slides,[selectedGame.value]:value};else draft.value.global_slides=value}
/** 移除仅修改草稿，保存后才改变公开内容。 */
function removeSlide(index:number):void{if(Number.isInteger(index)&&index>=0&&index<slides.value.length)setSlides(slides.value.filter((_,position)=>position!==index))}
/** 全局最多一个游戏槽，普通游戏图片不再嵌套游戏槽。 */
function addGameSlot():void{if(!selectedGame.value&&slides.value.length<12&&!slides.value.some(item=>item.kind==='game_slot'))setSlides([...slides.value,{id:'selected-game',kind:'game_slot',image_url:'',labels:[],target:null}])}
/** 编辑前保存排序，但原生确认只对发起账号和当前游戏生效。 */
async function editSlide(index?:number):Promise<void>{
  if(!visible||busy.value||confirming||!loaded.value||!sameSession(loadedSession)||index===undefined&&slides.value.length>=12)return
  const scope=selectedGame.value,item=index===undefined?null:slides.value[index];if(index!==undefined&&(!Number.isInteger(index)||!item||item.kind!=='image'))return
  const actor=session(),sequence=++dialogVersion,draftAtStart=draft.value,id=item?.id||''
  if(dirty.value){confirming=true;try{const answer=await uni.showModal({title:'先保存当前修改',content:'进入轮播编辑前，需要先保存当前公共信息与排序。',confirmText:'保存并继续'});if(!answer.confirm||!visible||sequence!==dialogVersion||!sameSession(actor)||draft.value!==draftAtStart||scope!==selectedGame.value)return}finally{if(sequence===dialogVersion)confirming=false};if(!await save())return}
  if(!visible||sequence!==dialogVersion||!sameSession(actor)||scope!==selectedGame.value)return
  go('/pages/manage/content/banner?game_id='+encodeURIComponent(scope)+(id?'&id='+encodeURIComponent(id):''))
}
/** 顶部提示只允许文章目标，过期弹层回包不能修改失权后的新配置。 */
function setTipArticle(key:MainTab,target:ContentTarget|null):void{if(!busy.value&&loaded.value&&sameSession(loadedSession))draft.value.page_tips[key].article_id=target?.kind==='article'?target.id:''}
/** 公告使用原比例官方上传；合法同会话隐藏保留草稿，旧finally不能清除新操作锁。 */
async function uploadAnnouncement():Promise<void>{
  if(!visible||busy.value||!loaded.value||!sameSession(loadedSession))return
  const actor=session(),sequence=++operationVersion;busy.value=true
  try{const url=await chooseUploadImage('announcement_image');if(sequence===operationVersion&&sameSession(actor))draft.value.announcement.image_url=url}
  catch(err){if(sequence===operationVersion&&sameSession(actor)&&visible&&!isCancelled(err))notify(err)}
  finally{if(sequence===operationVersion)busy.value=false}
}
/** 隐藏只撤销普通读取和未确认原生弹框，已提交保存与合法选图保留原锁。 */
function hide():void{visible=false;readVersion++;dialogVersion++;confirming=false;loading.value=false}
watch([()=>state.user?.id,()=>state.user?.role,()=>state.user?.platform_admin_role,()=>state.user?.status,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
onLoad(query=>{selectedGame.value=query?.game_id||''})
onShow(()=>{visible=true;if(!dirty.value&&!busy.value)void load()});onHide(hide);onUnload(()=>{disposed=true;hide();clear()})
</script>
<style scoped>
.content-tabs{margin-bottom:30.7692rpx}.scope-switch{display:flex;align-items:center;gap:23.0769rpx;min-width:0}.scope-switch .text-btn{flex-shrink:0}.content-add{margin-top:23.0769rpx}.content-disabled{pointer-events:none;opacity:.6}.content-textarea{height:307.6923rpx}.announcement-editor-image{margin:30.7692rpx 0}.announcement-preview{display:block;width:100%;height:auto;max-height:615.3846rpx;margin-bottom:23.0769rpx;object-fit:contain}.section-row{margin-top:0}.form-section .section-title{margin-bottom:23.0769rpx}
</style>
