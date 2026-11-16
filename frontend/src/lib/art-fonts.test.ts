/** 文件作用：验证按实际文字请求、白名单、请求合并、失败重试与切换后的迟到字体隔离。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {effectScope,nextTick,ref} from 'vue'
import {ART_FONTS,artFontFamily,artFontGroups,artFontID,fontCharacters} from './art-fonts'
import {fontSubsetURL} from '../api/fonts.api'
import {loadArtFont,useArtFonts} from './use-art-fonts'
import {serviceDraftFields} from './publish-draft'
import type {CoverLabel} from '../types/models'

test('字体资源仅包含选中字形，不引用完整文件，字集排序去重且名称无碰撞',()=>{
  assert.equal(ART_FONTS.length,7)
  assert.equal(artFontID('https://example.com/a.ttf'),'')
  assert.equal(fontSubsetURL('','你好'),'');assert.equal(fontSubsetURL('mashanzheng',''),'')
  assert.equal(fontCharacters('好你你好'),'你好')
  assert.equal(fontCharacters('\ud800😀'),'😀')
  assert.equal(artFontGroups([{text:'abcdefghijklmnop',font_family:'longcang'}])[0].text,'abcdefghijkl')
  const groups=artFontGroups([{text:'你好',font_family:'mashanzheng'},{text:'好朋友',font_family:'mashanzheng'},{text:'系统字'},{text:'',font_family:'longcang'}])
  assert.equal(groups.length,1);assert.equal(groups[0].text,fontCharacters('你好朋友'))
  const link=new URL(fontSubsetURL(groups[0].id,groups[0].text));assert.equal(link.pathname,'/api/fonts/1/mashanzheng.woff');assert.equal(link.searchParams.get('text'),groups[0].text)
  assert.notEqual(artFontFamily('longcang','a😀'),artFontFamily('longcang','a😁'))
  assert.equal(artFontFamily('longcang','你你好'),artFontFamily('longcang','好你'))
  const draft=serviceDraftFields({game_id:'delta',title:'标题',description:'说明',category:'陪练',price:'1',unit:'局',cover_url:'',labels:[{text:'你好',font_family:'longcang'}],tagsText:'',club_id:''})
  assert.equal(draft?.labels[0].font_family,'longcang')
})

test('并发同字集只加载一次，失败可重试，切换和隐藏后的迟到结果不能恢复旧字体',async t=>{
  const calls:UniApp.LoadFontFaceOptions[]=[]
  const original=Object.getOwnPropertyDescriptor(globalThis,'uni')
  Object.defineProperty(globalThis,'uni',{configurable:true,value:{loadFontFace:(options:UniApp.LoadFontFaceOptions)=>calls.push(options)}})
  t.after(()=>{if(original)Object.defineProperty(globalThis,'uni',original);else Reflect.deleteProperty(globalThis,'uni')})
  const first=loadArtFont('longcang','耐心',artFontFamily('longcang','耐心')),second=loadArtFont('longcang','耐心',artFontFamily('longcang','耐心'))
  assert.equal(first,second);assert.equal(calls.length,1);assert.equal(calls[0].global,true);calls[0].fail?.({});assert.equal(await first,false)
  const retry=loadArtFont('longcang','耐心',artFontFamily('longcang','耐心'));assert.equal(calls.length,2);calls[1].success?.({});assert.equal(await retry,true)
  const scope=effectScope(),labels=ref<CoverLabel[]>([{text:'组队',font_family:'zcoolkuaile'}]),active=ref(false)
  const fonts=scope.run(()=>useArtFonts(()=>labels.value,()=>active.value))!;assert.equal(calls.length,2)
  active.value=true;await nextTick();assert.equal(calls.length,3)
  labels.value=[{text:'冲分',font_family:'mashanzheng'}];await nextTick();assert.equal(calls.length,4)
  calls[2].success?.({});await nextTick();await nextTick();assert.deepEqual(fonts.style(labels.value[0]),{})
  calls[3].success?.({});await new Promise(resolve=>setImmediate(resolve));assert.match(fonts.style(labels.value[0]).fontFamily||'',/mashanzheng/)
  active.value=false;await nextTick();assert.deepEqual(fonts.style(labels.value[0]),{})
  scope.stop()
})
