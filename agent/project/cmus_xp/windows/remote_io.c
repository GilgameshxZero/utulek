/* Local same-user named pipes and authenticated upstream TCP transport.
 * GPL-2.0-or-later. No background thread touches the player or command state. */
#include <winsock2.h>
#include <ws2tcpip.h>
#include <sddl.h>
#include <errno.h>
#include <limits.h>
#include <stdint.h>
#include <stdio.h>
#include <string.h>
#include "remote_io.h"

#define CONNECTIONS 32
struct connection {
    int kind, listener, nonblocking, connected;
    HANDLE pipe;
    SOCKET socket;
    wchar_t *name;
    PSECURITY_DESCRIPTOR security;
};
static struct connection connections[CONNECTIONS];

static int allocate(void)
{
    for (int i = 0; i < CONNECTIONS; i++)
        if (!connections[i].kind) return i;
    errno = EMFILE;
    return -1;
}

static int socket_error(void)
{
    switch (WSAGetLastError()) {
    case WSAEWOULDBLOCK: errno = EAGAIN; break;
    case WSAEINTR: errno = EINTR; break;
    case WSAECONNREFUSED: errno = ECONNREFUSED; break;
    case WSAEADDRINUSE: errno = EADDRINUSE; break;
    case WSAETIMEDOUT: errno = ETIMEDOUT; break;
    case WSAEACCES: errno = EACCES; break;
    default: errno = EIO; break;
    }
    return -1;
}

static TOKEN_USER *current_user(void)
{
    HANDLE token;
    DWORD size = 0;
    if (!OpenProcessToken(GetCurrentProcess(), TOKEN_QUERY, &token)) return NULL;
    GetTokenInformation(token, TokenUser, NULL, 0, &size);
    TOKEN_USER *user = malloc(size);
    if (user && !GetTokenInformation(token, TokenUser, user, size, &size)) {
        free(user); user = NULL;
    }
    CloseHandle(token);
    return user;
}

int win_remote_local(const char *address)
{
    return strchr(address, '/') || strchr(address, '\\');
}

static wchar_t *pipe_name(const char *address, PSECURITY_DESCRIPTOR *security)
{
    TOKEN_USER *user = current_user();
    wchar_t *sid = NULL, *name = NULL;
    if (!user || !ConvertSidToStringSidW(user->User.Sid, &sid)) goto done;
    wchar_t sddl[256];
    swprintf(sddl, 256, L"D:P(A;;GA;;;%ls)", sid);
    if (!ConvertStringSecurityDescriptorToSecurityDescriptorW(sddl, SDDL_REVISION_1, security, NULL)) goto done;
    if (!strncmp(address, "\\\\.\\pipe\\", 9)) {
        name = win_wide(address);
    } else {
        char *absolute = win_absolute_path(address, NULL);
        wchar_t *wide = absolute ? win_wide(absolute) : NULL;
        free(absolute);
        if (!wide) goto done;
        CharLowerBuffW(wide, (DWORD)wcslen(wide));
        uint64_t hash = UINT64_C(14695981039346656037);
        for (wchar_t *p = wide; *p; p++) { hash ^= (unsigned)*p; hash *= UINT64_C(1099511628211); }
        free(wide);
        name = malloc(256 * sizeof(*name));
        if (name) swprintf(name, 256, L"\\\\.\\pipe\\cmus-%ls-%016llx", sid, (unsigned long long)hash);
    }
done:
    if (sid) LocalFree(sid);
    free(user);
    if (!name) { if (*security) LocalFree(*security); *security = NULL; errno = EACCES; }
    return name;
}

static HANDLE make_pipe(struct connection *c, int first)
{
    SECURITY_ATTRIBUTES sa = {sizeof(sa), c->security, FALSE};
    return CreateNamedPipeW(c->name, PIPE_ACCESS_DUPLEX | (first ? FILE_FLAG_FIRST_PIPE_INSTANCE : 0),
        PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_NOWAIT | PIPE_REJECT_REMOTE_CLIENTS,
        CONNECTIONS, 64 * 1024, 64 * 1024, 0, &sa);
}

