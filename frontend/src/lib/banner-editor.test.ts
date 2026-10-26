/** 文件作用：执行真实轮播编辑页及领域API，验证独立文案、数组隔离、版本核对、原生上传与管理员请求失效边界。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {computed,effectScope,reactive,ref,watch} from 'vue'
import {executable,injectBusinessAPIs,pageScript} from './sfc-test'
import type {DeferredRequest,DeferredUpload} from './sfc-test'
import {newLabel,normalizeLabelText,POSITIONS} from './cover'
import type {AdminPublicContent,ContentSlide} from '../types/content'

/** 每次返回全新的服务端配置，防止用例共享编辑引用。 */
function config(revision=3):AdminPublicContent{return{revision,global_slides:[{id:'global-A',kind:'image',image_url:'/api/media/global-A.jpg',labels:[{id:'l-A',text:'开机即组队',color:'#ffffff',background:'transparent',position:'bottom-left',font_size:16}],target:{kind:'service',id:'service-A'}},{id:'slot',kind:'game_slot',image_url:'',labels:[],target:null},{id:'global-B',kind:'image',image_url:'/api/media/global-B.jpg',labels:[{text:'其它原文',color:'#ffffff',background:'transparent'}],target:null}],game_slides:{delta:[{id:'delta-A',kind:'image',image_url:'/api/media/delta.jpg',labels:[],target:null}],honor:[{id:'honor-A',kind:'image',image_url:'/api/media/honor.jpg',labels:[],target:null}]},announcement:{id:'home',version:1,enabled:false,frequency:'once',mode:'text',image_url:'',title:'',body:'',target:null},page_tips:{player:{text:'',article_id:''},worker:{text:'',article_id:''},orders:{text:'',article_id:''},me:{text:'',article_id:''}},service_notice:'已有交易提醒',request_notice:'已有需求提醒',updated_at:''}}
/** 只替代HTTP、官方选图与页面生命周期，其余执行实际SFC和content.api。 */
function harness(t:TestContext,options:{game?:string;id?:string;role?:string}={}){
  const state=reactive({ready:true,user:{id:'admin-A',role:options.role||'admin',platform_admin_role:'root',status:'active'},gameId:'delta',games:[{id:'delta',name:'三角洲行动',primary_color:'#83e4b8'}]}),requests:DeferredRequest[]=[],uploads:DeferredUpload[]=[],notices:unknown[]=[]
  const hooks={load:(_value:Record<string,string>)=>{},show:()=>{},hide:()=>{},unload:()=>{}};let revision=0,backs=0
  const context=vm.createContext({ref,reactive,computed,watch,state,newLabel,normalizeLabelText,POSITIONS,init:async()=>{},sessionRevision:()=>revision,token:()=>state.user.id+'token',notify:(value:unknown)=>notices.push(value),back:()=>{backs++},onLoad:(callback:typeof hooks.load)=>{hooks.load=callback},onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},chooseUploadImage(purpose:string,options?:object){return new Promise((resolve,reject)=>uploads.push({purpose,options,resolve,reject}))},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context);vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8')),context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('manage/content/banner.vue'))+';globalThis.page={form,gameId,slideId,loaded,loading,busy,uploading,checking,error,conflict,missing,reviewedSlide,previewLabels,load,save,saveBody,uploadImage,recheckLatest,addLabel,removeLabel,cancel,safeLabels}',context));t.after(()=>scope.stop());hooks.load({game_id:options.game||'',id:options.id??'global-A'})
  return{state,page:context.page,hooks,requests,uploads,notices,backs:()=>backs,
    /** 取得确实由领域API发出的请求，测试不能补造不存在的行为。 */
    take(method='GET'){const request=requests.find(row=>!row.taken&&row.path==='/admin/public-content'&&row.method===method);assert.ok(request,'缺少'+method+'配置请求');request.taken=true;return request},
    /** 响应式切号触发真实草稿清理与新账号初始化。 */
    switchUser(){revision++;state.user.id='admin-B'},
  }
}
/** 排空真实响应式侦听和Promise，不等待任意动画时长。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 完成一次真实完整配置读取。 */
async function ready(s:ReturnType<typeof harness>,value=config()):Promise<void>{s.hooks.show();await flush();s.take().resolve(value);await flush()}

