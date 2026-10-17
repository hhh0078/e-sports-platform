# 集结 · 原创封面资产清单

文件作用：记录 7 款游戏分类、14 张原创封面的生成工具、完整提示词、文件位置和压缩验收。制作日期：2026-10-18。

## 来源与使用边界

- 全部素材使用内置 `image_gen.imagegen`，每张单独调用生成，无拼图裁切，无外部图库搬运。
- 生成内容为原创人物和场景的游戏氛围插画，不是官方游戏原画，不代表游戏厂商授权、合作或背书。游戏名称仅用于平台分类。
- 用户参考图用于理解深色页面、右侧人物与可叠文字区域，没有把图中广告价格、文字、Logo 或人物直接复制成资产。
- 原始 PNG 保留在 Codex 生成目录；项目中交付 JPEG。只使用 macOS `sips` 做等比缩放和 JPEG 编码，没有二次内容编辑。
- 初稿为 `1024 × 640`、JPEG 品质参数 `55`；为适配微信包体，最终交付统一缩为 `800 × 500`。最终 14 张合计 569,741 字节，单张 30,512–51,773 字节。
- 已逐张检查生成图：无内置文字、无品牌标识，主角集中右侧，左下保留暗部；另外抽查了压缩后的三角洲、和平和金铲铲成品。

## 清单

| 游戏分类 | 项目路径 | 色调 |
| --- | --- | --- |
| delta-1 | `frontend/src/static/covers/delta-1.jpg` | 黑绿雨夜战术 |
| delta-2 | `frontend/src/static/covers/delta-2.jpg` | 黑绿雨夜战术 |
| honor-1 | `frontend/src/static/covers/honor-1.jpg` | 暖金东方幻想 |
| honor-2 | `frontend/src/static/covers/honor-2.jpg` | 暖金东方幻想 |
| peace-1 | `frontend/src/static/covers/peace-1.jpg` | 青蓝海岛竞技 |
| peace-2 | `frontend/src/static/covers/peace-2.jpg` | 青蓝海岛竞技 |
| arena-1 | `frontend/src/static/covers/arena-1.jpg` | 棕橙工业搜集 |
| arena-2 | `frontend/src/static/covers/arena-2.jpg` | 棕橙工业搜集 |
| valorant-1 | `frontend/src/static/covers/valorant-1.jpg` | 珊瑚红科技竞技 |
| valorant-2 | `frontend/src/static/covers/valorant-2.jpg` | 珊瑚红科技竞技 |
| cfm-1 | `frontend/src/static/covers/cfm-1.jpg` | 炭黑赤橙都市枪战 |
| cfm-2 | `frontend/src/static/covers/cfm-2.jpg` | 炭黑赤橙都市枪战 |
| tft-1 | `frontend/src/static/covers/tft-1.jpg` | 深青暖金奇幻棋盘 |
| tft-2 | `frontend/src/static/covers/tft-2.jpg` | 深青暖金奇幻棋盘 |

## 完整提示词与原始出处

### delta-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-62105a73-8a8d-4ad6-8884-151a81f17de7.png`。

```text
Use case: stylized-concept. Asset type: premium mobile gaming service marketplace cover, one landscape image 16:10. Primary request: original cinematic tactical extraction shooter scene for a black-and-emerald military game category. Subject: a single elite modern tactical operator in charcoal armor and night-vision helmet holding a realistic carbine safely at low ready, stationed on the RIGHT half, 3/4 portrait, highly detailed. Scene: stormy industrial river delta extraction zone, distant helicopter silhouette and atmospheric rain, realistic cinematic 3D game key art. Lighting: sharp emerald green rim light against graphite and smoky dark forest green, restrained sparks, premium contrast. Composition: subject visual weight on right, LEFT THIRD and entire bottom left are simple dark smoky negative space for app text overlays, scene continues naturally. No letters, no words, no text, no logos, no watermarks, no borders, no UI, no brand marks. Original invented character and environment, do not copy any official game artwork. Deliver a single finished wide image, not a collage.
```