static int same_server_user(HANDLE pipe)
{
    ULONG pid;
    if (!GetNamedPipeServerProcessId(pipe, &pid)) return 0;
    HANDLE process = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid), token = NULL;
    DWORD size = 0;
    TOKEN_USER *peer = NULL, *self = current_user();
    int same = 0;
    if (process && self && OpenProcessToken(process, TOKEN_QUERY, &token)) {
        GetTokenInformation(token, TokenUser, NULL, 0, &size);
        peer = malloc(size);
        if (peer && GetTokenInformation(token, TokenUser, peer, size, &size))
            same = EqualSid(peer->User.Sid, self->User.Sid);
    }
    if (token) CloseHandle(token);
    if (process) CloseHandle(process);
    free(peer); free(self);
    return same;
}

static int tcp_open(const char *address, int listening)
{
    static int initialized;
    if (!initialized) {
        WSADATA data;
        if (WSAStartup(MAKEWORD(2, 2), &data)) { errno = EIO; return -1; }
        initialized = 1;
    }
    int id = allocate();
    if (id < 0) return -1;
    char *host = _strdup(address), *port = "3000";
    if (!host) { errno = ENOMEM; return -1; }
    if (host[0] == '[') {
        char *end = strchr(host, ']');
        if (!end || (end[1] && end[1] != ':')) { free(host); errno = EINVAL; return -1; }
        if (end[1] == ':') port = end + 2;
        *end = 0; memmove(host, host + 1, strlen(host));
    } else {
        char *colon = strrchr(host, ':');
        if (colon && strchr(host, ':') == colon) { *colon = 0; port = colon + 1; }
    }
    struct addrinfo hints = {0}, *addresses = NULL;
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_flags = listening ? AI_PASSIVE : 0;
    if (getaddrinfo(*host ? host : NULL, port, &hints, &addresses)) {
        free(host); errno = EINVAL; return -1;
    }
    SOCKET sock = INVALID_SOCKET;
    int saved = ECONNREFUSED;
    for (struct addrinfo *a = addresses; a; a = a->ai_next) {
        sock = socket(a->ai_family, a->ai_socktype, a->ai_protocol);
        if (sock == INVALID_SOCKET) continue;
        int rc;
        if (listening) {
            BOOL exclusive = TRUE;
            setsockopt(sock, SOL_SOCKET, SO_EXCLUSIVEADDRUSE, (const char *)&exclusive, sizeof(exclusive));
            rc = bind(sock, a->ai_addr, (int)a->ai_addrlen);
            if (!rc) rc = listen(sock, 10);
        } else rc = connect(sock, a->ai_addr, (int)a->ai_addrlen);
        if (!rc) break;
        socket_error(); saved = errno; closesocket(sock); sock = INVALID_SOCKET;
    }
    freeaddrinfo(addresses); free(host);
    if (sock == INVALID_SOCKET) { errno = saved; return -1; }
    if (listening) {
        u_long nonblocking = 1;
        ioctlsocket(sock, FIONBIO, &nonblocking);
    } else {
        DWORD timeout = 5000;
        setsockopt(sock, SOL_SOCKET, SO_RCVTIMEO, (char *)&timeout, sizeof(timeout));
        setsockopt(sock, SOL_SOCKET, SO_SNDTIMEO, (char *)&timeout, sizeof(timeout));
    }
    connections[id] = (struct connection){.kind = 2, .socket = sock, .listener = listening, .nonblocking = listening};
    return id;
}

int win_remote_listen(const char *address)
{
    if (!win_remote_local(address)) return tcp_open(address, 1);
    int id = allocate();
    if (id < 0) return -1;
    struct connection c = {.kind = 1, .listener = 1, .nonblocking = 1};
    c.name = pipe_name(address, &c.security);
    if (!c.name) return -1;
    c.pipe = make_pipe(&c, 1);
    if (c.pipe == INVALID_HANDLE_VALUE) {
        free(c.name); LocalFree(c.security); errno = EADDRINUSE; return -1;
    }
    connections[id] = c;
    return id;
}

