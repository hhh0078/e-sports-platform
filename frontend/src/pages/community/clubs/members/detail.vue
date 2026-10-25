<!-- 文件作用：限定俱乐部及成员双重范围的管理详情入口，不提供用户全局钱包。 -->
<template><AppShell title="成员详情" back><ManagedPersonCenter scope="club" :club-id="clubID" :target-id="userID"/></AppShell></template>
<script setup lang="ts">
import {ref} from 'vue'
import AppShell from '../../../../components/AppShell.vue'
import {onLoad} from '@dcloudio/uni-app'
import ManagedPersonCenter from '../../../../components/ManagedPersonCenter.vue'
const clubID=ref(''),userID=ref('')
/** 组织ID与成员ID同时传递，后端严格校验二者关系。 */
function applyRoute(options:Record<string,string|undefined>={}):void{clubID.value=options.club_id||'';userID.value=options.user_id||''}
onLoad(applyRoute)
</script>
