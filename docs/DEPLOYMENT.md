# 集结：从源码到 Docker 后端与微信小程序发布

本文对应当前项目：Go API + PostgreSQL + Redis；域名 `game.ai.58it.cn`。前端由微信发布，服务器不需要运行 Node、Vite 或 H5。游戏封面嵌入 Go 后端，随 `/static/covers/` 提供；用户上传的图片保存在 PostgreSQL 中。

## 本次已部署状态

2026-10-20 已从 Gitee `master` 的提交 `6ea28d5` 完成源码部署，当前 API 镜像为 `jijie-api:20261020-gitee-6ea28d5`，三个容器均为 healthy。源码位于 `/www/wwwroot/game.ai.58it.cn`，生产配置为该目录下的 `deploy/.env`，权限 `600`；部署前数据库备份为 `.deploy-backups/pre-gitee-20261020.sql.gz`，并已通过 gzip 完整性检查。原 `/opt/jijie` 发布目录及历史备份仍保留，可用于故障回退。

本次外网复查覆盖54项请求，其中43项为游戏图片，全部通过。完整非私密结果见 `output/community/production-http-check-20261020.json`，部署实屏见 `output/community/server-final-20261020.png`。演示数据数量与10月18日保持一致；演示清理保护、真实经营统计隔离及数据库错误脱敏已包含在新镜像中。

正式小程序包为 `frontend/dist/build/mp-weixin`，连接上述 HTTPS API 并开启合法域名校验，体积与配置以 `output/community/release-mini-20261020.json` 为准。官方 SDK 原生 `wx.request` 已成功读取正式目录，`wx.login` 与正式后端身份准备接口也已返回成功；页面联网检查见 `output/community/mini-production-20261020.json`。迁移到买家自己的 AppID 时仍须完成第5节的微信域名配置。

2026-10-20 已在 `43.228.79.108` 的 `/www/wwwroot/game.ai.58it.cn/deploy` 运行项目专属 Docker Compose：API、PostgreSQL 16、Redis 7 均为 healthy。Compose 项目名仍为 `jijie`，因此沿用原 PostgreSQL 与 Redis 命名卷。Nginx 原有证书及验证规则保留，站点反代至 `127.0.0.1:18090`。`https://game.ai.58it.cn/health` 返回数据库/Redis正常且 dev_mode=false。

正式域名已验证13款游戏配置、服务、俱乐部、协议及封面图片正常；开发体验账号接口404、未登录管理接口401、`.env`访问404。服务器 `.env` 权限600。60用户/6俱乐部/130服务/52订单/260消息/8评价的演示数据已经导入。

10月20日已验证当前 AppID/AppSecret 能调用微信官方接口，并通过原生 `wx.login` 获取 code，再由正式后端成功完成身份准备。此前独立自动化使用 `evaluate` 上下文出现的 `appid missing`、`url not in domain list` 是无效诊断，已经由原生调用和真实页面复核纠正。最终勾选协议建立正式账号、真机手机号和商户支付退款仍需独立验收；商户资料未填写。微信管理后台受当前工具访问限制，运营者的隐私指引及发布类目仍需按第5节核对，不能由构建替代。

## 1. 目录和配置文件

- `backend/Dockerfile`：直接从 Go 源码构建 API 和演示工具。
- `deploy/compose.yml`：独立的 API、PostgreSQL 16、Redis 7；不改现有其他项目。
- `deploy/.env.example`：Redis、PostgreSQL 和微信的配置模板。
- 当前服务器实际私密配置：`/www/wwwroot/game.ai.58it.cn/deploy/.env`，权限 `600`，并由 `.gitignore` 排除。里面有数据库密码、Redis 密码、微信密钥和支付配置加密主密钥，不可提交 Git、不可发给源码买家。Nginx 的 `/` 已反向代理到 API，不直接提供目录文件；部署后仍应验证公网访问 `.env` 返回非成功状态。
- 后端只监听服务器回环地址 `127.0.0.1:18090`，由宝塔 Nginx 提供 HTTPS。PG 和 Redis 不映射公网端口。

