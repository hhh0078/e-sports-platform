/** 文件作用：在可见文字需要时加载真实字体子集，合并请求、隔离迟到结果并在失败时保留系统字。 */
import {computed,ref,watch,onScopeDispose} from 'vue'
import type {CSSProperties,Ref} from 'vue'
import {artFontGroups,artFontID} from './art-fonts'
import {fontSubsetURL} from '../api/fonts.api'
import type {CoverLabel} from '../types/models'

const loads=new Map<string,Promise<boolean>>()
/** 加载同一字集仅发起一次；失败后移除，后续显示或编辑时可重新尝试。 */
export function loadArtFont(id:string,text:string,family:string):Promise<boolean>{
  const previous=loads.get(family);if(previous)return previous
  const url=fontSubsetURL(id,text);if(!url)return Promise.resolve(false)
  const pending=new Promise<boolean>(resolve=>{
    if(typeof uni==='undefined'||typeof uni.loadFontFace!=='function'){resolve(false);return}
    const timer=setTimeout(()=>resolve(false),20000)
    /** 原生API返回后结束超时，晚到结果不会覆盖已回退的展示。 */
    function finish(loaded:boolean):void{clearTimeout(timer);resolve(loaded)}
    try{uni.loadFontFace({global:true,family,source:'url("'+url+'")',desc:{style:'normal',weight:'400',variant:'normal'},success:()=>finish(true),fail:()=>finish(false)})}catch{finish(false)}
  }).then(loaded=>{if(!loaded)loads.delete(family);return loaded})
  loads.set(family,pending)
  // 原生/浏览器缓存继续有效，只限制JS请求索引，避免长列表无限增长。
  if(loads.size>256)loads.delete(loads.keys().next().value!)
  return pending
}
/** 仅允许最新可见字集应用字体，切图、输入、换字体或卸载不会被旧回包恢复。 */
export function useArtFonts(labels:()=>CoverLabel[],active:()=>boolean):{style:(label:CoverLabel)=>CSSProperties;loading:Ref<boolean>;failed:Ref<string[]>}{
  const groups=computed(()=>artFontGroups(labels())),ready=ref<Record<string,string>>({}),loading=ref(false),failed=ref<string[]>([])
  let version=0,disposed=false
  watch(()=>[active(),JSON.stringify(groups.value)] as const,async([enabled])=>{
    const current=++version;ready.value={};failed.value=[];loading.value=false;if(!enabled||disposed)return
    loading.value=groups.value.length>0
    const entries=await Promise.all(groups.value.map(async group=>[group.id,await loadArtFont(group.id,group.text,group.family)?group.family:''] as const))
    if(!disposed&&current===version&&active()){ready.value=Object.fromEntries(entries);failed.value=entries.filter(([,family])=>!family).map(([id])=>id);loading.value=false}
  },{immediate:true})
  onScopeDispose(()=>{disposed=true;version++})
  /** 成功后才应用艺术字；加载中、缺字和失败交由系统字体正常回退。 */
  function style(label:CoverLabel):CSSProperties{
    const family=ready.value[artFontID(label.font_family)]
    return family?{fontFamily:'"'+family+'", sans-serif',fontWeight:400}:{}
  }
  return{style,loading,failed}
}
