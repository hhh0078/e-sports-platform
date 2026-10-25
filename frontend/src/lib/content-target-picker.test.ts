/** 文件作用：执行真实目标选择器，验证类型白名单、失败页重试、已提交查询、页面隐藏与管理员撤权的迟到隔离。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {effectScope,reactive,ref,watch} from 'vue'
import {executable,injectBusinessAPIs} from './sfc-test'
import type {DeferredRequest} from './sfc-test'
import type {ContentTarget,ContentTargetOption} from '../types/content'

/** 创建受控网络和官方生命周期，保留组件实际分页及领域API。 */
function harness(t:TestContext,kinds:ContentTarget['kind'][]=['service','request','article']){
  const props=reactive<{modelValue:ContentTarget|null;kinds:ContentTarget['kind'][]}>({modelValue:null,kinds}),state=reactive({user:{id:'admin-A',role:'admin',platform_admin_role:'root',status:'active'}}),requests:DeferredRequest[]=[],emits:Array<{event:string;value:ContentTarget|null}>=[]
  const hooks={show:()=>{},hide:()=>{},unmount:()=>{}};let revision=0
  const context=vm.createContext({ref,watch,state,sessionRevision:()=>revision,token:()=>state.user.id+'token',defineProps:()=>props,withDefaults:(value:unknown)=>value,defineEmits:()=>((event:string,value:ContentTarget|null)=>emits.push({event,value})),onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onBeforeUnmount:(fn:()=>void)=>{hooks.unmount=fn},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8')),context)
  const script=readFileSync(new URL('../components/ContentTargetPicker.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(script)
  const effect=effectScope();effect.run(()=>vm.runInContext(executable(script)+';globalThis.page={opened,kind,query,results,offset,total,loading,error,begin,close,load,retryFailed,chooseKind,select,clear}',context));t.after(()=>{hooks.unmount();effect.stop()})
  return{page:context.page,props,state,hooks,requests,emits,
    /** 读取真实搜索请求与URL查询参数。 */
    take(){const request=requests.find(item=>!item.taken);assert.ok(request,'缺少目标搜索请求');request.taken=true;return{...request,params:new URL(request.path,'https://test.local').searchParams}},
    /** 使用真实角色修订使旧弹层失效。 */
    switchUser(){revision++;state.user.id='admin-B'},
  }
}
/** 合法公开候选，不附带手机号或交易秘密。 */
function row(id:string,kind:ContentTarget['kind']='service'):ContentTargetOption{return{kind,id,title:'目标'+id,image_url:'',game_id:'delta',subtitle:''}}
/** 排空Promise和真实侦听。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}

test('文章专用弹层只查文章，选择只发kind/id，关闭后旧点击不再提交',async t=>{
  const s=harness(t,['article']);s.props.modelValue={kind:'service',id:'old'};s.page.begin();const read=s.take();assert.equal(read.params.get('kind'),'article');read.resolve({items:[row('guide','article')],total:1});await flush();s.page.chooseKind('service');assert.equal(s.requests.length,1)
  s.page.select(row('guide','article'));assert.equal(s.emits.length,1);assert.deepEqual({...s.emits[0].value},{kind:'article',id:'guide'});s.page.select(row('guide','article'));s.page.clear();assert.equal(s.emits.length,1)
})

test('追加失败保留原offset与已提交query，重复触底只查一次，原始数量推进游标',async t=>{
  const s=harness(t);s.page.begin();s.take().resolve({items:Array.from({length:20},(_,i)=>row('id-'+i)),total:50});await flush();s.page.query.value='尚未提交的新词'
  const more=s.page.load(true);await s.page.load(true);const failed=s.take();assert.equal(failed.params.get('offset'),'20');assert.equal(failed.params.get('q'),null);failed.reject(new Error('网络失败'));await more;assert.equal(s.page.results.value.length,20)
  const retry=s.page.retryFailed(),reading=s.take();assert.equal(reading.params.get('offset'),'20');assert.equal(reading.params.get('q'),null);reading.resolve({items:Array.from({length:20},(_,i)=>row('id-'+(i+19))),total:50});await retry;assert.equal(s.page.results.value.length,39);assert.equal(s.page.offset.value,40)
})

test('刷新失败后触底重试第一页，输入草稿不会替换原失败查询',async t=>{
  const s=harness(t);s.page.begin();s.take().resolve({items:Array.from({length:20},(_,i)=>row('id-'+i)),total:50});await flush();s.page.query.value='已提交';const search=s.page.load();s.take().reject(new Error('搜索失败'));await search;s.page.query.value='另一个未提交词'
  const more=s.page.load(true),reading=s.take();assert.equal(reading.params.get('offset'),'0');assert.equal(reading.params.get('q'),'已提交');reading.resolve({items:[row('fresh')],total:1});await more;assert.equal(s.page.results.value.length,1);assert.equal(s.page.results.value[0].id,'fresh')
})

test('切类型、隐藏和切号拒收旧页，旧finally不能解锁新弹层搜索',async t=>{
  const s=harness(t);s.page.begin();const old=s.take();s.page.chooseKind('article');const fresh=s.take();old.resolve({items:[row('old')],total:1});await flush();assert.equal(s.page.loading.value,true);assert.equal(s.page.results.value.length,0)
  s.hooks.hide();fresh.resolve({items:[row('hidden','article')],total:1});await flush();assert.equal(s.page.opened.value,false);assert.equal(s.page.results.value.length,0);s.page.begin();assert.equal(s.requests.length,2)
  s.hooks.show();s.page.begin();const previous=s.take();s.switchUser();s.page.begin();const current=s.take();previous.resolve({items:[row('admin-A')],total:1});await flush();assert.equal(s.page.loading.value,true);current.resolve({items:[row('admin-B')],total:1});await flush();assert.equal(s.page.results.value[0].id,'admin-B')
})

test('冻结、角色撤销与服务器403都撤下私有候选，卸载不再搜索或提交',async t=>{
  const s=harness(t);s.state.user.status='frozen';s.page.begin();assert.equal(s.requests.length,0);s.state.user.status='active';s.page.begin();const pending=s.take();s.state.user.role='user';pending.resolve({items:[row('private')],total:1});await flush();assert.equal(s.page.opened.value,false);assert.equal(s.page.results.value.length,0);s.page.select(row('private'));assert.equal(s.emits.length,0)
  s.state.user.role='admin';s.page.begin();const denied=s.take();denied.reject({status:403,message:'无管理权限'});await flush();assert.equal(s.page.opened.value,false);assert.equal(s.page.results.value.length,0)
  s.hooks.unmount();s.page.begin();assert.equal(s.requests.length,2)
})
