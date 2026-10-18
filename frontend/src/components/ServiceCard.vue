<!-- 文件作用：以独立图片与悬浮文案展示服务，紧凑展示真实价格、认证和成交数。 -->
<template><view class="service-card" @click="go('/pages/service/index?id=' + service.id)"><CoverArt :src="service.cover_url" :labels="service.labels"/><view class="service-body"><view class="service-title">{{ service.title }}</view><view class="service-tags"><view v-if="service.club_verified" class="cert"><Icon name="shield" :size="10" />俱乐部认证</view><text v-else class="service-tag">{{ service.category }}</text><text v-for="tag in visibleTags" :key="tag" class="service-tag">{{ tag }}</text></view><view class="service-price"><view><text class="currency">¥</text><text class="amount">{{ money(service.price_cents) }}</text><text class="unit"> / {{ service.unit || '单' }}</text></view><text class="count">{{ service.completed_count || 0 }} 单</text></view></view></view></template>
<script setup>
import { computed } from 'vue'
import CoverArt from './CoverArt.vue'
import Icon from './Icon.vue'
import { money, go } from '../lib/format'
const props = defineProps({ service: { type: Object, required: true } })
const visibleTags = computed(() => (props.service.tags || []).filter(tag => tag !== '俱乐部认证').slice(0, 1))
</script>
<style scoped>
.service-card{border:1px solid #ffffff33;background:var(--surface);border-radius:10px;overflow:hidden;cursor:pointer}.service-body{position:relative;padding:3px 8px 6px;background:var(--surface)}.service-title{font-weight:750;font-size:13px;line-height:19px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.service-tags{display:flex;gap:4px;min-height:20px;align-items:center;overflow:hidden;white-space:nowrap}.service-tag,.cert{font-size:10px;line-height:15px;border:1px solid #ffffff30;padding:0 4px;border-radius:3px;color:#bdc8c1;display:inline-flex;align-items:center;gap:2px}.cert{color:#c7d2cb}.service-price{display:flex;justify-content:space-between;align-items:center;gap:4px;margin-top:2px;height:25px;line-height:1}.currency{color:var(--accent);font-size:12px;font-weight:700;margin-right:1px}.amount{color:var(--accent);font-size:23px;font-weight:800;line-height:1;letter-spacing:-.7px}.unit{font-size:10px;color:#bcc6bf}.count{font-size:10px;color:#a7b4ac;white-space:nowrap}@media(max-width:350px){.service-body{padding-left:6px;padding-right:6px}.service-title{font-size:12px}.service-tag,.cert{font-size:9px}.unit,.count{font-size:9px}}
</style>
