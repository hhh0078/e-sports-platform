/** 文件作用：执行真实文章列表组合函数，验证公开请求隔离、原始游标、失败重试、筛选竞态与管理员失权清理。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {computed,effectScope,reactive,ref,watch} from 'vue'
import {executable,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest} from './sfc-test'
import type {ArticleSummary} from '../types/content'
import type {NativePageRefreshOptions} from './native-page-refresh'

/** 只控制HTTP与页面生命周期，真实领域API和文章分页逻辑保持执行。 */
function harness(t:TestContext,admin=true){
  const state=reactive({ready:true,user:{id:'admin-A',role:'admin',platform_admin_role:'root',status:'active'}}),requests:DeferredRequest[]=[],publicCalls:string[]=[],privateCalls:string[]=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{}};let revision=0,initialize=async()=>{},native:NativePageRefreshOptions|undefined
  const context=vm.createContext({ref,computed,watch,state,init:()=>initialize(),sessionRevision:()=>revision,token:()=>state.user.id+'token',onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},useNativePageRefresh:(options:NativePageRefreshOptions)=>{native=options;return{}},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);context.requestPublic=(path:string)=>{publicCalls.push(path);return context.api(path)};context.request=(path:string,method:string,body:Record<string,unknown>)=>{privateCalls.push(path);return context.api(path,method,body)}
  vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8')),context)
  const effect=effectScope();effect.run(()=>vm.runInContext(executable(readFileSync(new URL('./use-articles.ts',import.meta.url),'utf8'))+';globalThis.page=useArticles('+String(admin)+')',context));t.after(()=>effect.stop())
  return{page:context.page,state,hooks,requests,publicCalls,privateCalls,
    /** 当前真实搜索请求及其编码后的参数。 */
    take(){const request=requests.find(item=>!item.taken);assert.ok(request,'缺少文章列表请求');request.taken=true;return{...request,params:new URL(request.path,'https://test.local').searchParams}},
    /** 模拟官方原生触底，不直接替换分页业务函数。 */
    async bottom(){await native?.more?.()},
    /** 控制初始化完成时机，核验离开后的请求守卫。 */
    setInit(callback:()=>Promise<void>){initialize=callback},
    /** 真实会话修订触发账号隔离。 */
    switchUser(){revision++;state.user.id='admin-B'},
  }
}
/** 已发布文章摘要只包含列表允许展示的字段。 */
function row(id:string):ArticleSummary{return{id,title:'文章'+id,summary:'',cover_url:'',status:'published',revision:1,created_at:'',updated_at:'',published_at:''}}
/** 排空真实侦听和Promise回调。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 初始化第一页真实服务响应。 */
async function ready(s:ReturnType<typeof harness>):Promise<void>{s.hooks.show();await flush();s.take().resolve({items:Array.from({length:20},(_,i)=>row('id-'+i)),total:50});await flush()}

test('公开列表即使本机管理员也只调用public API，不请求草稿接口',async t=>{
  const s=harness(t,false);await ready(s);assert.equal(s.publicCalls.length,1);assert.equal(s.privateCalls.length,0);assert.ok(s.publicCalls[0].startsWith('/articles?'));assert.equal(s.page.rows.value.length,20)
})

test('追加失败由真实重试入口保留offset和submitted，去重后仍用原始条数推进',async t=>{
  const s=harness(t);await ready(s);s.page.query.value='未提交搜索';const more=s.page.load(true);await flush();const failed=s.take();assert.equal(failed.params.get('offset'),'20');failed.reject(new Error('追加失败'));await more;assert.equal(s.page.rows.value.length,20)
  const retry=s.page.retryFailed();await flush();const reading=s.take();assert.equal(reading.params.get('offset'),'20');assert.equal(reading.params.get('q'),null);reading.resolve({items:Array.from({length:20},(_,i)=>row('id-'+(i+19))),total:50});await retry;assert.equal(s.page.rows.value.length,39)
  const next=s.page.load(true);await flush();const nextRead=s.take();assert.equal(nextRead.params.get('offset'),'40');nextRead.resolve({items:[],total:39});await next
  assert.match(readFileSync(new URL('../components/ArticleListView.vue',import.meta.url),'utf8'),/@retry="retryFailed"/)
})

test('刷新失败后原生触底重试第一页，不依据旧hasMore跳过失败页',async t=>{
  const s=harness(t);await ready(s);s.page.query.value='正式查询';const refresh=s.page.load();await flush();s.take().reject(new Error('刷新失败'));await refresh;s.page.query.value='未提交的新词'
  const bottom=s.bottom();await flush();const retry=s.take();assert.equal(retry.params.get('offset'),'0');assert.equal(retry.params.get('q'),'正式查询');retry.resolve({items:[row('fresh')],total:1});await bottom;assert.equal(s.page.rows.value[0].id,'fresh');assert.equal(s.page.rows.value.length,1)
})

test('状态筛选和切号使旧响应失效，旧finally不结束新请求',async t=>{
  const s=harness(t);s.hooks.show();await flush();const old=s.take();s.page.status.value='draft';await flush();const draft=s.take();assert.equal(draft.params.get('status'),'draft');old.resolve({items:[row('old')],total:1});await flush();assert.equal(s.page.loading.value,true);assert.equal(s.page.rows.value.length,0)
  s.switchUser();await flush();const fresh=s.take();draft.resolve({items:[row('admin-A')],total:1});await flush();assert.equal(s.page.loading.value,true);fresh.resolve({items:[row('admin-B')],total:1});await flush();assert.equal(s.page.rows.value[0].id,'admin-B')
})

test('管理员角色失权、冻结与HTTP403立即撤下私有文章，公开网络失败保留列表',async t=>{
  const s=harness(t);await ready(s);s.state.user.role='user';await flush();assert.equal(s.page.rows.value.length,0);assert.equal(s.requests.length,1)
  s.state.user.role='admin';s.state.user.status='frozen';await flush();assert.equal(s.requests.length,1);s.state.user.status='active';await flush();s.take().resolve({items:[row('draft')],total:1});await flush();const denied=s.page.load();await flush();s.take().reject({status:403,message:'文章权限已撤销'});await denied;assert.equal(s.page.rows.value.length,0);assert.equal(s.page.error.value,'文章权限已撤销')
  const publicList=harness(t,false);await ready(publicList);const failed=publicList.page.load();await flush();publicList.take().reject(new Error('网络失败'));await failed;assert.equal(publicList.page.rows.value.length,20)
})

test('初始化期间隐藏不再请求，已发送响应在隐藏或卸载后不回填',async t=>{
  const cold=harness(t);let finish=()=>{};cold.setInit(()=>new Promise<void>(resolve=>{finish=resolve}));cold.hooks.show();cold.hooks.hide();finish();await flush();assert.equal(cold.requests.length,0)
  const hidden=harness(t);hidden.hooks.show();await flush();const read=hidden.take();hidden.hooks.hide();read.resolve({items:[row('hidden')],total:1});await flush();assert.equal(hidden.page.rows.value.length,0)
  const disposed=harness(t);disposed.hooks.show();await flush();const previous=disposed.take();disposed.hooks.unload();previous.resolve({items:[row('disposed')],total:1});await flush();assert.equal(disposed.page.rows.value.length,0)
})