test('平台单项保存只PATCH当前完整数组，独立标签不修改其它项或游戏',async t=>{
  const s=harness(t),original=config();await ready(s,original);s.page.form.labels[0].text='新的独立文字';s.page.form.target={kind:'article',id:'guide'}
  assert.equal(original.global_slides[0].labels[0].text,'开机即组队');const saving=s.page.save(),writing=s.take('PATCH');assert.deepEqual(Object.keys(writing.body).sort(),['global_slides','revision'])
  const list=writing.body.global_slides as ContentSlide[];assert.equal(list.length,3);assert.equal(list[0].image_url,'/api/media/global-A.jpg');assert.equal(list[0].labels[0].text,'新的独立文字');assert.deepEqual({...list[0].target},{kind:'article',id:'guide'});assert.deepEqual(JSON.parse(JSON.stringify(list.slice(1))),original.global_slides.slice(1))
  writing.resolve(config(4));await saving;assert.equal(s.backs(),1)
})

test('游戏轮播只传指定game键，九种位置、颜色、字号均经共享归一化独立保存',async t=>{
  const s=harness(t,{game:'delta',id:'delta-A'});await ready(s);for(let i=0;i<7;i++)s.page.addLabel();assert.equal(s.page.form.labels.length,6)
  for(const [index,position] of POSITIONS.entries()){const label=s.page.form.labels[0];label.text='  组队\n冲分  ';label.position=position[0];label.color='#123aBc';label.background='transparent';label.font_size=99;const body=s.page.saveBody();assert.deepEqual(Object.keys(body).sort(),['game_slides','revision']);assert.deepEqual(Object.keys(body.game_slides),['delta']);assert.equal(body.game_slides.delta[0].labels[0].position,position[0]);assert.equal(body.game_slides.delta[0].labels[0].text,'组队 冲分');assert.equal(body.game_slides.delta[0].labels[0].font_size,24);assert.equal(body.game_slides.delta[0].labels[0].color,'#123aBc');assert.equal(index<9,true)}
  s.page.removeLabel(5);assert.equal(s.page.form.labels.length,5)
})

test('轮播历史底色在编辑预览和保存中统一透明，新文字不继承服务标签底色',async t=>{
  const s=harness(t),data=config();data.global_slides[0].labels[0].background='#b98132';await ready(s,data)
  assert.equal(s.page.previewLabels.value[0].background,'transparent')
  s.page.form.labels[0].background='url(unsafe)';assert.equal(s.page.saveBody().global_slides[0].labels[0].background,'transparent')
  s.page.addLabel();assert.equal(s.page.form.labels[1].background,'transparent')
})

test('无图片、空文案、非法颜色均不发布，预览不会使用任意CSS',async t=>{
  const s=harness(t,{id:''});await ready(s);await s.page.save();assert.equal(s.requests.length,1);s.page.form.image_url='/api/media/new.jpg';s.page.addLabel();s.page.form.labels[0].color='url(javascript:bad)';assert.equal(s.page.previewLabels.value[0].color,'#ffffff');await s.page.save();assert.equal(s.requests.length,1)
  s.page.form.labels[0].color='#ffffff';s.page.form.labels[0].text=' ';await s.page.save();assert.equal(s.requests.length,1);assert.equal(s.notices.length,3)
})

test('官方16:9上传期间保存独占，隐藏返回和取消选图不丢草稿',async t=>{
  const s=harness(t);await ready(s);s.page.form.labels[0].text='未保存的草稿';const selecting=s.page.uploadImage();assert.equal(s.uploads[0].purpose,'promotion_banner');await s.page.uploadImage();await s.page.save();assert.equal(s.uploads.length,1);assert.equal(s.requests.length,1)
  s.hooks.hide();s.hooks.show();await flush();assert.equal(s.requests.length,1);assert.equal(s.page.form.labels[0].text,'未保存的草稿');s.uploads[0].resolve('/api/media/replaced.jpg');await selecting;assert.equal(s.page.form.image_url,'/api/media/replaced.jpg')
  const cancel=s.page.uploadImage();s.uploads[1].reject({cancelled:true});await cancel;assert.equal(s.page.form.image_url,'/api/media/replaced.jpg');assert.equal(s.notices.length,0)
})

test('409核对只更新服务器基线，保留草稿，重新保存保留别人新加轮播',async t=>{
  const s=harness(t);await ready(s);s.page.form.image_url='/api/media/local.jpg';s.page.form.labels[0].text='本地稿';const saving=s.page.save();s.take('PATCH').reject({status:409,message:'版本冲突'});await saving;assert.match(s.page.conflict.value,/草稿已保留/)
  const checking=s.page.recheckLatest(),latest=config(9);latest.global_slides[0].image_url='/api/media/remote.jpg';latest.global_slides.push({id:'remote-new',kind:'image',image_url:'/api/media/remote-new.jpg',labels:[],target:null});s.take().resolve(latest);await checking
  assert.equal(s.page.form.image_url,'/api/media/local.jpg');assert.equal(s.page.form.labels[0].text,'本地稿');assert.equal(s.page.reviewedSlide.value.image_url,'/api/media/remote.jpg');const body=s.page.saveBody();assert.equal(body.revision,9);assert.equal(body.global_slides.length,4);assert.equal(body.global_slides[3].id,'remote-new');assert.equal(body.global_slides[0].image_url,'/api/media/local.jpg')
})

