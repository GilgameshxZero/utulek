#!/usr/bin/env python3
"""Convert CoMaps bookmark/track KML to the exact GPX style CoMaps itself
exports (creator="CoMaps", garmin/gpx_style namespaces, DisplayColor per track)."""
import os
import re
import sys
import xml.etree.ElementTree as ET

KML = "{http://www.opengis.net/kml/2.2}"
GX = "{http://www.google.com/kml/ext/2.2}"

HEADER = '''<?xml version="1.0"?>
<gpx version="1.1" creator="CoMaps" xmlns="http://www.topografix.com/GPX/1/1"
    xmlns:gpxx="http://www.garmin.com/xmlschemas/GpxExtensions/v3"
    xmlns:gpx_style="http://www.topografix.com/GPX/gpx_style/0/2"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://www.topografix.com/GPX/1/1 https://www.topografix.com/GPX/1/1/gpx.xsd http://www.topografix.com/GPX/gpx_style/0/2 https://www.topografix.com/GPX/gpx_style/0/2/gpx_style.xsd http://www.garmin.com/xmlschemas/GpxExtensions/v3 https://www.garmin.com/xmlschemas/GpxExtensionsv3.xsd">
<metadata>
  <name>%s</name>
</metadata>
'''

# CoMaps line colour (AABBGGRR) -> Garmin DisplayColor name
COLORS = {
    "FFBF3966": "DarkMagenta", "FF8241FF": "DarkMagenta", "FFB2249B": "DarkMagenta",
    "FF3C8C3C": "DarkGray", "FF807359": "DarkGray", "FF334680": "DarkGray",
    "FF737373": "DarkGray",
    "FF00C8FF": "Yellow", "FF0096FF": "DarkYellow", "FF3264F0": "DarkYellow",
    "FFCC6600": "DarkCyan", "FF8CA500": "DarkCyan",
    "FF231BE5": "Red",
    "FFF29C24": "Cyan",
}


def num(v):
    """CoMaps prints coordinates with up to 6 decimals, trailing zeros trimmed."""
    s = "%.6f" % float(v)
    s = s.rstrip("0").rstrip(".")
    return "0" if s in ("-0", "") else s


def cdata(text):
    return "<![CDATA[%s]]>" % text if "&" in text else text


def waypoints(kml_path):
    root = ET.parse(kml_path).getroot()
    out = []
    for pm in root.iter(KML + "Placemark"):
        pt = pm.find(KML + "Point")
        if pt is None:
            continue
        raw = (pt.findtext(KML + "coordinates") or "").strip()
        if not raw:
            continue
        parts = raw.split(",")
        lon, lat = parts[0], parts[1]
        name = (pm.findtext(KML + "name") or "").strip()
        desc = (pm.findtext(KML + "description") or "").strip()
        out.append('<wpt lat="%s" lon="%s">\n  <name>%s</name>\n' % (num(lat), num(lon), name))
        if desc:
            out.append("  <desc>%s</desc>\n" % cdata(desc))
        out.append("</wpt>\n")
    return out


def tracks(kml_path):
    root = ET.parse(kml_path).getroot()
    out = []
    for pm in root.iter(KML + "Placemark"):
        name = (pm.findtext(KML + "name") or "").strip()
        color = (pm.findtext(KML + "Style/" + KML + "LineStyle/" + KML + "color") or "").strip()
        pts = []
        tr = pm.find(GX + "Track")
        if tr is not None:
            whens = [(w.text or "").strip() for w in tr.findall(KML + "when")]
            for i, c in enumerate(tr.findall(GX + "coord")):
                parts = c.text.split()
                pts.append((parts[1], parts[0], parts[2] if len(parts) > 2 else None,
                            whens[i] if i < len(whens) else None))
        else:
            ls = pm.find(KML + "LineString")
            if ls is None:
                continue
            for chunk in (ls.findtext(KML + "coordinates") or "").split():
                parts = chunk.split(",")
                pts.append((parts[1], parts[0], parts[2] if len(parts) > 2 else None, None))
        if not pts:
            continue
        out.append("<trk>\n  <name>%s</name>\n" % name)
        garmin = COLORS.get(color)
        if garmin and len(color) == 8:
            rgb = color[6:8] + color[4:6] + color[2:4]  # AABBGGRR -> RRGGBB
            out.append("  <extensions>\n"
                       "    <gpxx:TrackExtension><gpxx:DisplayColor>%s</gpxx:DisplayColor></gpxx:TrackExtension>\n"
                       "    <gpx_style:line><gpx_style:color>%s</gpx_style:color></gpx_style:line>\n"
                       "    <xsi:gpx><color>#%s%s</color></xsi:gpx>\n"
                       "  </extensions>\n" % (garmin, rgb, color[0:2], rgb))
        out.append("  <trkseg>\n")
        for lat, lon, ele, t in pts:
            out.append('    <trkpt lat="%s" lon="%s">\n' % (num(lat), num(lon)))
            if ele not in (None, ""):
                out.append("      <ele>%s</ele>\n" % num(ele))
            if t:
                out.append("      <time>%s</time>\n" % t)
            out.append("    </trkpt>\n")
        out.append("  </trkseg>\n</trk>\n")
    return out


def convert(kml_path, gpx_path, name=None):
    base = name or os.path.splitext(os.path.basename(kml_path))[0]
    blocks = tracks(kml_path) if "track" in base else waypoints(kml_path)
    with open(gpx_path, "w", encoding="utf-8") as fh:
        fh.write(HEADER % base)
        fh.write("".join(blocks))
        fh.write("</gpx>")
    return sum(1 for b in blocks if b.startswith("<trk>") or b.startswith("<wpt"))


if __name__ == "__main__":
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    name = None
    for a in sys.argv[1:]:
        if a.startswith("--name="):
            name = a.split("=", 1)[1]
    print(convert(args[0], args[1], name=name), "records ->", args[1])
