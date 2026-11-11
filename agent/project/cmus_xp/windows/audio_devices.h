/* Native output selection/enumeration. GPL-2.0-or-later. */
#ifndef CMUS_WINDOWS_AUDIO_DEVICES_H
#define CMUS_WINDOWS_AUDIO_DEVICES_H
int win_waveout_device_id(const char *selection, unsigned int *id);
int win_list_output_devices(void);
#endif
