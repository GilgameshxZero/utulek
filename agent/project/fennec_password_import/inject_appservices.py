#!/usr/bin/env python3
"""Import a browser CSV export into Fennec/Firefox-for-Android's password store.

The store used by Fenix's Passwords screen and autofill is the app-services
SQLite database (`databases/logins2.sqlite`), not Gecko's `logins.json`.
Rows are encrypted the same way as mozilla/application-services db-crypto:

    secFields = JWE compact, alg=dir, enc=A256GCM
    plaintext = {"u": "<username>", "p": "<password>"}
    key       = "loginsKey" JWK from core_prefs_kp_pre_m.xml

On release-channel builds `forceInsecure = true`, so that JWK is stored in
plaintext. On Nightly/Debug it is Keystore-encrypted and this script cannot
be used without running code as the app uid.

Usage:
    python3 inject_appservices.py --csv export.csv \
        --prefs core_prefs_kp_pre_m.xml --canary loginsCrypto.xml \
        --db logins2.sqlite [--overwrite] [--dry-run]

Default mode merges: existing decryptable rows are kept and duplicate
(origin, username, password) triples are skipped. --overwrite clears
loginsL, loginsM and breachesL first, leaving only the CSV entries.
"""
import argparse
import base64
import csv
import json
import os
import secrets
import sqlite3
import sys
import time
import xml.etree.ElementTree as ET
from ctypes import (CDLL, POINTER, Structure, byref, c_char_p, c_int,
                    c_ubyte, c_uint, c_ulong, c_void_p, cast, sizeof)
from urllib.parse import urlsplit

NSS = CDLL("libnss3.so")

CKM_AES_GCM = 0x1087
PK11_ORIGIN_UNWRAP = 3
CKA_ENCRYPT_DECRYPT = 0x105
TAG_LEN = 16
IV_LEN = 12
CANARY_PLAINTEXT = b"a string for checking validity of the key"
HEADER = base64.urlsafe_b64encode(
    b'{"alg":"dir","enc":"A256GCM"}').rstrip(b"=").decode()
AAD = HEADER.encode()


class SECItem(Structure):
    _fields_ = [("type", c_int), ("data", POINTER(c_ubyte)), ("len", c_uint)]


class GCM(Structure):
    _fields_ = [("pIv", POINTER(c_ubyte)), ("ulIvLen", c_ulong),
                ("ulIvBits", c_ulong), ("pAAD", POINTER(c_ubyte)),
                ("ulAADLen", c_ulong), ("ulTagBits", c_ulong)]


NSS.NSS_NoDB_Init.argtypes = [c_char_p]
NSS.NSS_NoDB_Init.restype = c_int
NSS.NSS_Shutdown.argtypes = []
NSS.NSS_Shutdown.restype = c_int
NSS.PK11_GetInternalSlot.restype = c_void_p
NSS.PK11_ImportSymKey.argtypes = [c_void_p, c_ulong, c_int, c_ulong,
                                  POINTER(SECItem), c_void_p]
NSS.PK11_ImportSymKey.restype = c_void_p
NSS.PK11_Encrypt.argtypes = [c_void_p, c_ulong, POINTER(SECItem),
                             POINTER(c_ubyte), POINTER(c_uint), c_uint,
                             POINTER(c_ubyte), c_uint]
NSS.PK11_Encrypt.restype = c_int
NSS.PK11_Decrypt.argtypes = [c_void_p, c_ulong, POINTER(SECItem),
                             POINTER(c_ubyte), POINTER(c_uint), c_uint,
                             POINTER(c_ubyte), c_uint]
NSS.PK11_Decrypt.restype = c_int


def b64u(data):
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def b64u_decode(text):
    return base64.urlsafe_b64decode(text + "=" * (-len(text) % 4))


def read_jwk(prefs_path, name):
    for node in ET.parse(prefs_path).getroot():
        if node.get("name") == name:
            jwk = json.loads(node.text)
            if jwk.get("kty") != "oct":
                raise SystemExit(f"{name} is not an octet JWK")
            key = b64u_decode(jwk["k"])
            if len(key) != 32:
                raise SystemExit(f"{name} is {len(key)} bytes, expected 32")
            return key
    raise SystemExit(f"{name} not found in {prefs_path}")


