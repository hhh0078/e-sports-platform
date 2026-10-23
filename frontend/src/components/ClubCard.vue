<!-- 文件作用：统一俱乐部整图卡片，真实身份、简介、游戏和加入门槛均作为封面内独立文字层展示。 -->
<template>
  <view class="club-card" @click="openDetail">
    <image v-if="coverUrl" class="club-cover-image" :src="assetUrl(coverUrl)" mode="aspectFill"/>
    <view class="club-card-shade"/>
    <view class="club-card-overlay">
      <view class="club-identity"><view class="club-monogram">{{ club.name?.slice(0,1) }}</view><view class="club-heading"><view class="club-name-row"><text class="club-name">{{ club.name }}</text><Icon v-if="club.verified" name="shield" :size="15" class="club-certified"/></view><text class="club-meta">{{ club.member_count||0 }} 位成员 · {{ club.owner_name||'负责人待完善' }}{{ club.owner_name?' 主理':'' }}</text></view></view>
      <text v-if="club.description" class="club-description">{{ club.description }}</text>
      <view v-if="gameTags.length" class="game-tags"><text v-for="tag in gameTags.slice(0,2)" :key="tag.id" class="club-game-tag">{{ tag.name }}</text><text v-if="gameTags.length>2" class="club-game-more">+{{ gameTags.length-2 }}</text></view>
      <view class="club-footer"><view class="club-status-row"><view :class="['club-verification',{'club-active':club.status==='active'}]"><Icon :name="club.verified?'shield':'club'" :size="13"/><text class="club-status-label">{{ statusText }}</text></view><Icon name="arrow" :size="14" class="club-arrow"/></view><text class="club-admission">{{ admissionText }}</text></view>
    </view>
  </view>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import Icon from './Icon.vue'
import { state } from '../lib/store'
import { assetUrl, go, money, statusLabel } from '../lib/format'
import type { Club } from '../types/models'
const props=defineProps<{club:Club}>()
/** 优先使用俱乐部装修封面，未设置时回退到其已配置游戏的官方封面。 */
const coverUrl=computed(()=>props.club.cover_url||state.games.find(item=>props.club.game_ids.includes(item.id))?.cover_url||'')
/** 游戏标签保留真实顺序并去重，界面只展示前两项和其余数量。 */
const gameTags=computed(()=>[...new Set(props.club.game_ids)].map(id=>({id,name:state.games.find(item=>item.id===id)?.short_name||id})))
/** 使用真实审核状态与认证标记，不将待审核俱乐部展示为开放加入。 */
const statusText=computed(()=>props.club.status==='active'?(props.club.verified?'平台认证':'开放加入'):statusLabel(props.club.status))
/** 分别显示俱乐部真实考核与规则押金，未设置规则时才显示无额外门槛。 */
const admissionText=computed(()=>{
  const requirements:string[]=[]
  if(props.club.assessment?.trim())requirements.push('需考核')
  if(props.club.deposit_cents>0)requirements.push('规则押金 ¥'+money(props.club.deposit_cents))
  return requirements.join(' · ')||'无额外门槛'
})
/** 点击整张卡片进入对应俱乐部主页，编码标识避免破坏站内查询参数。 */
function openDetail():void{go('/pages/community/clubs/detail?id='+encodeURIComponent(props.club.id))}
</script>
<style scoped>
.club-card{position:relative;min-width:0;min-height:461.5385rpx;padding-top:75%;margin-bottom:30.7692rpx;overflow:hidden;border:1px solid var(--line-subtle);border-radius:19.2308rpx;background:var(--surface);box-sizing:border-box}
.club-cover-image{position:absolute;inset:0;display:block;width:100%;height:100%}.club-card-shade{position:absolute;inset:0;background:linear-gradient(180deg,transparent 12%,#08110e36 28%,#08110edb 56%,var(--bg) 100%)}
.club-card-overlay{position:absolute;right:0;bottom:0;left:0;display:flex;flex-direction:column;gap:9.6154rpx;min-width:0;padding:23.0769rpx;box-sizing:border-box;color:#f0f5f1}
.club-identity{display:flex;align-items:center;gap:19.2308rpx;min-width:0}.club-monogram{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:76.9231rpx;height:76.9231rpx;border:1px solid #ffffff36;border-radius:15.3846rpx;background:#08110e4d;color:#f0f5f1;font-size:40.3846rpx;font-weight:650}.club-heading{flex:1;min-width:0}.club-name-row{display:flex;align-items:center;gap:11.5385rpx;min-width:0}.club-name{display:block;min-width:0;font-size:34.6154rpx;line-height:46.1538rpx;font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.club-certified{flex-shrink:0;color:var(--accent)}.club-meta{display:block;min-width:0;font-size:21.1538rpx;line-height:34.6154rpx;color:#c0cdc5;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.club-description{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;max-height:69.2308rpx;min-width:0;overflow:hidden;overflow-wrap:anywhere;font-size:21.1538rpx;line-height:34.6154rpx;color:#d1dbd5}
.game-tags{display:flex;align-items:center;gap:11.5385rpx;min-width:0;max-width:100%;overflow:hidden}.club-game-tag{display:block;flex-shrink:1;min-width:0;max-width:38%;padding:3.8462rpx 11.5385rpx;border:1px solid #ffffff24;border-radius:9.6154rpx;background:#08110e66;font-size:19.2308rpx;line-height:28.8462rpx;color:#d1ddd5;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.club-game-more{flex-shrink:0;min-width:38.4615rpx;padding:3.8462rpx 7.6923rpx;border:1px solid #ffffff24;border-radius:9.6154rpx;background:#08110e66;text-align:center;font-size:19.2308rpx;line-height:28.8462rpx;color:#c0cdc5}
.club-footer{display:flex;flex-direction:column;gap:5.7692rpx;min-width:0}.club-status-row{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.club-verification{display:flex;align-items:center;gap:7.6923rpx;min-width:0;color:#c0cdc5;font-size:19.2308rpx;line-height:34.6154rpx}.club-active{color:var(--accent)}.club-status-label{display:block;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.club-arrow{flex-shrink:0;color:#c0cdc5}.club-admission{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;min-width:0;max-height:61.5385rpx;overflow:hidden;overflow-wrap:anywhere;font-size:19.2308rpx;line-height:30.7692rpx;color:#a8b9ae}
@media(max-width:350px){.club-card{min-height:461.5385rpx}.club-name{font-size:30.7692rpx;line-height:44.2308rpx}.club-meta{font-size:19.2308rpx}.club-card-overlay{gap:7.6923rpx}.club-monogram{width:69.2308rpx;height:69.2308rpx;font-size:36.5385rpx}}
</style>
