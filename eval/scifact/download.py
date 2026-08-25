#!/usr/bin/env python3
"""Download BEIR SciFact corpus, queries, and test qrels from Hugging Face."""

from __future__ import annotations

import argparse
import json
import sys

from datasets import load_dataset
from huggingface_hub import hf_hub_download

from common import (
    CORPUS_PATH,
    DATASET_NAME,
    QRELS_PATH,
    QUERIES_PATH,
    ensure_data_dir,
    save_json,
)


def build_corpus() -> int:
    ds = load_dataset("BeIR/scifact", "corpus", split="corpus")
    count = 0
    with CORPUS_PATH.open("w", encoding="utf-8") as out:
        for record in ds:
            doc = {
                "_id": str(record["_id"]),
                "title": (record.get("title") or "").strip(),
                "text": (record.get("text") or "").strip(),
            }
            out.write(json.dumps(doc, ensure_ascii=False) + "\n")
            count += 1
    return count


def build_queries() -> int:
    ds = load_dataset("BeIR/scifact", "queries", split="queries")
    count = 0
    with QUERIES_PATH.open("w", encoding="utf-8") as out:
        for record in ds:
            title = (record.get("title") or "").strip()
            text = (record.get("text") or "").strip()
            query_text = text if text else title
            if title and text and title != text:
                query_text = f"{title}\n{text}"
            doc = {"_id": str(record["_id"]), "text": query_text}
            out.write(json.dumps(doc, ensure_ascii=False) + "\n")
            count += 1
    return count


def build_qrels() -> tuple[int, int]:
    path = hf_hub_download("mteb/scifact", "qrels/test.tsv", repo_type="dataset")
    qrels: dict[str, list[str]] = {}
    with open(path, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("query-id"):
                continue
            parts = line.split("\t")
            if len(parts) < 3:
                continue
            query_id, corpus_id, score = parts[0], parts[1], parts[2]
            if int(score) <= 0:
                continue
            qrels.setdefault(query_id, []).append(corpus_id)

    save_json(QRELS_PATH, qrels)
    rel_total = sum(len(v) for v in qrels.values())
    return len(qrels), rel_total


def main() -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")

    parser = argparse.ArgumentParser(description=f"Download {DATASET_NAME} dataset")
    parser.parse_args()

    ensure_data_dir()
    print(f"Downloading {DATASET_NAME} from Hugging Face...")

    corpus_n = build_corpus()
    print(f"  corpus:  {corpus_n:,} docs -> {CORPUS_PATH}")

    queries_n = build_queries()
    print(f"  queries: {queries_n:,} -> {QUERIES_PATH}")

    qrels_n, rel_n = build_qrels()
    print(f"  qrels:   {qrels_n:,} test queries, {rel_n:,} relevance pairs -> {QRELS_PATH}")
    print("Done.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
