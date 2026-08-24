#!/usr/bin/env python3
"""Wait for corpus import to finish, repair if needed, then run full retrieval eval."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

import requests

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(Path(__file__).resolve().parent))

from common import CORPUS_PATH, EVAL_COLLECTION_ID, ID_MAP_PATH, import_lock_active, iter_jsonl  # noqa: E402


def corpus_size() -> int:
    return sum(1 for _ in iter_jsonl(CORPUS_PATH))


def mapped_count(retries: int = 5) -> int:
    for attempt in range(retries):
        try:
            data = json.loads(ID_MAP_PATH.read_text(encoding="utf-8"))
            return len(data.get("flux_to_beir", {}))
        except json.JSONDecodeError:
            if attempt + 1 >= retries:
                raise
            time.sleep(0.5)
    return 0


def api_doc_count(api_url: str) -> int:
    total = 0
    offset = 0
    base = api_url.rstrip("/")
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
        total += len(docs)
        if len(docs) < 500:
            break
        offset += 500
    return total


def run_repair_import(api_url: str, workers: int) -> int:
    cmd = [
        sys.executable,
        str(Path(__file__).with_name("import_corpus.py")),
        "--api-url",
        api_url,
        "--resume",
        "--workers",
        str(workers),
    ]
    print("Repair import:", " ".join(cmd), flush=True)
    return subprocess.call(cmd, cwd=ROOT)


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Wait for import completion and run eval")
    parser.add_argument("--api-url", default="http://localhost:8080")
    parser.add_argument("--target", type=int, default=0, help="Mapped docs target (0 = corpus size)")
    parser.add_argument("--poll-seconds", type=int, default=120)
    parser.add_argument("--repair-workers", type=int, default=2, help="Workers for auto-repair import")
    parser.add_argument("--no-repair", action="store_true", help="Never spawn a repair import")
    parser.add_argument("--top-k", type=int, default=10)
    args = parser.parse_args()

    target = args.target or corpus_size()
    threshold = int(target * 0.98)
    print(f"Waiting for mapped_documents >= {threshold:,} (target {target:,}) ...", flush=True)

    last_mapped = -1
    last_api = -1
    stable_rounds = 0

    while True:
        mapped = mapped_count()
        try:
            api_docs = api_doc_count(args.api_url)
        except requests.RequestException as exc:
            print(f"WARN: api_doc_count failed: {exc}", flush=True)
            api_docs = last_api if last_api >= 0 else 0

        print(f"mapped={mapped:,}/{target:,} api_docs={api_docs:,}", flush=True)
        if mapped >= threshold:
            break

        if mapped == last_mapped and api_docs == last_api:
            stable_rounds += 1
            if (
                not args.no_repair
                and stable_rounds >= 2
                and mapped < threshold
                and not import_lock_active()
            ):
                code = run_repair_import(args.api_url, max(1, args.repair_workers))
                stable_rounds = 0
                if code != 0:
                    print("Repair import finished with errors; will keep waiting.", flush=True)
                continue
            if stable_rounds >= 2 and mapped < threshold and import_lock_active():
                print("Import lock active; waiting for in-progress import...", flush=True)
                stable_rounds = 0
                continue
            if stable_rounds >= 3 and mapped >= threshold:
                print("Import appears stable near target; proceeding.", flush=True)
                break
        else:
            stable_rounds = 0

        last_mapped = mapped
        last_api = api_docs
        time.sleep(max(30, args.poll_seconds))

    cmd = [sys.executable, str(Path(__file__).with_name("run_eval.py")), "--top-k", str(args.top_k)]
    print("Running:", " ".join(cmd), flush=True)
    return subprocess.call(cmd, cwd=ROOT)


if __name__ == "__main__":
    raise SystemExit(main())
