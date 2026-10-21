# 玩家主页：已设置封面状态

文件作用：记录玩家已上传个人主页封面时的先行状态设计，与 detail.design.md 共同约束同一页面。

- 以现有 detail.png 为编辑目标，使用内置 ImageGen 生成状态变体，保存为390×844。
- 顺序为统一顶栏、16:9用户封面、16px正向留白、头像昵称身份行、三列统计、认证俱乐部、发布服务。
- 所有区域正常文档流排列，不用负margin、绝对定位身份叠层；图片上的服务文案与卡片下方信息依旧分离。
- 用户没有设置 profile_cover 时不渲染封面，采用原 detail.png；生成图山景不作为产品默认图。

## 最终编辑指令

Use case: precise-object-edit / ui-mockup. Edit target Image1 is the existing 集结 玩家主页 mobile UI design. Preserve its exact 390x844 portrait viewport, dark #0a1110 and mint #8de8b6 palette, statusbar, consistent compact top navigation 玩家主页 with left back chevron and right share, official-style centered icons, typography,16px margins, service cards and certification visual style. Make ONE specific state variation: user has uploaded a profile cover. Insert a normal-flow 16:9 landscape cover immediately below the shared top navigation, width358 height201 logical px,12px rounded corners,16px horizontal margins. Cover is a tasteful mountain/forest landscape user photograph with no text, no logo, no character. Then place existing avatar/nickname 北辰/certification/description identity row COMPLETELY BELOW that cover with16px positive spacing; avatar is not overlapping the cover edge. Following identity retain same 3 stats 已完成28 / 好评5.0 / 在售3, 山海电竞 已认证 row and 发布的服务 two-column service cards with image labels and normal-flow separate info. Because cover consumes space, naturally show only first service row and start of next at bottom; do not shrink everything. Absolute constraints: no negative margin, no identity/cover/service text overlap, no floating identity overlay, no bottom tab navigation, no slogan or additional feature. One flat screenshot UI no phone frame; all other design semantics unchanged.
