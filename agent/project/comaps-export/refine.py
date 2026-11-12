#!/usr/bin/env python3
"""Second pass: repair the confidence check (Photon returns lowercase state
codes), then retry low/no-confidence entries with Nominatim and review them."""
import json
import os
import sys
import re
import time
import urllib.parse
import urllib.request

HERE = os.path.abspath(sys.argv[1]) if len(sys.argv) > 1 else os.getcwd()
CACHE = os.path.join(HERE, "geocode_cache_v2.json")
UA = "comaps-place-fixup/1.0 (personal KML repair)"
US_TAIL = re.compile(r",\s*([A-Z]{2})(?:\s+(\d{5}))?(?:,\s*USA)?\s*$")
US_STATES = {
    "AL", "AK", "AZ", "AR", "CA", "CO", "CT", "DE", "DC", "FL", "GA", "HI",
    "ID", "IL", "IN", "IA", "KS", "KY", "LA", "ME", "MD", "MA", "MI", "MN",
    "MS", "MO", "MT", "NE", "NV", "NH", "NJ", "NM", "NY", "NC", "ND", "OH",
    "OK", "OR", "PA", "RI", "SC", "SD", "TN", "TX", "UT", "VT", "VA", "WA",
    "WV", "WI", "WY",
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


def http(url, tries=3, headers=None):
    hdr = {"User-Agent": UA, "Accept": "application/json"}
    if headers:
        hdr.update(headers)
    for attempt in range(tries):
        try:
            req = urllib.request.Request(url, headers=hdr)
            with urllib.request.urlopen(req, timeout=30) as fh:
                return fh.read().decode("utf-8", "replace")
        except Exception as exc:  # noqa: BLE001
            print("  ! %s (try %d): %s" % (url.split("?")[0], attempt + 1, exc), file=sys.stderr)
            time.sleep(3 * (attempt + 1))
    return None


def nominatim(q):
    url = ("https://nominatim.openstreetmap.org/search?format=jsonv2&limit=1"
           "&addressdetails=1&q=" + urllib.parse.quote(q))
    raw = http(url)
    if not raw:
        return None
    try:
        data = json.loads(raw)
    except json.JSONDecodeError:
        return None
    if not data:
        return None
    m = data[0]
    a = m.get("address", {})
    return {
        "lat": float(m["lat"]), "lon": float(m["lon"]),
        "matched": m.get("display_name", ""),
        "state": a.get("state", ""),
        "state_code": (a.get("ISO3166-2-lvl4", "").split("-")[-1] or ""),
        "country": a.get("country", ""), "countrycode": a.get("country_code", ""),
        "zip": a.get("postcode", ""), "city": a.get("city") or a.get("town") or a.get("village") or "",
        "name": m.get("name", ""), "osm_key": m.get("category", ""), "osm_value": m.get("type", ""),
        "source": "nominatim",
    }


def us_state(q):
    m = US_TAIL.search(q)
    if m and m.group(1) in US_STATES:
        return m.group(1), m.group(2)
    return None, None


def enrich(res):
    """Census batch results only carry a matched string: pull state/zip out of it."""
    if res and res.get("source") == "census" and not res.get("state"):
        parts = [p.strip() for p in (res.get("matched") or "").split(",")]
        if len(parts) >= 3:
            res["state_code"] = parts[-2].upper()
            res["zip"] = parts[-1]
            res["countrycode"] = "US"
            res["state"] = STATE_NAMES.get(parts[-2].upper(), "")
    return res


def confidence(q, res):
    res = enrich(res)
    if not res:
        return "none", "no match"
    st, zipc = us_state(q)
    got_cc = (res.get("countrycode") or "").upper()
    if st:
        if got_cc and got_cc not in ("US", ""):
            return "low", "outside US: %s" % got_cc
        state = (res.get("state") or "").strip().lower()
        code = (res.get("state_code") or "").strip().upper()
        if not code and len(state) == 2:
            code = state.upper()
        ok = code == st or state == STATE_NAMES[st]
        if not (code or state):
            return "medium", "no state returned"
        if not ok:
            return "low", "state %r != %s" % (code or state, st)
        if zipc and res.get("zip") and res["zip"][:5] != zipc:
            return "medium", "zip %s != %s" % (res["zip"], zipc)
        return "high", "US state ok"
    if got_cc:
        return "medium", "country %s" % got_cc
    return "medium", "no country returned"


IMPROVE = [
    (r"^(Cambridge|Oxford),\s*UK$", lambda s: s.replace(", UK", ", England")),
]

# explicit queries for the ones the generic pipeline gets wrong
MANUAL = {
    "Mekong River": "Mekong",
    "Link Light Rail, Washington": "Link light rail, Seattle, Washington",
    "Oxford, UK": "Oxford, England",
    "Cambridge, UK": "Cambridge, England",
    "Kelley Engineering Center, 2500 NW Monroe Ave, Corvallis, OR 97331":
        "Kelley Engineering Center, Oregon State University, Corvallis, Oregon",
}


def better_query(q):
    for pat, fn in IMPROVE:
        if re.match(pat, q):
            return fn(q)
    return q


def main():
    cache = json.load(open(CACHE))
    # recompute confidence for everything we have
    for q, res in cache.items():
        if q.startswith("cid:") or not res:
            continue
        res["confidence"], res["note"] = confidence(q, res)

    for q, mq in MANUAL.items():
        res = nominatim(mq)
        time.sleep(1.2)
        if res:
            res["confidence"], res["note"] = confidence(q, res)
            res["queried_as"] = mq
            cache[q] = res
        print("manual %-40s -> %s" % (q[:40], ((cache.get(q) or {}).get("matched") or "FAIL")[:60]))
    json.dump(cache, open(CACHE, "w"), indent=1)

    retry = [q for q, v in cache.items()
             if not q.startswith("cid:") and (v is None or v.get("confidence") == "low")]
    print("retrying", len(retry), "entries via nominatim")
    for i, q in enumerate(retry, 1):
        nq = better_query(q)
        res = nominatim(nq)
        time.sleep(1.2)
        if res:
            conf, note = confidence(q, res)
            if res.get("confidence") is None or conf != "low" or cache.get(q) is None:
                pass
            # prefer the new result when it is not "low"
            if conf != "low" or cache.get(q) is None or nq != q:
                res["confidence"], res["note"] = conf, note
                cache[q] = res
        old = cache.get(q)
        print("[%2d/%2d] %-52s -> %-38s %s%s" % (
            i, len(retry), q[:52], ((old or {}).get("matched") or "FAIL")[:38],
            (old or {}).get("confidence", "none"),
            "" if nq == q else "  (queried as %r)" % nq))
        json.dump(cache, open(CACHE, "w"), indent=1)

    from collections import Counter
    print("\nconfidence:", Counter((v or {}).get("confidence", "none") for v in cache.values()))
    for q, v in sorted(cache.items()):
        if not v or v.get("confidence") in ("low", "medium", "none"):
            print("  %-8s %-50s %s" % ((v or {}).get("confidence", "none"), q[:50],
                                        (v or {}).get("matched", "")[:60]))


if __name__ == "__main__":
    main()
