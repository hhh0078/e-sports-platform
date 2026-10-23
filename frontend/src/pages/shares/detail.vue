<!-- 文件作用：匿名解析真实小程序码scene，展示安全公开内容并转入白名单目标页面。 -->
<template><AppShell title="分享详情" immersive :game-id="metadata?.game_id"><view v-if="metadata" class="share-landing"><image class="landing-cover" :src="assetUrl(metadata.cover_url)||'/static/share/default-cover.jpg'" mode="aspectFill"/><view class="landing-body"><text class="landing-title">{{ metadata.title }}</text><text class="landing-description">{{ metadata.description }}</text><view class="landing-sharer"><image v-if="metadata.sharer.avatar_url" class="landing-avatar" :src="assetUrl(metadata.sharer.avatar_url)" mode="aspectFill"/><text class="landing-person">{{ metadata.sharer.nickname }} 邀你来集结</text></view><text v-if="timelinePreview" class="subtle-note">请从微信顶部打开小程序，查看完整内容。</text><button v-else class="btn" @click="openTarget">查看详情<Icon name="arrow" :size="15"/></button></view></view><view v-else-if="loading" class="landing-state loading-line">正在读取分享…</view><view v-else class="landing-state error-box stack"><text>{{ error }}</text><button class="btn-secondary" @click="load">重新加载</button></view></AppShell></template>
<script setup lang="ts">
import { ref } from 'vue'
import { onLoad, onUnload, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import { getShare } from '../../api/sharing.api'
import { errorMessage } from '../../api/client'
import { initPublic } from '../../lib/store'
import { assetUrl, go, notify } from '../../lib/format'
import { isTimelinePreview, readShareID, shareTargetPath, enablePublicShareMenu } from '../../lib/sharing'
import { createPublicShareHandlers } from '../../lib/use-public-share'
import type { ShareMetadata } from '../../types/sharing'
const metadata=ref<ShareMetadata|null>(null),loading=ref(true),error=ref(''),timelinePreview=isTimelinePreview()
let id='',version=0,disposed=false
/** 只读公共配置和分享记录，扫码不隐式登录也不读取私有订单。 */
async function load():Promise<void>{const current=++version;loading.value=true;error.value='';try{if(!id)throw new Error('分享不存在或二维码已失效');await initPublic();const result=await getShare(id);shareTargetPath(result);if(!disposed&&current===version)metadata.value=result}catch(err){if(!disposed&&current===version)error.value=errorMessage(err)}finally{if(!disposed&&current===version)loading.value=false}}
/** 经白名单核验后打开公共目标，朋友圈单页不执行导航交易。 */
function openTarget():void{if(timelinePreview)return notify('请先从微信顶部打开小程序');if(metadata.value)go(shareTargetPath(metadata.value))}
const publicShareHandlers=createPublicShareHandlers(()=>({title:metadata.value?.title||'集结',path:metadata.value?shareTargetPath(metadata.value):'/pages/main/index?tab=player',image:metadata.value?.cover_url||'',timelineQuery:id?'share_id='+encodeURIComponent(id):''}))
onShareAppMessage(publicShareHandlers.friend)
onShareTimeline(publicShareHandlers.timeline)
onShow(enablePublicShareMenu)
onLoad(options=>{id=readShareID(options);void load()})
onUnload(()=>{disposed=true;version++})
</script>
<style scoped>.landing-cover{display:block;width:100%;height:260px}.landing-body{padding:20px 16px 24px;min-width:0}.landing-title{display:block;font-size:25px;line-height:34px;font-weight:750;overflow-wrap:anywhere}.landing-description{display:block;margin:16px 0;color:var(--muted);font-size:14px;line-height:24px;white-space:pre-wrap;overflow-wrap:anywhere}.landing-sharer{display:flex;align-items:center;gap:10px;margin:24px 0;min-width:0}.landing-avatar{display:block;flex-shrink:0;width:36px;height:36px;border-radius:50%}.landing-person{font-size:13px;line-height:22px;overflow-wrap:anywhere;min-width:0}.landing-state{margin:100px 16px 24px}</style>
