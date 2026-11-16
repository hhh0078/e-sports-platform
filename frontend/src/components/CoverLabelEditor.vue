<!-- 文件作用：独立编辑一条封面文字，收起颜色选项并直接显示字体效果。 -->
<template>
  <view class="label-editor">
    <view class="label-heading"><text class="label-index">文字 {{ index+1 }}</text><button class="text-btn label-delete" :disabled="disabled" @click="emit('remove')">删除</button></view>
    <input v-model="label.text" class="input label-input" :disabled="disabled" maxlength="48" :aria-label="'封面文字'+(index+1)" placeholder="最多12个字，表情完整保留" @blur="label.text=normalizeLabelText(label.text)"/>
    <ArtFontPicker v-model="label.font_family" :preview-text="label.text" :disabled="disabled"/>
    <view class="color-controls">
      <button v-for="field in colorFields" :key="field.key" class="color-trigger" :disabled="disabled" :aria-expanded="colorEditor===field.key" @click="colorEditor=colorEditor===field.key?'':field.key">
        <text class="control-label">{{ field.title }}</text><view class="color-current" :class="{'color-transparent':label[field.key]==='transparent'}" :style="{backgroundColor:safeColor(label[field.key])}"/><Icon name="bottom" :size="11"/>
      </button>
    </view>
    <view v-if="colorEditor" class="color-panel">
      <view class="palette-heading"><text class="control-label">{{colorEditor==='color'?'文字颜色':'背景颜色'}}</text><button class="text-btn palette-done" @click="closeColor">完成</button></view>
      <view class="color-swatches"><button v-for="color in colorEditor==='color'?textColors:backgroundColors" :key="color" class="color-choice" :class="{'color-selected':label[colorEditor]===color,'color-transparent':color==='transparent'}" :style="{backgroundColor:safeColor(color)}" :aria-label="color==='transparent'?'透明背景':color" @click="chooseColor(color)"><Icon v-if="label[colorEditor]===color" name="check" :size="16" :style="{color:color==='#ffffff'?'#101714':'#ffffff'}"/></button></view>
      <view class="custom-color"><text class="control-label">自定义色值</text><input v-model="label[colorEditor]" class="color-code" :maxlength="colorEditor==='background'?11:7" aria-label="自定义颜色" placeholder="#ffffff"/></view>
    </view>
    <button class="position-trigger" :disabled="disabled" :aria-expanded="positionEditor" @click="positionEditor=!positionEditor"><text class="control-label">位置</text><view class="position-value">{{ positionName }}<Icon name="bottom" :size="11"/></view></button>
    <view v-if="positionEditor" class="position-grid"><button v-for="position in POSITIONS" :key="position[0]" class="position-choice" :class="{'position-selected':label.position===position[0]}" @click="label.position=position[0];positionEditor=false">{{ position[1] }}</button></view>
    <view class="font-size-control"><text class="control-label">字号 {{ label.font_size }}</text><slider class="font-size-slider" :disabled="disabled" :value="label.font_size" :min="12" :max="24" :step="1" :activeColor="accent" :block-size="sliderBlockSize" @change="label.font_size=$event.detail.value"/></view>
  </view>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue'
import {onHide} from '@dcloudio/uni-app'
import ArtFontPicker from './ArtFontPicker.vue'
import Icon from './Icon.vue'
import {normalizeLabelText,POSITIONS} from '../lib/cover'
import {notify} from '../lib/format'
import type {CoverLabel} from '../types/models'
const props=defineProps<{label:CoverLabel;index:number;disabled?:boolean;accent?:string;sliderBlockSize:number}>()
const emit=defineEmits<{remove:[]}>()
const colorEditor=ref<'color'|'background'|''>(''),positionEditor=ref(false)
const colorFields=[{key:'color',title:'文字颜色'},{key:'background',title:'背景颜色'}] as const
const textColors=['#ffffff','#00e18a','#f4c36c','#ff6b6b','#101714'],backgroundColors=['transparent','#101614','#00e18a','#b98132','#a43c3c']
const positionName=computed(()=>POSITIONS.find(position=>position[0]===props.label.position)?.[1]||'左下')
/** 色值输入未完成时使用透明占位，不把无效字符串传给原生样式。 */
function safeColor(value?:string):string{return /^#[0-9a-f]{6}$/i.test(value||'')?value!:'transparent'}
/** 常用色选择立即回填当前文字并收起色板。 */
function chooseColor(color:string):void{if(!props.disabled&&colorEditor.value){props.label[colorEditor.value]=color;colorEditor.value=''}}
/** 自定义色值有效后收起，输入错误保留现场供修改。 */
function closeColor():void{const field=colorEditor.value;if(!field)return;const value=props.label[field];if(!/^#[0-9a-f]{6}$/i.test(value||'')&&!(field==='background'&&value==='transparent'))return notify('请输入六位颜色值，例如 #ffffff');colorEditor.value=''}
onHide(()=>{colorEditor.value='';positionEditor.value=false})
</script>
<style scoped>
.label-editor{min-width:0;margin-top:30.7692rpx;padding:23.0769rpx;border:1px solid var(--line);border-radius:19.2308rpx;background:#ffffff03}
.label-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx}.label-index{font-size:26.9231rpx;font-weight:650}.label-delete{min-height:44px;color:var(--muted);padding:0 0 0 23.0769rpx}.label-input{font-size:30.7692rpx;margin:0 0 15.3846rpx}.control-label{color:var(--muted);font-size:23.0769rpx;flex-shrink:0}
.color-controls{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:15.3846rpx}.color-trigger{display:flex;align-items:center;justify-content:space-between;gap:7.6923rpx;min-width:0;min-height:44px;padding:0;background:transparent;color:var(--muted)}.color-current{flex-shrink:0;width:38.4615rpx;height:38.4615rpx;border:1px solid var(--line);border-radius:9.6154rpx}.color-transparent{background-image:repeating-linear-gradient(45deg,#ffffff18 0 7.6923rpx,transparent 7.6923rpx 15.3846rpx)}.color-panel{margin:15.3846rpx 0;padding:15.3846rpx;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--bg)}.palette-heading{display:flex;align-items:center;justify-content:space-between}.palette-done{min-height:44px}.color-swatches{display:flex;flex-wrap:wrap;gap:15.3846rpx}.color-choice{display:flex;align-items:center;justify-content:center;min-width:44px;min-height:44px;width:69.2308rpx;height:69.2308rpx;padding:0;border:1px solid var(--line);border-radius:11.5385rpx}.color-selected{outline:3.8462rpx solid var(--accent);outline-offset:3.8462rpx}.custom-color{display:flex;align-items:center;gap:15.3846rpx;margin-top:23.0769rpx;min-width:0}.color-code{flex:1;min-width:0;width:0;height:84.6154rpx;min-height:44px;padding:0 15.3846rpx;border:1px solid var(--line);border-radius:11.5385rpx;color:var(--text);font-family:monospace;font-size:25rpx}
.position-trigger{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;width:100%;min-height:44px;padding:0;background:transparent;color:var(--text);font-size:25rpx}.position-value{display:flex;align-items:center;gap:15.3846rpx}.position-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;padding:15.3846rpx 0}.position-choice{display:flex;align-items:center;justify-content:center;min-width:0;min-height:44px;padding:7.6923rpx;background:transparent;border:1px solid var(--line);font-size:23.0769rpx;color:var(--muted)}.position-selected{border-color:var(--accent);color:var(--accent)}.font-size-control{display:flex;align-items:center;gap:23.0769rpx;min-height:44px}.font-size-slider{flex:1;min-width:0;margin:0}
</style>
