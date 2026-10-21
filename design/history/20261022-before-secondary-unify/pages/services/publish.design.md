# 服务发布预览新版设计

文件作用：对应 `frontend/src/pages/services/publish.vue` 的先行设计图及统一ServiceCard编辑预览约束；2026-10-22根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/services/`。

- 保留普通统一顶部返回和发布服务标题；该页属于编辑表单，不使用封面沉浸返回，也不挂主导航底栏。
- 游戏选择压为紧凑一行，封面预览标题右侧放轻量“换一张”，避免与正文重复大按钮。
- 预览必须使用和列表同一ServiceCard，不能另造“图片+下方独立标题”组件。源封面仍16:9，展示裁切约4:3；标签在上、标题在下，均独立叠在图内渐变区；渐变上端透明、底部完全不透明并衔接主题表面色。
- 用户封面文案仍是可编辑独立图层，按设置位置显示；默认左下时需和固定标签/标题共同计算安全空间，不重叠、不出界，不合成进图片。图中示例文字的行序以真实防碰撞规则为准。
- 价格/成交数在图下正常流。草稿标题、价格来自当前表单；空值使用简洁占位，预览可隐藏无意义的成交等可选元数据。
- 封面文字编辑继续保留添加/删除、文字、字色、背景、位置、字号；支持最多六条与全部位置测试，不删除既有表单业务。
- 服务信息区在下方自然滚动；底部唯一业务操作44px提交审核，正文留足操作栏与系统安全距离。水平16px、8/12/16/24间距，官方图标固定盒居中。

## 生成与实施边界

内置ImageGen编辑旧图，完成后等比缩至390×844。生成游戏画面仅示意版式，运行时使用已核验官方游戏资产。系统状态栏/微信胶囊按真实API尺寸避让，不以生成图省略的胶囊为准。实际验收图保存在output。

## 完整编辑指令

Use case: ui-mockup. Edit the attached Chinese390x844 publish-service design, preserve existing pagepurpose and editorcontrols, dark#0a1110 panel#121d18 mint#8de8b6 theme, officialstyledcenteredicons,16pxinsets,12pxradii,44pxmainbuttons, ordinarysharedtopnavbar 发布服务 withleftback and nativecapsuleavoidance. Do not invent features. Compact选择游戏 panel to one44pxpickerrow, game三角洲行动. Heading封面预览 has a quiet换一张 link at right. Replace the current raw16:9picturepreview with the SAME unified ServiceCard usedonplayerhome: image displaycropabout4:3 (sourceart16:9), tacticaldamart. Inside imagebottom a transparent-at-top to fullyopaque #121d18 verticalgradient; independent mintuserwords 耐心沟通 稳定撤离 placed safelyabovemetadata; independent smalllabels 俱乐部认证 三角洲 任务协作 inrowabove boldwhite title 大坝任务协作 · 稳定撤离, title INSIDE image. No title outsideimage. Immediatelybelowimage a compactnormalflowrow ¥68/小时 left 已成交0right. No giant extrapreviewinformationpanel. Then 封面文字 section with smalloutlined 添加文字 inheader; include input耐心沟通 withremoveicon, fivecolorchips, transparent/dark/lightbackgroundchips,位置左下 selector,字号slider; can showremainingcontrols belowviewport asnatural scroll. Atbottom hint 服务信息 and partiallyvisible titleinput iffits; no duplicatebusinessfeatures. Sticky44pxmint 提交审核 actionandsafebottomspace. Need natural realistic首屏, notlongimage. NO primarybottomtabnav,no decorativeEnglish,no purple/bluegradient. UIbuttonsnotbakedimagesinruntime.
