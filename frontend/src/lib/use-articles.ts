/** 文件作用：为公开和管理文章列表共享真实分页、筛选与旧响应隔离，草稿接口只在管理员身份下调用。 */
import {computed,ref,watch} from 'vue'
import {onShow,onHide,onUnload} from '@dcloudio/uni-app'
import {listArticles,listAdminArticles} from '../api/content.api'
import {errorMessage,errorStatus,token} from '../api/client'
import {state,init,sessionRevision} from './store'
import {appendUniqueRows} from './list-page'
import {useNativePageRefresh} from './native-page-refresh'
import type {ArticleStatus,ArticleSummary} from '../types/content'
/** 构建对应公开或管理员文章列表，不改变其它页面的草稿和搜索。 */
export function useArticles(admin=false){
  const rows=ref<ArticleSummary[]>([]),query=ref(''),status=ref<ArticleStatus|''>(''),loading=ref(false),error=ref(''),total=ref(0),offset=ref(0)
  const allowed=computed(()=>!admin||state.user?.role==='admin'&&state.user.status!=='frozen'),hasMore=computed(()=>offset.value<total.value)
  let visible=false,disposed=false,version=0,submitted='',failed:{more:boolean;offset:number;query:string;status:ArticleStatus|''}|null=null
  /** 捕获权限与认证修订；公共读取也不允许旧账号响应污染新筛选。 */
  function session(){return{id:state.user?.id,role:state.user?.role,adminRole:state.user?.platform_admin_role,status:state.user?.status,token:token(),revision:sessionRevision()}}
  /** 私有草稿只交付给同一有效管理员，公开接口仍由领域API保证不携带令牌。 */
  function sameSession(actor:ReturnType<typeof session>):boolean{return !disposed&&allowed.value&&actor.id===state.user?.id&&actor.role===state.user?.role&&actor.adminRole===state.user?.platform_admin_role&&actor.status===state.user?.status&&actor.token===token()&&actor.revision===sessionRevision()}
  /** 身份或权限变化清空旧私有行和失败方向，旧finally不能关闭新请求。 */
  function clear():void{version++;rows.value=[];offset.value=0;total.value=0;loading.value=false;error.value='';failed=null}
  /** 读取真实分页；失败记录精确方向、偏移和已提交筛选，刷新失败不得跳到下一页。 */
  async function load(more=false,retry=false):Promise<void>{
    if(!visible||disposed)return
    if(more&&!retry&&failed){await retryFailed();return}
    if(more&&(loading.value||!hasMore.value))return
    const previous=retry?failed:null,append=previous?previous.more:more,start=previous?previous.offset:append?offset.value:0
    if(!append&&!previous)submitted=query.value.trim()
    const search=previous?previous.query:submitted,selected=previous?previous.status:status.value,current=++version,actor=session()
    loading.value=true;error.value=''
    try{await init(false,!admin);if(!visible||current!==version||disposed)return;if(!allowed.value){clear();return}if(!sameSession(actor))return;const filter={q:search,limit:20,offset:start};const page=admin?await listAdminArticles({...filter,status:selected}):await listArticles(filter);if(!visible||current!==version||!sameSession(actor)||selected!==status.value)return;rows.value=appendUniqueRows(append?rows.value:[],page.items);offset.value=start+page.items.length;total.value=page.total;failed=null}
    catch(err){if(current===version&&visible&&sameSession(actor)){if(admin&&[401,403].includes(errorStatus(err)||0)){rows.value=[];offset.value=0;total.value=0;failed=null}else failed={more:append,offset:start,query:search,status:selected};error.value=errorMessage(err)}}
    finally{if(current===version)loading.value=false}
  }
  /** 错误按钮和触底都重试原失败页，不把未提交搜索字或旧hasMore当作新查询。 */
  async function retryFailed():Promise<void>{if(!loading.value)await load(failed?.more||false,true)}
  /** 隐藏后废弃旧请求，恢复时读取最新发布状态。 */
  function hide():void{visible=false;version++;loading.value=false}
  watch(status,()=>{failed=null;if(visible)void load()},{flush:'sync'})
  watch([()=>state.user?.id,()=>state.user?.role,()=>state.user?.platform_admin_role,()=>state.user?.status,sessionRevision],()=>{clear();if(visible)void load()},{flush:'sync'})
  useNativePageRefresh({refresh:()=>load(),more:()=>load(true),context:()=>[state.user?.id,state.user?.role,state.user?.status,sessionRevision(),status.value]})
  onShow(()=>{visible=true;void load()});onHide(hide);onUnload(()=>{disposed=true;hide();clear()})
  return {rows,query,status,loading,error,total,hasMore,allowed,load,retryFailed}
}
