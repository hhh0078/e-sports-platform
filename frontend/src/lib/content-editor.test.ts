/** 文件作用：执行真实公共信息编辑页及领域API，验证全栏保存、排序作用域、原生公告上传、版本冲突与管理员异步失效边界。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {computed,effectScope,ref,watch,reactive} from 'vue'
import {executable,injectBusinessAPIs,pageScript} from './sfc-test'
import type {DeferredRequest,DeferredUpload} from './sfc-test'
import type {AdminPublicContent} from '../types/content'
import {IMAGE_RULES} from './media'

/** 与服务端一致的新配置，仅提供真实配置字段，不使用业务演示资金或身份。 */
function config(revision=3):AdminPublicContent{return{revision,global_slides:[{id:'A',kind:'image',image_url:'/api/media/A.jpg',labels:[],target:null},{id:'slot',kind:'game_slot',image_url:'',labels:[],target:null}],game_slides:{delta:[{id:'delta-A',kind:'image',image_url:'/api/media/delta.jpg',labels:[],target:null}],honor:[]},announcement:{id:'home',version:2,enabled:true,frequency:'once',mode:'text',image_url:'',title:'运营公告',body:'原始正文',target:null},page_tips:{player:{text:'玩家提示',article_id:''},worker:{text:'',article_id:''},orders:{text:'',article_id:''},me:{text:'',article_id:''}},service_notice:'原服务事项',request_notice:'原需求事项',updated_at:''}}
/** 执行真实编辑脚本，只控制HTTP、原生选图和确认时序。 */
function harness(t:TestContext,role='admin'){
  const state=reactive({ready:true,user:{id:'admin-A',role,platform_admin_role:'root',status:'active'},games:[{id:'delta',name:'三角洲行动'}]}),requests:DeferredRequest[]=[],uploads:DeferredUpload[]=[],notices:unknown[]=[],paths:string[]=[],copies:string[]=[],dialogs:Array<{resolve:(value:{confirm:boolean})=>void}>=[]
  const hooks={show:()=>{},hide:()=>{},unload:()=>{},load:(_options:Record<string,string>)=>{}};let revision=0,initialize=async()=>{}
  const context=vm.createContext({computed,ref,watch,reactive,state,init:()=>initialize(),sessionRevision:()=>revision,token:()=>state.user.id+'token',assetUrl:(value:string)=>value,notify:(value:unknown)=>notices.push(value),go:(value:string)=>paths.push(value),onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},onLoad:(fn:typeof hooks.load)=>{hooks.load=fn},uni:{showModal:()=>new Promise(resolve=>dialogs.push({resolve})),setClipboardData:({data}:{data:string})=>copies.push(data)},chooseUploadImage(purpose:string,options?:object){return new Promise((resolve,reject)=>uploads.push({purpose,options,resolve,reject}))},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);vm.runInContext(executable(readFileSync(new URL('./content.ts',import.meta.url),'utf8')),context);vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8')),context)
  const effect=effectScope();effect.run(()=>vm.runInContext(executable(pageScript('manage/content/index.vue'))+';globalThis.page={draft,tab,selectedGame,slides,loaded,loading,busy,error,dirty,allowed,load,reload,copyDraft,save,setSlides,removeSlide,addGameSlot,editSlide,setTipArticle,uploadAnnouncement}',context));t.after(()=>effect.stop())
  return{state,page:context.page,hooks,requests,uploads,notices,paths,copies,dialogs,
    /** 只读取实际发出的配置领域请求。 */
    take(method='GET'){const request=requests.find(row=>!row.taken&&row.path==='/admin/public-content'&&row.method===method);assert.ok(request,'缺少'+method+'公共信息请求');request.taken=true;return request},
    /** 切号触发真实状态清理，不手动更改页面内部代次。 */
    switchUser(){revision++;state.user.id='admin-B'},
    /** 挂起初始化用于复现隐藏后才完成登录恢复的真实竞态。 */
    setInit(callback:()=>Promise<void>){initialize=callback},
  }
}
/** 排空响应与Vue侦听。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 完成真实管理员配置读取。 */
async function ready(s:ReturnType<typeof harness>,value=config()):Promise<void>{s.hooks.show();await flush();s.take().resolve(value);await flush()}

