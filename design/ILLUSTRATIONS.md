# 集结场景插画素材

文件作用：记录本项目三张场景插画及手柄图标的来源、用途、尺寸、透明背景约束与生成提示词，供后续页面设计和素材维护使用。

## 来源与文件

2026-10-22 使用 Codex 内置 imagegen 工具独立生成三张同系列原创插画，未引用外部图库或具体游戏角色、商标。使用真实 alpha 透明背景，所有页面文案仍由 UI 元素承载，图片中没有文字、昵称或模拟业务数据。

项目目录：`frontend/src/static/illustrations/`；小程序引用路径为 `/static/illustrations/文件名.png`。

| 文件 | 最终尺寸 | 文件体积 | 主要用途 |
| --- | --- | --- | --- |
| empty-orders.png | 360 × 240 | 109,481 B / 106.9 KiB | 订单空态 |
| empty-services.png | 360 × 240 | 87,986 B / 85.9 KiB | 工作台起步空态 |
| profile-pass.png | 320 × 180 | 44,429 B / 43.4 KiB | 历史/可选身份插画，当前个人中心不使用 |
| gamepad-icon.png | 64 × 64 | 2,196 B / 2.1 KiB | 暗色界面的24px线性手柄图标 |

## 视觉与使用约束

- 低饱和薄荷绿、鼠尾草绿、米白和炭灰，轻微立体、细描边、手绘纸感；适配 `#0a1110` 等暗色页面。
- 空态插画建议按 180 × 120 逻辑像素居中展示，可随容器等比缩放。`profile-pass.png` 作为历史/可选素材保留；未来经设计确认复用时可约128 × 72逻辑像素放在名片右侧，不能自动加回当前个人中心。
- 当前个人中心使用所选游戏的 `game.cover_url` 全宽底图，叠加中性灰黑透明蒙版及到底部主题背景的渐隐。昵称、简介、头像和统计保持独立元素，约240px的顶部区域包含原生安全区，不将UI烘焙进图片。当前设计与说明见 `design/pages/main/me/`，上一轮图保留于 `design/history/20261022-before-secondary-unify/pages/`。
- 使用 `aspectFit`，完整展示物体；不额外加彩色矩形背景、霓虹光晕或外框，不拉伸。
- 订单与工作台空态插画不能替代真实订单卡片、服务封面或按钮。说明、登录提示与操作均保留为独立文本和官方控件。
- 通过 macOS `sips --resampleWidth` 等比例缩小，保留原始 alpha；最终三张 PNG 均小于 150 KB。已目视检查图形完整、无文字，并核验 PNG 透明通道与实际透明像素。

## 完整生成提示词

三张图片均使用以下公共提示词，再拼接对应追加提示词；使用内置工具模式，未调用 CLI 或自行安装图片生成依赖。

```text
Use case: stylized-concept. Asset type: small original editorial illustration for a refined Chinese game services mobile app on dark charcoal surfaces. Style: consistent family of sophisticated low-saturation hand-drawn editorial illustration, gentle dimensional paper shapes, delicately textured matte surfaces, fine imperfect charcoal outlines, soft restrained highlights, crafted and warm, not glossy 3D, not neon, not cartoon emoji. Palette limited to desaturated mint green, warm ivory, sage and charcoal gray. Real transparent alpha background, no background rectangle, no checkerboard, no halo. Make dark shapes outlined with a subtle ivory edge so they remain legible over #0a1110. No letters, no numbers, no writing, no labels, no logos, no watermark, no human faces, no copied game characters. Compact complete composition with generous clear alpha margin around all edges, no cropped objects. Keep scene simple, well balanced, readable when reduced to 180px wide.
```

### empty-orders.png

用途：订单列表的访客与无订单空态，双手柄、展开任务地图和空白小票表达组队与开始一单。

工具参数：`transparent_background: true`。原始生成文件：`/Users/Zhuanz/.codex/generated_images/01a15f53-aee3-74c1-bc00-be4cd4918060/exec-aad005d9-fd22-44a9-a351-70511e537446.png`（原件保留于本机，项目引用下方静态资源）。

追加提示词：

