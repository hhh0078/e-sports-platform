# 打手工作台新版设计

文件作用：对应 `frontend/src/pages/main/worker/worker.vue` 的先行设计图与实施约束；2026-10-22 根据用户最新要求编辑。旧版保存在 `design/history/20261022-before-immersive/pages/`。

- 本页无应用顶栏、无“打手工作台”重复标题、无返回按钮；状态栏/胶囊安全区由主容器统一提供。头像昵称可左排，但设置和其他动作必须在胶囊下方或左侧安全区。
- 身份行、三列经营统计直接排在背景上，使用细分隔线形成层级，不堆相同圆角盒子。
- 中部工作台采用左侧短标题和轻操作、右侧原创耳机/手柄/任务单插画；使用 `frontend/src/static/illustrations/empty-services.png`，透明3:2资源、等比展示，不拉伸。发布服务使用36px小按钮，查看订单是轻入口，禁止恢复两枚巨型按钮。
- 我的服务/接单大厅保持原业务切换。服务卡使用统一ServiceCard：源图16:9，展示裁切约4:3；底部渐变从透明到主题背景完全不透明；标签在上、标题在下，均独立悬浮在图片内；价格、成交和必要编辑/状态入口在图下正常流。
- 空态复用同套插画与少量文字；访客只小字提示登录，不做整页登录墙。已发布数据、接单状态、订单权限保持真实业务。
- 统一16px水平边距、8/12/16/24间距、44/36/28控件规格，图标双轴居中。

## 生成与实施边界

使用内置ImageGen编辑同路径旧图后等比缩放至390×844。图片用于可读首屏设计，所有界面需实现为真实组件；生成文字误差以本说明和真实业务为准。图中省略的微信胶囊仍必须按系统API真实避让。实际验收截图单独放output，不得替换设计图。

## 完整编辑指令

Use case: ui-mockup. Edit the attached 390x844 Chinese WeChat mini-program design. Output one flat 390:844 portrait UI, no device frame, no outside margins. Preserve brand 集结, dark charcoal-green #0a1110, white #edf4ef, muted graygreen #9dafA3, mint accent #8de8b6, fine restrained dividers, 16px horizontal insets and legible Chinese. Native statusbar and top-right WeChat capsule safe area are real constraints. Avoid generic AI dashboard appearance: no repeated big rounded boxes, no dashed placeholders, no decorative giant buttons, no blue/purple gradients. Official icons aligned centrally. Any game artwork is design-reference only. Primary tabs have only ONE persistent bottom nav 玩家 打手 订单 我的. Revise 打手 main view into a thoughtful game-service workbench. Completely REMOVE top app navigation bar, centered/left page title 打手工作台 and back arrow. Reserve y0-84 native safe area; keep first meaningful identity row left of capsule: small32 avatar 北辰, compact 接单中 status, quiet gear at safer y100. Under it a refined unboxed three-column earnings/stat strip 3进行中 28已完成 ¥1280收入 with subtle separators. Below a distinctive editorial workbench section at y180-300: left headline 把擅长，变成收入, quiet one-line guidance, small outlined 发布服务 plus-icon control and textlink 查看订单; right about130px a charming original green/ivory 3D gaming workstation illustration, console, headset and a few service/task tickets, matte softlight, no moneyglow. Never two huge full-width side-by-side buttons. Tabs 我的服务 接单大厅 with thin active underline. Two-column service grid: each SINGLE card contains16:9 art with transparent-to-opaque dark vertical gradient at bottom, independent small certification/game labels above a bold white title INSIDE the image; price below image in normal flow, compact status 接单中 and small 编辑 link can share a second quiet row. No title or label outside image. Show2 cards plus another partialrow. Quiet horizontal club-discovery entry near bottom if fits. Bottom 打手 active. All cards/controls consistent, polished hierarchy and no repetitive boxed sections.

二次编辑：替换为项目正式透明插画，保留布局。
Use case: compositing / ui-mockup. Image1 is the edit-target Chinese390x844 workbench UI; Image2 is the supporting insert: final transparent headset/controller/ticket stilllife illustration. Preserve EXACTLY the target UI layout, all readableChinese, all stats/tabs/servicecards/bottomnav and colors. Change ONLY the current glowing console illustration at upper-right beside 把擅长，变成收入: replace it with the matching matte pale-sage/ivory illustration from Image2, keep its identifiable headphone/controller/paper/pencil composition and gentle naturaltextures, scale toabout150pxwide, transparent background blends with target darkcanvas. Remove the glowingcomputer and cash/chips fromoldillustration, no extra decorativeobjects. No largebuttons, no topnavbar. In servicecards, labels/title stay independently overlaid inside the dark gradient bottomofimage, pricebodybelow. Outputflat390:844portrait, no deviceframe. Native rightcapsule area must be unobstructed, settings gear may move below it. Do not introduce purple/blue gradients or neon.
