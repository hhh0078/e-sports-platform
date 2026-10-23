# 2026-10-23 生产发布与回滚说明

文件作用：记录本次已完成的宝塔 MCP Docker 后端升级及可执行回滚边界，不含私密配置值。

## 本次实际发布

- 生产域名：`https://game.ai.58it.cn`。
- 源码：`/www/wwwroot/game.ai.58it.cn`，精确提交 `ecac760b185ec33f7025c01a7e072d734ef49343`。
- 本项目 origin：`https://github.com/hhh0078/e-sports-platform.git`。旧 Gitee 地址保存在最初私密备份的 `remote-migration.json`，未改全局 Git 配置。
- 当前镜像：`jijie-api:20261023-wallet-ecac760`。
- 当前镜像 ID：`sha256:2223ed469980de5cd3613cb059b9d84f5342e782228413623ebd604156eda3b8`。
- 旧镜像：`jijie-api:20261020-gitee-6ea28d5`，保留可用。
- 旧镜像 ID：`sha256:06c529ce28e5c62d73cbfa9c8ded4557b696d26438b64742ba8b1ea1949905aa`。

服务器 GitHub 拉取超时后，使用官方 MCP Upload 上传经 SHA256 校验的增量 Git bundle；服务器验证 bundle、核对 FETCH_HEAD 后精确切换源码。构建和激活分开，构建成功才更新 `.env` 的 IMAGE_TAG，并只执行 `docker compose up -d --no-deps --no-build api`。

PG、Redis 容器 ID、卷和端口均未改变；API 仍只绑定 `127.0.0.1:18090`。所有私密配置原字节保留，只有 IMAGE_TAG 一行改变；配置权限600，加密主密钥及运行时取值一致。没有进行真实支付、退款、转账或猜测授予管理员。

## 最终一致快照

最终快照目录：`/www/wwwroot/game.ai.58it.cn/.deploy-backups/pre-wallet-activation-20261023-055806`，目录权限700。

- `database.sql.gz`：PostgreSQL `pg_dump` 一致快照，123542字节，已独立检查 dump/gzip 退出状态并通过 `gzip -t`。
- SHA256：`3a276b89238c7fc99828dce1d365d51cd57c6cf90997d68b332db330140c7f00`。
- `config.env`：切换前的私密配置，只留服务器，权限600。
- `manifest.json`：切换前旧业务计数、容器 ID/卷/端口、旧新镜像记录。
- `verification-final.json`：最终脱敏复核结果，权限600。

最初的 `pre-wallet-polish-20261023-051108` 安全备份仍保留。一次最终快照包装脚本因已关闭 stdin 的 flush 报错，在修改配置及重建 API 之前结束；修正后重新完成上述最终一致快照。未用失败尝试替代有效备份。

旧业务行数保留：62用户、13游戏、130服务、6俱乐部、42成员、52订单、260消息、8评价、62图片证据、2身份映射。settings 从3增为10，新增的是幂等迁移标记及协议历史；没有凭旧订单状态补造钱包收入，所有新增资金事实表仍为空。

## 回滚操作

仅在新功能还没有发生真实资金交易、且新 API 验证失败时，可恢复旧 API 镜像。先保留错误日志和当前数据库状态；不要覆盖配置中的商户资料或加密主密钥。

通过宝塔 MCP Bash 在本项目目录执行下列步骤。先核对当前 IMAGE_TAG 仍为本次新标签；如果运营者之后已经升级，不要盲目回退。

```sh
cd /www/wwwroot/game.ai.58it.cn/deploy
python3 - <<'PY'
from pathlib import Path
import os

p = Path('.env')
lines = p.read_bytes().splitlines(keepends=True)
prefix = b'IMAGE_TAG='
found = [line for line in lines if line.startswith(prefix)]
if len(found) != 1 or found[0].strip() != b'IMAGE_TAG=20261023-wallet-ecac760':
    raise SystemExit('当前版本不符合本次回滚前提，停止')
updated = []
for line in lines:
    if line.startswith(prefix):
        ending = b'\r\n' if line.endswith(b'\r\n') else b'\n' if line.endswith(b'\n') else b''
        updated.append(b'IMAGE_TAG=20261020-gitee-6ea28d5' + ending)
    else:
        updated.append(line)
temporary = p.with_name('.env.rollback-wallet')
fd = os.open(str(temporary), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'wb') as target:
    target.write(b''.join(updated))
    target.flush()
    os.fsync(target.fileno())
os.replace(temporary, p)
PY
docker compose config --quiet
docker compose up -d --no-deps --no-build api
docker compose ps
curl --fail https://game.ai.58it.cn/health
```

实际执行时每步检查退出状态；修改配置成功不代表容器已切换，必须核对容器镜像及健康状态。PG/Redis 不执行重建，不执行 `down -v`，不切成其他 Compose 项目名。

新增真实订单、补款、退款、钱包或转账之后，旧 API 不理解完整新资金模型，不能将换旧镜像视为完整回滚。此时先停止新增交易并保留证据，优先向前修复；不得自动恢复旧数据库快照覆盖发布后交易。加密主密钥必须和数据库一起保留。

## 验收边界

后端完整结果见同目录 `production-deploy-20261023.json`。新增表、中文注释和资金不可变触发器已在生产核实；公共路由和图片正常，开发接口404、管理接口401、私密配置和快照文件不可公网读取。备份目录自身返回405且只允许OPTIONS，正文为通用拒绝，并非目录列表。

目前生产尚未配置平台管理员、AI审核、微信商户支付资料及提现场景；新生产审核在未配置时会阻止内容提交，资金能力不会假成功。部署与本地自动验收不能替代商户真实支付/退款/提现到账。小程序上传、体验版设置、提审及公开发布由独立微信交付证据说明。
