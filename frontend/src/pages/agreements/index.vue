<!-- 文件作用：向所有访客展示平台当前用户协议或隐私政策正文及版本，不执行自动同意。 -->
<template><AppShell :title="document?.title||fallbackTitle" subtitle="请认真阅读，了解你的权利和平台规则" back><view v-if="loading" class="loading-line">正在读取协议…</view><view v-else-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view><view v-else-if="document" class="agreement-document panel"><view class="document-meta"><text>版本 {{ document.version }}</text><text>{{ document.updated_at ? time(document.updated_at) : '' }}</text></view><text class="document-body" selectable>{{ document.content }}</text></view></AppShell></template>
<script setup>
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import { api } from '../../lib/api'
import { time } from '../../lib/format'
const type=ref('user_agreement'),document=ref(null),loading=ref(true),error=ref('')
const fallbackTitle=computed(()=>type.value==='privacy_policy'?'隐私政策':'用户协议')
/** 读取服务端当前版本，正文按纯文本显示，禁止执行后台输入的HTML。 */
async function load(){loading.value=true;error.value='';try{const data=await api('/agreements');document.value=data[type.value]||null;if(!document.value)throw new Error('协议暂不可用')}catch(err){error.value=err.message}finally{loading.value=false}}
/** 仅允许两种已知文档，非法查询回到用户协议。 */
function applyRoute(options){type.value=options.type==='privacy_policy'?'privacy_policy':'user_agreement';load()}
onLoad(applyRoute)
</script>
<style scoped>.agreement-document{padding:19px}.document-meta{display:flex;align-items:center;justify-content:space-between;gap:10px;color:#8fa297;font-size:11px;border-bottom:1px solid #ffffff14;padding-bottom:15px;margin-bottom:19px}.document-body{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:14px;line-height:1.9;color:#cbd7cf}</style>
