/* Minimal WinMM model used only by local regression checks. GPL-2.0-or-later. */
#ifndef CMUS_TEST_WAVE_API_H
#define CMUS_TEST_WAVE_API_H
#include <stdint.h>
typedef uint64_t ULONGLONG;
typedef void *HWAVEOUT;
typedef struct { unsigned long Data1; unsigned short Data2, Data3; unsigned char Data4[8]; } GUID;
typedef struct { unsigned short wFormatTag, nChannels; unsigned int nSamplesPerSec, nAvgBytesPerSec; unsigned short nBlockAlign, wBitsPerSample, cbSize; } WAVEFORMATEX;
typedef struct { WAVEFORMATEX Format; union { unsigned short wValidBitsPerSample; } Samples; unsigned int dwChannelMask; GUID SubFormat; } WAVEFORMATEXTENSIBLE;
typedef struct { char *lpData; unsigned int dwBufferLength, dwFlags; } WAVEHDR;
typedef struct { unsigned int wType; } MMTIME;
enum { MMSYSERR_NOERROR, MMSYSERR_ALLOCATED, MMSYSERR_INVALHANDLE, MMSYSERR_NODRIVER,
 MMSYSERR_BADDEVICEID, MMSYSERR_NOMEM, MMSYSERR_NOTSUPPORTED, WAVERR_STILLPLAYING,
 WAVERR_UNPREPARED, WAVERR_BADFORMAT, WAVERR_SYNC };
#define WHDR_PREPARED 1
#define WHDR_DONE 2
#define WAVE_MAPPER 0
#define CALLBACK_NULL 0
#define WAVE_FORMAT_PCM 1
#define WAVE_FORMAT_EXTENSIBLE 0xfffe
#define TIME_BYTES 4
ULONGLONG GetTickCount64(void);
void Sleep(unsigned int);
int waveOutOpen(HWAVEOUT *, int, const WAVEFORMATEX *, int, int, int);
int waveOutClose(HWAVEOUT);
int waveOutReset(HWAVEOUT);
int waveOutPrepareHeader(HWAVEOUT, WAVEHDR *, int);
int waveOutUnprepareHeader(HWAVEOUT, WAVEHDR *, int);
int waveOutWrite(HWAVEOUT, WAVEHDR *, int);
int waveOutPause(HWAVEOUT);
int waveOutRestart(HWAVEOUT);
int waveOutGetPosition(HWAVEOUT, MMTIME *, int);
#endif
