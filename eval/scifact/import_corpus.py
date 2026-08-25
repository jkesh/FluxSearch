#!/usr/bin/env python3
"""Import BEIR SciFact corpus into FluxSearch eval collection."""

from __future__ import annotations

import argparse
import atexit
import os
import sys
import time
from concurrent.futures import ThreadPoolExecutor, as_completed

import requests
from tqdm import tqdm

from common import (
    CORPUS_PATH,
    EVAL_COLLECTION_ID,
    ID_MAP_PATH,
    IMPORT_LOCK_PATH,
    IMPORT_STATE_PATH,
    ensure_data_dir,
    iter_jsonl,
    load_json,
    merge_id_maps,
    save_json,
)


def compose_content(doc: dict) -> str:
    title = (doc.get("title") or "").strip()
    text = (doc.get("text") or "").strip()
    if title and text:
        return f"{title}\n\n{text}"
    return title or text


def import_one(api_url: str, doc: dict, timeout: float) -> tuple[str, str | None, str | None]:
    beir_id = str(doc["_id"])
    title = f"scifact-{beir_id}"
    payload = {
        "title": title,
        "content": compose_content(doc),
        "source_type": "txt",
        "collection_id": EVAL_COLLECTION_ID,
        "metadata": {
            "beir_id": beir_id,
            "beir_dataset": "scifact",
            "eval_dataset": "scifact",
        },
    }
    try:
        resp = requests.post(f"{api_url.rstrip('/')}/api/v1/documents", json=payload, timeout=timeout)
        if resp.status_code not in (200, 201):
            detail = resp.text[:300]
            if "documents_collection_id_fkey" in detail:
                detail += (
                    " | Run: python eval/scifact/setup_collection.py "
                    "then FLUXSEARCH_MILVUS_COLLECTION=fluxsearch_eval_scifact "
                    "go run ./cmd/ensure-milvus -recreate"
                )
            return beir_id, None, f"HTTP {resp.status_code}: {detail}"
        body = resp.json()
        flux_id = body.get("document_id")
        if not flux_id:
            return beir_id, None, "missing document_id in response"
        status = body.get("status")
        outcome = body.get("outcome")
        if status != "indexed":
            if outcome == "skipped":
                try:
                    requests.delete(
                        f"{api_url.rstrip('/')}/api/v1/documents/{flux_id}",
                        timeout=timeout,
                    )
                    resp = requests.post(
                        f"{api_url.rstrip('/')}/api/v1/documents",
                        json=payload,
                        timeout=timeout,
                    )
                    if resp.status_code not in (200, 201):
                        detail = resp.text[:300]
                        return beir_id, None, f"retry HTTP {resp.status_code}: {detail}"
                    body = resp.json()
                    flux_id = body.get("document_id")
                    status = body.get("status")
                    outcome = body.get("outcome")
                except requests.RequestException as exc:
                    return beir_id, None, f"retry after delete failed: {exc}"
            if status != "indexed":
                return beir_id, None, f"document not indexed (status={status}, outcome={outcome})"
        if not body.get("vectors_stored", True):
            return beir_id, None, "vectors not stored"
        return beir_id, str(flux_id), None
    except requests.RequestException as exc:
        return beir_id, None, str(exc)


def acquire_import_lock() -> None:
    if IMPORT_LOCK_PATH.exists():
        try:
            pid = int(IMPORT_LOCK_PATH.read_text(encoding="utf-8").strip())
            os.kill(pid, 0)
            raise SystemExit(
                f"Another import is already running (pid={pid}). "
                f"Stop it or remove {IMPORT_LOCK_PATH} if stale."
            )
        except ValueError:
            raise SystemExit(f"Invalid import lock at {IMPORT_LOCK_PATH}")
        except OSError:
            IMPORT_LOCK_PATH.unlink(missing_ok=True)

    IMPORT_LOCK_PATH.write_text(str(os.getpid()), encoding="utf-8")

    def release() -> None:
        IMPORT_LOCK_PATH.unlink(missing_ok=True)

    atexit.register(release)


