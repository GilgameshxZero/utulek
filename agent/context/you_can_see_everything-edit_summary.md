# You can see everything — edit summary and methodological handoff

## Purpose and status

The user said they filmed the footage, lost the original, and wanted to recover a cleaner version from a platform copy. They approved short watermark-restoration samples, then authorized the full restoration. They subsequently requested removal of the opening platform intro, identification and repair of all bottom advertising banners, and removal of a full-screen platform insert. They reviewed three individual edit previews, liked them, and authorized the final stitch using exactly those edits.

The final stitch is complete. Its video contains **247,694 frames at 24 fps**, runs **2:52:00.583**, and measures **1568 × 1080**. It contains the two approved banner repairs and the approved insert cut, in addition to the previously approved permanent-watermark restoration, side-bar crop, and opening-intro removal. The complete working source and the input restoration were not modified during this final assembly.

This handoff deliberately contains no filenames, storage locations, or dependencies on surviving artifacts. Assume every intermediate, model, sample, and script has been destroyed, and that the final video cannot be inspected. It preserves enough reasoning and measurements to explain the work, not to recreate the exact fitted masks or every pixel from memory. The user plans to ask how the edits were discovered and derived. Distinguish actual measurements, model assumptions, rejected experiments, and irrecoverable information when answering.

## User preferences and distinctions that mattered

* The user rejected a broad delogo rectangle that smeared the center of the image. The large faint watermark affected the letters, not an entire horizontal band.
* For that large watermark, they wanted the original pixel values adjusted through an estimated inverse brightness/contrast/color operation. The accepted rendering does that inside a letter mask; it does not replace those letters with neighboring image content.
* This restriction did not make opaque marks invertible. The approved small moving logo repair reconstructed only its tight footprint. The banner repairs also reconstruct opaque lettering and the animated icon, while reversing the translucent panel elsewhere.
* Short samples and individual edits were approved before whole-film assembly. The user explicitly requested multithreading and later asked for fewer progress messages.
* The last three previews still showed diagonal residue, compression noise, and smoothing around opaque artwork. These limits were disclosed before the user approved them. Do not describe the result as exact recovery of the lost original.

## Coordinate systems and exact edit accounting

The platform copy had a 1920 × 1080 picture canvas, 24 fps, and **248,015 video frames**. Its first video timestamp was approximately 0.083 seconds rather than zero. This distinction between container timestamps and video-frame-relative time mattered during seeks and trimming.

The active picture was cropped horizontally from original x = 176 through x = 1743 inclusive: 176 pixels removed on each side, leaving 1568 × 1080. The crop did not resize the picture vertically or stretch it. Track overlays in original full-frame coordinates before deciding whether the crop already removes them.

The opening platform intro occupied the first **151 video frames**, or **6.2916667 seconds** of video. Its first retained film frame was an existing keyframe. The earlier container's cut timestamp was **6.374674479 seconds**; this is not the same number as the intro's frame duration because of the original video timestamp offset. That trim was stream-copied and retained 247,864 frames. Its first decoded frame was checked against the intended film-start frame. Film titles were preserved.

All following discovery times refer to the intro-trimmed movie **before** the later insert removal. Intervals use an inclusive start and exclusive end.

| Edit | Frame interval in that movie | Visible interval | Preview coverage |
| --- | --- | --- | --- |
| First bottom banner | [27,874, 28,234), 360 frames | 00:19:21.417–00:19:36.417 | 22 seconds, 00:19:18–00:19:40 |
| Full-screen platform insert | [122,577, 122,747), 170 frames | 01:25:07.375–01:25:14.458 | Six seconds before the cut and six seconds after it |
| Second bottom banner | [142,153, 142,514), 361 frames | 01:38:43.042–01:38:58.083 | 22 seconds, 01:38:40–01:39:02 |

The full-screen insert was removed entirely: **170 frames = 7.0833333 seconds**. The nearby film title “TWO YEARS LATER” was preserved. The cut occurs six seconds into its 12-second preview.

