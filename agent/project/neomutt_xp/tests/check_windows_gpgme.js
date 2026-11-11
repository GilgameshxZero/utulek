/* Native GPGME signed/encrypted reader; only disposable fixture keys/mail. */
const fs=require('fs'),path=require('path'),crypto=require('crypto');
const [modulePath,app,root,fixture,terminalModule]=process.argv.slice(2),pty=require(modulePath),{Terminal}=require(terminalModule);
const pause=ms=>new Promise(r=>setTimeout(r,ms)),posix=s=>s.replace(/\\/g,'/'),hash=b=>crypto.createHash('sha256').update(b).digest('hex');
async function check(bundled,state){
const dir=fs.mkdtempSync(path.join(root,state+'-')),box=path.join(dir,'inbox.maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
let cipher=fs.readFileSync(path.join(fixture,'signed_encrypted.gpg'));if(state==='damaged')cipher=cipher.subarray(0,Math.floor(cipher.length/2));
const raw='From: Fixture Reader <reader@example.invalid>\nTo: reader@example.invalid\nSubject: GPGME reader fixture\nMIME-Version: 1.0\nContent-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="fixture"\n\n--fixture\nContent-Type: application/pgp-encrypted\n\nVersion: 1\n\n--fixture\nContent-Type: application/octet-stream\nContent-Transfer-Encoding: base64\n\n'+cipher.toString('base64').match(/.{1,76}/g).join('\n')+'\n--fixture--\n';
const message=path.join(box,'cur','encrypted;2,S');fs.writeFileSync(message,raw);const before=hash(fs.readFileSync(message));
const home=state==='missing_key'?path.join(dir,'empty_gpg'):path.join(fixture,'gpg_home');fs.mkdirSync(home,{recursive:true});
const rc=path.join(dir,'test.rc');fs.writeFileSync(rc,'set folder="." crypt_use_gpgme=yes maildir_field_delimiter=";" charset="utf-8" read_only=yes quit=yes wait_key=no sleep_time=0 move=no\nunset beep\n');
let output='',exit=null;const t=new Terminal({cols:120,rows:40,allowProposedApi:true});
const env={...process.env,HOME:dir,TMPDIR:dir,GNUPGHOME:home,TERM:'xterm-256color',LANG:'C.UTF-8',GPGME_DEBUG:'9;'+posix(path.join(dir,'gpgme.log'))};delete env.GPG_AGENT_INFO;
const child=pty.spawn(app,['-d','5','-l',path.join(dir,'neomutt_debug'),'-n','-F',rc,'-f',box],{cols:120,rows:40,cwd:dir,env,useConpty:true,useConptyDll:bundled});
child.onData(s=>{output+=s;t.write(s);if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
const screen=()=>Array.from({length:40},(_,i)=>t.buffer.active.getLine(t.buffer.active.viewportY+i)?.translateToString(true)||'').join('\n');
async function until(test,timeout=20000){const start=Date.now();while(!test(screen())&&exit===null&&Date.now()-start<timeout)await pause(50);if(!test(screen()))throw Error('Screen condition failed: '+screen().split('\n').filter(s=>s.trim()).slice(-6).join(' / '));}
try{await until(s=>s.includes('GPGME reader fixture'));output='';child.write('\r');
if(state==='success'){await until(s=>s.includes('GPGME encrypted fixture secret Grüße 日本語'));if(!/Good signature|signature verified/i.test(output))throw Error('Signed payload was decrypted without confirmed signature result');child.write('q');await pause(250);}
else{await until(s=>/decryption failed|Could not decrypt|Could not copy message|No secret key|decryption\/verification failed|Invalid crypto/i.test(s));if(output.includes('GPGME encrypted fixture secret'))throw Error('Failed decrypt exposed payload');const log=fs.readFileSync(path.join(dir,'gpgme.log'),'utf8');if(!/gpgme_op_decrypt_verify[^\n]*error:/i.test(log))throw Error('No confirmed GPGME decryption error');if(state==='missing_key'&&!/No secret key/i.test(log))throw Error('Missing-key cause was not confirmed');}
for(let i=0;i<5&&exit===null;i++){child.write('q');await pause(300);}if(exit!==0)throw Error('Unclean exit '+exit);if(hash(fs.readFileSync(message))!==before)throw Error('Original encrypted message changed');
return{surface:bundled?'editor_conpty':'system_conpty',state,passed:true,original_message_unchanged:true};
}finally{fs.writeFileSync(path.join(dir,'terminal.vt'),output);if(exit===null)child.kill();t.dispose();}
}
(async()=>{fs.mkdirSync(root,{recursive:true});const results=[];for(const bundled of (process.argv.includes('--single')?[false]:[false,true]))for(const state of (process.argv.includes('--single')?['success']:['success','missing_key','damaged'])){try{results.push(await check(bundled,state));}catch(e){results.push({surface:bundled?'editor_conpty':'system_conpty',state,passed:false,error:String(e)});}fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify(results.at(-1)));}process.exit(results.every(r=>r.passed)?0:1);})();
