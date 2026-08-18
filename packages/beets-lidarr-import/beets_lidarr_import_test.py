# Tests for beets-lidarr-import.
#
# The API is faked at urllib level rather than by stubbing the module's own
# call(): the URL, the query string and the headers are as much a part of
# getting this right as the JSON is, and a stub above them would not check
# any of it.
import io
import json
import unittest
import urllib.error
from unittest import mock

import beets_lidarr_import as blimp


def item(path="/rips/A/Album/01 Track.flac", **overrides):
    """One entry of Lidarr's manualimport listing, fully resolved."""
    base = {
        "path": path,
        "artist": {"id": 34, "artistName": "MEOVV"},
        "album": {"id": 408, "title": "BITE NOW"},
        "albumReleaseId": 1589,
        "tracks": [{"id": 10808}],
        "quality": {"quality": {"id": 6, "name": "FLAC"}},
        "rejections": [],
    }
    base.update(overrides)
    return base


class FakeResponse(io.BytesIO):
    """Enough of an HTTP response for urlopen's context manager."""

    def __enter__(self):
        return self

    def __exit__(self, *_):
        self.close()
        return False


class FakeLidarr:
    """Answers calls in order, recording every request made."""

    def __init__(self, *replies):
        self.replies = list(replies)
        self.requests = []

    def __call__(self, request, timeout=None):
        self.requests.append(request)
        if not self.replies:
            raise AssertionError(f"unexpected call to {request.full_url}")

        reply = self.replies.pop(0)
        if isinstance(reply, Exception):
            raise reply
        return FakeResponse(json.dumps(reply).encode())

    def body(self, index):
        return json.loads(self.requests[index].data)


ENV = {
    "LIDARR_URL": "http://lidarr:8686/",
    "LIDARR_API_KEY": "secret",
}


def run(lidarr, argv=("prog", "/rips/A/Album"), env=None, **kwargs):
    """Run the importer against a fake Lidarr."""
    out = io.StringIO()
    with mock.patch.dict("os.environ", env or ENV, clear=True):
        with mock.patch("urllib.request.urlopen", lidarr):
            result = blimp.run(list(argv), out, **kwargs)
    return result, out.getvalue()


class TestPlaced(unittest.TestCase):
    def test_fully_resolved(self):
        self.assertTrue(blimp.placed(item()))

    def test_missing_pieces(self):
        cases = {
            "no artist": {"artist": {}},
            "no album": {"album": {}},
            "no release": {"albumReleaseId": None},
            "no tracks": {"tracks": []},
            "artist absent": {"artist": None},
        }
        for name, override in cases.items():
            with self.subTest(name):
                self.assertFalse(blimp.placed(item(**override)))


class TestIdentify(unittest.TestCase):
    def test_asks_for_the_folder(self):
        lidarr = FakeLidarr([item()])
        with mock.patch("urllib.request.urlopen", lidarr):
            blimp.identify("http://lidarr:8686/api/v1", "k", "/rips/A/B")

        request = lidarr.requests[0]
        self.assertEqual(request.get_method(), "GET")
        self.assertIn("folder=%2Frips%2FA%2FB", request.full_url)
        self.assertIn("filterExistingFiles=false", request.full_url)
        self.assertEqual(request.headers["X-api-key"], "k")

    def test_empty_listing_is_a_failure(self):
        # Nothing to import means the hook has to say so; carrying on would
        # report success for an album that never reached the library.
        lidarr = FakeLidarr([])
        with mock.patch("urllib.request.urlopen", lidarr):
            with self.assertRaises(blimp.Failure) as caught:
                blimp.identify("http://l/api/v1", "k", "/rips/A/B")
        self.assertIn("no importable files", str(caught.exception))

    def test_unreachable_lidarr(self):
        lidarr = FakeLidarr(urllib.error.URLError("refused"))
        with mock.patch("urllib.request.urlopen", lidarr):
            with self.assertRaises(blimp.Failure) as caught:
                blimp.identify("http://l/api/v1", "k", "/rips/A/B")
        self.assertIn("could not reach Lidarr", str(caught.exception))


class TestImportPayload(unittest.TestCase):
    def test_carries_every_id_lidarr_resolved(self):
        payload = blimp.import_payload([item()], "Copy")

        self.assertEqual(payload["name"], "ManualImport")
        self.assertEqual(payload["importMode"], "Copy")
        self.assertFalse(payload["replaceExistingFiles"])

        (file,) = payload["files"]
        self.assertEqual(file["artistId"], 34)
        self.assertEqual(file["albumId"], 408)
        self.assertEqual(file["albumReleaseId"], 1589)
        self.assertEqual(file["trackIds"], [10808])
        self.assertEqual(file["quality"]["quality"]["name"], "FLAC")

    def test_leaves_release_switching_on(self):
        # Turning it off would pin the album to whichever release Lidarr
        # happens to hold, which is the thing this script exists to avoid.
        (file,) = blimp.import_payload([item()], "Copy")["files"]
        self.assertFalse(file["disableReleaseSwitching"])

    def test_one_entry_per_file(self):
        items = [item("/a/01.flac"), item("/a/02.flac")]
        payload = blimp.import_payload(items, "Move")

        self.assertEqual(payload["importMode"], "Move")
        self.assertEqual(
            [f["path"] for f in payload["files"]],
            ["/a/01.flac", "/a/02.flac"],
        )