## 2. 第一次从源码创建 Docker

服务器需 Docker Engine 与 Compose 插件。将源码放到例如 `/opt/jijie-source`，执行：

```bash
cd /opt/jijie-source
cp deploy/.env.example deploy/.env
chmod 600 deploy/.env
openssl rand -hex 24
openssl rand -hex 24
openssl rand -base64 32
```

前两个随机值分别填入 `POSTGRES_PASSWORD`、`REDIS_PASSWORD`；最后一个填入 `CONFIG_ENCRYPTION_KEY`。密码采用十六进制，可直接安全拼接 PostgreSQL URL。填写自己的 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET`。保留 `POSTGRES_DB=jijie`、`POSTGRES_USER=jijie` 即可。

```bash
cd deploy
docker compose config --quiet
docker compose build api
docker compose up -d
docker compose ps
curl --fail http://127.0.0.1:18090/health
```

`Dockerfile` 构建阶段使用国内 Go 镜像 `https://goproxy.cn,direct`。若镜像仓库不可达，先在服务器配置可用的 Docker 镜像源，再执行 build；不要关闭 TLS 校验。

正式服务固定 `APP_ENV=production`、`DEV_MODE=false`。开发登录、模拟手机号与模拟支付接口不会因为前端按钮或请求参数而启用。生产数据库不会自动生成可登录的测试管理员。

本次部署也提供了本机交叉编译的 Linux amd64 二进制发布包，在服务器用精简运行镜像构建；这与上面的源码构建提供相同 API。更新源代码后可直接改用源码 Dockerfile 构建。

## 3. 使用已有 PostgreSQL / Redis

默认 Compose 使用项目专属的两个容器与持久卷。若要使用外部数据库，先给本项目单独建立数据库和账户，配置以下后端环境变量：

```dotenv
DATABASE_URL=postgres://jijie:已编码的密码@数据库主机:5432/jijie?sslmode=require
REDIS_ADDR=Redis主机:6379
REDIS_PASSWORD=专属密码
REDIS_PREFIX=jijie:production:
APP_ENV=production
DEV_MODE=false
ADDR=0.0.0.0:8090
```

跨服务器连接应使用数据库 TLS；同一个独立 Docker 私网中可用 Compose 的 `sslmode=disable`。外部数据库部署时，删除 compose 的本地 postgres/redis services 与 `depends_on`，并将 api.environment 里固定的 `DATABASE_URL`、`REDIS_ADDR` 替换成外部配置；只改 `.env` 而不移除该覆盖不会生效。

## 4. 宝塔域名和 HTTPS

1. DNS 的 `game.ai.58it.cn` 指向服务器；在宝塔确认这个站点属于本项目。
2. 安装有效 SSL 证书，开启自动续期；微信正式请求必须使用 HTTPS。
3. 在该站点配置中加入反向代理到 `http://127.0.0.1:18090`，保留现有证书路径与 `/.well-known/` 验证规则。

反向代理核心内容：

```nginx
client_max_body_size 5m;
location / {
    proxy_pass http://127.0.0.1:18090;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_connect_timeout 10s;
    proxy_read_timeout 30s;
}
```

站点不能同时保留另一个 `location /`，应修改原规则；不能覆盖服务器其他站点。保存前先备份原配置，执行 `nginx -t` 成功后 reload。测试：

```bash
curl --fail https://game.ai.58it.cn/health
curl --fail https://game.ai.58it.cn/api/bootstrap
curl -I https://game.ai.58it.cn/static/covers/official/delta-icon.jpg
```

## 5. 微信登录、手机号与合法域名

前端 `frontend/src/manifest.json` 的 `mp-weixin.appid` 与服务端 `.env` 的 `WECHAT_APP_ID` 必须一致。`AppSecret` 只在服务器配置。

微信公众平台 → 开发管理 → 开发设置 → 服务器域名，配置：

- request 合法域名：`https://game.ai.58it.cn`
- uploadFile 合法域名：`https://game.ai.58it.cn`
- downloadFile 合法域名：`https://game.ai.58it.cn`

