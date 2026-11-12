#!/usr/bin/env python3
"""Summarise CoMaps tracks from a GPX file, newest last.
Usage: summarize_run.py [track.gpx] [--count N] [--reverse]
"""
import json
import math
import sys
import urllib.request
import xml.etree.ElementTree as ET

G = "{http://www.topografix.com/GPX/1/1}"


def haversine(a, b):
    r = 6371000.0
    la1, lo1, la2, lo2 = map(math.radians, (a[0], a[1], b[0], b[1]))
    h = (math.sin((la2 - la1) / 2) ** 2
         + math.cos(la1) * math.cos(la2) * math.sin((lo2 - lo1) / 2) ** 2)
    return 2 * r * math.asin(math.sqrt(h))


def reverse(lat, lon):
    url = ("https://nominatim.openstreetmap.org/reverse?format=jsonv2&lat=%f&lon=%f&zoom=17"
           % (lat, lon))
    req = urllib.request.Request(url, headers={"User-Agent": "comaps-run-summary/1.0"})
    try:
        with urllib.request.urlopen(req, timeout=30) as fh:
            return json.loads(fh.read().decode())["display_name"]
    except Exception:  # noqa: BLE001
        return "%f, %f" % (lat, lon)


def main():
    path = next((a for a in sys.argv[1:] if not a.startswith("--")), "track.gpx")
    count = 1
    for a in sys.argv[1:]:
        if a.startswith("--count="):
            count = int(a.split("=", 1)[1])
    root = ET.parse(path).getroot()
    runs = []
    for trk in root.findall(G + "trk"):
        pts = trk.findall(G + "trkseg/" + G + "trkpt")
        if not pts:
            continue
        coords = [(float(p.get("lat")), float(p.get("lon")),
                   float(p.findtext(G + "ele") or 0)) for p in pts]
        times = [p.findtext(G + "time") for p in pts]
        dist = sum(haversine(coords[i], coords[i + 1]) for i in range(len(coords) - 1))
        gain = sum(max(0.0, coords[i + 1][2] - coords[i][2]) for i in range(len(coords) - 1))
        loss = sum(max(0.0, coords[i][2] - coords[i + 1][2]) for i in range(len(coords) - 1))
        runs.append({
            "name": trk.findtext(G + "name"), "points": len(pts), "distance_m": dist,
            "gain_m": gain, "loss_m": loss, "start_time": times[0], "end_time": times[-1],
            "start": coords[0][:2], "end": coords[-1][:2],
        })
    if not runs:
        print("no tracks in", path)
        return
    for r in runs[-count:]:
        print("=" * 62)
        print("track      :", r["name"])
        print("points     :", r["points"])
        print("distance   : %.2f km" % (r["distance_m"] / 1000))
        print("elevation  : +%.0f / -%.0f m" % (r["gain_m"], r["loss_m"]))
        print("start/end  : %s -> %s" % (r["start_time"], r["end_time"]))
        print("start pt   : %.6f, %.6f  %s" % (*r["start"], reverse(*r["start"])))
        print("end pt     : %.6f, %.6f  %s" % (*r["end"], reverse(*r["end"])))
    print("=" * 62)
    print("total tracks in file:", len(runs))


if __name__ == "__main__":
    main()
