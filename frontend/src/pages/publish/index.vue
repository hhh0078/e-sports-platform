<!-- 文件作用：发布与编辑游戏服务，配置游戏封面、文字样式和俱乐部挂靠。 -->
<template>
  <AppShell :title="serviceId ? '编辑服务' : '发布你的拿手好戏'" subtitle="让真正合拍的老板找到你" back>
    <view v-if="error" class="error-box mb">{{ error }}</view>
    <template>
      <view class="editor-intro"><text class="eyebrow">CREATE YOUR CARD</text><text class="muted">选张喜欢的底图，写上你的拿手好戏。</text></view>
      <view class="preview"><view class="preview-toolbar"><text>封面预览</text><button v-if="covers.length>1" class="text-btn" @click="changeCover">换一张底图</button></view><CoverArt :src="form.cover_url" :labels="form.labels"/><view class="preview-caption"><text>{{ form.title || '你的服务标题' }}</text><text class="accent">¥{{ form.price || '0' }} / {{ form.unit }}</text></view></view>
      <view class="panel form-panel"><text class="section-title">封面工作台</text><text class="hint block">换底图，文字还在。文字默认从左下依次排列，也可以自由调色、选位置，自动错开不重叠。</text>
        <view class="field"><text class="field-label">选择游戏</text><picker :range="state.games" range-key="name" :value="gameIndex" @change="changeGame"><view class="input picker-value">{{ selectedGame?.name || '选择游戏' }}<Icon name="arrow" :size="14"/></view></picker></view>
        <text class="field-label">游戏底图 · 也可以自己选</text>
        <view class="cover-options"><view v-for="src in covers" :key="src" :class="['cover-option',{chosen:form.cover_url===src}]" @click="form.cover_url=src"><image :src="assetUrl(src)" mode="aspectFill"/><view v-if="form.cover_url===src" class="cover-check"><Icon name="check" :size="12"/></view></view></view>
        <view class="section-row"><text class="field-label">叠加文字 {{ form.labels.length }} / 6</text><button class="text-btn" :disabled="form.labels.length>=6" @click="addLabel">＋ 添加文字</button></view>
        <view v-if="!form.labels.length" class="notice">也可以保持纯图片封面。添加文字可展示段位、服务特色或限时活动。</view>
        <view v-for="(label,index) in form.labels" :key="label.id" class="label-editor"><view class="row between"><text class="label-index">文案 {{ index+1 }}</text><button class="text-btn danger" @click="form.labels.splice(index,1)">删除</button></view><input v-model="label.text" class="input" maxlength="48" placeholder="最多12个字，表情完整保留" @blur="label.text=normalizeLabelText(label.text)"/><view class="colors"><view class="color-field"><text>文字颜色</text><view class="swatches"><button v-for="color in textColors" :key="color" :style="{background:color}" :class="{chosen:label.color===color}" :aria-label="'文字颜色'+color" @click="label.color=color"/></view><input v-model="label.color" class="color-code" maxlength="7" placeholder="#ffffff"/></view><view class="color-field"><text>背景颜色</text><view class="swatches"><button v-for="color in backgroundColors" :key="color" :style="{background:color}" :class="{chosen:label.background===color,transparent:color==='transparent'}" :aria-label="'背景颜色'+color" @click="label.background=color"/></view><input v-model="label.background" class="color-code" maxlength="11" placeholder="#000000"/></view></view><view class="label-bottom"><button class="position-picker" @click="positionEditor=positionEditor===label.id?'':label.id">位置 · {{ positionName(label.position) }} <Icon name="bottom" :size="11"/></button><view class="font-picker"><text>字号 {{ label.font_size }}</text><slider :value="label.font_size" :min="12" :max="24" :step="1" :activeColor="selectedGame?.primary_color" block-size="16" @change="label.font_size=$event.detail.value"/></view></view><view v-if="positionEditor===label.id" class="position-grid"><button v-for="position in POSITIONS" :key="position[0]" :class="{selected:label.position===position[0]}" @click="label.position=position[0];positionEditor=''">{{ position[1] }}</button></view></view>
      </view>
      <view class="panel form-panel"><text class="section-title">服务信息</text><view class="field"><text class="field-label">服务标题</text><input v-model="form.title" class="input" maxlength="40" placeholder="例如：三角洲双人护航 · 耐心沟通"/></view><view class="field"><text class="field-label">服务类型</text><picker :range="selectedGame?.categories || []" :value="categoryIndex" @change="form.category=selectedGame.categories[Number($event.detail.value)]"><view class="input picker-value">{{ form.category || '请选择' }}<Icon name="arrow" :size="14"/></view></picker></view><view class="price-grid"><view class="field"><text class="field-label">价格（元）</text><input v-model="form.price" type="digit" class="input" placeholder="例如 29"/></view><view class="field"><text class="field-label">计价单位</text><picker :range="units" :value="units.indexOf(form.unit)" @change="form.unit=units[Number($event.detail.value)]"><view class="input">每 {{ form.unit }}</view></picker></view></view><view class="field"><text class="field-label">服务说明与交付标准</text><textarea v-model="form.description" class="input description" maxlength="1000" placeholder="明确服务包含什么、预约方式、服务时长、验收标准以及不包含的内容。请勿提供外挂或索取账号密码。"/></view><view class="field"><text class="field-label">服务标签（逗号分隔，最多4个）</text><input v-model="form.tagsText" class="input" maxlength="60" placeholder="耐心沟通,稳定在线,新手友好"/></view><view class="field"><text class="field-label">俱乐部挂靠（可选）</text><picker :range="clubOptions" range-key="name" :value="clubIndex" @change="form.club_id=clubOptions[Number($event.detail.value)].id"><view class="input picker-value">{{ clubOptions[clubIndex]?.name || '自由打手 · 无需挂靠' }}<Icon name="arrow" :size="14"/></view></picker><text class="hint">仅能选择已通过挂靠的俱乐部。认证由平台审核，不影响自由接单。</text></view></view>
      <view class="notice mb">提交后由平台审核，审核通过自动上架。编辑已上架服务也会重新审核，已有订单的约定保持不变。</view><button class="btn submit" :disabled="busy" @click="save">{{ busy ? '正在提交…' : '提交审核' }}</button>
    </template>
  </AppShell>
