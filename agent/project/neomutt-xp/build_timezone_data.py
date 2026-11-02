#!/usr/bin/env python3
"""Reproduce the bundled standard-library ZoneInfo data from system TZif files."""
import argparse
from pathlib import Path
from zipfile import ZIP_DEFLATED,ZipFile,ZipInfo


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--source',type=Path,default=Path('/usr/share/zoneinfo'))
    parser.add_argument('--version',default='2026f')
    parser.add_argument('--output',type=Path,default=Path(__file__).parent/'python/timezones.zip')
    args=parser.parse_args()
    version=(args.source/'tzdata.zi').read_text().splitlines()[0]
    if version!='# version '+args.version:
        raise ValueError('Timezone data version differs from dependency pin: '+version)
    count=0
    with ZipFile(args.output,'w',compression=ZIP_DEFLATED) as archive:
        for path in sorted(args.source.rglob('*')):
            name=path.relative_to(args.source).as_posix()
            if name.split('/')[0] in ('posix','right') or name in ('localtime','posixrules') or not path.is_file():
                continue
            data=path.read_bytes()
            if not data.startswith(b'TZif'):continue
            info=ZipInfo(name,date_time=(1980,1,1,0,0,0))
            info.compress_type=ZIP_DEFLATED
            archive.writestr(info,data)
            count+=1
    print('Bundled',count,'timezone files')


if __name__=='__main__':
    main()
