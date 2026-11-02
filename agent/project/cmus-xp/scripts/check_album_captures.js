/* Compare populated Windows heading cells with upstream Linux captures.
 * Arguments: @xterm/headless module, Linux JSONL, Windows JSONL, result JSON. */
const fs = require('fs');
const [modulePath, linuxFile, windowsFile, output] = process.argv.slice(2);
const {Terminal} = require(modulePath);
const {Unicode11Addon} = require(require('path').resolve(modulePath,'../addon-unicode11'));
const labels = ['ASCII album title', '日本語のアルバム', 'Café é 🎵 long album title'];
const color = (cell, prefix) => cell['is' + prefix + 'Default']() ? 'default' :
    (cell['is' + prefix + 'RGB']() ? 'rgb:' : 'palette:') + cell['get' + prefix + 'Color']();
function cellState(cell) {
    const text = cell.getChars().normalize('NFC') || (cell.getWidth() ? ' ' : '');
    if (!cell.getWidth()) return {text:'',width:0};
    // Clearing an empty cell and writing a space have the same appearance.
    // Only its effective background matters unless an underline is visible.
    if (text === ' ' && !cell.isUnderline()) {
        const inverse = cell.isInverse(), bg = color(cell,inverse?'Fg':'Bg');
        return {text,width:1,background:bg==='default'?(inverse?'terminal_fg':'terminal_bg'):bg};
    }
    return {text, width: cell.getWidth(),
        fg: color(cell, 'Fg'), bg: color(cell, 'Bg'), bold: !!cell.isBold(),
        italic: !!cell.isItalic(), underline: !!cell.isUnderline(), inverse: !!cell.isInverse()};
}
async function snapshots(file) {
    const terminals = new Map(), result = [];
    for (const line of fs.readFileSync(file, 'utf8').split(/\r?\n/).filter(Boolean)) {
        const frame = JSON.parse(line);
        let terminal = terminals.get(frame.stream);
        if (!terminal) {
            terminal = new Terminal({cols:frame.cols, rows:frame.rows, allowProposedApi:true});
            terminal.loadAddon(new Unicode11Addon());
            terminal.unicode.activeVersion='11';
            terminals.set(frame.stream,terminal);
        } else if (terminal.cols !== frame.cols || terminal.rows !== frame.rows)
            terminal.resize(frame.cols,frame.rows);
        await new Promise(resolve => terminal.write(Buffer.from(frame.bytes,'base64'),resolve));
        const headings = [];
        for (let y=1; y<Math.min(frame.rows-4,12); y++) {
            const row = terminal.buffer.active.getLine(terminal.buffer.active.viewportY+y);
            const start = Math.floor(frame.cols*.33)+1;
            const text = row.translateToString(false,start).normalize('NFC');
            const label = labels.find(value => text.includes(value)) || (text.includes('Short') ? 'Short' : null);
            if (!label) continue;
            headings.push({label,text,cells:Array.from({length:frame.cols-start},(_,i)=>cellState(row.getCell(start+i)))});
        }
        result.push({...frame,bytes:undefined,headings});
    }
    for(const terminal of terminals.values()) terminal.dispose();
    return result;
}
(async()=>{
    const reference=await snapshots(linuxFile), actual=await snapshots(windowsFile), results=[];
    for (const frame of actual) {
        const expected=reference.find(r=>r.phase===frame.phase&&r.cols===frame.cols&&r.rows===frame.rows);
        const differences=[];
        if (!expected) throw Error('Missing reference phase '+frame.phase);
        if(frame.headings.length!==3) differences.push({reason:'heading count',actual:frame.headings.length});
        for(const heading of frame.headings) {
            const other=expected.headings.find(h=>h.label===heading.label);
            if(!other) {differences.push({label:heading.label,reason:'missing reference heading'});continue;}
            const cells=[];
            for(let i=0;i<heading.cells.length;i++)
                if(JSON.stringify(heading.cells[i])!==JSON.stringify(other.cells[i]))
                    cells.push({column:i+Math.floor(frame.cols*.33)+1,actual:heading.cells[i],expected:other.cells[i]});
            if(cells.length) differences.push({label:heading.label,actualText:heading.text,expectedText:other.text,cells});
        }
        const result={surface:frame.surface,phase:frame.phase,cols:frame.cols,rows:frame.rows,
            passed:differences.length===0,differences};
        results.push(result);
        console.log(frame.surface,frame.phase,frame.cols,result.passed?'PASS':'FAIL');
    }
    fs.writeFileSync(output,JSON.stringify(results,null,2)+'\n');
    if(results.some(r=>!r.passed)) process.exitCode=1;
})().catch(error=>{console.error(error);process.exitCode=1;});