不要填写 `/api` 路径。若微信要求主体、备案或域名验证，按该账号实际要求完成。开发者工具关闭域名校验只用于本地，不能当成正式发布验收。

进入小程序时 `wx.login` 取得临时 code，服务器向官方换身份并保存短期票据；用户主动勾选当前《用户协议》和《隐私政策》后，才创建账户和登录会话。客户端任意传 OpenID 无法登录。协议修改后版本不匹配会要求重新阅读、勾选。

手机号通过 `getPhoneNumber` 官方 code 兑换，用户拒绝、测试号无权限、官方失败均不标记成功。请在微信后台开通对应手机号能力；正式环境未验证手机号不能下单、发布服务或发布需求。登录后的头像昵称是可选的；所有账号也可从“我的 → 个人资料”设置。

在微信后台填写与实际数据处理一致的隐私保护指引，包括手机号、头像/相册、上传图文及订单证据；小程序内两份协议可在管理后台修改。默认文本是运营初稿，上线前补齐主体名称、联系方式、保存期限等实际信息。

## 6. 首次管理员

首次没有默认管理员密码。先使用自己的微信在此 AppID 下正常登录一次；从可信服务器运维渠道核对该用户私密 `identities` 记录里的 OpenID。把它填入服务器 `.env` 的 `BOOTSTRAP_ADMIN_OPENID`，重建 API 容器：

```bash
cd /www/wwwroot/game.ai.58it.cn/deploy
docker compose up -d --force-recreate api
```

退出小程序账号并再次微信登录，官方返回的身份精确匹配后将一次性授予管理员。确认“我的 → 管理后台”可见后，清空该变量并再次重建容器。此配置不通过公开页面获取或授予，昵称不作为判断依据。权限、审计和一次性标记保留在数据库。

## 7. 微信支付与退款

先完成微信支付商户开通、商户与小程序 AppID 关联及实际服务类目审核。在小程序“管理后台 → 设置 → 协议与支付配置”填写：

1. 商户号（MchID）。
2. 商户 API 证书序列号和 RSA 私钥 PEM。
3. 32 字节 APIv3 密钥。
4. 微信支付平台公钥 PEM 及其公钥 ID/证书序列号，必须与微信发送的 `Wechatpay-Serial` 对应。
5. 支付回调：`https://game.ai.58it.cn/api/payments/wechat/notify`。
6. 退款回调：`https://game.ai.58it.cn/api/payments/wechat/refund-notify`。

秘密输入留空表示保留，页面只显示“已配置/未配置”，不会回显原值。数据库中以 AES-GCM 加密存储；`CONFIG_ENCRYPTION_KEY` 必须长期保留并与数据库备份配套。当前单实例加载配置即可生效；多实例部署需同步重启所有实例。

支付金额由服务端订单决定，微信支付回调经过签名验证和金额校验。买家支付取消不应把订单标成已支付。退款申请 → 打手拒绝或同意 → 买家申请平台介入 → 平台裁决；真实退款成功以微信结果为准。商户配置未填写时不会伪装为支付成功。

真实测试必须使用普通真实用户发布并审核通过的服务。以允许的最低金额完成一次支付、同步查询、交付确认、退款协商、平台介入和退款到账核验；`demo_` 演示订单禁止真实付款。当前未配置商户的部署不能声称真实资金链路已验收。

## 8. 编译、上传和发布小程序

```bash
cd 项目源码目录
./scripts/release-mini.sh
```

脚本默认设置 `VITE_API_BASE=https://game.ai.58it.cn/api`，并将生成项目的 `setting.urlCheck` 设为 `true`。其他域名用 `VITE_API_BASE=https://自己的域名/api ./scripts/release-mini.sh`。本地开发构建仍可保持关闭校验，正式构建不得依赖该调试选项绕过微信后台域名设置。

