#!/usr/bin/env python3
"""Check real PulseAudio/PipeWire playback using only the test player's monitor.

Plays quiet generated tones on the existing default sink. Does not change the
default device, sink volume, or user configuration. Recording is restricted to
this process's sink input; other applications' audio is not captured.
"""
import argparse
import array
import fcntl
import json
import math
import os
from pathlib import Path
import pty
import select
import struct
import subprocess
import termios
import time

ROOT = Path(__file__).resolve().parent.parent


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--remote', type=Path, default=ROOT / '.bin/cmus-remote')
    parser.add_argument('--output-dir', type=Path, default=ROOT / '.obj/linux_audio_checks')
    parser.add_argument('--output-plugin', choices=('pulse', 'ao'), default='pulse')
    args = parser.parse_args()
    out = args.output_dir.resolve()
    out.mkdir(parents=True, exist_ok=True)
    # Short socket paths work even in long source checkouts.
    socket = Path('/tmp') / ('cmus-xp-audio-%d.sock' % os.getpid())
    config = out / ('config-%d' % os.getpid())
    config.mkdir()
    config.joinpath('rc').write_text('set mpris=false\nset output_plugin=' + args.output_plugin +
        '\nset softvol=true\nset softvol_state=100 100\n' +
        ('set dsp.ao.driver=pulse\n' if args.output_plugin == 'ao' else ''))
    fixtures = []
    signal = '0.01*sin(2*PI*if(lt(t,8),440,if(lt(t,16),880,1320))*t)'
    ffmpeg = ['ffmpeg', '-hide_banner', '-loglevel', 'error', '-y']
    for extension in ('opus', 'mkv'):
        path = out / ('quiet_tones.' + extension)
        command = ffmpeg + ['-f', 'lavfi', '-i', 'aevalsrc=' + signal + '|' + signal + ':s=48000:d=24:c=stereo']
        # Escape commas inside lavfi expressions, while preserving channel separators.
        command[-1] = command[-1].replace(',', '\\,')
        if extension == 'mkv':
            command += ['-f', 'lavfi', '-i', 'color=size=64x64:rate=5:duration=24',
                        '-map', '1:v', '-map', '0:a', '-c:v', 'mpeg4', '-c:a', 'flac']
        else:
            command += ['-c:a', 'libopus']
        subprocess.run(command + [str(path)], check=True)
        fixtures.append(path)
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 100, 0, 0))
    env = dict(os.environ, TERM='xterm-256color', CMUS_HOME=str(config))
    process = subprocess.Popen([str(args.binary.resolve()), '--listen', str(socket)],
                               stdin=slave, stdout=slave, stderr=slave, env=env)
    os.close(slave)
    screen = bytearray()
    def wait(seconds):
        end = time.monotonic() + seconds
        while time.monotonic() < end:
            if select.select([master], [], [], min(.05, max(0, end-time.monotonic())))[0]:
                try:
                    screen.extend(os.read(master, 65536))
                except OSError:
                    break
    def command(*commands):
        client = subprocess.Popen([str(args.remote.resolve()), '--server', str(socket), '-C', *commands],
                                  stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        deadline = time.monotonic() + 10
        try:
            # Keep draining the terminal while the server redraws in response.
            while client.poll() is None and time.monotonic() < deadline:
                wait(.02)
            output, errors = client.communicate(timeout=max(.01, deadline-time.monotonic()))
            if client.returncode:
                raise subprocess.CalledProcessError(client.returncode, client.args, output, errors)
            if 'Error:' in output:
                raise RuntimeError(output)
            return output
        finally:
            if client.poll() is None:
                client.kill()
                client.communicate()
    def status():
        return dict(line.split(' ', 1) for line in command('status').splitlines() if ' ' in line)
    def pulse(kind):
        return json.loads(subprocess.check_output(['pactl', '-f', 'json', 'list', kind], text=True))
    def stream():
        inputs = pulse('sink-inputs')
        return next(s for s in inputs if s['properties'].get('application.process.id') == str(process.pid))
    def capture(seconds=.8):
        own = stream()
        sink = next(s for s in pulse('sinks') if s['index'] == own['sink'])
        recorder = subprocess.Popen(['parec', '--raw', '--format=s16le', '--rate=48000', '--channels=2',
                                     '--latency-msec=20', '--monitor-stream=' + str(own['index']),
                                     '--device=' + str(sink.get('monitor_source_name') or sink['monitor_source'])], stdout=subprocess.PIPE,
                                    stderr=subprocess.PIPE)
        data = bytearray()
        try:
            end = time.monotonic() + seconds
            while time.monotonic() < end:
                wait(.02)
                if select.select([recorder.stdout], [], [], 0)[0]:
                    data.extend(os.read(recorder.stdout.fileno(), 65536))
        finally:
            recorder.terminate()
            tail, errors = recorder.communicate(timeout=5)
            data.extend(tail)
        if errors:
            raise RuntimeError(errors.decode(errors='replace'))
        samples = array.array('h', data[:len(data)//4*4])[::2]
        # Exclude initial monitor setup silence from the amplitude/frequency window.
        samples = samples[-24000:]
        rms = math.sqrt(sum(x*x for x in samples)/len(samples)) if samples else 0
        crossings = sum(a <= 0 < b for a, b in zip(samples, samples[1:]))
        hz = crossings * 48000 / len(samples) if samples else 0
        return {'rms': rms, 'frequency_hz': hz, 'samples': len(samples),
                'sink': sink['name'], 'sink_state': sink['state'],
                'device_api': sink['properties'].get('device.api'), 'stream_index': own['index']}
    results = []
    try:
        for _ in range(100):
            if socket.exists(): break
            if process.poll() is not None: raise RuntimeError('Player exited before startup')
            wait(.05)
        for path in fixtures:
            command('set softvol=true', 'set softvol_state=100 100', 'player-play ' + str(path))
            wait(.7)
            playing = status()
            initial = capture()
            assert playing['status'] == 'playing' and initial['rms'] > 20, (playing, initial)
            assert initial['sink_state'] == 'RUNNING' and initial['device_api'] == 'alsa', initial
            assert abs(initial['frequency_hz'] - 440) < 20, initial
            command('player-pause')
            wait(.3)
            paused = status()
            silence = capture(.6)
            wait(.4)
            assert paused['status'] == 'paused' and status()['position'] == paused['position'], paused
            assert silence['rms'] < 2, silence
            command('seek +10')
            wait(.2)
            sought = status()
            assert int(sought['position']) - int(paused['position']) == 10, (paused, sought)
            command('player-pause')
            wait(.3)
            resumed = capture()
            assert status()['status'] == 'playing' and resumed['rms'] > 20, resumed
            assert abs(resumed['frequency_hz'] - 880) < 20, resumed
            command('vol 50%')
            wait(.5)
            quieter = capture()
            assert 0 < quieter['rms'] < resumed['rms']*.8, (resumed, quieter)
            command('player-stop')
            wait(.2)
            assert status()['status'] == 'stopped'
            result = {'file': path.name, 'output_plugin': args.output_plugin,
                      'passed': True, 'playing': initial, 'paused': silence,
                      'resumed_after_seek': resumed, 'volume_50_percent': quieter,
                      'pause_position': paused['position'], 'seek_position': sought['position']}
            results.append(result)
            print(path.name + ': real sink/play/pause/seek/resume/volume/stop PASS', flush=True)
        command('quit')
        process.wait(timeout=5)
        assert process.returncode == 0
    finally:
        if process.poll() is None:
            try: command('quit'); process.wait(timeout=5)
            except (subprocess.SubprocessError, OSError):
                process.terminate(); process.wait(timeout=5)
        os.close(master)
        socket.unlink(missing_ok=True)
        (out / 'screen.txt').write_bytes(screen)
        (out / 'results.json').write_text(json.dumps(results, indent=2) + '\n')


if __name__ == '__main__':
    main()
