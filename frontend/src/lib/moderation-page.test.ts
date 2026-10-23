/** 文件作用：执行真实图文审核页面与领域API，核验权限、原文读取、人工决定、AI降级及账号生命周期。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { ref, reactive, computed, watch, effectScope } from 'vue'
import { executable, pageScript, injectBusinessAPIs } from './sfc-test'
import type { DeferredRequest } from './sfc-test'
import type { ModerationTask, ModerationStatus } from '../types/moderation'

/** 创建包含全部服务端字段的任务，不用摘要代替详情原文。 */
function task(id='task-A',status:ModerationStatus='manual_pending',kind:'text'|'image'='text'):ModerationTask{return{id,target_type:'services',target_id:'service-A',field_code:'title',field_label:'服务标题',kind,status,actor_id:'owner-A',owner_id:'owner-A',created_at:'2026-10-23T08:00:00Z',updated_at:'2026-10-23T08:00:00Z',reason:'',preview:'完整待审核文字',content_version:2,attempts:1,last_error:'',decision_source:''}}
/** 汇总由服务端提供，测试不靠已加载分页伪造。 */
function inbox(items:ModerationTask[]=[]){return{items,next_cursor:'',counts:{ai_pending:0,ai_processing:0,manual_pending:items.length,allowed:0,rejected:0,superseded:0,total:items.length}}}
/** 使用可控网络和官方确认边界执行生产页面，保留Vue真实侦听。 */
function harness(t:TestContext,role='admin'){
  const requests:DeferredRequest[]=[],notices:unknown[]=[],dialogs:Array<{resolve:(v:{confirm:boolean})=>void;content:string}>=[],files:Array<{path:string;resolve:(v:string)=>void}>=[],removed:string[]=[]
  const state=reactive({user:{id:'admin-A',role},ready:true}),revision=ref(1),storage=new Map([['jijie_token','token-A']]),hooks={show:()=>{},hide:()=>{},unload:()=>{}}
  const context=vm.createContext({ref,reactive,computed,watch,state,sessionRevision:()=>revision.value,token:()=>storage.get('jijie_token'),init:async()=>{},go:()=>{},time:(v:string)=>v,notify:(v:unknown)=>notices.push(v),
    uni:{showModal:(options:{content:string})=>new Promise(resolve=>dialogs.push({resolve,content:options.content})),getFileSystemManager:()=>({unlink:({filePath}:{filePath:string})=>removed.push(filePath)}),previewImage:()=>{}},
    api(path:string,method='GET',body:Record<string,unknown>={}){return new Promise<unknown>((resolve,reject)=>requests.push({path,method,body,resolve,reject,taken:false}))},
    download(path:string){return new Promise<string>(resolve=>files.push({path,resolve}))},
    onShow:(fn:()=>void)=>{hooks.show=fn},onHide:(fn:()=>void)=>{hooks.hide=fn},onUnload:(fn:()=>void)=>{hooks.unload=fn},
  })
  injectBusinessAPIs(context)
  vm.runInContext(executable(readFileSync(new URL('../api/moderation.api.ts',import.meta.url),'utf8')),context)
  const scope=effectScope();scope.run(()=>vm.runInContext(executable(pageScript('manage/moderation.vue'))+'\nglobalThis.page={loadList,chooseStatus,openTask,closeDetail,decide,retryAI,tasks,status,authorized,selected,selectedId,detailOpened,detailError,reason,busy,canDecide,imagePath,decisionSource}',context));t.after(()=>scope.stop())
  return{page:context.page,state,revision,storage,requests,notices,dialogs,files,removed,hooks,
    /** 只取得确实由真实领域API发出的请求。 */
    take(path:string,method='GET'){const found=requests.find(item=>!item.taken&&item.path===path&&item.method===method);assert.ok(found,`缺少${method} ${path}`);found.taken=true;return found},
  }
}
/** 排空实际await和Vue侦听，不用任意等待时长模拟顺序。 */
async function flush(){await new Promise(resolve=>setImmediate(resolve))}
/** 完成管理员真实队列加载。 */
async function load(subject:ReturnType<typeof harness>){subject.hooks.show();await flush();await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()}
/** 完成一次真实任务详情读取。 */
async function open(subject:ReturnType<typeof harness>,value=task()){const pending=subject.page.openTask(value.id);subject.take('/admin/moderation/'+value.id).resolve(value);await pending}

test('普通玩家和俱乐部负责人不能读取或处理平台图文原文',async t=>{for(const role of ['user','clubowner'])await t.test(role,async child=>{const subject=harness(child,role);subject.hooks.show();await flush();await subject.page.openTask('secret');await subject.page.decide('allow');assert.equal(subject.requests.length,0);assert.equal(subject.page.authorized.value,false)})})