```text
Primary request: two different friendly game controllers resting next to an unfolded small quest map, with a neatly curled blank order receipt tucked at the map edge. The map has only a simple dashed route and two tiny geometric waypoints, no writing or data. The two controllers suggest players teaming up. Small softly shaded grounded arrangement viewed at a gentle oblique top-down angle, no room or furniture behind it. Dominant mint and ivory with charcoal accents. Landscape 3:2 canvas, intended export 360x240. No text anywhere.
```

### empty-services.png

用途：接单工作台尚未发布服务时的起步场景，耳机、手柄、任务卡及铅笔表达准备服务。

工具参数：`transparent_background: true`。原始生成文件：`/Users/Zhuanz/.codex/generated_images/01a15f53-aee3-74c1-bc00-be4cd4918060/exec-9d854f1a-b745-47e6-8dec-75ec2846b48e.png`（原件保留于本机，项目引用下方静态资源）。

追加提示词：

```text
Primary request: a quietly inviting starter desk vignette with a carefully drawn over-ear gaming headset leaning against one game controller, beside a small blank task card and a pencil. The task card has one empty square outline and a few subtle horizontal strokes that do not resemble letters, no stats or fake data. Objects on one thin short matte sage desktop ledge, no room or backdrop. Gentle oblique top-down angle and calm ample spacing, mint headset cushions, warm ivory controller, charcoal details. Landscape 3:2 canvas, intended export 360x240. Match the subtle handmade editorial family described above. No text anywhere.
```

### profile-pass.png

历史设计用途：个人中心横向身份名片右侧的小场景，身份徽章、手柄和联机轨迹表达个人游戏身份。当前个人中心已改为游戏封面与灰黑蒙版，不再引用该素材；保留原文件和以下生成提示词供溯源及以后经设计确认的其他用途。

工具参数：`transparent_background: true`。原始生成文件：`/Users/Zhuanz/.codex/generated_images/01a15f53-aee3-74c1-bc00-be4cd4918060/exec-350b0dbf-ea29-43e1-a136-d1e9be9d4f4e.png`（原件保留于本机，项目引用下方静态资源）。

追加提示词：

```text
Primary request: a compact horizontal gaming identity vignette for the right side of a personal profile card: one simple clipped-corner physical game membership badge without any writing or numbers, a small game controller in front, and one thin curling connection route linking two tiny circular nodes. Badge contains only a clean abstract four-point emblem, no fake identity data. Warm ivory badge, soft mint controller and route, charcoal edge details. Calm asymmetrical composition weighted to the right, objects fully contained, low visual complexity. Wide 16:9 canvas intended export 320x180, genuine transparent alpha. No text anywhere.
```


### gamepad-icon.png

用途：官方图标库未覆盖的游戏手柄线性图标；白色单色描边，与24px官方图标的线条视觉相近，置于暗色背景中。小程序引用路径 `/static/illustrations/gamepad-icon.png`。

工具参数：内置 imagegen，`transparent_background: true`，无外部参考图。原始生成文件：`/Users/Zhuanz/.codex/generated_images/01a15f54-0a72-7993-af02-7a3f68bffb88/exec-9deb9bd1-2179-4dfa-8a70-c6e0306bbee7.png`（原件保留）。仅使用macOS sips等比缩至64×64，未安装依赖，未构建SDK。

校验：PNG为64×64、8位RGBA、2,196字节；4,096像素中3,496像素完全透明，有效可见像素RGB通道最低247，保持白色单色描边。已目视确认轮廓完整、透明背景和光学居中。该小图标不套用前三张场景插画的绿/米白配色。

完整提示词：

```text
Use case: logo-brand. Asset type: a single transparent PNG interface icon for a dark-background Chinese gaming marketplace. Generate exactly ONE clean simple game-controller outline icon, centered on a square transparent canvas. White monochrome stroke ONLY, no fills anywhere, no shadow, no glow, no gradient, no color, no text, no logo, no decorative scene. Visual style must match standard 24px uni-icons: rounded 2px white strokes at24px apparent size, optical balanced, round linecaps and joins, crisp minimal geometry. Controller outer silhouette is one familiar symmetrical ergonomic horizontal gamepad, two grip lobes, top flat gentlycurved body. Inside: one plus-shaped Dpad atleft, two small hollow circular buttons atright, verysmall center dots optional. Use large clear shapes so it remains readable at24px. Leave about12% transparent padding around the icon on all sides. All white lines on true transparent alpha background, not checkerboard, no opaque background. Output square 1:1, final icon will downscale to64x64 and render at24px.
```
