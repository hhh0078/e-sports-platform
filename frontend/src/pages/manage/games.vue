<!-- 文件作用：独立配置游戏主题、服务分类和多封面素材，使用官方弹层保留完整游戏编辑流程。 -->
<template>
  <AppShell title="游戏配置" back>
    <ManageList v-model:query="query" placeholder="搜索游戏名称或标识" :loading="loading" :busy="editorBusy" :error="error" :empty="!games.length" :has-more="hasMore" :count="games.length" @search="loadData()" @more="loadData(true)" @retry="retryFailed" @prepare="showPage">
      <view class="game-list-heading"><text class="section-title">全部游戏</text><button class="text-btn" :disabled="editorBusy" @click="editGame()"><Icon name="plus" :size="17"/>添加游戏</button></view>
      <view v-for="item in games" :key="item.id" class="game-row"><view class="game-summary"><image class="game-cover" :src="assetUrl(item.cover_url)" mode="aspectFill"/><view class="game-copy"><text class="game-name">{{ item.name }}</text><text class="game-id">{{ item.id }}</text><view class="game-colors"><view v-for="key in colorKeys" :key="key.id" class="game-color" :style="{ background: item[key.id] }"/></view><text class="game-categories">{{ (item.categories || []).join(' · ') }}</text></view></view><view class="game-footer"><view class="game-state"><text :class="['game-enabled', { hidden: !item.enabled }]">● {{ item.enabled ? '已启用' : '已隐藏' }}</text><text class="game-sort">排序 {{ item.sort_order }}</text></view><view class="row"><button class="text-btn" :disabled="editorBusy" @click="openBanners(item)">游戏轮播</button><button class="btn btn-secondary btn-small" :disabled="editorBusy" @click="editGame(item)">编辑</button></view></view></view>
    </ManageList>
    <UniPopup ref="editorPopup" type="bottom" :is-mask-click="!editorBusy" background-color="transparent" @change="popupChanged"><scroll-view v-if="editing" scroll-y class="game-sheet"><view class="editor-heading"><text class="section-title">{{ newGame ? '添加游戏' : '编辑游戏' }}</text><button class="text-btn" :disabled="editorBusy" @click="closeEditor">关闭</button></view><view class="form-section editor-form">
      <view class="field"><text class="field-label">游戏标识（小写英文）</text><input v-if="newGame" v-model="gameForm.id" class="input" maxlength="30" :disabled="editorBusy" placeholder="例如 new-game"/><text v-else class="readonly-id">{{ gameForm.id }}</text></view>
      <view class="two-fields"><view class="field"><text class="field-label">游戏名称</text><input v-model="gameForm.name" class="input" maxlength="30" :disabled="editorBusy"/></view><view class="field"><text class="field-label">简称</text><input v-model="gameForm.short_name" class="input" maxlength="8" :disabled="editorBusy"/></view></view>
      <view class="field"><text class="field-label">主题标语</text><input v-model="gameForm.subtitle" class="input" maxlength="80" :disabled="editorBusy"/></view>
      <view class="two-fields"><view v-for="key in colorKeys" :key="key.id" class="field"><text class="field-label">{{ key.name }}</text><view class="color-input"><view class="color-preview" :style="{ background: gameForm[key.id] }"/><input v-model="gameForm[key.id]" class="color-value" maxlength="7" :disabled="editorBusy" placeholder="#83e7b7"/></view></view></view>
      <view class="field"><text class="field-label">服务分类 · 逗号分隔</text><input v-model="gameForm.categoriesText" class="input" maxlength="203" :disabled="editorBusy" placeholder="陪练教学,组队陪玩,任务协作"/><text class="editor-hint">1—12项，每项最多16字。</text></view>
      <view class="field"><text class="field-label">独立游戏图标 · 1:1</text><view class="upload-actions"><image v-if="gameForm.icon_url" class="game-icon-preview" :src="assetUrl(gameForm.icon_url)" mode="aspectFill"/><button class="btn btn-secondary btn-small" :disabled="editorBusy" @click="uploadCover('icon')">上传图标</button><button class="text-btn" :disabled="editorBusy || !gameForm.covers.length" @click="useFirstCover">使用首张封面</button></view></view>
      <view class="editor-heading cover-heading"><text class="field-label">封面素材 · 16:9</text><button class="text-btn" :disabled="editorBusy || gameForm.covers.length >= 12" @click="uploadCover('cover')"><Icon name="plus" :size="14"/>上传封面</button></view><view class="cover-library"><view v-for="cover in coverLibrary" :key="cover" :class="['library-cover', { selected: gameForm.covers.includes(cover) }]" @click="toggleCover(cover)"><image class="library-image" :src="assetUrl(cover)" mode="aspectFill"/><text v-if="gameForm.covers.includes(cover)" class="library-number">{{ gameForm.covers.indexOf(cover) + 1 }}</text></view></view><text class="editor-hint cover-hint">按点击顺序排列，首张为默认封面。选择1—12张，可取消后重新排序。</text>
      <view class="two-fields"><view class="field"><text class="field-label">排序 · 0—999</text><input v-model="gameForm.sort_order" type="number" class="input" :disabled="editorBusy"/></view><view class="field"><text class="field-label">对玩家可见</text><view class="enabled-control"><NativeSwitch :checked="gameForm.enabled" :disabled="editorBusy" :color="gameForm.primary_color" @change="changeEnabled"/><text class="editor-hint">{{ gameForm.enabled ? '已启用' : '暂时隐藏' }}</text></view></view></view>
      <button class="btn save-game" :disabled="editorBusy" @click="saveGame">{{ saving ? '正在保存…' : uploading ? '图片上传中…' : '保存游戏配置' }}</button>
    </view></scroll-view></UniPopup>
  </AppShell>
