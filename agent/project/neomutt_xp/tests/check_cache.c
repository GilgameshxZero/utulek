/* Cache restart and disk-allocation check. GPL-2.0-or-later */
#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <lmdb.h>
#ifdef _WIN32
#include <windows.h>
#include <io.h>
#endif
int main(int argc, char **argv)
{
  assert(argc == 2);
  MDB_env *env; MDB_txn *transaction; MDB_dbi db;
  assert(mdb_env_create(&env) == 0);
  assert(mdb_env_set_mapsize(env, 100ULL * 1024 * 1024 * 1024) == 0);
  assert(mdb_env_open(env, argv[1], MDB_NOSUBDIR, 0600) == 0);
  assert(mdb_txn_begin(env, NULL, 0, &transaction) == 0);
  assert(mdb_dbi_open(transaction, NULL, MDB_CREATE, &db) == 0);
  for (unsigned i = 0; i < 1024; i++)
  {
    char name[64], value[128]; snprintf(name, sizeof(name), "header-%u", i);
    snprintf(value, sizeof(value), "Synthetic cached header %u Grüße 日本語", i);
    MDB_val key = { strlen(name), name }, data = { strlen(value), value };
    assert(mdb_put(transaction, db, &key, &data, 0) == 0);
  }
  assert(mdb_txn_commit(transaction) == 0);
  mdb_env_close(env);
  assert(mdb_env_create(&env) == 0);
  assert(mdb_env_open(env, argv[1], MDB_NOSUBDIR | MDB_RDONLY, 0600) == 0);
  assert(mdb_txn_begin(env, NULL, MDB_RDONLY, &transaction) == 0);
  assert(mdb_dbi_open(transaction, NULL, 0, &db) == 0);
  char name[] = "header-1023"; MDB_val key = { strlen(name), name }, data;
  assert(mdb_get(transaction, db, &key, &data) == 0);
  assert(data.mv_size > 10);
  mdb_txn_abort(transaction); mdb_env_close(env);
#ifdef _WIN32
  wchar_t file[32768];
  assert(MultiByteToWideChar(CP_UTF8, MB_ERR_INVALID_CHARS, argv[1], -1, file, 32768));
  DWORD high = 0; DWORD low = GetCompressedFileSizeW(file, &high);
  assert(low != INVALID_FILE_SIZE || GetLastError() == NO_ERROR);
  unsigned long long allocated = ((unsigned long long) high << 32) | low;
  assert(allocated < 2 * 1024 * 1024);
  printf("cache: 1024 headers persisted/reopened; allocated=%llu bytes for 100GiB logical map\n", allocated);
#else
  puts("cache: 1024 headers persisted/reopened");
#endif
  return 0;
}
