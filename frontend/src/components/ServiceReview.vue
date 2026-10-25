<!-- 文件作用：复用服务详情与全部评价的真实买家反馈，不补造头像、昵称、成交或评分。 -->
<template>
  <view class="service-review">
    <view class="review-avatar"><Icon name="user" :size="23"/></view>
    <view class="review-copy"><view class="review-top"><text class="review-name">{{ review.buyer_name || '玩家' }}</text><text class="review-completed">已完成订单</text></view><text class="review-date">{{ time(review.created_at) }}</text><view class="review-stars"><Icon v-for="star in 5" :key="star" name="star" :size="16" :class="['review-star',{'review-star-lit':star<=rating}]"/><text class="review-score">{{ rating }}/5</text></view><text class="review-body" selectable>{{ review.content }}</text></view>
  </view>
</template>
<script setup lang="ts">
import {computed} from 'vue'
import Icon from './Icon.vue'
import {time} from '../lib/format'
import type {Review} from '../types/models'
const props=defineProps<{review:Review}>()
/** 只展示后端合法范围内的真实评分，不让异常值增加虚构星级。 */
const rating=computed(()=>Math.max(0,Math.min(5,Number(props.review.rating)||0)))
</script>
<style scoped>
.service-review{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle)}.review-avatar{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:84.6154rpx;height:84.6154rpx;border:1px solid var(--line);border-radius:50%;background:var(--surface);color:var(--muted)}.review-copy{min-width:0;flex:1}.review-top{display:flex;align-items:center;justify-content:space-between;gap:15.3846rpx;min-width:0}.review-name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:28.8462rpx;font-weight:650;line-height:42.3077rpx}.review-completed{flex-shrink:0;color:var(--accent);font-size:19.2308rpx;line-height:30.7692rpx;padding:1.9231rpx 13.4615rpx;border:1px solid var(--line);border-radius:19.2308rpx}.review-date{display:block;font-size:21.1538rpx;line-height:34.6154rpx;color:var(--muted)}.review-stars{display:flex;align-items:center;gap:3.8462rpx;margin:9.6154rpx 0 15.3846rpx}.review-star{color:var(--line)}.review-star-lit{color:#e9bc71}.review-score{margin-left:11.5385rpx;font-size:21.1538rpx;color:var(--muted)}.review-body{display:block;white-space:pre-wrap;overflow-wrap:anywhere;font-size:26.9231rpx;line-height:46.1538rpx;color:var(--text)}
</style>
