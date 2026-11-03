/* Actual user-config keyboard workflows; read-only mailboxes, never send mail.
 * All writable compose/calendar artifacts belong to the selected test message. */
const fs=require('fs'),path=require('path');
const [modulePath,app,config,root,settingsFile]=process.argv.slice(2),pty=require(modulePath),settings=JSON.parse(fs.readFileSync(settingsFile,'utf8'));
const {Terminal}=require(settings.terminalModule);
const pause=ms=>new Promise(r=>setTimeout(r,ms)),posix=p=>p.replace(/\\/g,'/');
async function run(name,action){
 const directory=path.join(root,name);const scratch=path.join(directory,'snowfall');fs.mkdirSync(scratch,{recursive:true});const link=path.join(directory,'monochrome');if(!fs.existsSync(link))fs.symlinkSync(settings.monochromeDirectory,link,'junction');
 const extra='set quit=yes postpone=no recall=no sleep_time=0 from="'+settings.sender+'" editor="'+posix(path.join(path.dirname(app),'neomutt-xp-editor.exe'))+'"; cd "'+posix(scratch)+'"';
 const child=pty.spawn(app,['-R','-n','-F',config,'-f',settings.inbox,'-e',extra],{cols:120,rows:40,cwd:scratch,env:{...process.env,TERM:'xterm-256color',TZ:'UTC'},useConpty:true,useConptyDll:true});
 let output='',exit=null;const phases=[];const terminal=new Terminal({cols:120,rows:40,allowProposedApi:true});
 child.onData(s=>{output+=s;terminal.write(s);if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 const screen=()=>Array.from({length:40},(_,i)=>terminal.buffer.active.getLine(terminal.buffer.active.viewportY+i)?.translateToString(true)||'');
 const waitScreen=async(predicate,timeout=30000)=>{const start=Date.now();while(!predicate(screen())&&exit===null&&Date.now()-start<timeout)await pause(50);if(!predicate(screen()))throw Error('Rendered screen condition failed: '+screen().slice(-2).join(' / '));};
 const ui={scratch,screen,waitScreen,write:s=>child.write(s),clear:()=>{output='';},pause,until:async(token,timeout=30000)=>{const start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<timeout)await pause(50);if(!output.includes(token))throw Error('Missing '+token+'; '+JSON.stringify(output.slice(-1000)));},capture:n=>{fs.writeFileSync(path.join(scratch,n+'.vt'),output);phases.push(n);}};
 let result;
 try{await ui.until('NeoMutt:');await action(ui);for(let i=0;i<4&&exit===null;i++){child.write('q');await pause(800);}for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit!==0)throw Error('Unclean exit '+exit);result={name,passed:true,phases};console.log(JSON.stringify(result));}
 catch(e){result={name,passed:false,phases,error:String(e)};console.log(JSON.stringify({name,passed:false,error:String(e).slice(0,200)}));}
 finally{fs.writeFileSync(path.join(scratch,'final.vt'),output);if(exit===null)child.kill();terminal.dispose();}
 return result;
}
async function ownCalendar(ui){
 ui.clear();ui.write('l');await ui.until('Limit to messages matching:');ui.write('~i '+settings.messageId+'\r');await ui.waitScreen(lines=>lines.some(l=>l.includes(settings.subject)));ui.clear();ui.write('\r');await ui.until(settings.subject);ui.write('v');await ui.until('Attachments');ui.write(':exec last-entry\r');await ui.pause(300);ui.capture('attachment_selected');
}
(async()=>{fs.mkdirSync(root,{recursive:true});const results=[];
 if(process.argv[7]!=='calendar')results.push(await run('mailbox_browser',async ui=>{
  for(const box of settings.mailboxes){ui.clear();ui.write('y');await ui.until('Mailboxes');ui.capture('browser_'+box.name);ui.write('/'+box.search+'\r');await ui.pause(500);ui.clear();ui.write('\r');await ui.waitScreen(lines=>lines[38].includes(box.status)&&!/(Reading|Caching|\d+\/\d+ \()/.test(lines[39]),300000);await ui.pause(1000);ui.capture('opened_'+box.name);if(box.name==='archive'){ui.clear();ui.write('l');await ui.until('Limit to messages matching:');ui.write('~i '+settings.messageId+'\r');await ui.waitScreen(lines=>lines.some(l=>l.includes(settings.subject)));ui.clear();ui.write('\r');await ui.until(settings.subject);ui.capture('recorded_calendar_message');ui.write('q');await ui.pause(400);}}
 }));
 results.push(await run('calendar_display',async ui=>{await ownCalendar(ui);ui.clear();ui.write('\r');await ui.until('Title:');await ui.until(settings.calendarSummary);await ui.until('2030-05-01');ui.capture('production_calendar_display');}));
 results.push(await run('calendar_save',async ui=>{
  await ownCalendar(ui);const before=new Set(fs.readdirSync(settings.calendarDirectory));ui.clear();ui.write('S');
  const start=Date.now();let added=[];while(!added.length&&Date.now()-start<20000){await ui.pause(100);added=fs.readdirSync(settings.calendarDirectory).filter(n=>!before.has(n));}
  if(added.length!==1)throw Error('Expected exactly one saved test calendar');const file=path.join(settings.calendarDirectory,added[0]);const data=fs.readFileSync(file,'utf8');if(!data.includes(settings.uid)||!data.includes(settings.calendarSummary))throw Error('Saved calendar differs from selected invitation');
  fs.writeFileSync(path.join(ui.scratch,'saved_calendar.json'),JSON.stringify({file,uid:settings.uid,subject:settings.subject},null,2));ui.capture('production_calendar_save');await ui.pause(1500);
 }));
 for(const state of ['accept','decline','tentative','cancel'])results.push(await run('calendar_reply_'+state,async ui=>{
  await ownCalendar(ui);ui.clear();ui.write('r');await ui.until('Accept Invitation?');ui.capture('production_reply_prompt');ui.clear();ui.write(({accept:'y',decline:'n',tentative:'t',cancel:'c'})[state]+'\r');
  if(state==='cancel'){await ui.until('Press any key to continue');ui.write(' ');}
  await ui.until('Attachments');await ui.pause(2200);const file=path.join(ui.scratch,'.reply.tmp.ics');
  if(state==='cancel'){if(fs.existsSync(file))throw Error('Cancelled reply placeholder remains');}
  else{const data=fs.readFileSync(file,'utf8');const partstat={accept:'ACCEPTED',decline:'DECLINED',tentative:'TENTATIVE'}[state];for(const token of ['METHOD:REPLY','PARTSTAT='+partstat,'mailto:'+settings.replyIdentity,settings.uid])if(!data.includes(token))throw Error('Reply property missing: '+token);fs.writeFileSync(path.join(ui.scratch,'generated_reply.ics'),data);}
  for(const name of ['.event.tmp.ics','.compose.tmp.mbox'])if(fs.existsSync(path.join(ui.scratch,name)))throw Error('Calendar scratch remains: '+name);
  ui.capture('production_reply_generated');ui.clear();ui.write('q');await ui.until('Mail not sent');await ui.pause(1000);if(fs.existsSync(file))throw Error('Discarded calendar attachment remains');ui.capture('production_reply_discarded');
 }));
 fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));process.exit(results.every(r=>r.passed)?0:1);
})();
