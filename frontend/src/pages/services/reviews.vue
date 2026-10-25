<!-- 文件作用：按已确认设计展示服务全部评价，访客只读，下拉刷新与触底沿用原服务评价分页接口。 -->
<template>
  <AppShell title="全部评价" :game-id="service?.game_id" back>
    <view v-if="error" class="reviews-error"><text>{{ error }}</text><button class="text-btn" @click="retryFailed">重新加载</button></view>
    <view v-if="service" class="reviews-page"><button class="review-service" @click="go('/pages/services/detail?id='+encodeURIComponent(service.id))"><image class="review-service-image" :src="assetUrl(service.cover_url)" mode="aspectFill"/><view class="review-service-copy"><text class="review-service-title">{{ service.title }}</text><text class="review-service-sub">{{ service.category }} · {{ service.provider_name }}</text></view><Icon name="arrow" :size="15"/></button><ServiceReview v-for="review in reviews" :key="review.id" :review="review"/><EmptyState v-if="!loading&&!reviews.length" title="还没有评价" detail="完成订单后，买家可留下真实体验。" icon="star"/><ListLoadState v-if="reviews.length||loading" :loading="loading" :has-more="reviewsMore" :error="error" @retry="retryFailed"/><view class="reviews-note"><Icon name="shield-check" :size="13"/><text>完成验收的订单买家可在订单详情中评价</text></view></view>
  </AppShell>
</template>
<script setup lang="ts">
import {ref} from 'vue'
import {onLoad,onShow,onHide,onUnload} from '@dcloudio/uni-app'
import AppShell from '../../components/AppShell.vue'
import Icon from '../../components/Icon.vue'
import EmptyState from '../../components/EmptyState.vue'
import ListLoadState from '../../components/ListLoadState.vue'
import ServiceReview from '../../components/ServiceReview.vue'
import {getService} from '../../api/services.api'
import {state,init,sessionRevision} from '../../lib/store'
import {useNativePageRefresh} from '../../lib/native-page-refresh'
import {appendUniqueRows} from '../../lib/list-page'
import {errorMessage,errorStatus} from '../../api/client'
import {assetUrl,go} from '../../lib/format'
import type {Service,Review} from '../../types/models'
const service=ref<Service|null>(null),reviews=ref<Review[]>([]),loading=ref(true),error=ref(''),reviewsMore=ref(false)
let id='',visible=true,disposed=false,readVersion=0,reviewOffset=0,lastFailedAppend=false
/** 使用原始返回条数推进游标，去重不改变服务端分页，旧上下文回包不覆盖刷新。 */
async function load(append=false):Promise<void>{
  const adding=append===true;if(!id||!visible||disposed||adding&&(loading.value||!reviewsMore.value))return
  const current=++readVersion,target=id,offset=adding?reviewOffset:0;loading.value=true;error.value=''
  try{await init(false,true);if(!visible||disposed||current!==readVersion)return;const result=await getService(target,true,{limit:20,offset});if(!visible||disposed||current!==readVersion||target!==id)return
    const batch=result.reviews||[];reviews.value=appendUniqueRows(adding?reviews.value:[],batch);if(!adding)service.value=result;reviewOffset=offset+batch.length;reviewsMore.value=batch.length===20
  }catch(err){if(visible&&!disposed&&current===readVersion){if(errorStatus(err)===404){service.value=null;reviews.value=[];reviewsMore.value=false}lastFailedAppend=adding;error.value=errorMessage(err)}}finally{if(current===readVersion)loading.value=false}
}
/** 网络错误保留原列表并恢复失败方向，失效目标从首屏重新读取。 */
async function retryFailed():Promise<void>{if(!loading.value)await load(lastFailedAppend&&!!service.value)}
useNativePageRefresh({refresh:()=>load(),more:()=>error.value?retryFailed():load(true),context:()=>[state.user?.id,sessionRevision(),id]})
onLoad(options=>{id=options?.id||'';if(id)void load();else{loading.value=false;error.value='服务不存在或缺少编号'}})
onShow(()=>{visible=true;if(id&&!service.value&&!loading.value)void load()})
onHide(()=>{visible=false;readVersion++;loading.value=false})
onUnload(()=>{disposed=true;visible=false;readVersion++})
</script>
<style scoped>
.reviews-page{min-width:0}.review-service{display:flex;align-items:center;gap:23.0769rpx;width:100%;min-width:0;padding:23.0769rpx;border:1px solid var(--line-subtle);border-radius:15.3846rpx;background:var(--surface);text-align:left;color:var(--muted);margin-bottom:11.5385rpx}.review-service-image{display:block;flex-shrink:0;width:138.4615rpx;height:115.3846rpx;border-radius:11.5385rpx}.review-service-copy{flex:1;min-width:0}.review-service-title{display:-webkit-box;-webkit-box-orient:vertical;-webkit-line-clamp:2;overflow:hidden;font-size:30.7692rpx;line-height:44.2308rpx;color:var(--text);font-weight:650}.review-service-sub{display:block;margin-top:9.6154rpx;font-size:21.1538rpx;line-height:34.6154rpx;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.reviews-note{display:flex;align-items:center;justify-content:center;gap:11.5385rpx;font-size:21.1538rpx;line-height:36.5385rpx;color:var(--muted);padding:30.7692rpx 0}.reviews-error{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;color:#efb299;font-size:25rpx;line-height:42.3077rpx;overflow-wrap:anywhere}
</style>
