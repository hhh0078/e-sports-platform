/** 文件作用：执行真实官方排序组件、白名单跳转与共享运营读取，验证手势边界和跨游戏旧响应隔离。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {computed,effectScope,nextTick,reactive,ref,shallowRef,watch} from 'vue'
import {executable} from './sfc-test'
import type {ContentSlide} from '../types/content'

/** 只在测试中隔离官方窗口读取和组件事件，排序逻辑执行真实SFC。 */
function sorting(t:TestContext,width=390){
  const initial:ContentSlide[]=['a','game','b'].map(id=>({id,kind:id==='game'?'game_slot':'image',image_url:'',labels:[],target:null})),props=reactive({modelValue:initial}),events:Array<{name:string;rows:ContentSlide[]}>=[]
  const context=vm.createContext({computed,ref,watch,defineProps:()=>props,defineEmits:()=>((name:string,rows:ContentSlide[])=>events.push({name,rows})),useViewportWidth:()=>ref(width)})
  const source=readFileSync(new URL('../components/SortSlideList.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(source)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(source)+';globalThis.sort={begin,moved,finish,cancel,step,dragIndex,rowPixels,layoutVersion}',context));t.after(()=>scope.stop())
  return{page:context.sort,props,events,initial}
}
/** 将跨VM数组转为只含ID的稳定顺序文本。 */
function ids(rows:ContentSlide[]):string{return Array.from(rows,item=>item.id).join(',')}

test('官方拖动在320/390真实像素坐标松手后只发一次顺序，游戏槽保留',async t=>{
  for(const width of [320,390])await t.test(String(width),child=>{const s=sorting(child,width);s.page.begin(0);s.page.moved({detail:{source:'touch',y:s.page.rowPixels.value*2}});assert.equal(s.events.length,0);s.page.finish();s.page.finish();assert.equal(s.events.length,1);assert.equal(s.events[0].name,'update:modelValue');assert.equal(ids(s.events[0].rows),'game,b,a');assert.equal(ids(s.initial),'a,game,b');assert.equal(s.events[0].rows[0].kind,'game_slot')})
})
test('排序忽略动画坐标、取消拖动及未长按的普通滚动',t=>{
  const s=sorting(t);s.page.moved({detail:{source:'touch',y:9999}});s.page.finish();assert.equal(s.events.length,0);s.page.begin(0);s.page.moved({detail:{source:'animation',y:9999}});s.page.finish();assert.equal(s.events.length,0);s.page.begin(1);s.page.moved({detail:{source:'touch',y:0}});s.page.cancel();s.page.finish();assert.equal(s.events.length,0);assert.equal(ids(s.props.modelValue),'a,game,b')
})
test('排序非法索引和无效坐标不会把末项splice到首行，拖动越界安全夹紧',t=>{
  const s=sorting(t);for(const index of [-1,3,1.5,NaN,Infinity]){s.page.begin(index);s.page.finish();s.page.step(index,1)}s.page.step(0,-1);s.page.step(2,1);s.page.step(0,NaN);s.page.step(0,2);assert.equal(s.events.length,0)
  s.page.begin(2);s.page.moved({detail:{source:'touch',y:NaN}});s.page.finish();assert.equal(s.events.length,0);s.page.begin(2);s.page.moved({detail:{source:'touch',y:-9999}});s.page.finish();assert.equal(ids(s.events[0].rows),'b,a,game')
})
test('上移下移提交独立新数组，外部改稿与辅助移动会撤销旧拖动',async t=>{
  const s=sorting(t);s.page.step(1,-1);assert.equal(ids(s.events[0].rows),'game,a,b');assert.equal(ids(s.initial),'a,game,b');s.page.begin(0);s.page.moved({detail:{source:'touch',y:9999}});s.page.step(0,1);s.page.finish();assert.equal(s.events.length,2);assert.equal(ids(s.events[1].rows),'game,a,b')
  s.page.begin(1);s.props.modelValue=[{id:'new',kind:'image',image_url:'',labels:[],target:null}];s.page.finish();await nextTick();assert.equal(s.events.length,2);assert.equal(s.page.dragIndex.value,-1)
})

/** 直接执行真实白名单工具，导航只记录用户明确点击的合法站内路径。 */
function targets(){const navigations:string[]=[],context=vm.createContext({go:(value:string)=>navigations.push(value)});vm.runInContext(executable(readFileSync(new URL('./content.ts',import.meta.url),'utf8'))+';globalThis.targets={contentTargetPath,openContentTarget}',context);return{page:context.targets,navigations}}
test('运营跳转仅支持服务/需求/文章三种白名单，ID安全编码',()=>{
  const s=targets();for(const [kind,path]of [['service','services'],['request','requests'],['article','content/articles']]){const target={kind,id:'目标-1'};assert.equal(s.page.contentTargetPath(target),'/pages/'+path+'/detail?id='+encodeURIComponent(target.id));s.page.openContentTarget(target)}assert.equal(s.navigations.length,3)
})
test('异常类型、原型键、任意URL及无效资源ID不产生导航',()=>{
  const s=targets();for(const kind of ['constructor','__proto__','toString','url','https://example.com','',0,null]){assert.equal(s.page.contentTargetPath({kind,id:'id'}),'');s.page.openContentTarget({kind,id:'id'})}for(const id of ['',null,1,{},' ','a/b','a\\b','a?b','a#b','a\nb','demo_fake','a'.repeat(129),'\u0000']){assert.equal(s.page.contentTargetPath({kind:'article',id}),'');s.page.openContentTarget({kind:'article',id})}assert.equal(s.page.contentTargetPath(null),'');assert.equal(s.page.contentTargetPath(undefined),'');assert.equal(s.navigations.length,0)
})

/** 只控制公开GET完成顺序，真实共享模块负责同游戏合并与版本隔离。 */
function publicReads(){const state=reactive({gameId:'delta'}),requests:Array<{game:string;resolve:(value:unknown)=>void;reject:(error:unknown)=>void}>=[],context=vm.createContext({ref,shallowRef,state,initPublic:async()=>{},errorMessage:(error:Error)=>error.message,getPublicContent:(game:string)=>new Promise((resolve,reject)=>requests.push({game,resolve,reject}))});vm.runInContext(executable(readFileSync(new URL('./public-content.ts',import.meta.url),'utf8'))+';globalThis.public={publicContent,refreshPublicContent,resetPublicContent}',context);return{state,requests,page:context.public}}
/** 排空跨VM的公开初始化Promise，网络本身仍由测试显式完成。 */
async function flushPublicReads():Promise<void>{await new Promise<void>(resolve=>setImmediate(resolve))}
test('同游戏共享在途读取，A→B→A只显示最后一次当前游戏响应',async()=>{
  const s=publicReads(),first=s.page.refreshPublicContent();assert.equal(s.page.refreshPublicContent(),first);await flushPublicReads();s.state.gameId='honor';const middle=s.page.refreshPublicContent();await flushPublicReads();s.state.gameId='delta';const last=s.page.refreshPublicContent();await flushPublicReads();assert.equal(s.requests.length,3);s.requests[0].resolve({game_id:'delta',revision:1});await first;assert.equal(s.page.publicContent.value,null);s.requests[1].resolve({game_id:'honor',revision:2});await middle;assert.equal(s.page.publicContent.value,null);s.requests[2].resolve({game_id:'delta',revision:3});await last;assert.equal(s.page.publicContent.value.revision,3)
})
test('旧游戏失败的finally不清除新游戏在途，reset后迟到成功不复活旧图',async()=>{
  const s=publicReads(),old=s.page.refreshPublicContent(),handled=old.catch(()=>{});await flushPublicReads();s.state.gameId='honor';const current=s.page.refreshPublicContent();await flushPublicReads();s.requests[0].reject(new Error('旧游戏失败'));await handled;assert.equal(s.page.refreshPublicContent(),current);s.page.resetPublicContent();s.requests[1].resolve({game_id:'honor',revision:2});await current;assert.equal(s.page.publicContent.value,null);const retry=s.page.refreshPublicContent();await flushPublicReads();assert.equal(s.requests.length,3);s.requests[2].resolve({game_id:'honor',revision:3});await retry;assert.equal(s.page.publicContent.value.revision,3)
})