After this cut, subtract 7.0833333 seconds from all later old-timeline times. Thus the second repaired banner appears at approximately **01:38:35.958–01:38:51.000** in the final movie. The first banner's time is unchanged. Across the intro and inserted bumper, 321 original video frames were removed: 248,015 − 151 − 170 = **247,694**. The banner repairs themselves removed no film frames.

## Large transparent wordmark: discovery and mask construction

The faint OnlyFlix wordmark stayed in approximately the same image location while unrelated shots changed beneath it. That gave a useful separation principle: a platform graphic produces repeated stationary edges; scene details usually do not.

A calibration set contained **113 frames across roughly 2900–3800 seconds**. A first pass used reduced-resolution frames; native-resolution grayscale analysis then concentrated on the central y = 320–839 region. For each image I, form a spatial high-pass response:

`Q = I − GaussianBlur(I, sigma)`.

Take a temporal median of Q over diverse shots. Persistent glyph edges remain more consistently than moving scene texture. Gaussian scales of approximately 4 and 20 native pixels served different purposes: the smaller scale localized contours, and the larger scale emphasized the broader letter structure. A rough glyph prior limited the solution to plausible wordmark geometry.

A high-pass image alone does not determine filled letter interiors: it suppresses low spatial frequencies. Regularized inversion combined the stationary high-pass signal with the rough mask prior. In Fourier notation, for a high-pass transfer function H, observed stationary response Q, prior mask M0, and regularization lambda:

`M_hat = (conjugate(H) × Q_hat + lambda × M0_hat) / (abs(H)^2 + lambda)`.

For a Gaussian subtraction, `H(f) = 1 − exp(−2 × pi^2 × sigma^2 × |f|^2)`. This is a regularized estimate of glyph geometry, not proof that every temporal-median feature is a logo. Subsequent refinements used local stationary-edge zero crossings, modest morphological cleanup, and narrow contour adjustments. A signed-distance transition and approximately 0.65-pixel Gaussian softening represented antialiasing rather than applying a hard binary cut.

One later refinement normalized high-pass observations by their estimated brightness-dependent overlay amplitude before taking a median. This reduced the tendency of different underlying scene brightnesses to distort the inferred edge strength. Several regularization strengths were tried; the exact selected final value cannot be established from this handoff alone.

## Large wordmark: deriving and applying the inverse

The accepted model was an **effective per-channel affine transform in decoded RGB values**, not a separately identified gamma curve. For a fully covered letter pixel:

`Y_c = a_c × X_c + b_c`,

where X is the clean image, Y is the platform image, and c denotes an RGB channel. For an antialiased letter mask m and time-dependent strength s:

`Y_c = [1 + m × s × (a_c − 1)] × X_c + m × s × b_c`.

Therefore the rendered correction was:

`X_est_c = (Y_c − m × s × b_c) / [1 + m × s × (a_c − 1)]`.

Outside the letter mask, m = 0, so the operation is the identity. The output uses the observed pixel itself; it does not fill the large letters from surrounding pixels. It preserves underlying detail to the extent that the blend model and mask are accurate.

How were a and b estimated without a clean original? Find relatively straight glyph boundaries, estimate the boundary normal, and sample on both sides. Average a few samples along the tangent to suppress compression noise. Extrapolate each side's nearby intensities toward the boundary; where the real scene changes smoothly across that boundary, the discontinuity estimates the platform transform. Many boundaries and many unrelated frames reduce dependence on any single local scene assumption.

The implementation used reduced-resolution samples approximately 3 and 6 pixels from each side of a boundary, with tangent offsets around −2, 0, and 2 pixels. It rejected corners, competing nearby glyph edges, steep local image gradients, and clipped intensities. A robust soft-L1 fit reduced the influence of scene edges mistaken for watermark boundaries.

For a symmetric fit, define `d = inside − outside` and `u = (inside + outside)/2`, and fit `d = c0 + k × u`. If inside = a × outside + b, algebra gives:

