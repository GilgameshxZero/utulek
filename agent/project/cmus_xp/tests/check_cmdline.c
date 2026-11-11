/* Exercise allocation boundaries while inserting actual Unicode characters. */
#include "../cmdline.h"
#include <assert.h>
#include <stdlib.h>
#include <string.h>
int main(void)
{
    cmdline_init();
    for (int i = 0; i < 4096; i++) {
        cmdline_insert_ch('a');
        assert(cmdline.line[i + 1] == 0);
    }
    for (int i = 0; i < 4096; i++) cmdline_backspace();
    for (int i = 0; i < 1000; i++) cmdline_insert_ch(0x97f3);
    assert(cmdline.blen == 3000 && !cmdline.line[3000]);
    for (int i = 0; i < 1000; i++) cmdline_backspace();
    assert(cmdline.blen == 0 && cmdline.line[0] == 0);
    free(cmdline.line);
    return 0;
}
