/* Actual large-library commands/keys through system and editor ConPTY. */
const fs = require('fs');
const path = require('path');
const cp = require('child_process');
const [modulePath, app, music, output] = process.argv.slice(2);
const pty = require(modulePath);
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const results = [];
const forward = value => value.replace(/\\/g, '/');
const assert = (ok, message) => { if (!ok) throw Error(message); };

(async () => {
    for (const dll of [false, true]) {
        const surface = dll ? 'editor_conpty' : 'system_conpty';
        const dir = path.join(path.dirname(output), 'large-' + surface + '-' + process.pid);
        fs.mkdirSync(dir, {recursive:true});
        fs.writeFileSync(path.join(dir,'rc'), 'set softvol=true\n');
        const snapshot = path.join(dir, 'library.pl');
        const queue = path.join(dir, 'queue.pl');
        let child, exited, tail = '';
        const start = async () => {
            child = pty.spawn(app, [], {cols:100,rows:30,useConpty:true,useConptyDll:dll,
                cwd:dir,env:{...process.env,CMUS_HOME:dir,LANG:'C.UTF-8',LC_ALL:'C.UTF-8'}});
            exited = new Promise(resolve=>child.onExit(resolve));
            child.onData(text => {
                tail = (tail+text).slice(-4*1024*1024);
                if(text.includes('\x1b[6n'))child.write('\x1b[1;1R');
                if(text.includes('\x1b[c'))child.write('\x1b[?1;2c');
            });
            await pause(1000);
        };
        const stop = async () => {
            child.write(':quit\r');
            const value = await Promise.race([exited,pause(15000).then(()=>null)]);
            if (!value && fs.existsSync(path.join(path.dirname(output),'dump_test.exe'))) {
                cp.execFileSync(path.join(path.dirname(output),'dump_test.exe'),
                    [String(child.pid),path.join(path.dirname(output),'exit-'+surface+'.dmp')],
                    {timeout:10000,windowsHide:true});
            }
            assert(value && value.exitCode===0, 'clean exit failed: '+JSON.stringify(value));
            child = null;
        };
        const count = async expected => {
            const begin = Date.now();
            while(Date.now()-begin<120000) {
                if(fs.existsSync(snapshot))fs.unlinkSync(snapshot);
                child.write(':save -l '+forward(snapshot)+'\r');
                await pause(800);
                if(fs.existsSync(snapshot)) {
                    const lines=fs.readFileSync(snapshot,'utf8').trim().split(/\r?\n/).filter(Boolean);
                    if(lines.length===expected) {
                        assert(new Set(lines).size===expected,'duplicate exported paths');
                        return Date.now()-begin;
                    }
                }
                await pause(1000);
            }
            throw Error('library did not reach '+expected+' files');
        };
        const row={surface,passed:false};
        results.push(row);
        const checkpoint = step => {
            row.step=step;fs.writeFileSync(output,JSON.stringify(results,null,2)+'\n');
        };
        try {
            checkpoint('starting');
            await start();
            const begin=Date.now();
            child.write(':add -l '+forward(music)+'\r');
            await count(50000);
            row.import_ms=Date.now()-begin;
            checkpoint('imported');
            child.write('2/track_49999\r');
            await pause(500);
            child.write(':win-add-q\r:save -q '+forward(queue)+'\r');
            await pause(800);
            assert(fs.readFileSync(queue,'utf8').trim().endsWith('track_49999.wav'),'search/queue selection incorrect');
            for(const [cols,rows] of [[80,24],[120,40],[100,30]]) {
                child.resize(cols,rows);await pause(400);
                child.write('12');await pause(200);
            }
            child.write(':set shuffle=tracks\r:reshuffle\r');await pause(400);
            await count(50000);
            checkpoint('search/queue/resize/reshuffle passed');
            await stop();
            checkpoint('first exit passed');
            const restart=Date.now();
            await start();await count(50000);row.restart_ms=Date.now()-restart;
            checkpoint('restart passed');
            await stop();row.passed=true;
            row.checks=['50000 distinct-file import/export','search and queue exact last file',
                'resize/view switching','reshuffle','restart persistence','clean exits'];
        } catch(error) {row.error=String(error.stack);}
        finally {
            if(child){child.kill();await Promise.race([exited,pause(3000)]);}
            fs.writeFileSync(path.join(dir,'screen.txt'),tail);
            fs.writeFileSync(output,JSON.stringify(results,null,2)+'\n');
            console.log(JSON.stringify(row));
        }
    }
    process.exitCode=results.every(row=>row.passed)?0:1;
})().catch(error=>{console.error(error);process.exitCode=1;});
