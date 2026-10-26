<!-- 文件作用：复用官方资金弹层的根层、主题及关闭保护，为服务和需求展示同一恢复草稿选择。 -->
<template><FinancePopup :opened="opened" title="发现未发布的草稿" @close="emit('dismiss',contextID)"><view class="draft-dialog"><view class="draft-dialog-icon"><Icon name="edit" :size="24"/></view><text class="draft-dialog-copy">上次填写的内容已保存在本机，是否继续编辑？</text><view class="draft-dialog-actions"><button class="draft-dialog-discard" @click="emit('discard',contextID)">不用并删除</button><button class="draft-dialog-restore" @click="emit('restore',contextID)">恢复草稿</button></view></view></FinancePopup></template>
<script setup lang="ts">
import FinancePopup from './FinancePopup.vue'
import Icon from './Icon.vue'
const props=defineProps<{opened:boolean;context:number}>()
// 父级按上下文key重建；旧官方关闭动画始终携带其原ID，不能关闭新账号弹层。
const contextID=props.context
const emit=defineEmits<{restore:[context:number];discard:[context:number];dismiss:[context:number]}>()
</script>
<style scoped>
.draft-dialog{min-width:0}.draft-dialog-icon{display:flex;align-items:center;justify-content:center;width:76.9231rpx;height:76.9231rpx;margin:0 auto 23.0769rpx;border-radius:50%;background:rgba(255,255,255,.05);color:var(--muted,#9aada2)}.draft-dialog-copy{display:block;color:var(--muted,#9aada2);font-size:25rpx;line-height:42.3077rpx;text-align:center;white-space:pre-wrap;overflow-wrap:anywhere}.draft-dialog-actions{display:flex;align-items:center;gap:23.0769rpx;margin-top:30.7692rpx}.draft-dialog-discard,.draft-dialog-restore{display:flex;align-items:center;justify-content:center;flex:1;min-width:0;min-height:44px;padding:15.3846rpx 11.5385rpx;border-radius:15.3846rpx;font-size:25rpx;line-height:38.4615rpx}.draft-dialog-discard{border:1px solid var(--line,rgba(255,255,255,.12));background:transparent;color:var(--muted,#9aada2)}.draft-dialog-restore{border:1px solid var(--accent,#83e7b7);background:var(--accent,#83e7b7);color:#0a2116;font-weight:650}
</style>
