<!-- 文件作用：服务或需求详情共用多行注意事项，不将管理员配置压成单行标签。 -->
<template><view v-if="text" class="public-notice"><view class="public-notice-heading"><Icon name="info" :size="17"/><text>{{kind==='service'?'服务注意事项':'需求注意事项'}}</text></view><text class="public-notice-body">{{text}}</text></view></template>
<script setup lang="ts">
import {computed} from 'vue'
import {onShow} from '@dcloudio/uni-app'
import Icon from './Icon.vue'
import {publicContent,refreshPublicContent} from '../lib/public-content'
const props=defineProps<{kind:'service'|'request'}>()
const text=computed(()=>props.kind==='service'?publicContent.value?.service_notice:publicContent.value?.request_notice)
onShow(()=>{void refreshPublicContent().catch(()=>{})})
</script>
<style scoped>
.public-notice{min-width:0;padding:38.4615rpx 0;border-top:1px solid var(--line-subtle)}.public-notice-heading{display:flex;align-items:center;gap:11.5385rpx;font-size:28.8462rpx;font-weight:650;color:var(--text)}.public-notice-body{display:block;margin-top:19.2308rpx;font-size:25rpx;line-height:42.3077rpx;color:var(--muted);white-space:pre-wrap;overflow-wrap:anywhere}
</style>
