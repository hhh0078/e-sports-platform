# 玩家首页 · 左文右操作

文件作用：对应 `frontend/src/pages/main/player/player.vue` 的先行局部设计和实施依据；2026-10-22按用户最新局部修改编辑。上一版图及说明已归档 `design/history/20261022-before-game-refinements/pages/`。本图先保存后通知实现。

- 本轮仅调整顶部hero：文字组在左、发布需求按钮在右，同一横向业务行；不把按钮放回文案下方。右按钮紧凑36px、约96～104px宽、不可收缩，左文案min-width:0、两者至少12px间隔。
- 这行须位于原生胶囊底部安全距离之后，不按旧status+8直接排到胶囊下方；320px时标题可在左区自然换行、hero适度增高，禁止按钮遮字或进入胶囊区域。
- 游戏背景来自当前game.cover_url，示例使用项目已有官方delta素材；不硬编码当前游戏或替换为无来源角色。四主视图无应用顶栏。
- 游戏目录仍固定一行横滑，右端全部打开选择弹框。服务分类、搜索排序、服务卡、发现俱乐部和状态/滚动保留。
- 游戏弹框抽为共享GameSelector，三列图标/搜索/选中样式保持已批准首页风格；对应打开状态见[player-game-picker.design.md](player-game-picker.design.md)和同名PNG。横排GamePicker仅保留触发器及显式全局偏好绑定，其他页面复用弹框不能误写首页游戏缓存。
- 服务卡仍是图底透明至主题深色渐变，标签在上、标题在下独立图层，价格成交图下正常流，自定义文字防碰撞。
- 唯一底栏的玩家手柄在选中时必须跟随当前主题accent着色，不是固定白色；未选中为次要灰色。Root负责Icon实现，本图只表达正确选中视觉。

## 共同边界

390×844，内置ImageGen精确局部编辑后机械等比缩放；保留既有自然黑绿设计、字段和安全区。图中文字误差/示例数值以真实业务与本说明为准。图片只作设计参照，实际页面使用现有官方素材和独立组件。验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. PRECISE LOCAL EDIT of the attached existing Chinese390x844 design. Preserve the approved naturaldarkgreen UI (#0a1110,white#edf4ef,mintaccent#8de8b6), existingbusinessfields, spacing, hierarchy, thinseparators, typography, officialiconstyle andnativeWeChatstatus/capsulesafearea. DO NOT redesign unrelatedparts, addfeatures/headlines/English, inventnewgamesorcharacters, or introduceblue/purpleneongradients. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Existingartreferencesareprojectofficialassets. TextremainsindependentUI, neverbakedintobackground. Ordinarysecondarypagesretainsharedbackheader;primarytabsNOtopnavbar. Thisisprimaryplayerhome. ChangeONLYtopheroarrangement: textgrouponLEFTand发布需求compact36pxbuttononRIGHTintheSAMEhorizontalrow, alignedvertically, withmin12pxgap. Hero's meaningfulrowmustbeginBELOWnativecapsulebottom(abouty88-96), so rightbuttondoesNOTsitunderWeChatcapsule. Lefttextgroupmaycontainexisting开机电竞brand, 开机即组队，一起赢headline andcurrentgame/categorysmallline, compactreadable, canwrapwithinboundedwidth; rightbuttonwidthabout100pxdoesnotshrinkoroverlaptextat320px. DoNOTput发布需求belowtextasoldimage. Keepgamebackground/graydarkshade, singlehorizontalgamecatalogwithfixed全部entry,categoryfilters,servicecardsandbottomnavunchanged. Bottomnav玩家activegamepadiconmustbeMINTTHEMECOLOR, notplainwhite; othericonsmuted. Preservegameartfromexistingtarget;supportingofficialassetclarifiessource,don'tinventalternategameart. No primaryappnavbar. Allotherpartsremainasexistingdesign.

## 2026-10-22自愿平台押金局部增补

仅当真实provider_deposit_cents>0时，在服务图下价格区域的独立小行显示“平台押金”标识（可带实际金额）；0/未知不显示。此信息不进入图内自定义六条文字、标签/标题或俱乐部认证区，不代表担保/赔付；正文随标识自然增高。

完整局部编辑提示词：

Use case: ui-mockup. PreciseLOCALeditofattachedChinese390x844design. Preserveallunrelatedstyle/fields/layout/gameart/safeareas,approved#0a1110darknaturalgreenand#8de8b6mint, thinseparatorsandcentredofficialicons. No newfeatures, no giantcards, no Englishdecoration/no bluepurplegradient. AllnewtextindependentUI,nobakedgameart. Exact390:844portraitflatUI,noframe/nooutside margins. ChangeONLYservicecards: addsmallsubtle平台押金badgeinitsOWNnormal-flowrow BELOWtheimage andbeside/underpricebody, onlyoneortwosamplecardswithpositivedepositshowit;cardswithzerodepositdonotshowanything. Badgeplainmutedmint/graytextandtinywalleticon,noguaranteeshield,no赔付/担保promises. NEVERputbadgeinsidethecovergradient/customtext/clubcertificationarea. Keepall sixcustomtextsafezonesandimagecontentsunchanged. Allowcardbodygrowabout18-22pxandlowercardsfallbelowviewport,noshrinkingtypography. PreserveheroLEFTcopy/RIGHT发布需求, officialbackground,singlegamecatalog/allentry,andmintACTIVEplayergamepadiconunchanged.

## 品牌名称更新（2026-10-23）

首页品牌为开机电竞，口号为开机即组队，一起赢；保留左文右发布操作、游戏目录、服务卡及安全区。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 首页品牌为开机电竞，口号为开机即组队，一起赢；保留左文右发布操作、游戏目录、服务卡及安全区。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.
