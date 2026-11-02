#!/usr/bin/env python3
"""Restore the pre-install configuration; leave account/token/mailbox data alone."""
import argparse
import json
from pathlib import Path
from deployment import restore


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check',action='store_true',help='Validate recovery without changing files')
    parser.add_argument('--force',action='store_true',help='Also replace files edited since installation')
    args=parser.parse_args()
    directory=Path(__file__).resolve().parent
    recovery=directory/'../../app/neomutt-xp-rollback'
    pointer=json.loads((recovery/'current.json').read_text(encoding='utf-8'))
    manifest=recovery/pointer['manifest_relative_to_recovery']
    changed=restore(manifest,apply=not args.check,force=args.force)
    print(('Recovery verified for ' if args.check else 'Restored previous setup; changed ')+str(len(changed))+' files.')


if __name__=='__main__':
    main()