`a = (2 + k) / (2 − k)` and `b = c0 × (a + 1)/2`.

The recovered RGB forward parameters for the reference calibration were approximately:

| Channel | a, multiplicative slope | b, additive offset in 0–255 values |
| --- | --- | --- |
| Red | 0.9517187 | 10.03435 |
| Green | 0.9471530 | 10.08775 |
| Blue | 0.9439609 | 10.15364 |

These describe a modest contrast reduction plus a brightness lift. Do not interpret them as an independently proven physical alpha or exact branding color. A composited white logo, a color change in gamma-encoded values, and other effective transforms can be partly confounded.

The reference transform did not apply equally strongly throughout the film. A coarse temporal measurement of the stationary lettering was median/Gaussian smoothed and normalized to the reference. Strength was interpolated between samples during rendering. The second approved sample needed only about one fifth of the reference correction. The first 30 seconds of the original timeline were disabled where the large overlay was absent. Early speculation that the entire large graphic changed position or scale was not supported by later native-frame inspection; variable strength was the main issue.

The user's suggestion about gamma/contrast/brightness was therefore implemented as an inverse RGB blend. It was not a universal inverse gamma estimate. Faint contours remain possible because the mask, effective transform, opacity, and compressed input are all imperfect.

## Small moving logo: tracking and constrained reconstruction

The first successful large-mark sample was around 53:40, when the small logo lay inside a side bar and disappeared in the crop. A second approved 10-second sample around 58:30 deliberately put the small logo inside the retained picture, over a smooth wood-panel background.

An early tracking pass looked only on the left and was insufficient. Full-picture validation showed that the logo moved across the entire frame, including the right and top edges. Coarse tracking used two samples per second; native template correlation refined the position in a small search neighborhood for rendered frames. The glyph/icon template was high-pass filtered and adjusted to reduce scene and neutral-luminance interference.

The logo's approximate template footprint was 118 × 34 native pixels, surrounded by a 136 × 50 working patch. A narrow dilated/softened support mask limited changed pixels. Smooth coordinate interpolation was used between nearby track samples; jumps greater than roughly 30 pixels used the nearer sample instead. A tracking confidence below about 0.37 skipped repair to avoid damaging unrelated content.

Reconstruction depended on the visible background:

* **Smooth surfaces:** Fit each RGB channel outside the logo mask to `p(x,y) = c0 + c1*x + c2*y + c3*x^2 + c4*x*y + c5*y^2`. Robust reweighting reduced the effect of real edges or contaminated samples. When the fit's RMS error was below about 3 intensity units, evaluate this quadratic inside the logo footprint. Exclude cropped black bars from the background fit.
* **Detailed regions:** First create a harmonic boundary fill inside the mask. Search nearby unobscured frames about ±2, ±4, and ±8 seconds away for the same local scene using only context outside the logo. Require normalized correlation above about 0.7, refine translation, and fit limited RGB gain/offset differences. Accept temporal pixels only when enough unoccluded context agrees and a robust photometric error is below about 8 intensity units. Use those pixels where valid, retaining harmonic fill elsewhere.

A harmonic fill solves the discrete Laplace equation inside the masked region with surrounding known pixels held fixed. For each unknown pixel p, `4*u(p) − sum(u(q) for its four neighbors) = 0`; known boundary contributions move to the right-hand side. Sparse factorization can be reused for a fixed mask and solved for all three color channels. It produces a smooth continuation, not authentic lost texture.

The final small-logo method is reconstruction. A clean wall can look convincing; hair, faces, moving objects, and textured backgrounds are harder. The successful smooth-wall sample did not establish that all detailed regions could be restored exactly.

## Full-film processing before the final three edits

The restoration ran on an 8-core CPU using **four parallel workers**, two encoder threads per worker, and one BLAS/OpenMP thread per process to prevent numerical libraries from oversubscribing the machine. Video was processed in approximately ten-minute chunks. Native narrow image bands and half-resolution full frames at two samples per second reduced repeated decoding and supported tracking and reference searches.

