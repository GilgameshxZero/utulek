#!/usr/bin/env python3
"""Build the same pinned LMDB cache backend for Linux and native Windows."""
import argparse
from pathlib import Path
import shutil
import subprocess
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('target',choices=['linux','windows']);p.add_argument('--toolchain',type=Path)
a=p.parse_args()
source=ROOT/'.obj/deps/lmdb-0.9.37/libraries/liblmdb'
build=ROOT/('.obj/build-lmdb-'+a.target);build.mkdir(parents=True,exist_ok=True)
prefix=ROOT/('.obj/windows-prefix' if a.target=='windows' else '.obj/host-prefix')
(prefix/'include').mkdir(parents=True,exist_ok=True);(prefix/'lib').mkdir(exist_ok=True)
if a.target=='windows':
 if not a.toolchain:p.error('--toolchain is required for Windows')
 tools=a.toolchain.resolve()/'bin';cc=tools/'x86_64-w64-mingw32-clang';ar=tools/'llvm-ar'
else:cc='cc';ar='ar'
objects=[]
with (build/'build.log').open('w') as output:
 for name in ('mdb','midl'):
  obj=build/(name+'.o');objects.append(obj)
  subprocess.run(list(map(str,[cc,'-std=c11','-D_GNU_SOURCE','-O2','-c',source/(name+'.c'),'-o',obj])),stdout=output,stderr=subprocess.STDOUT,check=True)
 subprocess.run(list(map(str,[ar,'rcs',prefix/'lib/liblmdb.a',*objects])),stdout=output,stderr=subprocess.STDOUT,check=True)
shutil.copy2(source/'lmdb.h',prefix/'include/lmdb.h')
print(prefix)
