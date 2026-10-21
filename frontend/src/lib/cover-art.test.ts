/** 文件作用：直接执行真实CoverArt脚本，验证测量宽度后的实际像素预留、比例切换和监听清理。 */
import test from 'node:test'
import type { TestContext } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, ref, reactive, watch, nextTick, effectScope } from 'vue'
import { executable } from './sfc-test'
import { layoutLabels, POSITIONS } from './cover'
import type { CoverLabel } from '../types/models'

/** 测量器替身保留真实选择器、nextTick及尺寸回调顺序。 */
interface TestQuery { in: () => TestQuery; select: () => TestQuery; boundingClientRect: (callback: (rect: { width: number }) => void) => TestQuery; exec: () => void }

/** 只控制原生测量结果，实际布局与生命周期仍使用生产组件脚本。 */
function harness(t: TestContext, initialWidth: number, options: { aspectRatio?: number; reserveBottom?: number; reserveBottomPixels?: number; reserveTopPixels?: number; minLabelAreaPixels?: number } = {}) {
  const source = readFileSync(new URL('../components/CoverArt.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1]
  assert.ok(source)
  const labels: CoverLabel[] = Array.from({length:6},(_,id)=>({id,text:'六条最长独立文字标签护航',font_size:24,position:'bottom-left'}))
  const props = reactive({ labels, aspectRatio:16/9, reserveBottom:0, reserveBottomPixels:0, reserveTopPixels:0, minLabelAreaPixels:0, ...options })
  const mounted: Array<()=>void> = [], unmounted: Array<()=>void> = [], shown: Array<()=>void> = [], resized: Array<()=>void> = []
  let actualWidth = initialWidth, callback: ((rect: {width:number})=>void) | undefined, measureCount = 0
  const query: TestQuery = { in:()=>query,select:()=>query,boundingClientRect:read=>{callback=read;return query},exec:()=>{measureCount++;callback?.({width:actualWidth})} }
  const context = vm.createContext({ computed,ref,watch,nextTick,layoutLabels,defineProps:()=>props,withDefaults:(value:object)=>value,getCurrentInstance:()=>({proxy:{}}),
    mainNavigation:reactive({revision:0}),onMounted:(hook:()=>void)=>mounted.push(hook),onShow:(hook:()=>void)=>shown.push(hook),onBeforeUnmount:(hook:()=>void)=>unmounted.push(hook),
    uni:{createSelectorQuery:()=>query,onWindowResize:(hook:()=>void)=>resized.push(hook),offWindowResize:(hook:()=>void)=>{const index=resized.indexOf(hook);if(index>=0)resized.splice(index,1)}},
  })
  const scope = effectScope()
  scope.run(()=>vm.runInContext(executable(source.replace(/^[\t ]+(?=import\b)/gm,''))+'\nglobalThis.subject={arranged,reserved,reservedTop,canvasHeight,width,ratio,coverStyle,labelStyle}',context))
  t.after(()=>{unmounted.forEach(hook=>hook());scope.stop()})
  return { subject:context.subject,props,mounted,unmounted,shown,resized,
    /** 通过已注册的原生窗口监听触发真实测量。 */
    resize(width:number){actualWidth=width;resized.forEach(hook=>hook())},
    /** 查询停止测量后是否仍有已排队的回调污染旧组件。 */
    measureCount:()=>measureCount,
  }
}

/** 排空真实nextTick测量与Vue侦听。 */
async function flush():Promise<void>{await new Promise<void>(resolve=>setImmediate(resolve))}

test('CoverArt按测量宽度动态转换48像素预留，4:3和尺寸变化后仍不侵入标题区',async t=>{
  const subject=harness(t,138,{aspectRatio:4/3,reserveBottomPixels:48})
  subject.mounted.forEach(hook=>hook());await flush()
  assert.equal(subject.subject.coverStyle.value.paddingTop,'75%')
  for(const width of [138,173,326]){
    subject.resize(width);await flush()
    assert.equal(subject.subject.width.value,width)
    assert.equal(subject.subject.reserved.value,48*360/width)
    assert.equal(subject.subject.arranged.value.length,6)
    for(const label of subject.subject.arranged.value){assert.ok((label.top+label.height)/100*(width/(4/3))<=width/(4/3)-48+.001);assert.equal(subject.subject.labelStyle(label).visibility,'visible')}
  }
  const before=subject.measureCount()
  subject.unmounted.forEach(hook=>hook());subject.shown.forEach(hook=>hook());await flush()
  assert.equal(subject.resized.length,0)
  assert.equal(subject.measureCount(),before)
})

test('CoverArt旧reserveBottom保持画布单位，比例或像素参数无效时安全回退',async t=>{
  const subject=harness(t,180,{reserveBottom:50})
  subject.mounted.forEach(hook=>hook());await flush()
  assert.equal(subject.subject.reserved.value,50)
  assert.equal(subject.subject.coverStyle.value.paddingTop,'56.25%')
  assert.deepEqual(subject.subject.arranged.value,layoutLabels(subject.props.labels,50))
  subject.props.aspectRatio=NaN;subject.props.reserveBottomPixels=Infinity;await flush()
  assert.equal(subject.subject.ratio.value,16/9)
  assert.equal(subject.subject.reserved.value,50)
  subject.props.aspectRatio=4/3;subject.props.reserveBottomPixels=92;await flush()
  assert.equal(subject.subject.reserved.value,184)
  for(const label of subject.subject.arranged.value)assert.ok(label.y+label.h<=270-184-8+.001)
})

test('CoverArt沉浸模式按实测宽度同时保留顶部安全区与底部系统信息区',async t=>{
  const subject=harness(t,320,{aspectRatio:4/3,reserveTopPixels:96,reserveBottomPixels:108})
  subject.mounted.forEach(hook=>hook());await flush()
  assert.equal(subject.subject.reservedTop.value,108)
  assert.equal(subject.subject.arranged.value.length,6)
  for(const label of subject.subject.arranged.value){assert.ok(label.y>=108+7.999);assert.ok(label.y+label.h<=270-121.5-7.999)}
  subject.resize(390);await flush()
  assert.equal(subject.subject.reservedTop.value,96*360/390)
  assert.equal(subject.subject.arranged.value.length,6)
  subject.props.reserveTopPixels=NaN;await flush()
  assert.equal(subject.subject.reservedTop.value,0)
})

test('320宽沉浸详情按文字带增高，六条最长标签全部锚点实际字号至少10像素',async t=>{
  const subject=harness(t,320,{aspectRatio:4/3,reserveTopPixels:96,reserveBottomPixels:108,minLabelAreaPixels:72})
  subject.mounted.forEach(hook=>hook());await flush()
  assert.equal(subject.subject.canvasHeight.value/360*320,292)
  for(const [position] of POSITIONS){
    subject.props.labels=subject.props.labels.map(label=>({...label,position}));await flush()
    const labels=subject.subject.arranged.value
    assert.equal(labels.length,6)
    for(let index=0;index<labels.length;index++){
      const label=labels[index],height=subject.subject.canvasHeight.value/360*320
      assert.ok(label.font_size*320/360>=10,'增加封面高度后不能再用3—5像素实际字号掩盖拥挤')
      assert.ok(label.top/100*height>=96)
      assert.ok((label.top+label.height)/100*height<=height-108+.001)
      for(const other of labels.slice(index+1))assert.ok(label.left+label.width<=other.left||other.left+other.width<=label.left||label.top+label.height<=other.top||other.top+other.height<=label.top)
    }
  }
  subject.props.minLabelAreaPixels=0;await flush()
  assert.equal(subject.subject.canvasHeight.value,270,'默认关闭文字带增高时保持卡片原4:3比例')
})

test('不同标签数量的文字带使用最终高度布局，实际宽度扩大后仍可回到原比例',async t=>{
  const subject=harness(t,320,{aspectRatio:4/3,reserveTopPixels:96,reserveBottomPixels:108,minLabelAreaPixels:24})
  const original=subject.props.labels.slice()
  subject.mounted.forEach(hook=>hook());await flush()
  for(const [count,area] of [[1,24],[2,24],[3,48],[4,48],[5,72],[6,72]]){
    subject.props.labels=original.slice(0,count);subject.props.minLabelAreaPixels=area;await flush()
    const height=subject.subject.canvasHeight.value/360*320
    assert.ok(Math.abs(height-Math.max(240,96+108+area+16))<.001)
    assert.equal(subject.subject.arranged.value.length,count)
  }
  subject.resize(600);await flush()
  assert.equal(subject.subject.canvasHeight.value,270)
  subject.props.minLabelAreaPixels=Infinity;await flush()
  assert.equal(subject.subject.canvasHeight.value,270)
})