test('公共信息跨栏目草稿一起保存，保留多行内容并只发送契约字段',async t=>{
  const s=harness(t);await ready(s);s.page.draft.value.announcement.title='新的公告';s.page.draft.value.service_notice='第一行\n第二行';s.page.draft.value.request_notice='需求\n范围';s.page.tab.value='tips';s.page.draft.value.page_tips.worker.text='新的提示\n第二行'
  const saving=s.page.save(),writing=s.take('PATCH');assert.deepEqual(Object.keys(writing.body).sort(),['announcement','game_slides','global_slides','page_tips','request_notice','revision','service_notice']);assert.equal(writing.body.service_notice,'第一行\n第二行');assert.equal((writing.body.announcement as AdminPublicContent['announcement']).title,'新的公告')
  const saved=config(4);saved.announcement.title='新的公告';writing.resolve(saved);assert.equal(await saving,true);assert.equal(s.page.dirty.value,false);assert.equal(s.page.draft.value.revision,4)
})

test('平台和各游戏顺序独立，游戏槽唯一，顶部提示仅接受文章目标',async t=>{
  const s=harness(t);await ready(s);s.page.addGameSlot();assert.equal(s.page.slides.value.filter((row:{kind:string})=>row.kind==='game_slot').length,1)
  s.page.selectedGame.value='delta';const global=JSON.stringify(s.page.draft.value.global_slides);s.page.setSlides([{id:'delta-B',kind:'image',image_url:'/api/media/B.jpg',labels:[],target:null},...s.page.slides.value]);assert.equal(s.page.draft.value.game_slides.delta[0].id,'delta-B');assert.equal(JSON.stringify(s.page.draft.value.global_slides),global);assert.equal(s.page.draft.value.game_slides.honor.length,0)
  s.page.removeSlide(0);assert.equal(s.page.slides.value[0].id,'delta-A');s.page.setTipArticle('player',{kind:'article',id:'guide'});assert.equal(s.page.draft.value.page_tips.player.article_id,'guide');s.page.setTipArticle('player',{kind:'service',id:'service-A'});assert.equal(s.page.draft.value.page_tips.player.article_id,'');s.page.setTipArticle('me',null);assert.equal(s.page.draft.value.page_tips.me.article_id,'')
})

test('409保留全栏草稿，复制与取消重读不修改原稿，明确确认才替换',async t=>{
  const s=harness(t);await ready(s);s.page.draft.value.announcement.body='本地多行\n原稿';const saving=s.page.save();s.take('PATCH').reject({status:409,message:'版本冲突'});await saving;assert.match(s.page.error.value,/草稿已保留/);assert.equal(s.page.draft.value.announcement.body,'本地多行\n原稿');s.page.copyDraft();assert.equal(JSON.parse(s.copies[0]).announcement.body,'本地多行\n原稿')
  const cancelled=s.page.reload();s.dialogs[0].resolve({confirm:false});await cancelled;assert.equal(s.requests.length,2);assert.equal(s.page.draft.value.announcement.body,'本地多行\n原稿')
  const reloading=s.page.reload();s.dialogs[1].resolve({confirm:true});await flush();s.take().resolve(config(9));await reloading;assert.equal(s.page.draft.value.revision,9);assert.equal(s.page.dirty.value,false)
})

