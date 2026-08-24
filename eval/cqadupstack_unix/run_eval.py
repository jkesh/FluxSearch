#!/usr/bin/env python3
"""Run retrieval evaluation (Hit@K / MRR@K) for CQADupStack / Unix."""

from __future__ import annotations

import argparse
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

import requests
from tqdm import tqdm

from common import (
    EVAL_COLLECTION_ID,
    ID_MAP_PATH,
    QRELS_PATH,
    QUERIES_PATH,
    REPORT_DIR,
    ensure_data_dir,
    hit_at_k,
    iter_jsonl,
    load_json,
    mrr_at_k,
    recall_at_k,
    save_json,
)


def flux_ids_to_beir(flux_ids: list[str], flux_to_beir: dict[str, str]) -> list[str]:
    ranked: list[str] = []
    for flux_id in flux_ids:
        beir_id = flux_to_beir.get(flux_id)
        if beir_id and beir_id not in ranked:
            ranked.append(beir_id)
    return ranked


def search(api_url: str, query: str, top_k: int, timeout: float) -> list[str]:
    payload = {
        "query": query,
        "top_k": top_k,
        "collection_id": EVAL_COLLECTION_ID,
    }
    resp = requests.post(
        f"{api_url.rstrip('/')}/api/v1/search",
        json=payload,
        timeout=timeout,
    )
    resp.raise_for_status()
    body = resp.json()
    return [str(item["document_id"]) for item in body.get("results", [])]


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description="Evaluate CQADupStack / Unix retrieval")
    parser.add_argument("--api-url", default="http://localhost:8080", help="FluxSearch API base URL")
    parser.add_argument("--top-k", type=int, default=10, help="Search top_k passed to API")
    parser.add_argument("--ks", default="1,5,10", help="Comma-separated K values for metrics")
    parser.add_argument("--query-limit", type=int, default=0, help="Max queries to evaluate (0 = all)")
    parser.add_argument("--timeout", type=float, default=60.0, help="HTTP timeout per query")
    parser.add_argument("--output", default="", help="Report JSON path (default: eval/reports/...)")
    args = parser.parse_args()

    ensure_data_dir()
    for path in (QUERIES_PATH, QRELS_PATH, ID_MAP_PATH):
        if not path.exists():
            print(f"ERROR: missing {path}")
            print("Run download.py and import_corpus.py first.")
            return 1

    ks = sorted({int(k.strip()) for k in args.ks.split(",") if k.strip()})
    if not ks:
        print("ERROR: --ks must contain at least one integer")
        return 1

    id_map = load_json(ID_MAP_PATH, {})
    flux_to_beir: dict[str, str] = id_map.get("flux_to_beir", {})
    if not flux_to_beir:
        print("ERROR: id_map.json is empty. Import corpus first.")
        return 1

    qrels_raw = load_json(QRELS_PATH, {})
    qrels = {qid: set(ids) for qid, ids in qrels_raw.items()}

    queries: list[dict] = []
    for row in iter_jsonl(QUERIES_PATH):
        qid = str(row["_id"])
        if qid not in qrels:
            continue
        queries.append({"_id": qid, "text": row["text"]})
        if args.query_limit > 0 and len(queries) >= args.query_limit:
            break

    if not queries:
        print("ERROR: no queries with qrels found")
        return 1

    health = requests.get(f"{args.api_url.rstrip('/')}/healthz", timeout=10)
    health.raise_for_status()

    print(
        f"Evaluating {len(queries):,} queries (top_k={args.top_k}, ks={ks}) "
        f"against collection {EVAL_COLLECTION_ID}..."
    )

    hits = {k: [] for k in ks}
    mrrs = {k: [] for k in ks}
    recalls = {k: [] for k in ks}
    failures: list[dict] = []
    latencies_ms: list[float] = []

    for query in tqdm(queries, desc="eval"):
        qid = query["_id"]
        relevant = qrels[qid]
        try:
            t0 = time.perf_counter()
            flux_ranked = search(args.api_url, query["text"], args.top_k, args.timeout)
            latencies_ms.append((time.perf_counter() - t0) * 1000)
            ranked = flux_ids_to_beir(flux_ranked, flux_to_beir)
        except requests.RequestException as exc:
            failures.append({"query_id": qid, "error": str(exc)})
            for k in ks:
                hits[k].append(0.0)
                mrrs[k].append(0.0)
                recalls[k].append(0.0)
            continue

        if len(failures) < 20 and not any(doc_id in relevant for doc_id in ranked[: max(ks)]):
            failures.append(
                {
                    "query_id": qid,
                    "query": query["text"][:200],
                    "relevant": sorted(relevant),
                    "retrieved_top5": ranked[:5],
                }
            )

        for k in ks:
            hits[k].append(hit_at_k(ranked, relevant, k))
            mrrs[k].append(mrr_at_k(ranked, relevant, k))
            recalls[k].append(recall_at_k(ranked, relevant, k))

    def mean(values: list[float]) -> float:
        return sum(values) / len(values) if values else 0.0

    def pct(values: list[float]) -> float:
        return round(mean(values) * 100, 2)

    metrics = {
        "dataset": "cqadupstack-unix",
        "collection_id": EVAL_COLLECTION_ID,
        "queries_evaluated": len(queries),
        "top_k": args.top_k,
        "mapped_documents": len(flux_to_beir),
        "latency_ms": {
            "p50": round(sorted(latencies_ms)[len(latencies_ms) // 2], 1) if latencies_ms else 0,
            "p95": round(sorted(latencies_ms)[int(len(latencies_ms) * 0.95)], 1) if latencies_ms else 0,
            "mean": round(mean(latencies_ms), 1) if latencies_ms else 0,
        },
    }
    for k in ks:
        metrics[f"hit@{k}"] = pct(hits[k])
        metrics[f"mrr@{k}"] = round(mean(mrrs[k]), 4)
        metrics[f"recall@{k}"] = pct(recalls[k])

    ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    out_path = args.output or str(REPORT_DIR / f"cqadupstack-unix-{ts}.json")
    report = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "metrics": metrics,
        "failures_sample": failures[:20],
    }
    save_json(Path(out_path), report)

    print("\n=== CQADupStack / Unix Retrieval Report ===")
    for k in ks:
        print(f"Hit@{k}:     {metrics[f'hit@{k}']}%")
        print(f"MRR@{k}:    {metrics[f'mrr@{k}']}")
        print(f"Recall@{k}: {metrics[f'recall@{k}']}%")
    print(
        f"Latency: p50={metrics['latency_ms']['p50']}ms "
        f"p95={metrics['latency_ms']['p95']}ms "
        f"mean={metrics['latency_ms']['mean']}ms"
    )
    print(f"Report saved: {out_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
