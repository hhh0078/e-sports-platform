/** 文件作用：管理控制台真实经营数据和自定义周期，隔离普通用户、旧账号和迟到统计响应。 */
import { ref,watch } from 'vue'
import { onShow,onHide,onUnload } from '@dcloudio/uni-app'
import { getAdminOverview,getAdminAnalytics } from '../api/admin.api'
import type { AdminOverview } from '../api/admin.api'
import type { AdminAnalytics } from '../types/admin-analytics'
import { state,init,sessionRevision } from './store'
import { token,errorMessage } from '../api/client'
import { adminDateRange } from './admin-chart'
import { useNativePageRefresh } from './native-page-refresh'

/** 对自定义自然日周期做前置校验，最终仍由服务端验证权限与上限。 */
export function validateAdminPeriod(start:string,end:string):{start:string;end:string}{
  adminDateRange(1,start);adminDateRange(1,end)
  const days=(Date.parse(end+'T00:00:00Z')-Date.parse(start+'T00:00:00Z'))/86400000+1
  if(days<1||days>366)throw new Error('请选择起止顺序正确且不超过366天的周期')
  return{start,end}
}
/** 只有平台管理员才读取经营数据，切换周期只更新曲线和周期指标。 */
export function useAdminConsole(){
  const overview=ref<AdminOverview|null>(null),analytics=ref<AdminAnalytics|null>(null),loading=ref(false),chartLoading=ref(false),error=ref(''),chartError=ref(''),range=ref<'7'|'30'|'custom'>('7')
  const initial=adminDateRange(7),startDate=ref(initial.start),endDate=ref(initial.end)
  let visible=false,disposed=false,overviewVersion=0,chartVersion=0
  let appliedDates:{start:string;end:string}|undefined
  /** 捕获当前管理员会话，不以客户端角色作为最终授权。 */
  function session(){return{id:state.user?.id,token:token(),revision:sessionRevision()}}
  /** 隐藏页面或切换身份后拒绝数据回填。 */
  function same(owner:ReturnType<typeof session>):boolean{return!disposed&&visible&&state.user?.role==='admin'&&owner.id===state.user.id&&owner.token===token()&&owner.revision===sessionRevision()}
  /** 角色变化立即移除旧控制台数据，个人用户不出现商户或交易管理内容。 */
  function clear(){appliedDates=undefined;overview.value=null;analytics.value=null;error.value='';chartError.value='';overviewVersion++;chartVersion++;loading.value=false;chartLoading.value=false}
  watch([()=>state.user?.id,()=>state.user?.role],()=>{clear();if(visible)void load()})
  /** 读取独立平台总览，失败不影响已核实周期图。 */
  async function loadOverview(){
    if(!visible||state.user?.role!=='admin')return
    const owner=session(),version=++overviewVersion;loading.value=true;error.value=''
    try{const result=await getAdminOverview();if(same(owner)&&version===overviewVersion)overview.value=result}
    catch(err){if(same(owner)&&version===overviewVersion)error.value=errorMessage(err)}finally{if(version===overviewVersion)loading.value=false}
  }
  /** 请求真实周期统计，旧周期完成再晚也不能覆盖新周期。 */
  async function readChart(refreshApplied=false){
    if(!visible||state.user?.role!=='admin')return
    const owner=session(),version=++chartVersion;chartLoading.value=true;chartError.value=''
    try{
      const dates=refreshApplied&&appliedDates?{...appliedDates}:range.value==='custom'?validateAdminPeriod(startDate.value,endDate.value):adminDateRange(Number(range.value))
      const result=await getAdminAnalytics(dates)
      if(!same(owner)||version!==chartVersion)return
      analytics.value=result;appliedDates={start:result.period.start,end:result.period.end};if(!refreshApplied){startDate.value=result.period.start;endDate.value=result.period.end}
    }catch(err){if(same(owner)&&version===chartVersion)chartError.value=errorMessage(err)}finally{if(version===chartVersion)chartLoading.value=false}
  }
  /** 显式应用当前周期草稿，原生点击事件不被当作刷新参数。 */
  async function loadChart(){await readChart()}
  /** 同时刷新各自独立的数据源，不让一项失败销毁其它已核实结果。 */
  async function load(){try{await init();if(!visible||disposed)return;if(state.user?.role!=='admin'){clear();return}await Promise.all([loadOverview(),loadChart()])}catch(err){if(visible&&!disposed)error.value=errorMessage(err)}}
  /** 快捷周期立即查款，自定义则保留草稿等待显式应用。 */
  function chooseRange(value:'7'|'30'|'custom'){range.value=value;if(value!=='custom')void loadChart();else{chartVersion++;chartLoading.value=false}}
  /** 停止离开后的异步回填，真实私密数据不写入本地缓存。 */
  function hide(){visible=false;overviewVersion++;chartVersion++;loading.value=false;chartLoading.value=false}
  onShow(()=>{visible=true;void load()});onHide(hide);onUnload(()=>{disposed=true;hide();clear()})
  useNativePageRefresh({context:()=>[state.user?.id,state.user?.role,sessionRevision(),range.value],refresh:async()=>{await init();if(visible&&state.user?.role==='admin')await Promise.all([loadOverview(),readChart(true)])}})
  return{overview,analytics,loading,chartLoading,error,chartError,range,startDate,endDate,load,loadOverview,loadChart,chooseRange}
}
