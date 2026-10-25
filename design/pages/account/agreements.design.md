# 协议阅读 · 复用开放式正文设计

2026-10-26：本轮目录状态见 [协议与隐私统一入口](agreements.index-v2.design.md)，无type显示目录，现有type正文路由兼容。

文件作用：对应 `frontend/src/pages/account/agreements.vue` 的先行设计和本轮轻量统一依据。已读取现有PNG、说明与真实源码；原图已经采用裸背景、细分隔和可读正文，本轮依主任务指导复用现有390×844图片，不重新生成、不新增装饰。

- 普通统一顶部返回与真实协议标题，无主导航底栏；正文直接排在页面背景上，不套panel、不放插画、大按钮或额外宣传标题。
- 元信息为11～12px次要文字的一行版本/更新时间，长内容可换行，不抢协议正文层级；只在有真实值时显示。
- 正文内容、标题、版本和更新时间全部来自管理员实际配置。图中“开机电竞用户协议”、章节和日期只示意排版，不插入重复标题、不用生成文案替换协议。
- 正文使用舒适行高和段落间距、可选择文本与自然滚动；16px水平边距。加载、失败重试和返回均保留，阅读不自动勾选登录同意。
- 当前图复用自本轮之前的内置ImageGen先行设计；旧说明和原PNG另存 `design/history/20261022-before-secondary-unify/pages/account/`。验收截图另存output。

## 品牌名称更新（2026-10-23）

将协议标题与正文中的平台品牌替换为开机电竞，保留原章节、正文排版与阅读区域。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 将协议标题与正文中的平台品牌替换为开机电竞，保留原章节、正文排版与阅读区域。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.
