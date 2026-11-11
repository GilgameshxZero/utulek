#!/usr/bin/env python3
"""Independent IMAP observer for owned Dovecot fixtures, including Windows UI tests."""
import argparse
import hashlib
import imaplib
import json
from pathlib import Path
import ssl
import urllib.request

USER = 'fixture@example.invalid'
PASSWORD = 'owned-imap-fixture-password'


def message(number):
    return (f'From: sender@example.invalid\r\nTo: {USER}\r\n'
            f'Message-ID: <imap-fixture-{number}@example.invalid>\r\n'
            f'Subject: IMAP fixture {number}\r\nMIME-Version: 1.0\r\n'
            'Content-Type: text/plain; charset=utf-8\r\n\r\n'
            f'Owned IMAP body {number}: Grüße 日本語\r\n').encode()


class Connection:
    def __init__(self, host, port, cert, mailbox='INBOX'):
        self.host, self.port, self.cert = host, port, cert
        self.mailbox = mailbox

    def connect(self):
        client = imaplib.IMAP4_SSL(self.host, self.port, timeout=10,
                                   ssl_context=ssl.create_default_context(cafile=str(self.cert)))
        client.login(USER, PASSWORD)
        return client


def snapshot(server, mailbox):
    client = server.connect()
    try:
        assert client.select(mailbox)[0] == 'OK'
        status, ids = client.uid('search', None, 'ALL')
        assert status == 'OK'
        result = []
        for uid in ids[0].split():
            status, payload = client.uid('fetch', uid, '(FLAGS BODY.PEEK[])')
            assert status == 'OK'
            data = next(item for item in payload if isinstance(item, tuple))
            result.append({'uid': uid.decode(), 'flags': data[0].decode(),
                           'body': data[1], 'sha256': hashlib.sha256(data[1]).hexdigest()})
        return result
    finally:
        client.logout()


def by_number(items, number):
    return [item for item in items if f'Subject: IMAP fixture {number}\r\n'.encode() in item['body']]


def check(server, phase):
    mailbox = getattr(server, 'mailbox', 'INBOX')
    if phase == 'seed':
        client = server.connect()
        try:
            client.delete('Original')
            if mailbox != 'INBOX':
                client.create(mailbox)
            client.create('Archive')
            for box in [mailbox, 'Archive']:
                client.select(box)
                if client.search(None, 'ALL')[1][0]:
                    client.store('1:*', '+FLAGS', '(\\Deleted)')
                    client.expunge()
            for number in range(1, 4):
                assert client.append(mailbox, None, None, message(number))[0] == 'OK'
        finally:
            client.logout()
        return
    if phase == 'append_new':
        client = server.connect()
        try:
            assert client.append('INBOX', None, None, message(4))[0] == 'OK'
        finally:
            client.logout()
        return
    if phase == 'external_reply':
        client = server.connect()
        try:
            client.select('INBOX')
            uid = by_number(snapshot(server, 'INBOX'), 1)[0]['uid']
            assert client.uid('store', uid, '+FLAGS.SILENT', '(\\Answered)')[0] == 'OK'
            assert 'Answered' in by_number(snapshot(server, mailbox),1)[0]['flags']
        finally:
            client.logout()
        return
    if phase in ['replace_identity', 'shift_uid']:
        client = server.connect()
        try:
            if phase == 'replace_identity':
                assert client.rename(mailbox, 'Original')[0] == 'OK'
                assert client.create(mailbox)[0] == 'OK'
                assert client.append(mailbox, '(\\Seen)', None, message(9))[0] == 'OK'
            else:
                uid = by_number(snapshot(server, mailbox), 1)[0]['uid']
                client.select(mailbox)
                assert client.uid('store', uid, '+FLAGS.SILENT', '(\\Deleted)')[0] == 'OK'
                client.expunge()
        finally:
            client.logout()
        return
    inbox = snapshot(server, mailbox)
    if phase == 'read_flag':
        item = by_number(inbox, 1)[0]
        assert '\\Flagged' in item['flags'] and '\\Seen' in item['flags'], item['flags']
        assert item['body'] == message(1)
    elif phase == 'move':
        assert not by_number(inbox, 2)
        assert by_number(snapshot(server, 'Archive'), 2)[0]['body'] == message(2)
    elif phase == 'delete':
        assert not by_number(inbox, 3)
    elif phase in ['merged_unflag', 'merged_flag']:
        item = by_number(inbox, 1)[0]
        assert ('\\Flagged' in item['flags']) == (phase == 'merged_flag'), item['flags']
        assert '\\Answered' in item['flags'], item['flags']
    elif phase == 'recovered_delete':
        assert not by_number(inbox, 1)
        assert by_number(inbox, 2)[0]['body'] == message(2)
        assert by_number(inbox, 3)[0]['body'] == message(3)
    elif phase == 'recovered_shift':
        assert not by_number(inbox, 1)
        item = by_number(inbox, 2)[0]
        assert '\\Flagged' in item['flags'] and item['body'] == message(2)
    elif phase == 'recovered_identity':
        assert len(inbox) == 1 and inbox[0]['body'] == message(9)
        assert '\\Flagged' not in inbox[0]['flags']
        assert by_number(snapshot(server, 'Original'), 1)[0]['body'] == message(1)
    elif phase == 'final':
        assert len(inbox) == 2
        for number in [1, 4]:
            assert by_number(inbox, number)[0]['body'] == message(number)
        assert '\\Flagged' in by_number(inbox, 1)[0]['flags']
        assert by_number(snapshot(server, 'Archive'), 2)[0]['body'] == message(2)
    else:
        raise ValueError('Unknown fixture phase: '+phase)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('connection', type=Path)
    parser.add_argument('phase', choices=['seed', 'append_new', 'read_flag', 'external_reply',
                                         'merged_unflag', 'merged_flag', 'move', 'delete', 'final',
                                         'drop_network', 'resume_network','replace_identity','shift_uid',
                                         'recovered_delete','recovered_shift','recovered_identity'])
    args = parser.parse_args()
    settings = json.loads(args.connection.read_text())
    server = Connection(settings['host'], settings.get('observer_port',settings['port']), settings['cert'], settings.get('mailbox','INBOX'))
    if args.phase in ['drop_network', 'resume_network']:
        request = urllib.request.Request(settings['control_url']+'/'+args.phase, data=b'',
            headers={'X-Fixture-Token': settings['control_token']}, method='POST')
        with urllib.request.urlopen(request, timeout=5) as response:
            assert response.status == 200
    else:
        check(server, args.phase)
    print(json.dumps({'phase': args.phase, 'passed': True}))
