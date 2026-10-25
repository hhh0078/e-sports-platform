<!-- 文件作用：四主页面共用的轻量多行顶部提示，只执行已登记文章跳转。 -->
<template><view v-if="tip?.text" class="content-tip" :class="{'content-tip-linked':!!tip.article_id}" @click="open"><Icon name="bell" :size="14"/><text class="content-tip-text">{{tip.text}}</text><Icon v-if="tip.article_id" name="arrow" :size="13"/></view></template>
<script setup lang="ts">
import Icon from './Icon.vue'
import type {ContentTip} from '../types/content'
import {openContentTarget} from '../lib/content'
const props=defineProps<{tip?:ContentTip}>()
/** 配置没有目标时保持纯提示，避免伪造可点路径。 */
function open():void{if(props.tip?.article_id)openContentTarget({kind:'article',id:props.tip.article_id})}
</script>
<style scoped>
.content-tip{display:flex;align-items:center;gap:15.3846rpx;min-width:0;padding:15.3846rpx 23.0769rpx;border-radius:15.3846rpx;background:rgba(131,231,183,.08);color:var(--muted);font-size:23.0769rpx;line-height:34.6154rpx}.content-tip-text{flex:1;min-width:0;white-space:pre-wrap;overflow-wrap:anywhere}.content-tip-linked{color:var(--accent)}
</style>
