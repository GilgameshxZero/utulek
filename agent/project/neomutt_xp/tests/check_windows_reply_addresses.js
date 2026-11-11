/* Actual index/pager replies. Fixture drafts only; no send command is used. */
const fs=require('fs'),path=require('path');
const [modulePath,app,root]=process.argv.slice(2),pty=require(modulePath),pause=ms=>new Promise(r=>setTimeout(r,ms));
(async()=>{const cases=JSON.parse(fs.readFileSync(path.join(root,'cases.json'))),results=[];
for(const c of cases){let output='',exit=null;const p=pty.spawn(app,['-n','-F',c.rc,'-f',c.box],{cols:120,rows:40,cwd:c.directory,env:{...process.env,TERM:'xterm-256color'},useConpty:true,useConptyDll:true});
p.onData(s=>{output+=s;if(s.includes('\x1b[6n'))p.write('\x1b[1;1R');if(s.includes('\x1b[c'))p.write('\x1b[?1;2c');});p.onExit(e=>exit=e.exitCode);
const until=async token=>{let start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<15000)await pause(50);if(!output.includes(token))throw Error('Missing '+token+': '+output.slice(-1200));};
try{await until('Reply address fixture');await pause(200);fs.writeFileSync(path.join(c.directory,'index.vt'),output);if(c.pager){p.write('\r');await until('Synthetic reply body');}output='';p.write(c.key);await until('Attachments');p.write('Z');let start=Date.now();while(!fs.existsSync(c.draft)&&Date.now()-start<10000)await pause(50);if(!fs.existsSync(c.draft))throw Error('No serialized reply draft');await pause(300);p.write('q');await until('Mail not sent');p.write(c.pager?'qq':'q');while(exit===null&&Date.now()-start<15000)await pause(50);if(exit!==0)throw Error('Exit '+exit);results.push({name:c.name,passed:true});}
catch(e){results.push({name:c.name,passed:false,error:String(e)});}finally{fs.writeFileSync(path.join(c.directory,'terminal.vt'),output);if(exit===null)p.kill();}}
fs.writeFileSync(path.join(root,'ui_results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));process.exit(results.every(r=>r.passed)?0:1);
})();
