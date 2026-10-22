# 玩家首页沉浸式设计

文件作用：对应 `frontend/src/pages/main/player/player.vue` 的先行设计图和新版实施约束；2026-10-22 根据用户最新视觉要求编辑。旧图和旧说明归档于 `design/history/20261022-before-immersive/pages/`。

- 主视图没有应用顶栏、页面标题和返回按钮；只预留原生状态栏及微信胶囊安全空间。游戏主视觉可铺在安全区下方，品牌、口号和按钮不得被胶囊遮挡。
- 游戏目录固定单行横向滚动，右端固定“全部”轻入口；点击后打开游戏选择弹框，禁止排成两行，也不得挤窄相邻内容。
- 服务卡为一个业务组件。标签在上、标题在下，都是封面内独立元素；封面底部使用由上透明、下方完全不透明的主题背景渐变。文字层不得合成进图片。
- 价格、成交数在封面下方正常流；不重复展示标题，不叠到图片文字区。保留用户自定义文字的独立性及防重叠规则。
- 页面水平16px，间距8/12/16/24，控件44/36/28px；底部仅主容器挂载一次导航。
- 图中素材只示意版式，运行时使用项目官方游戏图；状态栏、胶囊的实际尺寸以系统API为准。

- 服务封面源图保持16:9；本次新版图内展示区按设计约4:3裁切，为标签、标题及自定义文案留足空间。所有独立文字按实际显示宽高适配，不能按源图尺寸写死坐标。

## 生成方式与指令

使用内置 ImageGen 编辑现有同路径图片，完成后机械等比缩至390×844；先设计后实施。生成图中的文字误差和系统胶囊缺省，以本说明和真实系统安全区为准。

Use case: ui-mockup. Edit the attached existing 390x844 Chinese mini-program design. Deliver exactly one flat portrait UI screenshot, same 390:844 aspect, no device frame, no surrounding space. Preserve the restrained dark charcoal-green theme #0a1110, panels #121d18, mint #8de8b6, 16px insets, 12px radii, clean Chinese type, 44px primary buttons and fine centered icons. Preserve correct Chinese words and existing screen purpose. Native WeChat status bar and native top-right capsule are present; reserve capsule-safe space. This is a design reference only: game artwork remains illustrative and app runtime uses official assets. Do not invent blue/purple gradients. Primary request: revise the player home. There is NO app navigation bar, NO centered page title, NO back button on this primary tab. Keep the attractive full-width tactical game hero behind the native status/capsule area; place 集结 brand and meaningful headline below the capsule safe zone, hero height about190px total. Keep one compact 发布需求 button. Replace the existing two-row game grid with exactly ONE horizontally scrolling row inside a restrained panel at y205: five compact game icons visible with labels 三角洲 王者荣耀 和平精英 暗区 无畏契约, next partial icon clipped naturally at right, and pinned narrow 全部 with down chevron at far right. No second game row. This 全部 opens a game-selection modal (not shown); distinguish it from content filter. Under game row keep a compact category chip row and 精选服务 heading. Rework every service card as ONE component: 16:9 image region; at its bottom an elegant vertical gradient from transparent upper image to fully opaque #121d18 at bottom; small independent overlay labels on a row ABOVE the bold service title on the image; title remains inside image at its lowest line. Labels example 俱乐部认证 三角洲 任务协作; title 大坝协作 · 稳定撤离. NO separate title or labels beneath the image, NO author row across image. Immediately below image, in normal flow dark body, only mint price ¥68/小时 left and 成交236 right, compact 34px height. Four cards in two columns with12px gaps, all image/title boundaries align. Bottom one persistent nav 玩家 打手 订单 我的; 玩家 active. Card title wraps at most2 lines within image and must not collide with tags or price. Keep high visual quality and density, not crowded.
