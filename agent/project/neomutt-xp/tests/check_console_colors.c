/* Verify actual Win32 cell attributes for default/reverse/mixed colors. */
#include <windows.h>
#include <assert.h>
#include <stdio.h>
#include <curses.h>
int main(void){
 HANDLE original=CreateFileW(L"CONOUT$",GENERIC_READ|GENERIC_WRITE,FILE_SHARE_READ|FILE_SHARE_WRITE,NULL,OPEN_EXISTING,0,NULL);
 CONSOLE_SCREEN_BUFFER_INFO old;assert(GetConsoleScreenBufferInfo(original,&old));
 assert(SetConsoleTextAttribute(original,0x1e));
 initscr();assert(start_color()==OK);assert(use_default_colors()==OK);
 assert(init_pair(1,-1,-1)==OK);assert(init_pair(2,COLOR_RED,-1)==OK);
 mvaddch(0,0,'N'|COLOR_PAIR(1));mvaddch(0,1,'R'|COLOR_PAIR(1)|A_REVERSE);mvaddch(0,2,'C'|COLOR_PAIR(2));mvaddch(0,3,'B'|COLOR_PAIR(1)|A_BOLD|A_REVERSE);refresh();
 HANDLE active=CreateFileW(L"CONOUT$",GENERIC_READ,FILE_SHARE_READ|FILE_SHARE_WRITE,NULL,OPEN_EXISTING,0,NULL);
 WORD attrs[4];DWORD count;assert(ReadConsoleOutputAttribute(active,attrs,4,(COORD){0,0},&count)&&count==4);
 endwin();SetConsoleTextAttribute(original,old.wAttributes);CloseHandle(original);CloseHandle(active);
 printf("color attributes: %x %x %x\n",attrs[0],attrs[1],attrs[2]);
 assert((attrs[0]&0xff)==0x1e);assert((attrs[1]&0xff)==0x1e);assert(attrs[1]&COMMON_LVB_REVERSE_VIDEO);assert(attrs[3]==attrs[1]);assert((attrs[2]&0xff)==0x14);
 puts("default/reverse/mixed color attributes passed");return 0;
}