class AesGcm:
    def __init__(self, key):
        if NSS.NSS_NoDB_Init(None) != 0:
            raise RuntimeError("NSS_NoDB_Init failed")
        slot = NSS.PK11_GetInternalSlot()
        key_buf = (c_ubyte * len(key)).from_buffer_copy(key)
        item = SECItem(0, key_buf, len(key))
        self.key = NSS.PK11_ImportSymKey(
            slot, CKM_AES_GCM, PK11_ORIGIN_UNWRAP, CKA_ENCRYPT_DECRYPT,
            byref(item), None)
        if not self.key:
            raise RuntimeError("PK11_ImportSymKey failed")

    def _params(self, iv):
        iv_buf = (c_ubyte * len(iv)).from_buffer_copy(iv)
        aad_buf = (c_ubyte * len(AAD)).from_buffer_copy(AAD)
        params = GCM(cast(iv_buf, POINTER(c_ubyte)), len(iv), len(iv) * 8,
                     cast(aad_buf, POINTER(c_ubyte)), len(AAD), 128)
        params_buf = (c_ubyte * sizeof(params)).from_buffer_copy(params)
        self._keep = (iv_buf, aad_buf, params_buf)
        return SECItem(0, params_buf, sizeof(params))

    def encrypt(self, iv, plaintext):
        item = self._params(iv)
        in_buf = (c_ubyte * len(plaintext)).from_buffer_copy(plaintext)
        out = (c_ubyte * (len(plaintext) + TAG_LEN))()
        out_len = c_uint(len(out))
        if NSS.PK11_Encrypt(self.key, CKM_AES_GCM, byref(item), out,
                            byref(out_len), len(out), in_buf,
                            len(plaintext)) != 0:
            raise RuntimeError("PK11_Encrypt failed")
        blob = bytes(out[:out_len.value])
        return blob[:-TAG_LEN], blob[-TAG_LEN:]

    def decrypt(self, iv, ciphertext, tag):
        item = self._params(iv)
        blob = ciphertext + tag
        in_buf = (c_ubyte * len(blob)).from_buffer_copy(blob)
        out = (c_ubyte * len(ciphertext))()
        out_len = c_uint(len(out))
        if NSS.PK11_Decrypt(self.key, CKM_AES_GCM, byref(item), out,
                            byref(out_len), len(out), in_buf,
                            len(blob)) != 0:
            raise RuntimeError("PK11_Decrypt failed")
        return bytes(out[:out_len.value])


def make_jwe(gcm, username, password):
    payload = json.dumps({"u": username, "p": password},
                         separators=(",", ":")).encode()
    iv = os.urandom(IV_LEN)
    ct, tag = gcm.encrypt(iv, payload)
    return ".".join([HEADER, "", b64u(iv), b64u(ct), b64u(tag)])


def open_jwe(gcm, value):
    header, enc_key, iv, ct, tag = value.split(".")
    if header != HEADER or enc_key:
        raise ValueError("unexpected JWE header")
    payload = gcm.decrypt(b64u_decode(iv), b64u_decode(ct), b64u_decode(tag))
    return json.loads(payload)


def check_canary(gcm, canary_path):
    for node in ET.parse(canary_path).getroot():
        if node.get("name") == "canaryPhrase":
            parts = node.text.split(".")
            plain = gcm.decrypt(b64u_decode(parts[2]), b64u_decode(parts[3]),
                                b64u_decode(parts[4]))
            if plain != CANARY_PLAINTEXT:
                raise SystemExit("canary does not match the key")
            return
    raise SystemExit("canaryPhrase not found")


