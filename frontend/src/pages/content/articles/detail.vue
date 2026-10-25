<!-- 文件作用：访客读取当前已发布文章的安全正文，隐藏或失效时撤下旧内容。 -->
<template><AppShell title="文章详情" back><view v-if="error" class="error-box" @click="load">{{error}} · 点击重试</view><view v-else-if="!article" class="loading-line">正在读取文章…</view><view v-else class="article-reader"><text class="reader-title">{{article.title}}</text><text class="reader-date">{{time(article.published_at)}} 发布</text><image v-if="article.cover_url" class="reader-cover" :src="assetUrl(article.cover_url)" mode="aspectFill"/><ArticleBody :html="article.body_html"/></view></AppShell></template>
<script setup lang="ts">
import {ref} from 'vue'
import {onLoad,onShow,onHide,onUnload} from '@dcloudio/uni-app'
import AppShell from '../../../components/AppShell.vue'
import ArticleBody from '../../../components/ArticleBody.vue'
import {getArticle} from '../../../api/content.api'
import {errorMessage} from '../../../api/client'
import {assetUrl,time} from '../../../lib/format'
import {useNativePageRefresh} from '../../../lib/native-page-refresh'
import type {PublicArticle} from '../../../types/content'
const id=ref(''),article=ref<PublicArticle|null>(null),error=ref('');let visible=true,version=0
/** 每次显示都重新核对公开状态，旧请求及下架正文不可继续展示。 */
async function load():Promise<void>{if(!id.value||!visible)return;const current=++version;error.value='';try{const value=await getArticle(id.value);if(visible&&current===version)article.value=value}catch(err){if(visible&&current===version){article.value=null;error.value=errorMessage(err)}}}
/** 隐藏后作废未完成的文章读取。 */
function hide():void{visible=false;version++}
onLoad(query=>{id.value=query?.id||'';if(!id.value)error.value='缺少文章编号'})
onShow(()=>{visible=true;void load()});onHide(hide);onUnload(hide)
useNativePageRefresh({refresh:load,context:()=>id.value})
</script>
<style scoped>
.article-reader{min-width:0;padding:15.3846rpx 0 46.1538rpx}.reader-title{display:block;font-size:44.2308rpx;line-height:61.5385rpx;font-weight:750;white-space:pre-wrap;overflow-wrap:anywhere}.reader-date{display:block;margin:23.0769rpx 0 38.4615rpx;color:var(--muted);font-size:23.0769rpx}.reader-cover{display:block;width:100%;height:384.6154rpx;border-radius:19.2308rpx;margin-bottom:38.4615rpx}
</style>
