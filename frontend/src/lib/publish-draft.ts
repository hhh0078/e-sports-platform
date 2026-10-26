/** 文件作用：按账号、发布类型和编辑对象隔离本机表单草稿，负责白名单、恢复确认、延迟保存及成功后的条件清理。 */
import {ref,watch} from 'vue'
import type {CoverLabel} from '../types/models'
/** 服务草稿只含十项用户输入，编辑ID放在独立存储元数据。 */
export interface ServicePublishDraft{game_id:string;title:string;description:string;category:string;price:string;unit:string;cover_url:string;labels:CoverLabel[];tagsText:string;club_id:string}
/** 需求只保存四项用户输入，不缓存订单或支付状态。 */
export interface RequestPublishDraft{game_id:string;title:string;requirements:string;price:string}
/** 官方同步存储边界，测试可注入配额、损坏和并行页面。 */
export interface DraftStorage{read:(key:string)=>unknown;write:(key:string,value:string)=>void;remove:(key:string)=>void}
/** 页面提供明确字段投影与回填，不允许直接序列化完整接口响应。 */
export interface PublishDraftOptions<T>{kind:'service'|'request';owner:()=>string;object:()=>string;read:()=>T;apply:(fields:T)=>void;parse:(value:unknown)=>T|null;storage?:DraftStorage}
/** 本地文档包含隔离元数据和原始输入，不含会话秘密。 */
interface DraftDocument<T>{version:1;owner:string;kind:'service'|'request';object_id:string;updated_at:number;fields:T}
/** 每次提交捕获原上下文和精确存储版本，迟到成功不能删掉后续新稿。 */
export interface DraftSubmission{key:string;raw:string|null;signature:string;context:number}
/** 活动上下文只保存本页已校验的快照。 */
interface DraftContext<T>{id:number;owner:string;object:string;key:string;baseline:string;stored:unknown;latest:DraftDocument<T>|null;completed:boolean}
const nativeDraftStorage:DraftStorage={read:key=>uni.getStorageSync(key),write:(key,value)=>uni.setStorageSync(key,value),remove:key=>uni.removeStorageSync(key)}
/** 本机游客具有独立命名空间，永远不自动迁入后来登录的账号。 */
export function publishDraftOwner(userID?:string):string{return userID?'user:'+userID:'guest'}
/** 账号、表单和编辑对象共同构成key，空编辑ID表示新建。 */
export function publishDraftKey(owner:string,kind:'service'|'request',object=''):string{return'jijie_publish_draft_v1:'+encodeURIComponent(owner)+':'+kind+':'+encodeURIComponent(object||'new')}
/** 对未知值安全缩窄为普通字段容器。 */
function record(value:unknown):Record<string,unknown>|null{return value!==null&&typeof value==='object'&&!Array.isArray(value)?value as Record<string,unknown>:null}
/** 保留未完成输入但限制本机数据大小，不把损坏缓存当成有效表单。 */
function strings(value:Record<string,unknown>,limits:Record<string,number>):boolean{return Object.entries(limits).every(([key,max])=>typeof value[key]==='string'&&(value[key] as string).length<=max)}
/** 提取服务十字段和独立文字白名单；缓存允许尚未填完整的颜色与金额。 */
export function serviceDraftFields(value:unknown):ServicePublishDraft|null{
  const row=record(value);if(!row||!strings(row,{game_id:128,title:80,description:2000,category:128,price:64,unit:32,cover_url:2048,tagsText:120,club_id:128})||!Array.isArray(row.labels)||row.labels.length>6)return null
  const labels:CoverLabel[]=[]
  for(const value of row.labels){const label=record(value);if(!label||!strings(label,{text:96})||['color','background','position'].some(key=>label[key]!==undefined&&(typeof label[key]!=='string'||(label[key] as string).length>32))||label.font_size!==undefined&&(typeof label.font_size!=='number'||!Number.isFinite(label.font_size)||label.font_size<0||label.font_size>100)||label.id!==undefined&&(typeof label.id!=='string'&&typeof label.id!=='number'||String(label.id).length>128))return null;labels.push({text:label.text as string,...(label.id!==undefined?{id:label.id as string|number}:{}),...(label.color!==undefined?{color:label.color as string}:{}),...(label.background!==undefined?{background:label.background as string}:{}),...(label.position!==undefined?{position:label.position as string}:{}),...(label.font_size!==undefined?{font_size:label.font_size as number}:{})})}
  return{game_id:row.game_id as string,title:row.title as string,description:row.description as string,category:row.category as string,price:row.price as string,unit:row.unit as string,cover_url:row.cover_url as string,labels,tagsText:row.tagsText as string,club_id:row.club_id as string}
}
/** 提取需求四字段，不接受订单、金额分区或支付字段。 */
export function requestDraftFields(value:unknown):RequestPublishDraft|null{const row=record(value);return row&&strings(row,{game_id:128,title:80,requirements:2000,price:64})?{game_id:row.game_id as string,title:row.title as string,requirements:row.requirements as string,price:row.price as string}:null}
/**
 * 管理本页草稿；页面先完成初值/本人服务读取再start，同实例返回不重复start。
 * @param options - 当前身份、编辑对象、白名单投影与回填函数。
 * @returns 恢复弹层状态、缓存提示及保存/提交/上下文销毁方法。
 */
