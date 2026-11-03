/* Exercise calendar reply/request macros through the real Windows UI.
 * Synthetic invitations only; never send a message. GPL-2.0-or-later */
const fs=require('fs'),path=require('path'),pty=require(process.argv[2]);
const [app,root,helpers]=process.argv.slice(3);const pause=ms=>new Promise(r=>setTimeout(r,ms));
const posix=p=>p.replace(/\\/g,'/');
async function check(bundled,state){
 const scratch=fs.mkdtempSync(path.join(root,'calendar-'+(bundled?'editor-':'system-')+state+'-'));
 const box=path.join(scratch,'inbox.maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
 const ics='BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:fixture-reply@example.invalid\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\nSUMMARY:Calendar macro fixture\r\nORGANIZER:mailto:sender@example.invalid\r\nATTENDEE;RSVP=TRUE:mailto:reader@example.invalid\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n';
 fs.writeFileSync(path.join(box,'cur','calendar;2,S'),'From: Sender <sender@example.invalid>\nTo: Reader <reader@example.invalid>\nDate: Tue, 03 Nov 2026 00:00:00 +0000\nSubject: Calendar macro fixture\nMIME-Version: 1.0\nContent-Type: text/calendar; charset=utf-8; method=REQUEST\n\n'+ics);
 const mailcap=path.join(scratch,'mailcap');fs.writeFileSync(mailcap,'text/calendar; python3 -X utf8 "'+posix(path.join(helpers,'mutt-ical.py'))+'" -D %s; copiousoutput; compose=true\n');
 const editor=posix(path.join(path.dirname(app),'neomutt-xp-editor.exe'));
 const post='<new-mime>.reply.tmp.ics<enter>text/calendar<enter><shell-escape>python3 -X utf8 "'+posix(path.join(helpers,'mutt-ical.py'))+'" -i -g .reply.tmp.ics .event.tmp.ics<enter><shell-escape>rm .event.tmp.ics<enter><last-entry><update-encoding>';
 const request='<write-fcc><kill-line>.compose.tmp.mbox<enter><new-mime>.request.tmp.ics<enter>text/calendar<enter><shell-escape>\"'+posix(path.join(helpers,'neomutt-ical-request'))+'\" .compose.tmp.mbox<enter><last-entry><update-encoding>';
 const macro='<pipe-entry>iconv -c --to-code=UTF-8 > .event.tmp.ics<enter><reply>'+post;
 const quote=s=>s.replace(/\\/g,'\\\\').replace(/"/g,'\\"');
 const rc=path.join(scratch,'test.rc');fs.writeFileSync(rc,'set folder="."\nset from="reader@example.invalid"\nset realname="Fixture Reader"\n'+
  'set maildir_field_delimiter=";"\nset charset="utf-8"\nset quit=yes\nset delete=yes\nset move=no\nset fast_reply=yes\nset include=yes\nset postpone=no\nset wait_key=no\nset confirm_create=no\nset confirm_append=no\nset sleep_time=0\n'+
  'unset record\nunset beep\nbind editor <space> noop\nset editor="'+editor+'"\nset mailcap_path="'+posix(mailcap)+'"\nauto_view text/calendar\n'+
  'send2-hook "~f .*" "setenv NEOMUTT_FROM=\'$from\'"\nmacro attach r "'+quote(macro)+'"\nmacro compose R "'+quote(request)+'"\n');
 const env={...process.env,HOME:scratch,USERPROFILE:scratch,TMPDIR:scratch,TERM:'xterm-256color',TZ:'UTC',LANG:'C.UTF-8'};
 const child=pty.spawn(app,['-n','-F',rc,'-f',box],{cols:120,rows:40,cwd:scratch,env,useConpty:true,useConptyDll:bundled});
 let output='',exit=null;const phases=[];
 child.onData(data=>{output+=data;if(data.includes('\x1b[6n'))child.write('\x1b[1;1R');if(data.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 async function until(token){const start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<15000)await pause(50);if(!output.includes(token))throw Error('Missing '+token+'; '+JSON.stringify(output.slice(-1600)));}
 const capture=name=>{fs.writeFileSync(path.join(scratch,name+'.vt'),output);phases.push(name);};
 try{
  await until('Calendar macro fixture');
  if(state==='request'){
   output='';child.write('m');await until('To:');child.write('target@example.invalid\r');await until('Subject:');child.write('Request Grüße 日本語\r');
   await until('Attachments');await pause(1200);output='';child.write('R');await until('Subject [');child.write('\r');
   await until('Start (YYYY-MM-DD HH:MM):');child.write('2030-05-01 10:00\r');await until('Duration (');output='';child.write('1h30m\r');
   await until('Attachments');await pause(2200);
   const file=path.join(scratch,'.request.tmp.ics'),payload=fs.readFileSync(file,'utf8');
   for(const token of ['METHOD:REQUEST','DTSTART:20300501T100000Z','DURATION:PT1H30M','reader@example.invalid','target@example.invalid','Request Grüße 日本語'])if(!payload.includes(token))throw Error('Missing request property '+token);
   if(fs.existsSync(path.join(scratch,'.compose.tmp.mbox')))throw Error('Serialized draft scratch remains');
   fs.writeFileSync(path.join(scratch,'generated_request.ics'),payload);capture('request_generated');
   output='';child.write('q');await until('Mail not sent');await pause(1200);
   if(fs.existsSync(file))throw Error('Discarded request file remains');capture('request_discarded');
   child.write('q');for(let i=0;i<100&&exit===null;i++)await pause(50);
   if(exit!==0)throw Error('Unclean request exit '+exit);
   return{surface:bundled?'editor_conpty':'system_conpty',state,passed:true,directory:path.basename(scratch),phases};
  }
  child.write('v');await until('Attachments');capture('calendar_attach');
  output='';child.write('r');await until('Accept Invitation?');capture('reply_prompt');
  output='';child.write(({accept:'y',decline:'n',tentative:'t',cancel:'c'})[state]+'\r');
  if(state==='cancel'){await until('Press any key to continue');child.write(' ');}
  await until('Attachments');await pause(2200);
  const reply=path.join(scratch,'.reply.tmp.ics');
  if(state==='cancel'){if(fs.existsSync(reply))throw Error('Cancelled reply placeholder remains');}
  else{
   if(!fs.existsSync(reply))throw Error('Reply not generated');
   const payload=fs.readFileSync(reply,'utf8');
   const expected={accept:'ACCEPTED',decline:'DECLINED',tentative:'TENTATIVE'}[state];
   if(!payload.includes('METHOD:REPLY')||!payload.includes('PARTSTAT='+expected)||!payload.includes('reader@example.invalid'))throw Error('Reply semantics changed');
   fs.writeFileSync(path.join(scratch,'generated_reply.ics'),payload);
  }
  if(fs.existsSync(path.join(scratch,'.event.tmp.ics')))throw Error('Extracted calendar scratch remains');capture('reply_generated');
  output='';child.write('q');await until('Mail not sent');await pause(1200);
  if(fs.existsSync(reply))throw Error('Discarded reply file remains');capture('reply_discarded');
  for(let i=0;i<3&&exit===null;i++){child.write('q');await pause(1400);}
  for(let i=0;i<100&&exit===null;i++)await pause(50);
  if(exit!==0)throw Error('Unclean exit '+exit);
  return{surface:bundled?'editor_conpty':'system_conpty',state,passed:true,directory:path.basename(scratch),phases};
 }finally{capture('calendar_final');if(exit===null)child.kill();}
}
(async()=>{const results=[];for(const bundled of [false,true])for(const state of (process.argv[6]?process.argv[6].split(','):['accept','decline','tentative','cancel','request'])){
 try{results.push(await check(bundled,state));}catch(e){results.push({surface:bundled?'editor_conpty':'system_conpty',state,passed:false,error:String(e)});}
 fs.writeFileSync(path.join(root,'calendar_results.json'),JSON.stringify(results,null,2));}
 console.log(JSON.stringify(results,null,2));process.stdout.write('',()=>process.exit(results.every(r=>r.passed)?0:1));})();
