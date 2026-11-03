/* Actual NeoMutt keyboard workflows in Windows ConPTY.
 * Run via an existing Electron Node runtime. No user mail/config is loaded.
 * SPDX-License-Identifier: GPL-2.0-or-later */
const fs = require('fs');
const path = require('path');
const [modulePath, app, root] = process.argv.slice(2);
const pty = require(modulePath);
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
fs.mkdirSync(root, {recursive: true});

async function check(bundled) {
  const scratch = fs.mkdtempSync(path.join(root, bundled ? 'editor space & % ! 日本語-' : 'system space & % ! 日本語-'));
  const mailbox = path.join(scratch, 'inbox.maildir');
  for (const dir of ['cur', 'new', 'tmp']) fs.mkdirSync(path.join(mailbox, dir), {recursive: true});
  for (let i = 1; i <= 3; i++) {
    fs.writeFileSync(path.join(mailbox, 'cur', `fixture${i};2,S`),
      `From: Sender <sender@example.invalid>\nTo: Reader <reader@example.invalid>\n` +
      `Date: Tue, 03 Nov 2026 00:00:00 +0000\nMessage-ID: <fixture${i}@example.invalid>\n` +
      `Subject: Windows fixture ${i}\nMIME-Version: 1.0\nContent-Type: text/plain; charset=utf-8\n` +
      `Content-Transfer-Encoding: 8bit\n\nfixture body ${i}\nUnicode: Grüße 日本語\n`);
  }
  const archive = path.join(scratch, 'archive.maildir');
  const postponed = path.join(scratch, 'postponed.maildir');
  const subjectBox=path.join(scratch,'subject-check.maildir');
  for (const box of [archive,postponed,subjectBox]) for(const sub of ['cur','new','tmp']) fs.mkdirSync(path.join(box,sub),{recursive:true});
  const posix = p => p.replace(/\\/g, '/');
  const rc = path.join(scratch, 'test.rc');
  fs.writeFileSync(rc, 'set folder="."\nset maildir_field_delimiter=";"\nset mbox_type=Maildir\n' +
    'set charset="utf-8"\nset realname="Fixture Reader"\nset from="reader@example.invalid"\n' +
    'set sort=unsorted\nset quit=yes\nset delete=yes\nset move=no\nset mark_old=no\n' +
    'set read_inc=1\nset write_inc=1\nset timeout=1\nset sleep_time=0\n' +
    'bind editor <space> noop\nunset record\nunset beep\nset arrow_cursor=yes\nset postpone=no\n' +
    'set confirm_append=no\nset postponed=\"' + posix(postponed) + '\"\n' +
    'set header_cache=\"' + posix(path.join(scratch,'cache')) + '/\"\n' +
    'set editor=\"' + path.join(path.dirname(app), 'neomutt-xp-editor.exe').replace(/\\/g, '/') + '\"\n');
  const env = {...process.env, HOME:scratch, USERPROFILE:scratch, TMPDIR:scratch,
               TERM:'xterm-256color', LANG:'C.UTF-8'};
  let child = pty.spawn(app, ['-n', '-F', rc, '-f', mailbox], {
    cols:100, rows:30, cwd:scratch, env, useConpty:true, useConptyDll:bundled,
  });
  let output = '', exit = null;
  function wire(current) {
    current.onData(data => {
      output += data;
      if (data.includes('\x1b[6n')) current.write('\x1b[1;1R');
      if (data.includes('\x1b[c')) current.write('\x1b[?1;2c');
    });
    current.onExit(event => { exit = event.exitCode; });
  }
  wire(child);
  const phases = [];
  const capture = name => {
    phases.push({name, output});
    fs.writeFileSync(path.join(scratch, name + '.vt'), output);
  };
  async function until(token, timeout=10000) {
    const start=Date.now();
    while (!output.includes(token) && exit === null && Date.now()-start < timeout) await pause(50);
    if (!output.includes(token)) throw Error(`Missing ${token}; exit=${exit}; tail=${JSON.stringify(output.slice(-1600))}`);
  }
  async function writeDraft(name) {
    const box=path.join(scratch,name);
    for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
    output='';child.write(':exec write-fcc\r');await until('Write message to mailbox');
    child.write('\x15'+name+'\r');
    const start=Date.now();let files=[];
    while(!files.length&&Date.now()-start<5000){
      files=[...fs.readdirSync(path.join(box,'cur')).map(n=>path.join(box,'cur',n)),
             ...fs.readdirSync(path.join(box,'new')).map(n=>path.join(box,'new',n))];
      if(!files.length)await pause(50);
    }
    if(files.length!==1)throw Error('Compose write-fcc failed: '+name);
    return fs.readFileSync(files[0],'utf8');
  }
  function messageBody(raw) {
    raw=raw.replace(/\r\n/g,'\n');const split=raw.indexOf('\n\n');
    const headers=raw.slice(0,split),body=raw.slice(split+2);
    if(/Content-Transfer-Encoding: base64/i.test(headers))return Buffer.from(body,'base64').toString('utf8');
    if(/Content-Transfer-Encoding: quoted-printable/i.test(headers)){
      const value=body.replace(/=\n/g,'');const bytes=[];
      for(let i=0;i<value.length;i++){if(value[i]==='='&&/^[0-9a-f]{2}$/i.test(value.slice(i+1,i+3))){bytes.push(parseInt(value.slice(i+1,i+3),16));i+=2;}else bytes.push(value.charCodeAt(i));}
      return Buffer.from(bytes).toString('utf8');
    }
    return body;
  }
  try {
    await until('Windows fixture'); capture('index');
    child.write('\r'); await until('fixture body'); capture('pager');
    child.resize(80,24); await pause(800); capture('resize_small');
    child.resize(120,40); await pause(800); capture('resize_large');
    child.write('q'); await pause(300);
    child.write('/Windows fixture 3\r'); await pause(400); capture('search');
    child.write('\r'); await until('fixture body 3'); capture('search_pager');
    child.write('q'); await pause(200);
    const marker = path.join(scratch, 'editor-called.txt');
    if (fs.existsSync(marker)) fs.unlinkSync(marker);
    output = ''; child.write('m'); await until('To:');
    child.write('recipient@example.invalid\r'); await until('Subject:');
    child.write('native fixture draft Grüße 日本語\r');
    const editedAt=Date.now();
    while(!fs.existsSync(marker) && Date.now()-editedAt<10000 && exit===null) await pause(50);
    if(!fs.existsSync(marker)) throw Error('Editor did not run: '+JSON.stringify(output.slice(-1500)));
    await until('Attachments');
    capture('compose_editor');
    const savedRaw=await writeDraft('subject-check.maildir');
    const rawSubject=(savedRaw.replace(/\r?\n[ \t]+/g,' ').match(/^Subject: (.*)$/m)||[])[1]||'';
    const decoded=rawSubject.replace(/\?=\s+(?==\?)/g,'?=').replace(/=\?utf-8\?([bq])\?([^?]*)\?=/gi,(_,type,value)=>{
      if(type.toLowerCase()==='b')return Buffer.from(value,'base64').toString('utf8');
      const bytes=[];value=value.replace(/_/g,' ');for(let i=0;i<value.length;i++){if(value[i]==='='&&/^[0-9a-f]{2}$/i.test(value.slice(i+1,i+3))){bytes.push(parseInt(value.slice(i+1,i+3),16));i+=2;}else bytes.push(value.charCodeAt(i));}
      return Buffer.from(bytes).toString('utf8');
    });
    if(decoded.trim()!=='native fixture draft Grüße 日本語')throw Error('Unicode subject payload changed: '+decoded);
    capture('compose_write_fcc');
    child.write('q'); await pause(500); capture('discard');
    // The selected third message is flagged, persisted, then purged through keys.
    child.write('F'); await pause(200); child.write('$'); await pause(500);
    if(!fs.readdirSync(path.join(mailbox,'cur')).some(n=>n.startsWith('fixture3;') && n.split(',')[1].includes('F')))
      throw Error('Flag did not persist');
    capture('flag_persisted');
    child.write('/Windows fixture 3\r'); await pause(250);
    child.write('d'); await pause(200); child.write('$'); await pause(500);
    if(fs.readdirSync(path.join(mailbox,'cur')).length!==2) throw Error('Delete/purge failed');
    capture('delete_persisted');
    child.write(':exec first-entry\r'); await pause(150); child.write('tt'); await pause(150);
    output=''; child.write(';s'); await until('Save tagged to mailbox');
    child.write('\x15'+posix(archive)+'\r'); await pause(500); child.write('$'); await pause(500);
    const archived=[...fs.readdirSync(path.join(archive,'cur')).map(n=>path.join(archive,'cur',n)),
                    ...fs.readdirSync(path.join(archive,'new')).map(n=>path.join(archive,'new',n))];
    if(archived.length!==2 || fs.readdirSync(path.join(mailbox,'cur')).length!==0) throw Error('Tagged save/move failed');
    const contents=archived.map(f=>fs.readFileSync(f,'utf8')).join('\n');
    if(!contents.includes('fixture body 1') || !contents.includes('fixture body 2')) throw Error('Moved message body changed');
    capture('tagged_move');
    output=''; child.write('c'); await until('Open mailbox');
    child.write('\x15'+posix(archive)+'\r'); await until('Windows fixture'); capture('archive_open');
    // Explicit postponement must survive exit/restart; the file is checked here.
    output=''; child.write('m'); await until('To:');
    child.write('recipient@example.invalid\r'); await until('Subject:');
    child.write('postponed fixture draft\r'); await until('Attachments');
    child.write('P'); await pause(700);
    const drafts=[...fs.readdirSync(path.join(postponed,'cur')),...fs.readdirSync(path.join(postponed,'new'))];
    if(drafts.length!==1) throw Error('Postponement did not persist');
    capture('postponed');
    child.write('q');
    const start=Date.now(); while(exit===null && Date.now()-start<5000) await pause(50);
    if(exit!==0) throw Error(`Unclean exit ${exit}; tail=${JSON.stringify(output.slice(-1200))}`);
    capture('first_exit');
    exit=null; output='';
    child=pty.spawn(app,['-n','-F',rc,'-p'],{cols:100,rows:30,cwd:scratch,env,useConpty:true,useConptyDll:bundled});
    wire(child);
    await until('Attachments');
    capture('restart_recall');
    const recalledRaw=await writeDraft('recall-check.maildir');
    if(!recalledRaw.includes('postponed fixture draft') || !messageBody(recalledRaw).includes('Editor fixture: Grüße 日本語'))throw Error('Recalled serialized payload changed');
    child.write('q');
    const recalledAt=Date.now(); while(exit===null && Date.now()-recalledAt<5000) await pause(50);
    // Upstream returns 1 for cancellation of the explicit -p compose action.
    if(exit!==1) throw Error('Recall/discard exit differs from upstream: '+exit);
    if([...fs.readdirSync(path.join(postponed,'cur')),...fs.readdirSync(path.join(postponed,'new'))].length!==0)
      throw Error('Discarded recalled draft remains');
    capture('restart_discard');
    return {surface:bundled?'editor_conpty':'system_conpty', passed:true, directory:path.basename(scratch), exit:0,recall_exit:exit,phases:phases.map(p=>p.name)};
  } finally {
    capture('final');
    if(exit===null) child.kill();
  }
}
(async()=>{
  const results=[];
  for(const bundled of [false,true]) {
    try { results.push(await check(bundled)); }
    catch(error) { results.push({surface:bundled?'editor_conpty':'system_conpty',passed:false,error:String(error)}); }
  }
  fs.writeFileSync(path.join(root,'results.json'), JSON.stringify(results,null,2));
  console.log(JSON.stringify(results,null,2));
  process.stdout.write('',()=>process.exit(results.every(r=>r.passed)?0:1));
})();
