#!/usr/bin/env python3
"""Build pinned GPGME/Autocrypt dependencies for Linux and native Windows."""
import argparse,json,os,shutil,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
p=argparse.ArgumentParser();p.add_argument('target',choices=['linux','windows']);p.add_argument('--toolchain',type=Path);a=p.parse_args()
manifest=json.loads((ROOT/'dependencies.json').read_text());windows=a.target=='windows'
prefix=ROOT/('.obj/windows-prefix' if windows else '.obj/host-prefix');prefix.mkdir(parents=True,exist_ok=True)
if windows:
 if not a.toolchain:p.error('--toolchain is required')
 tool=a.toolchain.resolve()/'bin';cc=str(tool/'x86_64-w64-mingw32-clang');ar=str(tool/'llvm-ar');ranlib=str(tool/'llvm-ranlib')
else:cc='cc';ar='ar';ranlib='ranlib'
env=dict(os.environ,CC=cc,AR=ar,RANLIB=ranlib,CC_FOR_BUILD='cc',CFLAGS='-O2',CPPFLAGS='-I'+str(prefix/'include'),LDFLAGS='-L'+str(prefix/'lib'),PKG_CONFIG_LIBDIR=str(prefix/'lib/pkgconfig'))
if windows:env.update(RC=str(tool/'x86_64-w64-mingw32-windres'),WINDRES=str(tool/'x86_64-w64-mingw32-windres'),DLLTOOL=str(tool/'llvm-dlltool'),STRIP=str(tool/'llvm-strip'))
for name in ('libgpg-error','libassuan','gpgme'):
 source=ROOT/'.obj/deps'/(name+'-'+manifest[name]['version']);build=ROOT/('.obj/build-'+name+'-'+a.target);build.mkdir(exist_ok=True)
 options=['--prefix='+str(prefix),'--disable-shared','--enable-static','--disable-nls','--disable-doc','--disable-tests']
 if windows:options+=['--host=x86_64-w64-mingw32']
 if name=='libassuan':options+=['--with-libgpg-error-prefix='+str(prefix)]
 if name=='gpgme':options+=['--with-libgpg-error-prefix='+str(prefix),'--with-libassuan-prefix='+str(prefix),'--disable-gpg-test','--disable-gpgsm-test','--disable-gpgconf-test','--disable-g13-test']
 for stage,cmd in [('configure',[source/'configure',*options]),('build',['make','-j4']),('install',['make','-C','src','install'] if name=='libgpg-error' else ['make','install'])]:
  with (build/(stage+'.log')).open('w') as output:subprocess.run(list(map(str,cmd)),cwd=build,env=env,stdout=output,stderr=subprocess.STDOUT,check=True)
 print(name,a.target,'built',flush=True)
source=ROOT/'.obj/deps'/('sqlite-'+manifest['sqlite']['version']);build=ROOT/('.obj/build-sqlite-'+a.target);build.mkdir(exist_ok=True)
for sub in ('include','lib/pkgconfig'):(prefix/sub).mkdir(parents=True,exist_ok=True)
with (build/'build.log').open('w') as output:
 subprocess.run([cc,'-O2','-DSQLITE_THREADSAFE=1','-DSQLITE_OMIT_LOAD_EXTENSION','-c',str(source/'sqlite3.c'),'-o',str(build/'sqlite3.o')],stdout=output,stderr=subprocess.STDOUT,check=True)
 subprocess.run([ar,'rcs',str(prefix/'lib/libsqlite3.a'),str(build/'sqlite3.o')],check=True)
shutil.copy2(source/'sqlite3.h',prefix/'include/sqlite3.h')
(prefix/'lib/pkgconfig/sqlite3.pc').write_text('prefix='+str(prefix)+'\nlibdir=${prefix}/lib\nincludedir=${prefix}/include\nName: SQLite\nDescription: SQLite\nVersion: '+manifest['sqlite']['version']+'\nLibs: -L${libdir} -lsqlite3\nCflags: -I${includedir}\n')
print('sqlite',a.target,'built')
