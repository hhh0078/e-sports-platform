# 游戏配置 · 独立列表与完整编辑

文件作用：为 `frontend/src/pages/manage/games.vue` 提供先行390×844独立设计、完整游戏编辑及图片素材规则；使用官方内置 ImageGen 独立生成，保存前未实现该页视觉。

## 列表布局与数据

- 延续控制台黑绿自然色、16px水平内边距、细分隔、普通统一二级返回，无主底栏。
- 搜索游戏名称或标识，轻刷新；“添加游戏”是安静加号文字入口，不使用突兀大按钮。
- 每个开放式游戏行展示16:9缩略封面、名称/标识、四主题色圆点、分类、启用状态、真实排序，以及36px紧凑编辑入口。
- 图中三个游戏仅布局示意，运行必须读取真实API；游戏官方素材仍使用已有素材库或管理员上传，不把生成图的示意缩略图抽出当官方素材。
- 仅当前平台管理员可读写；其他身份显示访问限制且不发管理接口请求。

## 完整编辑弹层

- 使用官方 `uni-popup` 底部弹层，最高85vh，内部正常滚动；圆角16px，16px水平内边距，安全区补白。标题“添加游戏”或“编辑游戏”，辅助关闭为文字入口。
- 必须保留：新建时游戏标识（已有标识只读）、游戏名称、简称、主题标语、四个六位十六进制主题色（强调色/背景/卡片/正文）、服务分类（支持中文或英文逗号）、排序、启用开关。
- 图标使用1:1官方裁剪压缩上传；封面使用16:9官方裁剪压缩上传。图标和封面独立存储，保留“使用首张封面”辅助操作；每种用途均调用既有统一上传流程和后端尺寸校验。
- 多封面素材以3列16:9缩略图展示，选中细主题边框+序号；按点击选择顺序保存，第一张作为默认封面，至少一张。可选择已有素材、上传新素材、取消选择和重新排序。
- 保存前校验标识、名称/简称、颜色、分类、至少一张封面、有效整数排序；只发送业务字段，不发编辑临时文本。底部唯一44px主按钮“保存游戏配置”。
- 上传/保存期间禁重复点击及中途切换编辑对象。原生选图hide/show保留同账号草稿，取消选图不丢表单；账号变化及普通离开清除草稿。上传或保存迟到回调不能覆盖新管理员。

## 状态与验收

- 加载、无结果、失败重试均为真实状态；20条分页，搜索变化回到offset0，旧结果不混页。
- 游戏编辑及新建后列表与公共bootstrap同步刷新，取消不得改动已保存全局主题。
- 320/390宽度无横向溢出，名称分类可换行，主题色和图标垂直居中；弹层长内容可滚动到保存。
- 逐项验证四颜色、分类、多封面顺序、独立图标、启用、排序、取消、上传取消及跨账号竞态，没有拆页丢功能。
- 图中文字以此说明和真实文案为准；生成图封面/排序为布局示例，不能作为真实配置默认值。

## 完整生成提示词

Use case: ui-mockup. Asset type: one independent professional Chinese WeChat Mini Program “游戏配置” admin page, flat390×844portrait, no device frame.
Design the standalone game catalog and theme management page for 开机电竞. Exact visual system background#0a1110,text#edf3ef,muted#91a299,mint#83e7b7,fine#ffffff12dividers,nopurplebluegradient;16pxhorizontalpadding,openeditorialrows,restrained8pxradii,thincenteredofficialicons. Safe status area9:41 then shared navigation thin left back arrow, centered “游戏配置”, no bottomtabs. At y100 search line with search icon and exactplaceholder “搜索游戏名称或标识” and quiet refresh glyph. Below section heading “全部游戏” left and quiet small plus “添加游戏” right. Show3generousopenrows separated byfinehorizontalrules. Each row includes left16:9cropped game cover thumbnail around104×59, right name16pxbold then muted short ID. Under name4tinycolorroundswatches (mint,nearblack,darkgreen,offwhite) and muted categories. Bottom of each row small status “已启用” or “已隐藏” and “排序 1” left, compact36pxoutlined “编辑” right. Exact sample rows “三角洲行动” / “delta” / “任务协作 · 组队陪玩” / “已启用” / “排序 1”; “王者荣耀” / “honor” / “陪练教学 · 组队陪玩” / “已启用” / “排序 2”; “和平精英” / “peace” / “组队陪玩 · 陪练教学” / “已启用” / “排序 3”. Thumbnail artwork should just be recognizable thematic crop: tacticalsoldierdarkgreen / Chinese fantasyhero warmgold / battlegroundhelmetdesert. Artis illustrative only, no generated fake official logos. Below list quiet “加载更多” text. Layout may naturally scroll; do not squeeze font. Editing occurs in separate officialuni-popupbottomsheet maximum85vh with complete form, not visible in this base-list image. DO NOT bake editor inputs into list. Future editor supports game id/name/shortname/subtitle,4themehexcolors,categories,1:1iconupload,multiple16:9coverswithselectionorder,enabledswitch,sort,one44pxsavebutton; show none of this form on list image. No dashboardmetrics,no massivecards,nounsupportedstatusfilters,nounneededEnglishdecorations. Generate exactly one independentpage, not a contact sheet. All data is illustrative sample content only.
