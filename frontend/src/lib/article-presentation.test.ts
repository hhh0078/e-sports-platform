/** 文件作用：核验安全Markdown展示层的托管图地址和响应式排版，不用前端正则代替服务端安全解析。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {presentArticleHTML} from './article-presentation'
test('微信正文和预览图片使用当前API域名，保持原比例且不改正文与换行',()=>{const html='<h2>服务范围</h2>\n<p>第一行<br>\n第二行</p>\n<p><img src="/api/media/image-id" alt="原图" /></p>';const result=presentArticleHTML(html,path=>'https://game.ai.58it.cn'+path);assert.match(result,/src="https:\/\/game\.ai\.58it\.cn\/api\/media\/image-id"/);assert.match(result,/max-width:100%;height:auto/);assert.match(result,/第一行<br>\n第二行/);assert.match(result,/alt="原图"/);assert.doesNotMatch(result,/font-size:\d+px|\/\s+style=/)})
test('重复渲染不保存正文缓存，代码和列表保留空白并安全编码资源属性',()=>{const old=presentArticleHTML('<p>旧内容</p>',path=>path),next=presentArticleHTML('<ul><li>新内容</li></ul><pre><code>a\n  b</code></pre><img src="/api/media/x">',()=>'/api/media/x?a=1&b="2"');assert.doesNotMatch(next,/旧内容/);assert.match(old,/旧内容/);assert.match(next,/white-space:pre-wrap/);assert.match(next,/a\n  b/);assert.match(next,/src="\/api\/media\/x\?a=1&amp;b=&quot;2&quot;"/)})
