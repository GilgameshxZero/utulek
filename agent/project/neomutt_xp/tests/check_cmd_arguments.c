/* Real CMD -> native/batch/Bash argv, non-expansion and injection checks. */
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <wchar.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <assert.h>
#include "xp_process.h"
int wmain(int argc, wchar_t **argv)
{
  SetErrorMode(SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX);
  if (argc > 1)
  {
    if (argc != 3 || wcscmp(argv[1], L"--dump"))
    { fprintf(stderr, "unexpected child argc=%d\n", argc); return 2; }
    int n = WideCharToMultiByte(CP_UTF8, 0, argv[2], -1, NULL, 0, NULL, NULL);
    char *s = malloc(n); assert(s);
    WideCharToMultiByte(CP_UTF8, 0, argv[2], -1, s, n, NULL, NULL);
    for (int i = 0; i < n-1; i++) printf("%02x", (unsigned char)s[i]);
    puts(""); free(s); return 0;
  }
  _putenv_s("NEOMUTT_XP_LITERAL_PERCENT", "%");
  _putenv_s("NEOMUTT_XP_SHELL", "");
  wchar_t wide[32768]; char self[65536];
  assert(GetModuleFileNameW(NULL, wide, 32768));
  assert(WideCharToMultiByte(CP_UTF8,0,wide,-1,self,sizeof(self),NULL,NULL));
  wchar_t *last = wcsrchr(wide, L'\\'); assert(last); *last=0;
  assert(SetCurrentDirectoryW(wide));
  assert(GetFileAttributesW(L"injection-sentinel") == INVALID_FILE_ATTRIBUTES);
  const char *base = strrchr(self, '\\'); base = base ? base+1 : self;
  FILE *f = fopen("argv_native.bat", "wb"); assert(f);
  fprintf(f, "@echo off\r\nsetlocal DisableDelayedExpansion\r\n\"%%~dp0%s\" %%*\r\n", base); fclose(f);
  f=fopen("argv_bash.bat", "wb"); assert(f);
  fputs("@echo off\r\nsetlocal DisableDelayedExpansion\r\nbash \"%~dp0argv_bash\" %*\r\n",f); fclose(f);
  f=fopen("argv_bash", "wb"); assert(f);
  fprintf(f, "#!/usr/bin/env bash\nexec \"./%s\" \"$@\"\n",base); fclose(f);
  const char *cases[] = {"plain", "space Grüße 日本語", "& | < > ^ ; ` ' !PATH! %PATH%",
    "a\"b", "a\"& echo injected>injection-sentinel & rem \"x", "trailing\\",
    "slashes\\\\\"quoted", "quote\"&quote\"|x", "", "tab\tinside", "%%!%UNDEFINED_VARIABLE%!%%"};
  f=fopen("argv_python.bat", "wb"); assert(f);
  fputs("@echo off\r\nsetlocal DisableDelayedExpansion\r\npython3 -X utf8 \"%~dp0argv_python.py\" %*\r\n", f); fclose(f);
  f=fopen("argv_python.py", "wb"); assert(f);
  fputs("import sys\nassert len(sys.argv)==3\nprint(sys.argv[2].encode('utf-8').hex())\n", f); fclose(f);
  const char *routes[] = {self, "argv_native.bat", "argv_bash.bat", "argv_python.bat"};
  for (size_t route=0; route<4; route++)
  {
    char *program=xp_quote_cmd_argument(routes[route],1); assert(program);
    for (size_t i=0; i<sizeof(cases)/sizeof(cases[0]); i++)
    {
      char *arg=xp_quote_cmd_argument(cases[i],1); assert(arg);
      size_t size=strlen(program)+strlen(arg)+32;
      char *command=malloc(size); assert(command);
      snprintf(command,size,"%s --dump %s",program,arg);
      FILE *out=NULL,*error=NULL;
      pid_t pid=xp_filter_create(command,NULL,&out,&error,-1,-1,-1,NULL);
      assert(pid>0 && out && error);
      char actual[4096]={0}, diagnostic[1024]={0};
      size_t n=fread(actual,1,sizeof(actual)-1,out);
      fread(diagnostic,1,sizeof(diagnostic)-1,error); fclose(out); fclose(error);
      int status; assert(xp_waitpid(pid,&status,0)==pid);
      char expected[4096], *at=expected;
      for(const unsigned char *s=(const unsigned char*)cases[i]; *s; s++) at+=sprintf(at,"%02x",*s);
      *at=0;
      while(n && (actual[n-1]=='\r' || actual[n-1]=='\n')) actual[--n]=0;
      if(status || strcmp(actual,expected) || *diagnostic)
      { fprintf(stderr,"route %zu case %zu failed status=%d actual=%s expected=%s error=%s\n",route,i,status,actual,expected,diagnostic); return 1; }
      assert(GetFileAttributesW(L"injection-sentinel")==INVALID_FILE_ATTRIBUTES);
      free(command); free(arg);
    }
    free(program);
  }
  puts("CMD: 44 native/batch/Bash/Python argument roundtrips and injection checks passed");
  return 0;
}
