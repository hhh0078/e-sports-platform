# 集结 · 验收结果与待验收项

文件作用：记录有原始证据支持的验收结果，并区分本地模拟、服务器部署与微信正式渠道。2026-10-20 重新运行自动验证、源码 Docker 构建、服务器更新与外网检查；小程序社区、导航与四方交易沿用 2026-10-18 的界面实测记录，分别标明日期。

本次（2026-10-22）前端目录、导航与TypeScript重构，以 [本轮验收记录](FRONTEND_REFACTOR_ACCEPTANCE.md) 为准。下方历史截图、数量和产物路径仅对应各自日期。

## 当前交付证据（2026-10-20）

| 范围 | 结果 | 原始证据 |
| --- | --- | --- |
| 后端测试（10 月 20 日） | `./scripts/verify.sh` 全过；63 项顶层测试、27 个子场景通过，0 失败；无 DATA RACE；真实本地 PG/Redis 集成与 `go vet` 通过 | [verification-20261020.txt](../output/community/verification-20261020.txt)；前次结构化记录 [backend-final-tests.jsonl](../output/community/backend-final-tests.jsonl) |
| 前端与双端构建（10 月 20 日） | 前端 52/52 通过，0 失败、0 跳过；H5、微信小程序 production 构建均成功；覆盖协议登录、会话/配置竞态、原生选图草稿保留与封面避让 | [verification-20261020.txt](../output/community/verification-20261020.txt) |
| 源码 Docker 构建（10 月 20 日） | 本机直接使用 `backend/Dockerfile` 构建成功；镜像内 API、演示工具与 CA 存在，非 root 用户 UID 100 检查通过。服务器使用 Linux amd64 发布镜像，下面单独验收 | `.run/docker-source-build-20261020.log` |
| 小程序访客与社区（10 月 18 日） | 15 个步骤通过，异常列表为空；访客三个工作区、协议阻拦、可跳过资料弹层、独立资料页、公开玩家/俱乐部主页、五个本地体验身份及配置权限均有实际点击 | [mini-community-report.json](../output/community/mini-community-report.json) |
| 小程序四方交易（10 月 18 日） | 本地订单 `14d58ecf813c2bf8e06cdf70a61391be` 完成测试付款、接单、开始、交付、四方聊天、退款申请/拒绝/介入/裁决退款；无关账号被拒绝，运行异常为空 | [mini-e2e.json](../output/playwright/mini-e2e.json) |
| 小程序导航回归（10 月 18 日） | 打手服务分页、身份/状态深链、资料校验、接单开关恢复与管理入口权限通过；异常与 console_errors 均为空 | [mini-navigation-report.json](../output/navigation-design/mini-navigation-report.json) |
| 服务器更新（10 月 20 日） | 项目镜像更新为 `jijie-api:20261020`，容器内二进制 SHA256 与本地发布文件相同；API、PG、Redis 均 healthy；`nginx -t` 与更新前备份完整性检查通过 | [server-final-20261020.png](../output/community/server-final-20261020.png) |
| 正式域名外网检查（10 月 20 日） | 54 个请求通过，其中 43 个为游戏封面/图标；包含 13 款游戏配置、服务、俱乐部、玩家主页、协议；开发账号接口与 `.env` 为 404，未登录管理接口为 401 | [production-http-check-20261020.json](../output/community/production-http-check-20261020.json) |
| 微信后端凭据（10 月 20 日） | 使用私密部署配置调用微信官方 `stable_token` 返回 HTTP 200、`errcode=0`、有效期 7200 秒，证明 AppID/AppSecret 配对有效；未把 access_token 写入验收记录。此项不代表客户端登录、手机号或支付通过 | [wechat-config-check-20261020.json](../output/community/wechat-config-check-20261020.json) |
| 正式小程序产物（10 月 20 日） | `frontend/dist/build/mp-weixin` 已生成；AppID 正确、API 为正式 HTTPS 域名、`url_check=true`。最终文件数与体积以产物报告为准 | [release-mini-20261020.json](../output/community/release-mini-20261020.json) |
| 微信身份准备（10 月 20 日） | 改用官方 SDK `mini.callWxMethod` 后，应用原生 `getAccountInfoSync` 返回正确 AppID，`wx.login` 成功取得 code；正式后端 `/api/auth/wechat/prepare` 返回 HTTP 200 及协议资料。未代替用户同意协议，未创建账号或最终登录会话 | [official-login-20261020.json](../output/community/official-login-20261020.json) |
| 正式小程序页面联网（10 月 20 日） | 6 步实际复核通过，运行异常为空：原生请求正式 HTTPS 目录、玩家页服务器演示服务与 HTTPS 图片、个人中心访客布局、订单登录提示、公开演示俱乐部、正式登录页无开发入口且协议默认未勾选；响应 `dev_mode=false`、13 款游戏 | [mini-production-20261020.json](../output/community/mini-production-20261020.json) |

