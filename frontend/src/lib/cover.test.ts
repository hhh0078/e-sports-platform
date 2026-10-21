/** 文件作用：验证六标签极端布局、混合锚点、空值和长度限制，防止封面文字重叠。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { layoutLabels, COVER_HEIGHT, POSITIONS, newLabel, normalizeLabelText, gameCovers, randomGameCover } from './cover.ts'
import type { CoverLabel } from '../types/models'

/** 验证输出标签在边界内且任意两项不重叠。 */
function assertLayout(labels: Array<CoverLabel | null | undefined>, reserveBottom = 0, fullHeight = COVER_HEIGHT, reserveTop = 0) {
  const result = layoutLabels(labels, reserveBottom, fullHeight, reserveTop)
  for (let index = 0; index < result.length; index++) {
    const item = result[index]
    assert.ok([item.x,item.y,item.w,item.h,item.font_size].every(Number.isFinite))
    assert.ok(item.x >= 7.999 && item.y >= reserveTop + 7.999)
    assert.ok(item.x + item.w <= 352.001 && item.y + item.h <= fullHeight - reserveBottom - 7.999)
    for (const other of result.slice(index + 1)) {
      assert.ok(item.left + item.width <= other.left || other.left + other.width <= item.left || item.top + item.height <= other.top || other.top + other.height <= item.top, '文案不能重叠')
    }
  }
  return result
}

test('六条最长最大字号文案在全部九宫格锚点不越界不重叠', () => {
  for (const [position] of POSITIONS) {
    assert.equal(assertLayout(Array.from({ length: 6 }, (_, id) => ({ id, text: '三角洲护航任务陪练一起玩游戏', position, font_size: 24 }))).length, 6)
  }
})
test('混合位置、字号与英文文本仍能安全布局', () => {
  for (let seed = 0; seed < 100; seed++) {
    assertLayout(Array.from({ length: 6 }, (_, index) => ({ text: index % 2 ? '轻松找队友' : 'WWWWWWWWWWWW', position: POSITIONS[(seed + index) % 9][0], font_size: 12 + (seed * 7 + index) % 13 })))
  }
})
test('默认左下、无数据和超量标签处理', () => {
  assert.deepEqual(layoutLabels(null), [])
  assert.equal(layoutLabels([{ text: '极速接单' }])[0].left, 8 / 360 * 100)
  assert.equal(layoutLabels(Array.from({ length: 10 }, () => ({ text: '1' }))).length, 6)
  assert.equal([...layoutLabels([{ text: '123456789012345' }])[0].text].length, 12)
})
test('浅色封面标签采用深色字，深色标签采用白字', () => {
  assert.equal(newLabel('#E6C76D').color, '#101714')
  assert.equal(newLabel('#00e18a').color, '#101714')
  assert.equal(newLabel('#07130f').color, '#ffffff')
})

test('新建随机选底图，主动换图避开当前，单图空图库和无效随机数安全处理', () => {
  const game = { covers: ['delta-1.jpg','delta-2.jpg','delta-3.jpg','delta-1.jpg','',null], cover_url: 'fallback.jpg' }
  assert.deepEqual(gameCovers(game), ['delta-1.jpg','delta-2.jpg','delta-3.jpg'])
  assert.equal(randomGameCover(game,'',()=>0), 'delta-1.jpg')
  assert.equal(randomGameCover(game,'',()=>0.999), 'delta-3.jpg')
  for (const random of [0,0.3,0.9,1,-1,NaN,Infinity]) assert.notEqual(randomGameCover(game,'delta-1.jpg',()=>random), 'delta-1.jpg')
  assert.equal(randomGameCover({ covers:['only.jpg'] },'only.jpg'), 'only.jpg')
  assert.equal(randomGameCover({ covers:[],cover_url:'fallback.jpg' }), 'fallback.jpg')
  assert.equal(randomGameCover(null), '')
  assert.equal(game.covers.length, 6, '选择底图不得修改游戏配置')
})

test('截字保持旗帜、肤色、连接符表情与组合音标完整，并遵守后端十二码点上限', () => {
  assert.equal(normalizeLabelText('三角洲一起护航撤离轻松玩游戏'), '三角洲一起护航撤离轻松玩')
  assert.equal(normalizeLabelText('12345678901😀'), '12345678901😀')
  for (const emoji of ['👍🏽','🇨🇳','👨‍👩‍👧‍👦','👩🏽‍💻','e\u0301','1\ufe0f\u20e3']) {
    assert.equal(normalizeLabelText('12345678901'+emoji), '12345678901')
    assert.ok(normalizeLabelText(emoji.repeat(12)).endsWith(emoji))
    assert.ok([...normalizeLabelText(emoji.repeat(12))].length <= 12)
  }
  assert.equal(normalizeLabelText('  陪练\n组队\t '), '陪练 组队')
  assert.equal(normalizeLabelText(null), '')
})

test('默认左下按加入顺序向右排，颜色透明层保留且不修改原始配置', () => {
  const labels = Array.from({length:3},(_,index)=>({id:String(index),text:'队友',font_size:16,position:'bottom-left',color:'#00e18a',background:'transparent'}))
  const original = JSON.stringify(labels)
  const arranged = assertLayout(labels)
  assert.equal(new Set(arranged.map(item=>item.y)).size, 1)
  assert.ok(arranged[0].x < arranged[1].x && arranged[1].x < arranged[2].x)
  assert.equal(arranged[0].background, 'transparent')
  assert.equal(arranged[0].color, '#00e18a')
  assert.equal(JSON.stringify(labels), original)
})

