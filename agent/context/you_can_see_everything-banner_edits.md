> Current status: the user approved all three individual edits and the final stitch is complete. Earlier restrictions against stitching and old running-session entries below are historical. The self-contained methodological handoff is [you_can_see_everything-edit_summary.md](you_can_see_everything-edit_summary.md). The final cut contains 247,694 frames at 24 fps and runs 2:52:00.583.

# Banner and insert edit previews (2026-10-25)

The user requested finding every “Watch Free Movies…” bottom banner, attempting inverse transparency restoration, and showing each individual edit before stitching the movie. They separately approved treating the full-screen platform bumper as an insert to cut. Do not stitch a new full movie before preview review. Keep updates sparse.

Input is `/home/gilgamesh/main.syncthing/snowfall/local.syncthing/you_can_see_everything_restored_no_intro.mp4`. All outputs are in sibling `you_can_see_everything_banner_edits/`. Source and full movie remain unchanged.

* Edit01: banner 1161.4166667–1176.4166667 seconds (00:19:21.417–00:19:36.417). Review window1158–1180,528 frames.
* Edit02: full-screen insert5107.375–5114.4583333 (01:25:07.375–01:25:14.458),170 frames. Cut preview has6 sec before+6 after; join at6sec. Preserve the film's “TWO YEARS LATER” title.
* Edit03: banner5923.0416667–5938.0833333 (01:38:43.042–01:38:58.083). Review window5920–5942,528 frames.

Global source scan reused20,668 retained raw thumbnails (2fps), detected only these3 overlays, then refined exact boundaries natively. Query retained analysis/scan_all.npz and event_01/event_03 arrays before repeating expensive work. Each event retains original native bottom240px and raw source black side bars for528 frames, matched clean reference frames, timing, offsets, and calibrated models.

Current renderer: /tmp/snowfall_banner_build/render_previews.py with repair_spatial.SpatialRepair. Calibrations: periodic_model.py then refine_phases.py then fit_transmission.py. Fit sources and artifacts retained under models/, scripts/. Large permanent mark restoration used earlier retained correction.npz; do not reapply to current input.

The bottom panel retains roughly18–20% film signal. Lettering and animated icon are much more opaque, so inverse brightness alone cannot recover those pixels. Current draft uses inverse RGB composite on transparent pixels and sparse harmonic fill for exact text/icon/outline masks. Artifacts remain: diagonal pattern, noise, smoothed icon area and text-shaped residue. Be candid; previews are drafts, not clean/exact restoration. Do not silently substitute whole strips from unrelated face references. Earlier attempted inverse-only methods produced stronger checker and bright text artifacts and should not be delivered as valid edits.

Period calibration found258.7356 native pixels (half-period ambiguity~129.37); original raw black right-side phase fitting was unreliable. refine_phases.py uses actual picture banner bands with high-pass correlation; still some ambiguity. Spatial emission/transmission fit needs BOTH even and odd columns: sampling only even columns leaves parity terms unidentified and causes black/white stripes. Current fit uses step3 and shared RGB transmission. Scripts are useful for further refinement, but quality remains limited by retained signal and compression.

Previews: previews/edit_01_00h19m21s_banner_preview.mp4, edit_02_01h25m07s_cut_preview.mp4, edit_03_01h38m43s_banner_preview.mp4. Before clips are in comparisons/. INDEX.md and media/report JSONs document exact intervals, limits and metadata. Last render invoked two parallel processes, encoder2threads each, BLAS1. No new full movie assembled.