test('公告走保留原比例官方用途，选图取消与隐藏返回均保留当前草稿',async t=>{
  assert.equal(IMAGE_RULES.announcement_image.ratio,undefined);assert.equal(IMAGE_RULES.promotion_banner.ratio,'16:9')
  const s=harness(t);await ready(s);s.page.draft.value.announcement.body='未保存正文';const uploading=s.page.uploadAnnouncement();assert.equal(s.uploads[0].purpose,'announcement_image');await s.page.save();assert.equal(s.requests.length,1);s.hooks.hide();s.hooks.show();await flush();s.uploads[0].resolve('/api/media/portrait.jpg');await uploading;assert.equal(s.page.draft.value.announcement.image_url,'/api/media/portrait.jpg');assert.equal(s.page.draft.value.announcement.body,'未保存正文')
  const cancelled=s.page.uploadAnnouncement();s.uploads[1].reject({cancelled:true});await cancelled;assert.equal(s.page.draft.value.announcement.image_url,'/api/media/portrait.jpg');assert.equal(s.notices.length,0)
})

test('隐藏及卸载作废在途读取，初始化期间离开不再发私有GET',async t=>{
  const s=harness(t);s.hooks.show();await flush();const old=s.take();s.hooks.hide();old.resolve(config(9));await flush();assert.equal(s.page.loaded.value,false)
  const cold=harness(t);let finish=()=>{};cold.setInit(()=>new Promise<void>(resolve=>{finish=resolve}));cold.hooks.show();cold.hooks.hide();finish();await flush();assert.equal(cold.requests.length,0)
  const unload=harness(t);unload.hooks.show();await flush();const reading=unload.take();unload.hooks.unload();reading.resolve(config(8));await flush();assert.equal(unload.page.loaded.value,false)
})

test('普通用户和冻结管理员不请求私有配置，角色失权与403立即清私有草稿',async t=>{
  const user=harness(t,'user');user.hooks.show();await flush();assert.equal(user.requests.length,0)
  const frozen=harness(t);frozen.state.user.status='frozen';frozen.hooks.show();await flush();assert.equal(frozen.requests.length,0)
  const revoked=harness(t);await ready(revoked);revoked.state.user.role='user';await flush();assert.equal(revoked.page.loaded.value,false);assert.equal(revoked.page.draft.value.announcement.body,'')
  const denied=harness(t);await ready(denied);const saving=denied.page.save();denied.take('PATCH').reject({status:403,message:'权限已撤销'});await saving;assert.equal(denied.page.loaded.value,false);assert.equal(denied.page.draft.value.announcement.body,'')
})

test('切号后旧上传不回填，新管理员可读取；旧finally不能解锁新管理员保存',async t=>{
  const s=harness(t);await ready(s);const uploading=s.page.uploadAnnouncement();s.switchUser();await flush();s.take().resolve(config(6));await flush();s.uploads[0].resolve('/api/media/admin-A.jpg');await uploading;assert.equal(s.page.draft.value.revision,6);assert.equal(s.page.draft.value.announcement.image_url,'')
  const first=s.page.save(),old=s.take('PATCH');s.switchUser();s.state.user.id='admin-C';await flush();s.take().resolve(config(7));await flush();const second=s.page.save(),fresh=s.take('PATCH');old.resolve(config(8));await first;assert.equal(s.page.busy.value,true);fresh.resolve(config(9));await second;assert.equal(s.page.busy.value,false);assert.equal(s.page.draft.value.revision,9)
})

test('旧账号重读或编辑确认不能覆盖新草稿，也不能为新账号保存导航',async t=>{
  const s=harness(t);await ready(s);s.page.draft.value.service_notice='A草稿';const reloading=s.page.reload();assert.equal(s.dialogs.length,1);s.switchUser();await flush();s.take().resolve(config(7));await flush();s.page.draft.value.service_notice='B草稿';const count=s.requests.length;s.dialogs[0].resolve({confirm:true});await reloading;await flush();assert.equal(s.requests.length,count);assert.equal(s.page.draft.value.service_notice,'B草稿')
  const editing=s.page.editSlide(0);s.state.user.id='admin-C';await flush();s.take().resolve(config(8));await flush();s.dialogs[1].resolve({confirm:true});await editing;assert.equal(s.requests.filter(row=>row.method==='PATCH').length,0);assert.equal(s.paths.length,0)
})
