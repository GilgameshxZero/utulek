/* Physical console-event translation for configured Alt bindings and AltGr. */
#include <windows.h>
#include <assert.h>
#include <stdio.h>
#include <locale.h>
#include <curses.h>
static void event(HANDLE input,BOOL down,WORD vk,WCHAR ch,DWORD modifiers){
 INPUT_RECORD record={0};record.EventType=KEY_EVENT;record.Event.KeyEvent.bKeyDown=down;record.Event.KeyEvent.wRepeatCount=1;record.Event.KeyEvent.wVirtualKeyCode=vk;record.Event.KeyEvent.uChar.UnicodeChar=ch;record.Event.KeyEvent.dwControlKeyState=modifiers;DWORD n;assert(WriteConsoleInputW(input,&record,1,&n)&&n==1);
}
static void key(HANDLE input,WORD vk,WCHAR ch,DWORD modifiers){event(input,TRUE,vk,ch,modifiers);}
int main(void){
 setlocale(LC_ALL,".UTF-8");initscr();cbreak();noecho();keypad(stdscr,TRUE);meta(stdscr,TRUE);timeout(1500);
 HANDLE input=CreateFileW(L"CONIN$",GENERIC_READ|GENERIC_WRITE,FILE_SHARE_READ|FILE_SHARE_WRITE,NULL,OPEN_EXISTING,0,NULL);assert(input!=INVALID_HANDLE_VALUE);FlushConsoleInputBuffer(input);
 key(input,'E','e',LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()=='e');
 key(input,VK_DELETE,0,LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()==KEY_DC);
 key(input,'Q',0x00e4,RIGHT_ALT_PRESSED|LEFT_CTRL_PRESSED);assert(getch()==0xc3);assert(getch()==0xa4);
 key(input,'E',0x00e9,LEFT_ALT_PRESSED);assert(getch()==27);assert(getch()==0xc3);assert(getch()==0xa9);
 /* Ordinary Unicode key-up must not insert a second copy. */
 key(input,0,0xd83d,0);event(input,FALSE,0,0xd83d,0);
 key(input,0,0xde00,0);event(input,FALSE,0,0xde00,0);key(input,'Z','Z',0);
 assert(getch()==0xf0);assert(getch()==0x9f);assert(getch()==0x98);assert(getch()==0x80);assert(getch()=='Z');
 /* Alt+numpad characters arrive on release, including surrogate halves.
  * Ignored modifier and keypad events between halves must preserve the pair. */
 event(input,FALSE,VK_MENU,0x00e9,0);assert(getch()==0xc3);assert(getch()==0xa9);
 event(input,FALSE,VK_MENU,0xd83d,0);key(input,VK_MENU,0,LEFT_ALT_PRESSED);
 key(input,VK_NUMPAD6,0,LEFT_ALT_PRESSED);event(input,FALSE,VK_NUMPAD6,0,LEFT_ALT_PRESSED);
 event(input,FALSE,VK_MENU,0xde00,0);key(input,'Z','Z',0);
 assert(getch()==0xf0);assert(getch()==0x9f);assert(getch()==0x98);assert(getch()==0x80);assert(getch()=='Z');
 endwin();CloseHandle(input);puts("Alt bindings, AltGr, supplementary input and Alt-release Unicode passed");return 0;
}
