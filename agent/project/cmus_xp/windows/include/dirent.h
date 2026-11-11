/* Native UTF-8 directory entries, including full Unicode Windows filenames. */
#ifndef CMUS_WIN_DIRENT_H
#define CMUS_WIN_DIRENT_H
typedef struct win_directory DIR;
struct dirent {
    unsigned short d_namlen;
    int d_is_link;
    char d_name[1024];
};
DIR *win_opendir(const char *);
struct dirent *win_readdir(DIR *);
int win_closedir(DIR *);
#endif
