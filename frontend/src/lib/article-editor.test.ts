/** 文件作用：执行真实Markdown编辑页，核验安全预览、正文限额、版本冲突、上传用途与管理员会话隔离。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import vm from 'node:vm'
import {readFileSync} from 'node:fs'
import {computed,effectScope,nextTick,reactive,ref,watch} from 'vue'
import {executable,injectBusinessAPIs,pageScript} from './sfc-test'
import type {DeferredRequest,DeferredUpload} from './sfc-test'
import type {AdminArticle} from '../types/content'

const saved:AdminArticle={id:'article-A',title:'服务与验收指南',summary:'服务约定与交付说明',cover_url:'',body_markdown:'# 开始前确认\n\n- 服务范围',status:'draft',revision:2,created_at:'2026-10-26T00:00:00Z',updated_at:'2026-10-26T00:00:00Z',published_at:''}
/** 在受控传输边界内执行真实页面及真实content.api，保留Vue侦听与官方选图生命周期。 */
function harness(t:TestContext,id='article-A'){
  const state=reactive({ready:true,user:{id:'admin-A',role:'admin',platform_admin_role:'root',status:'active'}}),requests:DeferredRequest[]=[],uploads:DeferredUpload[]=[],notices:unknown[]=[],copies:string[]=[],dialogs:Array<{resolve:(value:{confirm:boolean})=>void}>=[]
  const hooks={load:(_options:Record<string,string>)=>{},show:()=>{},hide:()=>{},unload:()=>{}};let revision=0
  const context=vm.createContext({ref,computed,reactive,watch,nextTick,state,init:async()=>{},sessionRevision:()=>revision,token:()=>state.user.id+'token',notify:(message:unknown)=>notices.push(message),assetUrl:(url:string)=>url,onLoad:(callback:typeof hooks.load)=>{hooks.load=callback},onShow:(callback:()=>void)=>{hooks.show=callback},onHide:(callback:()=>void)=>{hooks.hide=callback},onUnload:(callback:()=>void)=>{hooks.unload=callback},uni:{setClipboardData:({data}:{data:string})=>{copies.push(data)},showModal:()=>new Promise(resolve=>dialogs.push({resolve}))},chooseUploadImage(purpose:string,options?:object){return new Promise((resolve,reject)=>uploads.push({purpose,options,resolve,reject}))},api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))}})
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8')),context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('manage/articles/edit.vue'))+';globalThis.page={load,save,refreshPreview,selectMode,form,articleId,bodyBytes,loaded,loading,busy,uploading,error,conflict,previewError,previewHTML,previewing,uploadImage,rememberCursor,copyDraft,reloadLatest}',context));t.after(()=>scope.stop());hooks.load(id?{id}:{})
  return{state,page:context.page,hooks,requests,uploads,notices,copies,dialogs,
    /** 请求必须由真实业务方法发出，测试只决定响应先后。 */
    take(path:string,method='GET'){const row=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(row,'缺少请求 '+method+' '+path);row.taken=true;return row},
    /** 切换管理员触发真实会话清理，旧返回不得跨账号回填。 */
    switchUser(){revision++;state.user.id='admin-B'},
  }
}
/** 排空微任务和Vue侦听，不依赖固定睡眠时间。 */
async function flush():Promise<void>{await new Promise(resolve=>setImmediate(resolve))}
/** 完成管理员初始化与既有文章读取。 */
async function ready(s:ReturnType<typeof harness>,article:AdminArticle=saved):Promise<void>{s.hooks.show();await flush();if(s.page.articleId.value){s.take('/admin/articles/'+s.page.articleId.value).resolve({...article});await flush()}}