class TestRun(unittest.TestCase):
    def test_imports_what_lidarr_identified(self):
        lidarr = FakeLidarr(
            [item("/a/01.flac"), item("/a/02.flac")],
            {"id": 165885},
            {"status": "completed", "result": "successful"},
        )
        result, out = run(lidarr)

        self.assertIn("imported", result)
        self.assertIn("queued command 165885", out)
        self.assertIn("2 files", out)

        posted = lidarr.body(1)
        self.assertEqual(posted["name"], "ManualImport")
        self.assertEqual(len(posted["files"]), 2)
        self.assertEqual(
            lidarr.requests[1].full_url,
            "http://lidarr:8686/api/v1/command",
        )

    def test_never_scans(self):
        # DownloadedAlbumsScan is the path that applies the 80% threshold and
        # rejects a correct rip whose release Lidarr's mirror lacks. Reaching
        # for it again would undo the whole point of this script.
        lidarr = FakeLidarr(
            [item()],
            {"id": 1},
            {"status": "completed", "result": "successful"},
        )
        run(lidarr)

        for request in lidarr.requests:
            if request.data:
                self.assertNotIn("DownloadedAlbumsScan", request.data.decode())

    def test_refuses_a_partly_placed_album(self):
        # Importing the resolved files alone would leave the album silently
        # short, which is worse than not importing it.
        lidarr = FakeLidarr(
            [
                item("/a/01.flac"),
                item(
                    "/a/02.flac",
                    tracks=[],
                    rejections=[
                        {"reason": "Unknown album"},
                    ],
                ),
            ]
        )
        with self.assertRaises(blimp.Failure) as caught:
            run(lidarr)

        message = str(caught.exception)
        self.assertIn("could not place 1 of 2", message)
        self.assertIn("02.flac: Unknown album", message)
        # Nothing was sent: the files stay where beets put them.
        self.assertEqual(len(lidarr.requests), 1)

    def test_reports_an_unsuccessful_import(self):
        lidarr = FakeLidarr(
            [item()],
            {"id": 7},
            {"status": "completed", "result": "unsuccessful"},
        )
        with self.assertRaises(blimp.Failure) as caught:
            run(lidarr)
        self.assertIn("imported nothing", str(caught.exception))

    def test_reports_a_failed_command(self):
        lidarr = FakeLidarr(
            [item()],
            {"id": 7},
            {"status": "failed", "exception": "disk full"},
        )
        with self.assertRaises(blimp.Failure) as caught:
            run(lidarr)
        self.assertIn("disk full", str(caught.exception))

    def test_rejected_request(self):
        lidarr = FakeLidarr([item()], {})
        with self.assertRaises(blimp.Failure) as caught:
            run(lidarr)
        self.assertIn("rejected the request", str(caught.exception))

    def test_polls_until_the_command_finishes(self):
        lidarr = FakeLidarr(
            [item()],
            {"id": 9},
            {"status": "queued"},
            {"status": "started"},
            {"status": "completed", "result": "successful"},
        )
        slept = []
        result, _ = run(lidarr, sleep=slept.append)

        self.assertIn("imported", result)
        self.assertEqual(slept, [2.0, 2.0])

    def test_gives_up_eventually(self):
        clock = iter([0.0, 1.0, 99.0])
        lidarr = FakeLidarr(
            [item()],
            {"id": 9},
            {"status": "started"},
        )
        with self.assertRaises(blimp.Failure) as caught:
            run(
                lidarr,
                env=dict(ENV, LIDARR_IMPORT_TIMEOUT="5"),
                now=lambda: next(clock),
                sleep=lambda _: None,
            )
        self.assertIn("gave up waiting 5s", str(caught.exception))

    def test_honours_the_import_mode(self):
        lidarr = FakeLidarr(
            [item()],
            {"id": 1},
            {"status": "completed", "result": "successful"},
        )
        run(lidarr, env=dict(ENV, LIDARR_IMPORT_MODE="Move"))
        self.assertEqual(lidarr.body(1)["importMode"], "Move")

    def test_defaults_to_copy(self):
        # Copy plus Lidarr's copyUsingHardlinks keeps the rip as its own tree
        # at no extra disk cost; Move would take it away.
        lidarr = FakeLidarr(
            [item()],
            {"id": 1},
            {"status": "completed", "result": "successful"},
        )
        run(lidarr)
        self.assertEqual(lidarr.body(1)["importMode"], "Copy")

    def test_requires_an_album(self):
        with self.assertRaises(blimp.Failure) as caught:
            run(FakeLidarr(), argv=("prog",))
        self.assertIn("usage", str(caught.exception))

    def test_requires_the_environment(self):
        for missing in ("LIDARR_URL", "LIDARR_API_KEY"):
            with self.subTest(missing):
                env = {k: v for k, v in ENV.items() if k != missing}
                with self.assertRaises(blimp.Failure) as caught:
                    run(FakeLidarr(), env=env)
                self.assertIn(missing, str(caught.exception))

    def test_rejects_a_nonsense_timeout(self):
        with self.assertRaises(blimp.Failure) as caught:
            run(FakeLidarr(), env=dict(ENV, LIDARR_IMPORT_TIMEOUT="soon"))
        self.assertIn("not a number", str(caught.exception))

    def test_trailing_slash_in_the_url(self):
        lidarr = FakeLidarr(
            [item()],
            {"id": 1},
            {"status": "completed", "result": "successful"},
        )
        run(lidarr, env=dict(ENV, LIDARR_URL="http://lidarr:8686///"))
        self.assertTrue(
            lidarr.requests[0].full_url.startswith(
                "http://lidarr:8686/api/v1/manualimport?"
            ),
            lidarr.requests[0].full_url,
        )


class TestMain(unittest.TestCase):
    def test_reports_failure_on_stderr(self):
        stderr = io.StringIO()
        with mock.patch.dict("os.environ", {}, clear=True):
            with mock.patch("sys.stderr", stderr):
                code = blimp.main(["prog", "/rips/A/B"])

        self.assertEqual(code, 1)
        self.assertIn("LIDARR_URL is not set", stderr.getvalue())


if __name__ == "__main__":
    unittest.main()
