/* Assert theme-relative reverse video in actual index and status rows. */
const fs=require('fs'),path=require('path');
const [modules,capture,output]=process.argv.slice(2),{Terminal}=require(path.join(modules,'@xterm/headless'));
(async()=>{let results=[];for(const theme of [{name:'light',foreground:'#000000',background:'#ffffff'},{name:'dark',foreground:'#ffffff',background:'#000000'}]){
const t=new Terminal({cols:120,rows:40,allowProposedApi:true,theme});await new Promise(r=>t.write(fs.readFileSync(capture,'utf8'),r));
for(const token of ['Reply address fixture','NeoMutt:']){let found=null;for(let y=0;y<40;y++){const line=t.buffer.active.getLine(t.buffer.active.viewportY+y),x=line.translateToString().indexOf(token);if(x>=0){const c=line.getCell(x);found={inverse:!!c.isInverse(),fgDefault:c.isFgDefault(),bgDefault:c.isBgDefault()};break;}}
results.push({theme:theme.name,row:token,passed:!!found&&found.inverse&&found.fgDefault&&found.bgDefault,attributes:found});}t.dispose();}
fs.writeFileSync(output,JSON.stringify(results,null,2));console.log(JSON.stringify(results,null,2));process.exit(results.every(r=>r.passed)?0:1);})();
