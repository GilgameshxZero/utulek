"""Transparent loopback proxy for interrupting owned test connections."""
import select
import socket
import socketserver
import threading
import http.server
import secrets


class DisconnectProxy:
    def __init__(self, target_port):
        self.target_port = target_port
        self.connections = set()
        self.lock = threading.Lock()
        self.blocked = False
        owner = self

        class Handler(socketserver.BaseRequestHandler):
            def handle(self):
                with owner.lock:
                    if owner.blocked:
                        return
                try:
                    upstream = socket.create_connection(('127.0.0.1', owner.target_port), timeout=3)
                except OSError:
                    return
                pair = (self.request, upstream)
                with owner.lock:
                    owner.connections.add(pair)
                try:
                    while True:
                        ready, _, _ = select.select(pair, [], [], .2)
                        for source in ready:
                            payload = source.recv(65536)
                            if not payload:
                                return
                            destination = upstream if source is self.request else self.request
                            destination.sendall(payload)
                except (OSError, ValueError):
                    pass
                finally:
                    with owner.lock:
                        owner.connections.discard(pair)
                    upstream.close()

        class Server(socketserver.ThreadingTCPServer):
            daemon_threads = True

        self.server = Server(('127.0.0.1', 0), Handler)
        self.port = self.server.server_address[1]
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .1}, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def disconnect(self, block=False):
        with self.lock:
            self.blocked = block
            pairs = list(self.connections)
        for pair in pairs:
            for sock in pair:
                try:
                    sock.shutdown(socket.SHUT_RDWR)
                except OSError:
                    pass
                sock.close()

    def resume(self):
        with self.lock:
            self.blocked = False

    def __exit__(self, *_):
        self.disconnect(block=True)
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)


class DisconnectControl:
    """Loopback-only control channel for the Windows fixture's network outage."""
    def __init__(self, proxy):
        self.token = secrets.token_hex(16)
        owner = self

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                if self.headers.get('X-Fixture-Token') != owner.token:
                    self.send_error(403)
                    return
                if self.path == '/drop_network':
                    proxy.disconnect(block=True)
                elif self.path == '/resume_network':
                    proxy.resume()
                else:
                    self.send_error(404)
                    return
                self.send_response(200)
                self.end_headers()

            def log_message(self, *_):
                pass

        self.server = http.server.HTTPServer(('127.0.0.1', 0), Handler)
        self.port = self.server.server_address[1]
        self.thread = threading.Thread(target=self.server.serve_forever, kwargs={'poll_interval': .1}, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *_):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=2)