export function createPublishDraft<T>(options:PublishDraftOptions<T>){
  const storage=options.storage||nativeDraftStorage,pending=ref(false),issue=ref('')
  let context:DraftContext<T>|null=null,candidate:T|null=null,sequence=0,applying=false,disposed=false,timer:ReturnType<typeof setTimeout>|undefined
  /** 撤销尚未执行的延迟保存，不触碰任何账号的缓存。 */
  function cancelTimer():void{if(timer!==undefined)clearTimeout(timer);timer=undefined}
  /** 当前页面和身份仍属于原上下文时才允许回填或继续编辑。 */
  function current(value:DraftContext<T>):boolean{return !disposed&&context===value&&value.owner===options.owner()&&value.object===options.object()}
  /** 只在存储仍为本页读取版本时写入，避免覆盖另一页面的新草稿。 */
  function write(value:DraftContext<T>):void{
    const document=value.latest;if(!document||value.completed)return
    try{if(storage.read(value.key)!==value.stored)throw new Error('草稿已在其它页面更新，本页输入仍保留');const raw=JSON.stringify(document);storage.write(value.key,raw);value.stored=raw;if(storage.read(value.key)!==raw)throw new Error('草稿保存后核验失败');value.latest=null;issue.value=''}catch(error){issue.value=error instanceof Error?error.message:'本机草稿保存失败，请检查存储空间'}
  }
  /** 输入变化只复制白名单字段，初始化/确认期间不写空初值覆盖旧稿。 */
  function changed():void{
    const value=context;if(!value||!current(value)||applying||pending.value||value.completed)return
    const fields=options.parse(options.read());if(!fields)return
    const signature=JSON.stringify(fields);if(signature===value.baseline&&!value.stored){cancelTimer();value.latest=null;return}
    value.latest={version:1,owner:value.owner,kind:options.kind,object_id:value.object,updated_at:Date.now(),fields};cancelTimer();timer=setTimeout(()=>{timer=undefined;if(current(value))write(value)},250)
  }
  const stop=watch(options.read,changed,{deep:true,flush:'sync'})
  /** 保存最后一次已捕获输入；退出时不重新读取可能已切号的表单。 */
  function flush():void{cancelTimer();if(context)write(context)}
  /** 新账号或页面销毁前保存旧快照，撤销旧确认，绝不自动合并游客稿。 */
  function leave():void{flush();sequence++;context=null;candidate=null;pending.value=false}
  /** 校验新上下文缓存，有有效草稿才显示恢复选择；损坏内容不阻塞正常填写。 */
  function start():void{
    leave();if(disposed)return;issue.value=''
    const owner=options.owner(),object=options.object(),key=publishDraftKey(owner,options.kind,object),fields=options.parse(options.read());if(!fields)return
    const value:DraftContext<T>={id:++sequence,owner,object,key,baseline:JSON.stringify(fields),stored:undefined,latest:null,completed:false};context=value
    try{const raw=storage.read(key);value.stored=raw;if(raw===undefined||raw===null||raw==='')return;if(typeof raw!=='string'||raw.length>65536)throw new Error('本机草稿格式异常，原缓存已保留')
      const document=record(JSON.parse(raw));if(!document||document.version!==1||document.owner!==owner||document.kind!==options.kind||document.object_id!==object||typeof document.updated_at!=='number'||!Number.isFinite(document.updated_at))throw new Error('本机草稿归属或格式异常，原缓存已保留')
      const saved=options.parse(document.fields);if(!saved)throw new Error('本机草稿内容异常，原缓存已保留');candidate=saved;pending.value=true
    }catch(error){issue.value=error instanceof SyntaxError?'本机草稿无法读取，原缓存已保留':error instanceof Error?error.message:'本机草稿暂时无法读取'}
  }
  /** 明确恢复且身份未变化才回填，官方弹层迟到点击不能操作新账号。 */
  function restore(id:number):void{const value=context;if(!value||value.id!==id||!current(value)||!pending.value||!candidate)return;const fields=options.parse(candidate);if(!fields)return;applying=true;try{options.apply(fields)}finally{applying=false};pending.value=false;candidate=null}
  /** 只有明确选择不用才删除当前精确版本；遮罩或关闭只撤销展示。 */
  function discard(id:number):void{const value=context;if(!value||value.id!==id||!current(value)||!pending.value)return;try{if(storage.read(value.key)!==value.stored)throw new Error('草稿已更新，未删除其它页面的新内容');storage.remove(value.key);if(storage.read(value.key))throw new Error('草稿删除失败');value.stored=storage.read(value.key);candidate=null;pending.value=false;issue.value=''}catch(error){issue.value=error instanceof Error?error.message:'草稿删除失败，原缓存仍保留'}}
  /** 遮罩、关闭和隐藏不等于删除，旧上下文的关闭事件同样被忽略。 */
  function dismiss(id:number):void{if(context?.id===id){pending.value=false;candidate=null}}
  /** 发起业务请求前补存并捕获准确版本，后续成功只清这一份提交稿。 */
  function submitting():DraftSubmission|null{const value=context;if(!value||!current(value))return null;changed();flush();return{key:value.key,raw:typeof value.stored==='string'?value.stored:null,signature:JSON.stringify(options.parse(options.read())),context:value.id}}
  /** 成功不等待支付/订阅；即使已切号也只条件删除原key，不能清新输入。 */
  function complete(ticket:DraftSubmission|null):void{if(!ticket)return;const value=context;try{if(ticket.raw!==null&&storage.read(ticket.key)===ticket.raw){storage.remove(ticket.key);if(value?.id===ticket.context&&value.stored===ticket.raw)value.stored=storage.read(ticket.key)}}catch{if(value?.id===ticket.context&&current(value))issue.value='发布成功，但本机草稿清理失败，请检查存储空间'}if(value?.id===ticket.context&&current(value)&&JSON.stringify(options.parse(options.read()))===ticket.signature){cancelTimer();value.completed=true;value.latest=null;pending.value=false;candidate=null}}
  /** 组件卸载停止监听，最后一次输入仅写回其原命名空间。 */
  function dispose():void{leave();disposed=true;stop()}
  return{pending,issue,start,flush,leave,restore,discard,dismiss,submitting,complete,dispose,contextID:()=>context?.id||0}
}
