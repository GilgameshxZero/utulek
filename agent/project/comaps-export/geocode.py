#!/usr/bin/env python3
"""Geocode the 0,0,0 placemarks: US street addresses via the US Census batch
geocoder (one request), everything else via Photon with Nominatim fallback.
Writes geocode_cache_v2.json keyed by the original Google Maps 'q' string."""
import csv
import io
import json
import os
import sys
import re
import time
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET

HERE = os.path.abspath(sys.argv[1]) if len(sys.argv) > 1 else os.getcwd()
CACHE = os.path.join(HERE, "geocode_cache_v2.json")
UA = "comaps-place-fixup/1.0 (personal KML repair)"
KML = "{http://www.opengis.net/kml/2.2}"
US_TAIL = re.compile(r",\s*([A-Z]{2})(?:\s+(\d{5}))?(?:,\s*USA)?\s*$")
UNIT = re.compile(r"\s*#\s*[A-Za-z0-9-]+$")
COUNTRY_HINTS = {
    "usa": "US", "united states": "US", "uk": "GB", "united kingdom": "GB",
    "china": "CN", "japan": "JP", "spain": "ES", "israel": "IL",
    "turkey": "TR", "indonesia": "ID", "saudi arabia": "SA",
}
STATE_NAMES = {
    "AL": "alabama", "AK": "alaska", "AZ": "arizona", "AR": "arkansas",
    "CA": "california", "CO": "colorado", "CT": "connecticut", "DE": "delaware",
    "DC": "district of columbia", "FL": "florida", "GA": "georgia",
    "HI": "hawaii", "ID": "idaho", "IL": "illinois", "IN": "indiana",
    "IA": "iowa", "KS": "kansas", "KY": "kentucky", "LA": "louisiana",
    "ME": "maine", "MD": "maryland", "MA": "massachusetts", "MI": "michigan",
    "MN": "minnesota", "MS": "mississippi", "MO": "missouri", "MT": "montana",
    "NE": "nebraska", "NV": "nevada", "NH": "new hampshire", "NJ": "new jersey",
    "NM": "new mexico", "NY": "new york", "NC": "north carolina",
    "ND": "north dakota", "OH": "ohio", "OK": "oklahoma", "OR": "oregon",
    "PA": "pennsylvania", "RI": "rhode island", "SC": "south carolina",
    "SD": "south dakota", "TN": "tennessee", "TX": "texas", "UT": "utah",
    "VT": "vermont", "VA": "virginia", "WA": "washington",
    "WV": "west virginia", "WI": "wisconsin", "WY": "wyoming",
}


def http(url, data=None, headers=None, tries=3):
    hdr = {"User-Agent": UA}
    if headers:
        hdr.update(headers)
    for attempt in range(tries):
        try:
            req = urllib.request.Request(url, data=data, headers=hdr)
            with urllib.request.urlopen(req, timeout=30) as fh:
                return fh.read().decode("utf-8", "replace")
        except Exception as exc:  # noqa: BLE001
            print("  ! %s (try %d): %s" % (url.split("?")[0], attempt + 1, exc), file=sys.stderr)
            time.sleep(3 * (attempt + 1))
    return None


def split_us(q):
    """-> (street, city, state, zip) or None if it doesn't look like a US street address."""
    m = US_TAIL.search(q)
    if not m:
        return None
    state, zipc = m.group(1), m.group(2) or ""
    body = q[:m.start()].strip().rstrip(",")
    body = re.sub(r",\s*USA\s*$", "", body, flags=re.I)
    parts = [p.strip() for p in body.split(",") if p.strip()]
    if not parts:
        return None
    city = parts[-1]
    street = ", ".join(parts[:-1])
    street = UNIT.sub("", street).strip()
    if not re.match(r"^\d", street):
        return None  # no house number -> not a census street address
    return street, city, state, zipc


def census_batch(rows):
    """rows: list of (key, street, city, state, zip). Returns dict key -> result."""
    if not rows:
        return {}
    buf = io.StringIO()
    w = csv.writer(buf, lineterminator="\n")
    for i, (_, st, city, state, zipc) in enumerate(rows, 1):
        w.writerow([i, st, city, state, zipc])
    body = buf.getvalue()
    boundary = "----comapsfixup"
    payload = (
        "--%s\r\nContent-Disposition: form-data; name=\"addressFile\"; filename=\"a.csv\"\r\n"
        "Content-Type: text/csv\r\n\r\n%s\r\n"
        "--%s\r\nContent-Disposition: form-data; name=\"benchmark\"\r\n\r\nPublic_AR_Current\r\n"
        "--%s\r\nContent-Disposition: form-data; name=\"format\"\r\n\r\njson\r\n"
        "--%s--\r\n" % (boundary, body, boundary, boundary, boundary)
    ).encode()
    raw = http("https://geocoding.geo.census.gov/geocoder/locations/addressbatch",
               data=payload,
               headers={"Content-Type": "multipart/form-data; boundary=%s" % boundary})
    out = {}
    if not raw:
        return out
    for rec in csv.reader(io.StringIO(raw)):
        if len(rec) < 6:
            continue
        idx, _addr, status, _exact, matched, coord = rec[0], rec[1], rec[2], rec[3], rec[4], rec[5]
        if status != "Match" or not coord:
            continue
        lon, lat = [float(v) for v in coord.split(",")]
        out[rows[int(idx) - 1][0]] = {
            "lat": lat, "lon": lon, "matched": matched, "source": "census",
        }
    return out


