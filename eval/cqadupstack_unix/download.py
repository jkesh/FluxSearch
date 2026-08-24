#!/usr/bin/env python3
"""Download BEIR CQADupStack / Unix corpus, queries, and qrels from Hugging Face."""

from __future__ import annotations

import argparse
import json
import sys

import pandas as pd
from huggingface_hub import hf_hub_download

from common import (
    CORPUS_PATH,
    DATASET_NAME,
    QRELS_PATH,
    QUERIES_PATH,
    ensure_data_dir,
    save_json,
)


def download_parquet(repo_id: str, filename: str) -> str:
    return hf_hub_download(repo_id, filename, repo_type="dataset")


def build_corpus() -> int:
    path = download_parquet("BeIR/cqadupstack", "unix/corpus/corpus-00000-of-00001.parquet")
    df = pd.read_parquet(path)
    count = 0
    with CORPUS_PATH.open("w", encoding="utf-8") as out:
        for record in df.to_dict(orient="records"):
            doc = {
                "_id": str(record["_id"]),
                "title": (record.get("title") or "").strip(),
                "text": (record.get("text") or "").strip(),
            }
            out.write(json.dumps(doc, ensure_ascii=False) + "\n")
            count += 1
    return count


def build_queries() -> int:
    path = download_parquet("BeIR/cqadupstack", "unix/queries/queries-00000-of-00001.parquet")
    df = pd.read_parquet(path)
    count = 0
    with QUERIES_PATH.open("w", encoding="utf-8") as out:
        for record in df.to_dict(orient="records"):
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
    path = download_parquet(
        "orgrctera/beir_cqadupstack_unix",
        "data/test-00000-of-00001.parquet",
    )
    df = pd.read_parquet(path)
    qrels: dict[str, list[str]] = {}
    for _, row in df.iterrows():
        query_id = str(row["metadata.query_id"])
        expected = row["expected_output"]
        if isinstance(expected, str):
            items = json.loads(expected)
        else:
            items = expected
        rel_ids = [str(item["id"]) for item in items if int(item.get("score", 0)) > 0]
        qrels[query_id] = rel_ids

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
    print(f"  qrels:   {qrels_n:,} queries, {rel_n:,} relevance pairs -> {QRELS_PATH}")
    print("Done.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
