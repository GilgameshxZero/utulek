#!/usr/bin/env python3
"""Rewrite place.kml -> place_fixed.kml, filling in coordinates and names for
the placemarks that were stored as 0,0,0. Everything else is left byte-identical."""
import json
import os
import sys
import re
import urllib.parse

HERE = os.path.abspath(sys.argv[1]) if len(sys.argv) > 1 else os.getcwd()
SRC = os.path.join(HERE, "place.kml")
DST = os.path.join(HERE, "place_fixed.kml")
CACHE = os.path.join(HERE, "geocode_cache_v2.json")
ZERO_COORD = "<coordinates>0,0,0</coordinates>"
ZERO_NAME = "0,0,0"


def esc(t):
    return (t.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;"))


def key_for(block):
    m = re.search(r"<description>(.*?)</description>", block, re.S)
    if not m:
        return None, None
    qs = urllib.parse.parse_qs(urllib.parse.urlparse(m.group(1).strip()).query)
    q = urllib.parse.unquote_plus(qs.get("q", [""])[0]).strip()
    cid = qs.get("cid", [""])[0]
    return (q or None), ("cid:" + cid if cid else None)


def main():
    cache = json.load(open(CACHE))
    raw = open(SRC, encoding="utf-8").read()
    parts = re.split(r"(?=<Placemark>)", raw)
    fixed = skipped = 0
    report = []
    rows = []
    out = []
    for block in parts:
        if not block.startswith("<Placemark>") or ZERO_COORD not in block:
            out.append(block)
            continue
        q, cidkey = key_for(block)
        res = cache.get(q) if q else cache.get(cidkey)
        if not res:
            out.append(block)
            skipped += 1
            report.append(("UNRESOLVED", q or cidkey, "", ""))
            continue
        lat, lon = res["lat"], res["lon"]
        name = q
        if not name:
            matched = [x.strip() for x in (res.get("matched") or "").split(",") if x.strip()]
            city = res.get("city") or ""
            if len(matched) >= 2:
                name = "%s %s" % (matched[1], matched[0])
                if city:
                    name += ", " + city
            else:
                name = "%.6f,%.6f" % (lat, lon)
        new = block.replace(ZERO_COORD, "<coordinates>%.6f,%.6f,0</coordinates>" % (lon, lat))
        new = new.replace("<name>%s</name>" % ZERO_NAME, "<name>%s</name>" % esc(name))
        new = new.replace('code="default">%s</mwm:lang>' % ZERO_NAME,
                          'code="default">%s</mwm:lang>' % esc(name))
        if new.count("<coordinates>%.6f,%.6f,0</coordinates>" % (lon, lat)) != 1:
            print("! coordinate substitution problem for", name, file=sys.stderr)
        out.append(new)
        fixed += 1
        report.append((res.get("confidence", "?"), name, res.get("matched", ""), res.get("source", "")))
        rows.append((res.get("confidence", "?"), name, res.get("lat"), res.get("lon"), res.get("matched", ""), res.get("source", ""), res.get("queried_as", "")))

    open(DST, "w", encoding="utf-8").write("".join(out))
    print("fixed:", fixed, " skipped (no result):", skipped)
    with open(os.path.join(HERE, "geocode_report.tsv"), "w", encoding="utf-8") as fh:
        fh.write("confidence\tassigned_name\tlat\tlon\tgeocoder_match\t\n")
        for conf, name, lat, lon, matched, src, qa in rows:
            fh.write("%s\t%s\t%s\t%s\t%s [%s]%s\n" % (
                conf, name, lat, lon, matched, src,
                (" queried_as=" + qa) if qa else ""))
    from collections import Counter
    print("confidence:", Counter(c for c, *_ in report).most_common())


if __name__ == "__main__":
    main()