Each frame received the letter-only inverse correction, the applicable small-logo repair, and the side-bar crop. The video encoder was H.264 through libx264, slow preset, CRF 16, 24 fps, 8-bit yuv420p, with BT.709 tags in the main render. Chunks used short closed GOPs of up to 48 frames. Original audio was copied at that stage. Ten-second progress previews were provided at ten-minute intervals, 18 in total. Those old progress previews used the timeline before the later intro and inserted-bumper cuts.

A dark short-clip benchmark achieved about 51 aggregate fps with four workers, suggesting roughly 81 minutes for encoding alone. Actual work included tracking, varied grain and texture, temporal reconstruction, and analysis, so that estimate was not a reliable total-job duration. Do not promise another film the same speed.

## Finding all bottom advertisements

The user's example near 1:38:47 established the banner's appearance and bottom-of-frame position. The scan reused **20,668 full-frame thumbnails at two samples per second**. The raw source's black side bars were particularly useful: normally they should remain almost black, while an overlaid banner contributes its own graphics there.

For each thumbnail, inspect lower portions of both side bars, reduce their RGB values to row-brightness profiles, and score rows that rise above a small black-level threshold. Group nearby positive samples into candidate events. Exclude the opening title/intro interval from this later-banner scan. Inspect contact sheets to classify each event rather than assuming every positive signal is a translucent banner.

This detected two bottom banners and one separate full-screen animated bumper. Native-rate extractions around those candidates refined first and last affected frames and distinguished a roughly 15-second banner from a 7.083-second full-screen insert. This was a full-duration coarse scan followed by precise analysis of detections, not a guarantee that an entirely different kind of advertisement could never escape that particular detector.

For each banner, retain several seconds of clean context before and after the visible interval. In this case each preview window was 22 seconds. Analysis used the bottom 240 native rows, y = 840–1079, plus raw-source side-bar observations. The panel's top outline was around y = 884 and its lower outline around y = 1045. Tracking these lines measured the slide-in and slide-out displacement. Subpixel vertical estimates used roughly one-eighth-pixel steps. The first occurrence's stationary raster was about one native pixel lower than the second's.

## Banner restoration: separating the problems

The banner was not a simple uniform tint. It contained a dark translucent patterned panel, white outlines, opaque or nearly opaque text, an animated icon, and a typewriter entrance with a moving cursor. Treating all of these as one fixed brightness change failed.

For the background panel, use the effective per-channel model:

`Y(x,y,t,c) = T(x,y,t) × X(x,y,t,c) + E(x,y,t,c)`.

T is the surviving clean-picture fraction; E is the panel's premultiplied RGB contribution. The estimated T over most of the panel was approximately **0.18–0.20**. The inverse is `(Y − E)/T`. For example, if Y = 60, E = 40, and T = 0.2, X is estimated as 100. A one-unit error in Y or E becomes approximately five units in the result. This explains amplified compression noise and pattern residue even when the average blend is well estimated.

Foreground artwork needs a second compositing layer:

`Y = V × (T_bg × X + E_bg) + F`,

where V is foreground visibility/transmission and F is premultiplied foreground color. When V approaches zero, the clean signal is gone. No contrast or gamma adjustment can uniquely recover an arbitrary image from a fully opaque pixel. Observations of almost identical lettering pixels over very different underlying scenes supported this limitation.

The final approved repair therefore inverted the translucent panel and reconstructed only the opaque text, icon, and border masks. It did not recover hidden opaque pixels by dividing them by a guessed opacity.

## Banner references, patterned background, and fitting

Unobscured views of the same scene were sought within about **±90 seconds**, excluding the affected intervals and the full-screen insert. A coarse descriptor narrowed candidates; robust affine alignment was fitted to the uncovered upper image, above y = 840. RGB gain and offset compensated modest photometric differences. This avoided aligning directly to the platform artwork.

Reference alignment was good in some repeated shots and poor in others. The second banner's early wide salon shot had errors around 4 intensity units, while a bearded close-up appearing only beneath the banner could have errors around 18. Several other usable references were around 5–8. Matches above approximately 9 were excluded from template training. A similar-looking but differently posed face was not a trustworthy clean replacement.

