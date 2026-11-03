/* OpenSSL BIO for full-width SOCKETs, bounded waits and cancellation.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <winsock2.h>
#include <windows.h>
#include <wincrypt.h>
#include <openssl/ssl.h>
#include <openssl/err.h>
#include <errno.h>
#include <stdlib.h>
#include "xp_socket.h"
struct TlsIo { intptr_t fd; int seconds; volatile sig_atomic_t *cancel; };
static BIO_METHOD *Method;
static INIT_ONCE Once = INIT_ONCE_STATIC_INIT;
static int create(BIO *bio) { BIO_set_init(bio, 1); return 1; }
static int destroy(BIO *bio) { if (bio) free(BIO_get_data(bio)); return 1; }
static int read_data(BIO *bio, char *buffer, int length)
{
  struct TlsIo *io = BIO_get_data(bio);
  BIO_clear_retry_flags(bio);
  return length <= 0 ? 0 : xp_socket_read(io->fd, buffer, (size_t) length, io->seconds, io->cancel);
}
static int write_data(BIO *bio, const char *buffer, int length)
{
  struct TlsIo *io = BIO_get_data(bio);
  BIO_clear_retry_flags(bio);
  return length <= 0 ? 0 : xp_socket_write(io->fd, buffer, (size_t) length, io->seconds, io->cancel);
}
static long control(BIO *bio, int command, long argument, void *pointer)
{
  (void) bio; (void) argument; (void) pointer;
  return command == BIO_CTRL_FLUSH ? 1 : 0;
}
static BOOL CALLBACK initialize(PINIT_ONCE once, PVOID arg, PVOID *context)
{
  (void) once; (void) arg; (void) context;
  Method = BIO_meth_new(BIO_TYPE_SOURCE_SINK | BIO_get_new_index(), "NeoMutt Winsock");
  if (!Method) return FALSE;
  if (!BIO_meth_set_create(Method, create) || !BIO_meth_set_destroy(Method, destroy) ||
      !BIO_meth_set_read(Method, read_data) || !BIO_meth_set_write(Method, write_data) ||
      !BIO_meth_set_ctrl(Method, control))
  { BIO_meth_free(Method); Method = NULL; return FALSE; }
  return TRUE;
}
int xp_tls_set_socket(SSL *ssl, intptr_t fd, int seconds, volatile sig_atomic_t *cancel)
{
  if (!InitOnceExecuteOnce(&Once, initialize, NULL, NULL)) return 0;
  BIO *bio = BIO_new(Method);
  struct TlsIo *io = calloc(1, sizeof(*io));
  if (!bio || !io) { BIO_free(bio); free(io); return 0; }
  *io = (struct TlsIo) { fd, seconds, cancel };
  BIO_set_data(bio, io);
  SSL_set_bio(ssl, bio, bio);
  return 1;
}
int xp_tls_system_certs(SSL_CTX *context)
{
  HCERTSTORE store = CertOpenSystemStoreW(0, L"ROOT");
  if (!store) return 0;
  PCCERT_CONTEXT certificate = NULL;
  int count = 0;
  while ((certificate = CertEnumCertificatesInStore(store, certificate)))
  {
    const unsigned char *bytes = certificate->pbCertEncoded;
    X509 *decoded = d2i_X509(NULL, &bytes, certificate->cbCertEncoded);
    if (decoded)
    {
      if (X509_STORE_add_cert(SSL_CTX_get_cert_store(context), decoded)) count++;
      X509_free(decoded);
    }
    ERR_clear_error();
  }
  CertCloseStore(store, 0);
  return count > 0;
}
#endif
