/* Exercise the production scanner against a local deep-directory fixture. */
#include "../load_dir.h"
#include <assert.h>
#include <stdio.h>
#include <string.h>
int main(int argc, char **argv) {
    assert(argc == 2 && strlen(argv[1]) > 1024);
    struct directory dir;
    assert(dir_open(&dir, argv[1]) == 0);
    int file = 0, link = 0; const char *name;
    while ((name = dir_read(&dir))) {
        if (!strcmp(name, "long_music.opus")) { assert(S_ISREG(dir.st.st_mode)); file++; }
        if (!strcmp(name, "loop")) { assert(dir.is_link && S_ISDIR(dir.st.st_mode)); link++; }
    }
    dir_close(&dir); assert(file == 1 && link == 1);
    puts("deep-directory scanning and recursion-link detection: PASS");
    return 0;
}
