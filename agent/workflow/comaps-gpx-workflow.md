# CoMaps export workflow

Pull saved places and recorded tracks from CoMaps on the Mudita Kompakt, rebuild the GPX files in `monochrome/module/map`, and summarise the newest run.

## When to use

After recording a new track, or any time the user wants `monochrome/module/map/place.gpx` and `track.gpx` brought up to date with the phone.

## Environment

* Phone: Mudita Kompakt, Android 12, Magisk root, adb over TCP/IP at `10.8.58.57:5555` (port 5555 is already open; no pairing needed).
* `adb` is on `PATH` from the scrcpy bundle.
* CoMaps package: `app.comaps.fdroid`, data in `/data/user/0/app.comaps.fdroid`.
* Bookmarks and tracks live as KML in `/data/user/0/app.comaps.fdroid/files/bookmarks/`:
  * `place.kml` — one Placemark per saved place (764 as of 2026-11-11)
  * `track.kml` — one Placemark per recorded track, either `gx:Track` (with times) or `LineString` (geometry only)
* These are app-private, so `su -c` is required. The app is **not** debuggable, so `run-as` will not work.

## Normal run

```sh
cd ~/main.syncthing/utulek/agent
project/comaps-export/refresh.sh                 # pull + convert + summary
project/comaps-export/refresh.sh --no-pull       # reconvert existing KMLs only
```

That script force-stops CoMaps, pulls both KMLs into `.obj/comaps_export/`, repairs any unlocated places, writes `place.gpx` and `track.gpx` into `monochrome/module/map`, and prints the newest track's stats.

## Manual equivalent

```sh
adb connect 10.8.58.57:5555
adb shell "su -c 'am force-stop app.comaps.fdroid'"
adb shell "su -c 'cp /data/user/0/app.comaps.fdroid/files/bookmarks/place.kml /data/local/tmp/p.kml'"
adb pull /data/local/tmp/p.kml .obj/comaps_export/place.kml
adb shell "su -c 'rm -f /data/local/tmp/p.kml'"
```

Repeat for `track.kml`, then convert:

```sh
python3 project/comaps-export/kml2gpx.py .obj/comaps_export/track.kml monochrome/module/map/track.gpx --name=track
python3 project/comaps-export/kml2gpx.py .obj/comaps_export/place.kml monochrome/module/map/place.gpx --name=place
python3 project/comaps-export/summarize_run.py monochrome/module/map/track.gpx
```

Force-stop the app before copying files out of it: CoMaps rewrites `place.kml`/`track.kml` on changes and on exit, and a pull during a write can catch a partial file.

## GPX format

`kml2gpx.py` reproduces CoMaps' own GPX export byte for byte: `creator="CoMaps"`, the `gpxx` / `gpx_style` / `xsi` namespaces, `<metadata><name>` taken from the KML file's base name, per-track `<gpxx:DisplayColor>` plus `gpx_style:line` and `xsi:gpx` colours, and 6-decimal trimmed coordinates. The track conversion was verified identical to an app-exported `track.gpx`; treat any diff as a converter bug.

Two consequences worth remembering:

* Coordinates are printed with `%.6f` and trailing zeros trimmed. CoMaps itself rounds bookmarks through 32-bit floats on export, so its own files can differ from ours by up to ~5e-6° (~0.5 m) with no real change in position.
* `track.kml` gives every track `<mwm:localId>0</mwm:localId>` — a 30-way collision. Do not rely on it for identity when re-importing.

## Repairing unlocated places

Some imported places carry `0,0,0` with the name `0,0,0` and a Google Maps link in the description. They are recoverable from the link's `q=` parameter (a street address, or a place name) and occasionally `cid=`.

```sh
python3 project/comaps-export/geocode.py  .obj/comaps_export   # US -> Census batch, rest -> Photon
python3 project/comaps-export/refine.py   .obj/comaps_export   # fixes confidence, Nominatim retries, manual overrides
python3 project/comaps-export/patch_place.py .obj/comaps_export  # -> place_fixed.kml + geocode_report.tsv
```

* Results are cached in `.obj/comaps_export/geocode_cache_v2.json`, keyed by the original `q` string; delete an entry to re-resolve it.
* `geocode_report.tsv` lists every repaired place with its confidence, the assigned name, coordinates, and the geocoder's match string. `high` = US state/ZIP or exact Census match; `medium` = non-US city/landmark matched by name; anything lower needs a manual look.
* Names are set to the original address text (the app stores names separately from `description`, so the Google link stays intact as provenance).
* The one `?cid=` bookmark is resolved by following `https://maps.google.com/?cid=<id>` and reading the `center=` parameter, then reverse-geocoding the point for a name.
* Unit numbers (`#326`) and non-US addresses usually miss the Census match; those fall through to Photon. Watch out for `UK` being read as a US state code — the refine step maps it to `, England`.
* `refine.py` has a `MANUAL` table for entries the generic pipeline gets wrong (e.g. "Mekong River" resolving to a street in Toronto). Add to it rather than hand-editing output.

## Putting repaired data back into CoMaps

The app reads these files at startup, so replace them while it is stopped:

```sh
adb push .obj/comaps_export/place_fixed.kml /data/local/tmp/place_fixed.kml
adb shell "su -c 'am force-stop app.comaps.fdroid'"
adb shell "su -c 'cat /data/local/tmp/place_fixed.kml > /data/user/0/app.comaps.fdroid/files/bookmarks/place.kml'"
adb shell "su -c 'chown u0_a255:u0_a255 /data/user/0/app.comaps.fdroid/files/bookmarks/place.kml'"
adb shell "su -c 'chmod 600 /data/user/0/app.comaps.fdroid/files/bookmarks/place.kml'"
adb shell "su -c 'restorecon -v /data/user/0/app.comaps.fdroid/files/bookmarks/place.kml'"
```

Write with `cat > file` rather than `mv`: that keeps the inode, owner, SELinux context (`u:object_r:app_data_file:s0:c255,...`) and 0600 mode intact. Then start the app and confirm:

```sh
adb shell monkey -p app.comaps.fdroid -c android.intent.category.LAUNCHER 1
adb logcat -d --pid=$(adb shell pidof app.comaps.fdroid) | grep -iE 'bookmark|kml|exception'
```

A previous copy of the file is left at `/data/local/tmp/place.kml.before-fix` for rollback. To verify what the app actually loaded (rather than what is on disk), have it export a backup — Favorites → Export all Favorites — then pull `/data/user/0/app.comaps.fdroid/cache/CoMaps_backup_<yymmdd>.kmz`, unzip, and read `files/place.kml`. On 2026-11-12 this showed 764 places and 0 remaining `0,0,0` entries.

## Summary of a run

`summarize_run.py` prints per-track points, distance (haversine over the raw points), elevation gain/loss, start/end times, and reverse-geocoded start/end descriptions.

Caveats when reporting numbers: elevation comes from GPS, so on flat ground the gain figure is inflated by noise (a 10 km Central Park loop reports +455 m); treat it as relative effort, not real climb. `LineString` tracks (recorded before 2026-07) have no timestamps or elevation at all, so distance still works but there is no pace.
