<!-- 文件作用：用当前文案直接展示已选字体和可见候选字效，按需加载字形并提示失败重试。 -->
<template>
  <view class="art-font-picker">
    <button class="art-font-trigger" :disabled="disabled" :aria-expanded="opened" @click="opened=!opened">
      <text class="art-font-label">字体</text><view class="art-font-value"><text class="art-font-current" :style="fonts.style(selectedLabel)">{{sample}}</text><text class="art-font-name">{{selected.label}}</text><Icon name="bottom" :size="11"/></view>
    </button>
    <view v-if="opened" class="art-font-panel">
      <view class="art-font-options"><button v-for="font in ART_FONTS" :key="font.id" class="art-font-option" :class="{'art-font-selected':selected.id===font.id}" :disabled="disabled" :aria-label="font.label" :aria-pressed="selected.id===font.id" @click="choose(font.id)"><view class="art-font-option-heading"><text class="art-font-option-name">{{font.label}}</text><Icon v-if="selected.id===font.id" name="check" :size="12"/></view><text class="art-font-sample" :style="fonts.style({text:sample,font_family:font.id})">{{sample}}</text><text v-if="fonts.failed.value.includes(font.id)" class="art-font-status">字效未载入</text></button></view>
      <text v-if="fonts.loading.value" class="art-font-status">正在加载字体效果…</text><button v-if="fonts.failed.value.length" class="text-btn art-font-retry" @click="retry">部分字效未载入，点击重试</button>
    </view>
  </view>
</template>
<script setup lang="ts">
import {computed,ref,watch,nextTick} from 'vue'
import {onHide,onShow} from '@dcloudio/uni-app'
import Icon from './Icon.vue'
import {ART_FONTS,artFontID} from '../lib/art-fonts'
import {useArtFonts} from '../lib/use-art-fonts'
import {normalizeLabelText} from '../lib/cover'
import {sessionRevision} from '../lib/store'
const props=defineProps<{modelValue?:string;previewText?:string;disabled?:boolean}>()
const emit=defineEmits<{'update:modelValue':[id:string]}>()
const opened=ref(false),active=ref(true)
const sample=computed(()=>normalizeLabelText(props.previewText?.trim()||'文字效果'))
const selected=computed(()=>ART_FONTS.find(font=>font.id===artFontID(props.modelValue))||ART_FONTS[0])
const selectedLabel=computed(()=>({text:sample.value,font_family:selected.value.id}))
const fonts=useArtFonts(()=>opened.value?ART_FONTS.map(font=>({text:sample.value,font_family:font.id})):[selectedLabel.value],()=>active.value)
/** 仅有效候选可回填，选择后收起面板。 */
function choose(id:string):void{if(props.disabled||!active.value||!opened.value)return;emit('update:modelValue',artFontID(id));opened.value=false}
/** 显式重试可见字形，成功的字体继续复用已有加载缓存。 */
async function retry():Promise<void>{active.value=false;await nextTick();active.value=true}
watch([sessionRevision,()=>props.disabled],()=>{opened.value=false},{flush:'sync'})
onHide(()=>{opened.value=false;active.value=false})
onShow(()=>{active.value=true})
</script>
<style scoped>
.art-font-picker{display:block;min-width:0;margin:15.3846rpx 0}.art-font-trigger{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;width:100%;min-width:0;min-height:44px;padding:0;background:transparent;color:var(--text);text-align:left}.art-font-label{flex-shrink:0;font-size:23.0769rpx;color:var(--muted)}.art-font-value{display:flex;align-items:center;justify-content:flex-end;flex-wrap:wrap;gap:7.6923rpx 15.3846rpx;min-width:0}.art-font-current{max-width:100%;min-width:0;font-size:30.7692rpx;line-height:1.6;overflow-wrap:anywhere}.art-font-name{font-size:19.2308rpx;color:var(--muted)}.art-font-panel{padding:15.3846rpx 0}.art-font-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:15.3846rpx}.art-font-option{display:flex;flex-direction:column;gap:15.3846rpx;min-width:0;min-height:44px;padding:15.3846rpx;background:var(--bg);color:var(--text);border:1px solid var(--line);border-radius:15.3846rpx;text-align:left}.art-font-selected{border-color:var(--accent)}.art-font-option-heading{display:flex;align-items:center;justify-content:space-between;gap:7.6923rpx;width:100%;color:var(--accent)}.art-font-option-name{min-width:0;color:var(--muted);font-size:23.0769rpx;overflow-wrap:anywhere}.art-font-sample{max-width:100%;font-size:34.6154rpx;line-height:1.6;font-weight:400;overflow-wrap:anywhere}.art-font-status{display:block;font-size:21.1538rpx;line-height:1.7;color:var(--muted);padding-top:15.3846rpx}.art-font-retry{min-height:44px;white-space:normal;text-align:left}
</style>
