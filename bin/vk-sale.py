#!/usr/bin/env python3
"""vk-sale - sweep Verkkokauppa.com for deep discounts in PC-related categories.

Pages the live search API with filter[has_discount]=true for the given
categories, computes the real discount % (original vs current price) per
product, keeps items at or above --min-disc, and prints a table with live
stock. Uses the same public JSON services as the web storefront - no auth.

Usage:
  python bin/vk-sale.py [--category information_technology,gaming]
                        [--min-disc 50] [--page-size 48] [--json]
"""
import json
import os
import sys
import time
import urllib.parse
import urllib.request
import uuid

SEARCH = "https://search.service.verkkokauppa.com/fi/api/v1/product-search"
AVAIL = "https://product.service.verkkokauppa.com/fi/api/v1/availability"
UA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/126.0"


def get(url, params):
    q = urllib.parse.urlencode(params)
    req = urllib.request.Request(
        f"{url}?{q}", headers={"Accept": "application/json", "User-Agent": UA}
    )
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.loads(r.read().decode())


def price_of(p):
    attrs = p.get("attributes") or {}
    arts = attrs.get("articles") or []
    if arts and (arts[0].get("price") or {}).get("current"):
        return arts[0]["price"]
    return attrs.get("price") or {}


def sweep_category(cat, sid, page_size, delay=0.25):
    prods, page = [], 1
    while True:
        d = get(SEARCH, {
            "page[number]": page,
            "page[size]": page_size,
            "filter[category]": cat,
            "filter[has_discount]": "true",
            "sessionId": sid,
            "include": "campaigns,category",
        })
        batch = d.get("data", [])
        prods.extend(batch)
        total = (d.get("meta") or {}).get("totalResults", 0)
        if not batch or page * page_size >= total:
            break
        page += 1
        time.sleep(delay)
    return prods


def fetch_availability(pids):
    out = {}
    for i in range(0, len(pids), 100):
        chunk = pids[i:i + 100]
        try:
            d = get(AVAIL, {"pids": ",".join(chunk)})
            for it in d:
                st = (it.get("status") or {})
                out[str(it.get("pid") or it.get("id") or "")] = st.get("schema") or st.get("og") or "?"
        except Exception as e:  # availability is best-effort
            sys.stderr.write(f"availability chunk failed: {e}\n")
    return out


def main():
    args = sys.argv[1:]
    cats = ["information_technology", "pc_gaming", "computer_peripherals", "headsets_for_gaming"]
    min_disc = 50.0
    page_size = 48
    as_json = False
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--category" and i + 1 < len(args):
            cats = [c.strip() for c in args[i + 1].split(",") if c.strip()]; i += 2
        elif a == "--min-disc" and i + 1 < len(args):
            min_disc = float(args[i + 1]); i += 2
        elif a == "--page-size" and i + 1 < len(args):
            page_size = int(args[i + 1]); i += 2
        elif a == "--json":
            as_json = True; i += 1
        elif a in ("-h", "--help"):
            print(__doc__)
            return 0
        else:
            sys.stderr.write(f"unknown arg: {a}\n")
            return 2

    sid = os.environ.get("VK_SESSION_ID") or str(uuid.uuid4())
    rows, seen = [], set()
    for cat in cats:
        prods = sweep_category(cat, sid, page_size)
        for p in prods:
            pid = str(p.get("id") or "")
            if not pid or pid in seen:
                continue
            seen.add(pid)
            pr = price_of(p)
            cur = pr.get("current") or 0
            orig = pr.get("original") or cur
            disc = ((orig - cur) / orig * 100) if orig > 0 else 0
            if disc < min_disc:
                continue
            rows.append({
                "pid": pid,
                "name": (p.get("attributes") or {}).get("name", ""),
                "price": round(cur, 2),
                "original": round(orig, 2),
                "discount_pct": round(disc, 1),
                "category": cat,
            })

    rows.sort(key=lambda r: -r["discount_pct"])
    if as_json:
        print(json.dumps(rows, indent=2, ensure_ascii=False))
        return 0

    if not rows:
        print(f"No products with >= {min_disc:g}% discount found in: {', '.join(cats)}")
        return 0

    pids = [r["pid"] for r in rows]
    avail = fetch_availability(pids)

    print(f"{'PID':<9}{'NAME':<58}{'PRICE':>9}  {'WAS':>9}  {'-':>5}  STOCK")
    for r in rows:
        name = r["name"][:56]
        price = f"{r['price']:,.2f}".replace(",", " ")
        was = f"{r['original']:,.2f}".replace(",", " ")
        print(f"{r['pid']:<9}{name:<58}{price:>9}  {was:>9}  {r['discount_pct']:>4.1f}%  {avail.get(r['pid'], '?'):<20}")
    print(f"\n{len(rows)} products >= {min_disc:g}% off across: {', '.join(cats)}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
