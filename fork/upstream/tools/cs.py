#!/usr/bin/env python3
"""cs.py: search the local corpus of cue-lang/cue's public record.

Usage: cs.py [-i] [--since YYYY-MM-DD] [--all] REGEX [REGEX ...]
  CUE_CORPUS names the directory written by fetch-corpus.py.
  Every REGEX must match the same document (AND). Case-insensitive with -i.
  Sources: GitHub issues/PRs (title+body), issue/PR comments, PR review
  comments, GitHub Discussions (with comments), public Gerrit changes
  (subject, commit message, review messages; visible up to ~2026-08), and
  upstream git log messages (upstream/master).
  Prints one line per hit: source, id, date, state, title, and a snippet
  around the first match. --all prints every hit (default: first 40).
"""
import json, os, re, subprocess, sys

D = os.environ.get("CUE_CORPUS") or sys.exit("set CUE_CORPUS to the directory fetch-corpus.py wrote")
args = sys.argv[1:]
flags = 0; since = ''; limit = 40
while args and args[0].startswith('-'):
    a = args.pop(0)
    if a == '-i': flags = re.I
    elif a == '--since': since = args.pop(0)
    elif a == '--all': limit = 10**9
pats = [re.compile(p, flags) for p in args]
if not pats:
    sys.exit(__doc__)

def load(name):
    p = os.path.join(D, name)
    return [json.loads(l) for l in open(p)] if os.path.exists(p) else []

issues = {x['n']: x for x in load('issues.jsonl')}
docs = []
for x in issues.values():
    kind = 'PR' if x['pr'] else 'issue'
    docs.append((kind, '#%d' % x['n'], x['created'][:10], x['state'], x['title'], x['title'] + '\n' + x['body']))
for c in load('comments.jsonl'):
    i = issues.get(c['n'], {})
    docs.append(('comment', '#%d' % c['n'], c['created'][:10], i.get('state', '?'), i.get('title', ''), c['body']))
for c in load('reviewcomments.jsonl'):
    i = issues.get(c['n'], {})
    docs.append(('review', '#%d' % c['n'], c['created'][:10], i.get('state', '?'), i.get('title', ''), c['path'] + '\n' + c['body']))
for x in load('discussions.jsonl'):
    docs.append(('discussion', 'D%d' % x['n'], x['created'][:10], x['cat'], x['title'], x['title'] + '\n' + x['text']))
for x in load('gerrit.jsonl'):
    docs.append(('gerrit', 'CL%d' % x['cl'], x['updated'], x['status'], x['subject'], x['subject'] + '\n' + x['message'] + '\n' + x['review']))
log = subprocess.run(['git', '-C', os.path.expanduser('~/code/graphops/cue'), 'log', 'upstream/master',
                      '--format=%x1e%h%x1f%cs%x1f%s%x1f%B'], capture_output=True, text=True).stdout
for rec in log.split('\x1e')[1:]:
    h, d, s, b = rec.split('\x1f', 3)
    docs.append(('commit', h, d, 'master', s, b))

hits = 0
for kind, ident, date, state, title, text in sorted(docs, key=lambda d: d[2], reverse=True):
    if since and date < since:
        continue
    if all(p.search(text) for p in pats):
        m = pats[0].search(text)
        snip = text[max(0, m.start() - 80): m.end() + 80].replace('\n', ' ')
        print('%-10s %-8s %s %-9s %s\n           …%s…' % (kind, ident, date, state, title[:70], snip))
        hits += 1
        if hits >= limit:
            print('(more hits; use --all)')
            break
print('%d hit(s)' % hits, file=sys.stderr)
