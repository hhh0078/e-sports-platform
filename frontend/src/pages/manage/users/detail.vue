<!-- 文件作用：平台用户详情真实页面入口，路由只指定目标ID，权限由服务端再次判断。 -->
<template><AppShell title="用户详情" back><ManagedPersonCenter scope="platform" :target-id="id"/></AppShell></template>
<script setup lang="ts">
import {ref} from 'vue'
import AppShell from '../../../components/AppShell.vue'
import {onLoad} from '@dcloudio/uni-app'
import ManagedPersonCenter from '../../../components/ManagedPersonCenter.vue'
const id=ref('')
/** 只接收公开用户ID作为管理目标，不接受角色、余额或微信身份参数。 */
function applyRoute(options:Record<string,string|undefined>={}):void{id.value=options.id||''}
onLoad(applyRoute)
</script>
