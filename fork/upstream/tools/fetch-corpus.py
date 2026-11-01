#!/usr/bin/env python3
"""fetch-corpus.py DIR: download cue-lang/cue's public record into DIR for
offline duplicate searches with cs.py.

Fetches, as JSON lines:
  issues.jsonl         every GitHub issue and pull request (title, body, state)
  comments.jsonl       every issue and pull request comment
  reviewcomments.jsonl every pull request review comment
  discussions.jsonl    every GitHub Discussion with its comments and replies
  gerrit.jsonl         every change visible anonymously on cue.gerrithub.io
                       (subject, commit message, review messages)

Needs an authenticated `gh`. Anonymous Gerrit visibility ends around
2026-08: maintainers' changes since then are reviewed privately, and their
only public traces are issue comments and the commits on master, which
cs.py reads from the local clone.
"""
import datetime, json, os, subprocess, sys, urllib.error, urllib.request

out = sys.argv[1] if len(sys.argv) > 1 else sys.exit(__doc__)
os.makedirs(out, exist_ok=True)

def gh_paginate(path, jq, name):
    with open(os.path.join(out, name), 'w') as f:
        subprocess.run(['gh', 'api', '--paginate', path, '-q', jq], stdout=f, check=True)

gh_paginate('repos/cue-lang/cue/issues?state=all&per_page=100',
            '.[] | {n:.number, pr:(.pull_request!=null), state:.state, title:.title, body:(.body//""), '
            'created:.created_at, labels:[.labels[].name], user:.user.login}', 'issues.jsonl')
gh_paginate('repos/cue-lang/cue/issues/comments?per_page=100',
            '.[] | {n:(.issue_url|split("/")|last|tonumber), user:.user.login, created:.created_at, body:(.body//"")}',
            'comments.jsonl')
gh_paginate('repos/cue-lang/cue/pulls/comments?per_page=100',
            '.[] | {n:(.pull_request_url|split("/")|last|tonumber), user:.user.login, created:.created_at, '
            'path:.path, body:(.body//"")}', 'reviewcomments.jsonl')

q = ('query($cursor:String){repository(owner:"cue-lang",name:"cue"){discussions(first:100, after:$cursor)'
     '{pageInfo{hasNextPage endCursor} nodes{number title body createdAt url category{name} '
     'comments(first:50){nodes{body replies(first:30){nodes{body}}}}}}}}')
with open(os.path.join(out, 'discussions.jsonl'), 'w') as f:
    cursor = None
    while True:
        args = ['gh', 'api', 'graphql', '-f', 'query=' + q] + (['-f', 'cursor=' + cursor] if cursor else [])
        ds = json.loads(subprocess.run(args, capture_output=True, text=True, check=True).stdout)['data']['repository']['discussions']
        for x in ds['nodes']:
            cs = x['comments']['nodes']
            text = [x['body']] + [c['body'] for c in cs] + [r['body'] for c in cs for r in c['replies']['nodes']]
            f.write(json.dumps({'n': x['number'], 'title': x['title'], 'created': x['createdAt'], 'url': x['url'],
                                'cat': x['category']['name'], 'text': '\n'.join(text)}) + '\n')
        if not ds['pageInfo']['hasNextPage']:
            break
        cursor = ds['pageInfo']['endCursor']

# Gerrit caps paging at 1000 results per query, so query by date windows.
seen = {}
def gerrit(a, b):
    start = 0
    while True:
        url = ('https://cue.gerrithub.io/changes/?q=project:cue-lang/cue+after:%s+before:%s&n=100&S=%d'
               '&o=CURRENT_REVISION&o=CURRENT_COMMIT&o=MESSAGES' % (a, b, start))
        with urllib.request.urlopen(url, timeout=120) as r:
            d = json.loads(r.read().decode()[4:])
        for c in d:
            rev = c.get('revisions', {}).get(c.get('current_revision', ''), {})
            seen[c['_number']] = {'cl': c['_number'], 'status': c['status'], 'updated': c['updated'][:10],
                                  'subject': c['subject'], 'message': rev.get('commit', {}).get('message', ''),
                                  'review': '\n'.join(m.get('message', '') for m in c.get('messages', []))}
        if not d or not d[-1].get('_more_changes'):
            return
        start += len(d)
day, end = datetime.date(2019, 1, 1), datetime.date.today() + datetime.timedelta(days=1)
while day < end:
    nxt = min(day + datetime.timedelta(days=45), end)
    gerrit(day.isoformat(), nxt.isoformat())
    day = nxt
with open(os.path.join(out, 'gerrit.jsonl'), 'w') as f:
    for v in sorted(seen.values(), key=lambda x: x['cl']):
        f.write(json.dumps(v) + '\n')
for n in ('issues', 'comments', 'reviewcomments', 'discussions', 'gerrit'):
    print(n, sum(1 for _ in open(os.path.join(out, n + '.jsonl'))))
