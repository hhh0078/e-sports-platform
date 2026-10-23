# 操作审计 · 只读管理记录

文件作用：为 `frontend/src/pages/manage/audit.vue` 提供先行390×844独立审计页设计与只读查询规则；使用官方内置 ImageGen 独立生成，保存前未实现该页视觉。

## 布局与内容

- 统一黑绿自然主题、16px水平边距、普通二级返回，无主底栏。搜索行的图标居中，操作区保持轻量。
- 搜索操作、操作者或目标，使用具名API查询服务端真实审计；没有硬编码记录、统计、状态筛选或伪造时间线。
- 开放式记录以细分隔区分：动作与时间、操作人ID、目标ID、可选说明。动作、ID和说明都作为纯文本渲染，长ID及JSON说明换行且不水平撑开页面。
- 不设置修改、删除审计的操作。图中 `admin-demo` 等仅布局样例，运行时完全由后端记录驱动。
- 仅平台管理员可访问，俱乐部管理员和玩家不读取/展示管理数据。

## 空错态及分页

- 首次加载为轻加载；没有记录显示“暂无操作记录”，不造演示记录填空。
- 读取失败保留明确错误与重试，不能把错误隐藏为无数据。20条分页，原始offset前进且去重；“加载更多”为文字入口。
- 输入新搜索词后重新查询从0读取；不能将新搜索的第二页追加进旧列表。隐藏及换账号作废旧请求，恢复显示核验当前管理员。
- 无编辑弹层；页面只读。时间使用现有共享格式，生成图文案存在误字时以实际接口和说明为准。

## 验收点

- 320/390宽度，长操作名、ID和说明可换行，动作与时间不重叠；图标双轴居中。
- 实际新增审核/冻结/游戏编辑后可从审计查到对应记录。
- 搜索、分页、空态、错误重试及非管理员权限通过；旧账号响应不回填。

## 完整生成提示词

Use case: ui-mockup. Asset type: one independent production Chinese WeChat Mini Program readonly “操作审计” admin page, exactflat390×844portrait, nodeviceframe.
Match 开机电竞’s naturalblackgreeneditorialdesign:background#0a1110,text#edf3ef,muted#91a299,mint#83e7b7,thin#ffffff12divider,16pxinsets,officialthincenteredicons,nobluepurplegradient. Native safestatus9:41 then sharedsecondarynavleftbackchevron, centered exacttitle “操作审计”, nobottomnavigation. At y100 quietfullwidth searchinput smallsearchicon placeholder “搜索操作、用户或对象” and light refreshglyph. Below showactualrecordshapedreadonlyopenrows with fine horizontal separators, no timeline decoration, no cards orfakecharts. Each row bold action left and time right, next lines “操作人” + ID and “目标”+ID, optional fullwrappedplain-text note. Exactlayoutsample rows: “service.active” / “10-22 13:08” / “操作人 admin-demo” / “目标 service-demo-01” / “服务审核通过”; “club.active” / “10-22 12:45” / “操作人 admin-demo” / “目标 club-demo-01” / “俱乐部申请通过并认证”; “user.frozen” / “10-22 12:10” / “操作人 admin-demo” / “目标 user-demo-02” / “冻结账号”; “game.save” / “10-22 11:56” / “操作人 admin-demo” / “目标 delta” / “更新游戏配置”. These IDs are obviously illustrative only, no fabricated real identities or financial claims. Use ample lineheight and readable14pxsecondarytext, longobjectIDs wrap instead of squeezing. Below rows quiet centered “加载更多”. Page contains only search and actualauditrecords; no modify/delete buttons, no extra unsupportedstatusfilters,noheroillustration,nofakecounts,noadjudicationcontrols. Distinctstronghierarchy,butminimalquietadminutility. Generateexactlyonescreen,nota multi-screenboard. Chinese exact wording desirable; implementation documentation remains authoritative.

## 2026-10-24 原生下拉刷新与触底加载

沿用本页已批准设计图、原生页面滚动、主题与版式，不新增嵌套滚动容器。下拉只刷新当前可读数据，所有成功、错误、访客、权限拒绝和被保护的忙碌状态都结束原生刷新动画；不调用支付、审核、创建、撤销等写接口。已有数据刷新期间保留，失败可重试；同一列表触底只允许一条请求，刷新或筛选后的旧页不得追加。分页页脚统一使用官方uni-load-more的轻状态，只有失败时显示重试入口，移除普通“加载更多”按钮。
保持服务端搜索、筛选与20条原始offset，刷新重置分页，分页去重不改变服务端offset；管理编辑、原生确认和提交草稿保持不变。

错误重试统一恢复刚才失败的首读或追加请求，不根据旧页脚推断；搜索输入是草稿，触底不得自动提交新关键词，需明确搜索后才应用。页面身份和筛选变化同步撤销旧手势加载状态，原业务版本守卫继续隔离迟到回包。