test('人工拒绝必须先填原因并二次确认，完整详情才是审核依据',async t=>{
  const subject=harness(t);await load(subject);await open(subject)
  await subject.page.decide('reject');assert.equal(subject.dialogs.length,0);assert.match(String(subject.notices.at(-1)),/1—500/)
  subject.page.reason.value='包含违规招揽内容';const deciding=subject.page.decide('reject');assert.equal(subject.dialogs.length,1);subject.dialogs[0].resolve({confirm:true});await flush()
  const write=subject.take('/admin/moderation/task-A/decision','POST');assert.equal(write.body.decision,'reject');assert.equal(write.body.reason,'包含违规招揽内容');write.resolve({...task('task-A','rejected'),reason:'包含违规招揽内容'});await deciding
  assert.equal(subject.page.canDecide.value,false);assert.equal(subject.page.selected.value.status,'rejected');await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()
})

test('已更新和已处理任务只读，不向服务器提交重复决定',async t=>{for(const status of ['superseded','allowed','rejected'] as const)await t.test(status,async child=>{const subject=harness(child);await load(subject);await open(subject,task('old-task',status));await subject.page.decide('allow');assert.equal(subject.dialogs.length,0);assert.equal(subject.requests.filter(item=>item.method==='POST').length,0)})})

test('离开再返回后确认旧弹框，也不能执行已经撤销的人工决定',async t=>{
  const subject=harness(t);await load(subject);await open(subject);const deciding=subject.page.decide('allow');subject.hooks.hide();await load(subject);subject.dialogs[0].resolve({confirm:true});await deciding
  assert.equal(subject.requests.filter(item=>item.method==='POST').length,0);assert.equal(subject.page.selected.value,null);assert.equal(subject.page.busy.value,false)
})

test('新任务详情优先于迟到旧原文，关闭后图片下载被丢弃并释放',async t=>{
  const subject=harness(t);await load(subject);const old=subject.page.openTask('old');const oldRequest=subject.take('/admin/moderation/old');await open(subject,task('new'));oldRequest.resolve(task('old'));await old;assert.equal(subject.page.selected.value.id,'new')
  await open(subject,task('image-A','manual_pending','image'));assert.equal(subject.files[0].path,'/admin/moderation/image-A/image');assert.equal(subject.files[0].path.includes('token'),false)
  subject.page.closeDetail();subject.files[0].resolve('/tmp/private-image.png');await flush();assert.equal(subject.page.imagePath.value,'');assert.deepEqual(subject.removed,['/tmp/private-image.png'])
})

test('账号变更立即清除原文与图片，迟到结果不能写回新管理员',async t=>{
  const subject=harness(t);await load(subject);await open(subject,task('image-A','manual_pending','image'));subject.files[0].resolve('/tmp/old-admin.png');await flush();assert.equal(subject.page.imagePath.value,'/tmp/old-admin.png')
  subject.state.user={id:'admin-B',role:'admin'};subject.storage.set('jijie_token','token-B');subject.revision.value++;await flush()
  assert.equal(subject.page.selected.value,null);assert.equal(subject.page.imagePath.value,'');assert.ok(subject.removed.includes('/tmp/old-admin.png'));await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()
})

test('无AI配置重试仍保留人工待审，管理员可以继续手工处理',async t=>{
  const subject=harness(t);await load(subject);await open(subject);const retry=subject.page.retryAI();subject.take('/admin/moderation/task-A/retry','POST').resolve({...task(),last_error:'AI尚未配置'});await retry
  assert.equal(subject.page.selected.value.status,'manual_pending');assert.equal(subject.page.canDecide.value,true);assert.equal(subject.page.busy.value,false);assert.match(String(subject.notices.at(-1)),/继续等待人工/);await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()
})

test('审核期间原字段更新时409关闭操作，保留可读错误并要求重新读取',async t=>{
  const subject=harness(t);await load(subject);await open(subject);const deciding=subject.page.decide('allow');subject.dialogs[0].resolve({confirm:true});await flush();subject.take('/admin/moderation/task-A/decision','POST').reject(Object.assign(new Error('内容已更新'),{status:409}));await deciding
  assert.equal(subject.page.selected.value,null);assert.equal(subject.page.canDecide.value,false);assert.match(subject.page.detailError.value,/重新读取/);await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()
})


test('AI处理中的任务可人工接管，确认提示竞争规则且来源文案准确',async t=>{
  const subject=harness(t);await load(subject);await open(subject,task('processing-task','ai_processing'))
  assert.equal(subject.page.canDecide.value,true)
  const deciding=subject.page.decide('allow');assert.match(subject.dialogs[0].content,/AI正在处理中.*先完成的有效审核结果生效/)
  subject.dialogs[0].resolve({confirm:false});await deciding;await flush();subject.take('/admin/moderation?limit=20').resolve(inbox());await flush()
  assert.equal(subject.page.decisionSource('inherited'),'沿用可信图片审核');assert.equal(subject.page.decisionSource('source'),'关联图片审核结果');assert.equal(subject.page.decisionSource('system'),'系统处理')
})