test('原项被删除不能悄悄重建，核对失败也保留草稿',async t=>{
  const s=harness(t);await ready(s);s.page.form.labels[0].text='本地继续编辑';const failed=s.page.recheckLatest();s.take().reject(new Error('网络暂不可用'));await failed;assert.equal(s.page.form.labels[0].text,'本地继续编辑');assert.match(s.page.conflict.value,/核对失败/)
  const checking=s.page.recheckLatest(),latest=config(8);latest.global_slides=latest.global_slides.filter(item=>item.id!=='global-A');s.take().resolve(latest);await checking;assert.equal(s.page.missing.value,true);await s.page.save();assert.equal(s.requests.filter(row=>row.method==='PATCH').length,0);assert.equal(s.page.form.labels[0].text,'本地继续编辑')
})

test('新建稳定ID在未知保存后可核对恢复，不重复追加；每组最多12项',async t=>{
  const s=harness(t,{id:''});await ready(s);s.page.form.image_url='/api/media/new.jpg';const id=s.page.form.id,first=s.page.save();s.take('PATCH').reject(new Error('响应未知'));await first;assert.equal(s.page.form.id,id)
  const latest=config(7);latest.global_slides.push(JSON.parse(JSON.stringify(s.page.form)));const checking=s.page.recheckLatest();s.take().resolve(latest);await checking;assert.equal(s.page.saveBody().global_slides.filter((item:ContentSlide)=>item.id===id).length,1)
  const full=harness(t,{id:'',game:'delta'}),data=config();data.game_slides.delta=Array.from({length:12},(_,i)=>({id:'image-'+i,kind:'image',image_url:'/api/media/'+i+'.jpg',labels:[],target:null}));await ready(full,data);full.page.form.image_url='/api/media/new.jpg';await full.page.save();assert.equal(full.requests.length,1);assert.match(String(full.notices[0]),/12/)
})

test('无管理权限不请求私有配置，读取失权清空，旧管理员上传和保存回包失效',async t=>{
  const guest=harness(t,{role:'user'});guest.hooks.show();await flush();assert.equal(guest.requests.length,0)
  const s=harness(t);await ready(s);const uploading=s.page.uploadImage();s.switchUser();await flush();const fresh=config(6);fresh.global_slides[0].image_url='/api/media/admin-B.jpg';s.take().resolve(fresh);await flush();s.uploads[0].resolve('/api/media/admin-A.jpg');await uploading;assert.equal(s.page.form.image_url,'/api/media/admin-B.jpg')
  const saving=s.page.save(),old=s.take('PATCH');s.state.user.role='user';await flush();old.resolve(config(8));await saving;assert.equal(s.page.loaded.value,false);assert.equal(s.page.form.image_url,'');assert.equal(s.backs(),0);assert.equal(s.notices.length,0)
  const denied=harness(t);denied.hooks.show();await flush();denied.take().reject({status:403,message:'无配置权限'});await flush();assert.equal(denied.page.loaded.value,false);assert.equal(denied.page.error.value,'无配置权限')
})

test('隐藏不解除保存锁，回来不重读或重复发单；卸载旧读取不回填',async t=>{
  const s=harness(t);await ready(s);const saving=s.page.save(),writing=s.take('PATCH');s.hooks.hide();s.hooks.show();await s.page.save();await flush();assert.equal(s.page.busy.value,true);assert.equal(s.requests.length,2);writing.resolve(config(4));await saving;assert.equal(s.page.busy.value,false)
  const loading=harness(t);loading.hooks.show();await flush();const read=loading.take();loading.hooks.unload();read.resolve(config());await flush();assert.equal(loading.page.loaded.value,false);assert.equal(loading.page.form.image_url,'')
})

test('游戏槽和已删ID不能冒充图片编辑，保存被撤权立即清空私有草稿',async t=>{
  for(const id of ['slot','deleted']){const s=harness(t,{id});await ready(s);assert.equal(s.page.loaded.value,false);assert.match(s.page.error.value,/不存在或已删除/);await s.page.save();assert.equal(s.requests.length,1)}
  const s=harness(t);await ready(s);const saving=s.page.save();s.take('PATCH').reject({status:403,message:'管理权限已撤销'});await saving;assert.equal(s.page.loaded.value,false);assert.equal(s.page.form.image_url,'');assert.equal(s.page.error.value,'管理权限已撤销')
})
