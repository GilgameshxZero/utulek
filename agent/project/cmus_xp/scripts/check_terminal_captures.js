/* Validate check_terminal_surfaces.js JSONL with xterm's actual parser.
 * Arguments: @xterm/headless module path, capture JSONL, results JSON.
 * Install parser dependencies in an ignored .obj/ directory. GPL-2.0-or-later.
 */
const fs = require('fs');
const [modulePath, input, output] = process.argv.slice(2);
const {Terminal} = require(modulePath);

(async () => {
    const terminals = new Map();
    const results = [];
    for (const line of fs.readFileSync(input, 'utf8').split(/\r?\n/)) {
        if (!line.startsWith('{')) continue;
        const frame = JSON.parse(line);
        let terminal = terminals.get(frame.stream);
        if (!terminal) {
            terminal = new Terminal({
                cols: frame.cols, rows: frame.rows, allowProposedApi: true,
            });
            terminals.set(frame.stream, terminal);
        } else if (terminal.cols !== frame.cols || terminal.rows !== frame.rows) {
            terminal.resize(frame.cols, frame.rows);
        }
        await new Promise(resolve => terminal.write(
            Buffer.from(frame.bytes, 'base64'), resolve));
        const positions = [];
        const lines = [];
        for (let y = 0; y < terminal.rows; y++) {
            const text = terminal.buffer.active.getLine(
                terminal.buffer.active.viewportY + y)?.translateToString() || '';
            lines.push(text);
            for (let x = 0; x < text.length; x++)
                if (text[x] === '│') positions.push([y, x]);
        }
        const columns = [...new Set(positions.map(position => position[1]))];
        const expectedColumn = Math.floor(frame.cols * .33);
        const expectedRows = frame.rows - (frame.surface.endsWith('_tmux') ? 5 : 4);
        const body = terminal.buffer.active.getLine(
            terminal.buffer.active.viewportY + 1)?.getCell(0);
        const title = terminal.buffer.active.getLine(
            terminal.buffer.active.viewportY)?.getCell(0);
        const inheritsTerminal = !!body?.isFgDefault() && !!body?.isBgDefault();
        const explicitTitleColor = !!title?.isBgPalette() && title.getBgColor() === 4;
        const pass = columns.length === 1 && columns[0] === expectedColumn &&
            positions.length === expectedRows && lines[0].includes('Library') &&
            lines[0].includes('Empty (use :add)') && inheritsTerminal && explicitTitleColor;
        const result = {
            surface: frame.surface, phase: frame.phase,
            cols: frame.cols, rows: frame.rows, dividerColumns: columns,
            dividerRows: positions.length, expectedColumn, expectedRows, pass,
            inheritsTerminal, explicitTitleColor,
        };
        results.push(result);
        console.log(result.surface, result.phase,
            `${result.cols}x${result.rows}`, pass ? 'PASS' : 'FAIL');
    }
    fs.writeFileSync(output, JSON.stringify(results, null, 2) + '\n');
    for (const terminal of terminals.values()) terminal.dispose();
    if (results.length !== 20 || results.some(result => !result.pass))
        process.exitCode = 1;
})().catch(error => { console.error(error); process.exitCode = 1; });