本地体验包独立保存在 `frontend/dist/local/mp-weixin`，继续连接本地测试环境；正式产物仍在 `frontend/dist/build/mp-weixin`，没有被本地包覆盖。分别导入对应目录，避免把本地模拟流程当成正式小程序验收。

**诊断纠正：** 早先通过 `mini.evaluate` 发起微信 API 调用，执行上下文不属于实际小程序，产生的 `appid missing` 与 `url not in domain list` 不能作为应用配置失败证据。改用 `mini.callWxMethod` 后，原生网络请求返回 200、官方登录 code 与正式服务端身份准备均已成功。旧的失败判断作废；社区报告里的 10 月 18 日登录态过期结果仅为当时历史记录。

**真实最终登录仍待用户完成。** 已验证官方 `wx.login` 与正式后端 `code2session`/身份票据准备；尚未实测真人主动同意两协议后建立账号和最终会话，不能称完整登录链路通过。正式页面联网复核结果以 [mini-production-20261020.json](../output/community/mini-production-20261020.json) 为准。

**真实手机号、商户扣款退款、真机网络与微信提审仍待验收。** 本地身份、模拟号码与测试付款可验证产品流程，但未证明运营账号权限或真实到账。首次正式管理员绑定也需可信运维渠道配置，生产不存在默认体验管理员。具体步骤见 [发布指南](DEPLOYMENT.md) 与 [微信配置清单](WECHAT_BACKEND_SETUP.md)。

本轮新增的登录、个人中心与俱乐部设计图分别为 [login-v2.png](design/login-v2.png)、[me-v2.png](design/me-v2.png)、[club-v2.png](design/club-v2.png)。设计与实屏的最终比较以 [design-qa.md](../design-qa.md) 中明确标记的对应轮次为准。

演示数据为虚构展示内容：60 用户、6 俱乐部、42 条成员关系、130 服务、52 订单、260 消息、8 评价及 60 头像；命令和只读交易保护见 [DEMO_DATA.md](DEMO_DATA.md)。发布包与源码包必须分开：本地 `.deploy/jijie-release-20261018.tar.gz` 包含私密 `.env`，仅用于部署，不可交给源码买家；源码仅包含 `.env.example`。

以下为前几轮的范围说明和历史记录。其测试数量、构建体积与订单编号仅对应当时版本；同名报告会由复测脚本更新，当前结果以本节链接的记录为准。

## 历史：2026-10-18 其余三个导航按图改版

先生成并保存 [打手](design/navigation-worker.png)、[订单](design/navigation-orders.png)、[我的](design/navigation-me.png) 三张图，再据图实现。源图/实屏的逐轮对照及修复见 [design-qa.md](../design-qa.md)，三页总览见 [实际页面](../output/navigation-design/navigation-actual.jpg)。

- 打手页改为游戏主题工作台、真实统计、服务分组；保留暂停/待审和独立封面文字，20条分页至38项全部显示。
- 订单页读取角色/阶段深链，SQL先筛选后分页，卖方待支付不出现付款动作；停用游戏使用订单游戏名快照。
- 我的页按图重排，资料限制和后端一致；计数与路由一致，俱乐部入口正确mine=1，管理员权限保留。
- 修复跨会话迟到资料/401覆盖、旧读取覆盖接单开关、WXSS不兼容选择器和320px溢出。含query的主导航也重置页面栈。

该轮实测记录：Go真实PG/Redis集成与race（38项顶层+21子场景）、vet、前端33项测试、H5/微信构建通过。导航报告已被后续复测更新，当前统计见本文首节。该轮 H5 记录 [h5-checks.json](../output/navigation-design/h5-checks.json)。

该轮微信订单 `103e0d5caebc5ab88ea9d5505cd3c16f` 完整走通支付→接单→开始→交付→四方聊天→退款申请→打手拒绝→平台介入→全额退款，陌生账号无权访问。交易报告已被后续复测更新，最新样本见本文首节。这里的支付退款均为本地测试，不扣真实款。

本轮以前的验收与历史业务边界保留如下。

文件作用：区分当前代码测试、真实界面验收、历史证据与外部渠道待验收项。更新日期：2026-10-18。

## 本轮结论

