/* Actual terminal passphrase input for an isolated encrypted fixture.
 * GPL-2.0-or-later. No real keys or configuration are read. */
const fs=require('fs'),path=require('path');
const [modulePath,app,root,ciphertext]=process.argv.slice(2),pty=require(modulePath);
const pause=ms=>new Promise(r=>setTimeout(r,ms)),posix=s=>s.replace(/\\/g,'/');
(async()=>{
 const results=[];
 for(const bundled of [false,true]){
  const scratch=fs.mkdtempSync(path.join(root,'pinentry-'+(bundled?'editor-':'system-')));
  const home=path.join(scratch,'gpg_home');fs.mkdirSync(home);
  const cipher=path.join(scratch,'settings.gpg');fs.copyFileSync(ciphertext,cipher);
  const command='gpg --homedir "'+posix(home).replace(/^([A-Za-z]):/,(_,d)=>'/'+d.toLowerCase())+'" --quiet --no-symkey-cache --pinentry-mode loopback --status-fd 2 --decrypt "'+posix(cipher)+'"';
  const chosen=process.argv[6]?'python3 -X utf8 \"'+posix(process.argv[6])+'\" \"'+posix(process.argv[7])+'\" \"'+posix(scratch)+'\"':command;
  const rc=path.join(scratch,'test.rc');fs.writeFileSync(rc,'set crypt_use_gpgme=no\nsource "'+chosen.replace(/"/g,'\\"')+'|"\n');
  let output='',exit=null,sent=false,promptCount=0,promptTail='';
  const child=pty.spawn(app,['-n','-F',rc,'-Q','my_fixture_decryption_marker'],{cols:120,rows:40,cwd:scratch,env:{...process.env,HOME:scratch,USERPROFILE:scratch,GNUPGHOME:home,TERM:'xterm-256color'},useConpty:true,useConptyDll:bundled});
  child.onData(s=>{output+=s;if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');
   promptTail+=s;let match;while((match=/(?:Enter passphrase|Repeat passphrase|Repeat this passphrase):/i.exec(promptTail))){promptCount++;sent=true;child.write('fixture-only-passphrase\r');promptTail=promptTail.slice(match.index+match[0].length);}});
  child.onExit(e=>exit=e.exitCode);
  const start=Date.now();while(exit===null&&Date.now()-start<60000)await pause(50);
  if(exit===null)child.kill();
  fs.writeFileSync(path.join(scratch,'terminal.vt'),output);
  const passed=exit===0&&sent&&(process.argv[6]||output.includes('DECRYPTION_OKAY'))&&output.includes('my_fixture_decryption_marker')&&!output.includes('fixture-only-passphrase')&&(!process.argv[6]||output.includes('console token refresh verified'));
  results.push({surface:bundled?'editor_conpty':'system_conpty',passed,exit,prompt_answered:sent,prompt_count:promptCount,passphrase_echoed:output.includes('fixture-only-passphrase'),directory:path.basename(scratch)});
 }
 fs.writeFileSync(path.join(root,'pinentry_results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));process.exit(results.every(r=>r.passed)?0:1);
})();
