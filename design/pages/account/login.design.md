# 登录 · 恢复官方三联游戏主视觉

文件作用：对应 `frontend/src/pages/account/login.vue` 的先行局部设计和实施依据；2026-10-22按用户最新局部修改编辑。上一版图及说明已归档 `design/history/20261022-before-game-refinements/pages/`。本图先保存后通知实现。

- 只替换主视觉为以前的三联官方游戏拼图，保留已认可的黑绿开放式登录布局。素材顺序：`backend/internal/assets/covers/official/delta-3.jpg`、`honor-1.jpg`、`peace-1.jpg`；对应运行静态路径由既有assetUrl处理，不引入外部或生成游戏角色。
- 用户本地login.vue已删除顶部brand与先逛逛，本轮必须保留该修改：不加回“开机电竞”小品牌行、brand图标或先逛逛链接。普通统一顶栏“登录开机电竞”和左上返回保留。
- 保留两行有效文案“热爱游戏 / 自由接单”和副文案“无需挂靠，凭实力找到队友”；拼图约180～200px高并自然淡入页面背景，图片只承载画面。
- 保留三项卖点、44px微信一键登录、主动勾选两协议、开发模式轻入口及原有错误重试；登录成功的可跳过资料弹层和微信code换身份流程不改。
- 无主导航底栏；状态栏/胶囊真实避让、320/390宽度协议正常换行。图中的字体/间距示意不授权恢复用户删掉的元素。

## 共同边界

390×844，内置ImageGen精确局部编辑后机械等比缩放；保留既有自然黑绿设计、字段和安全区。图中文字误差/示例数值以真实业务与本说明为准。图片只作设计参照，实际页面使用现有官方素材和独立组件。验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. PRECISE LOCAL EDIT of the attached existing Chinese390x844 design. Preserve the approved naturaldarkgreen UI (#0a1110,white#edf4ef,mintaccent#8de8b6), existingbusinessfields, spacing, hierarchy, thinseparators, typography, officialiconstyle andnativeWeChatstatus/capsulesafearea. DO NOT redesign unrelatedparts, addfeatures/headlines/English, inventnewgamesorcharacters, or introduceblue/purpleneongradients. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Existingartreferencesareprojectofficialassets. TextremainsindependentUI, neverbakedintobackground. Ordinarysecondarypagesretainsharedbackheader;primarytabsNOtopnavbar. ChangeONLYtheseitems: restorethePREVIOUSthree-panelofficialgamecollageasmainvisual,inplaceofheadset/controllerillustration. Useexactsupportingimages:Image2DeltaForceofficialshipwreck atleft,Image3HonorofKingsofficialmaleherointhemiddle,Image4PeaceEliteofficialhelmetfighteratright. Threeverticalslightlyslantedpanelswithgentlefadeattop/bottomintodarkbackground, coherentabout190pxhigh, no reinventedcharacters. REMOVE oldsmall开机电竞brandrow(iconandword) andREMOVE先逛逛link, asusermanuallydeletedboth. DoNOTbringthemback. Keepordinarytopbar登录开机电竞withleftback. Keepexisting2-lineheadline 热爱游戏 / 自由接单 andsubtitle 无需挂靠，凭实力找到队友, threequietbenefits 自由接单 订单留证 俱乐部认证,ONE44px微信一键登录buttonanduncheckedexplicitconsentlinks. KeepallotherUIunchangedandbalancedaftersmallbrandremoval. Noheadsetartanywhere, no new decorativebrand.

## 品牌名称更新（2026-10-23）

统一顶部标题为登录开机电竞，保留三联官方游戏图、登录按钮、主动协议勾选与原有留白。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 统一顶部标题为登录开机电竞，保留三联官方游戏图、登录按钮、主动协议勾选与原有留白。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.

## 2026-10-23 重复登录修正

仅服务端first_login明确为true的新注册账号显示可跳过资料弹层。老用户保留已有头像昵称直接回到目标页，不能因默认头像或历史资料被当作新用户。保留用户已修改的模板、文案和排版。


## 2026-10-30 双模式支付

本轮先行布局及状态规则见 [login.commerce.design.md](login.commerce.design.md) 与同名前缀 `.commerce.png`。金币、人民币及费用分别展示；公众号手机号短信和无服务配置提示复用登录设计变体。
