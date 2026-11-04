/* PGP/MIME reader decryption using encrypted synthetic mail, no real keys.
 * GPL-2.0-or-later. */
const fs=require('fs'),path=require('path');
const [modulePath,app,root,cipher]=process.argv.slice(2),pty=require(modulePath);
const pause=ms=>new Promise(r=>setTimeout(r,ms));
async function check(bundled,state){
 const scratch=fs.mkdtempSync(path.join(root,'encrypted-'+(bundled?'editor-':'system-')+state+'-'));
 const box=path.join(scratch,'inbox.maildir');for(const dir of ['cur','new','tmp'])fs.mkdirSync(path.join(box,dir),{recursive:true});
 fs.mkdirSync(path.join(scratch,'gpg_home'));
 const message='From: sender@example.invalid\nTo: reader@example.invalid\nSubject: Encrypted MIME fixture\nMIME-Version: 1.0\nContent-Type: multipart/encrypted; protocol="application/pgp-encrypted"; boundary="fixture"\n\n--fixture\nContent-Type: application/pgp-encrypted\n\nVersion: 1\n\n--fixture\nContent-Type: application/octet-stream\nContent-Transfer-Encoding: base64\n\n'+fs.readFileSync(cipher).toString('base64').match(/.{1,76}/g).join('\n')+'\n--fixture--\n';
 fs.writeFileSync(path.join(box,'cur','encrypted;2,S'),message);
 const command='gpg --homedir ./gpg_home --batch --quiet --no-symkey-cache --pinentry-mode loopback --passphrase-fd 0 --status-fd 2 --decrypt %f';
 const rc=path.join(scratch,'test.rc');fs.writeFileSync(rc,'set folder="."\nset maildir_field_delimiter=";"\nset charset="utf-8"\nset quit=yes\nset move=no\nset wait_key=no\nset sleep_time=0\nunset beep\nunset pgp_use_gpg_agent\nset crypt_use_gpgme=no\nbind editor <space> noop\nset pgp_decrypt_command="'+command+'"\nset pgp_decode_command="'+command+'"\n');
 let output='',exit=null;const phases=[];
 const child=pty.spawn(app,['-n','-F',rc,'-f',box],{cols:120,rows:40,cwd:scratch,env:{...process.env,HOME:scratch,USERPROFILE:scratch,TMPDIR:scratch,TERM:'xterm-256color',LANG:'C.UTF-8'},useConpty:true,useConptyDll:bundled});
 child.onData(s=>{output+=s;if(s.includes('\x1b[6n'))child.write('\x1b[1;1R');if(s.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 async function until(token){const start=Date.now();while(!output.includes(token)&&exit===null&&Date.now()-start<15000)await pause(50);if(!output.includes(token))throw Error('Missing '+token+'; '+JSON.stringify(output.slice(-1500)));}
 const capture=name=>{fs.writeFileSync(path.join(scratch,name+'.vt'),output);phases.push(name);};
 try{
  await until('Encrypted MIME fixture');output='';child.write('\r');await until('Enter PGP passphrase:');capture('passphrase_prompt');
  output='';child.write(state==='cancel'?'\x07':(state==='success'?'fixture-only-passphrase':'incorrect-fixture-passphrase')+'\r');
  if(state==='success'){await until('Encrypted viewer fixture secret');await until('DECRYPTION_OKAY');if(output.includes('fixture-only-passphrase'))throw Error('Passphrase echoed');capture('decrypted_pager');child.write('q');await pause(700);}
  else{if(state==='wrong'){const start=Date.now();while(!/Decryption failed|Could not copy message/.test(output)&&exit===null&&Date.now()-start<15000)await pause(50);if(!/Decryption failed|Could not copy message/.test(output))throw Error('Missing decryption rejection');}await pause(700);if(output.includes('Encrypted viewer fixture secret'))throw Error('Plaintext exposed on rejected input');capture('decryption_rejected');}
  child.write('q');for(let i=0;i<100&&exit===null;i++)await pause(50);
  if(exit!==0)throw Error('Unclean exit '+exit);
  return{surface:bundled?'editor_conpty':'system_conpty',state,passed:true,directory:path.basename(scratch),phases};
 }finally{capture('final');if(exit===null)child.kill();}
}
(async()=>{fs.mkdirSync(root,{recursive:true});const results=[];for(const bundled of [false,true])for(const state of ['success','wrong','cancel']){
 try{results.push(await check(bundled,state));}catch(e){results.push({surface:bundled?'editor_conpty':'system_conpty',state,passed:false,error:String(e)});}
 fs.writeFileSync(path.join(root,'encrypted_viewer_results.json'),JSON.stringify(results,null,2));}
 console.log(JSON.stringify(results,null,2));process.exit(results.every(r=>r.passed)?0:1);})();
