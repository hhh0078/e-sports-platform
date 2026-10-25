<!-- 文件作用：统一服务卡片，封面内按标题在上标签在下展示并为角标预留自定义文案空间，价格和操作保持图下正常流。 -->
<template>
  <view class="service-card" :style="pageTheme(service.game_id)" @click="openDetail">
    <CoverArt :src="service.cover_url" :labels="service.labels" :active="active" lazy-load :aspect-ratio="4 / 3" :reserve-bottom-pixels="bottomReservation" :reserve-top-pixels="topReservation" readable :min-font-pixels="10">
      <view v-if="hasDeposit||hasClubDeposit||service.club_verified" class="service-deposits"><DepositBadge v-if="hasDeposit" :amount-cents="service.provider_deposit_cents"/><DepositBadge v-if="hasClubDeposit" kind="user_club" :amount-cents="service.provider_club_deposit_cents"/><SecurityMarker v-if="service.club_verified" label="认证" description="俱乐部认证成员"/></view>
      <view class="service-overlay"><text class="service-title">{{ service.title }}</text><view class="service-tags"><view v-if="service.club_verified" class="service-cert"><Icon name="shield" :size="9"/><text>俱乐部认证</text></view><text v-if="gameLabel" class="service-tag">{{ gameLabel }}</text><text v-if="service.category" class="service-category">{{ service.category }}</text></view></view>
    </CoverArt>
    <view v-if="!coverOnly" class="service-body">
      <view v-if="!coverOnly" class="service-price"><view class="price-value"><text class="currency">¥</text><text class="amount">{{ money(service.price_cents) }}</text><text class="unit">/{{ service.unit || '单' }}</text></view><text class="count">成交 {{ service.completed_count || 0 }}</text></view>
      <view v-if="!coverOnly&&$slots.actions" class="service-actions" @click.stop><slot name="actions" /></view>
    </view>
  </view>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import CoverArt from './CoverArt.vue'
import DepositBadge from './DepositBadge.vue'
import SecurityMarker from './SecurityMarker.vue'
import Icon from './Icon.vue'
import { money, go } from '../lib/format'
import { state, pageTheme } from '../lib/store'
import type { Service } from '../types/models'
import { designPixels } from '../lib/design-units'
import { useViewportWidth } from '../lib/viewport'
const props = withDefaults(defineProps<{ service: Service; interactive?: boolean; coverOnly?: boolean; active?: boolean }>(), { interactive: true, coverOnly: false, active: true })
const viewportWidth = useViewportWidth()
/** 仅对后端当前有效的正整数平台押金显示角标，布局同步预留顶部安全带。 */
const hasDeposit = computed(() => Number.isSafeInteger(props.service.provider_deposit_cents) && (props.service.provider_deposit_cents || 0) > 0)
/** 只展示服务当前挂靠俱乐部内已实缴的押金，不使用俱乐部规则金额。 */
const hasClubDeposit = computed(() => Number.isSafeInteger(props.service.provider_club_deposit_cents) && (props.service.provider_club_deposit_cents || 0) > 0)
const bottomReservation = computed(() => designPixels(52, viewportWidth.value))
const topReservation = computed(() => designPixels(hasDeposit.value||hasClubDeposit.value||props.service.club_verified?36:0, viewportWidth.value))
const gameLabel = computed(() => { const game = state.games.find(item => item.id === props.service.game_id); return game?.short_name || game?.name || '' })
/** 通过当前业务路由打开服务，发布预览可关闭跳转。 */
function openDetail(): void { if (props.interactive) go('/pages/services/detail?id=' + encodeURIComponent(props.service.id)) }
</script>
<style scoped>
.service-card{display:flex;flex-direction:column;min-width:0;overflow:hidden;border:1px solid var(--line-subtle);background:var(--surface);border-radius:19.2308rpx}
.service-deposits{position:absolute;top:0;right:0;z-index:3;display:flex;flex-direction:row;align-items:flex-start;gap:0;max-width:100%;line-height:0;pointer-events:none}
.service-overlay{position:absolute;z-index:3;right:15.3846rpx;bottom:0;left:15.3846rpx;display:flex;flex-direction:column;justify-content:flex-end;gap:7.6923rpx;height:100rpx;padding-bottom:19.2308rpx;box-sizing:border-box;min-width:0;pointer-events:none}
.service-body{display:flex;flex-direction:column;gap:11.5385rpx;min-width:0;padding:0 15.3846rpx 11.5385rpx}
.service-title{display:block;flex-shrink:0;min-width:0;font-size:25rpx;line-height:34.6154rpx;font-weight:700;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.service-tags{display:flex;align-items:center;gap:7.6923rpx;flex-shrink:0;height:30.7692rpx;min-width:0;overflow:hidden;color:var(--muted);font-size:15.3846rpx;line-height:23.0769rpx}
.service-category,.service-tag{display:block;flex-shrink:1;min-width:0;max-width:45%;padding:1.9231rpx 7.6923rpx;border:1px solid #ffffff0d;background:#ffffff08;border-radius:5.7692rpx;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.service-cert{display:inline-flex;flex-shrink:0;align-items:center;gap:3.8462rpx;padding:1.9231rpx 5.7692rpx;border:1px solid #dfbb7133;border-radius:5.7692rpx;background:#a17d191a;color:#dfc27f;white-space:nowrap}
.service-price{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:baseline;gap:3.8462rpx 7.6923rpx;min-width:0}
.price-value{display:flex;flex-wrap:wrap;align-items:baseline;gap:0 1.9231rpx;min-width:0;max-width:100%}.currency{font-size:23.0769rpx;color:var(--accent);font-weight:700}.amount{font-size:40.3846rpx;line-height:48.0769rpx;color:var(--accent);font-weight:750;white-space:nowrap}.unit{max-width:100%;font-size:19.2308rpx;color:var(--muted);overflow-wrap:anywhere}.count{max-width:100%;font-size:17.3077rpx;color:var(--muted);white-space:nowrap}.service-actions{display:block;border-top:1px solid var(--line-subtle);padding-top:15.3846rpx}
@media(max-width:350px){.service-title{font-size:23.0769rpx}.amount{font-size:36.5385rpx}.count{font-size:15.3846rpx}}
</style>
