"""Spike 003 — Python slice of Hilvan (stdlib only)."""
import json, math, re, subprocess, sys

TEXTS = [
    'Refactor the Codex adapter to normalise JSONL events and add tests',
    'Fix bug in login flow when the token expires',
    'Document the API in the README with examples',
    'Investigate terminal UI alternatives for low resource machines',
    'Add support for SSH destinations in the launcher',
    'Review the authentication pull request for security issues',
    'Increase unit test coverage of the policy evaluator',
    'Implement export of task history to CSV files',
]
K, BUCKETS = 7, 65536
SPLIT = re.compile(r'[^a-z0-9]+')

def fnv1a(s: str) -> int:
    h = 0x811C9DC5
    for ch in s:
        h ^= ord(ch)
        h = (h * 0x01000193) & 0xFFFFFFFF
    return h

def tokenize(t):
    return [w[:6] if len(w) > 6 else w for w in SPLIT.split(t.lower()) if len(w) >= 2]

def features(t):
    toks = tokenize(t)
    f = []
    for i, tok in enumerate(toks):
        f.append('w:' + tok)
        if i + 1 < len(toks):
            f.append(f'b:{tok}_{toks[i + 1]}')
    return f

def weights_for(b):
    return [(((b * (c + 1) * 2654435761) & 0xFFFFFFFF) % 1000) / 1000 - 0.5 for c in range(K)]

def criterio(n):
    W = {}
    for t in TEXTS:
        for f in features(t):
            b = fnv1a(f) % BUCKETS
            W[b] = weights_for(b)
    check = 0
    for i in range(n):
        z = [0.0] * K
        for f in features(f'{TEXTS[i % 8]} {i % 100}'):
            w = W.get(fnv1a(f) % BUCKETS)
            if w:
                for c in range(K):
                    z[c] += w[c]
        m = max(z)
        p = [math.exp(v - m) for v in z]
        check += max(range(K), key=lambda c: p[c])
    print(f'criterio n={n} check={check}')

def read_lines(lines):
    st = dict(lines=0, session=0, text=0, limits=0, results=0, tokens=0)
    for line in lines:
        line = line.strip()
        if not line:
            continue
        st['lines'] += 1
        e = json.loads(line)
        t = e.get('type')
        if t == 'system' and e.get('subtype') == 'init':
            st['session'] += 1
        elif t == 'assistant':
            st['text'] += sum(1 for c in e.get('message', {}).get('content', []) if c.get('type') == 'text')
        elif t == 'rate_limit_event':
            st['limits'] += 1
        elif t == 'result':
            st['results'] += 1
            u = e.get('usage') or {}
            st['tokens'] += u.get('input_tokens', 0) + u.get('output_tokens', 0)
    print('lines={lines} session={session} text={text} limits={limits} results={results} tokens={tokens}'.format(**st))

def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else ''
    if mode == 'version':
        print('hilvan-python 0.0.0')
    elif mode == 'criterio':
        criterio(int(sys.argv[2]))
    elif mode == 'jsonl':
        with open(sys.argv[2], encoding='utf-8') as fh:
            read_lines(fh)
    elif mode == 'spawn':
        p = subprocess.Popen(['cat', sys.argv[2]], stdout=subprocess.PIPE, text=True, encoding='utf-8')
        read_lines(p.stdout)
        p.wait()
    else:
        sys.exit('usage: version | criterio N | jsonl FILE | spawn FILE')

main()
