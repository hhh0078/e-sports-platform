<!-- 文件作用：维护单张平台或游戏轮播，独立渲染可编辑文字图层，按修订保存当前数组并保留冲突草稿。 -->
<template>
  <AppShell :title="slideId?'编辑轮播':'新增轮播'" back>
    <view v-if="error" class="error-box banner-error" @click="load">{{error}} · 点击重试</view>
    <EmptyState v-if="state.ready&&!canEdit" title="仅平台管理员可访问" detail="轮播图片与文字仅向平台管理员开放。" icon="shield"/>
    <view v-else-if="loading||!loaded&&!error" class="loading-line">正在读取轮播…</view>
    <template v-else-if="loaded&&canEdit">
      <view class="banner-preview">
        <CoverArt v-if="form.image_url" :src="form.image_url" :labels="previewLabels" :shade="false"/>
        <view v-else class="banner-empty"><Icon name="photo" :size="32"/><text>上传轮播图片</text><text class="banner-empty-note">16:9 · 不超过3MB</text></view>
        <button class="banner-image-action" :disabled="locked" @click="uploadImage"><Icon name="photo" :size="14"/><text>{{uploading?'图片处理中…':form.image_url?'更换图片 · 16:9':'选择图片 · 16:9'}}</text></button>
      </view>
      <text v-if="gameId" class="subtle-note banner-scope">{{gameName}}专属轮播</text>
      <view v-if="conflict" class="banner-conflict"><text class="banner-conflict-copy">{{conflict}}</text><button class="text-btn" :disabled="locked" @click="recheckLatest">{{checking?'核对中…':'核对最新版本'}}</button><view v-if="reviewedSlide" class="banner-latest"><text class="banner-latest-label">服务器当前图片（本地草稿已保留）</text><CoverArt :src="reviewedSlide.image_url" :labels="safeLabels(reviewedSlide.labels,false)" :shade="false"/></view></view>
      <view class="form-section banner-section">
        <view class="banner-section-heading"><text class="section-title">图片文字</text><text class="banner-caption">纯文字 · 最多6条</text></view>
        <view v-for="(label,index) in form.labels" :key="label.id" class="banner-label-editor">
          <view class="banner-label-row"><button class="banner-label-color" :style="{backgroundColor:colorValue(label.color,'#ffffff',false)}" :aria-label="'调整第'+(index+1)+'条文字样式'" :disabled="locked" @click="settingsEditor=settingsEditor===label.id?'':label.id"/><input v-model="label.text" class="input banner-label-input" maxlength="48" :disabled="locked" placeholder="最多12个字" @blur="label.text=normalizeLabelText(label.text)"/><button class="banner-position" :disabled="locked" @click="positionEditor=positionEditor===label.id?'':label.id"><Icon name="location" :size="12"/><text>{{positionName(label.position)}}</text><Icon name="bottom" :size="10"/></button><button class="banner-delete" :disabled="locked" aria-label="删除文字" @click="removeLabel(index)"><Icon name="trash" :size="18"/></button></view>
          <view v-if="positionEditor===label.id" class="banner-position-grid"><button v-for="position in POSITIONS" :key="position[0]" class="banner-position-option" :class="{'banner-position-selected':label.position===position[0]}" :disabled="locked" @click="label.position=position[0];positionEditor=''">{{position[1]}}</button></view>
          <template v-if="settingsEditor===label.id"><view class="banner-label-settings"><view class="banner-font"><text>字号 {{label.font_size||16}}</text><slider class="banner-slider" :value="label.font_size||16" :min="12" :max="24" :step="1" :disabled="locked" :block-size="18" :active-color="accent" @change="label.font_size=$event.detail.value"/></view></view><view class="banner-color-field"><text class="banner-color-label">文字颜色</text><view class="banner-swatches"><button v-for="color in textColors" :key="color" class="banner-swatch" :class="{'banner-swatch-selected':label.color===color}" :style="{backgroundColor:color}" :aria-label="'文字颜色'+color" :disabled="locked" @click="label.color=color"/></view><input v-model="label.color" class="banner-color-code" :disabled="locked" maxlength="7" placeholder="#ffffff"/></view>
          </template>
        </view>
        <button v-if="form.labels.length<6" class="text-btn banner-add" :disabled="locked" @click="addLabel"><Icon name="plus" :size="16"/><text>添加文字</text></button>
      </view>
      <view class="form-section banner-section banner-target-section" :class="{'banner-disabled':locked}"><text class="section-title">点击跳转</text><text class="subtle-note banner-target-hint">选择服务、需求或文章，支持按ID和名称搜索</text><ContentTargetPicker v-model="form.target"/></view>
      <view class="action-footer banner-actions"><button class="btn banner-save" :disabled="locked||missing" @click="save">{{busy?'保存中…':'保存轮播'}}</button><button class="text-btn" :disabled="locked" @click="cancel">取消</button></view>
    </template>
  </AppShell>