</template>
<script setup lang="ts">
import { ref, reactive, computed, watch, nextTick } from 'vue'
import UniPopup from '@dcloudio/uni-ui/lib/uni-popup/uni-popup.vue'
import AppShell from '../../components/AppShell.vue'
import ManageList from '../../components/ManageList.vue'
import NativeSwitch from '../../components/NativeSwitch.vue'
import Icon from '../../components/Icon.vue'
import { useManageResource } from '../../lib/manage-resource'
import { chooseUploadImage } from '../../lib/media'
import { state, init } from '../../lib/store'
import { createGame, updateGame } from '../../api/admin.api'
import { isCancelled } from '../../api/client'
import { assetUrl, notify, go } from '../../lib/format'
import type { Game } from '../../types/models'

/** 四项主题颜色与独立编辑字段。 */
type GameColorKey = 'primary_color' | 'bg_color' | 'surface_color' | 'text_color'
type GameDraft = Omit<Game, 'sort_order'> & { categoriesText: string; sort_order: string | number }
/** 官方弹层只暴露打开和关闭操作。 */
interface PopupController { open: (type?: string) => void; close: () => void }
const colorKeys: Array<{ id: GameColorKey; name: string }> = [{ id: 'primary_color', name: '主题强调色' }, { id: 'bg_color', name: '页面背景色' }, { id: 'surface_color', name: '卡片背景色' }, { id: 'text_color', name: '正文颜色' }]
const editing = ref(false), newGame = ref(false), saving = ref(false), uploading = ref(false), editorPopup = ref<PopupController | null>(null), uploadedCovers = ref<string[]>([])
const gameForm = reactive<GameDraft>(emptyGame()), editorBusy = computed(() => saving.value || uploading.value)
let choosingImage = false, editorVersion = 0
const { items: games, query, loading, error, hasMore, loadData, retryFailed, currentSession, sameSession, showPage } = useManageResource('games', { preserveDraft: () => choosingImage, clearDraft: clearGameDraft })
const coverLibrary = computed(() => [...new Set([...gameForm.covers, ...uploadedCovers.value, ...state.games.flatMap(item => [...(item.covers || []), item.cover_url].filter(Boolean))])])

