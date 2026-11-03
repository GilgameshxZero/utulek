/* Physical console-event translation for configured Alt bindings and AltGr. */
#include <windows.h>
#include <assert.h>
#include <stdio.h>
#include <locale.h>
#include <curses.h>
static void key(HANDLE input,WORD vk,WCHAR ch,DWORD modifiers){
 INPUT_RECORD event={0};event.EventType=KEY_EVENT;event.Event.KeyEvent.bKeyDown=TRUE;event.Event.KeyEvent.wRepeatCount=1;event.Event.KeyEvent.wVirtualKeyCode=vk;event.Event.KeyEvent.uChar.UnicodeChar=ch;event.Event.KeyEvent.dwControlKeyState=modifiers;DWORD n;assert(WriteConsoleInputW(input,&event,1,&n)&&n==1);
}
int main(void){
 setlocale(LC_ALL,".UTF-8");initscr();cbreak();noecho();keypad(stdscr,TRUE);meta(stdscr,TRUE);
 HANDLE input=CreateFileW(L"CONIN$",GENERIC_READ|GENERIC_WRITE,FILE_SHARE_READ|FILE_SHARE_WRITE,NULL,OPEN_EXISTING,0,NULL);assert(input!=INVALID_HANDLE_VALUE);FlushConsoleInputBuffer(input);
 key(input,'E','e',LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()=='e');
 key(input,VK_DELETE,0,LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()==KEY_DC);
 key(input,'Q',0x00e4,RIGHT_ALT_PRESSED|LEFT_CTRL_PRESSED);assert(getch()==0xc3);assert(getch()==0xa4);
 key(input,'E',0x00e9,LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()==0xc3);assert(getch()==0xa9);
 endwin();CloseHandle(input);puts("Alt-letter, Alt-Delete, Alt-Unicode and AltGr passed");return 0;
}
