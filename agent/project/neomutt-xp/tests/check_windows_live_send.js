/* Explicitly authorized live send harness: hard recipient/sender allowlist.
 * Never use fixture addresses or additional recipients for live submissions. */
const fs=require('fs'),path=require('path');
const [modulePath,app,rc,root,from,to,subject,attachment]=process.argv.slice(2),pty=require(modulePath);
const authorization=JSON.parse(fs.readFileSync(path.join(path.dirname(rc),'live_authorization.json'),'utf8'));
if(!authorization.allowed_senders.includes(from)||!authorization.allowed_recipients.includes(to))throw Error('Live address authorization refused');
const pause=ms=>new Promise(r=>setTimeout(r,ms)),posix=p=>p.replace(/\\/g,'/');
(async()=>{
 const box=path.join(root,'empty.maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
 fs.writeFileSync(path.join(box,'cur','owned;2,S'),'From: '+from+'\nTo: '+to+'\nSubject: Owned navigation fixture\n\nOwned navigation body\n');
 const proof=path.join(root,'guard.maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(proof,sub),{recursive:true});
 const extra='set from="'+from+'" envelope_from_address="'+from+'" use_envelope_from=yes editor="'+posix(path.join(path.dirname(app),'neomutt-xp-editor.exe'))+'" quit=yes postpone=no recall=no sleep_time=0; cd \"'+posix(root)+'\"';
 const child=pty.spawn(app,['-n','-F',rc,'-f',box,'-e',extra],{cols:120,rows:40,cwd:root,env:{...process.env,TERM:'xterm-256color'},useConpty:true,useConptyDll:true});
 let output='',exit=null;const events=[];
 const progressTimer=setInterval(()=>fs.writeFileSync(path.join(root,'progress.vt'),output),500);
 child.onData(s=>{output+=s;if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 async function until(token,timeout=20000){const start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<timeout){if(token==='Mail sent'&&/authentication failed|Could not send|Message not sent/i.test(output))throw Error('Submission failed before completion');await pause(50);}if(!output.includes(token))throw Error('Missing '+token+'; '+JSON.stringify(output.slice(-1000)));}
 let passed=false,error=null;
 try{
  await until('NeoMutt:');output='';child.write('y');await until('Mailboxes');events.push('index_y_mailboxes');child.write('y');await pause(700);
  output='';child.write('\r');await until('Owned navigation body');output='';child.write('y');await until('Mailboxes');events.push('pager_y_mailboxes');child.write('y');await pause(700);
  output='';child.write('m');await until('To:');child.write(to+'\r');await until('Subject:');child.write(subject+'\r');await until('Attachments');await pause(800);
  if(attachment==='generate-calendar'){
   output='';child.write('R');await until('Subject [');child.write('\r');await until('Start (YYYY-MM-DD HH:MM):');child.write('2030-05-01 10:00\r');await until('Duration (');output='';child.write('1h\r');await until('Attachments');await pause(1200);
   const generated=fs.readFileSync(path.join(root,'.request.tmp.ics'),'utf8');
   for(const expected of ['METHOD:REQUEST','mailto:'+from,'mailto:'+to,'SUMMARY:'+subject])if(!generated.includes(expected))throw Error('Generated calendar missing '+expected);
   fs.writeFileSync(path.join(root,'generated_request.ics'),generated);events.push('actual_calendar_request_macro');
  }else if(attachment&&attachment!=='none'){output='';child.write('a');await until('Attach file');child.write('\x15'+attachment+'\r');await pause(800);}
  if(process.argv[10]){
   for(const [key,value] of [['PASS','smtp_pass'],['SOURCE','my_smtp_pass_gilgamesh_cc'],['OAUTH','smtp_oauth_refresh_command'],['URL','smtp_url']]){
    child.write(':setenv NEOMUTT_PROBE_'+key+'="$'+value+'"\r');await pause(250);
   }
   output='';child.write(':exec shell-escape\r');await until('Shell command:');
   child.write('python3 -X utf8 "'+posix(process.argv[10])+'" "'+posix(path.join(root,'credential_probe.json'))+'"\r');await until('Credential comparison complete');await pause(1000);
   for(const key of ['PASS','SOURCE','OAUTH','URL']){child.write(':unsetenv NEOMUTT_PROBE_'+key+'\r');await pause(200);}
  }
  output='';child.write(':exec write-fcc\r');await until('Write message to mailbox');child.write('\x15'+posix(proof)+'\r');await pause(1000);
  let files=[];for(const d of ['cur','new'])files.push(...fs.readdirSync(path.join(proof,d)).map(n=>path.join(proof,d,n)));
  if(files.length!==1)throw Error('Expected exactly one serialized draft for authorization check');
  const raw=fs.readFileSync(files[0],'utf8'),head=raw.split(/\r?\n\r?\n/)[0].replace(/\r?\n[ \t]+/g,' ');
  const field=n=>(head.match(new RegExp('^'+n+': *(.*)$','im'))||[])[1]||'';
  const addresses=s=>s.match(/[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9.-]+/ig)||[];
  if(attachment&&attachment!=='none'&&!raw.includes(attachment==='generate-calendar'?'.request.tmp.ics':'test_request.ics'))throw Error('Expected calendar attachment is missing');
  if(JSON.stringify(addresses(field('From')))!==JSON.stringify([from])||JSON.stringify(addresses(field('To')))!==JSON.stringify([to])||field('Cc')||field('Bcc'))throw Error('Serialized message violates live address authorization');
  events.push('serialized_address_guard_passed');
  fs.writeFileSync(path.join(root,'authorized_headers.json'),JSON.stringify({from,to,subject,message_id:field('Message-ID'),attachment:attachment||null},null,2));
  output='';child.write('y');await until('Mail sent',90000);events.push('mail_sent');await pause(1000);child.write('q');
  for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit!==0)throw Error('Unclean exit '+exit);passed=true;
 }catch(e){error=String(e);}finally{clearInterval(progressTimer);fs.writeFileSync(path.join(root,'terminal.vt'),output);if(exit===null)child.kill();}
 const report={passed,from,to,subject,events,exit,error};fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));process.exit(passed?0:1);
})();
