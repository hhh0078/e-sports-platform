/** 文件作用：联执行真实store、公共运营状态和主页面入口，验证首屏权威顺序、深链及弱网竞态。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {computed,effectScope,reactive,ref,shallowRef,watch} from 'vue'
import {executable,pageScript} from './sfc-test'
import type {PublicContent} from '../types/content'

/** 由测试显式决定一次网络读取的完成时刻。 */
interface Pending<T>{resolve:(value:T)=>void;reject:(error:unknown)=>void}
/** 为公开接口提供完整当前游戏DTO，不伪造原生图片或微信验收。 */
function content(gameID='delta',title='平台第一图',revision=1):PublicContent{return{revision,game_id:gameID,slides:[{id:'platform',kind:'image',image_url:'/api/media/platform.jpg',labels:[{text:title,font_size:24}],target:null},{id:'game-'+gameID,kind:'image',image_url:'/static/covers/'+gameID+'.jpg',labels:[],target:null}],announcement:{id:'home',version:0,enabled:false,frequency:'once',mode:'text',image_url:'',title:'',body:'',target:null},page_tips:{player:{text:'',article_id:''},worker:{text:'',article_id:''},orders:{text:'',article_id:''},me:{text:'',article_id:''}},service_notice:'',request_notice:''}}
/** 将原生页面生命周期和网络完成顺序交给测试，其余执行真实生产脚本。 */
function harness(t:TestContext,cachedGame='delta'){
  const storage=new Map([['jijie_token','token-A'],['jijie_game',cachedGame]]),writes:Array<[string,string]>=[]
  const bootstrap:Array<Pending<unknown>>=[],me:Array<Pending<unknown>>=[],reads:Array<Pending<PublicContent>&{game:string}>=[]
  const hooks={load:(_query:Record<string,string>)=>{},show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({computed,reactive,ref,shallowRef,watch,
    uni:{getStorageSync:(key:string)=>storage.get(key),setStorageSync:(key:string,value:string)=>{writes.push([key,value]);storage.set(key,value)},removeStorageSync:(key:string)=>storage.delete(key)},
    token:()=>storage.get('jijie_token')||'',errorStatus:(error:{status?:number})=>error.status,errorMessage:(error:Error)=>error.message,
    getBootstrap:()=>new Promise((resolve,reject)=>bootstrap.push({resolve,reject})),getMe:()=>new Promise((resolve,reject)=>me.push({resolve,reject})),
    getPublicContent:(game:string)=>new Promise<PublicContent>((resolve,reject)=>reads.push({game,resolve,reject})),
    mainNavigation:reactive({tab:'player'}),themeStyle:{},
    isTimelinePreview:()=>false,enablePublicShareMenu:()=>{},readSafeArea:()=>({statusBarHeight:20,capsuleSpace:44,capsuleRightInset:100}),createPublicShareHandlers:()=>({friend:()=>{},timeline:()=>{}}),
    onShareAppMessage:()=>{},onShareTimeline:()=>{},applyMainQuery:()=>{},notify:()=>{},
    onLoad:(fn:typeof hooks.load)=>{hooks.load=fn},onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},
  })
  vm.runInContext(executable(readFileSync(new URL('./store.ts',import.meta.url),'utf8')),context)
  vm.runInContext(executable(readFileSync(new URL('./public-content.ts',import.meta.url),'utf8')),context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('main/index.vue')),context));t.after(()=>scope.stop())
  vm.runInContext('globalThis.subject={state,init,initPublic,prepareEntryGame,selectGame,sessionRevision,clearSession,publicContent,publicContentLoading,publicContentError,refreshPublicContent,resetPublicContent}',context)
  return{page:context.subject,hooks,bootstrap,me,reads,storage,writes,
    /** 仅返回平台初始化所需的公开游戏字段。 */
    boot(){return{games:[{id:'delta',name:'三角洲',cover_url:'/static/covers/delta.jpg'},{id:'honor',name:'王者',cover_url:'/static/covers/honor.jpg'}],dev_mode:false}},
  }
}
/** 排空已完成的网络Promise与Vue侦听，不用固定等待伪装网络成功。 */
async function flush():Promise<void>{await new Promise<void>(resolve=>setImmediate(resolve))}

