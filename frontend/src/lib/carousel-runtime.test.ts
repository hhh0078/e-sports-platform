/** 文件作用：执行真实双swiper组件，验证300ms字幕、混合空文案、真实字号与隐藏/配置切换的计时清理。 */
import test from 'node:test'
import type {TestContext} from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import vm from 'node:vm'
import {computed,effectScope,nextTick,reactive,ref,watch} from 'vue'
import postcss from 'postcss'
import {layoutCarouselSlides} from './carousel-layout'
import {executable} from './sfc-test'
import type {ContentSlide} from '../types/content'

/** 使用真实生产布局与Vue计算，仅把时间推进及生命周期放到测试控制。 */
function harness(t:TestContext){
  const slides:ContentSlide[]=[{id:'text',kind:'image',image_url:'a.jpg',labels:[{text:'第一张文案',font_size:24}],target:{kind:'service',id:'a'}},{id:'empty',kind:'image',image_url:'b.jpg',labels:[],target:{kind:'article',id:'b'}}]
  const props=reactive({slides,fallback:'',active:true,geometry:{width:320,baseHeight:182,reserveTop:110,reserveBottom:17}}),events:Array<{name:string;value:unknown}>=[],opened:unknown[]=[],jobs=new Map<number,{due:number;run:()=>void}>(),hooks={show:()=>{},hide:()=>{},unmount:()=>{}};let clock=0,sequence=0
  const context=vm.createContext({computed,ref,watch,layoutCarouselSlides,defineProps:()=>props,withDefaults:(value:unknown)=>value,defineEmits:()=>((name:string,value:unknown)=>events.push({name,value})),openContentTarget:(target:unknown)=>opened.push(target),setTimeout:(run:()=>void,delay:number)=>{const id=++sequence;jobs.set(id,{due:clock+delay,run});return id},clearTimeout:(id:number)=>jobs.delete(id),onShow:(run:()=>void)=>{hooks.show=run},onHide:(run:()=>void)=>{hooks.hide=run},onBeforeUnmount:(run:()=>void)=>{hooks.unmount=run}})
  const source=readFileSync(new URL('../components/ContentCarousel.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1];assert.ok(source);const scope=effectScope();scope.run(()=>vm.runInContext(executable(source)+';globalThis.carousel={imageChanged,imageIndex,textIndex,arrangement,labelStyle,canAnimate,textHasLabels,open}',context));t.after(()=>scope.stop())
  return{page:context.carousel,props,events,opened,jobs,hooks,
    /** 只推进已排队timer，Vue队列由测试单独nextTick。 */
    advance(milliseconds:number):void{clock+=milliseconds;for(const[id,job]of [...jobs].sort((a,b)=>a[1].due-b[1].due))if(job.due<=clock){jobs.delete(id);job.run()}},
  }
}

test('混合有字/空字图片的字幕晚300ms，同步当前caption而稳定高度不随切图变化',async t=>{
  const s=harness(t);await nextTick();assert.equal(s.page.textHasLabels.value,true);const height=s.page.arrangement.value.height,heightEvents=s.events.filter(item=>item.name==='height-change').length
  s.page.imageChanged({detail:{current:1}});assert.equal(s.page.imageIndex.value,1);assert.equal(s.page.textIndex.value,0);s.advance(299);await nextTick();assert.equal(s.page.textHasLabels.value,true);s.advance(1);await nextTick();assert.equal(s.page.textIndex.value,1);assert.equal(s.page.textHasLabels.value,false);assert.equal(s.events.filter(item=>item.name==='caption-change').at(-1)?.value,false)
  assert.equal(s.page.arrangement.value.height,height);assert.equal(s.events.filter(item=>item.name==='height-change').length,heightEvents)
  s.page.imageChanged({detail:{current:0}});s.advance(300);await nextTick();assert.equal(s.page.textHasLabels.value,true)
  const label=s.page.arrangement.value.slides[0][0];assert.ok(Number.parseFloat(s.page.labelStyle(label).fontSize)>=10)
})
test('快速滑动只保留最后timer，隐藏与返回同步当前图而不复活旧字幕',async t=>{
  const s=harness(t);s.page.imageChanged({detail:{current:1}});s.advance(150);s.page.imageChanged({detail:{current:0}});assert.equal(s.jobs.size,1);s.advance(150);assert.equal(s.page.textIndex.value,0);s.advance(150);assert.equal(s.jobs.size,0)
  s.page.imageChanged({detail:{current:1}});s.hooks.hide();assert.equal(s.jobs.size,0);assert.equal(s.page.canAnimate.value,false);s.advance(1000);assert.equal(s.page.textIndex.value,0);s.hooks.show();await nextTick();assert.equal(s.page.textIndex.value,1);assert.equal(s.page.textHasLabels.value,false)
})
test('失活、换配置、卸载均清timer；非法原生索引不触发字幕或跳转',async t=>{
  const s=harness(t);for(const current of [-1,2,1.5,NaN])s.page.imageChanged({detail:{current}});assert.equal(s.jobs.size,0)
  s.page.imageChanged({detail:{current:1}});s.props.active=false;assert.equal(s.jobs.size,0);s.page.open(s.props.slides[0]);assert.equal(s.opened.length,0);s.props.active=true
  s.page.imageChanged({detail:{current:1}});s.props.slides=[{id:'new',kind:'image',image_url:'new.jpg',labels:[],target:null}];assert.equal(s.jobs.size,0);s.advance(1000);await nextTick();assert.equal(s.page.imageIndex.value,0);assert.equal(s.page.textIndex.value,0);assert.equal(s.page.textHasLabels.value,false)
  s.hooks.unmount();s.hooks.show();assert.equal(s.page.canAnimate.value,false)
})
test('旧文字过渡期间点击图片使用当前图目标，文字层不吞掉图片手势',async t=>{
  const s=harness(t);s.page.imageChanged({detail:{current:1}});assert.equal(s.page.textIndex.value,0);s.page.open(s.props.slides[1]);assert.equal((s.opened[0]as{kind:string;id:string}).kind,'article');assert.equal((s.opened[0]as{id:string}).id,'b')
  const source=readFileSync(new URL('../components/ContentCarousel.vue',import.meta.url),'utf8'),css=source.match(/<style scoped>([\s\S]*?)<\/style>/)?.[1];assert.ok(css);const declarations=new Map<string,string>();postcss.parse(css).walkRules(rule=>{for(const selector of rule.selectors)rule.walkDecls('pointer-events',decl=>{declarations.set(selector,decl.value)})});for(const name of ['.carousel-labels','.carousel-label-plane','.carousel-label','.carousel-dots'])assert.equal(declarations.get(name),'none')
  assert.equal([...source.matchAll(/<swiper\s/g)].length,2);assert.doesNotMatch(source,/<view class="carousel-label-plane"[^>]*@click/)
})

test('等值新图组和热返回保留当前图片，几何重测不会把轮播归零',async t=>{
  const s=harness(t);s.page.imageChanged({detail:{current:1}});s.advance(300);await nextTick()
  s.props.slides=JSON.parse(JSON.stringify(s.props.slides));s.props.geometry={...s.props.geometry,width:390};await nextTick()
  assert.equal(s.page.imageIndex.value,1);assert.equal(s.page.textIndex.value,1);assert.equal(s.jobs.size,0)
  s.hooks.hide();s.hooks.show();await nextTick();assert.equal(s.page.imageIndex.value,1);assert.equal(s.page.textIndex.value,1)
})
test('真实图文审核投影变化立即撤销旧timer，并采用当前图片的新文字和目标',async t=>{
  const s=harness(t);s.page.imageChanged({detail:{current:1}});assert.equal(s.jobs.size,1)
  const replacement=JSON.parse(JSON.stringify(s.props.slides)) as ContentSlide[]
  replacement[1].labels=[{text:'审核后的新文字',font_size:18}];replacement[1].target=null;s.props.slides=replacement;await nextTick()
  assert.equal(s.jobs.size,0);assert.equal(s.page.imageIndex.value,1);assert.equal(s.page.textIndex.value,1);assert.equal(s.page.textHasLabels.value,true)
  s.page.open(s.props.slides[1]);assert.equal(s.opened.at(-1),null)
  s.advance(1000);await nextTick();assert.equal(s.page.textIndex.value,1)
})
test('后台排序变化按图片ID定位，当前图片移除或显式清空才安全回到首项',async t=>{
  const s=harness(t);s.page.imageChanged({detail:{current:1}});s.advance(300);s.props.slides=[s.props.slides[1],s.props.slides[0]];await nextTick()
  assert.equal(s.page.imageIndex.value,0);assert.equal(s.page.textIndex.value,0);assert.equal(s.props.slides[s.page.imageIndex.value].id,'empty')
  s.props.slides=[s.props.slides[1]];await nextTick();assert.equal(s.page.imageIndex.value,0);assert.equal(s.page.textHasLabels.value,true)
  s.props.slides=[];await nextTick();assert.equal(s.page.imageIndex.value,0);assert.equal(s.page.textIndex.value,0);assert.equal(s.page.textHasLabels.value,false);assert.equal(s.jobs.size,0)
})
test('轮播历史背景色仅留作配置数据，真实样式强制透明且无标签圆角',async t=>{
  const s=harness(t);s.props.slides[0].labels[0].background='#FF6600';await nextTick();const label=s.page.arrangement.value.slides[0][0],style=s.page.labelStyle(label)
  assert.equal(label.background,'#FF6600');assert.equal(style.background,'transparent');assert.equal(style.backgroundImage,undefined)
  const source=readFileSync(new URL('../components/ContentCarousel.vue',import.meta.url),'utf8'),css=source.match(/<style scoped>([\s\S]*?)<\/style>/)?.[1];assert.ok(css)
  postcss.parse(css).walkRules(rule=>{if(rule.selectors.includes('.carousel-label'))rule.walkDecls(decl=>{assert.ok(!decl.prop.startsWith('background')&&decl.prop!=='border-radius','纯文字CSS不得恢复底色或标签圆角')})})
})
