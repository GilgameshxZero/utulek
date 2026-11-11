/* Actual Windows IMAP UI with an independent Python server-state observer.
 * Owned loopback fixtures only. SPDX-License-Identifier: GPL-2.0-or-later */
const fs=require('fs'),path=require('path'),cp=require('child_process');
const [ptyModule,terminalModule,python,app,root,connectionFile]=process.argv.slice(2);
const pty=require(ptyModule),{Terminal}=require(terminalModule);
const connection=JSON.parse(fs.readFileSync(connectionFile,'utf8'));
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
const posix=p=>p.replace(/\\/g,'/');
function control(phase){
 const result=cp.spawnSync(python,[path.join(__dirname,'imap_fixture_control.py'),connectionFile,phase],{encoding:'utf8',windowsHide:true});
 if(result.status!==0)throw Error('Independent '+phase+' check: '+result.stderr);
}
async function check(bundled){
 control('seed');
 const scratch=fs.mkdtempSync(path.join(root,bundled?'editor-':'system-'));
 const url='imaps://localhost:'+connection.port+'/';
 const rc=path.join(scratch,'test.rc');
 fs.writeFileSync(rc,'set folder="'+url+'" imap_user="fixture@example.invalid" imap_pass="owned-imap-fixture-password"\n'+
  'set imap_authenticators=login ssl_force_tls=yes ssl_use_system_certs=no certificate_file="'+posix(connection.cert)+'"\n'+
  'set ssl_verify_host=yes ssl_verify_dates=yes socket_timeout=3 charset="utf-8" sort=unsorted use_threads=no\n'+
  'set quit=yes delete=yes move=no mark_old=no wait_key=no sleep_time=0 confirm_create=no confirm_append=no timeout=1 mail_check=1\nunset record\nunset beep\n');
 let current=null;const phases=[];
 function launch(){
  const t=new Terminal({cols:120,rows:40,allowProposedApi:true});let output='',exit=null;
  const child=pty.spawn(app,['-n','-F',rc,'-f',url+(connection.mailbox||'INBOX')],{cols:120,rows:40,cwd:scratch,
   env:{...process.env,HOME:scratch,USERPROFILE:scratch,TMPDIR:scratch,TERM:'xterm-256color',LANG:'C.UTF-8'},useConpty:true,useConptyDll:bundled});
  child.onData(data=>{output+=data;t.write(data);if(data.includes('\x1b[6n'))child.write('\x1b[1;1R');if(data.includes('\x1b[c'))child.write('\x1b[?1;2c');});
  child.onExit(e=>exit=e.exitCode);
  const screen=()=>Array.from({length:40},(_,i)=>t.buffer.active.getLine(t.buffer.active.viewportY+i)?.translateToString(true)||'').join('\n');
  return{send:s=>child.write(s),screen,
   until:async token=>{const end=Date.now()+15000;while(!screen().includes(token)&&exit===null&&Date.now()<end)await pause(50);if(!screen().includes(token))throw Error('Missing '+token+': '+screen().slice(-1300));},
   quit:async()=>{child.write('q');for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit!==0)throw Error('Unclean exit '+exit);},
   close:()=>{if(exit===null)child.kill();t.dispose();},
   capture:()=>fs.writeFileSync(path.join(scratch,'failure.vt'),output)};
 }
 try{
  current=launch();await current.until('IMAP fixture 1');current.send(':exec first-entry\r');await pause(150);
  current.send('\r');await current.until('Owned IMAP body 1');current.send('q');await pause(150);
  if(connection.interrupt){
   const kind=connection.interruption_kind||'flag';
   if(kind==='uid_shift'){current.send('/IMAP fixture 2\r');await pause(200);current.send('\r');await current.until('Owned IMAP body 2');current.send('q');await pause(150);}
   current.send(kind==='delete'?'d':'F');await pause(150);control('drop_network');current.send('$');await pause(800);
   const diagnostic=current.screen();
   if(/Reconnected to .*failed to reopen/.test(diagnostic))throw Error('Failed login was reported as reconnected');
   if(!diagnostic.includes('IMAP fixture 1'))throw Error('Offline failure discarded the visible mailbox');
   if(kind==='uid_shift')control('shift_uid');
   if(kind==='uidvalidity')control('replace_identity');
   control('resume_network');current.send('$');
   const phase={flag:'read_flag',delete:'recovered_delete',uid_shift:'recovered_shift',uidvalidity:'recovered_identity'}[kind];
   let verified=false;const deadline=Date.now()+12000;
   if(kind==='uidvalidity'){await current.until('identity changed');control(phase);phases.push('mailbox_identity_guard');return{surface:bundled?'editor_conpty':'system_conpty',kind,passed:true,phases,new_identity_not_modified:true};}
   while(Date.now()<deadline){await pause(250);try{control(phase);verified=true;break;}catch{}}
   if(!verified)control(phase);
   await current.quit();phases.push('pending_flag_survived_connection_failure_and_retry');
   return{surface:bundled?'editor_conpty':'system_conpty',kind,passed:true,phases,original_bytes_preserved:true};
  }
  current.send('F$');await pause(400);control('read_flag');phases.push('read_and_flag_verified_on_server');
  current.send('/IMAP fixture 1\r');await pause(200);current.send('F');await pause(150);control('external_reply');await pause(connection.concurrent_idle_delay_ms||0);current.send('$');await pause(400);control('merged_unflag');
  current.send('/IMAP fixture 1\r');await pause(200);current.send('F$');await pause(400);control('merged_flag');phases.push('same_message_local_and_external_flag_deltas_preserved');
  current.send('/IMAP fixture 2\r');await pause(200);current.send('s');await current.until('Save to mailbox');
  current.send('\x15'+url+'Archive\r');await pause(400);current.send('$');await pause(400);control('move');phases.push('move_verified_on_server');
  current.send('/IMAP fixture 3\r');await pause(200);current.send('d$');await pause(400);control('delete');phases.push('delete_expunge_verified_on_server');
  control('append_new');current.send('$');await current.until('IMAP fixture 4');current.send('/IMAP fixture 4\r');await pause(200);
  current.send('\r');await current.until('Owned IMAP body 4');current.send('q');await pause(150);phases.push('new_delivery_visible_and_readable');
  await current.quit();current.close();current=launch();await current.until('IMAP fixture 4');
  current.send('/IMAP fixture 1\r');await pause(200);current.send('\r');await current.until('Owned IMAP body 1');current.send('q');await pause(150);
  await current.quit();control('final');phases.push('restart_preserves_server_state_and_original_bytes');
  return{surface:bundled?'editor_conpty':'system_conpty',passed:true,phases};
 }catch(error){if(current)current.capture();return{surface:bundled?'editor_conpty':'system_conpty',passed:false,phases,error:String(error)};}
 finally{if(connection.interrupt){try{control('resume_network');}catch{}}if(current)current.close();}
}
(async()=>{fs.mkdirSync(root,{recursive:true});const results=[];for(const bundled of [false,true]){results.push(await check(bundled));fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results.at(-1)));}process.exit(results.every(r=>r.passed)?0:1);})().catch(e=>{console.error(e.message);process.exitCode=1});
