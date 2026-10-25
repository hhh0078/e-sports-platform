# 用户管理 · 独立模块设计

2026-10-26：本轮最新见 [安全摘要列表](users.list-v2.design.md) 及 [用户详情](users/detail.design.md)，列表不再显示简介或直接冻结。

文件作用：对应 `frontend/src/pages/manage/users.vue` 的先行ImageGen设计与实施依据；2026-10-22管理模块拆分。

- 共享返回标题“用户管理”，无主底栏；q搜索昵称或用户ID、真实分页、加载/失败重试均保留。
- 开放列表按头像、昵称、角色、用户ID、状态、简介排列；仅显示服务端真实角色。冻结/解冻使用36px紧凑动作，冻结自身管理员无可用按钮，操作前确认。
- 图中ID、角色拼写及人物仅为版式样例，使用真实字段且长昵称/ID换行或省略，禁止照抄示例数据。无不支持的状态筛选。

- 自然黑绿主题，白色主文案、灰绿辅助、薄荷主色；生成文字误差以真实业务和本说明为准。原生胶囊/状态栏按系统安全区实现，设计中没有胶囊的示例不可据此去掉避让。空态轻量，错误可重试，内容自然滚动。

## 完整生成提示词

Use case: ui-mockup. Create a professional Chinese WeChat Mini Program independent administration secondary screen, flat portrait exact390:844, no device frame or margins. Approved editorial system: dark natural charcoal-green #0a1110, white #edf4ef, muted #9dafa3, mint #8de8b6. 16px insets,24px section gaps,thin separators,44px search input,36px compact actions,official outline icons perfectly centered. Native statusbar and top-right capsule safearea, shared back arrow and page title beneath,NO main bottom navigation. Open lists on background,NOT nested cards,no dashed frames,no giant buttons,no illustrations,no blue/purple gradient,no decorativeEnglish. Clear readable Chinese 16px row titles,14pxsecondary, quiet real-business status tags. Existing API uses real paginated q search; design placeholders are illustrative, no fake charts/funds. Title用户管理. Underheader a quiet underlined search 搜索昵称或用户ID with compact 搜索 text. Open list of3users separated byhairlines. Each48pxavatarleft;mainnickname北辰 withsmallrole普通用户; nextmutedID…7e21 and状态正常; next bio 三角洲行动接单者. Right36px描边compact冻结. Seconduser晚风 role俱乐部管理员状态正常,冻结. Thirduser云川 role平台管理员状态正常,show不可冻结自己asmutedtextinsteadbutton. Fourth partially visibleuser状态冻结,compact解冻. Clear freeze action requires runtimeconfirm,notshown. Listpaginationbottomquiet已展示4位用户 · 加载更多. No status filter unless backend supports, primary search is enough. No merchantconfiguration. Generous ordinary flow, screencomfortablewith5rows, nofakebalances.

## 2026-10-24 原生下拉刷新与触底加载

沿用本页已批准设计图、原生页面滚动、主题与版式，不新增嵌套滚动容器。下拉只刷新当前可读数据，所有成功、错误、访客、权限拒绝和被保护的忙碌状态都结束原生刷新动画；不调用支付、审核、创建、撤销等写接口。已有数据刷新期间保留，失败可重试；同一列表触底只允许一条请求，刷新或筛选后的旧页不得追加。分页页脚统一使用官方uni-load-more的轻状态，只有失败时显示重试入口，移除普通“加载更多”按钮。
保持服务端搜索、筛选与20条原始offset，刷新重置分页，分页去重不改变服务端offset；管理编辑、原生确认和提交草稿保持不变。

错误重试统一恢复刚才失败的首读或追加请求，不根据旧页脚推断；搜索输入是草稿，触底不得自动提交新关键词，需明确搜索后才应用。页面身份和筛选变化同步撤销旧手势加载状态，原业务版本守卫继续隔离迟到回包。
