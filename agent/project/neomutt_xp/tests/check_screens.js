/* Parse real terminal captures and check layout after resize. GPL-2.0-or-later */
const fs=require('fs'),path=require('path');
const [modules,root,output]=process.argv.slice(2);
const {Terminal}=require(path.join(modules,'@xterm/headless'));
const {Unicode11Addon}=require(path.join(modules,'@xterm/addon-unicode11'));
(async()=>{
 let cases;
 if(fs.existsSync(path.join(root,'index.vt')))cases=[root];
 else{const report=JSON.parse(fs.readFileSync(path.join(root,'results.json'),'utf8'));
 cases=report.filter(r=>r.directory).map(r=>path.join(root,r.directory));
 if(!cases.length)cases=fs.readdirSync(root).map(n=>path.join(root,n)).filter(p=>fs.existsSync(path.join(p,'restart_discard.vt')));}
 const results=[];
 for(const folder of cases){
  const terminal=new Terminal({cols:100,rows:30,allowProposedApi:true});terminal.loadAddon(new Unicode11Addon());terminal.unicode.activeVersion='11';
  let previous='';
  for(const [phase,cols,rows] of [['index',100,30],['pager',100,30],['resize_small',80,24],['resize_large',120,40],['compose_editor',120,40],['flag_persisted',120,40],['postponed',120,40]]){
   const raw=fs.readFileSync(path.join(folder,phase+'.vt'),'utf8');
   const delta=raw.startsWith(previous)?raw.slice(previous.length):raw;previous=raw;
   terminal.resize(cols,rows);await new Promise(resolve=>terminal.write(delta,resolve));
   const lines=Array.from({length:rows},(_,i)=>terminal.buffer.active.getLine(terminal.buffer.active.viewportY+i)?.translateToString(true)||'');
   const status=lines[rows-2];
   let passed;
   if(phase==='index')passed=lines.some(l=>l.includes('Windows fixture'));
   else if(phase==='compose_editor')passed=status.includes('Compose')&&lines.some(l=>l.includes('Editor fixture: Grüße 日本語'));
   else if(phase==='flag_persisted'||phase==='postponed')passed=status.includes('NeoMutt:');
   else passed=status.includes('Windows fixture')&&lines.some(l=>l.includes('fixture body'));
   results.push({case:path.basename(folder),phase,cols,rows,passed,status,lines});
  }
  terminal.dispose();
 }
 fs.writeFileSync(output,JSON.stringify(results,null,2));
 for(const r of results)console.log(JSON.stringify({case:r.case,phase:r.phase,passed:r.passed,status:r.status}));
 process.exitCode=results.length&&results.every(r=>r.passed)?0:1;
})().catch(e=>{console.error(e);process.exitCode=1;});
