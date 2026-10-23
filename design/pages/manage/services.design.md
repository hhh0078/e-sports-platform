# 服务审核 · 独立模块设计

文件作用：对应 `frontend/src/pages/manage/services.vue` 的先行ImageGen设计与实施依据；2026-10-22管理模块拆分。

- 共享返回标题“服务审核”，无主底栏；q搜索、服务端状态筛选与真实分页，允许安全枚举深链status=pending。支持的筛选必须以当前API为准，禁止仅筛选已取第一页造成漏单。
- 16:9缩略图、标题/状态、发布者/分类、金额和简介正常流；下方查看详情轻入口，待审核允许拒绝/通过，已通过允许暂停。36px操作，细分隔，无重复套卡。
- 图中多个不同状态行仅展示各行版式；实际待审核选中时只能显示服务端待审核结果。游戏缩略图使用项目已有官方素材，不截取设计图作为运行图片。

- 自然黑绿主题，白色主文案、灰绿辅助、薄荷主色；生成文字误差以真实业务和本说明为准。原生胶囊/状态栏按系统安全区实现，设计中没有胶囊的示例不可据此去掉避让。空态轻量，错误可重试，内容自然滚动。

## 完整生成提示词

Use case: ui-mockup. Create a professional Chinese WeChat Mini Program independent administration secondary screen, flat portrait exact390:844, no device frame or margins. Approved editorial system: dark natural charcoal-green #0a1110, white #edf4ef, muted #9dafa3, mint #8de8b6. 16px insets,24px section gaps,thin separators,44px search input,36px compact actions,official outline icons perfectly centered. Native statusbar and top-right capsule safearea, shared back arrow and page title beneath,NO main bottom navigation. Open lists on background,NOT nested cards,no dashed frames,no giant buttons,no illustrations,no blue/purple gradient,no decorativeEnglish. Clear readable Chinese 16px row titles,14pxsecondary, quiet real-business status tags. Existing API uses real paginated q search; design placeholders are illustrative, no fake charts/funds. Title服务审核. q search服务标题、发布者 withcompact搜索. Beneathhorizontalcompact underlinefilter 全部 待审核 已通过 已暂停 已拒绝,default待审核selectedmint, maxwidthconstrained scrollat320. Open list3services. Each16:9coverthumb96x54usesrecognizablegame-artdescription（三角洲operator,王者hero,和平精英soldier）only designvisual. Righttitle 三角洲护航·任务协助,small待审核badge,发布者北辰 · 任务协助,price¥39.00; shortdescription一行. Below quiet查看详情 → atleftand36px拒绝/通过 actionsright. Second王者荣耀双排陪玩,待审核,¥25.00,anotherpair. Third已通过serviceexamplehas暂停36pxanddetail. Alldataexampleonly. Thin separators and normalflow,not cardswithin cards. Last加载更多textcenter. Truepaginationandserverfilter, no frontendonly currentpagefilter. Keep room for standard nativecapsule.

## 2026-10-24 原生下拉刷新与触底加载

沿用本页已批准设计图、原生页面滚动、主题与版式，不新增嵌套滚动容器。下拉只刷新当前可读数据，所有成功、错误、访客、权限拒绝和被保护的忙碌状态都结束原生刷新动画；不调用支付、审核、创建、撤销等写接口。已有数据刷新期间保留，失败可重试；同一列表触底只允许一条请求，刷新或筛选后的旧页不得追加。分页页脚统一使用官方uni-load-more的轻状态，只有失败时显示重试入口，移除普通“加载更多”按钮。
保持服务端搜索、筛选与20条原始offset，刷新重置分页，分页去重不改变服务端offset；管理编辑、原生确认和提交草稿保持不变。

错误重试统一恢复刚才失败的首读或追加请求，不根据旧页脚推断；搜索输入是草稿，触底不得自动提交新关键词，需明确搜索后才应用。页面身份和筛选变化同步撤销旧手势加载状态，原业务版本守卫继续隔离迟到回包。
