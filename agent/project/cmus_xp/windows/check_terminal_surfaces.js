/* Run with the editor's Electron executable and ELECTRON_RUN_AS_NODE=1.
 * Arguments: node-pty module, cmus.exe, tmux.exe, private scratch directory.
 * Captures go to stdout as JSONL. Only isolated cmus configs/tmux servers are
 * used. The app is not asked to play music. GPL-2.0-or-later.
 */
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const [modulePath, app, tmux, scratch] = process.argv.slice(2);
const pty = require(modulePath);
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const label = 'cmus_render_check_' + process.pid;
const baseEnv = {...process.env, CMUS_HOME: path.join(scratch, 'config')};
fs.mkdirSync(baseEnv.CMUS_HOME, {recursive: true});
let sequence = 0;

function open(exe, args, cols, rows, dll, name) {
    const child = pty.spawn(exe, args, {
        cols, rows, env: baseEnv, cwd: scratch,
        useConpty: true, useConptyDll: dll,
    });
    const state = {child, cols, rows, data: '', name, id: ++sequence};
    child.onData(text => {
        state.data += text;
        // Model responses supplied by xterm/VS Code during ConPTY startup.
        if (text.includes('\x1b[6n')) child.write('\x1b[1;1R');
        if (text.includes('\x1b[c')) child.write('\x1b[?1;2c');
    });
    return state;
}

async function capture(state, phase, wait = 1500) {
    await pause(wait);
    console.log(JSON.stringify({
        surface: state.name, stream: state.id, phase,
        cols: state.cols, rows: state.rows,
        bytes: Buffer.from(state.data).toString('base64'),
    }));
    state.data = '';
}

function control(...args) {
    return cp.execFileSync(tmux, ['-L', label, ...args], {
        env: baseEnv, cwd: scratch, windowsHide: true,
        stdio: ['ignore', 'pipe', 'pipe'],
    });
}

async function check(dll, multiplexed) {
    const surface = (dll ? 'editor_conpty' : 'system_conpty') +
        (multiplexed ? '_tmux' : '_direct');
    if (multiplexed)
        control('-f', 'NUL', 'new-session', '-d', '-s', 'check',
            '-x', '100', '-y', '30', '"' + app + '"');
    const launch = (cols, rows) => multiplexed
        ? open(tmux, ['-L', label, 'attach-session', '-t', 'check'],
            cols, rows, dll, surface)
        : open(app, [], cols, rows, dll, surface);
    let state = launch(100, 30);
    try {
        await capture(state, 'startup', 2200);
        for (const [cols, rows] of [[80, 24], [120, 40], [100, 30]]) {
            state.cols = cols;
            state.rows = rows;
            state.child.resize(cols, rows);
            await capture(state, 'resize');
        }
        if (multiplexed) {
            for (const [cols, rows] of [[80, 24], [120, 40]]) {
                state.child.write('\x02d');
                await pause(500);
                state.child.kill();
                state = launch(cols, rows);
                await capture(state, 'reattach', 2200);
            }
        }
        state.child.write(multiplexed ? '\x02d' : ':quit\r');
        await pause(500);
    } finally {
        state.child.kill();
        if (multiplexed) control('kill-server');
    }
}

(async () => {
    try {
        for (const dll of [false, true]) {
            await check(dll, false);
            await check(dll, true);
        }
    } catch (error) {
        console.error(error);
        process.exitCode = 1;
        try { control('kill-server'); } catch (_) {}
    }
    // node-pty's helper handles can outlive all exited test children.
    process.stdout.write('', () => process.exit(process.exitCode || 0));
})();