/** 创建独立草稿，不修改已经发布的公共游戏主题。 */
function emptyGame(): GameDraft { return { id: '', name: '', short_name: '', subtitle: '', primary_color: '#83e7b7', bg_color: '#0a1110', surface_color: '#15201e', text_color: '#f2f5f3', cover_url: '', covers: [], icon_url: '', enabled: true, sort_order: Math.min(999, state.games.length + 1), categories: [], categoriesText: '' } }
/** 普通离开或账号变化时作废编辑操作并移除旧管理员草稿。 */
function clearGameDraft(): void { editorVersion++; choosingImage = false; saving.value = false; uploading.value = false; editing.value = false; uploadedCovers.value = []; Object.assign(gameForm, emptyGame()) }
/** 使用独立运营配置维护本游戏轮播，不改服务封面池。 */
function openBanners(item:Game):void{if(!editorBusy.value&&sameSession(currentSession()))go('/pages/manage/content/index?game_id='+encodeURIComponent(item.id))}
/** 新建或复制已有游戏配置，拷贝数组以免取消编辑影响全局状态。 */
function editGame(item?: Game): void { if (editorBusy.value || !sameSession(currentSession())) return; editorVersion++; newGame.value = !item; Object.assign(gameForm, emptyGame(), item ? JSON.parse(JSON.stringify(item)) : {}); gameForm.categoriesText = (gameForm.categories.length ? gameForm.categories : ['陪练教学', '组队陪玩', '任务协作']).join(','); editing.value = true }
/** 编辑或上传期间禁止关闭，其他关闭会清空未保存草稿。 */
function closeEditor(): void { if (!editorBusy.value) clearGameDraft() }
/** 官方遮罩关闭后同步清理草稿。 */
function popupChanged(event: { show: boolean }): void { if (!event.show && editing.value && !editorBusy.value) clearGameDraft() }
/** 点击顺序决定多封面的保存顺序，限制为服务端支持的12张。 */
function toggleCover(url: string): void { if (editorBusy.value || !editing.value) return; const index = gameForm.covers.indexOf(url); if (index >= 0) gameForm.covers.splice(index, 1); else if (gameForm.covers.length < 12) gameForm.covers.push(url); else notify('每个游戏最多选择12张封面') }
/** 用户主动选择时才把首张封面用于图标，仍保留独立图标字段。 */
function useFirstCover(): void { if (!editorBusy.value) gameForm.icon_url = gameForm.covers[0] || '' }
/** 将官方开关事件转换为纯布尔业务字段。 */
function changeEnabled(event: { detail: { value: boolean } }): void { if (!editorBusy.value) gameForm.enabled = event.detail.value }
/** 官方裁剪压缩上传；原生选图隐藏不丢草稿，旧账号结果不回填。 */
async function uploadCover(kind: 'icon' | 'cover' = 'cover'): Promise<void> {
  if (editorBusy.value || !editing.value || !sameSession(currentSession())) return
  if (kind === 'cover' && gameForm.covers.length >= 12) return notify('每个游戏最多选择12张封面')
  const session = currentSession(), version = editorVersion; uploading.value = true; choosingImage = true
  try { const url = await chooseUploadImage(kind === 'icon' ? 'game_icon' : 'game_cover'); if (version !== editorVersion || !sameSession(session, false)) return; if (kind === 'icon') gameForm.icon_url = url; else { if (!uploadedCovers.value.includes(url)) uploadedCovers.value.push(url); if (!gameForm.covers.includes(url)) gameForm.covers.push(url) }; notify('图片已处理并上传') }
  catch (err) { if (version === editorVersion && sameSession(session, false) && !isCancelled(err)) notify(err) }
  finally { if (version === editorVersion) { uploading.value = false; choosingImage = false; if (sameSession(session)) void loadData() } }
}
/** 按后端规则校验完整游戏草稿，只提交已声明的业务字段。 */
function gameBody(): Game {
  const id = gameForm.id.trim(), name = gameForm.name.trim(), shortName = gameForm.short_name.trim(), subtitle = gameForm.subtitle.trim()
  if (!/^[a-z][a-z0-9-]{1,29}$/.test(id) || Array.from(name).length < 2 || Array.from(name).length > 30 || !shortName || Array.from(shortName).length > 8 || Array.from(subtitle).length > 80) throw new Error('请填写有效游戏标识、2—30字名称和1—8字简称')
  if (colorKeys.some(key => !/^#[0-9a-f]{6}$/i.test(gameForm[key.id]))) throw new Error('颜色请使用六位十六进制值')
  if (gameForm.covers.length < 1 || gameForm.covers.length > 12) throw new Error('请选择1—12张游戏封面')
  const categories = [...new Set(gameForm.categoriesText.split(/[,，]/).map(item => item.trim()).filter(Boolean))]
  if (!categories.length || categories.length > 12 || categories.some(item => Array.from(item).length > 16)) throw new Error('服务分类需1—12项，每项最多16字')
  const sort = String(gameForm.sort_order).trim(); if (!/^\d{1,3}$/.test(sort)) throw new Error('排序需填写0—999的整数')
  return { id, name, short_name: shortName, subtitle, primary_color: gameForm.primary_color, bg_color: gameForm.bg_color, surface_color: gameForm.surface_color, text_color: gameForm.text_color, icon_url: gameForm.icon_url, covers: [...gameForm.covers], cover_url: gameForm.covers[0] || '', categories, enabled: gameForm.enabled, sort_order: Number(sort) }
}
/** 创建或更新后同步公共主题与列表，迟到写入结果不能关闭新管理员草稿。 */
async function saveGame(): Promise<void> {
  if (editorBusy.value || !editing.value || !sameSession(currentSession())) return
  let body: Game; try { body = gameBody() } catch (err) { notify(err); return }
  const session = currentSession(), version = editorVersion, creating = newGame.value; saving.value = true
  try { if (creating) await createGame(body); else await updateGame(body.id, body); if (version !== editorVersion || !sameSession(session)) return; editing.value = false; await init(true); if (version !== editorVersion || !sameSession(session)) return; await loadData(); if (version === editorVersion && sameSession(session)) notify('游戏主题已更新') }
  catch (err) { if (version === editorVersion && sameSession(session)) notify(err) }
  finally { if (version === editorVersion) saving.value = false }
}
// 始终按最新编辑状态同步官方弹层，避免快速取消后旧nextTick重新打开。
watch(editing, async () => { await nextTick(); if (editing.value) editorPopup.value?.open('bottom'); else editorPopup.value?.close() })
</script>
<style scoped>
.game-list-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;margin:46.1538rpx 0 7.6923rpx;min-width:0}.game-row{display:flex;flex-direction:column;gap:23.0769rpx;padding:38.4615rpx 0;border-bottom:1px solid var(--line-subtle);min-width:0}.game-summary{display:flex;align-items:flex-start;gap:23.0769rpx;min-width:0}.game-cover{display:block;flex-shrink:0;width:246.1538rpx;height:138.4615rpx;border-radius:9.6154rpx}.game-copy{display:flex;flex-direction:column;gap:9.6154rpx;min-width:0;flex:1}.game-name{font-size:30.7692rpx;font-weight:650;line-height:46.1538rpx;overflow-wrap:anywhere}.game-id,.game-categories{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.game-colors{display:flex;align-items:center;gap:11.5385rpx}.game-color{width:25rpx;height:25rpx;border-radius:50%;border:1px solid var(--line)}.game-footer,.game-state{display:flex;align-items:center;gap:23.0769rpx;min-width:0}.game-footer{justify-content:space-between}.game-enabled{font-size:23.0769rpx;color:var(--accent)}.game-enabled.hidden{color:var(--muted)}.game-sort{font-size:23.0769rpx;color:var(--muted)}.game-sheet{width:100vw;max-width:600px;max-height:85vh;height:85vh;margin:0 auto;padding:30.7692rpx 30.7692rpx calc(46.1538rpx + env(safe-area-inset-bottom));border-radius:30.7692rpx 30.7692rpx 0 0;background:var(--surface);color:var(--text)}.editor-heading{display:flex;align-items:center;justify-content:space-between;gap:23.0769rpx;min-width:0}.editor-form{padding-top:46.1538rpx;border-bottom:0}.two-fields{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:23.0769rpx;min-width:0}.color-input{display:flex;align-items:center;gap:15.3846rpx;min-width:0;border-bottom:1px solid var(--line);min-height:44px}.color-preview{width:38.4615rpx;height:38.4615rpx;border:1px solid var(--line);border-radius:7.6923rpx;flex-shrink:0}.color-value{width:100%;min-width:0;font-size:25rpx}.readonly-id{font-size:26.9231rpx;line-height:46.1538rpx;color:var(--muted);overflow-wrap:anywhere}.editor-hint{font-size:23.0769rpx;line-height:38.4615rpx;color:var(--muted);overflow-wrap:anywhere}.upload-actions{display:flex;align-items:center;flex-wrap:wrap;gap:19.2308rpx;min-width:0}.game-icon-preview{display:block;flex-shrink:0;width:84.6154rpx;height:84.6154rpx;border-radius:15.3846rpx}.cover-heading{margin-bottom:23.0769rpx}.cover-library{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:15.3846rpx;min-width:0}.library-cover{position:relative;height:0;padding-bottom:56.25%;overflow:hidden;border:3.8462rpx solid transparent;border-radius:11.5385rpx}.library-cover.selected{border-color:var(--accent)}.library-image{position:absolute;inset:0;display:block;width:100%;height:100%}.library-number{position:absolute;right:7.6923rpx;bottom:7.6923rpx;display:flex;align-items:center;justify-content:center;width:38.4615rpx;height:38.4615rpx;border-radius:50%;background:var(--accent);color:var(--bg);font-size:21.1538rpx}.cover-hint{display:block;padding:23.0769rpx 0 46.1538rpx}.enabled-control{display:flex;align-items:center;gap:15.3846rpx;min-height:44px;flex-wrap:wrap}.save-game{width:100%;margin-top:23.0769rpx}@media(max-width:350px){.game-cover{width:200rpx;height:112.5rpx}.game-summary{gap:19.2308rpx}.two-fields{gap:19.2308rpx}.game-state{gap:15.3846rpx}}
</style>
