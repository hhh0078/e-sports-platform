# 开机电竞 · 品牌更新记录

文件作用：记录公开品牌改名范围、既有数据兼容方式、README 更新及本次验证边界。

## 名称与展示

2026-10-23 起，当前对外产品名称统一为「开机电竞」。GitHub README 已重写为项目介绍，加入角色功能、技术标签、功能实屏、启动与部署入口，以及底部中英文相关关键词；英文 README 也已替换原模板。

客户端名称、H5 标题、导航与登录标题、首页、分享标题、默认站点、系统发信人、支付商品文案及默认提现备注同步改名。当前设计图与说明同步品牌文案；`design/history/`、既有 `output/` 验收记录和历史交易快照保留原名称。

## 数据兼容

- `jijie` Go 模块、数据库、缓存键、容器和命名卷等内部技术标识保留，避免改名影响已有数据和会话。
- 品牌迁移在后端原有种子事务与锁内执行，成功后记录一次性标记，重复启动保持幂等。
- 旧默认站点名只在名称、公告精确匹配且没有管理员保存审计时升级；自定义配置不覆盖。开发体验管理员仅处理固定开发账号与原默认资料，不改真实用户。
- 默认提现备注精确匹配时升级，历史转账和资金快照不改。AI 旧默认提示词在读取时精确兼容，原密文、模型和密钥保留，不增加启动解密条件。
- 默认服务协议与隐私政策发布新版本 `2026-10-22.3`，保留旧默认 `.2` 正文和用户同意记录。已定制条款或被占用的版本号不覆盖，应由管理员核对后另发新版本。

## 本次验证

- 前端类型检查、开发规范检查、329 项现有测试、H5 与微信构建通过。
- 额外执行 `./scripts/release-mini.sh`，正式微信包构建通过，合法域名校验 `urlCheck=true`，且未携带本地 `project.private.config.json`；没有上传或发布到微信。
- 使用本地模拟公开目录和协议，浏览器复核首页与登录页在 320 / 390 宽度下的品牌显示、按钮间距与横向溢出；四张实屏见 `output/playwright/kaiji-brand-*.png`。
- 后端普通测试、静态检查、全量真实 PostgreSQL / Redis 集成与竞态检查通过。新增测试覆盖默认迁移、定制配置保留、重复启动、协议版本碰撞，以及旧同意、聊天、订单快照不变。
- README 的四张功能预览来自既有历史本地模拟实屏，已明确标注；它们不作为本轮新功能或真实资金验收证据。
- README 按 GitHub 风格 Markdown 渲染后复核：7 张表格、所有图片与锚点正常，无原样加粗标记；桌面与 390 宽度无页面横向溢出，实屏见 `output/playwright/kaiji-readme-*.png`。此项为本地渲染检查。

本次没有部署生产，也没有改写生产配置或数据库。真实微信登录、手机号、商户付款退款、商家转账到账、真实 AI 和真机发布仍需独立验收。

## 当前设计资产

品牌文字使用内置 ImageGen 的 `text-localization` 模式局部编辑，保持原 390 × 844 尺寸及无关布局、画面和控件。完整编辑提示词保存在各页面设计说明的“品牌名称更新（2026-10-23）”部分。

| 页面 | 设计图 | 编辑说明与提示词 |
| --- | --- | --- |
| 用户协议 | [agreements.png](../design/pages/account/agreements.png) | [说明](../design/pages/account/agreements.design.md) |
| 登录 | [login.png](../design/pages/account/login.png) | [说明](../design/pages/account/login.design.md) |
| 主页面 | [index.png](../design/pages/main/index.png) | [说明](../design/pages/main/index.design.md) |
| 玩家首页 | [player.png](../design/pages/main/player/player.png) | [说明](../design/pages/main/player/player.design.md) |
| 游戏选择 | [player-game-picker.png](../design/pages/main/player/player-game-picker.png) | [说明](../design/pages/main/player/player-game-picker.design.md) |
| 打手工作台 | [worker.png](../design/pages/main/worker/worker.png) | [说明](../design/pages/main/worker/worker.design.md) |
| 平台设置 | [platform.png](../design/pages/manage/platform.png) | [说明](../design/pages/manage/platform.design.md) |
| 协议管理 | [settings.png](../design/pages/manage/settings.png) | [说明](../design/pages/manage/settings.design.md) |
