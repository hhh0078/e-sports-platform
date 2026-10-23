# 图文审核

文件作用：对应 `frontend/src/pages/manage/moderation.vue` 的先行界面与交互依据。

- 先行图为 `moderation.png`，390×844，由内置 ImageGen 生成后机械缩放。沿用深绿自然色控制台、统一二级返回、16设计像素边距和44像素触控下限；实现尺寸使用rpx，不设置主底栏。
- 顶部显示待人工、AI处理中、已拒绝三个真实数量；加载时为占位。六种状态加全部筛选横向受控滚动，不挤压页面。AI待审与处理中分别保留状态。
- 内容为紧凑分隔列表：类型图标、字段、所属对象、摘要、状态和时间。选择后在官方 `FinancePopup` 浮层读取完整详情；图中下半屏仅表示打开的详情浮层，并非内嵌表单。
- 详情显示安全纯文本，图片从管理员专用鉴权接口下载为本地临时路径，不将令牌拼接到地址。原文不按HTML渲染。
- 人工通过和拒绝必须二次确认；拒绝原因1—500字且显示计数。过期、已通过、已拒绝任务不可再操作；正在AI处理时允许管理员接管，确认框明确先完成的有效结果生效；若竞争结果已完成，409提示重新读取。仅待人工任务可请求AI重试。来源区分人工、AI、可信图片沿用、关联图片撤销和系统过期。
- 页面隐藏、退出、换号后立即清除原文、原因和图片临时文件，不接受旧请求回填。初始化故障在权限判断外可见，可重新核验身份。
- 无AI配置时继续保留人工待审，不能让用户发布或管理员人工操作被AI故障阻塞。实际文案与服务端状态为准，图中数字只表示排版。

## 生成提示词

Use case: ui-mockup. Single flat Chinese 390:844 mobile admin UI, no device frame. Restrained dark natural green #0a1110/#15201e, white text and mint accent, no blue/purple. Shared back title 图文审核. Three compact real-data placeholders, status filters, thin-divider moderation rows, and lower detail sheet with plain text, required rejection reason counter, muted red 拒绝 and mint 通过 actions at 44px touch height. 16px margins, readable body, no promotional illustration or bottom tabbar. Original generated source saved under Codex generated_images and final project reference is moderation.png.

## 2026-10-24 原生下拉刷新与触底加载

沿用本页已批准设计图、原生页面滚动、主题与版式，不新增嵌套滚动容器。下拉只刷新当前可读数据，所有成功、错误、访客、权限拒绝和被保护的忙碌状态都结束原生刷新动画；不调用支付、审核、创建、撤销等写接口。已有数据刷新期间保留，失败可重试；同一列表触底只允许一条请求，刷新或筛选后的旧页不得追加。分页页脚统一使用官方uni-load-more的轻状态，只有失败时显示重试入口，移除普通“加载更多”按钮。
审核队列沿用服务端next_cursor，触底追加；完整原文弹层、拒绝原因草稿及审核处理中不被下拉清除。

错误重试统一恢复刚才失败的首读或追加请求，不根据旧页脚推断；搜索输入是草稿，触底不得自动提交新关键词，需明确搜索后才应用。页面身份和筛选变化同步撤销旧手势加载状态，原业务版本守卫继续隔离迟到回包。
