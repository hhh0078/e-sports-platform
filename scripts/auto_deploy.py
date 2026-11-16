"""文件作用：串行拉取main，按应用输入构建并切换Go/H5，验证线上字体，不创建或恢复数据备份。"""
import argparse
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time
import urllib.parse
import urllib.request

SOURCE = Path(__file__).resolve().parents[1]
STATE = Path("/opt/jijie-auto-deploy")
PUBLIC = "https://game.ai.58it.cn"
INPUTS = ("backend", "frontend", "deploy", "auto_deploy.sh", "scripts/auto_deploy.py",
          "scripts/check-frontend-standards.ts", "scripts/check-share-runtime-hooks.ts")


def write_json(path, value):
    """将不含凭据的状态原子写入指定文件，避免定时检查读到半份结果。"""
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")
    temporary.replace(path)


def read_json(path):
    """读取部署状态；首次部署没有记录时返回空对象，损坏记录则明确失败。"""
    return json.loads(path.read_text()) if path.is_file() else {}


def input_digest(tree):
    """根据Git记录的应用文件路径和对象摘要判断是否需要部署，排除纯文档改动。"""
    return hashlib.sha256(tree.encode()).hexdigest()


def request(url):
    """读取固定本机或正式域名资源，设置超时且不携带登录信息。"""
    with urllib.request.urlopen(url, timeout=25) as response:
        return response.headers, response.read(1024 * 1024)


def health():
    """确认现有数据库和Redis可用，且正式服务没有开启开发模式。"""
    _, raw = request("http://127.0.0.1:18090/health")
    value = json.loads(raw).get("data", {})
    if value.get("status") != "ok" or value.get("dev_mode") is not False:
        raise RuntimeError("正式API健康状态无效")


