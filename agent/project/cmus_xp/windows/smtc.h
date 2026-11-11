/* Windows System Media Transport Controls integration. GPL-2.0-or-later. */
#ifndef CMUS_WIN_SMTC_H
#define CMUS_WIN_SMTC_H

#ifdef CMUS_NATIVE_WINDOWS

void win_smtc_init(void);
int win_smtc_available(void);
int win_smtc_stage(void);
long win_smtc_result(void);
void win_smtc_free(void);
void win_smtc_process(void);
void win_smtc_playback_status_changed(void);
void win_smtc_metadata_changed(void);
void win_smtc_seeked(void);

#else

#define win_smtc_init() ((void)0)
#define win_smtc_free() ((void)0)
#define win_smtc_process() ((void)0)
#define win_smtc_playback_status_changed() ((void)0)
#define win_smtc_metadata_changed() ((void)0)
#define win_smtc_seeked() ((void)0)

#endif

#endif
