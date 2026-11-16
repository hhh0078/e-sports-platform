# main自动部署

文件作用：说明宝塔两分钟自动部署入口、实际版本判断与本次用户明确要求的不备份边界。

- 宝塔Shell任务每两分钟执行 `cd /www/wwwroot/game.ai.58it.cn && bash auto_deploy.sh`。每轮先取得 `/opt/jijie-auto-deploy/job.lock` 的非阻塞进程锁，已有部署运行时直接跳过。
- 默认只跟踪GitHub的 `origin/main`。跟踪文件有未提交修改、提交无法快进或远端访问失败时停止；站点原有未跟踪文件和 `deploy/.env` 保留。不会使用 `git reset --hard` 或 `git clean`。
- 根据backend、frontend、deploy及发布入口等实际Git对象计算应用输入摘要。只有实际API镜像标签和H5版本文件均匹配才跳过构建，避免“Git更新成功但发布失败”永远不再重试。纯文档提交不触发应用重建。
- 新版本依次执行npm ci、类型检查、规范检查、前端测试、H5/微信正式构建和分享事件检查，再构建包含Python FontTools的后端镜像。国内npm、Go、Alpine和pip镜像沿用项目配置。
- Docker磁盘不足3GiB时停止，不清理其他项目缓存。仅更新Compose的api服务，不重启PostgreSQL或Redis，不更换卷、现有密钥或账户。
- H5保存到 `/opt/jijie-auto-deploy/releases/<输入摘要>/h5`，用current软链切换；`assets`保留已有内容哈希资源以支持已打开的旧浏览器页面。API镜像和公开 `deployment-version.json` 分别记录真实应用提交。
- 激活后验证API健康、正式网页版本及六款字体的WOFF、大小、跨域和缓存响应。失败记录在 `status.json`，下一轮重试；原镜像/已有H5软链能恢复时恢复应用指向。
- **用户于2026-11-17明确要求不备份，本流程不创建数据库、配置或源码备份，也不自动恢复数据库。** 应用启动的现有幂等迁移按项目正常执行，不盲目撤回或恢复真实业务数据。
- 完整日志由宝塔任务记录；`/opt/jijie-auto-deploy/status.json`仅含版本、阶段及非私密结果。手动调用 `bash auto_deploy.sh --verify` 可只验证正式域名字体。
- 自动发布范围是Go后端和H5网页。微信包会通过构建检查；微信公众平台的上传、审核及正式发布仍独立进行。

测试入口：`python3 -m unittest discover -s scripts -p 'test_auto_deploy.py'`。