</template>
<script setup>
import { requirePhone } from '../../lib/phone'
/** 文件作用：维护可保存的封面配置，检查金额与服务内容并提交后台审核。 */
import { ref, reactive, computed } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import CoverArt from '../../components/CoverArt.vue'
import Icon from '../../components/Icon.vue'
import { state, init, refreshMe, selectGame } from '../../lib/store'
import { api } from '../../lib/api'
import { go, notify, assetUrl } from '../../lib/format'
import { newLabel, POSITIONS, gameCovers, randomGameCover, normalizeLabelText } from '../../lib/cover'
const positionEditor = ref('')
const serviceId = ref(''), clubs = ref([]), error = ref(''), busy = ref(false)
const form = reactive({ game_id:'delta', title:'', description:'', category:'', price:'', unit:'局', cover_url:'', labels:[], tagsText:'', club_id:'' })
const selectedGame = computed(() => state.games.find(item => item.id===form.game_id) || state.games[0])
const covers = computed(() => gameCovers(selectedGame.value))
const gameIndex = computed(() => Math.max(0,state.games.findIndex(item=>item.id===form.game_id)))
const categoryIndex = computed(() => Math.max(0,(selectedGame.value?.categories || []).indexOf(form.category)))
const clubOptions = computed(() => [{id:'',name:'自由打手 · 无需挂靠'},...clubs.value.filter(club=>club.status==='active'&&['owner','active'].includes(club.membership_status)&&(club.game_ids||[]).includes(form.game_id))])
const clubIndex = computed(() => Math.max(0,clubOptions.value.findIndex(item=>item.id===form.club_id)))
const units = ['局','小时','次','任务'], textColors = ['#ffffff','#00e18a','#f4c36c','#ff6b6b','#101714'], backgroundColors = ['transparent','#07130f','#00e18a','#b98132','#a43c3c']
/** 加载游戏和可挂靠俱乐部，编辑时回填已有服务。 */
async function load() { try { await init(); if(serviceId.value) { const service=await api('/services/'+serviceId.value); Object.assign(form,service,{price:(service.price_cents/100).toFixed(2),tagsText:(service.tags||[]).join(','),labels:service.labels||[]});selectGame(form.game_id) } else { Object.assign(form,{game_id:state.gameId,title:'',description:'',category:'',price:'',unit:'局',cover_url:'',labels:[],tagsText:'',club_id:''});chooseDefaults() } if(state.user) clubs.value=await api('/clubs?mine=1'); if(form.club_id&&!clubOptions.value.some(club=>club.id===form.club_id))form.club_id=''  } catch(e){error.value=e.message} }
/** 为新建服务或新选游戏随机设置底图，并使用当前游戏默认分类。 */
function chooseDefaults() { selectGame(form.game_id);form.cover_url=randomGameCover(selectedGame.value);form.category=selectedGame.value?.categories?.[0] || '' }
/** 仅更换底图，保留所有文字、颜色、位置和服务信息。 */
function changeCover() { form.cover_url=randomGameCover(selectedGame.value,form.cover_url) }
/** 切换游戏，避免带入不适用的分类或封面。 */
function changeGame(event) { const selected=state.games[Number(event.detail.value)];if(!selected||selected.id===form.game_id)return;form.game_id=selected.id;chooseDefaults();if(!clubOptions.value.some(club=>club.id===form.club_id))form.club_id='' }
/** 加入一条默认左下角文案。 */
function addLabel() { if(form.labels.length<6) form.labels.push(newLabel(selectedGame.value?.primary_color)) }
/** 获取当前九宫格位置索引。 */
function positionIndex(value) { return Math.max(0,POSITIONS.findIndex(item=>item[0]===value)) }
/** 获取中文位置名称。 */
function positionName(value) { return POSITIONS[positionIndex(value)][1] }
/** 验证标题、金额和颜色后提交；发布需要用户明确启用打手身份。 */
async function save() {
  if (!(await requirePhone())) return
  if(busy.value)return
  error.value=''
  form.labels.forEach(label=>{label.text=normalizeLabelText(label.text)})
  const price=Math.round(Number(form.price)*100)
  if(!form.title.trim()||!form.description.trim()||!form.category||!form.cover_url)return notify('请完整填写标题、说明、类型和封面')
  if(!/^\d+(\.\d{1,2})?$/.test(form.price)||price<100||price>1000000)return notify('价格请填写1至10000元，最多两位小数')
  if(form.labels.some(label=>!label.text.trim()||!/^#[0-9a-f]{6}$/i.test(label.color)||!(label.background==='transparent'||/^#[0-9a-f]{6}$/i.test(label.background))))return notify('请填写文案，并使用正确的六位颜色值')
  busy.value=true
  try { if(!state.user.provider_enabled){await api('/me','PATCH',{provider_enabled:true});await refreshMe()} const body={game_id:form.game_id,title:form.title.trim(),description:form.description.trim(),category:form.category,price_cents:price,unit:form.unit,cover_url:form.cover_url,labels:form.labels,tags:form.tagsText.split(/[,，]/).map(item=>item.trim()).filter(Boolean).slice(0,4),club_id:form.club_id}; await api(serviceId.value?'/services/'+serviceId.value:'/services',serviceId.value?'PATCH':'POST',body); notify('服务已提交审核');go('/pages/worker/index') } catch(e){error.value=e.message;notify(e)}finally{busy.value=false}
}
onLoad(options=>{serviceId.value=options.id||'';load()})
</script>
<style scoped>
.position-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:6px;margin-top:12px}.position-grid button{margin:0;width:100%;padding:5px;background:#ffffff06;border:1px solid #ffffff20;border-radius:6px;color:#c7d2cb;font-size:12px;line-height:24px}.position-grid button.selected{border-color:var(--accent);color:var(--accent)}.position-picker{margin:0;color:var(--text);line-height:26px;background:#ffffff07;border:1px solid #ffffff20}.editor-intro{display:flex;flex-direction:column;gap:8px;margin:6px 0 18px}
.editor-intro .muted{font-size:13px;color:#a8b4ae}
.preview{border:1px solid #ffffff20;border-radius:16px;overflow:hidden;margin-bottom:22px;background:var(--surface)}
.preview-toolbar{display:flex;align-items:center;justify-content:space-between;padding:5px 15px;border-bottom:1px solid #ffffff12;color:#bac5bf;font-size:12px}
.preview-caption{padding:16px;display:flex;justify-content:space-between;align-items:center;gap:12px;font-size:14px;font-weight:650}
.preview-caption>text:first-child{max-width:68%;overflow:hidden;white-space:nowrap;text-overflow:ellipsis}
.preview-caption>.accent{white-space:nowrap}
.form-panel{margin-bottom:16px}
.form-panel>.section-title{display:block;margin-bottom:13px}
.form-panel .hint{font-size:12px;color:#a4afa9}
.form-panel .field-label{font-size:13px;color:#c4cdc8}
.form-panel .text-btn,.preview .text-btn{min-height:38px;line-height:38px;font-size:13px;padding:0 6px}
.form-panel .text-btn[disabled]{opacity:.4}
.form-panel .input{font-size:14px;min-height:48px}
.block{display:block;margin-bottom:20px}
.picker-value{display:flex;justify-content:space-between;align-items:center}
.cover-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px;margin-top:12px}
.cover-option{height:92px;border-radius:10px;overflow:hidden;position:relative;border:2px solid transparent}
.cover-option image{width:100%;height:100%}
.cover-option.chosen{border-color:var(--accent)}
.cover-check{position:absolute;bottom:6px;right:6px;background:var(--accent);color:#07120d;border-radius:50%;height:22px;width:22px;display:flex;align-items:center;justify-content:center}
.label-editor{padding:18px 0;border-top:1px solid #ffffff16}
.label-index{color:#c1cbc5;font-size:13px;font-weight:600}
.label-editor .input{margin-top:8px}
.colors{display:flex;flex-direction:column;gap:16px;margin-top:16px}
.color-field{min-width:0;font-size:12px;color:#abbab1}
.swatches{display:flex;gap:10px;margin-top:10px}
.swatches button{height:34px;width:34px;min-width:34px;border:1px solid #ffffff50;border-radius:50%;padding:0;margin:0}
.swatches button.chosen{outline:2px solid var(--text);outline-offset:3px}
.swatches .transparent{border-style:dashed;background-color:#ffffff06!important}
.color-code{width:100%;font-size:13px;height:38px;margin-top:12px;padding:0 10px;color:#d1dcd5;background:#ffffff05;border:1px solid #ffffff14;border-radius:7px}
.label-bottom{display:flex;align-items:center;justify-content:space-between;margin-top:16px;gap:12px}
.position-picker{font-size:12px;min-height:40px;background:#ffffff08;border:1px solid #ffffff16;border-radius:8px;padding:9px 12px;display:flex;align-items:center;gap:10px;white-space:nowrap}
.font-picker{display:flex;align-items:center;flex:1;font-size:12px;color:#bac7bf;min-width:0}
.font-picker>text{white-space:nowrap}
.font-picker slider{flex:1;min-width:45px;margin:0 3px 0 8px}
.price-grid{display:grid;grid-template-columns:1fr 1fr;gap:12px}
.description{height:150px!important;line-height:1.75}
.submit{margin-bottom:20px;min-height:50px;font-size:15px}
@media(max-width:350px){.swatches{gap:8px}.swatches button{width:32px;min-width:32px;height:32px}.preview-caption{font-size:13px}.label-bottom{gap:8px}.position-picker{padding:9px}.font-picker{font-size:11px}}
</style>