本轮已按用户参考图先生成设计目标，再修改首页并进行等比例实屏比较。13款游戏使用各自官方来源图标/封面；新增服务随机匹配底图，文字保留为独立可编辑图层。H5和微信开发者工具均有真实操作证据，详见根目录 [design-qa.md](../design-qa.md)。

当前 API、H5、PostgreSQL 和 Redis 已运行。入口：[集结本地体验](http://127.0.0.1:5173/)。这是本地可用版本，真实微信身份、商户扣款/退款、分账提现、微信提审与真机网络未完成正式渠道验收。生产模式缺少配置会明确失败，不会退回测试付款成功。

## 历史：首页改版验证结果

| 项目 | 实际结果 | 证据 |
| --- | --- | --- |
| Go真实PG/Redis测试 | 35项顶层测试、21个子场景全部通过；race无报告 | [后端记录](../backend/TEST_RESULTS.md)、[原始JSON](../output/redesign/backend-tests.json) |
| Go静态与构建 | `go vet ./...`和二进制构建成功 | 后端记录与可复现命令 |
| 前端测试 | 12/12通过；六条最长文案、混合字号/九宫格/emoji/宽英文、默认排列、随机换图、空值边界、金额/资源地址 | `cd frontend && npm test` |
| H5与微信最终构建 | 两端production构建成功；微信目录145个文件、1,978,226字节（1.8866 MiB，小于2 MiB） | `frontend/dist/build/`，未把素材来源清单打进小程序 |
| 官方素材核对 | 13游戏、30封面、13独立图标；43个JPEG哈希/尺寸/大小与清单一致，共1,377,437字节 | [来源](ASSETS.md)、[清单](assets-manifest.json) |
| H5实屏 | 390px和320px下13游戏、主题、图像加载、目录搜索、服务筛选/空结果、三排序通过；最后console error/warn为空 | `output/redesign/games-*.json`、`filter-sort-ui.json`及截图 |
| H5独立文字 | 六文案实际发布、审核、重新进入编辑，底图和全部文案样式/位置一致，换图不丢文字，Canvas数量0 | `overlay-dom.json`、`overlay-persisted.json`、`overlay-reopened.png` |
| H5管理员 | 独立图标实际文件上传、保存、重开；颜色和四张封面不变。验收后精确恢复原内置图标 | `admin-icon-ui.json`、`admin-icon-upload.png` |
| 微信视觉与编辑 | 最终构建实际操作13游戏、首排、目录搜索、三排序，六文案九宫格9位置无重叠/越界，换图保持全部样式；运行异常0 | [机器结果](../output/redesign/mini-redesign-smoke.json)、[脚本](../scripts/mini-redesign-smoke.cjs) |
| 微信四方交易 | 新建订单、测试支付、接单、开始、交付、四方聊天、退款申请/拒绝/介入/裁决、陌生人拒绝，全部通过；运行异常0 | [机器结果](../output/playwright/mini-e2e.json)、[现行交易脚本](../scripts/mini-transaction.ts) |

数据集成测试在 `jijie_dev` 内建立随机独立schema和Redis键前缀，不重置本地展示数据。并发测试证明业务正确性，不代表生产容量压测。当前前端测试有Node模块类型提示，构建与测试成功；未为消除此无害提示调整整项目模块制式。

## 视觉与封面重点

| 三角洲真实首页 | 王者真实首页 | 六条独立文字回填 |
| --- | --- | --- |
| ![三角洲](../output/redesign/home-final.png) | ![王者](../output/redesign/honor-final.png) | ![封面编辑](../output/redesign/overlay-reopened.png) |

默认游戏顺序为热门推荐：王者、和平、三角洲、金铲铲、无畏契约、英雄联盟、源能行动、CF手游、暗区、使命召唤手游、第五人格、永劫手游、蛋仔。各来源的DAU/峰值/收入指标不能直接当作统一玩家数排名，具体依据见 [GAME_POPULARITY.md](GAME_POPULARITY.md)，管理员可以改排序。

三角洲与王者各有4张底图，其余各2张。`cover_url` 与 `labels` 分开保存；`image` 与独立文字层分开渲染，无Canvas烘焙。标签默认左下依次向右，空间不足换行；改位置后自动避让。编辑已有服务保留原图；切游戏随机取该游戏图，换底图保留全部文字配置。

新六文案样本：`9d6fbce4bae12a9e94a3e3d1e1e9aa57`，标题“分层验收 · 六文案护航”。已暂停，防止测试商品占据演示首页；北辰可在我的服务继续编辑。

管理员配置和历史数据保护已有测试：只迁移明确的旧默认图，不覆盖自定义主题、封面、服务内容/金额/标签或历史订单快照；有管理员保存记录的游戏主题不自动覆盖。本机本轮改色和新增封面通过管理员接口显式保存。

## 历史：首页改版微信实测

环境：微信开发者工具Stable 2.02.2608040、基础库3.17.4、游客模式、本地API。导入目录 `frontend/dist/build/mp-weixin`；HBuilderX已导入 `frontend`。

该轮四方退款订单：`b21ee0a617daedcd3bc68664c4696f98`，2026-10-18 08:37（北京时间）完成。

1. 云间旅人填写约定、下单、测试付款，发送老板消息。
2. 北辰接单、开始、发送打手消息、提交交付。
3. 山海主理人进入关联订单、发送俱乐部消息。
4. 买方申请退款，卖方拒绝，买方申请平台介入。
5. 集结管理员核对约定和四方证据、发送平台消息、支持买方退款。
6. 页面显示已退款、系统事件保留；自由玩家访问被拒绝，不渲染聊天室。

本轮最终视觉与图文脚本于08:52在重建后通过；其中价格/成交/综合三排序逐张比较页面标题、分价和成交数与后端响应。自有代码的系统尺寸读取已改用 `getWindowInfo`。不将业务异常0等同于开发工具自身全部警告消失。

| 微信首页 | 文字编辑 | 退款完成 |
| --- | --- | --- |
| ![微信首页](../output/redesign/mini-home-redesign.png) | ![微信图层](../output/redesign/mini-overlay-editor.png) | ![已退款](../output/playwright/mini-refunded.png) |

早先图文脚本因SDK不能跨uni-app自定义组件查询而超时，另有一次按钮文案匹配错误；均修正为按真实组件路径和可见文案操作后完整重跑。`mini-redesign-failure.png`仅是历史脚本失败截图，不计作成功证据。构建中瞬时app.json缺失、游客AppID限制与页面业务错误分开记录，最终构建已可实际运行。

## 上一轮保留的完整业务证据

以下是改版前的同一后端流程实测，不冒充本轮逐步重跑；本轮后端完整回归与微信新订单已覆盖其核心调用链。

- H5正常完成订单 `956e2a8e012cc855316e4ef5b16aba18`：数量2、38元，支付→接单→开始→交付→验收→五星评价；真实UI上传800×500 JPEG，图片可续签预览，四方聊天可回读。
- 旧六文案服务 `8f2027255b77a0097db037e613a7dee1`：价格25.50元、保存回填、审核与修改重审通过。
- 公开需求 `78696d2bbd64c679b71edc232a95a720`：玩家创建付款，北辰选择山海电竞领取，负责人进入该订单沟通。保留已接单状态，可继续测试。
- 服务停止/重启、重复启动复用PID、跨执行会话存活、PG/Redis健康检查均实际执行过；本轮源代码更新后重新启动，数据库保留。

## 重要业务回归

- 16次并发付款只成功记账一次；20次抢单只有一个成功。
- 预支付与取消互斥，未知结果保留付款意图；单数据库连接的付款与俱乐部创建/编辑均不因嵌套借连接而阻塞。
- 退款先保存稳定退款号和冻结状态，三入口上游超时均可恢复；金额、商户、付款人、流水、重复回调与验签均有协议测试。
- 四方聊天、陌生人隔离、私密截图与续签权限；120条历史与150条增量无分页丢失。
- 多俱乐部成员审核、退出/移除游戏撤销新认证；历史订单按原组织快照授权。公开需求使用字段白名单。
- 130条服务及80条隐藏/冻结记录覆盖完整数据集搜索和分页排序；三种排序跨页无遗漏重复，拒绝未知排序。
- 前端不再通过自由标签伪造“俱乐部认证”；退出或移除支持游戏后，当前展示和新订单权限一致撤销。

完整测试范围与独立复核见 [TEST_RESULTS.md](../backend/TEST_RESULTS.md)、[REVIEW.md](REVIEW.md)。

## 交付边界

- 当前全额退款、四秒增量轮询聊天；没有语音视频通话、微信模板消息、部分退款或可提现钱包。收入为订单统计。
- 真实微信登录、实际商户扣款/退款、分账提现和真机网络需正式资料后联调，本地流程不替代这些验收。
- 新素材来自官方公开页面/CDN，来源可追溯；公开可下载不等于取得商业使用授权，正式运营授权边界见 [ASSETS.md](ASSETS.md)。
- 当前仓库没有Git远程，只能完成本地中文提交，不能宣称已推送云端。

人工体验请按 [USER_TEST_CHECKLIST.md](USER_TEST_CHECKLIST.md)，启动方式见 [README](../README.md)。
