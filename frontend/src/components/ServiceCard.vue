<!-- 文件作用：统一服务卡片，封面内独立展示标签标题并预留自定义文案空间，价格和操作保持图下正常流。 -->
<template>
  <view class="service-card" :style="pageTheme(service.game_id)" @click="openDetail">
    <CoverArt :src="service.cover_url" :labels="service.labels" :aspect-ratio="4 / 3" :reserve-bottom-pixels="48">
      <view class="service-overlay">
        <view class="service-tags"><view v-if="service.club_verified" class="service-cert"><Icon name="shield" :size="9" /><text>俱乐部认证</text></view><text v-if="gameLabel" class="service-tag">{{ gameLabel }}</text><text v-if="service.category" class="service-category">{{ service.category }}</text></view>
        <text class="service-title">{{ service.title }}</text>
      </view>
    </CoverArt>
    <view v-if="!coverOnly" class="service-body">
      <view class="service-price"><view class="price-value"><text class="currency">¥</text><text class="amount">{{ money(service.price_cents) }}</text><text class="unit">/{{ service.unit || '单' }}</text></view><text class="count">成交 {{ service.completed_count || 0 }}</text></view>
      <view v-if="$slots.actions" class="service-actions" @click.stop><slot name="actions" /></view>
    </view>
  </view>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import CoverArt from './CoverArt.vue'
import Icon from './Icon.vue'
import { money, go } from '../lib/format'
import { state, pageTheme } from '../lib/store'
import type { Service } from '../types/models'
const props = withDefaults(defineProps<{ service: Service; interactive?: boolean; coverOnly?: boolean }>(), { interactive: true, coverOnly: false })
const gameLabel = computed(() => { const game = state.games.find(item => item.id === props.service.game_id); return game?.short_name || game?.name || '' })
/** 通过当前业务路由打开服务，发布预览可关闭跳转。 */
function openDetail(): void { if (props.interactive) go('/pages/services/detail?id=' + encodeURIComponent(props.service.id)) }
</script>
<style scoped>
.service-card{display:flex;flex-direction:column;min-width:0;overflow:hidden;border:1px solid var(--line-subtle);background:var(--surface);border-radius:10px}
.service-overlay{position:absolute;right:8px;bottom:0;left:8px;display:flex;flex-direction:column;justify-content:flex-end;gap:4px;height:48px;padding-bottom:8px;box-sizing:border-box;min-width:0;pointer-events:none}
.service-body{display:flex;flex-direction:column;gap:8px;min-width:0;padding:4px 8px 6px}
.service-title{display:block;flex-shrink:0;min-width:0;font-size:13px;line-height:18px;font-weight:700;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.service-tags{display:flex;align-items:center;gap:4px;flex-shrink:0;height:16px;min-width:0;overflow:hidden;color:var(--muted);font-size:8px;line-height:12px}
.service-category,.service-tag{display:block;flex-shrink:1;min-width:0;max-width:45%;padding:1px 4px;border:1px solid #ffffff0d;background:#ffffff08;border-radius:3px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.service-cert{display:inline-flex;flex-shrink:0;align-items:center;gap:2px;padding:1px 3px;border:1px solid #dfbb7133;border-radius:3px;background:#a17d191a;color:#dfc27f;white-space:nowrap}
.service-price{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:baseline;gap:2px 4px;min-width:0}
.price-value{display:flex;flex-wrap:wrap;align-items:baseline;gap:0 1px;min-width:0;max-width:100%}.currency{font-size:12px;color:var(--accent);font-weight:700}.amount{font-size:21px;line-height:25px;color:var(--accent);font-weight:750;white-space:nowrap}.unit{max-width:100%;font-size:10px;color:var(--muted);overflow-wrap:anywhere}.count{max-width:100%;font-size:9px;color:var(--muted);white-space:nowrap}.service-actions{display:block;border-top:1px solid var(--line-subtle);padding-top:8px}
@media(max-width:350px){.service-title{font-size:12px}.amount{font-size:19px}.count{font-size:8px}}
</style>
