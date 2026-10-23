<!-- 文件作用：为当前分享人实时绘制海报、好友和朋友圈图片，真实小程序码配置失败时明确保留可分享卡片。 -->
<template><AppShell title="分享" back :game-id="metadata?.game_id"><view v-if="loading" class="loading-line">正在准备分享…</view><view v-else-if="error" class="error-box stack"><text>{{ error }}</text><button class="btn-secondary" @click="load">重新加载</button></view><template v-else-if="metadata"><view class="poster-preview"><image v-if="images.poster" class="poster-image" :src="images.poster" mode="widthFix"/><view v-else class="poster-wait"><image class="poster-cover" :src="assetUrl(metadata.cover_url)||'/static/share/default-cover.jpg'" mode="aspectFill"/><text class="poster-title">{{ metadata.title }}</text><text class="poster-explanation">{{ timelinePreview?metadata.description:posterError||'正在生成真实小程序码与海报…' }}</text></view></view><view v-if="posterError" class="share-warning"><text>{{ posterError }}</text><button class="text-btn" :disabled="drawing" @click="draw">重试生成</button></view><text v-if="timelinePreview" class="subtle-note">请从微信顶部打开小程序查看详情。</text><view v-else class="share-actions"><button class="share-action" open-type="share" :disabled="!images.friend" @click="friend"><Icon name="weixin" :size="23"/><text>微信好友</text></button><button class="share-action" :disabled="!images.poster" @click="timeline"><Icon name="pyq" :size="23"/><text>朋友圈</text></button><button class="share-action" :disabled="!images.poster" @click="save"><Icon name="download" :size="23"/><text>保存海报</text></button></view><text class="share-caption">好友与朋友圈使用专属尺寸封面。扫码可查看公开内容。</text></template><canvas canvas-id="share-poster" id="share-poster" class="share-canvas poster-canvas"/><canvas canvas-id="share-friend" id="share-friend" class="share-canvas friend-canvas"/><canvas canvas-id="share-timeline" id="share-timeline" class="share-canvas timeline-canvas"/></AppShell></template>
<script setup lang="ts">
import { getCurrentInstance, nextTick, ref, watch } from 'vue'
import type { ComponentPublicInstance } from 'vue'
import { onLoad, onUnload, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { createShare, getShare } from '../../api/sharing.api'
import { token, errorMessage } from '../../api/client'
import { init, initPublic, state, sessionRevision, pageTheme } from '../../lib/store'
import { go, notify, assetUrl } from '../../lib/format'
import { isShareKind, isTimelinePreview, shareCreatePath, shareTargetPath, readShareID, showPosterShareMenu, enablePublicShareMenu } from '../../lib/sharing'
import { loadShareBitmap, renderShareCover, renderSharePoster } from '../../lib/share-canvas'
import { createPublicShareHandlers } from '../../lib/use-public-share'
import type { ShareImages, ShareInput, ShareMetadata } from '../../types/sharing'
const timelinePreview=isTimelinePreview(),instance=getCurrentInstance(),metadata=ref<ShareMetadata|null>(null),images=ref<ShareImages>({poster:'',friend:'',timeline:''}),loading=ref(true),drawing=ref(false),error=ref(''),posterError=ref('')
let input:ShareInput|null=null,sharedID='',version=0,disposed=false,identityReady=false
/** 把会话快照与生命周期一起比较，切号后丢弃旧海报和分享人。 */
function identity(){return {id:state.user?.id,token:token(),revision:sessionRevision()}}
/** 只有当前账号及页面代次一致才允许更新实时图片。 */
function current(snapshot:ReturnType<typeof identity>,revision:number):boolean{return !disposed&&version===revision&&snapshot.id===state.user?.id&&snapshot.token===token()&&snapshot.revision===sessionRevision()}
/** 创建分享记录，不接受路由中伪造的分享人头像和昵称。 */
async function load():Promise<void>{const revision=++version;loading.value=true;error.value='';metadata.value=null;images.value={poster:'',friend:'',timeline:''};try{if(sharedID||timelinePreview){if(!sharedID)throw new Error('分享不存在');await initPublic();const result=await getShare(sharedID);const target=shareTargetPath(result);if(!disposed&&revision===version){metadata.value=result;if(!timelinePreview)go(target)}return}await init();if(disposed||revision!==version)return;if(!input)throw new Error('分享内容不存在');if(!state.user){loading.value=false;go('/pages/account/login?next='+encodeURIComponent(shareCreatePath(input)));return}identityReady=true;const snapshot=identity(),result=await createShare(input);if(!current(snapshot,revision))return;shareTargetPath(result);metadata.value=result;await nextTick();await draw()}catch(err){if(!disposed&&revision===version)error.value=errorMessage(err)}finally{if(!disposed&&revision===version)loading.value=false}}
/** 三种图片逐一绘制；二维码失败不伪造海报，好友封面仍可单独使用。 */
async function draw():Promise<void>{if(drawing.value||!metadata.value||!instance?.proxy)return;const revision=version,snapshot=identity(),data=metadata.value,owner=instance.proxy as ComponentPublicInstance;drawing.value=true;posterError.value='';try{const cover=await loadShareBitmap(data.cover_url||'/static/share/default-cover.jpg').catch(()=>loadShareBitmap('/static/share/default-cover.jpg'));if(!current(snapshot,revision))return;const accent=pageTheme(data.game_id)['--accent'];const friend=await renderShareCover(data,cover,'share-friend',owner,false,accent);if(!current(snapshot,revision))return;images.value.friend=friend;const timeline=await renderShareCover(data,cover,'share-timeline',owner,true,accent);if(!current(snapshot,revision))return;images.value.timeline=timeline;const [code,avatar]=await Promise.all([loadShareBitmap(data.code_url),loadShareBitmap(data.sharer.avatar_url).catch(()=>null)]);if(!current(snapshot,revision))return;const path=await renderSharePoster(data,cover,avatar,code,'share-poster',owner,accent);if(current(snapshot,revision))images.value.poster=path}catch(err){if(current(snapshot,revision))posterError.value=errorMessage(err)}finally{if(!disposed&&revision===version)drawing.value=false}}
/** H5复制同一公开链接，微信端由原生open-type分享，不自行发送消息。 */
function friend():void{
  // #ifdef H5
  if(metadata.value)uni.setClipboardData({data:location.origin+'/#'+shareTargetPath(metadata.value),success:()=>notify('分享链接已复制')})
  // #endif
}
/** 使用微信官方图片菜单分享到朋友圈，不把系统菜单打开当作已发送。 */
async function timeline():Promise<void>{if(!images.value.poster)return;try{await showPosterShareMenu(images.value.poster,metadata.value?shareTargetPath(metadata.value).replace(/^\//,''):undefined)}catch(err){notify(err)}}
/** 保存当前实时海报，拒绝相册权限时给出具体恢复路径。 */
function save():void{if(!images.value.poster)return;
  // #ifdef H5
  const link=document.createElement('a');link.href=images.value.poster;link.download='集结分享海报.png';link.click();return;
  // #endif
  uni.saveImageToPhotosAlbum({filePath:images.value.poster,success:()=>notify('海报已保存，可从相册分享到朋友圈'),fail:()=>notify('未保存海报，请在微信设置中允许保存到相册后重试')})}
/** 只读取业务目标，未知类型及缺失目标都在调用接口前拒绝。 */
function applyRoute(options:Record<string,string|undefined>={}):void{sharedID=readShareID(options);input=isShareKind(options.kind)?{kind:options.kind,target_id:options.target_id||'',game_id:options.game_id}:null;if(input&&input.kind!=='home'&&!input.target_id)input=null;void load()}
const publicShareHandlers=createPublicShareHandlers(()=>({title:metadata.value?.title||'集结 · 和好队友一起游戏',path:metadata.value?shareTargetPath(metadata.value):'/pages/main/index?tab=player',image:metadata.value?.cover_url||'',friendImage:images.value.friend,timelineImage:images.value.timeline,timelineQuery:metadata.value?'share_id='+encodeURIComponent(metadata.value.id):''}))
onShareAppMessage(publicShareHandlers.friend)
onShareTimeline(publicShareHandlers.timeline)
onShow(enablePublicShareMenu)
watch([()=>state.user?.id,sessionRevision],()=>{if(!identityReady)return;version++;metadata.value=null;images.value={poster:'',friend:'',timeline:''};drawing.value=false;if(!disposed){loading.value=false;error.value='登录身份已变化，请重新生成分享'}})
onLoad(applyRoute)
onUnload(()=>{disposed=true;version++;images.value={poster:'',friend:'',timeline:''}})
</script>
<style scoped>.poster-preview{width:100%;max-width:750rpx;margin:0 auto;border:1px solid var(--line);border-radius:23.0769rpx;overflow:hidden;background:#0a1510}.poster-image{display:block;width:100%}.poster-wait{padding-bottom:46.1538rpx}.poster-cover{display:block;width:100%;height:461.5385rpx}.poster-title{display:block;padding:38.4615rpx 38.4615rpx 15.3846rpx;font-size:38.4615rpx;line-height:57.6923rpx;font-weight:700;overflow-wrap:anywhere}.poster-explanation{display:block;padding:15.3846rpx 38.4615rpx;color:var(--muted);font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}.share-actions{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:19.2308rpx;margin-top:30.7692rpx}.share-action{display:flex;flex-direction:column;align-items:center;justify-content:center;gap:15.3846rpx;min-height:146.1538rpx;padding:19.2308rpx 7.6923rpx;border:1px solid var(--line);border-radius:19.2308rpx;background:var(--surface);color:var(--accent);font-size:23.0769rpx;line-height:34.6154rpx;min-width:0}.share-caption{display:block;margin-top:30.7692rpx;text-align:center;color:var(--muted);font-size:21.1538rpx;line-height:38.4615rpx}.share-warning{display:flex;flex-direction:column;align-items:flex-start;gap:7.6923rpx;margin-top:23.0769rpx;color:#dbbb80;font-size:23.0769rpx;line-height:38.4615rpx;overflow-wrap:anywhere}.share-canvas{position:fixed;left:-10000px;top:-10000px;pointer-events:none}/* 离屏位图与绘制器坐标一致，保持实际像素，页面预览外框另用响应式尺寸。 */.poster-canvas{width:750px;height:1120px}.friend-canvas{width:500px;height:400px}.timeline-canvas{width:500px;height:500px}</style>