</template>
<script setup lang="ts">
import {computed,reactive,ref,watch} from 'vue'
import {onLoad,onShow,onHide,onUnload} from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import EmptyState from '../../../components/EmptyState.vue'
import Icon from '../../../components/Icon.vue'
import CoverArt from '../../../components/CoverArt.vue'
import ContentTargetPicker from '../../../components/ContentTargetPicker.vue'
import {getAdminPublicContent,savePublicContent} from '../../../api/content.api'
import {errorMessage,errorStatus,isCancelled,token} from '../../../api/client'
import {state,init,sessionRevision} from '../../../lib/store'
import {chooseUploadImage} from '../../../lib/media'
import {notify} from '../../../lib/format'
import {back} from '../../../lib/navigation'
import {newLabel,normalizeLabelText,POSITIONS} from '../../../lib/cover'
import type {AdminPublicContent,ContentSlide,PublicContentPatch} from '../../../types/content'
import type {CoverLabel} from '../../../types/models'

const textColors=['#ffffff','#83e4b8','#f4c36c','#ff8a77','#101714']
const form=reactive<ContentSlide>({id:'',kind:'image',image_url:'',labels:[],target:null}),gameId=ref(''),slideId=ref(''),loaded=ref(false),loading=ref(false),busy=ref(false),uploading=ref(false),checking=ref(false),error=ref(''),conflict=ref(''),missing=ref(false),reviewedSlide=ref<ContentSlide|null>(null),positionEditor=ref<string|number>(''),settingsEditor=ref<string|number>('')
const canEdit=computed(()=>state.user?.role==='admin'&&state.user.status!=='frozen'),locked=computed(()=>busy.value||uploading.value||checking.value)
const gameName=computed(()=>state.games.find(item=>item.id===gameId.value)?.name||'当前游戏'),accent=computed(()=>state.games.find(item=>item.id===state.gameId)?.primary_color||'#83e4b8')
const previewLabels=computed(()=>safeLabels(form.labels,false))
let visible=false,disposed=false,readVersion=0,writeVersion=0,uploadVersion=0,baseline:AdminPublicContent|null=null,loadedSession:ReturnType<typeof currentSession>|null=null
/** 捕获具体管理员及其权限修订，迟到数据不能流入另一个账号。 */
function currentSession(){return{id:state.user?.id,role:state.user?.role,adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(),revision:sessionRevision()}}
/** 相同有效管理员才能继续上传和保存；原生选图临时隐藏不会改变账号归属。 */
function sameSession(session:ReturnType<typeof currentSession>|null):boolean{return !!session&&canEdit.value&&session.id===state.user?.id&&session.role===state.user?.role&&session.adminRole===state.user?.platform_admin_role&&session.status===state.user?.status&&session.token===token()&&session.revision===sessionRevision()}
/** 复制独立文字和跳转对象，编辑当前项不修改读取基线或相邻轮播。 */
function cloneSlide(value:ContentSlide):ContentSlide{return{...value,labels:(value.labels||[]).map(label=>({...label})),target:value.target?{...value.target}:null}}
/** 按路由定位唯一数组，游戏编辑不能写到平台数组。 */
function slides(config:AdminPublicContent):ContentSlide[]{return gameId.value?config.game_slides[gameId.value]||[]:config.global_slides}
/** 清除旧账号私有配置、草稿和操作代次，不取消服务器已经收到的保存。 */
function clear():void{readVersion++;writeVersion++;uploadVersion++;Object.assign(form,{id:'',kind:'image',image_url:'',labels:[],target:null});baseline=null;loadedSession=null;loaded.value=false;loading.value=false;busy.value=false;uploading.value=false;checking.value=false;error.value='';conflict.value='';missing.value=false;reviewedSlide.value=null;positionEditor.value='';settingsEditor.value=''}
/** 新图片使用稳定客户端ID，未知保存重试不会向数组重复追加同一张。 */
function newSlideID():string{return 'banner-'+Date.now()+'-'+Math.random().toString(36).slice(2,9)}
/** 只在首次读取或明确重试初始化时回填草稿，原生选图返回不重新读配置。 */
async function load():Promise<void>{if(!visible||disposed||locked.value)return;const current=++readVersion;loading.value=true;error.value='';let session:ReturnType<typeof currentSession>|null=null;try{await init();if(!visible||current!==readVersion)return;if(!canEdit.value){clear();return}session=currentSession();const config=await getAdminPublicContent();if(!visible||current!==readVersion||!sameSession(session))return;const item=slideId.value?slides(config).find(value=>value.id===slideId.value):null;if(slideId.value&&(!item||item.kind!=='image'))throw new Error('轮播图片不存在或已删除，请返回列表重新选择');baseline=config;const draft=item?cloneSlide(item):null;if(draft)draft.labels=draft.labels.map((label,index)=>({...label,id:label.id||'label-'+index}));Object.assign(form,draft||{id:newSlideID(),kind:'image',image_url:'',labels:[],target:null});loaded.value=true;loadedSession=session;conflict.value='';reviewedSlide.value=null;missing.value=false}catch(err){if(visible&&current===readVersion&&(!session||sameSession(session))){if([401,403].includes(errorStatus(err)||0))clear();error.value=errorMessage(err)}}finally{if(current===readVersion)loading.value=false}}
/** 文字颜色仅允许六位十六进制色，原始输入永不直接拼入CSS。 */
function colorValue(value:string|undefined,fallback:string,strict:boolean):string{if(/^#[\da-f]{6}$/i.test(value||''))return value!;if(strict)throw new Error('文字颜色需为#开头的六位色值');return fallback}
/** 沿用封面文字归一化，约束数量、位置和字号；预览只使用校验后的样式值。 */
function safeLabels(labels:CoverLabel[],strict:boolean):CoverLabel[]{if(strict&&labels.length>6)throw new Error('图片文字最多6条');return labels.slice(0,6).map((label,index)=>{const text=normalizeLabelText(label.text);if(strict&&!text)throw new Error('请填写第'+(index+1)+'条图片文字');const position=POSITIONS.some(item=>item[0]===label.position)?label.position:'bottom-left',font=Number(label.font_size);return{id:label.id||'label-'+index,text,color:colorValue(label.color,'#ffffff',strict),background:'transparent',position,font_size:Number.isFinite(font)?Math.max(12,Math.min(24,Math.round(font))):16}})}
/** 添加默认左下文案，已有六条时不再增加。 */
function addLabel():void{if(!locked.value&&form.labels.length<6)form.labels.push({...newLabel(accent.value),background:'transparent'})}
/** 删除明确指定文字，不影响图片或其它图层位置。 */
function removeLabel(index:number):void{if(!locked.value)form.labels.splice(index,1)}
/** 使用九宫格的中文名称展示真实位置。 */
function positionName(position:string|undefined):string{return POSITIONS.find(item=>item[0]===position)?.[1]||'左下'}
/** 按最终16:9用途走官方裁剪压缩；取消与隐藏保留草稿，切号后旧回包作废。 */
async function uploadImage():Promise<void>{if(locked.value||!loaded.value||!sameSession(loadedSession))return;const session=currentSession(),current=++uploadVersion;uploading.value=true;try{const url=await chooseUploadImage('promotion_banner');if(!disposed&&current===uploadVersion&&sameSession(session))form.image_url=url}catch(err){if(!disposed&&current===uploadVersion&&sameSession(session)&&!isCancelled(err))notify(err)}finally{if(current===uploadVersion)uploading.value=false}}
/** 只提交当前数组与修订号，保留所有相邻项以及其它游戏与运营设置。 */
function saveBody():PublicContentPatch{if(!baseline)throw new Error('请先读取轮播配置');if(!form.image_url)throw new Error('请先上传16:9轮播图片');const item:ContentSlide={id:form.id,kind:'image',image_url:form.image_url,labels:safeLabels(form.labels,true),target:form.target?{...form.target}:null},list=slides(baseline).map(cloneSlide),index=list.findIndex(value=>value.id===item.id);if(index<0){if(slideId.value)throw new Error('原轮播已删除，请返回列表核对');if(list.length>=12)throw new Error('每组轮播最多12项，请先移除不需要的图片');list.push(item)}else list[index]=item;return gameId.value?{revision:baseline.revision,game_slides:{[gameId.value]:list}}:{revision:baseline.revision,global_slides:list}}
/** 保存独占请求；版本冲突保留全部草稿，旧账号及卸载后的回包不导航或提示。 */
async function save():Promise<void>{if(locked.value||missing.value||!loaded.value||!sameSession(loadedSession))return;let body:PublicContentPatch;try{body=saveBody()}catch(err){notify(err);return}const session=currentSession(),current=++writeVersion;busy.value=true;try{const config=await savePublicContent(body);if(!disposed&&current===writeVersion&&sameSession(session)){baseline=config;slideId.value=form.id;conflict.value='';if(visible){notify('轮播已保存');back()}}}catch(err){if(!disposed&&current===writeVersion&&sameSession(session)){if(errorStatus(err)===409)conflict.value='配置已由其他管理员更新，本地草稿已保留。请核对最新版本后再次保存。';else if([401,403].includes(errorStatus(err)||0)){clear();error.value=errorMessage(err)}else if(visible)notify(err)}}finally{if(current===writeVersion)busy.value=false}}
/** 明确核对才更新列表基线与版本，保留本条全部草稿；目标被删除时禁止悄悄重新创建。 */
async function recheckLatest():Promise<void>{if(locked.value||!loaded.value||!sameSession(loadedSession))return;const session=currentSession(),current=++readVersion;checking.value=true;try{const config=await getAdminPublicContent();if(!visible||disposed||current!==readVersion||!sameSession(session))return;const item=slides(config).find(value=>value.id===form.id);baseline=config;reviewedSlide.value=item?.kind==='image'?cloneSlide(item):null;missing.value=!!slideId.value&&(!item||item.kind!=='image');conflict.value=missing.value?'原轮播已删除或变更，本地草稿已保留，请返回列表处理。':'已核对最新列表，草稿未替换。再次保存会更新本条，其它轮播保持最新。'}catch(err){if(visible&&current===readVersion&&sameSession(session)){if([401,403].includes(errorStatus(err)||0)){clear();error.value=errorMessage(err)}else conflict.value='核对失败：'+errorMessage(err)+'。本地草稿已保留。'}}finally{if(current===readVersion)checking.value=false}}
/** 取消返回上级列表，不写入服务端。 */
function cancel():void{if(!locked.value)back()}
/** 同一管理员返回保留草稿，上传与保存中不重新请求或释放操作锁。 */
function showPage():void{visible=true;if(loaded.value&&sameSession(loadedSession))return;if(!loading.value)void load()}
/** 隐藏作废普通读取，原生上传和已提交保存继续沿原任务收尾。 */
function hidePage():void{visible=false;readVersion++;loading.value=false;checking.value=false}
watch([()=>state.user?.id,()=>state.user?.role,()=>state.user?.platform_admin_role,()=>state.user?.status,sessionRevision],()=>{clear();if(visible)void load()})
onLoad(options=>{gameId.value=options?.game_id||'';slideId.value=options?.id||''})
onShow(showPage)
onHide(hidePage)
onUnload(()=>{disposed=true;hidePage();clear()})
</script>
<style scoped>
.banner-error{margin-bottom:30.7692rpx}.banner-preview{position:relative;min-width:0;border:1px solid var(--line);border-radius:11.5385rpx;overflow:hidden}.banner-empty{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:19.2308rpx;min-height:373.0769rpx;color:var(--muted);font-size:26.9231rpx;background:var(--surface)}.banner-empty-note{font-size:21.1538rpx}.banner-image-action{position:absolute;right:11.5385rpx;bottom:11.5385rpx;display:flex;align-items:center;gap:11.5385rpx;min-height:61.5385rpx;padding:7.6923rpx 15.3846rpx;border:1px solid rgba(255,255,255,.3);border-radius:9.6154rpx;background:rgba(10,19,17,.8);color:#fff;font-size:21.1538rpx;line-height:1.5}.banner-scope{margin-top:15.3846rpx}.banner-section{padding:38.4615rpx 0}.banner-section-heading{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.banner-caption{font-size:21.1538rpx;color:var(--muted);white-space:nowrap}.banner-label-editor{padding:23.0769rpx 0;border-bottom:1px solid var(--line-subtle);min-width:0}.banner-label-row{display:flex;align-items:center;gap:15.3846rpx;min-width:0}.banner-label-color{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:30.7692rpx;height:30.7692rpx;min-height:30.7692rpx;padding:0;border:1px solid var(--line);border-radius:50%}.banner-label-input{flex:1;min-width:0;font-size:25rpx}.banner-delete{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:46.1538rpx;height:69.2308rpx;min-height:44px;padding:0;background:transparent;color:var(--muted)}.banner-label-settings{display:flex;align-items:center;flex-wrap:wrap;gap:23.0769rpx;margin-top:19.2308rpx;min-width:0}.banner-position{display:flex;align-items:center;justify-content:space-between;gap:11.5385rpx;min-width:153.8462rpx;min-height:69.2308rpx;padding:7.6923rpx 11.5385rpx;background:transparent;color:var(--text);border:1px solid var(--line);border-radius:7.6923rpx;font-size:23.0769rpx}.banner-font{display:flex;align-items:center;gap:15.3846rpx;flex:1;min-width:230.7692rpx;color:var(--muted);font-size:21.1538rpx}.banner-slider{flex:1;margin:0;min-width:76.9231rpx}.banner-position-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:11.5385rpx;margin-top:19.2308rpx}.banner-position-option{display:flex;align-items:center;justify-content:center;min-width:0;min-height:61.5385rpx;padding:7.6923rpx;background:transparent;color:var(--muted);border:1px solid var(--line);border-radius:7.6923rpx;font-size:23.0769rpx}.banner-position-selected{border-color:var(--accent);color:var(--accent)}.banner-color-field{display:flex;align-items:center;flex-wrap:wrap;gap:11.5385rpx;min-width:0;margin-top:19.2308rpx}.banner-color-label{font-size:21.1538rpx;color:var(--muted);min-width:92.3077rpx}.banner-swatches{display:flex;gap:11.5385rpx;min-width:0}.banner-swatch{display:flex;align-items:center;justify-content:center;width:46.1538rpx;height:46.1538rpx;min-height:46.1538rpx;padding:0;border:3.8462rpx solid transparent;border-radius:7.6923rpx}.banner-swatch-selected{border-color:var(--accent);box-shadow:0 0 0 1px var(--bg)}.banner-color-code{width:165.3846rpx;min-width:0;height:53.8462rpx;padding:0 11.5385rpx;margin-left:auto;border:1px solid var(--line);border-radius:7.6923rpx;font-size:21.1538rpx;line-height:1.5;color:var(--muted);font-family:monospace}.banner-add{justify-content:flex-start;gap:11.5385rpx;padding:23.0769rpx 0 0}.banner-target-section{border-bottom:0}.banner-target-hint{margin:15.3846rpx 0}.banner-disabled{pointer-events:none;opacity:.65}.banner-actions{display:flex;flex-direction:column;gap:15.3846rpx;margin-top:15.3846rpx}.banner-save{width:100%}.banner-conflict{padding:23.0769rpx 0;border-bottom:1px solid var(--line);color:#e9b69d}.banner-conflict-copy{display:block;font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.banner-latest{margin-top:15.3846rpx;max-width:480.7692rpx;min-width:0}.banner-latest-label{display:block;margin-bottom:11.5385rpx;font-size:21.1538rpx;line-height:34.6154rpx;color:var(--muted)}
</style>
