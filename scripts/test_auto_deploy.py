"""文件作用：验证自动部署的应用输入判断、实际线上版本门禁和字体响应校验，测试不连接生产。"""
import importlib.util
import json
from pathlib import Path
import tempfile
import subprocess
import sys
import time
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("auto_deploy", Path(__file__).with_name("auto_deploy.py"))
deploy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deploy)


class AutoDeployTests(unittest.TestCase):
    """以临时目录和命令边界替身验证发布决策，保留真实生产逻辑。"""

    def subject(self, root, coherent):
        """创建有上一版上线记录的最小场景，明确区分源码HEAD和实际镜像。"""
        source, state = Path(root) / "source", Path(root) / "state"
        source.mkdir(); state.mkdir()
        release = deploy.Release(source, state)
        values = {("rev-parse", "HEAD"): "a" * 40, ("rev-parse", "origin/main"): "a" * 40,
                  ("branch", "--list", "main"): "main", ("status", "--porcelain", "--untracked-files=no"): ""}
        release.git = lambda *args: "tree" if args[0] == "ls-tree" else values.get(args, "")
        digest = deploy.input_digest("tree")
        old = state / "release" / "h5"; old.mkdir(parents=True)
        deploy.write_json(old / "deployment-version.json", {"source_commit": "a" * 40, "input_digest": digest})
        (state / "current").symlink_to(old.parent)
        release.running = lambda: {"running": True, "input": digest if coherent else "old", "commit": "a" * 40, "image_id": "image"}
        return release

    def test_same_running_application_skips_build(self):
        """实际两端均匹配时不运行npm或Docker构建，状态仍记录最近检查。"""
        with tempfile.TemporaryDirectory() as root, patch.object(deploy, "health"):
            release = self.subject(root, True)
            release.run = lambda *args, **kwargs: self.fail("无更新时不应构建")
            release.deploy()
            self.assertEqual(deploy.read_json(release.record)["status"], "up_to_date")

    def test_updated_git_does_not_skip_failed_release(self):
        """上次Git已更新但镜像未上线时仍进入构建，不能永久跳过失败版本。"""
        with tempfile.TemporaryDirectory() as root, patch.object(deploy, "health"):
            release = self.subject(root, False)
            calls = []
            def run(args, **kwargs):
                """磁盘检查后在npm边界停止，不运行任何真实安装或容器操作。"""
                calls.append(args)
                if args[0] == "docker":
                    return root
                raise RuntimeError("reached npm build")
            release.run = run
            with self.assertRaisesRegex(RuntimeError, "reached npm build"):
                release.deploy()
            self.assertEqual(calls[-1][:3], ["npm", "--prefix", "frontend"])

    def test_font_content_and_headers(self):
        """字体不能是HTML占位页、完整大字库或缺少跨域头的二进制。"""
        with tempfile.TemporaryDirectory() as root:
            source = Path(root); folder = source / "backend/internal/artfonts"; folder.mkdir(parents=True)
            deploy.write_json(folder / "catalog.json", {"version": "1", "fonts": [{"id": "longcang"}]})
            release = deploy.Release(source, source)
            good = {"Content-Type": "font/woff", "Access-Control-Allow-Origin": "*", "Cache-Control": "public, immutable"}
            for headers, raw in [(good, b"<html>"), ({}, b"wOFF" + b"0" * 80), (good, b"wOFF" + b"0" * 131072)]:
                with patch.object(deploy, "request", return_value=(headers, raw)), self.assertRaises(RuntimeError):
                    release.verify_fonts()
            with patch.object(deploy, "request", return_value=(good, b"wOFF" + b"0" * 80)):
                self.assertEqual(release.verify_fonts(), [{"font": "longcang", "bytes": 84}])

    def test_cleanup_only_rebuildable_files(self):
        """清理仅删除发布副本的依赖和临时构建，不删除线上回滚目录或配置。"""
        with tempfile.TemporaryDirectory() as root:
            release = self.subject(root, True)
            for name in ("frontend/node_modules", "frontend/dist", "deploy"):
                path = release.source / name
                path.mkdir(parents=True)
                (path / "keep-test").write_text("test")
            release.clean_build_files()
            self.assertFalse((release.source / "frontend/node_modules").exists())
            self.assertFalse((release.source / "frontend/dist").exists())
            self.assertTrue((release.source / "deploy/keep-test").is_file())
            self.assertTrue((release.root / "current/h5/deployment-version.json").is_file())

    def test_timeout_ends_download_descendants(self):
        """超时必须停止整个下载进程组，避免孙进程继续持有发布锁。"""
        with tempfile.TemporaryDirectory() as root:
            release = self.subject(root, True)
            delayed = Path(root) / "orphan-wrote"
            child = "import time;from pathlib import Path;time.sleep(.5);Path(" + repr(str(delayed)) + ").write_text('bad')"
            parent = "import subprocess,sys,time;subprocess.Popen([sys.executable,'-c'," + repr(child) + "]);time.sleep(30)"
            with self.assertRaises(subprocess.TimeoutExpired):
                release.run([sys.executable, "-c", parent], timeout=.1, capture=True)
            time.sleep(.6)
            self.assertFalse(delayed.exists())


if __name__ == "__main__":
    unittest.main()
