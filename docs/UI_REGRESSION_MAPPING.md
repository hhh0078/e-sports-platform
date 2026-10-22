# 微信模拟器回归脚本与当前界面映射

文件作用：记录本轮界面升级后的自动化入口、权限、选择器和实际运行证据，避免旧版结构断言误报；脚本能力、分段通过、整套中断及微信真实能力分别记录。

## 运行边界

- 根代理独占微信自动化连接，串行运行各脚本；禁止多个脚本同时控制同一个模拟器。
- 资金及布局回归只接受本机API，且要求bootstrap返回`dev_mode=true`。钱包脚本会创建模拟充值、押金和订单；不得把它指向生产。
- `UI_TEST_PROJECT` 必须对应当前模拟器真正打开的编译产物，`WX_AUTOMATOR_ENDPOINT` 对应该窗口端口；320/390 分别设置 `UI_EXPECT_WIDTH` 并使用不同 `UI_TEST_OUTPUT`。
- 交付截图和报告写入`output/`，不能替换`design/pages/`。失败/中断的过程报告可归档到不提交的`.run/acceptance-attempts/`；旧版报告不证明本次版本通过。
- 原生头像授权、真实手机号、真实支付/退款与商家转账到账仍须另行真机和商户验收。

## mini-ui-regression.ts

控制台共有15个入口：13个 `.module-card` 加2个 `.finance-module-card`。脚本分别核对这两个数量、15个真实标题，逐项点击并验证真实路由、授权内容和两级返回。

| 入口 | 路由 | 授权内容锚点 |
| --- | --- | --- |
| 用户管理 | `/pages/manage/users` | `.manage-search` |
| 服务审核 | `/pages/manage/services` | `.manage-search` |
| 俱乐部管理 | `/pages/manage/clubs` | `.manage-search` |
| 订单与售后 | `/pages/manage/orders` | `.manage-search` |
| 游戏配置 | `/pages/manage/games` | `.game-summary`，另验编辑弹层 |
| 商家转账配置 | `/pages/manage/wallet` | `.payment-link` |
| 微信支付 | `/pages/manage/settings?tab=payment` | `.payment-status`，密钥为空 |
| 协议管理 | `/pages/manage/settings?tab=agreements` | `.agreement-content` |
| 平台设置 | `/pages/manage/platform` | 名称输入框 |
| 操作审计 | `/pages/manage/audit` | `.manage-search` |
| AI 设置 | `/pages/manage/ai` | `.ai-base-url` |
| 发送平台消息 | `/pages/manage/messages` | `.notice-title` |
| 客服设置 | `/pages/manage/support` | `.support-input` |
| 资金审核 | `/pages/manage/finance/index` | `.audit-filters` |
| 交易规则 | `/pages/manage/finance/settings` | `.finance-input` |

另增加普通玩家直接打开资金列表、资金明细、交易规则、客服设置和钱包管理时的权限拒绝检查；非负责人打开俱乐部控制台、资金页时不能见到管理操作。负责人控制台/资金页、成员押金页检查二级返回和无主底栏。

可用分段为`UI_TEST_SCOPE=admin`、`club-phone`、`club-owner`；未设置时执行完整导航。分段只证明对应范围，不可把分段成功修改成原整套成功。

已同步的其他升级：登录后可跳过按钮为“稍后完善”；个人中心无 `.stats-row` 与 `.provider-row`，订单下方为 `.me-wallet`；服务卡标题在上、标签在下；发布预览必须存在 `.service-body/.service-price`，六条独立文案至少10px，不能隐藏。用户精简打手顶部统计/大插画后，回归不再要求这些旧元素存在。

长资料回归会临时修改专用本地账号昵称/简介并恢复。旧最高价场景使用编译卡片的原生渲染数据做临时布局夹具并恢复，不将其解释为真实服务价格或财务记录。

## mini-wallet-regression.ts

保留原充值、押金独立记账、余额支付、取消订单按原资金分区退回、在途订单阻止退押金等真实本地流程。

服务详情不再查已删除的 `.provider-deposit`。现在查封面内 `.detail-deposits` 中的 `DepositBadge`，并核对“平台押金实际金额元”。不能只看容器存在就判定金额正确。

管理员钱包保留6个 `.status-filter` 和商家转账公开配置，同时有2个 `.finance-link`。旧列表不得出现直接执行转账的主按钮；“资金来源审核”必须实际跳到独立列表 `/pages/manage/finance/index`，该列表有7个 `.audit-filter`，只能进一步进入逐笔来源明细。此脚本只验证入口与原钱包流程，不能把打开来源列表写成提现已审核或真实到账。

## mini-card-matrix.ts 只读核对要点

