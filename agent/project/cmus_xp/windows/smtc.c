/* Windows System Media Transport Controls integration. GPL-2.0-or-later. */
#include "smtc.h"

#ifdef CMUS_NATIVE_WINDOWS

#define INITGUID
#include "compat.h"
#include <roapi.h>
#include <winstring.h>
#include <windows.media.h>
#include <systemmediatransportcontrolsinterop.h>
#include <stdatomic.h>
#include <wchar.h>
#include <string.h>

#include "../cmus.h"
#include "../debug.h"
#include "../player.h"

static __x_ABI_CWindows_CMedia_CISystemMediaTransportControls *controls;
static __x_ABI_CWindows_CMedia_CISystemMediaTransportControlsDisplayUpdater *updater;
static __x_ABI_CWindows_CMedia_CIMusicDisplayProperties *music;
static EventRegistrationToken button_token;
static HANDLE console_window;
static atomic_int pending_action;
static int handler_registered;
static int initialized;

enum {
	ACTION_NONE,
	ACTION_PLAY,
	ACTION_PAUSE,
	ACTION_STOP,
	ACTION_NEXT,
	ACTION_PREV,
};

struct button_handler {
	__FITypedEventHandler_2_Windows__CMedia__CSystemMediaTransportControls_Windows__CMedia__CSystemMediaTransportControlsButtonPressedEventArgsVtbl *lpVtbl;
	LONG refs;
};

typedef __FITypedEventHandler_2_Windows__CMedia__CSystemMediaTransportControls_Windows__CMedia__CSystemMediaTransportControlsButtonPressedEventArgs Handler;

static HRESULT STDMETHODCALLTYPE handler_QueryInterface(
		Handler *this_, REFIID iid, void **object)
{
	struct button_handler *handler = (struct button_handler *)this_;
	if (!object)
		return E_POINTER;
	if (!IsEqualIID(iid, &IID_IUnknown)) {
		*object = NULL;
		return E_NOINTERFACE;
	}
	handler->refs++;
	*object = handler;
	return S_OK;
}

static ULONG STDMETHODCALLTYPE handler_AddRef(Handler *this_)
{
	return ++((struct button_handler *)this_)->refs;
}

static ULONG STDMETHODCALLTYPE handler_Release(Handler *this_)
{
	return --((struct button_handler *)this_)->refs;
}

static HRESULT STDMETHODCALLTYPE handler_Invoke(Handler *this_,
		__x_ABI_CWindows_CMedia_CISystemMediaTransportControls *sender,
		__x_ABI_CWindows_CMedia_CISystemMediaTransportControlsButtonPressedEventArgs *event)
{
	enum __x_ABI_CWindows_CMedia_CSystemMediaTransportControlsButton button;
	int action = ACTION_NONE;
	(void)this_;
	(void)sender;
	if (!event || FAILED(event->lpVtbl->get_Button(event, &button)))
		return S_OK;
	switch (button) {
	case SystemMediaTransportControlsButton_Play: action = ACTION_PLAY; break;
	case SystemMediaTransportControlsButton_Pause: action = ACTION_PAUSE; break;
	case SystemMediaTransportControlsButton_Stop: action = ACTION_STOP; break;
	case SystemMediaTransportControlsButton_Next: action = ACTION_NEXT; break;
	case SystemMediaTransportControlsButton_Previous: action = ACTION_PREV; break;
	default: break;
	}
	atomic_store(&pending_action, action);
	return S_OK;
}

static __FITypedEventHandler_2_Windows__CMedia__CSystemMediaTransportControls_Windows__CMedia__CSystemMediaTransportControlsButtonPressedEventArgsVtbl
handler_vtbl = {
	handler_QueryInterface,
	handler_AddRef,
	handler_Release,
	handler_Invoke,
};

static struct button_handler handler = { &handler_vtbl, 1 };

static HSTRING string(const char *value)
{
	HSTRING result = NULL;
	wchar_t *wide;
	if (!value || !*value)
		return NULL;
	wide = win_wide(value);
	if (!wide)
		return NULL;
	if (FAILED(WindowsCreateString(wide, (UINT32)wcslen(wide), &result)))
		result = NULL;
	free(wide);
	return result;
}

enum property { PROPERTY_TITLE, PROPERTY_ARTIST, PROPERTY_ALBUMARTIST };

static void set_string(enum property which, const char *value)
{
	HSTRING h;
	if (!music || !value || !*value)
		return;
	h = string(value);
	if (!h)
		return;
	switch (which) {
	case PROPERTY_TITLE: music->lpVtbl->put_Title(music, h); break;
	case PROPERTY_ARTIST: music->lpVtbl->put_Artist(music, h); break;
	case PROPERTY_ALBUMARTIST: music->lpVtbl->put_AlbumArtist(music, h); break;
	}
	WindowsDeleteString(h);
}

