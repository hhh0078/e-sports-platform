# 俱乐部管理 · 独立模块设计

文件作用：对应 `frontend/src/pages/manage/clubs.vue` 的先行ImageGen设计与实施依据；2026-10-22管理模块拆分。

- 共享返回标题“俱乐部管理”，无主底栏；q搜索、服务端状态筛选与真实分页，安全深链status=pending。筛选结果完整依赖服务端，不在第一页做局部过滤。
- 开放列表保留俱乐部图标/名称/审核状态/认证状态、管理员、成员数、简介；主页、成员、订单为安静入口。待审核拒绝/通过并认证，已通过按真实认证状态认证或取消认证。
- 紧凑操作36px，官方图标固定盒双轴居中。图中混合状态仅说明行样式；选中待审核时不得混入已通过数据。长名称及辅助入口320px不得互挤溢出。

- 自然黑绿主题，白色主文案、灰绿辅助、薄荷主色；生成文字误差以真实业务和本说明为准。原生胶囊/状态栏按系统安全区实现，设计中没有胶囊的示例不可据此去掉避让。空态轻量，错误可重试，内容自然滚动。

## 完整生成提示词

Use case: ui-mockup. Create a professional Chinese WeChat Mini Program independent administration secondary screen, flat portrait exact390:844, no device frame or margins. Approved editorial system: dark natural charcoal-green #0a1110, white #edf4ef, muted #9dafa3, mint #8de8b6. 16px insets,24px section gaps,thin separators,44px search input,36px compact actions,official outline icons perfectly centered. Native statusbar and top-right capsule safearea, shared back arrow and page title beneath,NO main bottom navigation. Open lists on background,NOT nested cards,no dashed frames,no giant buttons,no illustrations,no blue/purple gradient,no decorativeEnglish. Clear readable Chinese 16px row titles,14pxsecondary, quiet real-business status tags. Existing API uses real paginated q search; design placeholders are illustrative, no fake charts/funds. Title俱乐部管理. q search俱乐部名称、管理员 withcompact搜索. Horizontalunderlinetabs 全部 待审核 已通过 已拒绝,待审核selected,constrainedwidthscroll. Openlistclubs separatedbythinlines. Firstclubimage72x72,leftsmallofficialgamethemeemblem; main 晨曦电竞,small待审核tag,管理员北辰 · 12名成员,description 专注三角洲行动任务协助; quietview主页/成员/订单links; right36px拒绝 and通过并认证mintoutline. Second长风俱乐部已通过small已认证tag,管理员晚风 · 24名成员,shortdescription; auxiliary主页/成员/订单 and36px取消认证. Third未认证approvedclub36px认证. Currentdatarealfrombackend; samplesillustrativeonly. No giant fullwidthbuttons,nofakebalance,not enrollmentclubpublicpage,no merchants. Centered加载更多textafterlist. Natural scroll contentdo notshrink. Standard backtitleandcapsule.
