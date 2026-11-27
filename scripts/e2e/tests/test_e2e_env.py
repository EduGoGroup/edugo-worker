"""Tests of the guard, of `--out-dir` and of the exit-code rule. No network, no Docker."""
import datetime
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import e2e_env  # noqa: E402 - needs the path above

LOCAL_URLS = {"EDUGO_IDENTITY_URL": "http://localhost:8070",
              "EDUGO_LEARNING_URL": "http://127.0.0.1:8065",
              "EDUGO_ACADEMIC_URL": "http://[::1]:8060"}
ALL_CONNECTED = {name: True for name in LOCAL_URLS}


def summary(needs_model, failed=(), error=None, seeded=True, case="X"):
    return {"case": case, "needs_model": needs_model, "failed": list(failed), "error": error,
            "seeded": seeded}


class GuardTest(unittest.TestCase):
    def test_guard_rejects_non_loopback_url(self):
        urls = dict(LOCAL_URLS, EDUGO_LEARNING_URL="http://example.invalid:8065")
        problems = e2e_env.guard_problems(urls, "local\n", ALL_CONNECTED)
        self.assertEqual(len(problems), 1)
        self.assertIn("EDUGO_LEARNING_URL", problems[0])
        # A host that merely starts like a loopback one is not loopback.
        self.assertFalse(e2e_env.is_loopback("http://localhost.example.com:8065"))
        self.assertFalse(e2e_env.is_loopback("https://ep-x.neon.tech"))

    def test_guard_rejects_missing_mode_file(self):
        problems = e2e_env.guard_problems(LOCAL_URLS, None, ALL_CONNECTED)
        self.assertEqual(len(problems), 1)
        self.assertIn("does not exist", problems[0])
        with tempfile.TemporaryDirectory() as tmp:
            self.assertIsNone(e2e_env.read_mode(os.path.join(tmp, "edugo-mode")))

    def test_guard_rejects_remote_mode(self):
        problems = e2e_env.guard_problems(LOCAL_URLS, "remote\n", ALL_CONNECTED)
        self.assertEqual(len(problems), 1)
        self.assertIn("'remote'", problems[0])

    def test_guard_rejects_api_without_local_db_connection(self):
        facts = dict(ALL_CONNECTED, EDUGO_LEARNING_URL=False)
        problems = e2e_env.guard_problems(LOCAL_URLS, "local", facts)
        self.assertEqual(len(problems), 1)
        self.assertIn("EDUGO_LEARNING_URL", problems[0])
        # Fails closed: a process that could not be inspected is also a problem.
        unknown = dict(ALL_CONNECTED, EDUGO_IDENTITY_URL=None)
        self.assertEqual(len(e2e_env.guard_problems(LOCAL_URLS, "local", unknown)), 1)

    def test_guard_accepts_local_environment(self):
        self.assertEqual(e2e_env.guard_problems(LOCAL_URLS, "local\n", ALL_CONNECTED), [])

    def test_enforce_guard_makes_no_request_for_a_non_loopback_url(self):
        calls = []
        original = e2e_env.process_has_local_db_connection
        e2e_env.process_has_local_db_connection = lambda url: calls.append(url) or True
        try:
            urls = dict(LOCAL_URLS, EDUGO_LEARNING_URL="http://example.invalid:8065")
            with self.assertRaises(e2e_env.GuardError):
                e2e_env.enforce_guard(urls, urls)
        finally:
            e2e_env.process_has_local_db_connection = original
        self.assertEqual(calls, [])


class OutDirTest(unittest.TestCase):
    def test_out_dir_required_and_never_inside_evidence(self):
        with self.assertRaises(e2e_env.UsageError):
            e2e_env.check_out_dir(None)
        with self.assertRaises(e2e_env.UsageError):
            e2e_env.check_out_dir("")
        with tempfile.TemporaryDirectory() as tmp:
            evidence = os.path.join(tmp, "documentacion", "analisis", "inferencia-e2e")
            for inside in (evidence, os.path.join(evidence, "resultados"),
                           os.path.join(evidence, "resultados", "nueva")):
                with self.assertRaises(e2e_env.UsageError):
                    e2e_env.check_out_dir(inside)
            self.assertFalse(os.path.exists(evidence))  # validating creates nothing
            outside = os.path.join(tmp, "corridas")
            self.assertEqual(e2e_env.check_out_dir(outside), os.path.abspath(outside))

    def test_out_dir_is_one_folder_per_run(self):
        now = datetime.datetime(2026, 11, 25, 12, 0, 0, tzinfo=datetime.timezone.utc)
        with tempfile.TemporaryDirectory() as tmp:
            first = e2e_env.new_run_dir(tmp, now)
            with open(os.path.join(first, "R5.json"), "w", encoding="utf-8") as fh:
                fh.write("{}")
            second = e2e_env.new_run_dir(tmp, now)  # same second: still another folder
            self.assertNotEqual(first, second)
            self.assertEqual(os.path.basename(first), "20261125T120000Z")
            self.assertEqual(os.listdir(second), [])
            self.assertEqual(os.listdir(first), ["R5.json"])


class ExitCodeTest(unittest.TestCase):
    def test_exit_code_model_case_failed_assertion_is_zero(self):
        failed = summary(True, failed=["puntos"])
        timed_out = summary(True, error="EnvError: timeout (300 s) waiting for: x")
        self.assertEqual(e2e_env.exit_code([failed, timed_out]), e2e_env.EXIT_OK)
        self.assertIn("2", e2e_env.unmet_line([failed, timed_out]))

    def test_exit_code_no_model_case_failed_assertion_is_one(self):
        self.assertEqual(e2e_env.exit_code([summary(False, failed=["candado"])]),
                         e2e_env.EXIT_FAILED)
        self.assertEqual(e2e_env.exit_code([summary(False, error="EnvError: timeout")]),
                         e2e_env.EXIT_FAILED)
        self.assertEqual(e2e_env.exit_code([summary(False), summary(True)]), e2e_env.EXIT_OK)

    def test_exit_code_environment_error_is_one(self):
        # A case with model that broke before seeding anything could not run: 1.
        broken = summary(True, error="ApiError: POST auth/login -> 502", seeded=False)
        self.assertEqual(e2e_env.exit_code([summary(True), broken]), e2e_env.EXIT_FAILED)
        self.assertEqual(e2e_env.unmet_line([broken]), "casos con modelo no cumplidos: 0")

    def test_missing_external_path_names_its_variable(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(e2e_env.MissingPathError) as ctx:
                e2e_env.require_external_paths(tmp)  # no Makefile
            self.assertIn(e2e_env.EDUBACK_VAR, str(ctx.exception))
            self.assertIn("Makefile", str(ctx.exception))
            open(os.path.join(tmp, "Makefile"), "w", encoding="utf-8").close()
            with self.assertRaises(e2e_env.MissingPathError) as ctx:
                e2e_env.require_external_paths(tmp)  # no .env.local
            self.assertIn(e2e_env.EDUBACK_VAR, str(ctx.exception))
            self.assertIn(".env.local", str(ctx.exception))


if __name__ == "__main__":
    unittest.main()