在微信开发者工具导入 `frontend/dist/build/mp-weixin`，确认 AppID、勾选域名校验，重新编译。用本人微信扫码真机预览，完成本文末尾测试，然后“上传”体验版，添加体验成员，确认无误后提交审核并发布。测试号能力不能替代正式主体账号可发布能力；需要迁移时同时改前端 AppID 和后端两项微信配置。

H5 只用于本地开发预览，不部署到本次服务器。发布包通过后端托管游戏图，首屏需能访问 downloadFile 域名。

另有独立的本地模拟包 `frontend/dist/local/mp-weixin`，连接本机 `127.0.0.1:8090` 并允许本地调试，用于五种体验身份与测试交易，不能作为正式上传包。重新生成本地包：

```bash
UNI_OUTPUT_DIR="$(pwd)/frontend/dist/local/mp-weixin" npm --prefix frontend run build:mp-weixin
```

本地 H5 仍为 `http://127.0.0.1:5173/`。两个包各自独立，重新运行 `release-mini.sh` 不会把本地目录当作正式发布产物。

## 9. 演示数据与源码展示

```bash
cd /www/wwwroot/game.ai.58it.cn/deploy
docker compose exec -T api /app/jijie-demo seed
docker compose exec -T api /app/jijie-demo status
```

会生成 60 个虚构用户、6 个俱乐部、42 条成员关系、130 项服务、52 个订单、260 条聊天、8 条评价和 60 个头像。重复 seed 不覆盖已有记录。所有记录带演示标记；管理员可查看订单，但不允许付款、退款、接单或修改交易。普通访客可看演示服务、玩家和俱乐部主页。

清理命令 `docker compose exec -T api /app/jijie-demo clear` 仅清理有双重演示标记的数据；发现真实业务引用则全部拒绝，避免损坏真实订单。详见 `DEMO_DATA.md`。

出售源码前移除 `.env`、`.deploy`、数据库备份、证书私钥、日志、用户数据、会话与真实配置；买家使用自己的 AppID、商户号、域名和密码。游戏素材来源清单见 `ASSETS.md`，商业分发时应确认相应素材使用许可。

## 10. 备份、升级与回滚

```bash
cd /www/wwwroot/game.ai.58it.cn/deploy
mkdir -p backup
chmod 700 backup
docker compose exec -T postgres sh -c 'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"' | gzip > backup/jijie-$(date +%Y%m%d-%H%M).sql.gz
cp .env backup/config-$(date +%Y%m%d-%H%M).env
chmod 600 backup/*
```

数据库包含上传图、订单证据及协议内容，备份需覆盖整个数据库。Redis 会话有独立持久卷，丢失后用户需重新登录；不要执行 `docker compose down -v`，它会删除业务持久卷。

升级前备份并记录当前镜像标签；源码构建新标签、`docker compose up -d api`，验证 health/真实页面。失败时把 `.env` 的 `IMAGE_TAG` 恢复旧值，再 `docker compose up -d --no-build api`。本次新增字段在 JSON 结构中，未删表；以后有破坏性迁移时需单独设计数据库回滚，不能只换镜像。

## 11. 发布验收清单

- HTTPS、域名校验、所有游戏图片和上传图均正常。
- 四个导航访客可浏览；个人中心零数据，订单小字提示。
- 登录未勾选被拦截，两协议能打开；版本更新要求重新同意。
- 真实微信登录成功，可跳过头像昵称；个人资料能独立保存。
- 未验证手机号下单/发布被拦，拒绝授权不能绕过。
- 头像方形、封面 16:9、详情/证据保留比例；大图压缩、取消不丢表单。
- 俱乐部装修、审核、申请/考核/认证成员与分享正确。
- 管理员可管理协议与支付；普通用户无管理权限。
- 非演示订单完成真实支付、四方聊天、退款与介入链路。
- 演示数据能展示、不能真实交易，清理不影响真实数据。
- 微信真机长昵称、13游戏标签、320px窄屏不挤压或溢出。

本地自动验证入口：`./scripts/verify.sh`；真实微信、手机号和商户资金验收需按当前账号实际能力分别记录，不能由构建通过替代。
