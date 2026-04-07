#!/usr/bin/env python3
"""Analyze Claude Code JSON output files and compare token usage."""

import json
import sys


def stats(path):
    data = json.load(open(path))
    result = next((m for m in data if m.get("type") == "result"), {})

    total_in = 0
    total_out = 0
    total_cache = 0
    tool_count = 0

    for msg in data:
        u = msg.get("message", {}).get("usage", {})
        total_in += u.get("input_tokens", 0)
        total_out += u.get("output_tokens", 0)
        total_cache += (
            u.get("cache_read_input_tokens", 0)
            + u.get("cache_creation_input_tokens", 0)
        )

        if msg.get("type") == "assistant":
            for c in msg.get("message", {}).get("content", []):
                if c.get("type") == "tool_use":
                    tool_count += 1

    return {
        "duration": result.get("duration_ms", 0) / 1000,
        "turns": result.get("num_turns", 0),
        "tool_calls": tool_count,
        "input": total_in,
        "output": total_out,
        "cache": total_cache,
        "total": total_in + total_out + total_cache,
    }


def tool_calls(path):
    data = json.load(open(path))
    calls = []
    for msg in data:
        if msg.get("type") == "assistant":
            for c in msg.get("message", {}).get("content", []):
                if c.get("type") == "tool_use":
                    inp = c.get("input", {})
                    name = c["name"]
                    if name == "Bash":
                        calls.append(("Bash", inp.get("command", "")[:100]))
                    elif name == "Write":
                        calls.append(("Write", inp.get("path", "")))
                    elif name == "Read":
                        calls.append(("Read", inp.get("path", "")[:100]))
                    else:
                        calls.append((name, str(inp)[:100]))
    return calls


def pct(new, old):
    if old == 0:
        return "n/a"
    return f"{(new - old) / old * 100:+.0f}%"


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <result1.json> [result2.json] [result3.json]")
        sys.exit(1)

    labels = [chr(ord('A') + i) for i in range(len(sys.argv) - 1)]
    results = []

    for i, path in enumerate(sys.argv[1:]):
        s = stats(path)
        results.append((labels[i], path, s))

    # Summary table
    header = f"{'':20}"
    for label, path, _ in results:
        name = path.rsplit("/", 1)[-1].replace("result-", "").replace("-multi.json", "").replace(".json", "")
        header += f" {name:>12}"
    print(header)
    print("-" * (20 + 13 * len(results)))

    for metric in ["duration", "turns", "tool_calls", "input", "output", "cache", "total"]:
        row = f"{metric:20}"
        for _, _, s in results:
            val = s[metric]
            if metric == "duration":
                row += f" {val:>11.1f}s"
            else:
                row += f" {val:>12,}"
        print(row)

    # Percentage vs first
    if len(results) > 1:
        base = results[0][2]
        print(f"\n{'vs ' + results[0][1].rsplit('/', 1)[-1]:>20}", end="")
        for _, _, s in results:
            print(f" {pct(s['total'], base['total']):>12}", end="")
        print()

    # Tool calls detail
    for label, path, _ in results:
        print(f"\n=== {path} tool calls ===")
        for i, (name, detail) in enumerate(tool_calls(path), 1):
            print(f"  {i:2}. {name}: {detail}")


if __name__ == "__main__":
    main()
