#ifndef NEOMUTT_XP_TERMIOS_H
#define NEOMUTT_XP_TERMIOS_H
struct termios { unsigned long c_lflag; unsigned char c_cc[2]; unsigned long console_mode; };
#define ICANON 1
#define ECHO 2
#define VMIN 0
#define VTIME 1
#define TCSANOW 0
int tcgetattr(int, struct termios *);
int tcsetattr(int, int, const struct termios *);
#endif