### delta-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-dcdf4b13-e83f-4ffe-8961-2c140de49302.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original cinematic tactical extraction game concept art. Landscape 16:10. A small modern special operations squad at the RIGHT of an abandoned concrete floodgate, lead masked operator shown waist-up moving through rain with rifle safely down, dark armored gear, distant industrial scaffolding and river. Emerald green practical rim lights, near-black charcoal atmosphere, premium realistic 3D game key art. Keep left third and bottom left simple dark atmospheric negative space for text overlays. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### honor-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-f94dee93-8d90-4902-be65-8f098d08253e.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original Chinese-inspired heroic fantasy MOBA game key art, landscape 16:10. Powerful elegant male warrior in intricately detailed warm gold and ivory armor with flowing crimson cape, holding a radiant spear angled up, heroic face on the RIGHT. Far background ancient mountain citadel and drifting golden petals. Amber sunset, bronze, ochre, cream and warm gold, premium painterly realistic 3D. Left third and bottom left dark brown atmospheric negative space for text overlays. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### honor-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-4ca2d983-822d-447b-b2cf-b4a234f69dc2.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original heroic fantasy MOBA game concept art, landscape 16:10. Graceful female swordmaster with black hair and ornate ivory, burnt red and brushed gold armor stands on RIGHT foreground, blade upright, strong composed expression, no resemblance to an existing character. Temple courtyards and clouds behind, delicate gold firelight, premium polished cinematic 3D fantasy art. Warm amber brown palette, no blue purple gradient. Left third and bottom left clean dark atmospheric negative space for app text. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### peace-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-ff58e070-b1ab-4f08-8cf1-6944dff13fb9.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original military battle royale game cinematic concept art. Landscape 16:10. A heroic young adult combatant in modern steel helmet with face guard, white rolled-sleeve tactical shirt and dark body armor, rifle at low ready, standing on RIGHT foreground. Sunlit coastal island and turquoise sea with small cargo aircraft far behind, bright cyan sky and crisp white clouds. Polished realistic 3D game key art, cool teal, blue, white and graphite palette. Keep bottom-left foreground dark teal shadow with left third atmospheric negative space for overlay text. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### peace-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-ca29c7f8-953c-4d0f-91a7-8e1cc7cedaed.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original military battle royale game cinematic concept art. Landscape 16:10. A young adult woman wearing practical olive combat jacket and modern helmet, goggles raised, holding carbine safely down, dynamic prepared pose on RIGHT foreground beside an offroad vehicle. Sunlit abandoned coastal village and deep turquoise distant hills, clear blue-cyan sky, warm sunlight. Premium realistic 3D tactical cover, distinct invented character, adventurous upbeat tone. Left third and bottom left dark teal shaded soft negative space for overlay text. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### arena-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-bd202bfa-5951-4846-8c82-fa7d1d5994db.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original hardcore extraction shooter game concept art. Landscape 16:10. A heavily armored anonymous tactical scavenger with full helmet and amber visor carrying a large weathered backpack and rifle at low ready, waist-up on RIGHT. Claustrophobic derelict warehouse with scattered utility crates and orange dusty shafts of light, gritty realistic textures and premium cinematic rendering. Near-black, warm brown, muted khaki and amber palette. Left third and bottom left low-detail dark brown shadow negative space for text overlays. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### arena-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-26587500-68e8-4236-beb3-a914ae725caf.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original hardcore extraction shooter game concept art. Landscape 16:10. Weathered tactical scout in khaki body armor and half-face respirator, helmet mounted flashlight, inspecting a rugged supply case on RIGHT foreground, compact rifle slung safely. Derelict underground metro station and dust in an amber shaft of light. Premium gritty photorealistic cinematic game art, near-black warm brown copper orange palette. Left third and bottom-left uncluttered dark brown negative space for overlay text. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### valorant-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-0277a87a-1c52-4938-baa7-a5f2f8eda68a.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original futuristic competitive hero shooter concept art. Landscape 16:10. Confident young adult female agent with short dark hair wearing sleek ivory and charcoal tech jacket, angular coral red illuminated gauntlet, holding futuristic rifle low and safely. Character on RIGHT foreground, minimal geometric urban rooftop architecture behind. Premium stylized 3D painterly video game art with crisp angular forms, coral red, warm off-white and deep graphite, small cyan highlights only. Left third and bottom-left dark graphite negative space for overlay text. No existing game character. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### valorant-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-822eb06c-0f47-4fe3-93f0-f9a150fa725b.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original futuristic competitive hero shooter concept art. Landscape 16:10. Cool young adult male tactical agent with dark textured hair, sleek graphite armor, warm coral red energy device in one hand and modern stylized rifle slung, posed on RIGHT foreground in geometric sci-fi corridor. Premium crisp stylized painterly 3D video game key art, strong shapes, coral red accent lighting, near-black shadows and warm off-white details. Left third and bottom-left simple dark graphite atmospheric negative space for text. No existing game character. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### cfm-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-8a13757e-7e05-4dfb-b2f8-6b5ad736abe4.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original high-energy urban competitive shooter game concept art. Landscape 16:10. Elite adult male combat operative wearing graphite urban armor, black tactical mask and protective helmet, strong waist-up stance on RIGHT foreground, holding rifle low ready. Industrial city rooftop at dusk with distant orange emergency lights and flying embers, no violence or injuries. Premium polished realistic cinematic 3D game key art. Charcoal black, fiery orange, steel grey palette, strong orange edge light. Left third and bottom-left simple dark charcoal negative space for text overlays. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### cfm-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-e14952f6-69f3-4de9-92f5-ead068cfa85b.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original modern competitive shooter game concept art. Landscape 16:10. Elite adult female marksman in practical black tactical uniform and protective goggles with short tied hair, crouched beside a concrete barricade on RIGHT foreground, realistic sniper rifle held ready. Abandoned industrial street with steel structures and distant glowing orange sunset, dramatic smoke, no gore or casualties. Premium high-detail cinematic 3D key art, charcoal grey and intense burnt orange palette. Left third and bottom left clear dark shadow negative space for text overlays. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### tft-1

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-a3ed4504-7dce-4034-81f5-e74526310947.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original whimsical fantasy auto-battler strategy game key art. Landscape 16:10. Adorable small round jade dragon mascot with cream belly, tiny warm golden armor and curious expressive eyes standing on a floating stone chess arena on RIGHT foreground, a few original miniature fantasy knight pieces beside it. Enchanted forest ruins and warm golden motes, premium polished stylized 3D game render, tactile materials. Deep teal green shadows and warm gold palette, no purple. Left third and bottom-left dark teal soft atmospheric negative space for text overlays. Completely invented mascot, not existing game character. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

