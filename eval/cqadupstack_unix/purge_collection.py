#!/usr/bin/env python3
"""Delete all documents in the eval collection (reset before re-import)."""

from __future__ import annotations

import argparse
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests
from requests.adapters import HTTPAdapter
from tqdm import tqdm
from urllib3.util.retry import Retry

from common import EVAL_COLLECTION_ID


def make_session() -> requests.Session:
    session = requests.Session()
    retry = Retry(
        total=5,
        backoff_factor=0.5,
        status_forcelist=(429, 500, 502, 503, 504),
        allowed_methods=("GET", "DELETE"),
    )
    adapter = HTTPAdapter(max_retries=retry, pool_connections=4, pool_maxsize=4)
    session.mount("http://", adapter)
    session.mount("https://", adapter)
    return session


def delete_one(session: requests.Session, base: str, doc_id: str, timeout: float) -> tuple[str, bool, str]:
    try:
        r = session.delete(f"{base}/api/v1/documents/{doc_id}", timeout=timeout)
        if r.status_code in (200, 204):
            return doc_id, True, ""
        return doc_id, False, f"{r.status_code} {r.text[:120]}"
    except requests.RequestException as exc:
        return doc_id, False, str(exc)


def list_documents(session: requests.Session, base: str, limit: int) -> list[dict]:
    for attempt in range(8):
        try:
            resp = session.get(
                f"{base}/api/v1/documents",
                params={"collection_id": EVAL_COLLECTION_ID, "limit": limit},
                timeout=60,
            )
            resp.raise_for_status()
            return resp.json().get("documents", [])
        except requests.RequestException as exc:
            if attempt == 7:
                raise
            time.sleep(min(2**attempt, 10))
            err = str(exc)
            if "10048" in err or "10061" in err:
                time.sleep(2)
    return []


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Purge eval collection documents")
    parser.add_argument("--api-url", default="http://localhost:8080")
    parser.add_argument("--workers", type=int, default=4, help="Parallel delete workers (keep low on Windows)")
    parser.add_argument("--timeout", type=float, default=60.0)
    parser.add_argument("--yes", action="store_true", help="Skip confirmation")
    args = parser.parse_args()

    base = args.api_url.rstrip("/")
    deleted = 0
    warned = 0
    confirmed = args.yes
    workers = max(1, min(args.workers, 8))

    with make_session() as session:
        while True:
            docs = list_documents(session, base, 500)
            if not docs:
                break

            if not confirmed:
                print(f"Will delete documents from eval collection (batch size {len(docs)}, workers={workers}).")
                if input("Continue? [y/N] ").strip().lower() != "y":
                    print("Cancelled.")
                    return 1
                confirmed = True

            with ThreadPoolExecutor(max_workers=workers) as pool:
                futures = {
                    pool.submit(delete_one, session, base, doc["id"], args.timeout): doc["id"]
                    for doc in docs
                }
                for future in tqdm(as_completed(futures), total=len(futures), desc="purge", leave=False):
                    doc_id, ok, err = future.result()
                    if ok:
                        deleted += 1
                    elif warned < 20:
                        print(f"WARN: failed to delete {doc_id}: {err}")
                        warned += 1

    if deleted == 0:
        print("No documents to delete.")
        return 0

    print(f"Deleted {deleted:,} documents.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