- 当前类名匹配：`.cover-art/.cover-label`，卡片 `.service-title/.service-tags/.service-deposits/.service-price`，详情 `.detail-title/.detail-labels/.detail-deposits`。
- 断言应为卡片至少10px、详情至少14px；文案数量必须完整、文字内容一致，实际尺寸大于0，且不是隐藏或透明状态。
- 完整卡片与发布预览的价格区必须存在，不能用 `if(price)` 跳过。押金组件数量与金额应按服务端两类实缴值严格核对；0押金不显示容器。
- 所有文案、标题、标签、押金均应在图内且互不重叠；卡片价格不得侵入图内，标题在标签上方。黑金角标还应核对贴右上、仅左下圆角和实际盾牌形状。
- 各组件查询须兼容 uni-app 注册名和路径，尤其 `DepositBadge` 在 `CoverArt` 插槽内；路径字符串转义必须以实际 SDK 查询为准，不能删掉徽标断言绕过查询失败。
- 当前首屏列表截图只能证明首屏。若需要逐项视觉证据，应滚动到各卡片后分别截图；所有详情/预览截图仍需在对应文字区域可见时核看。

## mini-finance-smoke.ts

完整`all`覆盖玩家、打手、负责人、管理员的钱包分区/规则、权限、押金入口、客服、手机号门槛、技术支持。`support`分段只验证创建手机号和平台/俱乐部已配置联系方式。二维码临时配置会在结束恢复，复制和电话只调用SDK spy，不发生真实拨号、客服会话、支付或转账。无待审提现时必须在`remaining`保留“来源明细UI未执行”，不能用列表空态代替逐笔审核。

## 2026-10-23实际串行运行结果

API为本地`http://127.0.0.1:18091/api`，打开的构建目录为`frontend/dist/build/mp-weixin`。320为真实iPhone5模拟器320×568，390为iPhone12/13(Pro)390×844，均读取实际窗口信息。报告路径相对于`output/deposit-finance/`。

| 脚本与范围 | 报告 | 结果 |
| --- | --- | --- |
| 卡片矩阵390 | `card-390/report.json` | `passed=true`，12组/37项，无运行异常 |
| 卡片矩阵320 | `card-320/report.json` | `passed=true`，12组/37项，无运行异常；含最终双押金64px预留 |
| 资金/客服320完整`all` | `finance-320/report.json` | `passed=true`，31项；0异常、0Canvas颜色告警；来源明细UI无样本未执行 |
| 资金/客服390的`support` | `finance-support-390/report.json` | `passed=true`，6项；2次电话spy、2次复制spy；无真实拨号 |
| 导航390的`club-phone` | `navigation-phone-390/report.json` | `passed=true`，2项；申请手机号弹层层级和草稿保留 |
| 导航390的`club-owner` | `navigation-owner-390/report.json` | `passed=true`，7项；创建/装修游戏多选及四页返回 |
| 导航390的`admin` | `navigation-admin-390/report.json` | `passed=true`，19项；15入口、两级返回、真实曲线/游戏编辑器及支付秘密不回显 |
| 钱包390最终完整 | `wallet-390/report.json` | **未完整通过**，11项完成、0捕获异常，随后SDK读取超时；金融链路已执行，管理员另以如下scope通过，不据此改写全量结果 |
| 钱包390管理员`admin` | `wallet-admin-390/report.json` | `passed=true`，2项、0捕获异常；实际点击来源审核并读取7状态列表，无来源明细样本、未出款 |
| 导航390完整 | `.run/acceptance-attempts/navigation-390/report.json`（项目根目录） | **未完整通过**，45项完成后`timeout waiting for automator response`，随后按上三段复验；不得改写原报告 |

`finance-390/`、`finance-clubs-390/`、`navigation-390/`等早期SDK超时尝试已移至项目根目录`.run/acceptance-attempts/`，只保留为私有过程记录，不能计完整通过。钱包此前12项后超时的报告在`wallet-390-before-webview/report.json`，此前管理员超时尝试在`wallet-admin-390-before-webview/`；最终独立管理员scope通过不覆盖这些失败。导航已完成部分的34张PNG另存`navigation-evidence-390/`，附README解释整套超时，不附伪成功报告。SDK等待修复包括延后导航先回RPC、等真实数据就绪、稳定按钮定位及按官方SDK的新WebView接续间隔等待。管理员独立scope复验通过，最终全量仍有SDK读取超时；不再反复运行制造额外模拟充值，也不删除资金/徽标断言掩盖失败。

本地夹具已通过正常业务API清理：累计3个中断未付款订单取消；12个服务须同时匹配“样式验收”标题及“本地样式验收”标签后暂停，原有模拟数据保留。配置及资料临时改动恢复，未操作生产。

官方CLI读取模拟器控制台后按`error`筛选没有匹配行；这不表示IDE全部历史错误为零。SDK报告的0异常/0颜色告警只适用于各次实际捕获范围。

应用层界面本轮修复了渐变层级压暗用户文案、徽标行内基线间隙、双金额小屏底部预留；卡片矩阵及截图对照用于验证这些修复。上述报告不包含微信真机、真实授权、原生客服人工接待、商户扣款、退款或提现到账。正式产物`frontend/dist/release/mp-weixin`与本地SDK产物分开，且本轮未部署生产。完整证据、日志位置和用户复验清单见[交付验收记录](DEPOSIT_FINANCE_ACCEPTANCE.md)。
