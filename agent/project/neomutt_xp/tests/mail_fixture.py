"""Loopback-only SMTP fixture. It never relays mail or uses production accounts."""
import base64
import socketserver
import ssl
import threading
import sys

USER = 'fixture'
PASSWORD = 'fixture-pass'
TOKEN = 'fixture-oauth-token'

class SmtpHandler(socketserver.StreamRequestHandler):
    def send(self, text):
        self.wfile.write(text.encode('ascii') + b'\r\n')
        self.wfile.flush()

    def handle(self):
        self.connection.settimeout(15)
        session = {'commands': [], 'authenticated': False, 'tls': False, 'messages': []}
        self.server.sessions.append(session)
        if self.server.implicit_tls:
            self.connection = self.server.context.wrap_socket(self.connection, server_side=True)
            self.rfile = self.connection.makefile('rb')
            self.wfile = self.connection.makefile('wb')
            session['tls'] = True
        self.send('220 fixture.example.invalid ESMTP fixture')
        sender = None
        recipients = []
        while True:
            line = self.rfile.readline(65537)
            if not line or len(line) > 65536: return
            text = line.decode('utf-8', 'replace').rstrip('\r\n')
            verb, _, argument = text.partition(' ')
            verb = verb.upper()
            # Record mechanisms, not even the disposable credential payload.
            session['commands'].append(verb + (' ' + argument.split(' ')[0] if verb == 'AUTH' else ''))
            if verb in ('EHLO', 'HELO'):
                self.send('250-fixture.example.invalid')
                if self.server.context and not session['tls']: self.send('250-STARTTLS')
                self.send('250-AUTH ' + ' '.join(self.server.auth_methods))
                self.send('250-8BITMIME')
                self.send('250 SIZE 10485760')
            elif verb == 'STARTTLS' and self.server.context:
                self.send('220 Ready for TLS')
                self.connection = self.server.context.wrap_socket(self.connection, server_side=True)
                self.rfile = self.connection.makefile('rb')
                self.wfile = self.connection.makefile('wb')
                session['tls'] = True
            elif verb == 'AUTH':
                mechanism, _, encoded = argument.partition(' ')
                valid = False
                if mechanism.upper() not in self.server.auth_methods:
                    self.send('504 Unsupported authentication mechanism'); continue
                if mechanism.upper() == 'PLAIN':
                    if not encoded:
                        self.send('334 ')
                        encoded = self.rfile.readline().decode().strip()
                    try:
                        fields = base64.b64decode(encoded).split(b'\0')
                        valid = fields[-2:] == [USER.encode(), PASSWORD.encode()]
                    except Exception: pass
                elif mechanism.upper() == 'LOGIN':
                    self.send('334 '+base64.b64encode(self.server.login_prompts[0]).decode())
                    username = base64.b64decode(self.rfile.readline().strip())
                    self.send('334 '+base64.b64encode(self.server.login_prompts[1]).decode())
                    password = base64.b64decode(self.rfile.readline().strip())
                    valid = username == USER.encode() and password == PASSWORD.encode()
                elif mechanism.upper() == 'XOAUTH2':
                    try:
                        value = base64.b64decode(encoded)
                        valid = b'user=fixture\x01' in value and b'auth=Bearer ' + TOKEN.encode() + b'\x01' in value
                    except Exception: pass
                session['authenticated'] = valid
                self.send('235 2.7.0 fixture authenticated' if valid else '535 5.7.8 fixture rejected')
            elif verb == 'MAIL':
                if not session['authenticated']: self.send('530 Authentication required'); continue
                sender = argument
                recipients = []
                self.send('250 sender accepted')
            elif verb == 'RCPT':
                recipients.append(argument)
                self.send('250 recipient accepted')
            elif verb == 'DATA':
                if not session['authenticated']: self.send('530 Authentication required'); continue
                self.send('354 End with dot')
                payload = bytearray()
                while True:
                    piece = self.rfile.readline(10485761)
                    if not piece: return
                    if piece == b'.\r\n': break
                    if piece.startswith(b'..'): piece = piece[1:]
                    payload.extend(piece)
                    if len(payload) > 10485760: return
                if self.server.drop_data: return
                if self.server.reject_data:
                    self.send('550 5.7.1 fixture DATA rejected');continue
                session['messages'].append({'sender':sender,'recipients':recipients,'data':bytes(payload)})
                if self.server.drop_ack:return
                self.send('250 2.0.0 fixture captured')
                if self.server.drop_after_ack:return
            elif verb == 'QUIT': self.send('221 bye'); return
            elif verb in ('RSET','NOOP'): self.send('250 OK')
            else: self.send('502 unsupported fixture command')

class SmtpFixture(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True
    def __init__(self, context=None, implicit_tls=False, drop_data=False, auth_methods=None, login_prompts=None, drop_ack=False, drop_after_ack=False, reject_data=False):
        super().__init__(('127.0.0.1',0),SmtpHandler)
        self.login_prompts=login_prompts if login_prompts is not None else (b'Username:',b'Password:')
        self.auth_methods=auth_methods or ['PLAIN','LOGIN','XOAUTH2']
        self.context=context
        self.implicit_tls=implicit_tls
        self.drop_data=drop_data
        self.drop_ack=drop_ack
        self.reject_data=reject_data
        self.drop_after_ack=drop_after_ack
        self.sessions=[]
        self.errors=[]
    def handle_error(self, request, client_address):
        self.errors.append(str(sys.exc_info()[1]))
    def __enter__(self):
        self.thread=threading.Thread(target=self.serve_forever,daemon=True)
        self.thread.start()
        return self
    def __exit__(self,*args):
        self.shutdown();self.server_close();self.thread.join()