void win_smtc_init(void)
{
	HSTRING class_name;
	ISystemMediaTransportControlsInterop *interop = NULL;
	HRESULT hr;

	if (initialized)
		return;
	initialized = 1;
	if (FAILED(RoInitialize(RO_INIT_MULTITHREADED)) &&
			FAILED(RoInitialize(RO_INIT_SINGLETHREADED)))
		return;
	console_window = GetConsoleWindow();
	if (!console_window)
		return;
	class_name = string("Windows.Media.SystemMediaTransportControls");
	if (!class_name)
		return;
	hr = RoGetActivationFactory(class_name,
			&IID_ISystemMediaTransportControlsInterop, (void **)&interop);
	WindowsDeleteString(class_name);
	if (FAILED(hr) || !interop)
		return;
	hr = interop->lpVtbl->GetForWindow(interop, console_window,
			&IID___x_ABI_CWindows_CMedia_CISystemMediaTransportControls, (void **)&controls);
	interop->lpVtbl->Release(interop);
	if (FAILED(hr) || !controls)
		return;
	controls->lpVtbl->put_IsEnabled(controls, TRUE);
	controls->lpVtbl->put_IsPlayEnabled(controls, TRUE);
	controls->lpVtbl->put_IsPauseEnabled(controls, TRUE);
	controls->lpVtbl->put_IsStopEnabled(controls, TRUE);
	controls->lpVtbl->put_IsNextEnabled(controls, TRUE);
	controls->lpVtbl->put_IsPreviousEnabled(controls, TRUE);
	if (SUCCEEDED(controls->lpVtbl->get_DisplayUpdater(controls, &updater)) && updater) {
		updater->lpVtbl->put_Type(updater, MediaPlaybackType_Music);
		if (FAILED(updater->lpVtbl->get_MusicProperties(updater, &music)))
			music = NULL;
	}
	if (SUCCEEDED(controls->lpVtbl->add_ButtonPressed(controls,
				(void *)&handler, &button_token)))
		handler_registered = 1;
	d_print("smtc: initialized\n");
}

void win_smtc_free(void)
{
	if (handler_registered && controls) {
		controls->lpVtbl->remove_ButtonPressed(controls, button_token);
		handler_registered = 0;
	}
	if (music) {
		music->lpVtbl->Release(music);
		music = NULL;
	}
	if (updater) {
		updater->lpVtbl->Release(updater);
		updater = NULL;
	}
	if (controls) {
		controls->lpVtbl->Release(controls);
		controls = NULL;
	}
}

void win_smtc_playback_status_changed(void)
{
	enum __x_ABI_CWindows_CMedia_CMediaPlaybackStatus status;
	if (!controls)
		return;
	switch (player_info.status) {
	case PLAYER_STATUS_PLAYING: status = MediaPlaybackStatus_Playing; break;
	case PLAYER_STATUS_PAUSED: status = MediaPlaybackStatus_Paused; break;
	default: status = MediaPlaybackStatus_Stopped; break;
	}
	controls->lpVtbl->put_PlaybackStatus(controls, status);
}

void win_smtc_metadata_changed(void)
{
	struct track_info *ti = player_info.ti;
	if (!controls || !updater || !music)
		return;
	set_string(PROPERTY_TITLE, ti ? ti->title : NULL);
	set_string(PROPERTY_ARTIST, ti ? ti->artist : NULL);
	set_string(PROPERTY_ALBUMARTIST,
			ti && ti->albumartist ? ti->albumartist : (ti ? ti->artist : NULL));
	updater->lpVtbl->Update(updater);
	win_smtc_playback_status_changed();
}

void win_smtc_seeked(void)
{
	/* Position reporting is not exposed by the base SMTC interface. */
}

void win_smtc_process(void)
{
	int action = atomic_exchange(&pending_action, ACTION_NONE);
	if (action == ACTION_NONE)
		return;
	switch (action) {
	case ACTION_PLAY:
		if (player_info.status != PLAYER_STATUS_PLAYING)
			player_pause();
		break;
	case ACTION_PAUSE:
		if (player_info.status == PLAYER_STATUS_PLAYING)
			player_pause();
		break;
	case ACTION_STOP:
		player_stop();
		break;
	case ACTION_NEXT:
		cmus_next();
		break;
	case ACTION_PREV:
		cmus_prev();
		break;
	}
}

#endif