test('emoji、极宽英文、中文在不同字号九宫格组合下都有安全间距', () => {
  const texts = ['WWWWWWWWWWWW','护航撤离任务一起轻松组队','😀😀😀😀😀😀😀😀😀😀😀😀','👨‍👩‍👧‍👦👩🏽‍💻','🇨🇳🇨🇳🇨🇳🇨🇳🇨🇳🇨🇳','MWi陪练👍🏽']
  for (let seed=0;seed<120;seed++) {
    const result = assertLayout(texts.map((text,index)=>({text,position:POSITIONS[(seed+index*3)%9][0],font_size:12+(seed+index)%13})))
    for (let index=0;index<result.length;index++) for (const other of result.slice(index+1)) {
      const item=result[index]
      assert.ok(item.x+item.w+3.999<=other.x || other.x+other.w+3.999<=item.x || item.y+item.h+3.999<=other.y || other.y+other.h+3.999<=item.y, '文案之间需要保留四像素间距')
    }
  }
  assert.ok(layoutLabels([{text:'WWWWWWWWWWWW',font_size:24}])[0].w >= 12*24+16, '宽英文不能按普通字符宽度压缩')
})

test('空配置、无效位置与字号不会导致无效坐标或渲染异常', () => {
  const result = assertLayout([null,undefined,{text:''},{text:'   '},{text:'有效',font_size:Infinity,position:'unexpected'},{text:'正常',font_size:-99}])
  assert.equal(result.length,2)
  assert.equal(result[0].position,'bottom-left')
  assert.equal(result[0].font_size,16)
  assert.equal(result[1].font_size,12)
})

/** 卡片底部留出信息叠层时，六条最长标签依然必须保持在可见区域内。 */
test('卡片信息叠层预留区域内六段文案无碰撞', () => {
  const labels = Array.from({ length: 6 }, (_, i) => ({ id: String(i), text: '最长文案十二字一起开心玩', font_size: 24, position: 'bottom-left' }))
  const placed = layoutLabels(labels, 50)
  for (const label of placed) {
    assert.ok(label.top >= 0)
    assert.ok(label.top + label.height <= (COVER_HEIGHT - 50) / COVER_HEIGHT * 100 + .001)
  }
  for (let a = 0; a < placed.length; a++) for (let b = a + 1; b < placed.length; b++) {
    const x = placed[a], y = placed[b]
    assert.ok(x.left + x.width <= y.left || y.left + y.width <= x.left || x.top + x.height <= y.top || y.top + y.height <= x.top)
  }
})

test('4:3卡片与详情按实际宽度保留48/92像素系统文字区，六标签在全部锚点均不碰撞', () => {
  const fullHeight = 360 / (4 / 3)
  for (const [width, reservedPixels] of [[138,48],[156,48],[173,48],[326,48],[320,92],[390,92],[600,92]]) {
    const reserved = reservedPixels * 360 / width
    for (const [position] of POSITIONS) {
      const labels = Array.from({length:6},(_,id)=>({id,text:id%2?'护航撤离一起认真玩游戏':'WWWWWWWWWWWW',font_size:24,position}))
      const result = assertLayout(labels,reserved,fullHeight)
      assert.equal(result.length,6)
      for (const label of result) {
        assert.equal(label.top,label.y/fullHeight*100)
        assert.equal(label.height,label.h/fullHeight*100)
        assert.ok((label.top+label.height)/100*(width/(4/3)) <= width/(4/3)-reservedPixels+.001, '用户文字不得进入系统标签与标题的真实像素区域')
      }
    }
  }
})

test('底部预留超过55画布单位仍全部生效，空间耗尽时不输出越界或负字号标签', () => {
  const labels = Array.from({length:6},(_,id)=>({id,text:'六条独立文案',font_size:24,position:POSITIONS[id%9][0]}))
  const result = assertLayout(labels,125,270)
  assert.equal(result.length,6)
  assert.deepEqual(layoutLabels(labels,270,270),[])
  assert.deepEqual(layoutLabels(labels,1000,270),[])
  assert.deepEqual(layoutLabels(labels,0,10),[])
  for (const fullHeight of [NaN,Infinity,0,-1]) assert.deepEqual(layoutLabels(labels,0,fullHeight),layoutLabels(labels))
  for (const reserved of [NaN,Infinity,-50]) assert.deepEqual(layoutLabels(labels,reserved),layoutLabels(labels))
})

test('沉浸详情顶部避让返回和胶囊、底部保留108像素，六标签不消失也不碰撞', () => {
  for (const width of [320,390,600]) for (const topPixels of [56,72,96]) for (const [position] of POSITIONS) {
    const top = topPixels * 360 / width, bottom = 108 * 360 / width
    const labels = Array.from({length:6},(_,id)=>({id,text:id%2?'一起撤离护航任务配合':'WWWWWWWWWWWW',font_size:24,position}))
    const result = assertLayout(labels,bottom,270,top)
    assert.equal(result.length,6)
    for (const label of result) {
      assert.ok(label.top/100*(width/(4/3)) >= topPixels, '自定义文字必须位于返回按钮与胶囊区域下方')
      assert.ok((label.top+label.height)/100*(width/(4/3)) <= width/(4/3)-108+.001)
      assert.ok(label.font_size>0)
    }
  }
  const labels=[{text:'顶部',position:'top-right',font_size:24}]
  assert.deepEqual(layoutLabels(labels,0,270,270),[])
  for (const top of [NaN,Infinity,-20]) assert.deepEqual(layoutLabels(labels,0,270,top),layoutLabels(labels,0,270))
})