### tft-2

工具：`image_gen.imagegen`，`transparent_background=false`。原始文件：`/Users/Zhuanz/.codex/generated_images/01a14bf0-f8c9-7973-b8ea-cd6d7a349a4d/exec-c04c7e9a-4abe-44ff-95fc-4e64bfde2c8a.png`。

```text
Use case: stylized-concept. Asset type: mobile game service cover. Original whimsical fantasy auto-battler strategy game concept art. Landscape 16:10. Charming small fox-like magical tactician with ivory fur, teal cape and a tiny gold crown holding a glowing golden chess crystal, character on RIGHT foreground beside intricately carved floating chessboard with small invented armored figurines. Lush ancient forest ruins in the background, magical warm gold particles, premium high-end stylized 3D cinematic game art. Deep evergreen teal and honey gold palette, no purple. Left third and bottom-left simple dark teal negative space for text overlays. Completely original mascot, not any existing game character. Single finished wide image, not collage. Absolutely no text, words, letters, logos, watermark, borders, UI or brand marks. Invent original characters and environment, do not copy official game artwork.
```

## 验收

- 文件数：14，七款游戏均有两张。
- 最终尺寸检查：14/14 均为 800 × 500。
- 最终体积检查：14/14 均小于 52,000 字节；缩图后三角洲封面再次目视检查通过。
- 图片内容：所有素材均为独立生成画面；未把 UI、文字或价格烘焙进图片。
- 前端文字标签应保持独立配置，默认左下，以布局容器排列避免叠字。素材本身不实现标签布局。
