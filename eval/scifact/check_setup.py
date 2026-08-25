#!/usr/bin/env python3
"""Verify SciFact eval prerequisites: API settings, PostgreSQL collection, Milvus vector dim."""

from __future__ import annotations

import argparse
import subprocess
import sys

import requests

from common import EVAL_COLLECTION_ID, EVAL_MILVUS_COLLECTION, ROOT


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Check SciFact eval setup")
    parser.add_argument("--api-url", default="http://localhost:8080")
    args = parser.parse_args()

    ok = True
    base = args.api_url.rstrip("/")

    print("=== SciFact eval setup check ===\n")

    try:
        health = requests.get(f"{base}/healthz", timeout=5)
        health.raise_for_status()
        print("[ok] API reachable")
    except requests.RequestException as exc:
        print(f"[fail] API not reachable: {exc}")
        return 1

    settings = requests.get(f"{base}/api/v1/settings", timeout=10).json()
    embed_dim = int(settings.get("embedding_dim") or 0)
    embed_status = settings.get("embedding_status", "")
    print(f"[ok] embedding_dim={embed_dim} ({embed_status})")
    if not settings.get("embedding_ready"):
        print("[fail] embedding not configured")
        ok = False

    env = {
        **dict(__import__("os").environ),
        "FLUXSEARCH_MILVUS_COLLECTION": EVAL_MILVUS_COLLECTION,
    }
    proc = subprocess.run(
        ["go", "run", "./cmd/ensure-milvus"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
        timeout=60,
        check=False,
    )
    milvus_line = (proc.stdout or proc.stderr or "").strip().splitlines()[-1:] or [""]
    print(f"[{'ok' if proc.returncode == 0 else 'fail'}] {milvus_line[0]}")
    if proc.returncode != 0:
        ok = False
    elif f"actual_dim={embed_dim}" not in milvus_line[0]:
        print(
            f"[fail] Milvus eval collection dim does not match API embedding_dim={embed_dim}. "
            f"Stop API, then run:\n"
            f"  $env:FLUXSEARCH_MILVUS_COLLECTION = \"{EVAL_MILVUS_COLLECTION}\"\n"
            f"  go run ./cmd/ensure-milvus -recreate\n"
            f"  go run ./cmd/api"
        )
        ok = False

    print(f"\nEval collection_id: {EVAL_COLLECTION_ID}")
    print(f"Milvus collection:  {EVAL_MILVUS_COLLECTION}")

    if ok:
        print("\nReady. Run:")
        print("  python eval/scifact/import_corpus.py --purge --limit 200")
        print("  python eval/scifact/run_eval.py --query-limit 50")
        return 0

    print("\nFix the issues above, then re-run this script.")
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
