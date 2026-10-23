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
