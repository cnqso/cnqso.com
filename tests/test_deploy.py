"""Exercise the real deploy script with disposable Git repos and fake host services."""
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SOURCE = Path(__file__).resolve().parents[1]
REAL_GIT = shutil.which("git")


class DeploymentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cnqso-deploy-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        for name in ("origin", "sub"):
            repo = self.root / name
            repo.mkdir()
            self.git(repo, "init", "-b", "main")
            self.git(repo, "config", "user.name", "Deployment test")
            self.git(repo, "config", "user.email", "test@example.invalid")
        self.sub = self.root / "sub"
        (self.sub / "file").write_text("one")
        self.git(self.sub, "add", ".")
        self.git(self.sub, "commit", "-m", "one")
        self.origin = self.root / "origin"
        (self.origin / "scripts").mkdir()
        shutil.copy(SOURCE / "scripts/compose.sh", self.origin / "scripts/compose.sh")
        self.git(self.origin, "submodule", "add", str(self.sub), "child")
        self.git(self.origin, "add", ".")
        self.git(self.origin, "commit", "-m", "one")
        self.git(self.root, "clone", "--recurse-submodules", str(self.origin), "checkout")
        self.checkout = self.root / "checkout"
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.state = self.root / "state"
        self.state.mkdir()
        self.trace = self.root / "trace"
        self.env = {**os.environ,
                    "PATH": str(self.bin) + ":" + os.environ["PATH"],
                    "CNQSO_REPO_DIR": str(self.checkout),
                    "CNQSO_DEPLOY_USER": "root",
                    "CNQSO_DEPLOY_STATE_DIR": str(self.state),
                    "TEST_ROOT": str(self.root),
                    "GIT_ALLOW_PROTOCOL": "file"}
        self.script("git", '#!/usr/bin/env bash\n'
                    'if [[ "$*" == *"submodule update"* && -e "$TEST_ROOT/fail-submodule" ]]; then exit 1; fi\n'
                    'if [[ "$*" == *"fetch --quiet"* && -e "$TEST_ROOT/fail-fetch" ]]; then exit 1; fi\n'
                    'printf "git %s\\n" "$*" >> "$TEST_ROOT/trace"\n'
                    f'exec "{REAL_GIT}" "$@"\n')
        self.script("flock", "#!/bin/sh\nexit 0\n")
        self.script("curl", '#!/bin/sh\ntest ! -e "$TEST_ROOT/unhealthy"\n')
        self.script("systemctl", '#!/bin/sh\nprintf "systemctl %s\\n" "$*" >> "$TEST_ROOT/trace"\ntest ! -e "$TEST_ROOT/fail-build"\n')
        self.script("docker", '#!/bin/sh\nprintf "docker %s\\n" "$*" >> "$TEST_ROOT/trace"\nrm -f "$TEST_ROOT/unhealthy"\n')

    def script(self, name, body):
        p = self.bin / name
        p.write_text(body)
        p.chmod(0o755)

    def git(self, repo, *args):
        return subprocess.run([REAL_GIT, "-c", "protocol.file.allow=always", *args],
                              cwd=repo, text=True, capture_output=True, check=True).stdout.strip()

    def deploy(self):
        return subprocess.run(["bash", str(SOURCE / "scripts/deploy.sh")],
                              env=self.env, text=True, capture_output=True)

    def advance(self):
        (self.sub / "file").write_text("two")
        self.git(self.sub, "commit", "-am", "two")
        self.git(self.origin / "child", "pull")
        self.git(self.origin, "commit", "-am", "two")

    def test_failed_submodule_update_recovers_on_next_poll(self):
        self.advance()
        (self.root / "fail-submodule").touch()
        self.assertNotEqual(self.deploy().returncode, 0)
        (self.root / "fail-submodule").unlink()
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.git(self.checkout / "child", "rev-parse", "HEAD"),
                         self.git(self.sub, "rev-parse", "HEAD"))
        self.assertEqual((self.state / "deployed-revision").read_text().strip(),
                         self.git(self.checkout, "rev-parse", "HEAD"))

    def test_initial_install_initializes_submodules_at_matching_head(self):
        self.git(self.checkout, "submodule", "deinit", "-f", "--all")
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.checkout / "child/file").is_file())

    def test_local_submodule_edits_are_preserved(self):
        (self.checkout / "child/file").write_text("my edits")
        result = self.deploy()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("local changes in submodule", result.stderr)
        self.assertEqual((self.checkout / "child/file").read_text(), "my edits")

    def test_staged_submodule_pin_is_preserved(self):
        self.advance()
        self.git(self.checkout / "child", "fetch")
        self.git(self.checkout / "child", "checkout", self.git(self.sub, "rev-parse", "HEAD"))
        self.git(self.checkout, "add", "child")
        before = self.git(self.checkout, "diff", "--cached")
        self.assertNotEqual(self.deploy().returncode, 0)
        self.assertEqual(before, self.git(self.checkout, "diff", "--cached"))

    def test_root_edits_are_preserved(self):
        (self.checkout / "scripts/compose.sh").write_text("my edits")
        self.assertNotEqual(self.deploy().returncode, 0)
        self.assertEqual((self.checkout / "scripts/compose.sh").read_text(), "my edits")

    def test_diverged_branch_is_preserved(self):
        self.git(self.checkout, "config", "user.name", "Deployment test")
        self.git(self.checkout, "config", "user.email", "test@example.invalid")
        (self.checkout / "local").write_text("local commit")
        self.git(self.checkout, "add", ".")
        self.git(self.checkout, "commit", "-m", "local")
        before = self.git(self.checkout, "rev-parse", "HEAD")
        self.advance()
        self.assertNotEqual(self.deploy().returncode, 0)
        self.assertEqual(before, self.git(self.checkout, "rev-parse", "HEAD"))

    def test_failed_build_is_not_marked_deployed(self):
        (self.root / "fail-build").touch()
        self.assertNotEqual(self.deploy().returncode, 0)
        self.assertFalse((self.state / "deployed-revision").exists())

    def test_healthy_deployed_revision_does_not_restart(self):
        (self.state / "deployed-revision").write_text(self.git(self.checkout, "rev-parse", "HEAD"))
        result = self.deploy()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn("systemctl", self.trace.read_text())
        self.assertNotIn("docker", self.trace.read_text())

    def test_unhealthy_container_recovers_even_when_fetch_fails(self):
        (self.state / "deployed-revision").write_text(self.git(self.checkout, "rev-parse", "HEAD"))
        (self.root / "unhealthy").touch()
        (self.root / "fail-fetch").touch()
        self.assertNotEqual(self.deploy().returncode, 0)
        self.assertFalse((self.root / "unhealthy").exists())
        self.assertIn("docker compose up -d --no-build --force-recreate web", self.trace.read_text())


if __name__ == "__main__":
    unittest.main()
