/* Capture populated album headings in isolated editor/system ConPTY sessions.
 * Arguments: node-pty module, cmus.exe, fixture directory, output JSONL.
 * No playback or user configuration is touched. */
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const [modulePath, app, fixtures, output, tmux] = process.argv.slice(2);
const pty = require(modulePath);
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const frames = [];
const commands = [
    ['albums', null],
    ['standout', ':set color_trackwin_album_attr=standout\r'],
    ['reverse', ':set color_trackwin_album_attr=reverse\r'],
    ['bold', ':set color_trackwin_album_attr=bold\r'],
    ['palette', ':set color_trackwin_album_attr=default\r:set color_trackwin_album_fg=red\r:set color_trackwin_album_bg=blue\r'],
    ['restore_colors', ':set color_trackwin_album_fg=default\r:set color_trackwin_album_bg=default\r'],
    ['attributes', ':set color_trackwin_album_attr=bold|reverse|underline|italic\r'],
    ['short_heading', ':set format_trackwin_album= Short %= R \r'],
    ['restore_heading', ':set format_trackwin_album= %l %= %y %{duration} \r'],
    ['focus_tracks', '\t'],
    ['selection', 'jjjj'],
    ['view_switch', '21'],
];

(async () => {
    for (const [dll,multiplexed] of [[false,false],[true,false],...(tmux?[[false,true],[true,true]]:[])]) {
        const surface = (dll ? 'editor_conpty' : 'system_conpty') + (multiplexed ? '_tmux' : '');
        const config = path.join(path.dirname(output), 'config-' + surface + '-' + process.pid);
        fs.mkdirSync(config, {recursive: true});
        fs.writeFileSync(path.join(config, 'rc'), [
            'set show_all_tracks=true', 'set color_separator=red',
            'set color_trackwin_album_fg=default', 'set color_trackwin_album_bg=default',
            'set color_trackwin_album_attr=standout|reverse|bold',
        ].join('\n') + '\n');
        const env = {...process.env, CMUS_HOME: config, LANG:'C.UTF-8', LC_ALL:'C.UTF-8'};
        const label = 'cmus_album_' + process.pid + '_' + (dll ? 1 : 0);
        const control = (...args) => cp.execFileSync(tmux,['-u','-L',label,...args],
            {env,cwd:path.dirname(output),windowsHide:true,stdio:['ignore','pipe','pipe']});
        if (multiplexed) control('-f','NUL','new-session','-d','-s','check','-x','100','-y','30','"'+app+'"');
        const child = pty.spawn(multiplexed ? tmux : app,
            multiplexed ? ['-u','-L',label,'attach-session','-t','check'] : [], {cols: 100, rows: 30,
            env, cwd: path.dirname(output),
            useConpty: true, useConptyDll: dll});
        let bytes = '';
        const exited = new Promise(resolve => child.onExit(resolve));
        child.onData(text => {
            bytes += text;
            if (text.includes('\x1b[6n')) child.write('\x1b[1;1R');
            if (text.includes('\x1b[c')) child.write('\x1b[?1;2c');
        });
        const capture = async (phase, cols=100, rows=30) => {
            await pause(600);
            frames.push({surface, stream: surface, phase, cols, rows,
                bytes: Buffer.from(bytes).toString('base64')});
            bytes = '';
        };
        try {
            await pause(1000);
            child.write(':add ' + fixtures.replace(/\\/g, '/') + '\r1');
            await pause(1000);
            for (const [phase, text] of commands) {
                if (text) child.write(text);
                await capture(phase);
            }
            for (const [cols, rows] of [[80,24], [120,40], [100,30]]) {
                child.resize(cols, rows);
                await capture('resize', cols, rows);
            }
            child.write(':quit\r');
            await pause(500);
        } finally {
            child.kill();
            await Promise.race([exited, pause(2000)]);
            if (multiplexed) { try {control('kill-server');} catch (_) {} }
        }
    }
    fs.writeFileSync(output, frames.map(frame => JSON.stringify(frame)).join('\n') + '\n');
    process.exit(0);
})().catch(error => { fs.writeFileSync(output + '.error', String(error.stack)); process.exitCode=1; });
