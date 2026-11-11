/* Runtime availability checks for the Windows media transport controls.
 * GPL-2.0-or-later. */
#include "compat.h"
#include "smtc.h"
#include "../player.h"

#include <stdio.h>
#include <string.h>

struct player_info player_info;
static int paused, stopped, nexts, prevs;

void player_pause(void) { paused++; }
void player_stop(void) { stopped++; }
void cmus_next(void) { nexts++; }
void cmus_prev(void) { prevs++; }

int win_resource(const char *name, const char **data, size_t *size)
{
	(void)name; (void)data; (void)size;
	return 0;
}

int main(void)
{
	static struct track_info track;
	player_info.ti = &track;
	player_info.status = PLAYER_STATUS_PLAYING;
	track.title = "Legacy title";
	track.artist = "Artist";
	track.albumartist = "Album artist";

	printf("console=%p\n", (void *)GetConsoleWindow());
	win_smtc_init();
	int available = win_smtc_available();
	printf("available=%d stage=%d result=0x%08lx\n", available,
			win_smtc_stage(), (unsigned long)win_smtc_result());
	win_smtc_metadata_changed();
	win_smtc_playback_status_changed();
	win_smtc_seeked();
	win_smtc_process();
	player_info.status = PLAYER_STATUS_PAUSED;
	win_smtc_playback_status_changed();
	player_info.status = PLAYER_STATUS_STOPPED;
	win_smtc_playback_status_changed();
	win_smtc_free();
	int released = !win_smtc_available();
	printf("%s\n", available && released ? "PASS" : "FAILED");
	return available && released ? 0 : 1;
}

const char *program_name = "check_smtc";
