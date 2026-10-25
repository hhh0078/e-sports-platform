/** 文件作用：从真实Vue模板与CSS验证多行正文保留换行，独立检查运营配置的传输正文不被压成单行。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import postcss from 'postcss'
import {compile,computed,reactive} from 'vue'
import type {ComputedRef,VNode} from 'vue'
import {executable} from './sfc-test'
import {presentArticleHTML} from './article-presentation'

/** 读取真实样式最终对目标类声明的white-space，避免仅在注释中命中pre-wrap。 */
function whiteSpace(path:string,className:string):string{const source=readFileSync(new URL('../'+path,import.meta.url),'utf8'),css=[...source.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map(item=>item[1]).join('\n');let result='';postcss.parse(css).walkRules(rule=>{if(rule.selectors.includes('.'+className))rule.walkDecls('white-space',decl=>{result=decl.value})});return result}
test('说明、售后、简介、评价、协议及公共提示各真实显示层保留换行',()=>{
  const cases=[['pages/services/detail.vue','description'],['pages/requests/detail.vue','request-text'],['pages/orders/detail.vue','agreement-body'],['pages/orders/detail.vue','refund-panel'],['pages/orders/detail.vue','review-result'],['pages/community/players/detail.vue','player-bio'],['pages/community/clubs/detail.vue','detail-text'],['pages/community/clubs/detail.vue','rule-body'],['pages/account/agreements.vue','document-body'],['pages/account/messages/detail.vue','message-body'],['components/ClubCard.vue','club-description'],['components/ServiceReview.vue','review-body'],['components/FundsForm.vue','funds-form-note'],['components/PublicNotice.vue','public-notice-body'],['components/ContentTip.vue','content-tip-text'],['components/HomeAnnouncement.vue','announcement-body'],['components/ManagedPersonCenter.vue','record-note']]
  for(const [path,cls]of cases)assert.equal(whiteSpace(path,cls),'pre-wrap',path+' .'+cls)
})
/** 收集Vue生成的真实文字节点，不把模板属性当成可见内容。 */
function text(node:unknown):string{if(typeof node==='string')return node;if(Array.isArray(node))return node.map(text).join('');return node&&typeof node==='object'&&'children'in node?text(node.children):''}
test('公共注意事项与顶部提示实际Vue节点原样保留两行和中间空行',()=>{
  const body='第一行说明\n\n第二行说明',cases=[{path:'components/PublicNotice.vue',context:{text:body,kind:'service'}},{path:'components/ContentTip.vue',context:{tip:{text:body,article_id:''}}}]
  for(const item of cases){const source=readFileSync(new URL('../'+item.path,import.meta.url),'utf8'),template=source.match(/<template>([\s\S]*?)<\/template>/)?.[1];assert.ok(template);const render=compile(template,{prefixIdentifiers:true,isCustomElement:()=>true})as unknown as(context:unknown,cache:unknown[])=>VNode;assert.ok(text(render(item.context,[])).includes(body),item.path)}
})
test('运营配置传输保持多行服务/需求说明和公告正文，不去掉内部换行',async()=>{
  const writes:Array<{path:string;body:Record<string,unknown>}>=[],context=vm.createContext({request:(path:string,_method:string,body:Record<string,unknown>)=>{writes.push({path,body});return Promise.resolve(body)}});vm.runInContext(executable(readFileSync(new URL('../api/content.api.ts',import.meta.url),'utf8'))+';globalThis.save=savePublicContent',context)
  const lines='第一行\n\n第二行',body={revision:1,service_notice:lines,request_notice:lines,announcement:{body:lines}};await context.save(body);assert.equal(writes[0].path,'/admin/public-content');assert.equal(writes[0].body.service_notice,lines);assert.equal(writes[0].body.request_notice,lines);assert.equal((writes[0].body.announcement as {body:string}).body,lines)
})

test('文章组件执行真实计算层，为官方rich-text保留换行、解析托管图并随安全正文更新',()=>{
  const source=readFileSync(new URL('../components/ArticleBody.vue',import.meta.url),'utf8'),template=source.match(/<template>([\s\S]*?)<\/template>/)?.[1],script=source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(template);assert.ok(script)
  const props=reactive({html:'<p>第一行<br>\n第二行</p><p>下一段</p><img src="/api/media/article-image" alt="正文原图" />'}),context=vm.createContext({computed,defineProps:()=>props,presentArticleHTML,assetUrl:(path:string)=>'http://127.0.0.1:18091'+path})
  vm.runInContext(executable(script)+';globalThis.actualNodes=nodes',context)
  const nodes=context.actualNodes as ComputedRef<string>,render=compile(template,{prefixIdentifiers:true,isCustomElement:()=>true})as unknown as(context:unknown,cache:unknown[])=>VNode,root=render({nodes:nodes.value},[]),child=(root.children as VNode[])[0]
  assert.equal(child.type,'rich-text');assert.equal(child.props?.nodes,nodes.value);assert.equal(child.props?.['user-select'],true)
  assert.match(String(child.props?.nodes),/第一行<br>\n第二行/);assert.match(String(child.props?.nodes),/<p[^>]+style="[^"]*line-height:1\.9/);assert.match(String(child.props?.nodes),/src="http:\/\/127\.0\.0\.1:18091\/api\/media\/article-image"/);assert.match(String(child.props?.nodes),/max-width:100%;height:auto/);assert.match(String(child.props?.nodes),/alt="正文原图"/)
  props.html='<p>当前审核投影<br>\n新正文</p>'
  const updated=(render({nodes:nodes.value},[]).children as VNode[])[0]
  assert.equal(updated.props?.nodes,nodes.value);assert.match(String(updated.props?.nodes),/当前审核投影<br>\n新正文/);assert.doesNotMatch(String(updated.props?.nodes),/第一行|article-image/)
})
