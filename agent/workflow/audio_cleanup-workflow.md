# Local audio cleanup samples

* Inspect input audio channels, codec and sample rate with ffprobe. Read relevant task notes before choosing sample timestamps, particularly for edited film timelines.
* FFmpeg here includes `afftdn`, `highpass`, `equalizer`, `acompressor`, `dynaudnorm`, `loudnorm` and `alimiter`; no extra model download is needed for conventional dialogue enhancement. Check available filters rather than assuming a neural denoiser/model is installed.
* For dialogue, a conservative sample can combine light rumble removal and noise reduction, a modest low-mid cut and presence boost, and restrained level compression/normalization. These do not reconstruct missing words. Aggressive denoising can erase quiet consonants.
* Provide identical picture excerpts with original and processed audio for review. Encode the picture once and copy that video for the other variant. Clearly note whether comparisons are loudness matched; louder alone can sound clearer.
* Give adaptive filters several seconds of lead-in and tail, then trim to exact intended boundaries and reset timestamps. Preserve stereo unless a specific channel issue justifies a change.
* Use measured loudness normalization and check duration/channel layout. Numerical loudness and peak measurements do not substitute for listening; do not claim subjective improvement without hearing it.
* Record source, exact sample boundaries, output paths, filter settings and measurements in the task's context note. Keep whole-film processing separate until authorized.
