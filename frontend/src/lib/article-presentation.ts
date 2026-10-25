/** 文件作用：仅为服务端已经安全解析的Markdown HTML补充跨端排版和托管图绝对地址；不承担原始HTML清洗。 */
/** 将已校验的资源地址安全放回HTML属性，不改变其它正文文字。 */
function attribute(value:string):string{return value.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
/**
 * 为Goldmark固定生成的标签补展示样式，避免微信把相对图当成本地包文件。
 * @param html - 仅限文章/预览接口返回的当前安全HTML，禁止直接传用户Markdown或原始HTML。
 * @param resolveAsset - 项目统一的托管资源地址解析器。
 * @returns 可交给官方rich-text的同一安全正文，不缓存旧审核内容。
 */
export function presentArticleHTML(html:string,resolveAsset:(path:string)=>string):string{
  const styles:Record<string,string>={h1:'font-size:1.6em;line-height:1.5;margin:1em 0 .6em',h2:'font-size:1.35em;line-height:1.5;margin:1.2em 0 .6em',h3:'font-size:1.15em;line-height:1.6;margin:1em 0 .5em',h4:'font-weight:700;margin:1em 0 .5em',p:'margin:.7em 0;line-height:1.9;overflow-wrap:anywhere',ul:'padding-left:1.5em;margin:.7em 0',ol:'padding-left:1.5em;margin:.7em 0',li:'margin:.3em 0;line-height:1.85',blockquote:'margin:1em 0;padding:.3em 0 .3em 1em;border-left:.2em solid #83e7b7;color:#b8cbbf',pre:'max-width:100%;box-sizing:border-box;white-space:pre-wrap;overflow-wrap:anywhere;padding:1em;background:rgba(255,255,255,.04);border-radius:.4em',code:'font-family:monospace;white-space:pre-wrap;overflow-wrap:anywhere',img:'display:block;max-width:100%;height:auto;margin:1em 0',a:'color:#83e7b7;text-decoration:underline'}
  return String(html||'').replace(/<(h[1-4]|p|ul|ol|li|blockquote|pre|code|img|a)\b([^>]*)>/g,(_whole:string,tag:string,attrs:string)=>{const selfClosing=/\/\s*$/.test(attrs);let attributes=attrs.replace(/\/\s*$/,'');if(tag==='img')attributes=attributes.replace(/\bsrc="([^"]*)"/,(_match:string,src:string)=>'src="'+attribute(resolveAsset(src))+'"');return '<'+tag+attributes+' style="'+styles[tag]+'"'+(selfClosing?' />':'>')})
}
