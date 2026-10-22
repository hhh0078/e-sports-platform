/** 文件作用：执行真实俱乐部卡片脚本，验证游戏标签、真实准入规则、封面回退及整卡站内导航。 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { computed, reactive } from 'vue'
import { executable } from './sfc-test'
import { money, statusLabel } from './format'
import type { Club } from '../types/models'

/** 注入真实响应式Props与格式工具，只替代页面导航出口。 */
function harness(overrides: Partial<Club>={}) {
  const source=readFileSync(new URL('../components/ClubCard.vue',import.meta.url),'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)?.[1]
  assert.ok(source)
  const initial:Club={id:'club-A',name:'山海电竞',description:'认真游戏',owner_id:'owner-A',owner_name:'山海主理人',game_ids:['delta'],status:'active',verified:true,member_count:24,created_at:'',cover_url:'',detail_blocks:[],deposit_cents:0,assessment:'',...overrides}
  const props=reactive({club:initial}),state=reactive({games:[{id:'delta',short_name:'三角洲',cover_url:'/delta.jpg'},{id:'honor',short_name:'王者荣耀',cover_url:'/honor.jpg'},{id:'peace',short_name:'和平精英',cover_url:'/peace.jpg'}]}),navigations:string[]=[]
  const context=vm.createContext({computed,defineProps:()=>props,state,money,statusLabel,go:(url:string)=>navigations.push(url)})
  vm.runInContext(executable(source)+'\nglobalThis.subject={coverUrl,gameTags,statusText,admissionText,openDetail}',context)
  return{subject:context.subject,props,state,navigations}
}

test('俱乐部装修封面优先，无封面时回退到已配置游戏，均不虚构素材',()=>{
  const subject=harness({cover_url:'/club-cover.jpg'})
  assert.equal(subject.subject.coverUrl.value,'/club-cover.jpg')
  subject.props.club.cover_url=''
  assert.equal(subject.subject.coverUrl.value,'/delta.jpg')
  subject.props.club.game_ids=['missing-game']
  assert.equal(subject.subject.coverUrl.value,'')
})

test('俱乐部游戏标签保留真实顺序去重，配置改名后响应更新且未知游戏保留标识',()=>{
  const subject=harness({game_ids:['peace','delta','honor','peace','other']})
  assert.deepEqual(Array.from(subject.subject.gameTags.value,(tag:{id:string})=>tag.id),['peace','delta','honor','other'])
  subject.state.games[2].short_name='和平精英手游'
  assert.deepEqual(Array.from(subject.subject.gameTags.value,(tag:{name:string})=>tag.name),['和平精英手游','三角洲','王者荣耀','other'])
})

test('俱乐部状态按真实审核结果展示，待审和拒绝不标开放加入',()=>{
  const subject=harness()
  assert.equal(subject.subject.statusText.value,'平台认证')
  subject.props.club.verified=false
  assert.equal(subject.subject.statusText.value,'开放加入')
  subject.props.club.status='pending'
  assert.equal(subject.subject.statusText.value,'审核中')
  subject.props.club.status='rejected'
  assert.equal(subject.subject.statusText.value,'未通过')
})

test('俱乐部准入分别显示真实考核和规则押金，不混称平台已收款',()=>{
  const subject=harness()
  assert.equal(subject.subject.admissionText.value,'无额外门槛')
  subject.props.club.assessment='需通过试训'
  assert.equal(subject.subject.admissionText.value,'需考核')
  subject.props.club.deposit_cents=20050
  assert.equal(subject.subject.admissionText.value,'需考核 · 规则押金 ¥200.5')
  subject.props.club.assessment='  '
  assert.equal(subject.subject.admissionText.value,'规则押金 ¥200.5')
})

test('整卡进入对应俱乐部主页，标识不会插入其他查询参数',()=>{
  const subject=harness({id:'club A&mine=1'})
  subject.subject.openDetail()
  assert.deepEqual(subject.navigations,['/pages/community/clubs/detail?id=club%20A%26mine%3D1'])
})
