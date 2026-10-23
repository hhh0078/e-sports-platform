# 个人中心 · 连续背景与紧凑头部设计

文件作用：本轮个人中心背景修复的先行局部设计依据；对应 `frontend/src/pages/main/me/me.vue`，保留 `me.png` 既有设计。设计图由内置 ImageGen 以真实当前页面局部编辑生成，再机械缩为390×844，不用验收截图代替设计图。

## 调整范围

- 顶部继续使用当前选择游戏的原有背景图和灰黑蒙版。蒙版按0%／55%／100%分段，末端必须落在当前主题 `--bg`，不能超过100%或留下硬边。三角洲示例底色为自然黑绿 `#0a1110`，切换游戏时使用对应主题色。
- 原约196px头部在390px屏宽收紧至约168px，320px小屏收紧至140px，避免小屏状态栏较矮却沿用大屏留白。头像、昵称和副文案整体上移约12px，保留微信状态栏和胶囊安全避让；订单区顶部留约8px，缩短上下两区距离。
- 订单区、快捷入口、菜单和底部安全区背景保持同一主题底色。页面框架及滚动背景均显式同色，不用多个不透明黑块拼接。
- 保留钱包轻薄表面和细边框，保留可提现、进行中、结算中、提现中各自资金分区。充值余额不能与可提现余额合并，底行明确写作“充值余额（仅消费）”。
- 保留所有入口、图标尺寸、底部四导航和原安全区，不新增视觉装饰或功能，不使用蓝紫渐变。菜单按正常文档流自然滚动，不为填满首屏拉大间距。

## 状态与验收

图示是访客零值，登录后文案与金额只取真实服务端。生成图中的文字或几何细节以本说明及实现参数为准，图中底栏上方留白不能变成固定高度占位。

320／390宽度检查访客与登录状态、顶部／滚动到底／切换导航后、不同游戏主题：封面末端没有水平接缝或黑色矩形；身份信息不压住原生胶囊；订单区没有多余头部空白；钱包层次仍清楚；不发生横向溢出。

## 生成记录

工具：内置 `image_gen`，未使用CLI或外部API。编辑目标：`.run/background-delta-before.png`。设计尺寸：390×844。第一次生成确定连续渐变，随后仅定向收紧头部几何并保存最终版本。

最终提示词要点：`ui-mockup precise local edit; preserve the current Delta Force hero, gray-black scrim, compact wallet, all menu rows and native safe areas; avatar/name/subtitle up about 12 logical pixels; hero about 168px; orders move upward about 28px; smooth hero lower-half fade into the exact current theme background; preserve recharge as a separate row named 充值余额（仅消费）; keep bottom navigation fixed; no extra decorations, no blue/purple gradients, no abrupt background sections.`
