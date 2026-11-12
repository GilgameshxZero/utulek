#!/bin/sh
# Pull CoMaps bookmarks/tracks from the phone, rebuild the GPX files used by
# monochrome/module/map, and print a summary of the newest recorded track.
#
# Usage: ./refresh.sh [device[:port]] [--no-pull]
#   COMAPS_WORK   scratch dir for the pulled KMLs (default: <agent>/.obj/comaps_export)
#   COMAPS_MAP    output dir (default: $HOME/main.syncthing/monochrome/module/map)
set -e

HERE=$(cd "$(dirname "$0")" && pwd)
AGENT=$(cd "$HERE/../.." && pwd)
DEV=10.8.58.57:5555
PULL=1
for arg in "$@"; do
    case "$arg" in
        --no-pull) PULL=0 ;;
        *) DEV="$arg" ;;
    esac
done
DATA="${COMAPS_WORK:-$AGENT/.obj/comaps_export}"
MAP="${COMAPS_MAP:-$HOME/main.syncthing/monochrome/module/map}"
APP=app.comaps.fdroid
BOOKMARKS=/data/user/0/$APP/files/bookmarks

mkdir -p "$DATA" "$MAP"

if [ "$PULL" = 1 ]; then
    echo "== pulling $DEV"
    adb connect "$DEV" >/dev/null
    adb devices | grep -q "$DEV[[:space:]]*device" || { echo "device not connected"; exit 1; }
    # stop the app first: it rewrites these files on exit
    adb shell "su -c 'am force-stop $APP'"
    adb shell "su -c 'cp $BOOKMARKS/place.kml /data/local/tmp/p.kml; cp $BOOKMARKS/track.kml /data/local/tmp/t.kml; chmod 644 /data/local/tmp/p.kml /data/local/tmp/t.kml'"
    adb pull /data/local/tmp/p.kml "$DATA/place.kml" >/dev/null
    adb pull /data/local/tmp/t.kml "$DATA/track.kml" >/dev/null
    adb shell "su -c 'rm -f /data/local/tmp/p.kml /data/local/tmp/t.kml'"
fi

PLACE_SRC="$DATA/place.kml"
if grep -q '<coordinates>0,0,0</coordinates>' "$PLACE_SRC"; then
    echo "== repairing unlocated bookmarks (needs network)"
    python3 "$HERE/geocode.py" "$DATA"
    python3 "$HERE/refine.py" "$DATA"
    python3 "$HERE/patch_place.py" "$DATA"
    PLACE_SRC="$DATA/place_fixed.kml"
fi

echo "== converting"
python3 "$HERE/kml2gpx.py" "$PLACE_SRC" "$MAP/place.gpx" --name=place
python3 "$HERE/kml2gpx.py" "$DATA/track.kml" "$MAP/track.gpx" --name=track
grep -c '<wpt' "$MAP/place.gpx" | sed 's/^/   waypoints: /'
grep -c '<trkpt' "$MAP/track.gpx" | sed 's/^/   track points: /'
grep -c '<trk>' "$MAP/track.gpx" | sed 's/^/   tracks: /'

echo "== summary (newest track)"
python3 "$HERE/summarize_run.py" "$MAP/track.gpx"
