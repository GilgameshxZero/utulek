#!/usr/bin/env python3
"""Bracketed-paste check through a real Windows ConPTY using pywinpty."""
import json
import os
from pathlib import Path
import sys
import threading
import time

from winpty import PtyProcess


def main():
    app, scratch = sys.argv[1], sys.argv[2]
    config = Path(scratch) / 'config'
    config.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, CMUS_HOME=str(config))
    process = PtyProcess.spawn([app], dimensions=(30, 100), cwd=scratch, env=env)
    data = bytearray()
    lock = threading.Lock()
    done = False

    def reader():
        while not done:
            try:
                chunk = process.read(4096)
            except EOFError:
                break
            except Exception:
                time.sleep(.05)
                continue
            if chunk:
                with lock:
                    data.extend(chunk if isinstance(chunk, bytes) else chunk.encode())
            else:
                time.sleep(.02)

    thread = threading.Thread(target=reader, daemon=True)
    thread.start()

    def take():
        with lock:
            value = bytes(data)
            data.clear()
        return value.decode('utf-8', 'replace')

    time.sleep(2.0)
    startup = take()
    process.write('\x1b[200~q\x1b[201~')
    time.sleep(1.0)
    pasted = take()
    process.write(':echo pasted\x1b[200~-text\x1b[201~\r')
    time.sleep(1.0)
    command = take()
    process.write('\x1b[A\x1bOP')
    time.sleep(.4)
    keys = take()
    process.write('q')
    time.sleep(.6)
    quit_prompt = take()
    process.write('n')
    result = {
        'enable_sequence': '\x1b[?2004h' in startup,
        'paste_ignored_quit': 'Really quit' not in pasted and 'quit?' not in pasted,
        'command_paste': 'pasted-text' in command or 'pasted' in command,
        'markers_hidden': '200~' not in command and '201~' not in command,
        'command_snippet': command[-400:],
        'alive': process.isalive(),
        'normal_quit_prompt': 'Quit cmus?' in quit_prompt or 'Really quit' in quit_prompt,
        'quit_snippet': quit_prompt[-300:],
        'keys_alive': process.isalive(),
    }
    result['passed'] = all(result[name] for name in
                           ('enable_sequence', 'paste_ignored_quit', 'alive', 'markers_hidden',
                            'normal_quit_prompt', 'keys_alive'))
    done = True
    try:
        process.terminate()
    except Exception:
        pass
    print(json.dumps(result))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
