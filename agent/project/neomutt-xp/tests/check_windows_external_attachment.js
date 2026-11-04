/* Production image mailcap; a test-only batch audit delegates real START. */
const fs = require('fs'), path = require('path'), crypto = require('crypto'), cp = require('child_process');
const [modulePath, app, root, production, phase] = process.argv.slice(2);
const pty = require(modulePath), pause = ms => new Promise(r => setTimeout(r, ms));
const posix = s => s.replace(/\\/g, '/');
(async () => {
  const nativePython = cp.execSync('python3 -c "import sys; print(sys.executable)"', {encoding:'utf8'}).trim();
  fs.mkdirSync(root, {recursive:true});
  const cwd=path.join(root,'snowfall'); fs.mkdirSync(cwd,{recursive:true});
  if (!fs.existsSync(path.join(root,'monochrome'))) fs.symlinkSync(production,path.join(root,'monochrome'),'junction');
  const trace=path.join(root,'opened.json'), wrong=path.join(root,'editor_called.txt'), audit=path.join(root,'audit');
  fs.mkdirSync(audit,{recursive:true});
  fs.writeFileSync(path.join(audit,'audit.py'),String.raw`import os,sys,pathlib,json,hashlib
p=pathlib.Path(sys.argv[1])
r={"path":str(p),"exists":p.is_file(),"sha256":hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None,"route":"native_batch_start"}
pathlib.Path(os.environ["OPENER_TRACE"]).write_text(json.dumps(r),encoding="utf-8")
sys.exit(0 if r["exists"] else 1)
`);
  fs.writeFileSync(path.join(audit,'xdg-open.bat'),[
    '@echo off','setlocal DisableDelayedExpansion','echo arrived > "%OPENER_TRACE%.batch"',
    `"${nativePython}" -X utf8 "%~dp0audit.py" "%~1"`,
    'if errorlevel 1 exit /b %errorlevel%',
    `"${path.join(production,'bin','xdg-open.bat')}" %*`,''
  ].join('\r\n'));
  const editor=path.join(root,'capture_editor');
  fs.writeFileSync(editor,'#!/usr/bin/env bash\nprintf called > "$OPENER_EDITOR_MARKER"\n');
  const image=Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aQ4sAAAAASUVORK5CYII=','base64');
  const box=path.join(root,'fixture.maildir'); for(const d of ['cur','new','tmp']) fs.mkdirSync(path.join(box,d),{recursive:true});
  const file=path.join(box,'cur','image;2,S');
  const raw='From: fixture@example.invalid\nTo: reader@example.invalid\nSubject: Image opener fixture\nMIME-Version: 1.0\nContent-Type: image/png\nContent-Disposition: attachment; filename="image space 日本語 & %PATH% !HOME!.png"\nContent-Transfer-Encoding: base64\n\n'+image.toString('base64')+'\n';
  fs.writeFileSync(file,raw);
  const rc=path.join(root,'test.rc');
  fs.writeFileSync(rc,'set folder="." crypt_use_gpgme=no quit=yes wait_key=no sleep_time=0 maildir_field_delimiter=";"\nset mailcap_path="'+posix(path.join(production,'module/email/mailcap'))+'"\n');
  const env={...process.env,TERM:'xterm-256color',EDITOR:posix(editor),OPENER_REAL_BATCH:path.join(production,'bin/xdg-open.bat'),OPENER_TRACE:trace,OPENER_EDITOR_MARKER:wrong};
  for(const key of Object.keys(env)) if(key.toLowerCase()==='path') delete env[key];
  env.Path=audit+';'+process.env.PATH;
  let output='',exit=null;
  const child=pty.spawn(app,['-n','-R','-F',rc,'-f',box],{cols:120,rows:40,cwd,env,useConpty:true,useConptyDll:true});
  child.onData(s=>{output+=s;if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});
  child.onExit(e=>exit=e.exitCode);
  async function until(token){let start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<12000)await pause(50);if(!output.includes(token))throw Error('Missing '+token);}
  let result;
  try {
    await until('Image opener fixture'); child.write('v'); await until('Attachments');
    child.write(':exec last-entry\r'); await pause(300); output=''; child.write('\r');
    let start=Date.now(); while(!fs.existsSync(trace)&&!fs.existsSync(wrong)&&Date.now()-start<12000)await pause(50);
    const record=fs.existsSync(trace)?JSON.parse(fs.readFileSync(trace)):null;
    result={phase,passed:!!record&&record.exists&&record.sha256===crypto.createHash('sha256').update(image).digest('hex')&&!fs.existsSync(wrong),wrong_editor_called:fs.existsSync(wrong),audit_batch_entered:fs.existsSync(trace+'.batch'),record,original_message_unchanged:fs.readFileSync(file,'utf8')===raw};
    const commandFailed=output.includes('Press any key');if(commandFailed)child.write(' ');
    for(let i=0;i<4&&exit===null;i++){child.write('q');await pause(400);}
    if(exit!==0)throw Error('Exit '+exit);
    result.native_start_returned_without_error=!commandFailed;result.passed=result.passed&&result.original_message_unchanged&&!commandFailed;
  } catch(e) {result={...result,passed:false,error:String(e)};}
  finally {fs.writeFileSync(path.join(root,'terminal.vt'),output);if(exit===null)child.kill();}
  fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result,null,2));process.exit(result.passed?0:1);
})();