test('文章编辑显式校验Unicode标题摘要和48KiB真实UTF-8字节，非法正文不写入',async t=>{
  const s=harness(t);await ready(s)
  for(const patch of [{title:'字'.repeat(61)},{summary:'摘'.repeat(301)},{body_markdown:'文'.repeat(16385)},{body_markdown:'  \n '}]){Object.assign(s.page.form,saved,patch);await s.page.save('published')}
  assert.equal(s.requests.filter(row=>row.method!=='GET').length,0);assert.equal(s.notices.length,4)
  s.page.form.title='😀指南';s.page.form.summary='';s.page.form.body_markdown='😀'.repeat(12288);assert.equal(s.page.bodyBytes.value,49152)
  const saving=s.page.save('draft');const writing=s.take('/admin/articles/article-A','PATCH');assert.equal(writing.body.body_markdown,s.page.form.body_markdown);assert.equal(writing.body.revision,2);writing.resolve({...saved,revision:3});await saving
})
test('新文章连点只创建一次，成功后沿用真实ID和修订继续保存',async t=>{
  const s=harness(t,'');await ready(s);Object.assign(s.page.form,{title:'发布指南',summary:'',body_markdown:'# 正文',cover_url:'/media/cover.jpg'})
  const first=s.page.save('published');await s.page.save('published');const creating=s.take('/admin/articles','POST');assert.equal(s.requests.length,1);assert.equal(creating.body.revision,undefined);assert.equal(creating.body.status,'published')
  creating.resolve({...saved,status:'published',revision:1});await first;assert.equal(s.page.articleId.value,'article-A')
  const second=s.page.save('hidden');const updating=s.take('/admin/articles/article-A','PATCH');assert.equal(updating.body.revision,1);assert.equal(updating.body.status,'hidden');updating.resolve({...saved,status:'hidden',revision:2});await second
  assert.equal(s.page.form.status,'hidden');assert.equal(s.page.busy.value,false)
})
test('409保留标题、封面和多行Markdown草稿，读取最新前需明确确认',async t=>{
  const s=harness(t);await ready(s);Object.assign(s.page.form,{title:'本地修改',cover_url:'/media/local.jpg',body_markdown:'# 本地\n\n换行正文'})
  const saving=s.page.save('published');s.take('/admin/articles/article-A','PATCH').reject({status:409,message:'版本已更新'});await saving
  assert.match(s.page.conflict.value,/本地草稿已保留/);assert.equal(s.page.form.title,'本地修改');assert.equal(s.page.form.cover_url,'/media/local.jpg');assert.equal(s.page.form.body_markdown,'# 本地\n\n换行正文')
  s.page.copyDraft();assert.match(s.copies[0],/# 本地修改/);assert.match(s.copies[0],/换行正文/)
  const cancelled=s.page.reloadLatest();s.dialogs[0].resolve({confirm:false});await cancelled;assert.equal(s.requests.length,2);assert.equal(s.page.form.title,'本地修改')
  const reloading=s.page.reloadLatest();s.dialogs[1].resolve({confirm:true});await flush();s.take('/admin/articles/article-A').resolve({...saved,title:'服务器新稿',revision:5});await reloading;assert.equal(s.page.form.title,'服务器新稿');assert.equal(s.page.form.revision,5)
})
test('安全预览不保存，正文变化与切回编辑会作废旧HTML响应',async t=>{
  const s=harness(t);await ready(s);s.page.form.body_markdown='# 旧稿';await flush();s.page.selectMode('preview');const old=s.take('/admin/articles/preview','POST')
  s.page.form.body_markdown='# 新稿';await flush();s.page.selectMode('preview');const current=s.take('/admin/articles/preview','POST');assert.equal(current.body.body_markdown,'# 新稿')
  old.resolve({body_html:'<p>旧稿</p>'});await flush();assert.equal(s.page.previewHTML.value,'');assert.equal(s.page.previewing.value,true)
  current.resolve({body_html:'<h1>新稿</h1>'});await flush();assert.equal(s.page.previewHTML.value,'<h1>新稿</h1>')
  s.page.selectMode('preview');const hidden=s.take('/admin/articles/preview','POST');s.page.selectMode('edit');hidden.resolve({body_html:'<p>失效预览</p>'});await flush();assert.equal(s.page.previewHTML.value,'')
  assert.ok(s.requests.filter(row=>row.method!=='GET').every(row=>row.path==='/admin/articles/preview'))
})
test('预览中隐藏后返回重新获取当前正文，旧回包不留下空白或覆盖新结果',async t=>{
  const s=harness(t);await ready(s);s.page.selectMode('preview');const old=s.take('/admin/articles/preview','POST');s.hooks.hide();s.hooks.show();const current=s.take('/admin/articles/preview','POST')
  old.resolve({body_html:'<p>隐藏前响应</p>'});await flush();assert.equal(s.page.previewHTML.value,'');current.resolve({body_html:'<p>当前预览</p>'});await flush();assert.equal(s.page.previewHTML.value,'<p>当前预览</p>');assert.equal(s.requests.filter(row=>row.method==='GET').length,1)
})
test('服务端按Markdown语法拒绝超过20图时保留整篇草稿并显示错误',async t=>{
  const s=harness(t);await ready(s);const body=Array.from({length:21},(_,i)=>'![说明'+i+'](/media/image'+i+'.jpg)').join('\n');s.page.form.body_markdown=body;await flush()
  const preview=s.page.refreshPreview();s.take('/admin/articles/preview','POST').reject({status:400,message:'文章正文最多20张图片'});await preview;assert.equal(s.page.previewError.value,'文章正文最多20张图片');assert.equal(s.page.form.body_markdown,body)
  const saving=s.page.save('draft');s.take('/admin/articles/article-A','PATCH').reject({status:400,message:'文章正文最多20张图片'});await saving;assert.equal(s.page.form.body_markdown,body);assert.equal(s.page.busy.value,false)
})
test('封面和正文图使用不同官方用途，原生选图隐藏返回不清草稿并在光标处插入',async t=>{
  const s=harness(t);await ready(s);s.page.form.title='未保存标题';s.page.form.body_markdown='前后';s.page.rememberCursor({detail:{cursor:1}})
  const inserting=s.page.uploadImage('article_image');assert.equal(s.uploads[0].purpose,'article_image');s.hooks.hide();s.hooks.show();await flush();assert.equal(s.requests.length,1);assert.equal(s.page.form.title,'未保存标题')
  s.uploads[0].resolve('/media/body.jpg');await inserting;assert.equal(s.page.form.body_markdown,'前\n![图片说明](/media/body.jpg)\n后')
  const covering=s.page.uploadImage('article_cover');assert.equal(s.uploads[1].purpose,'article_cover');s.uploads[1].resolve('/media/cover.jpg');await covering;assert.equal(s.page.form.cover_url,'/media/cover.jpg')
  const cancelled=s.page.uploadImage('article_cover');s.uploads[2].reject({cancelled:true});await cancelled;assert.equal(s.page.form.cover_url,'/media/cover.jpg');assert.equal(s.notices.length,0)
  s.page.form.body_markdown='甲乙';s.page.rememberCursor({target:{selectionStart:1}});const web=s.page.uploadImage('article_image');s.uploads[3].resolve('/media/web.jpg');await web;assert.equal(s.page.form.body_markdown,'甲\n![图片说明](/media/web.jpg)\n乙')
})
test('切号废弃旧读取、上传和保存，旧管理员稿件不进入新账号',async t=>{
  const s=harness(t);await ready(s);const uploading=s.page.uploadImage('article_cover');s.switchUser();await flush();s.take('/admin/articles/article-A').resolve({...saved,title:'B账号新稿'});await flush();s.uploads[0].resolve('/media/admin-A.jpg');await uploading;assert.equal(s.page.form.title,'B账号新稿');assert.equal(s.page.form.cover_url,'')
  const saving=s.page.save('published');const writing=s.take('/admin/articles/article-A','PATCH');s.state.user.role='user';await flush();writing.resolve({...saved,title:'A账号旧稿',cover_url:'/media/secret.jpg'});await saving;assert.equal(s.page.loaded.value,false);assert.equal(s.page.form.title,'');assert.equal(s.page.form.cover_url,'');assert.equal(s.notices.length,0)
})
test('读取或保存失去权限立即清空私有文章，普通用户不发管理员接口',async t=>{
  const s=harness(t);s.state.user.role='user';await flush();s.hooks.show();await flush();assert.equal(s.requests.length,0);assert.equal(s.page.loaded.value,false)
  const admin=harness(t);await ready(admin);admin.page.form.title='私有内容';const saving=admin.page.save('draft');admin.take('/admin/articles/article-A','PATCH').reject({status:403,message:'已无文章管理权'});await saving;assert.equal(admin.page.form.title,'');assert.equal(admin.page.loaded.value,false);assert.equal(admin.page.error.value,'已无文章管理权')
  const reading=harness(t);reading.hooks.show();await flush();reading.take('/admin/articles/article-A').reject({status:403,message:'不能读取文章'});await flush();assert.equal(reading.page.form.body_markdown,'');assert.equal(reading.page.loaded.value,false);assert.equal(reading.page.error.value,'不能读取文章')
})
test('页面隐藏不释放仍在途保存锁，同账号返回不重新初始化或重复发布',async t=>{
  const s=harness(t);await ready(s);s.page.form.title='发布中的草稿';const saving=s.page.save('published'),writing=s.take('/admin/articles/article-A','PATCH');s.hooks.hide();s.hooks.show();await s.page.save('published');await flush();assert.equal(s.requests.length,2);assert.equal(s.page.busy.value,true);assert.equal(s.page.form.title,'发布中的草稿')
  writing.resolve({...saved,title:'发布中的草稿',status:'published',revision:3});await saving;assert.equal(s.page.form.revision,3);assert.equal(s.page.busy.value,false)
})
