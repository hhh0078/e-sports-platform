# 订单与售后 · 独立管理页

文件作用：为 `frontend/src/pages/manage/orders.vue` 提供先行390×844设计图、真实数据及售后入口规则；使用官方内置 ImageGen 独立生成，保存前未实现该页视觉。

## 布局与数据

- 延续平台控制台的黑绿自然主题，普通统一二级返回，16px水平内边距，无主底栏。
- 顶部搜索订单号、标题、买卖双方；刷新使用居中官方图标。搜索查询由领域API传至后端，不只过滤当前已加载页。
- 仅“全部订单”和“退款与争议”两个服务端支持的筛选；`refund=1` 深链预选售后，其他状态不得凭图杜撰。
- 开放式订单行：标题与真实金额、订单号、买方→打手、订单状态及售后状态、详情入口。长标题、昵称和编号换行，价格不被挤窄。
- 点击已有订单详情查看聊天室、证据和平台裁定；本页不复制独立仲裁大面板。列表数据、金额、时间、售后状态均来自接口，图中示例不是运行数据。
- 只允许当前平台管理员读取，玩家及俱乐部管理员不显示管理内容，后端仍核验权限。

## 加载、空态、错误与分页

- 首次加载使用统一轻加载；无记录时用简洁空态与“暂无匹配订单”，不造订单或金额。
- 失败显示接口错误及重试，不能伪装为空数据；刷新或加载更多期间禁重复请求。
- 每次20条，真实offset分页，追加去重；搜索/筛选变化重新从0读取。旧查询、旧账号、隐藏页面的响应不得回填。
- “加载更多”为轻量文字入口。二级页无底栏，内容正常纵向滚动。

## 验收点

- 320/390宽度无横向溢出，图标双轴居中，订单号及长昵称可换行。
- 两种筛选、`refund=1`预选、查询全库、分页及订单详情回跳可用。
- 退款处理中、买方申请、卖方拒绝、平台介入及退款完成文案准确，不能将处理中写成到账。
- 仅管理员可读取；跨账号及hide/show不保留旧管理数据。
- 生成图中文字、示例状态与原生安全区以本说明和真实产品文案为准。

## 完整生成提示词

Use case: ui-mockup.
Asset type: Production-ready standalone Chinese WeChat Mini Program ADMIN ORDER LIST design image, one exact flat portrait 390×844 screen, no device frame.
Primary request: design the independent “订单与售后” admin page for 集结 game services marketplace. This is an actual manageable list with navigation, search, refunds, evidence, not a dashboard. Match an understated dark editorial UI: background #0a1110, text #edf3ef, muted #91a299, mint accent #83e7b7, fine #ffffff12 separators, no purple or blue gradient. Every content area has16px horizontal inset, buttons36px compact or44px primary, rounded8px only if needed, all thin official icons centered.
Layout: safe status area with9:41, below one shared secondary navigation with a thin left back chevron and centered exact title “订单与售后”; NO bottom main navigation. At y100 a quiet single full-width search input with small search icon and placeholder “搜索订单号、标题或用户”, a compact refresh glyph on right. Below at y160 two horizontally bounded text tabs “全部订单” (mint underline selected) and “退款与争议”. Below four OPEN list rows separated only by thin fullwidth rules, NOT a stack of raised cards. Each row: strong readable Chinese service title left, exact money right; next line muted order number; next line buyer → provider; last line compact status and muted售后状态 plus right chevron. Use clearly artificial sample labels only: first “三角洲行动 · 任务协作” / “¥88.00” / “JJ202610220001” / “玩家甲 → 打手乙” / “进行中” / “查看订单”; second “王者荣耀 · 组队陪玩” / “¥40.00” / “JJ202610220002” / “玩家丙 → 打手丁” / “退款申请” / “等待卖方处理”; third “和平精英 · 陪练教学” / “¥60.00” / “JJ202610220003” / “玩家戊 → 打手己” / “平台介入” / “查看证据”; fourth “金铲铲之战 · 战术教学” / “¥30.00” / “JJ202610220004” / “玩家庚 → 打手辛” / “已完成”. Finance sample numbers are layout examples only, no fake totals, no merchant settings or keys. All rows lead to existing order detail chat/evidence/after-sales; DO NOT add a separate adjudication form, filters not listed, random charts, extra metrics, empty illustrations or pills everywhere.
Below rows quiet centered “加载更多” text entry. Keep body comfortably readable, no tiny squeezed typography. Natural vertical scrolling is allowed. This image should show only this one independent page, not multiple screens or a contact sheet. Layout visual precision, professional minimal product UI. Chinese text should be exact; explanatory documentation takes precedence if glyphs vary.
