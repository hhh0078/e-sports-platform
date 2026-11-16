<!-- 文件作用：用原生选择器配置单条封面文案字体，不预载未选字体。 -->
<template>
  <picker class="art-font-picker" :range="ART_FONTS" range-key="label" :value="selected" :disabled="disabled" @change="choose">
    <view class="art-font-row" :class="{'art-font-disabled':disabled}"><text class="art-font-label">字体</text><view class="art-font-value"><text class="art-font-name">{{ART_FONTS[selected].label}}</text><Icon name="bottom" :size="12"/></view></view>
  </picker>
</template>
<script setup lang="ts">
import {computed} from 'vue'
import Icon from './Icon.vue'
import {ART_FONTS,artFontID} from '../lib/art-fonts'
const props=defineProps<{modelValue?:string;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[id:string]}>()
const selected=computed(()=>Math.max(0,ART_FONTS.findIndex(font=>font.id===artFontID(props.modelValue))))
/** 只接收原生picker的有效索引，取消和禁用不修改已有字体。 */
function choose(event:{detail:{value:string|number}}):void{const index=Number(event.detail.value);if(!props.disabled&&Number.isInteger(index)&&ART_FONTS[index])emit('update:modelValue',ART_FONTS[index].id)}
</script>
<style scoped>
.art-font-picker{display:block;min-width:0;margin:15.3846rpx 0}.art-font-row{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0;min-height:44px;padding:15.3846rpx 23.0769rpx;background:var(--surface);border:1px solid var(--line-subtle);border-radius:11.5385rpx;box-sizing:border-box}.art-font-label{flex-shrink:0;font-size:25rpx;color:var(--muted)}.art-font-value{display:flex;align-items:center;justify-content:flex-end;gap:15.3846rpx;min-width:0;color:var(--text);font-size:25rpx}.art-font-name{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.art-font-disabled{opacity:.5}
</style>