These references supplied calibration targets for the banner blend. The final opaque-shape renderer used boundary reconstruction; a proposed reference-guided icon variant was not used in the approved previews. Do not describe whole clean strips or neural image synthesis as the final method.

The panel had diagonal repeating artwork plus fine alternating-column structure. A donor early in the second occurrence, about 20 frames after its onset, still had an untyped right-hand region. That region helped estimate the panel without contaminating the template with future text. Using a donor after the text appeared had previously baked letters into the background model.

The fitted full repeating period was about **258.7356 native pixels**, with a strong half-period ambiguity near **129.37 pixels**. A restricted early search around that half-period produced poor results. A wider fit and inspection exposed the ambiguity.

The model represented emission with row-dependent Fourier coefficients in horizontal position and animation phase, plus alternating-column terms. The later spatial fit used roughly 20 emission harmonics, alternating-column interactions through about 8 harmonics, and 10 harmonics for the shared RGB transmission. Three-row bands pooled observations and light vertical smoothing reduced instability. The transmission fit used regularization and bounded reconstructed denominators rather than allowing arbitrary amplification; a floor near 0.04 limited the worst divisions.

Phase estimates from a narrow black side bar alone were unreliable: several pattern phases can resemble one another there. Refinement used high-pass correlations across multiple actual banner bands outside the main text area, covering much more of the image.

One important implementation failure was sampling only even x positions while also fitting an even/odd-column component. That makes the ordinary and parity terms indistinguishable during training; evaluating odd columns then produced artificial black/white stripes. Sampling every third column included both parities and removed that degeneracy. This is a general identifiability issue, not merely an encoding artifact.

## Final banner rendering and its limitations

For each affected frame, shift the canonical panel model by the measured slide displacement, estimate the animated pattern phase, and invert emission/transmission in the panel support. Unaffected pixels outside that support are retained. Only the bottom working strip is processed; the upper image is unchanged before encoding.

A stable text mask was derived from bright stationary lettering across training frames, constrained to the actual text layout, then expanded enough to include antialiasing and compression fringes. The icon had a tight rounded rectangular mask. The outlines received thin horizontal masks. During the first roughly 4.4 seconds, the visible text/cursor mask was derived from the current residual so that future, untyped letters were not subtracted from the image.

Opaque mask pixels were filled by a sparse harmonic solve using nearby recovered boundary pixels. Mask factorizations were cached when possible. This differs from filling the entire 1568-pixel-wide band: most panel pixels still come from the inverse operation on their own observed values. Nevertheless, connected dense text can leave a smoother strip and text-shaped or color residue, and the animated icon's footprint is visibly smoother.

The two approved 22-second previews contained the entire slide-in, typing, stationary display, and slide-out, with unchanged comparison clips available during review. Both were encoded with the same broad H.264 quality settings as the restoration. The user explicitly approved the results after being told about remaining pattern and smoothing artifacts.

## Rejected experiments and what they established

* A large rectangular delogo filter smeared the central picture and ignored the actual letter geometry. It was rejected by the user.
* Neighbor-fill or removelogo tests on the large watermark produced artifacts. They were not the accepted method for that mark.
* A preliminary small optical-flow model did not produce a reliable inverse transform; fits were close to identity and were not evidence of a solved restoration. Neural optical flow was not part of the final approved renderer.
* Inverting the banner with one static average panel left animated diagonal patterns, glowing letters, or outlined text. The background animation and opaque foreground needed different treatment.
* Regressing all foreground pixels as if they preserved 20% of the picture was wrong. Opaque artwork required masking and reconstruction.
* Estimating only an emission curve while fixing one transmission everywhere did not fully capture spatial variation and residual compression effects. The spatial emission/transmission model improved the result but did not remove every artifact.
* Reference frames were rejected when the underlying pose, object placement, or photometric match was poor. A low-error scene alignment in one shot did not justify substituting a different close-up later.