def persist_id_map(beir_to_flux: dict[str, str], flux_to_beir: dict[str, str]) -> None:
    merged = merge_id_maps({"beir_to_flux": beir_to_flux, "flux_to_beir": flux_to_beir})
    beir_to_flux.clear()
    beir_to_flux.update(merged["beir_to_flux"])
    flux_to_beir.clear()
    flux_to_beir.update(merged["flux_to_beir"])
    save_json(ID_MAP_PATH, merged)


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Import SciFact corpus into FluxSearch")
    parser.add_argument("--api-url", default="http://localhost:8080", help="FluxSearch API base URL")
    parser.add_argument("--limit", type=int, default=0, help="Max documents to import (0 = all)")
    parser.add_argument("--workers", type=int, default=2, help="Parallel import workers")
    parser.add_argument("--timeout", type=float, default=120.0, help="HTTP timeout per document (seconds)")
    parser.add_argument("--resume", action="store_true", help="Skip documents already in id_map.json")
    parser.add_argument("--purge", action="store_true", help="Delete existing eval documents before import")
    args = parser.parse_args()

    acquire_import_lock()

    if args.purge:
        from purge_collection import main as purge_main

        purge_argv = ["purge_collection.py", "--api-url", args.api_url, "--yes"]
        old_argv = sys.argv
        sys.argv = purge_argv
        try:
            purge_main()
        finally:
            sys.argv = old_argv
        save_json(ID_MAP_PATH, {"beir_to_flux": {}, "flux_to_beir": {}})

    ensure_data_dir()
    if not CORPUS_PATH.exists():
        print(f"ERROR: corpus not found at {CORPUS_PATH}")
        print("Run: python eval/scifact/download.py")
        return 1

    health = requests.get(f"{args.api_url.rstrip('/')}/healthz", timeout=10)
    health.raise_for_status()

    id_map = load_json(ID_MAP_PATH, {"beir_to_flux": {}, "flux_to_beir": {}})
    beir_to_flux: dict[str, str] = id_map.get("beir_to_flux", {})
    flux_to_beir: dict[str, str] = id_map.get("flux_to_beir", {})
    state = load_json(IMPORT_STATE_PATH, {"imported": 0, "failed": 0, "errors": []})

    docs: list[dict] = []
    for doc in iter_jsonl(CORPUS_PATH):
        beir_id = str(doc["_id"])
        if args.resume and beir_id in beir_to_flux:
            continue
        docs.append(doc)
        if args.limit > 0 and len(docs) >= args.limit:
            break

    if not docs:
        print("Nothing to import (all documents already mapped).")
        return 0

    print(
        f"Importing {len(docs):,} documents into collection {EVAL_COLLECTION_ID} "
        f"via {args.api_url} (workers={args.workers})..."
    )

    imported = 0
    failed = 0
    errors: list[dict] = []

    with ThreadPoolExecutor(max_workers=max(1, args.workers)) as pool:
        futures = {
            pool.submit(import_one, args.api_url, doc, args.timeout): doc
            for doc in docs
        }
        for future in tqdm(as_completed(futures), total=len(futures), desc="import"):
            beir_id, flux_id, err = future.result()
            if err:
                failed += 1
                if len(errors) < 50:
                    errors.append({"beir_id": beir_id, "error": err})
                continue
            beir_to_flux[beir_id] = flux_id
            flux_to_beir[flux_id] = beir_id
            imported += 1
            if imported % 25 == 0:
                persist_id_map(beir_to_flux, flux_to_beir)

    persist_id_map(beir_to_flux, flux_to_beir)
    save_json(
        IMPORT_STATE_PATH,
        {
            "imported": state.get("imported", 0) + imported,
            "failed": state.get("failed", 0) + failed,
            "errors": (state.get("errors", []) + errors)[-100:],
            "updated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        },
    )

    print(f"Done. imported={imported:,} failed={failed:,} total_mapped={len(beir_to_flux):,}")
    print(f"id_map -> {ID_MAP_PATH}")
    if failed:
        print("Some imports failed. Check import_state.json for details.")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