def photon(q):
    # strip the "&ftid=..." leftovers and any URL encoding
    query = q.replace("&", " ").strip()
    url = "https://photon.komoot.io/api/?limit=1&q=" + urllib.parse.quote(query)
    raw = http(url, headers={"Accept": "application/json"})
    if not raw:
        return None
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return None
    feats = data.get("features") or []
    if not feats:
        return None
    p = feats[0]
    pr = p.get("properties", {})
    lon, lat = p["geometry"]["coordinates"]
    bits = [pr.get("name"), pr.get("street"), pr.get("city"), pr.get("state"),
            pr.get("postcode"), pr.get("country")]
    return {
        "lat": float(lat), "lon": float(lon),
        "matched": ", ".join(b for b in bits if b),
        "state": pr.get("state", ""), "country": pr.get("country", ""),
        "countrycode": pr.get("countrycode", ""), "zip": pr.get("postcode", ""),
        "city": pr.get("city", ""), "name": pr.get("name", ""),
        "osm_key": pr.get("osm_key", ""), "osm_value": pr.get("osm_value", ""),
        "source": "photon",
    }


def confidence(q, res):
    if not res:
        return "none", "no match"
    want = None
    low = q.lower()
    for token, code in COUNTRY_HINTS.items():
        if re.search(r"\b%s\b" % re.escape(token), low):
            want = code
    if want and res.get("countrycode") and res["countrycode"].upper() != want:
        return "low", "country %s != %s" % (res["countrycode"], want)
    m = US_TAIL.search(q)
    if m:
        st = (res.get("state") or "").lower()
        got = (res.get("countrycode") or "").upper()
        if got and got != "US":
            return "low", "not in US: %s" % got
        if st and st not in (STATE_NAMES.get(m.group(1), ""),):
            return "low", "state %s != %s" % (st, m.group(1))
        if m.group(2) and res.get("zip") and res["zip"][:5] != m.group(2):
            return "medium", "zip %s != %s" % (res["zip"], m.group(2))
        return "high", "US ok"
    if res.get("source") == "census":
        return "high", "census exact"
    return "medium", "non-US unverified"


def main():
    tree = ET.parse(os.path.join(HERE, "place.kml"))
    zero = []
    for pm in tree.getroot().iter(KML + "Placemark"):
        c = (pm.findtext(KML + "Point/" + KML + "coordinates") or "").strip()
        if not c.startswith("0,0,0"):
            continue
        desc = (pm.findtext(KML + "description") or "").strip()
        qs = urllib.parse.parse_qs(urllib.parse.urlparse(desc).query)
        q = urllib.parse.unquote_plus(qs.get("q", [""])[0]).strip()
        cid = qs.get("cid", [""])[0]
        zero.append({"q": q, "cid": cid})
    uniq = sorted({z["q"] for z in zero if z["q"]})
    print("zero placemarks:", len(zero), " unique queries:", len(uniq))

    cache = json.load(open(CACHE)) if os.path.exists(CACHE) else {}

    us_rows, us_map = [], {}
    for q in uniq:
        parts = split_us(q)
        if parts:
            us_rows.append((q, *parts))
            us_map[q] = parts
    print("census batch rows:", len(us_rows))
    got = census_batch(us_rows)
    for q, res in got.items():
        conf, note = confidence(q, res)
        res["confidence"], res["note"] = conf, note
        cache[q] = res
    print("census matched:", len(got), "/", len(us_rows))

    rest = [q for q in uniq if q not in cache]
    print("falling back to photon for", len(rest))
    for i, q in enumerate(rest, 1):
        res = photon(q)
        time.sleep(0.6)
        conf, note = confidence(q, res)
        if res:
            res["confidence"], res["note"] = conf, note
        cache[q] = res
        print("[%2d/%2d] %-58s -> %s %s" % (i, len(rest), q[:58],
                                            (res or {}).get("matched", "FAIL")[:34], conf))
        json.dump(cache, open(CACHE, "w"), indent=1)

    # the lone cid-only bookmark
    cid = next((z["cid"] for z in zero if z["cid"] and not z["q"]), None)
    if cid and ("cid:" + cid) not in cache:
        raw = http("https://maps.google.com/?cid=" + cid,
                   headers={"User-Agent": "Mozilla/5.0 (X11; Linux x86_64)"})
        res = None
        if raw:
            m = re.search(r"center=(-?[0-9.]+)%2C(-?[0-9.]+)", raw)
            if m:
                res = {"lat": float(m.group(1)), "lon": float(m.group(2)),
                       "matched": "google cid " + cid, "source": "google-cid"}
        if res:
            rev = photon("%f,%f" % (res["lat"], res["lon"]))
            if rev:
                res["matched"] = (rev.get("name") or rev.get("city") or "") + " (" + rev["matched"] + ")"
                res["country"] = rev.get("country", "")
                res["city"] = rev.get("city", "")
                res["name"] = rev.get("name", "")
                res["countrycode"] = rev.get("countrycode", "")
            res["confidence"] = "medium"
            res["note"] = "coordinates from Google CID; name reverse-geocoded"
        cache["cid:" + cid] = res
        print("cid entry ->", (res or {}).get("matched"))

    json.dump(cache, open(CACHE, "w"), indent=1)
    from collections import Counter
    print("\nresolution:", Counter((v or {}).get("source", "FAIL") for v in cache.values()))
    print("confidence:", Counter((v or {}).get("confidence", "none") for v in cache.values()))
    for q, v in cache.items():
        if not v or v.get("confidence") == "low":
            print("  review: %-55s %s" % (q[:55], (v or {}).get("note", "no match")))


if __name__ == "__main__":
    main()
