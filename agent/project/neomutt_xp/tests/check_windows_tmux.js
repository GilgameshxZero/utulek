/* Reproduce Windows tmux repaint regressions using owned mail and an isolated server. */
const fs=require('fs'),path=require('path'),cp=require('child_process');
const pty=require(process.argv[2]);
const [app,tmux,root,parser]=process.argv.slice(3);
const {Terminal}=require(parser);
const pause=ms=>new Promise(r=>setTimeout(r,ms));
async function check(bundled,theme){
 const dir=fs.mkdtempSync(path.join(root,theme+'-'+(bundled?'editor-':'system-')));
 const label='neomutt-test-'+process.pid+'-'+Date.now();
 const env={...process.env,HOME:dir,USERPROFILE:dir,LOCALAPPDATA:dir,TEMP:dir,TMP:dir,TMPDIR:dir,TERM:'xterm-256color',LANG:'C.UTF-8'};
 const box=path.join(dir,'maildir');for(const sub of ['cur','new','tmp'])fs.mkdirSync(path.join(box,sub),{recursive:true});
 for(let i=0;i<100;i++)fs.writeFileSync(path.join(box,'cur',String(i).padStart(3,'0')+';2,S'),`From: sender@example.invalid\nTo: reader@example.invalid\nSubject: Rendering fixture ${String(i).padStart(3,'0')}\nDate: Tue, 03 Nov 2026 00:00:00 +0000\n\n`+Array.from({length:200},(_,n)=>`Body line ${String(n).padStart(3,'0')} — terminal rendering fixture`).join('\n')+'\n');
 const rc=path.join(dir,'test.rc');fs.writeFileSync(rc,'set folder="." maildir_field_delimiter=";" crypt_use_gpgme=no quit=yes move=no delete=yes sort=subject sleep_time=0\nunset beep\ncolor normal white black\ncolor indicator brightwhite blue\ncolor status black cyan\ncolor header yellow black .\n');
 if(theme==='default_reverse')fs.appendFileSync(rc,'color normal default default\ncolor indicator reverse default default\ncolor status reverse default default\ncolor header cyan default .\n');
 const config=path.join(dir,'tmux.conf');fs.writeFileSync(config,'set -g status off\nset -g default-terminal "tmux-256color"\n');
 let server=null,child=null,terminal=null,exit=null,lastData=0;const observations=[];
 function start(args){
  terminal=new Terminal({cols:120,rows:40,allowProposedApi:true});exit=null;
  child=pty.spawn(tmux,['-L',label,'-f',config,...args],{cols:120,rows:40,cwd:dir,env,useConpty:true,useConptyDll:bundled});
  child.onData(data=>{lastData=Date.now();terminal.write(data);if(data.includes('\x1b[6n'))child.write('\x1b[1;1R');if(data.includes('\x1b[c'))child.write('\x1b[?1;2c');});child.onExit(e=>exit=e.exitCode);
 }
 const text=()=>Array.from({length:terminal.rows},(_,i)=>terminal.buffer.active.getLine(i)?.translateToString(true)||'').join('\n');
 async function settle(token){const deadline=Date.now()+20000;while(Date.now()<deadline){if(exit!==null)throw Error('tmux exited '+exit+' '+text().slice(-1000));if((!token||text().includes(token))&&Date.now()-lastData>600)return;await pause(50);}throw Error('Timed out '+token+' '+text().slice(-1000));}
 // Blank and erased cells and their invisible foregrounds are visually equal; pager status text can change on redraw.
 function cells(){const rows=[];for(let y=0;y<terminal.rows-1;y++){const row=[];for(let x=0;x<terminal.cols;x++){const c=terminal.buffer.active.getLine(y)?.getCell(x);row.push(c?[(y===terminal.rows-2?'':(c.getChars()||' ')),(y!==terminal.rows-2&&(c.getChars()||' ')===' '&&!c.isInverse()?0:c.getFgColorMode()),(y!==terminal.rows-2&&(c.getChars()||' ')===' '&&!c.isInverse()?-1:c.getFgColor()),c.getBgColorMode(),c.getBgColor(),!!c.isInverse(),!!c.isBold()]:null);}rows.push(row);}return rows;}
 async function compare(name){await settle();if(server&&process.argv.includes('--diagnostic'))await new Promise(resolve=>cp.execFile(path.join(path.dirname(root),'console_snapshot.exe'),[String(server.pid),path.join(dir,name+'_console.txt')],{timeout:5000,windowsHide:true},()=>resolve()));if(name==='pager_initial'&&process.argv.includes('--diagnostic'))await new Promise(resolve=>cp.execFile(tmux,['-L',label,'capture-pane','-p','-e'],{env,encoding:'utf8',timeout:5000,windowsHide:true},(error,stdout,stderr)=>{fs.writeFileSync(path.join(dir,'pager_tmux_capture.txt'),JSON.stringify({error:String(error||''),stdout,stderr}));resolve();}));const before=cells();child.write('\x0c');await pause(300);await settle();const after=cells();const differences=[];for(let y=0;y<after.length;y++)for(let x=0;x<after[y].length;x++)if(JSON.stringify(before[y][x])!==JSON.stringify(after[y][x]))differences.push({x,y,before:before[y][x],after:after[y][x]});observations.push({name,changed_cells:differences.length,differences});fs.writeFileSync(path.join(dir,name+'.json'),JSON.stringify({before,after,differences}));}
 try{
  server=cp.spawn(tmux,['-D','-L',label,'-f',config],{cwd:dir,env,windowsHide:true,stdio:'ignore'});await pause(1000);
  start(['new-session','-s','fixture',app,'-n','-F',rc,'-f',box]);
  await settle('q:Quit');
  child.write('=');await pause(500);if(process.argv.includes('--diagnostic-wrap'))await new Promise(resolve=>cp.execFile(path.join(path.dirname(root),'console_snapshot.exe'),[String(child.pid),path.join(dir,'mode_fix.txt'),'wrap'],{timeout:5000,windowsHide:true},()=>resolve()));await compare('initial');
  for(let i=0;i<75;i++){child.write('j');await pause(15);}await pause(300);await compare('index_scrolled');
  for(let i=0;i<75;i++){child.write('k');await pause(15);}await pause(300);await compare('index_returned');
  child.write('\r');await settle('i:Exit');await compare('pager_initial');await settle('Body line 000');
  child.write('\x1b[6~');await pause(300);await compare('pager_scrolled');
  child.write('\x1b[5~');await pause(300);await compare('pager_returned');
  child.write('q');await settle('q:Quit');await compare('index_after_pager');
  child.write('\x02d');for(let i=0;i<100&&exit===null;i++)await pause(50);if(exit===null)throw Error('Detach did not finish');terminal.dispose();
  start(['attach-session','-t','fixture']);await settle('Rendering fixture');await compare('reattached');
  child.resize(100,32);terminal.resize(100,32);await pause(1000);await compare('resized');
  child.resize(120,40);terminal.resize(120,40);await pause(1000);await compare('restored_size');
  return {surface:bundled?'editor_conpty':'system_conpty',theme,passed:observations.every(o=>o.changed_cells===0),observations,directory:path.basename(dir)};
 }catch(error){fs.writeFileSync(path.join(dir,'pane_capture.txt'),cp.spawnSync(tmux,['-L',label,'capture-pane','-p','-e'],{env,encoding:'utf8',timeout:10000}).stdout||'');return{surface:bundled?'editor_conpty':'system_conpty',theme,passed:false,error:String(error),observations,directory:path.basename(dir),screen:terminal?text():''};}
 finally{await new Promise(resolve=>cp.execFile(tmux,['-L',label,'kill-server'],{env,windowsHide:true,timeout:10000},()=>resolve()));if(child&&exit===null)child.kill();if(server)server.kill();if(terminal)terminal.dispose();}
}
(async()=>{fs.mkdirSync(root,{recursive:true});const results=[];for(const bundled of (process.argv[7]==='system'?[false]:[false,true]))for(const theme of ['explicit','default_reverse']){results.push(await check(bundled,theme));fs.writeFileSync(path.join(root,'results.json'),JSON.stringify(results,null,2));}console.log(JSON.stringify(results.map(r=>({...r,observations:r.observations.map(o=>({name:o.name,changed_cells:o.changed_cells}))})),null,2));process.stdout.write('',()=>process.exit(results.every(r=>r.passed)?0:1));})();
