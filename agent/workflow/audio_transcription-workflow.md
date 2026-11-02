# Local audio transcription

## Available approach

* `ffprobe` and `ffmpeg` are installed. Inspect duration, channels and sample rate before processing.
* No speech-to-text tool was exposed in the October 26, 2026 session. A temporary virtual environment at `/tmp/dominic-transcription-venv` was created with `faster-whisper` 1.2.1. Treat `/tmp` paths as disposable and verify availability in later sessions.
* Official implementation/docs: https://github.com/SYSTRAN/faster-whisper . CPU inference can use `WhisperModel('small.en', device='cpu', compute_type='int8', cpu_threads=6, download_root=...)`, with English forced when appropriate, `beam_size=5`, `vad_filter=True`, and `word_timestamps=True`.
* Model and package downloads require network access. Use writable temporary cache directories, e.g. pip `--cache-dir /tmp/...`, `HF_HOME=/tmp/...`, and an explicit model `download_root`. A scoped sandbox escalation may be needed for network downloads. Do not upload private recordings to an unrelated service merely to bypass local setup.
* `HF_HUB_DISABLE_XET=1` and `HF_HUB_DISABLE_TELEMETRY=1` can simplify downloading the model. Downloaded package/model availability is separate from transcription success.

## Preserve usable results

* Save incremental timestamped text and JSONL segments in the relevant task's `context/` paths so an interrupted process still leaves queryable work. Include source audio path, model and options, source duration, and completion status.
* Keep raw recognition separate from edited notes. Preserve uncertain recognition instead of silently making up words, names or numbers. Compare proper names to existing correspondence where appropriate and record normalization.
* For decisions, speaker identity, compensation, dates and submission permission, read the surrounding dialogue and revisit unclear excerpts. Automated confidence is a screening aid, not proof.
* Do not invent speaker diarization. If the transcript is not diarized, infer speakers only when the dialogue clearly supports it, and mark ambiguous statements.
* Put useful call outcomes and action items near the top of the existing notes document, preserving pre-call material with a clear label. Link the source recording and retain timestamp references for significant commitments and unresolved points.
* Changes to Imperium outside the current workspace require the normal filesystem approval mechanism; explicit user requests authorize preparing and saving those notes, subject to tool access.

## Decoder compatibility

October 27, 2026: verified a fresh Python 3.14 virtual environment at `/tmp/preference-model-transcription-venv` with faster-whisper 1.2.1; downloaded `small.en` into `/tmp/preference-model-whisper-small-en`. These temporary paths may disappear. Package and model downloads required network escalation, but actual transcription ran locally under the normal sandbox without uploading audio. `BatchedInferencePipeline(model).transcribe(waveform, language='en', beam_size=5, vad_filter=True, word_timestamps=True, batch_size=8)` worked with CPU int8 and six threads; a 44-minute stereo Opus recording transcribed in about 202 seconds after setup. FFmpeg decoded it to 16 kHz mono float32 first. For short critical passages, ordinary `WhisperModel.transcribe` on an independently decoded excerpt helped recover omitted negatives and clarify spoken units such as “bips.” Add the excerpt's source offset to every returned timestamp. Repeated recognition is still not independent proof of a factual claim.

`faster-whisper` 1.2.1 with PyAV 19.0.2 failed because `av.open()` rejected `metadata_errors`. A local workaround is to decode with installed FFmpeg and pass a NumPy array to `transcribe`:

```python
pcm = subprocess.check_output([
    'ffmpeg', '-v', 'error', '-i', str(audio_path),
    '-f', 'f32le', '-ac', '1', '-ar', '16000', 'pipe:1',
])
waveform = np.frombuffer(pcm, dtype=np.float32)
segments, info = model.transcribe(waveform, language='en', beam_size=5,
                                  vad_filter=True, word_timestamps=True)
```

This avoids the incompatible PyAV decode call. A mono mix can merge voices; it is transcription, not speaker separation.
