#!/usr/bin/env python3
"""Rebuild id_map.json from indexed API documents (title unix-{beir_id})."""

from __future__ import annotations

import argparse
import re
import sys

import requests

from common import EVAL_COLLECTION_ID, ID_MAP_PATH, ensure_data_dir, merge_id_maps, save_json

TITLE_RE = re.compile(r"^unix-(\d+)$")


def sync_from_api(api_url: str) -> int:
    base = api_url.rstrip("/")
    beir_to_flux: dict[str, str] = {}
    offset = 0
    while True:
        resp = requests.get(
            f"{base}/api/v1/documents",
            params={"collection_id": EVAL_COLLECTION_ID, "limit": 500, "offset": offset},
            timeout=60,
        )
        resp.raise_for_status()
        docs = resp.json().get("documents", [])
        if not docs:
            break
        for doc in docs:
            if doc.get("status") != "indexed":
                continue
            flux_id = doc.get("id")
            title = (doc.get("title") or "").strip()
            if not flux_id or not title:
                continue
            match = TITLE_RE.match(title)
            if not match:
                continue
            beir_to_flux[match.group(1)] = str(flux_id)
        if len(docs) < 500:
            break
        offset += 500

    merged = merge_id_maps({"beir_to_flux": beir_to_flux, "flux_to_beir": {}})
    save_json(ID_MAP_PATH, merged)
    print(f"Synced id_map: {len(merged['flux_to_beir']):,} mapped documents -> {ID_MAP_PATH}")
    return 0


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Rebuild id_map.json from API indexed docs")
    parser.add_argument("--api-url", default="http://localhost:8080")
    args = parser.parse_args()

    ensure_data_dir()
    return sync_from_api(args.api_url)


if __name__ == "__main__":
    raise SystemExit(main())
