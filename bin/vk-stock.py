#!/usr/bin/env python3
"""vk-stock - one-shot Verkkokauppa.com search + price + availability view.

Runs the generated verkkokauppa-pp-cli twice under the hood:
  1. fi search-products (query, filters)   -> pids + names + prices
  2. fi get-availability (pids)            -> stock status per product

Usage:
  vk-stock "rtx 5090" [--size N] [--sort KEY] [--category KEY] [--json]
"""
import json
import os
import subprocess
import sys
import uuid

BASE = os.path.dirname(os.path.abspath(__file__))
BIN = os.environ.get(
    "VK_BIN", os.path.join(os.path.dirname(BASE), "verkkokauppa-pp-cli.exe")
)
if not os.path.exists(BIN):
    BIN = os.path.join(os.path.dirname(BASE), "verkkokauppa-pp-cli")


def run(args):
    p = subprocess.run([BIN] + args, capture_output=True, text=True)
    if p.returncode != 0:
        sys.stderr.write(p.stderr)
        sys.exit(p.returncode)
    return p.stdout


def price_of(prod):
    attrs = prod.get("attributes") or {}
    arts = attrs.get("articles") or []
    if arts:
        return arts[0].get("price") or {}
    return attrs.get("price") or {}


def main():
    args = sys.argv[1:]
    size, sort_key, category, as_json = 8, "-score", None, False
    pos = []
    i = 0
    while i < len(args):
        a = args[i]
        if a == "--size" and i + 1 < len(args):
            size = args[i + 1]; i += 2
        elif a == "--sort" and i + 1 < len(args):
            sort_key = args[i + 1]; i += 2
        elif a == "--category" and i + 1 < len(args):
            category = args[i + 1]; i += 2
        elif a == "--json":
            as_json = True; i += 1
        elif a in ("-h", "--help"):
            print("Usage: vk-stock QUERY [--size N] [--sort KEY] [--category KEY] [--json]")
            return 0
        else:
            pos.append(a); i += 1
    query = " ".join(pos).strip()
    if not query:
        print("error: a search query is required", file=sys.stderr)
        print("Usage: vk-stock QUERY [--size N] [--sort KEY] [--category KEY] [--json]", file=sys.stderr)
        return 2

    sid = os.environ.get("VK_SESSION_ID") or str(uuid.uuid4())
    sflags = ["fi", "search-products", "--filter-q", query, "--session-id", sid,
              "--page-size", str(size), "--sort", sort_key, "--json", "--no-learn"]
    if category:
        sflags += ["--filter-category", category]

    search = json.loads(run(sflags))
    prods = search.get("results") or search.get("data") or []
    if as_json:
        print(json.dumps(search, indent=2, ensure_ascii=False))
        return 0
    if not prods:
        print(f"No results for: {query}")
        return 0

    pids = [p.get("id") for p in prods if p.get("id")]
    status = {}
    if pids:
        avail = json.loads(run(["fi", "get-availability", "--pids", ",".join(map(str, pids)),
                                "--json", "--no-learn"]))
        for it in avail.get("results") or []:
            pid = str(it.get("pid") or it.get("id") or "")
            st = (it.get("status") or {})
            status[pid] = st.get("schema") or st.get("og") or "?"

    print(f"{'PID':<9}{'NAME':<62}{'PRICE':>9}  {'WAS':>9}  STOCK")
    for p in prods:
        pid = p.get("id")
        attrs = p.get("attributes") or {}
        name = (attrs.get("name") or "")[:60]
        price = price_of(p)
        cur = price.get("current")
        orig = price.get("original")
        cur_s = "" if cur is None else f"{float(cur):,.2f}".replace(",", " ")
        orig_s = "" if (orig is None or orig == cur) else f"{float(orig):,.2f}".replace(",", " ")
        st = status.get(str(pid), "?")
        print(f"{str(pid):<9}{name:<62}{cur_s:>9}  {orig_s:>9}  {st}")
    if len(prods) >= 25:
        print(f"\n{len(prods)} results. Narrow with --category / --size, or pass --json for full data.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
