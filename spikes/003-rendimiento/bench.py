#!/usr/bin/env python3
"""Spike 003 runner: builds inputs, runs each implementation, measures wall time and peak RSS.

Usage: python3 bench.py OUT_DIR   (OUT_DIR holds generated data and builds; not committed)
Peak RSS per run comes from os.wait4 (ru_maxrss, KiB on Linux).
"""
import json, os, pathlib, shutil, statistics, subprocess, sys, time

HERE = pathlib.Path(__file__).resolve().parent
FIXTURES = HERE.parent / '002-invocacion-headless' / 'fixtures'
OUT = pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else HERE / 'out').resolve()
OUT.mkdir(parents=True, exist_ok=True)
LINES = 200_000
DECISIONS = 100_000
STARTUP_RUNS = 30
HEAVY_RUNS = 5


def make_jsonl() -> pathlib.Path:
    path = OUT / 'events.jsonl'
    if path.exists():
        return path
    src = []
    for name in ('claude-stream-ok.jsonl', 'claude-stream-sigint.jsonl'):
        src += [l for l in (FIXTURES / name).read_text(encoding='utf-8').splitlines() if l.strip()]
    with path.open('w', encoding='utf-8') as fh:
        for i in range(LINES):
            fh.write(src[i % len(src)] + '\n')
    return path


def run(cmd):
    t0 = time.perf_counter()
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    out = p.stdout.read()
    err = p.stderr.read()
    _, status, ru = os.wait4(p.pid, 0)
    dt = time.perf_counter() - t0
    if os.waitstatus_to_exitcode(status) != 0:
        raise RuntimeError(f'{cmd} failed: {err.decode()[:300]}')
    return dt, ru.ru_maxrss / 1024, out.decode().strip()


def measure(cmd, runs):
    times, rss, out = [], [], ''
    for _ in range(runs):
        dt, mem, out = run(cmd)
        times.append(dt)
        rss.append(mem)
    return statistics.median(times) * 1000, max(rss), out


def main():
    data = make_jsonl()
    go_bin = OUT / 'hilvan-go'
    java_cp = OUT / 'java'
    impls = {
        'Node.js 22 (TS→JS)': ['node', str(HERE / 'node' / 'bench.mjs')],
        'Go 1.17': [str(go_bin)],
        'Python 3.14': [sys.executable, str(HERE / 'python' / 'bench.py')],
        'Java 25 (JVM)': ['java', '-cp', str(java_cp), 'Bench'],
    }
    artefact = {
        'Node.js 22 (TS→JS)': (HERE / 'node' / 'bench.mjs').stat().st_size,
        'Go 1.17': go_bin.stat().st_size if go_bin.exists() else 0,
        'Python 3.14': (HERE / 'python' / 'bench.py').stat().st_size,
        'Java 25 (JVM)': sum(f.stat().st_size for f in java_cp.rglob('*.class')),
    }
    runtime = {
        'Node.js 22 (TS→JS)': 'Node ≥ 22 (~100 MB instalado)',
        'Go 1.17': 'ninguno (binario estático)',
        'Python 3.14': 'Python 3 (~100 MB)',
        'Java 25 (JVM)': 'JRE (~200 MB)',
    }
    rows = []
    for name, base in impls.items():
        print(f'… {name}', file=sys.stderr)
        start_ms, start_rss, _ = measure(base + ['version'], STARTUP_RUNS)
        crit_ms, crit_rss, crit_out = measure(base + ['criterio', str(DECISIONS)], HEAVY_RUNS)
        jsonl_ms, jsonl_rss, jsonl_out = measure(base + ['jsonl', str(data)], HEAVY_RUNS)
        spawn_ms, spawn_rss, _ = measure(base + ['spawn', str(data)], HEAVY_RUNS)
        rows.append(dict(name=name, start_ms=start_ms, start_rss=start_rss, crit_ms=crit_ms,
                         crit_rss=crit_rss, crit_out=crit_out, jsonl_ms=jsonl_ms, jsonl_rss=jsonl_rss,
                         jsonl_out=jsonl_out, spawn_ms=spawn_ms, spawn_rss=spawn_rss,
                         artefact_kb=artefact[name] / 1024, runtime=runtime[name]))
    (OUT / 'results.json').write_text(json.dumps(rows, indent=2), encoding='utf-8')
    print(f'| Implementación | Arranque (ms) | RSS arranque (MB) | Criterio {DECISIONS//1000}k (ms) | '
          f'JSONL {LINES//1000}k líneas (ms) | RSS JSONL (MB) | Spawn+stream (ms) | Artefacto (KB) | Runtime requerido |')
    print('|---|---:|---:|---:|---:|---:|---:|---:|---|')
    for r in rows:
        print(f"| {r['name']} | {r['start_ms']:.0f} | {r['start_rss']:.0f} | {r['crit_ms']:.0f} | {r['jsonl_ms']:.0f} | "
              f"{r['jsonl_rss']:.0f} | {r['spawn_ms']:.0f} | {r['artefact_kb']:.0f} | {r['runtime']} |")
    print('\nParidad:', file=sys.stderr)
    for r in rows:
        print(f"  {r['name']}: {r['crit_out']} | {r['jsonl_out']}", file=sys.stderr)


if __name__ == '__main__':
    main()
