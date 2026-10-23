<!-- 文件作用：向所有访客展示平台当前用户协议或隐私政策正文及版本，不执行自动同意。 -->
<template><AppShell :title="document?.title||fallbackTitle" back><view v-if="loading" class="loading-line">正在读取协议…</view><view v-else-if="error" class="error-box" @click="load">{{ error }} · 点击重试</view><view v-else-if="document" class="agreement-document"><view class="document-meta"><text>版本 {{ document.version }}</text><text>{{ document.updated_at ? time(document.updated_at) : '' }}</text></view><text class="document-body" user-select>{{ document.content }}</text></view></AppShell></template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import { getAgreements } from '../../api/auth.api'
import type { Agreement, Agreements } from '../../types/models'
import { errorMessage } from '../../api/client'
import { time } from '../../lib/format'
const type=ref<keyof Agreements>('user_agreement'),document=ref<Agreement | null>(null),loading=ref(true),error=ref('')
const fallbackTitle=computed(()=>type.value==='privacy_policy'?'隐私政策':'用户协议')
/** 读取服务端当前版本，正文按纯文本显示，禁止执行后台输入的HTML。 */
async function load(){loading.value=true;error.value='';try{const data=await getAgreements();document.value=data[type.value]||null;if(!document.value)throw new Error('协议暂不可用')}catch(err){error.value=errorMessage(err)}finally{loading.value=false}}
/** 仅允许两种已知文档，非法查询回到用户协议。 */
function applyRoute(options: Record<string,string | undefined> = {}){type.value=options.type==='privacy_policy'?'privacy_policy':'user_agreement';load()}
onLoad(applyRoute)
</script>
<style scoped>.agreement-document{padding:15.3846rpx 0}.document-meta{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:15.3846rpx;color:var(--muted);font-size:21.1538rpx;border-bottom:1px solid var(--line-subtle);padding-bottom:30.7692rpx;margin-bottom:46.1538rpx}.document-body{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:28.8462rpx;line-height:1.9;color:#cbd7cf}</style>
