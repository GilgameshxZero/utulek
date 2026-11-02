#!/usr/bin/env python3
"""Install the Windows executable and argument-forwarding batch launcher."""
import argparse
import os
from pathlib import Path
import shutil
import tempfile


def atomic_copy(source, destination):
    destination.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".khard-win-", dir=destination.parent)
    os.close(fd)
    try:
        shutil.copyfile(source, temporary)
        os.chmod(temporary, 0o755)
        os.replace(temporary, destination)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path.home() / "main.syncthing/monochrome")
    args = parser.parse_args()
    project = Path(__file__).resolve().parents[1]
    source = project / ".bin/khard.exe"
    if not source.is_file():
        parser.error("build .bin/khard.exe first")
    atomic_copy(source, args.root / "app/khard.exe")
    launcher = project / ".bin/khard.bat"
    launcher.write_bytes(b'@"%~dp0..\\app\\khard.exe" %*\r\n')
    atomic_copy(launcher, args.root / "bin/khard.bat")
    print(args.root / "app/khard.exe")
    print(args.root / "bin/khard.bat")


if __name__ == "__main__":
    main()
