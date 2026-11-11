#!/usr/bin/env python3
"""Build the pinned static OpenSSL libraries for native Windows."""
import argparse
import os
from pathlib import Path
import subprocess
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser()
p.add_argument('--toolchain',type=Path,required=True)
a=p.parse_args()
tools=a.toolchain.resolve()/'bin'
source=ROOT/'.obj/deps/openssl-3.5.9'
build=ROOT/'.obj/build-openssl';build.mkdir(parents=True,exist_ok=True)
prefix=ROOT/'.obj/windows-prefix'
env=dict(os.environ,CC=str(tools/'x86_64-w64-mingw32-clang'),AR=str(tools/'llvm-ar'),RANLIB=str(tools/'llvm-ranlib'),RC=str(tools/'x86_64-w64-mingw32-windres'))
for label,command in [
 ('configure',['perl',source/'Configure','mingw64','no-shared','no-tests','no-module','no-apps','no-docs','--prefix='+str(prefix),'--libdir=lib']),
 ('build',['make','-j4','build_libs']),
 ('install',['make','install_dev'])]:
 with (build/(label+'.log')).open('w') as output:
  subprocess.run(list(map(str,command)),cwd=build,env=env,stdout=output,stderr=subprocess.STDOUT,check=True)
print(prefix)
