<!-- 文件作用：为国际读者介绍开机电竞，并指向完整中文功能、配置与部署文档。 -->

# 🎮 KaiJi Esports · 开机电竞

**A multi-game service marketplace connecting players, service providers and gaming clubs.**

[中文介绍与完整使用指南](README.md) · [Repository](https://github.com/hhh0078/e-sports-platform) · [Deployment](docs/DEPLOYMENT.md) · [License](LICENSE)

## ✨ What is included?

- A WeChat Mini Program client, an H5 development preview and a Go API backend.
- Game service listings, player requests, order placement, delivery, acceptance and reviews.
- Permission-controlled order conversations for buyers, providers, associated clubs and platform administrators.
- Refund requests, evidence and platform dispute resolution.
- Club profiles, membership applications, service management and administration.
- Separate wallet balances for recharge, service earnings, pending settlement, withdrawal holds and voluntary platform deposits.
- Administration, transaction trends, server-side AI content review and system notifications.

One account can buy and provide services. Club membership is optional.

## 🧩 Stack

| Area | Technologies |
| --- | --- |
| Client | uni-app, Vue 3, TypeScript, Vite, uni-ui |
| Backend | Go 1.24, standard-library `net/http`, pgx/v5, go-redis/v9 |
| Data | PostgreSQL 16, Redis 7 |
| Deployment | Docker Compose, Nginx and HTTPS |

## 🚀 Getting started

```bash
git clone --branch main https://github.com/hhh0078/e-sports-platform.git
cd e-sports-platform
```

The local development scripts target macOS. Prepare Go 1.24+, Node.js/npm, Python 3, curl, lsof, PostgreSQL and Redis. Create the dedicated local `jijie_dev` database first; the startup script checks dependencies without creating databases or containers. Default ports are PostgreSQL `5432`, Redis `56379`, API `8090` and H5 `5173`.

Follow the [Chinese quick start](README.md#-快速开始) to configure package mirrors, install dependencies, copy the public environment templates and start the application.

## 🧪 Validation and deployment

`./scripts/verify.sh` runs backend database integration tests, race detection, static analysis, frontend checks, tests and both H5 and WeChat builds. It requires the dedicated local PostgreSQL and Redis services.

Real WeChat signup and phone authorization, merchant payments and refunds, transfer settlement, external AI capabilities, physical devices and WeChat release approval require separate configuration and acceptance. Historical production records do not validate every newer feature.

The current product name is **开机电竞 / KaiJi Esports**. Internal `jijie` module, database, cache and deployment identifiers remain compatible with existing installations.

## 📄 License and assets

See [Apache License 2.0](LICENSE) for the source code. Third-party game images and icons have separate ownership and usage boundaries documented in [ASSETS.md](docs/ASSETS.md).

## 🔎 Keywords

esports platform, esports marketplace, gaming services, game companion platform, game coaching marketplace, gaming club management, WeChat Mini Program, uni-app, Vue 3, TypeScript, Golang, PostgreSQL, Redis, order management, wallet, refunds, AI content moderation, open source.