def normalize(raw_url):
    if "://" not in raw_url:
        raw_url = "https://" + raw_url
    parts = urlsplit(raw_url)
    if parts.scheme not in ("http", "https") or not parts.hostname:
        return None
    return f"{parts.scheme}://{parts.netloc}"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--csv", required=True)
    ap.add_argument("--prefs", required=True,
                    help="core_prefs_kp_pre_m.xml with plaintext loginsKey")
    ap.add_argument("--canary", help="loginsCrypto.xml (strong key check)")
    ap.add_argument("--db", required=True, help="copy of logins2.sqlite")
    ap.add_argument("--overwrite", action="store_true")
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    gcm = AesGcm(read_jwk(args.prefs, "loginsKey"))
    if args.canary:
        check_canary(gcm, args.canary)

    conn = sqlite3.connect(args.db)
    version = conn.execute("PRAGMA user_version").fetchone()[0]
    if version != 5:
        raise SystemExit(f"unexpected schema version {version}")

    existing = set()

    now = int(time.time() * 1000)
    rows, guids, seen = [], set(), set()
    stats = dict(rows=0, inserted=0, skip_password=0, skip_scheme=0,
                 skip_host=0, dupes=0)

    if not args.overwrite:
        for origin, sec_fields in conn.execute(
                "SELECT origin, secFields FROM loginsL UNION ALL "
                "SELECT origin, secFields FROM loginsM"):
            if not sec_fields:
                continue
            data = open_jwe(gcm, sec_fields)
            existing.add((origin, data.get("u", ""), data.get("p", "")))

    with open(args.csv, newline="", encoding="utf-8", errors="replace") as fh:
        reader = csv.DictReader(fh)
        cols = {f.strip().lower(): f for f in (reader.fieldnames or [])}
        for row in reader:
            stats["rows"] += 1
            username = (row.get(cols.get("username", "")) or "").strip()
            password = row.get(cols.get("password", "")) or ""
            raw = (row.get(cols.get("url", "")) or "").strip()
            if not password:
                stats["skip_password"] += 1
                continue
            if not raw:
                stats["skip_scheme"] += 1
                continue
            origin = normalize(raw)
            if not origin:
                stats["skip_scheme"] += 1
                continue
            key3 = (origin, username, password)
            if key3 in seen or key3 in existing:
                stats["dupes"] += 1
                continue
            seen.add(key3)
            sec_fields = make_jwe(gcm, username, password)
            if open_jwe(gcm, sec_fields) != {"u": username, "p": password}:
                raise RuntimeError("encryption self-test failed")
            guid = secrets.token_urlsafe(9)
            while guid in guids:
                guid = secrets.token_urlsafe(9)
            guids.add(guid)
            rows.append((origin, origin, now, now, now, sec_fields, guid, now))

    stats["new"] = len(rows)
    if args.dry_run:
        print(json.dumps(stats, indent=2))
        return

    if args.overwrite:
        stats["cleared_L"] = conn.execute(
            "SELECT COUNT(*) FROM loginsL").fetchone()[0]
        stats["cleared_M"] = conn.execute(
            "SELECT COUNT(*) FROM loginsM").fetchone()[0]
        conn.execute("DELETE FROM loginsL")
        conn.execute("DELETE FROM loginsM")
        conn.execute("DELETE FROM breachesL")

    conn.executemany(
        """INSERT INTO loginsL
           (origin, httpRealm, formActionOrigin, usernameField, passwordField,
            timesUsed, timeCreated, timeLastUsed, timePasswordChanged,
            timeLastBreachAlertDismissed, secFields, guid, local_modified,
            is_deleted, sync_status)
           VALUES (?, NULL, ?, NULL, NULL, 0, ?, ?, ?, NULL, ?, ?, ?, 0, 2)""",
        rows)
    conn.commit()
    stats["inserted"] = len(rows)
    stats["L_after"] = conn.execute(
        "SELECT COUNT(*) FROM loginsL").fetchone()[0]
    stats["M_after"] = conn.execute(
        "SELECT COUNT(*) FROM loginsM").fetchone()[0]
    stats["integrity"] = conn.execute("PRAGMA integrity_check").fetchone()[0]
    conn.execute("PRAGMA wal_checkpoint(TRUNCATE)")
    conn.close()
    NSS.NSS_Shutdown()
    print(json.dumps(stats, indent=2))


if __name__ == "__main__":
    main()
