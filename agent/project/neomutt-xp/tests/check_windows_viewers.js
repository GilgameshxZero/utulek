/* Native NeoMutt mailcap/HTML/calendar/contact query keyboard workflows.
 * All messages, contacts and paths are isolated fixtures. GPL-2.0-or-later */
const fs=require('fs'),path=require('path');
const [modulePath,app,root,helpers]=process.argv.slice(2),pty=require(modulePath);
const pause=ms=>new Promise(r=>setTimeout(r,ms));
const posix=p=>p.replace(/\\/g,'/');
async function check(bundled){
 const scratch=fs.mkdtempSync(path.join(root,bundled?'viewer-editor-':'viewer-system-'));
 const box=path.join(scratch,'inbox.maildir');
 for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
 const headers=subject=>`From: Sender <sender@example.invalid>\nTo: Reader <reader@example.invalid>\nDate: Tue, 03 Nov 2026 00:00:00 +0000\nSubject: ${subject}\nMIME-Version: 1.0\n`;
 fs.writeFileSync(path.join(box,'cur','html;2,S'),headers('HTML fixture subject')+
  'Content-Type: text/html; charset=utf-8\nContent-Transfer-Encoding: 8bit\n\n'+
  '<html><head><meta charset="UTF-8"></head><body><h1>HTML fixture heading</h1><p>Grüße 日本語</p></body></html>\n');
 const ics='BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Fixture//EN\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\n'+
  'UID:fixture-calendar@example.invalid\r\nDTSTAMP:20261103T000000Z\r\nDTSTART:20300501T100000Z\r\nDTEND:20300501T110000Z\r\n'+
  'SUMMARY:Calendar fixture event\r\nDESCRIPTION:Calendar description Grüße 日本語\r\nLOCATION:Fixture room\r\n'+
  'ORGANIZER:mailto:sender@example.invalid\r\nATTENDEE;CN=Fixture Reader;RSVP=TRUE:mailto:reader@example.invalid\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n';
 fs.writeFileSync(path.join(box,'cur','calendar;2,S'),headers('Calendar fixture subject')+
  'Content-Type: text/calendar; charset=utf-8; method=REQUEST\nContent-Transfer-Encoding: 8bit\n\n'+ics);
 const query=path.join(scratch,'query.py');
 fs.writeFileSync(query,'import sys\nprint("fixture query results")\nprint("fixture@example.invalid\\tFixture Person\\tfixture contact")\n');
 const mailcap=path.join(scratch,'mailcap');
 fs.writeFileSync(mailcap,'text/html; lynx -dump -hiddenlinks=ignore %s; copiousoutput; nametemplate=%s.html\n'+
  'text/calendar; python3 -X utf8 "'+posix(path.join(helpers,'mutt-ical.py'))+'" -D %s; copiousoutput; compose=true\n');
 const rc=path.join(scratch,'test.rc');
 fs.writeFileSync(rc,'set folder="."\nset maildir_field_delimiter=";"\nset charset="utf-8"\n'+
  'set realname="Fixture Reader"\nset from="reader@example.invalid"\nset quit=yes\nset delete=yes\nset move=no\nset mark_old=no\n'+
  'set timeout=1\nset sleep_time=0\nset wait_key=no\nset pager_stop=yes\nset sort=date\nset sort_aux=date\n'+
  'unset record\nunset beep\nset mailcap_path="'+posix(mailcap)+'"\nauto_view text/calendar text/html\n'+
  'set query_command="python3 -X utf8 \\"'+posix(query)+'\\" %s"\nbind editor <space> noop\n');
 const env={...process.env,HOME:scratch,USERPROFILE:scratch,TMPDIR:scratch,TERM:'xterm-256color',LANG:'C.UTF-8'};
 const child=pty.spawn(app,['-n','-F',rc,'-f',box],{cols:120,rows:40,cwd:scratch,env,useConpty:true,useConptyDll:bundled});
 let output='',exit=null;const phases=[];
 child.onData(data=>{output+=data;if(data.includes('\x1b[6n'))child.write('\x1b[1;1R');if(data.includes('\x1b[c'))child.write('\x1b[?1;2c');});
 child.onExit(event=>exit=event.exitCode);
 async function until(token){const start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<12000)await pause(50);
  if(!output.includes(token))throw Error('Missing '+token+'; '+JSON.stringify(output.slice(-1400)));}
 const capture=name=>{fs.writeFileSync(path.join(scratch,name+'.vt'),output);phases.push(name);};
 try{
  await until('fixture subject');
  child.write('/HTML fixture subject\r');await pause(250);output='';child.write('\r');
  await until('HTML fixture heading');capture('html_mailcap');
  child.write('q');await pause(250);child.write('/Calendar fixture subject\r');await pause(250);output='';child.write('\r');
  await until('Calendar fixture event');await until('Fixture room');capture('calendar_mailcap');
  child.write('q');await pause(250);output='';child.write('Q');await until('Query:');
  child.write('Grüße 日本語\r');await until('Fixture Person');capture('contact_query');
  child.write('q');await pause(250);child.write('q');
  const end=Date.now();while(exit===null&&Date.now()-end<5000)await pause(50);
  if(exit!==0)throw Error('Unclean exit '+exit);
  return{surface:bundled?'editor_conpty':'system_conpty',passed:true,phases};
 }finally{capture('viewer_final');if(exit===null)child.kill();}
}
(async()=>{const results=[];for(const bundled of [false,true]){try{results.push(await check(bundled));}catch(e){results.push({surface:bundled?'editor_conpty':'system_conpty',passed:false,error:String(e)});}}
 fs.writeFileSync(path.join(root,'viewer_results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));
 process.stdout.write('',()=>process.exit(results.every(r=>r.passed)?0:1));})();