int win_remote_connect(const char *address)
{
    if (!win_remote_local(address)) return tcp_open(address, 0);
    int id = allocate();
    if (id < 0) return -1;
    PSECURITY_DESCRIPTOR security = NULL;
    wchar_t *name = pipe_name(address, &security);
    if (!name) return -1;
    HANDLE pipe = CreateFileW(name, GENERIC_READ | GENERIC_WRITE, 0, NULL, OPEN_EXISTING, 0, NULL);
    if (pipe == INVALID_HANDLE_VALUE && GetLastError() == ERROR_PIPE_BUSY && WaitNamedPipeW(name, 2000))
        pipe = CreateFileW(name, GENERIC_READ | GENERIC_WRITE, 0, NULL, OPEN_EXISTING, 0, NULL);
    free(name); LocalFree(security);
    if (pipe == INVALID_HANDLE_VALUE) { errno = ECONNREFUSED; return -1; }
    if (!same_server_user(pipe)) { CloseHandle(pipe); errno = EACCES; return -1; }
    connections[id] = (struct connection){.kind = 1, .pipe = pipe};
    return id;
}

int win_remote_ready(int id, int writing)
{
    struct connection *c = &connections[id];
    if (c->kind == 2) {
        WSAPOLLFD p = {.fd = c->socket, .events = writing ? POLLWRNORM : POLLRDNORM};
        return WSAPoll(&p, 1, 0) > 0;
    }
    if (c->listener) {
        if (c->connected) return 1;
        if (ConnectNamedPipe(c->pipe, NULL) || GetLastError() == ERROR_PIPE_CONNECTED) c->connected = 1;
        return c->connected;
    }
    if (writing) return 1;
    DWORD available;
    return !PeekNamedPipe(c->pipe, NULL, 0, NULL, &available, NULL) || available != 0;
}

int win_remote_accept(int listener)
{
    struct connection *c = &connections[listener];
    int id = allocate();
    if (id < 0) return -1;
    if (c->kind == 2) {
        SOCKET sock = accept(c->socket, NULL, NULL);
        if (sock == INVALID_SOCKET) return socket_error();
        u_long nonblocking = 1;
        ioctlsocket(sock, FIONBIO, &nonblocking);
        connections[id] = (struct connection){.kind = 2, .socket = sock, .nonblocking = 1};
    } else {
        if (!win_remote_ready(listener, 0)) { errno = EAGAIN; return -1; }
        HANDLE next = make_pipe(c, 0);
        if (next == INVALID_HANDLE_VALUE) { errno = EIO; return -1; }
        connections[id] = (struct connection){.kind = 1, .pipe = c->pipe, .nonblocking = 1};
        c->pipe = next; c->connected = 0;
    }
    return id;
}

ssize_t win_remote_read(int id, void *buffer, size_t size)
{
    struct connection *c = &connections[id];
    if (size > INT_MAX) size = INT_MAX;
    if (c->kind == 2) {
        int n = recv(c->socket, buffer, (int)size, 0);
        return n == SOCKET_ERROR ? socket_error() : n;
    }
    ULONGLONG start = GetTickCount64();
    DWORD available, count;
    while (PeekNamedPipe(c->pipe, NULL, 0, NULL, &available, NULL)) {
        if (available) {
            if (size > available) size = available;
            if (ReadFile(c->pipe, buffer, (DWORD)size, &count, NULL)) return count;
            errno = EIO; return -1;
        }
        if (c->nonblocking) { errno = EAGAIN; return -1; }
        if (GetTickCount64() - start >= 5000) { errno = ETIMEDOUT; return -1; }
        Sleep(10);
    }
    if (GetLastError() == ERROR_BROKEN_PIPE) return 0;
    errno = EIO; return -1;
}

ssize_t win_remote_write(int id, const void *buffer, size_t size)
{
    struct connection *c = &connections[id];
    if (size > INT_MAX) size = INT_MAX;
    if (c->kind == 2) {
        int n = send(c->socket, buffer, (int)size, 0);
        return n == SOCKET_ERROR ? socket_error() : n;
    }
    DWORD count;
    if (!WriteFile(c->pipe, buffer, (DWORD)size, &count, NULL)) { errno = EPIPE; return -1; }
    if (!count && size) { errno = EAGAIN; return -1; }
    return count;
}

void win_remote_close(int id)
{
    if (id < 0 || id >= CONNECTIONS || !connections[id].kind) return;
    struct connection *c = &connections[id];
    if (c->kind == 2) closesocket(c->socket);
    else CloseHandle(c->pipe);
    free(c->name);
    if (c->security) LocalFree(c->security);
    *c = (struct connection){0};
}
