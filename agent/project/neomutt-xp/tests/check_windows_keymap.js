/* Retain effective bindings/macros from the actual Windows interactive client. */
const fs=require('fs'),path=require('path');
const [modulePath,app,config,root,capture]=process.argv.slice(2),pty=require(modulePath),pause=ms=>new Promise(r=>setTimeout(r,ms)),posix=p=>p.replace(/\\/g,'/');
(async()=>{fs.mkdirSync(root,{recursive:true});const box=path.join(root,'fixture.maildir');for(const d of ['cur','new','tmp'])fs.mkdirSync(path.join(box,d),{recursive:true});fs.writeFileSync(path.join(box,'cur','owned;2,S'),'From: fixture@example.invalid\nSubject: Keymap fixture\n\nKeymap fixture body\n');
 let output='',exit=null;const child=pty.spawn(app,['-F',config,'-f',box,'-e','set quit=yes'],{cols:120,rows:30,cwd:root,env:{...process.env,TERM:'xterm-256color'},useConpty:true,useConptyDll:true});
 child.onData(s=>{output+=s;if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 try{
  const start=Date.now();while(!output.includes('Keymap fixture')&&exit===null&&Date.now()-start<15000)await pause(50);if(!output.includes('Keymap fixture'))throw Error(output.slice(-1000));
  for(const kind of ['bind','macro']){const file=path.join(root,kind+'.rc');child.write(':set pager="python3 '+posix(capture)+' '+posix(file)+'"\r');await pause(300);child.write(':'+kind+'\r');const start=Date.now();while(!fs.existsSync(file)&&exit===null&&Date.now()-start<12000)await pause(50);if(!fs.existsSync(file))throw Error('Missing '+kind+' export '+output.slice(-1000));await pause(500);}
  child.write('q');for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit!==0)throw Error('Unclean exit '+exit);console.log('Effective Windows bindings/macros exported');process.exit(0);
 }finally{fs.writeFileSync(path.join(root,'terminal.vt'),output);if(exit===null)child.kill();}
})();