These failures explain the final design. They should not be represented as successful intermediate edits or as guarantees that the inverse is exact.

## Final assembly with minimal new encoding

The final assembly reused all three approved preview videos: 22 + 12 + 22 = **56 seconds**, or **1344 frames**. There was no new restoration pass on them. Longer unchanged portions were copied from the already-restored movie.

Arbitrary H.264 cuts cannot always be copied safely because predicted frames depend on pictures elsewhere in a GOP. The solution was to inspect keyframe positions, stream-copy complete internal GOP spans, and encode short bridge portions between those safe keyframes and the exact desired boundaries. Closing a copied range at a keyframe boundary also avoids keeping B-frames whose reference pictures were discarded.

The resulting video accounting was:

* **246,211 original-restoration frames** copied as compressed video.
* **1344 approved-preview frames** reused as compressed video.
* **139 boundary frames** newly encoded, about **5.792 seconds** total.
* **170 insert frames** removed.

The six new boundary runs contained 14, 12, 45, 12, 40, and 16 frames. They used libx264 slow, CRF 16. Those few frames can have another generation of compression loss. Approximately **99.944% of final video frames required no new video encoding** in this assembly.

The final concat/mux itself used stream copying. An MP4 container does not apply another image compression pass. Matching resolution, frame rate, codec family, pixel format and compatible parameters made reuse practical. Parameter-set handling and timestamps still matter; concatenation is not validated merely because a player opens the result.

Audio was handled separately from preview audio to preserve one continuous source timeline. At 48 kHz, the insert boundaries were sample indices **245,154,000** and **245,494,000**. Removing their difference deletes exactly **340,000 samples = 7.0833333 seconds**. The retained audio was concatenated and encoded once to AAC at **192 kbps**, then muxed with copied video. This audio pass has generation loss, but avoids repeated preview-encoder delays and permits an exact sample cut. The final audio is roughly 0.388 seconds shorter than video, consistent with the pre-existing input's shorter audio tail; no time stretching was introduced.

A short bridge's container-reported duration differed from its frame-derived duration by about 0.000325 seconds because of container-duration rounding. Its packet timestamps and 14-frame count were correct. Explicit frame-derived concatenation durations and final packet checks avoided interpreting that rounding as a missing frame.

## What was checked at completion

The final video contains exactly **247,694 packets/frames**, with one presentation timestamp every **512 ticks in a 1/12,288-second time base**: uniform 24 fps with no gaps or duplicate presentation times. Decode timestamps are strictly increasing. Video duration is **10,320.583333 seconds**, starting at zero. Both streams are present; final audio duration is approximately 10,320.195667 seconds.

Every decoded frame of each approved preview was hashed and compared against its corresponding final-movie section. All **1344 frames matched pixel-for-pixel**. The first and last three decoded frames of each copied original span also matched their source counterparts. All **139 new bridge frames** decoded without errors. Input sizes and modification times were checked against their initial values.

This was targeted verification of the edits, joins, copied-span endpoints, and full timeline structure. It was not a fresh visual review or full decode of every unchanged frame in the entire movie. State that limit if asked about verification coverage.

## Answering future methodological questions

Explain the work as three distinct operations: estimating an inverse transform where picture signal survives; reconstructing tightly masked areas where opaque artwork destroyed that signal; and cutting a true inserted segment with no underlying movie to recover.

The mathematical models are estimated, not uniquely identified physical truths. The final parameters and masks depended on this footage, its encoded color values, repeated shots, and observed platform graphics. Another video needs new calibration. RGB inversion can amplify noise; harmonic fill invents smooth continuation; temporal references only help when they depict the same content accurately.

Without surviving media, a later agent can explain the equations, reasoning, thresholds, timing, experiments, quality limits, and final assembly from this account. It cannot inspect an unrecorded frame, recover the exact learned mask, establish additional overlay occurrences, or claim to have reproduced the output. Be explicit about those boundaries rather than inventing missing evidence.