class Release:
    """只更新本项目API和H5，不重建数据库/Redis、不读取或输出环境文件内容。"""

    def __init__(self, source=SOURCE, state=STATE):
        """绑定源码及独立状态目录；生产环境不接受任意远程仓库或分支参数。"""
        self.source, self.root = Path(source), Path(state)
        self.record = self.root / "status.json"
        self.stage = "preflight"
        self.target = ""

    def run(self, args, timeout=120, capture=False, env=None):
        """执行固定参数命令，凭据仅由Compose环境文件自行读取，不进入命令或日志。"""
        result = subprocess.run(args, cwd=self.source, env={**os.environ, **(env or {})},
                                text=True, stdout=subprocess.PIPE if capture else None,
                                stderr=subprocess.PIPE if capture else None, timeout=timeout)
        if result.returncode:
            raise RuntimeError(f"{self.stage}阶段命令失败：{Path(args[0]).name}，退出码{result.returncode}")
        return result.stdout.strip() if capture else ""

    def git(self, *args):
        """以固定源码目录执行Git，只返回所需元数据，不使用强制重置或清理。"""
        return self.run(["git", *args], capture=True)

    def update(self, status, **values):
        """保留上次实际部署身份，记录本次检查/失败，不能把拉取成功当作上线成功。"""
        previous = read_json(self.record)
        write_json(self.record, {**previous, **values, "status": status, "stage": self.stage,
                                "checked_commit": self.target,
                                "checked_at": dt.datetime.now(dt.timezone.utc).isoformat()})

    def running(self):
        """只读取API容器镜像、公开版本标签及状态，不获取环境变量中的私密信息。"""
        template = ('{"image_id":"{{.Image}}","image":"{{.Config.Image}}",'
                    '"running":{{.State.Running}},"commit":"{{index .Config.Labels "org.opencontainers.image.revision"}}",'
                    '"input":"{{index .Config.Labels "cn.58it.jijie.input"}}"}')
        return json.loads(self.run(["docker", "inspect", "jijie-api-1", "--format", template], capture=True))

    def assert_source(self):
        """拒绝发布过程中漂移的提交或跟踪文件，保留站点原有未跟踪文件。"""
        if self.git("rev-parse", "HEAD") != self.target or self.git("status", "--porcelain", "--untracked-files=no"):
            raise RuntimeError("发布过程中源码发生变化，停止切换")

    def verify_fonts(self):
        """从正式HTTPS域名验证六款WOFF实际字形子集、跨域和缓存头。"""
        catalog = read_json(self.source / "backend/internal/artfonts/catalog.json")
        rows = []
        for font in catalog["fonts"]:
            url = PUBLIC + "/api/fonts/" + catalog["version"] + "/" + font["id"] + ".woff?text=" + urllib.parse.quote("限时优惠")
            headers, raw = request(url)
            if not raw.startswith(b"wOFF") or not 44 < len(raw) < 128 * 1024:
                raise RuntimeError("正式字体子集无效：" + font["id"])
            if headers.get("Access-Control-Allow-Origin") != "*" or "immutable" not in headers.get("Cache-Control", ""):
                raise RuntimeError("正式字体跨域或缓存响应无效：" + font["id"])
            if not headers.get("Content-Type", "").startswith("font/woff"):
                raise RuntimeError("正式字体类型无效：" + font["id"])
            rows.append({"font": font["id"], "bytes": len(raw)})
        return rows

    def deploy(self):
        """同步main并验证新构建，成功后才登记实际前后端版本；失败不复制或恢复数据库。"""
        if self.git("status", "--porcelain", "--untracked-files=no"):
            raise RuntimeError("服务器跟踪文件有未提交修改，停止更新")
        self.stage = "fetch"
        self.git("fetch", "--filter=blob:none", "origin", "refs/heads/main:refs/remotes/origin/main")
        self.target = self.git("rev-parse", "origin/main")
        self.git("merge-base", "--is-ancestor", "HEAD", self.target)
        branches = self.git("branch", "--list", "main")
        self.git("switch", "main") if branches else self.git("switch", "-c", "main")
        self.git("merge", "--ff-only", "origin/main")
        self.assert_source()
        digest = input_digest(self.git("ls-tree", "-r", self.target, "--", *INPUTS))
        before = self.running()
        current = self.root / "current"
        live = read_json(current / "h5/deployment-version.json")
        if before["running"] and before["input"] == digest and live.get("input_digest") == digest and live.get("source_commit") == before["commit"]:
            health()
            if (self.root / "nginx-ready").is_file():
                _, version = request(PUBLIC + "/deployment-version.json")
                if json.loads(version).get("input_digest") != digest:
                    raise RuntimeError("正式H5版本不匹配，保留当前应用并停止")
            self.stage = "idle"
            self.update("up_to_date", deployed_commit=before["commit"], input_digest=digest, image_id=before["image_id"])
            print("主分支已核对，实际前后端已是当前应用版本，无需构建", flush=True)
            return
        docker_root = self.run(["docker", "info", "--format", "{{.DockerRootDir}}"], capture=True)
        if shutil.disk_usage(docker_root).free < 2 * 1024 ** 3:
            raise RuntimeError("Docker磁盘余量不足2GiB，保留现有服务并停止构建")
        self.stage = "build"
        self.update("building")
        print("开始构建 main=" + self.target, flush=True)
        self.run(["npm", "--prefix", "frontend", "ci", "--registry=https://registry.npmmirror.com", "--no-audit", "--no-fund"], timeout=900)
        for task in ("typecheck", "check:standards", "test", "build:h5", "build:mp-weixin"):
            if task == "test":
                tests = sorted(str(path.relative_to(self.source)) for path in (self.source / "frontend/src/lib").glob("*.test.ts"))
                self.run([str(self.source / "frontend/node_modules/.bin/tsx"), "--test", "--test-concurrency=2", *tests], timeout=900)
            else:
                self.run(["npm", "--prefix", "frontend", "run", task], timeout=900)
        self.run([str(self.source / "frontend/node_modules/.bin/tsx"), "scripts/check-share-runtime-hooks.ts", "frontend/dist/build/mp-weixin"])
        tag = "auto-" + digest[:16]
        image = "jijie-api:" + tag
        self.run(["docker", "build", "--build-arg", "SOURCE_COMMIT=" + self.target,
                  "--build-arg", "SOURCE_INPUT=" + digest, "-t", image, "backend"], timeout=1800)
        self.assert_source()
        release = self.root / "releases" / digest
        release.mkdir(parents=True, exist_ok=True)
        h5 = release / "h5"
        shutil.copytree(self.source / "frontend/dist/build/h5", h5, dirs_exist_ok=True)
        write_json(h5 / "deployment-version.json", {"source_commit": self.target, "input_digest": digest})
        # 内容哈希资源合并到独立缓存，旧浏览器标签页仍能取得上一版本JS；不复制旧配置或业务数据。
        assets = self.root / "assets"
        shutil.copytree(h5 / "assets", assets, dirs_exist_ok=True)
        self.stage = "activate"
        self.update("activating")
        compose = ["docker", "compose", "--env-file", "deploy/.env", "-f", "deploy/compose.yml", "-p", "jijie"]
        previous = current.resolve() if current.is_symlink() else None
        activated = False
        try:
            activated = True
            self.run([*compose, "up", "-d", "--no-deps", "--no-build", "api"], timeout=180, env={"IMAGE_TAG": tag})
            for attempt in range(45):
                try:
                    health()
                    break
                except Exception:
                    if attempt == 44:
                        raise
                    time.sleep(2)
            temporary = self.root / "current.next"
            if temporary.is_symlink():
                temporary.unlink()
            temporary.symlink_to(release)
            temporary.replace(current)
            # 首次安装先通过本机验证，配置完成后必须再次执行正式域名验收。
            if not (self.root / "nginx-ready").is_file():
                fonts = []
            else:
                _, page = request(PUBLIC + "/")
                if b"<html" not in page.lower():
                    raise RuntimeError("正式H5首页不是HTML")
                _, version = request(PUBLIC + "/deployment-version.json")
                if json.loads(version).get("input_digest") != digest:
                    raise RuntimeError("正式H5版本不匹配")
                fonts = self.verify_fonts()
            running = self.running()
            if running["input"] != digest or running["commit"] != self.target:
                raise RuntimeError("实际API镜像与候选源码不匹配")
            self.stage = "complete"
            self.update("deployed", deployed_commit=self.target, input_digest=digest, image_id=running["image_id"], fonts=fonts)
            print("前后端切换完成：" + self.target, flush=True)
        except Exception:
            # 只恢复现有镜像/软链，不创建备份、不触碰数据库内容；已有迁移不会被盲目逆转。
            if previous:
                temporary = self.root / "current.rollback"
                if temporary.is_symlink():
                    temporary.unlink()
                temporary.symlink_to(previous)
                temporary.replace(current)
            elif current.is_symlink():
                current.unlink()
            if activated and before["image"].startswith("jijie-api:"):
                self.run([*compose, "up", "-d", "--no-deps", "--no-build", "api"], timeout=180,
                         env={"IMAGE_TAG": before["image"].split(":", 1)[1]})
            raise


def main():
    """持有非阻塞进程锁后执行发布；重叠轮次跳过，异常写入状态并让下轮重试。"""
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify", action="store_true", help="只验证正式域名的六款字体")
    args = parser.parse_args()
    os.umask(0o022)
    STATE.mkdir(parents=True, exist_ok=True)
    with (STATE / "job.lock").open("a") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("已有发布在运行，本轮跳过")
            return
        release = Release()
        try:
            if args.verify:
                health()
                rows = release.verify_fonts()
                previous = read_json(release.record)
                write_json(release.record, {**previous, "fonts": rows, "public_verified": True})
                print(json.dumps(rows, ensure_ascii=False))
            else:
                release.deploy()
        except Exception as error:
            release.update("failed", error=str(error))
            print("自动发布失败：" + str(error), file=sys.stderr)
            sys.exit(1)


if __name__ == "__main__":
    main()
