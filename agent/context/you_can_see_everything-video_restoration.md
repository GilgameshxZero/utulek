> Current status: the user approved all three individual edits and the final stitch is complete. Earlier restrictions against stitching and old running-session entries below are historical. The self-contained methodological handoff is [you_can_see_everything-edit_summary.md](you_can_see_everything-edit_summary.md). The final cut contains 247,694 frames at 24 fps and runs 2:52:00.583.

# You can see everything — video restoration

The user owns the footage and wants a clean platform copy. Do not modify the source. Work on a short sample until the user approves full-film processing.

* Source: `/home/gilgamesh/main.syncthing/snowfall/slush/fielder-you_can_see_everything.mp4`.
* Working copy and all user-facing clips: `/home/gilgamesh/main.syncthing/snowfall/local.syncthing/`.
* Working copy: `you_can_see_everything_source_copy.mp4`.
* Unchanged comparison: `you_can_see_everything_original_3220.mp4`.
* New tonal sample: `you_can_see_everything_sample_3220_tonal_restore_v2.mp4` (10 seconds from 3220, cropped x=176..1743, 1568x1080 at 24 fps, original audio copied).
* Settings, recovered matte, edge observations, report, and scripts: `you_can_see_everything_tonal_restore_v2/`. `correction.npz` has full-resolution `mask`, `forward_rgb_slope`, and `forward_rgb_offset`; `report.json` records parameters and unchanged input size/mtime.

The large transparent watermark must be corrected at its letter pixels through inverse brightness/contrast/color adjustment. The user rejected broad delogo blur, neighbor fill, and replacing image content. The v2 correction reverses an RGB affine blend: `(observed - mask*offset) / (1 + mask*(slope-1))`. It preserves original pixel detail. Alpha and tonal parameters are estimates; faint contours and compression residue remain. The small moving logo is entirely inside the cropped left bar during this sample. Do not assume that holds at other times.

The previous masks in `/tmp/onlyflix*mask*.png` are inaccurate and do not match the large actual lettering. The new mask was obtained from the temporal stationary high-pass signal across 113 frames in 2900..3800 seconds, with contour refinement. No optical flow or neighbor substitution was used for rendering. Calibration intermediates currently remain in `/tmp/restore-v2-*`; retain/query existing arrays before repeating this analysis.

The old `sample_3220.mp4` and `sample_3220_neighbor_restore.mp4` are unsuccessful approaches; do not present them as clean results.

## Second sample: small logo inside the picture

The user liked the 3220 sample and requested another short clip whose small moving logo survives the pillarbox crop. No full-film processing is authorized by this request.

* Selected section: 3510..3520 seconds (58:30), small logo entirely over a smooth wood-panel background, native tracking anchor approximately x230..240, y680..666.
* New output: `you_can_see_everything_sample_3510_tracked_logo.mp4`; comparison: `you_can_see_everything_original_3510.mp4`.
* Settings, tracking, per-frame background models, previews, and report: `you_can_see_everything_restore_3510/`.
* The large watermark fades through this region. The existing letter-only inverse tonal correction is scaled from 0.22 to 0.20 of its previous strength.
* Small-logo inverse compositing with measured alpha/RGB models and phase templates left colored outlines. Final approach uses a tight tracked icon/text mask and per-frame quadratic background reconstruction on the smooth wood panel. This is reconstruction of the small covered region, not exact recovery of its pixels. No spatial fill is applied to the large letters. Do not generalize success over this smooth wall to detailed or moving backgrounds.
* Calibration caches in `/tmp/small-logo-*.npz` and scripts `/tmp/small_logo*.py` retain the unsuccessful inverse fits and native frame tracking; query these before repeating analysis. Target sample's 240 patches, positions, and background fits are retained in the destination assets.
* During this turn the original `slush/fielder-you_can_see_everything.mp4` path was unavailable. The complete working copy remained readable and was used. Do not claim that this missing original was verified unchanged; record only the working-copy size/mtime check.

## Parallel processing preference and time estimate

The user approved both samples, asked how long the full film would take, and explicitly requested multithreading. Use parallel video processing for the full run. A full-film output was not launched during this timing estimate.

Machine inspected: Intel Core Ultra 7 258V, 8 CPU cores, ~21 GiB available memory and 265 GiB free disk at measurement. No `/dev/dri` exposed in this execution environment. Film has 248015 frames, 24 fps, duration 10334.041333 seconds.

Measured the approved crop + inverse tonal correction + libx264 `slow`, CRF 16 on the 10-second 3220 comparison, encoding to the null muxer. Float32 processing, decoder threads=1, encoder threads=2 per process; BLAS/OMP threads=1. Benchmark includes actual frame correction and encoding, but excludes full-film tracking, small-logo background repair, and review. Results are queryable in `context/you_can_see_everything-render_benchmark.json`:

* 1 process: 16.49 aggregate fps; estimated render only 4.18 hours.
* 2 processes: 31.82 aggregate fps; estimated render only 2.16 hours.
* 4 processes: 51.00 aggregate fps; estimated render only 1.35 hours (81 minutes).

Select 4 parallel processing workers, 2 encoder threads each, for subsequent full-film work. Allow roughly 3–6 hours total provisionally; the unmeasured tracking and complex-background restoration make this uncertain. This is a short-sample extrapolation, not a completed full-pipeline benchmark.

Eight representative frames were inspected at 180, 1200, 2400, 4200, 5400, 6600, 7800 and 9000 seconds; contact sheet `/tmp/snowfall-full-estimate-scenes.jpg`, timestamp index `/tmp/snowfall-estimate-scenes.json`. Both watermark placement/strength and background content vary. The large matte appears to change location/scale outside the calibrated section, so do not apply the fixed 3220 mask indiscriminately. The small mark sometimes crosses detailed content; the smooth-wall quadratic reconstruction is not suitable everywhere. Full processing needs a tracking/calibration pass and adaptive restoration before rendering chunks. Keep the complete working copy untouched and save render chunks/final video under snowfall/local.syncthing.

## Full run authorized and active

The user subsequently said: “okay. start the full render with this plan. in addition, clip me 10 seconds of every 10 minutes of processed footage so i can keep tabs. update me with progress while you work”. This overrides earlier notes saying full-film work had not been authorized or started. See `context/you_can_see_everything-full_render_run.md` for the active job, paths, pipeline, progress, and tool session IDs. The first four previews were delivered and the user instructed us to continue.

The authorized full run has now completed. Final file: `snowfall/local.syncthing/you_can_see_everything_restored_full.mp4` (6.77 GiB; all 248015 frames; 1568x1080; original AAC audio). All 18 requested 10-second previews are verified and indexed at `you_can_see_everything_full_restore/previews/INDEX.md`. See `context/you_can_see_everything-full_render_run.md` and the job's completion report for retained settings/results. The source working copy was unchanged.

## Final intro removal

The user requested removal of the opening OnlyFlix intro. The current preferred final version is `/home/gilgamesh/main.syncthing/snowfall/local.syncthing/you_can_see_everything_restored_no_intro.mp4`. The preceding restored full file was preserved.

The last platform-intro frame was video frame 150; frame 151 was the first film frame and an existing keyframe at PTS 78332 / 12288 = 6.374674479 seconds in the previous container. Stream-copied from that boundary, preserving all film titles and both streams without re-encoding. Verified 247864 remaining frames, total duration 10327.666667 seconds, video/audio starts both zero, and the first decoded frame MD5 exactly matches prior frame 151. Details: `context/you_can_see_everything-intro_trim.json`. Earlier progress previews retain their pre-trim timeline; they were not regenerated.
