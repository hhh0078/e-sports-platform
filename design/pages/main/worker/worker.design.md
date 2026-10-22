# 打手工作台 · 默认接单身份

文件作用：对应 `frontend/src/pages/main/worker/worker.vue` 的先行局部设计与实施依据；2026-10-22根据用户最新默认身份要求编辑。原图和说明在 `design/history/20261022-before-game-refinements/pages/`。

- 仅移除账户级“休息中/去开启/在个人中心开启接单”等身份启停状态与提示；所有新老用户默认拥有接单身份且不能关闭，昵称旁为非交互的接单中状态，不留下拉箭头或启用开关。
- 访客显示登录后查看和小字“登录后即可接单”，不做整页登录墙，不暗示还要手动开启身份。
- 保留经营统计、纸感工作台插画、小发布入口、查看订单、我的服务/接单大厅、服务卡、俱乐部入口与唯一底栏；不新造钱包操作或其他按钮。
- 单个服务的审核、上架/暂停与账户接单身份是不同状态，既有业务保留；默认身份不能绕过手机号、冻结、简介与发布/交易校验。
- 四主视图无应用顶栏；第一行业务元素避开系统状态栏和原生胶囊。服务卡仍图内渐变标签/标题与独立文案、价格图下正常流。
- 玩家手柄仅选中时为当前主题accent，此页打手选中；所有图标双轴居中，保留既有自然黑绿设计和320/390受控宽度。
- 接单大厅的现有GamePicker横排改复用共享GameSelector；正常态版式不重做，弹框外观直接沿用[首页游戏弹框](../player/player-game-picker.design.md)。保持本页既有显式全局筛选语义，不让组件自行读取/写入全局偏好；单选即选即回，13项受控纵向滚动。

## 生成边界与完整提示词

390×844，内置ImageGen编辑原图后机械等比缩放。图中文字误差以真实数据、业务与本说明为准；不把设计图作为运行背景。实际SDK验收由root执行，本交付不代表真机通过。

Use case: ui-mockup. PRECISE LOCAL EDIT of attachedexistingChinese390x844design. Preserveallunrelatedapprovedlayout, naturaldarkgreen#0a1110,mint#8de8b6,thinsectiondividers,16pxinsets,officialcenteredicons,existingfieldsandnativeWeChatsafearea. DoNOTinventfeatures,newart,headlines,English,bluepurplegradients,orreplacegameartsource. Onlyedittheexplicitprovideridentitypoint. Exact390:844flatportrait,no deviceframe,nooutside margins. TextisindependentUI, notbakedintoimages. Thisisprimaryworkertab,noapptopnavbar. Keepidentity/stats/refinedworkbenchillustration/publishsmallentry/ordertlink/tabs/servicecards/clubbanner/bottomnavunchanged. Onlyclarifydefaultprovideridentity: nexttonickname北辰 showsSTATIC●接单中badgewithoutdropdownarroworclickcontrol; optionallytiny 默认开启 understateifneededbutavoidredundancy. Removeany休息中、去开启、在个人中心开启接单、去完善启用 notices/actions; thereisNOaccount-levelenabletoggleandNOrestingstate. New/existingusersallhaveprovidercapabilitybydefault; guestshows登录后查看andquiet登录后即可接单onlywhereappropriate,nobigloginwall. DoNOTaddnewbuttons. Existingindividualserviceliststatuses/editactionsareseparatefromaccountcapabilityandremainasactualbusiness, don'tinventpausedserviceexamplesforaccountstate. Retainnecessaryphoneverification/frozen/profileintrochecksintheactualflows. Allartunchanged,allotherlayoutunchanged. Bottomnav打手activeandmint,playerinactivegray.

## 品牌名称更新（2026-10-23）

品牌说明为在开机电竞，用游戏技能赚取真实回报；保留统计、插画、服务与接单入口布局。 本次只局部替换品牌文字，不新增界面、控件、图片或视觉规则。历史图保留原名称。

编辑提示词：Use case: text-localization. Edit only the product brand text in the attached existing 390x844 design. Use exact Chinese brand 开机电竞. 品牌说明为在开机电竞，用游戏技能赚取真实回报；保留统计、插画、服务与接单入口布局。 Keep all unrelated layout, artwork, colors, type sizes, fields, amounts, safe areas and dimensions unchanged. No redesign, stretching, added height or new decorations.
