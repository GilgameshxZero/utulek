/* Exercise embedded defaults/MIME lookup from a directory containing only the EXE.
 * All data is synthetic, and no mail is submitted. */
const fs=require('fs'),path=require('path'),cp=require('child_process');
const [ptyModule,app,root,parser,editor]=process.argv.slice(2);
const pty=require(ptyModule),{Terminal}=require(parser),pause=ms=>new Promise(r=>setTimeout(r,ms));
const posix=s=>s.replace(/\\/g,'/');
const run=(args,options)=>new Promise((resolve,reject)=>cp.execFile(app,args,{...options,encoding:'utf8',timeout:20000},(error,stdout,stderr)=>error?reject(Error(String(error)+' '+stdout+' '+stderr)):resolve(stdout+stderr)));
async function check(bundled,override){
 const dir=fs.mkdtempSync(path.join(root,override?'override-':'embedded-'));
 const env={...process.env,HOME:dir,USERPROFILE:dir,TMPDIR:dir,TEMP:dir,TMP:dir,TERM:'xterm-256color',LANG:'C.UTF-8'};delete env.NEOMUTT_XP_CRYPTO_DIR;
 const box=path.join(dir,'inbox.maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
 fs.writeFileSync(path.join(box,'cur','fixture;2,S'),'From: sender@example.invalid\nTo: reader@example.invalid\nSubject: Single executable fixture\n\nOwned test message\n');
 const pdf=path.join(dir,override?'report.fixturepdf':'report.pdf'),ics=path.join(dir,'invitation.ics');fs.writeFileSync(pdf,'%PDF-1.7\nOwned fixture\n');fs.writeFileSync(ics,'BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR\r\n');
 if(override)fs.writeFileSync(path.join(dir,'.mime.types'),'application/x-fixture-pdf fixturepdf\n');
 const rc=path.join(dir,'test.rc');fs.writeFileSync(rc,'set folder="." from="reader@example.invalid" crypt_use_gpgme=no maildir_field_delimiter=";" charset="utf-8" quit=yes postpone=no recall=no abort_unmodified=no wait_key=no sleep_time=0 confirm_create=no confirm_append=no mbox_type=mbox\nunset record\nset editor="'+posix(editor)+'"\nmacro compose Z "<write-fcc><kill-line>observed.mbox<enter>"\n');
 const baseline=await run(['-F',rc,'-Q','time_inc'],{cwd:dir,env});
 if(!baseline.includes('time_inc = 250'))throw Error('Embedded baseline was not loaded: '+baseline);
 const skipped=await run(['-n','-F',rc,'-Q','time_inc'],{cwd:dir,env});
 if(skipped.includes('time_inc = 250'))throw Error('-n did not skip embedded baseline: '+skipped);
 const t=new Terminal({cols:120,rows:40,allowProposedApi:true});let exit=null,lastData=0;
 const child=pty.spawn(app,['-F',rc,'-f',box],{cols:120,rows:40,cwd:dir,env,useConpty:true,useConptyDll:bundled});
 child.onData(s=>{lastData=Date.now();t.write(s);if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 const screen=()=>Array.from({length:t.rows},(_,y)=>t.buffer.active.getLine(y)?.translateToString(true)||'').join('\n');
 async function until(token){const deadline=Date.now()+20000;while(Date.now()<deadline&&exit===null){if(screen().includes(token)&&Date.now()-lastData>100)return;await pause(30);}throw Error('Missing '+token+' '+screen().slice(-1500));}
 try{
  await until('Single executable fixture');child.write('m');await until('To:');child.write('recipient@example.invalid\r');await until('Subject:');child.write('Owned attachment draft\r');await until('Attachments');
  for(const file of [pdf,ics]){child.write('a');await until('Attach file');child.write(posix(file)+'\r');await pause(300);await until('Attachments');}
  child.write('Z');const saved=path.join(dir,'observed.mbox');for(let i=0;i<100&&!fs.existsSync(saved);i++)await pause(50);
  const message=fs.readFileSync(saved,'utf8');const expected=override?'application/x-fixture-pdf':'application/pdf';
  if(!message.includes('Content-Type: '+expected)||!message.includes('Content-Type: text/calendar'))throw Error('Embedded MIME lookup or user override failed');
  child.write('q');await until('Mail not sent');child.write('q');for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit!==0)throw Error('Unclean exit '+exit);
  if(fs.readdirSync(dir).some(name=>name.startsWith('neomutt-resource-')))throw Error('Embedded resource temporary file remains');
  return {surface:bundled?'editor_conpty':'system_conpty',user_mime_extension:override,passed:true};
 }finally{if(exit===null)child.kill();t.dispose();}
}
(async()=>{
 fs.mkdirSync(root,{recursive:true});const siblings=fs.readdirSync(path.dirname(app));if(siblings.length!==1||siblings[0]!==path.basename(app))throw Error('Stage exactly one executable in its isolated directory');
 const results=[];for(const bundled of [false,true])for(const override of [false,true]){try{results.push(await check(bundled,override));}catch(error){results.push({surface:bundled?'editor_conpty':'system_conpty',user_mime_extension:override,passed:false,error:String(error)});}fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(results,null,2));}
 console.log(JSON.stringify(results,null,2));process.stdout.write('',()=>process.exit(results.every(r=>r.passed)?0:1));
})().catch(error=>{console.error(String(error));process.exitCode=1;});
