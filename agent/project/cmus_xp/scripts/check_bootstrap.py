#!/usr/bin/env python3
"""Offline regression checks of dependency verification and safe extraction."""
import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
import build_windows_release as build

class BootstrapChecks(unittest.TestCase):
    def setUp(self):
        (build.ROOT / '.obj').mkdir(exist_ok=True)
        self.temp = tempfile.TemporaryDirectory(dir=build.ROOT / '.obj')
        self.previous = build.DEPS
        build.DEPS = Path(self.temp.name)

    def tearDown(self):
        build.DEPS = self.previous
        self.temp.cleanup()

    def archive(self, name):
        path = build.DEPS / 'fixture.tar.gz'
        with tarfile.open(path, 'w:gz') as target:
            item = tarfile.TarInfo(name); data = b'fixture'; item.size = len(data)
            target.addfile(item, io.BytesIO(data))
        return {'archive':path.name, 'version':'test', 'url':'https://invalid.example/never-used',
                'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}

    def test_verified_cached_archive(self):
        spec = self.archive('root/source.c')
        build.prepare('dependency', spec, True)
        self.assertEqual((build.DEPS / 'dependency/source.c').read_bytes(), b'fixture')
        build.prepare('dependency', spec, True)

    def test_corrupt_cache_is_rejected(self):
        spec = self.archive('root/source.c')
        (build.DEPS / spec['archive']).write_bytes(b'corrupt')
        with self.assertRaisesRegex(RuntimeError, 'SHA-256 mismatch'):
            build.prepare('dependency', spec, True)
        self.assertFalse((build.DEPS / 'dependency').exists())

    def test_archive_cannot_escape(self):
        spec = self.archive('root/../../escape')
        with self.assertRaises(tarfile.FilterError):
            build.prepare('dependency', spec, True)
        self.assertFalse((build.DEPS / 'escape').exists())

    def test_offline_missing_cache(self):
        with self.assertRaisesRegex(RuntimeError, 'Offline cache missing'):
            build.prepare('dependency', {'archive':'missing.tar.gz'}, True)

if __name__ == '__main__':
    unittest.main()
