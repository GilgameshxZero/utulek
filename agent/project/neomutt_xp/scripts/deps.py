"""Shared dependency, toolchain and build-stamp helpers.

The pinned versions and platform dependency lists live in dependencies.json;
this module is the only place that knows the resulting paths and library names.
"""
import hashlib
import json
import os
from pathlib import Path
import subprocess

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = json.loads((ROOT / 'dependencies.json').read_text())
STAMPS = ROOT / '.obj/stamps'
CHUNK = 1024 * 1024

WINDOWS_LIBRARIES = ('ncursesw', 'iconv', 'pcre2-posix', 'pcre2-8', 'ssl',
                     'crypto', 'lmdb', 'gpgme', 'assuan', 'gpg-error', 'sqlite3')
HOST_LIBRARIES = ('lmdb', 'gpgme', 'assuan', 'gpg-error', 'sqlite3')


def source(name):
    item = MANIFEST[name]
    if name == 'neomutt':
        return ROOT / '.obj/upstream'
    return ROOT / '.obj/deps' / f'{name}-{item["version"]}'


def prefix(target):
    return ROOT / ('.obj/windows-prefix' if target == 'windows' else '.obj/host-prefix')


def libraries(target):
    return WINDOWS_LIBRARIES if target == 'windows' else HOST_LIBRARIES


def sha256_file(path):
    digest = hashlib.sha256()
    with open(path, 'rb') as stream:
        while block := stream.read(CHUNK):
            digest.update(block)
    return digest.hexdigest()


def verify_archive(name, archive):
    """Verify one downloaded archive, remembering the result by file identity."""
    cache_path = ROOT / '.obj/downloads/sha256.json'
    cache = json.loads(cache_path.read_text()) if cache_path.exists() else {}
    key = str(archive.relative_to(ROOT))
    stat = archive.stat()
    entry = cache.get(key)
    if entry and entry['size'] == stat.st_size and entry['mtime_ns'] == stat.st_mtime_ns:
        actual = entry['sha256']
    else:
        actual = sha256_file(archive)
        cache[key] = {'size': stat.st_size, 'mtime_ns': stat.st_mtime_ns,
                      'sha256': actual}
        cache_path.parent.mkdir(parents=True, exist_ok=True)
        cache_path.write_text(json.dumps(cache, indent=2, sort_keys=True) + '\n')
    expected = MANIFEST[name]['sha256']
    if actual != expected:
        raise SystemExit(f'{archive}: SHA256 mismatch: {actual}')
    return actual


def tree_fingerprint(path):
    """Cheap identity of a source tree: changing any file changes the stamp."""
    latest, total, count = 0, 0, 0
    for at in Path(path).rglob('*'):
        if at.is_file():
            stat = at.stat()
            latest = max(latest, stat.st_mtime_ns)
            total += stat.st_size
            count += 1
    return {'files': count, 'bytes': total, 'mtime_ns': latest}


def source_identity(name):
    """Identity of a prepared dependency: pinned archive plus applied patches.

    Generated files inside an in-tree configure/make run do not perturb this,
    unlike a tree fingerprint.
    """
    stamp = ROOT / '.obj/source-stamps' / (name + '.json')
    patches = json.loads(stamp.read_text()) if stamp.exists() else {}
    return {'archive': MANIFEST[name]['sha256'], 'patches': patches}


def jobs(requested):
    return requested if requested else (os.cpu_count() or 4)


def run(command, cwd, log, env=None):
    cwd = Path(cwd)
    cwd.mkdir(parents=True, exist_ok=True)
    with (cwd / log).open('w') as output:
        subprocess.run([str(item) for item in command], cwd=cwd, env=env,
                       stdout=output, stderr=subprocess.STDOUT, check=True)


def stamp_matches(name, payload):
    path = STAMPS / name
    if not path.exists():
        return False
    try:
        return json.loads(path.read_text()) == payload
    except ValueError:
        return False


def write_stamp(name, payload):
    STAMPS.mkdir(parents=True, exist_ok=True)
    (STAMPS / name).write_text(json.dumps(payload, sort_keys=True, indent=2) + '\n')


class Toolchain:
    """The pinned LLVM-MinGW cross toolchain."""

    def __init__(self, path):
        self.path = Path(path).resolve()
        self.bin = self.path / 'bin'

    def tool(self, name):
        return self.bin / name

    @property
    def clang(self):
        return self.tool('x86_64-w64-mingw32-clang')

    def env(self, **extra):
        environment = dict(os.environ,
                           CC=str(self.clang),
                           AR=str(self.tool('llvm-ar')),
                           RANLIB=str(self.tool('llvm-ranlib')),
                           STRIP=str(self.tool('llvm-strip')),
                           WINDRES=str(self.tool('x86_64-w64-mingw32-windres')),
                           RC=str(self.tool('x86_64-w64-mingw32-windres')),
                           DLLTOOL=str(self.tool('llvm-dlltool')))
        environment.update({key: str(value) for key, value in extra.items()})
        return environment