test('App先启动初始化时深链仍先于首轮轮播生效，私人接口失败不阻断公开首屏',async t=>{
  const s=harness(t),privateInit=s.page.init().catch(()=>{})
  s.hooks.load({game_id:'honor'});s.hooks.show();assert.equal(s.bootstrap.length,1);assert.equal(s.page.publicContentLoading.value,true);assert.equal(s.page.publicContent.value,null)
  s.bootstrap[0].resolve(s.boot());await flush();assert.deepEqual(s.reads.map(row=>row.game),['honor']);assert.equal(s.page.state.gameId,'honor');assert.equal(s.me.length,1);assert.equal(s.page.publicContent.value,null)
  s.me[0].reject(Object.assign(new Error('本人接口暂不可用'),{status:503}));await privateInit
  s.reads[0].resolve(content('honor'));await flush()
  assert.equal(s.page.publicContent.value.slides[0].id,'platform');assert.equal(s.page.publicContent.value.game_id,'honor');assert.equal(s.page.publicContentLoading.value,false);assert.equal(s.page.publicContentError.value,'');assert.equal(s.storage.get('jijie_game'),'delta');assert.deepEqual(s.writes,[])
})
test('首次bootstrap失败显示冷启动错误，重试仍消费原深链且不等待本人接口',async t=>{
  const s=harness(t);s.hooks.load({game_id:'honor'});s.hooks.show();s.bootstrap[0].reject(new Error('公开配置读取失败'));await flush()
  assert.equal(s.page.publicContent.value,null);assert.equal(s.page.publicContentLoading.value,false);assert.equal(s.page.publicContentError.value,'公开配置读取失败');assert.equal(s.reads.length,0)
  const retry=s.page.refreshPublicContent();assert.equal(s.page.publicContentLoading.value,true);assert.equal(s.page.publicContentError.value,'');s.bootstrap[1].resolve(s.boot());await flush()
  assert.deepEqual(s.reads.map(row=>row.game),['honor']);assert.equal(s.me.length,0);s.reads[0].resolve(content('honor'));await retry
  assert.equal(s.page.publicContent.value.game_id,'honor');assert.equal(s.storage.get('jijie_game'),'delta')
})
test('无效深链保留已启用缓存游戏，已初始化入口同步选游戏且不写偏好',async t=>{
  const s=harness(t,'honor');s.hooks.load({game_id:'missing'});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush()
  assert.deepEqual(s.reads.map(row=>row.game),['honor']);s.reads[0].resolve(content('honor'));await flush();s.hooks.hide()
  s.hooks.load({game_id:'delta'});assert.equal(s.page.state.gameId,'delta');assert.equal(s.page.publicContent.value,null);assert.equal(s.reads.length,1)
  s.hooks.show();await flush();assert.deepEqual(s.reads.map(row=>row.game),['honor','delta']);s.reads[1].resolve(content());await flush();assert.deepEqual(s.writes,[])
})
test('热刷新等值数据保留对象，失败保留已确认画面，同revision审核投影仍正确采纳',async t=>{
  const s=harness(t);s.hooks.load({});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush();s.reads[0].resolve(content());await flush()
  const original=s.page.publicContent.value,same=s.page.refreshPublicContent();assert.equal(s.page.refreshPublicContent(),same);assert.equal(s.page.publicContent.value,original);await flush();s.reads[1].resolve(content());await same;assert.equal(s.page.publicContent.value,original)
  const changed=s.page.refreshPublicContent();await flush();s.reads[2].resolve(content('delta','审核失败，请查看消息'));await changed;const safe=s.page.publicContent.value;assert.notEqual(safe,original);assert.equal(safe.revision,1);assert.equal(safe.slides[0].labels[0].text,'审核失败，请查看消息')
  const failed=s.page.refreshPublicContent(),handled=failed.catch(()=>{});await flush();s.reads[3].reject(new Error('暂时离线'));await handled
  assert.equal(s.page.publicContent.value,safe);assert.equal(s.page.publicContentLoading.value,false);assert.equal(s.page.publicContentError.value,'暂时离线')
})
test('A到B再到A弱网不让旧成功或旧错误覆盖当前游戏及加载状态',async t=>{
  const s=harness(t);s.hooks.load({});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush()
  s.page.selectGame('honor');await flush();s.page.selectGame('delta');await flush();assert.deepEqual(s.reads.map(row=>row.game),['delta','honor','delta'])
  s.reads[0].resolve(content('delta','过时第一轮A'));s.reads[1].reject(new Error('过时B错误'));await flush()
  assert.equal(s.page.publicContent.value,null);assert.equal(s.page.publicContentLoading.value,true);assert.equal(s.page.publicContentError.value,'')
  s.reads[2].resolve(content('delta','最后一轮A'));await flush();assert.equal(s.page.publicContent.value.slides[0].labels[0].text,'最后一轮A');assert.equal(s.page.publicContentLoading.value,false)
})
test('公开接口失败可重试，服务端显式空轮播不被替换成游戏封面',async t=>{
  const s=harness(t);s.hooks.load({});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush();s.reads[0].reject(new Error('运营接口读取失败'));await flush()
  assert.equal(s.page.publicContent.value,null);assert.equal(s.page.publicContentError.value,'运营接口读取失败')
  const retry=s.page.refreshPublicContent();await flush();const empty=content();empty.slides=[];s.reads[1].resolve(empty);await retry
  assert.equal(s.page.publicContent.value.slides.length,0);assert.equal(s.page.publicContentError.value,'');assert.equal(s.page.publicContentLoading.value,false)
})
test('响应游戏不匹配作为读取错误处理，不把错误投影当成当前游戏配置',async t=>{
  const s=harness(t);s.hooks.load({});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush();s.reads[0].resolve(content('honor'));await flush()
  assert.equal(s.page.publicContent.value,null);assert.match(s.page.publicContentError.value,/当前游戏不一致/);assert.equal(s.page.publicContentLoading.value,false)
})
test('热返回仍读取公共更新，旧本人请求未完成也不阻塞，隐藏切游戏不提前读取',async t=>{
  const s=harness(t);s.hooks.load({});s.hooks.show();s.bootstrap[0].resolve(s.boot());await flush();s.reads[0].resolve(content());await flush();const old=s.page.publicContent.value
  s.hooks.hide();s.hooks.show();await flush();assert.equal(s.me.length,1);assert.equal(s.reads.length,2);s.reads[1].resolve(content());await flush();assert.equal(s.page.publicContent.value,old)
  s.hooks.hide();s.page.selectGame('honor');await flush();assert.equal(s.reads.length,2);assert.equal(s.page.publicContent.value,null)
  s.hooks.show();await flush();assert.deepEqual(s.reads.map(row=>row.game),['delta','delta','honor']);s.reads[2].resolve(content('honor'));await flush();assert.equal(s.page.publicContent.value.game_id,'honor')
})
