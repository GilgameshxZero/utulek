/* Winsock descriptors never pass through CRT IO functions.
 * SPDX-License-Identifier: GPL-2.0-or-later */
#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <winsock2.h>
#include <ws2tcpip.h>
#include <windows.h>
#include <limits.h>
#include <errno.h>
#include <stdio.h>
#include "xp_socket.h"
static int socket_error(void)
{
  switch (WSAGetLastError()) {
    case WSAEINTR: errno = EINTR; break;
    case WSAETIMEDOUT: errno = ETIMEDOUT; break;
    case WSAEWOULDBLOCK: errno = EAGAIN; break;
    case WSAECONNREFUSED: errno = ECONNREFUSED; break;
    case WSAECONNRESET: errno = ECONNRESET; break;
    case WSAENETUNREACH: errno = ENETUNREACH; break;
    case WSAEHOSTUNREACH: errno = EHOSTUNREACH; break;
    default: errno = EIO;
  }
  return -1;
}
static INIT_ONCE Startup = INIT_ONCE_STATIC_INIT;
static BOOL CALLBACK startup(PINIT_ONCE once, PVOID arg, PVOID *context)
{ (void) once; (void) arg; (void) context; WSADATA data; return WSAStartup(MAKEWORD(2, 2), &data) == 0; }
int xp_socket_poll(intptr_t socket, int milliseconds, int writing, volatile sig_atomic_t *cancel)
{
  ULONGLONG deadline = GetTickCount64() + (milliseconds < 0 ? 0 : (ULONGLONG) milliseconds);
  for (;;)
  {
    if (cancel && *cancel) { errno = EINTR; return -1; }
    ULONGLONG now = GetTickCount64();
    int slice = milliseconds < 0 ? 100 : (int) (deadline > now ? deadline - now : 0);
    if (slice > 100) slice = 100;
    struct timeval timeout = { 0, slice * 1000 };
    fd_set read, write, errors;
    FD_ZERO(&read); FD_ZERO(&write); FD_ZERO(&errors);
    if (writing) FD_SET((SOCKET) socket, &write); else FD_SET((SOCKET) socket, &read);
    FD_SET((SOCKET) socket, &errors);
    int result = select(0, writing ? NULL : &read, writing ? &write : NULL, &errors, &timeout);
    if (result == SOCKET_ERROR) return socket_error();
    if (result > 0) return 1;
    if (milliseconds >= 0 && GetTickCount64() >= deadline) return 0;
  }
}
intptr_t xp_socket_open(const char *host, unsigned short port, int seconds, int ipv6, volatile sig_atomic_t *cancel)
{
  if (!InitOnceExecuteOnce(&Startup, startup, NULL, NULL)) { errno = EIO; return -1; }
  struct addrinfo hints = { 0 }, *addresses = NULL;
  hints.ai_family = ipv6 ? AF_UNSPEC : AF_INET; hints.ai_socktype = SOCK_STREAM;
  char service[6]; snprintf(service, sizeof(service), "%u", port);
  if (getaddrinfo(host, service, &hints, &addresses)) { errno = EHOSTUNREACH; return -1; }
  SOCKET result = INVALID_SOCKET;
  for (struct addrinfo *at = addresses; at; at = at->ai_next)
  {
    SOCKET fd = socket(at->ai_family, at->ai_socktype, at->ai_protocol);
    if (fd == INVALID_SOCKET) continue;
    SetHandleInformation((HANDLE) fd, HANDLE_FLAG_INHERIT, 0);
    u_long nonblocking = 1;
    if (ioctlsocket(fd, FIONBIO, &nonblocking)) { closesocket(fd); continue; }
    int rc = connect(fd, at->ai_addr, (int) at->ai_addrlen);
    if (rc == SOCKET_ERROR && WSAGetLastError() == WSAEWOULDBLOCK)
    {
      rc = xp_socket_poll((intptr_t) fd, seconds > 0 ? seconds * 1000 : -1, 1, cancel);
      if (rc > 0)
      {
        int error = 0, size = sizeof(error);
        if (getsockopt(fd, SOL_SOCKET, SO_ERROR, (char *) &error, &size) || error)
        { if (error) WSASetLastError(error); rc = socket_error(); }
        else rc = 0;
      }
      else if (rc == 0) { errno = ETIMEDOUT; rc = -1; }
    }
    else if (rc == SOCKET_ERROR) rc = socket_error();
    if (!rc) { result = fd; break; }
    closesocket(fd);
    if (cancel && *cancel) break;
  }
  freeaddrinfo(addresses);
  return result == INVALID_SOCKET ? -1 : (intptr_t) result;
}
int xp_socket_read(intptr_t fd, char *buffer, size_t count, int seconds, volatile sig_atomic_t *cancel)
{
  if (!count) return 0;
  int rc = xp_socket_poll(fd, seconds > 0 ? seconds * 1000 : -1, 0, cancel);
  if (rc <= 0) { if (!rc) errno = ETIMEDOUT; return -1; }
  rc = recv((SOCKET) fd, buffer, count > INT_MAX ? INT_MAX : (int) count, 0);
  return rc == SOCKET_ERROR ? socket_error() : rc;
}
int xp_socket_write(intptr_t fd, const char *buffer, size_t count, int seconds, volatile sig_atomic_t *cancel)
{
  size_t sent = 0;
  while (sent < count)
  {
    int rc = xp_socket_poll(fd, seconds > 0 ? seconds * 1000 : -1, 1, cancel);
    if (rc <= 0) { if (!rc) errno = ETIMEDOUT; return -1; }
    rc = send((SOCKET) fd, buffer + sent, count - sent > INT_MAX ? INT_MAX : (int) (count - sent), 0);
    if (rc == SOCKET_ERROR) return socket_error();
    if (!rc) { errno = EPIPE; return -1; }
    sent += (size_t) rc;
  }
  return sent > INT_MAX ? INT_MAX : (int) sent;
}
int xp_socket_close(intptr_t fd) { return closesocket((SOCKET) fd) ? socket_error() : 0; }
#endif
