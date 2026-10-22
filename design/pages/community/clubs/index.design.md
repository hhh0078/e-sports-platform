# 俱乐部目录 · 整图信息卡

文件作用：对应 `frontend/src/pages/community/clubs/index.vue` 的先行局部设计和实施依据；2026-10-22按用户最新局部修改编辑。上一版图及说明已归档 `design/history/20261022-before-game-refinements/pages/`。本图先保存后通知实现。

- 保留统一俱乐部返回、搜索、发现/我的切换与创建入口；本轮仅把列表改为整张封面卡，所有名称、认证、人数/主理、简介、游戏、准入与状态均位于图内底部的独立UI。
- 图片作为卡片全尺寸背景（absolute inset:0、aspectFill）；图上叠从上透明到下方主题深色完全不透明的渐变。信息整体容器贴底、内部正常纵向流，禁止各字段各自absolute造成重叠；信息不得烘焙进图片或放到卡外。
- 显示尺寸约4:3，390屏内容宽约358px、卡高约269px；320屏内容宽288px时卡高至少232～240px，必要时随信息增高。不能沿用16:9约162px的高度堆六行文字。
- 12px内边距、4～6px字段间隔；名称单行省略，主理元信息单行省略，简介最多两行，游戏最多2项加真实+N，其余内容详情页可完整查看。准入/状态最多两短行；小屏不通过缩成几像素或越界解决。
- 状态、认证、人数、负责人使用真实数据。押金展示读取deposit_cents，考核读取assessment，分别显示真实门槛；没有门槛才能显示无额外门槛，不虚构平台认证或开放加入。待审核/拒绝等原审核状态保留。
- 客户端列表不代收俱乐部押金，也不把平台自愿押金与俱乐部门槛混同；完整规则与确认在既有俱乐部主页流程中处理。空态、创建弹窗、权限与旧响应隔离不改。
- 创建弹框的主要游戏统一使用共享GameSelector多选；打开状态见[index-game-picker.design.md](index-game-picker.design.md)与同名PNG。共享弹层必须在原创建modal上方，关闭/取消只关闭游戏选择，确认仅回填game_ids，仍由原提交申请按钮提交；无新建创建路由。

## 共同边界

390×844，内置ImageGen精确局部编辑后机械等比缩放；保留既有自然黑绿设计、字段和安全区。图中文字误差/示例数值以真实业务与本说明为准。图片只作设计参照，实际页面使用现有官方素材和独立组件。验收截图另存output。

## 完整编辑提示词

Use case: ui-mockup. PRECISE LOCAL EDIT of the attached existing Chinese390x844 design. Preserve the approved naturaldarkgreen UI (#0a1110,white#edf4ef,mintaccent#8de8b6), existingbusinessfields, spacing, hierarchy, thinseparators, typography, officialiconstyle andnativeWeChatstatus/capsulesafearea. DO NOT redesign unrelatedparts, addfeatures/headlines/English, inventnewgamesorcharacters, or introduceblue/purpleneongradients. Exact390:844flatportraitUI,no deviceframe,nooutside margins. Existingartreferencesareprojectofficialassets. TextremainsindependentUI, neverbakedintobackground. Ordinarysecondarypagesretainsharedbackheader;primarytabsNOtopnavbar. ChangeONLYclubLISTCARDS, preserveordinary俱乐部header/search/发现俱乐部/我的俱乐部tabsand创建textentry. EachclubentrybecomesONEimmersivefullcoverimagecardabout4:3,358x269pxat390viewport; at320viewport,minheightabout232px isallowedforlegibletext. AllinformationcurrentlybelowimagemovesINSIDEimageatBOTTOMasindependentUI: compactclubmonogram+bold山海电竞 andverifiedbadge, metadata24位成员·山海主理人主理, concise2-lineintro, smallgamechips(max2gamesplus+Noverflowindicator), actualstatus认证/开放加入/审核状态, andadmissiongatebrief例如需考核·规则押金¥200. No information/bodyrowoutsidetheimage. ImagebottomhasstrongverticalgradientfromtransparentupperportiontoFULLYOPAQUE#0a1110bottom; textmuststayclearagainstbusyart. Placealloverlaychildreninnormalverticalflowwithinbottomcontentblockwith12pxpaddingand4pxgaps, notindividuallystackedabsoluteovereachother. Longclubnameone-lineellipsis,intromax2lines,ownerone-lineellipsis,gamechipsbounded,thresholdmax2shortlines, NO textoutsidecard. Displaytwofullcardsandpartialthirdifspaceallows; secondclub云上竞技 usesitsactualcover/metadatawith无额外门槛asexample,don'tfalselyclaimverification. No extragamefilterorCTA, noherooutsidecards. Preserveallotherfields/style. Anygameartmustderivefromexistingtarget/projectofficialsupport, noinventednewsource.
