<!-- 文件作用：显示服务端当前审核投影生成的安全Markdown富文本，正文换行和图片尺寸由安全渲染器处理。 -->
<template><view class="article-body"><rich-text class="article-rich-text" :nodes="nodes" :user-select="true"/></view></template>
<script setup lang="ts">
import {computed} from 'vue'
import {assetUrl} from '../lib/format'
import {presentArticleHTML} from '../lib/article-presentation'
const props=defineProps<{html:string}>()
const nodes=computed(()=>presentArticleHTML(props.html,assetUrl))
</script>
<style scoped>
.article-body{min-width:0;max-width:100%;font-size:28.8462rpx;line-height:1.9;color:var(--text);overflow-wrap:anywhere}.article-rich-text{display:block;min-width:0;max-width:100%;overflow-wrap:anywhere}
</style>
