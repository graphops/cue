#!/usr/bin/env python3
"""regen-pr.py TOPIC BRANCH: rewrite fork/upstream/TOPIC/PR.md from the live
branch on cjorge-graphops/cue, so its title and body are exactly the commit's.

Run from the cue-upstream checkout root with ~/code/graphops/cue fetched. The
header names the commit, the fixup commits to squash at filing time, and the
files that still hold the XXXX issue-number placeholder. Sections of the old
PR.md from the first "## At filing time" or "## Reviewer note" heading on
are kept as they are.
"""
import os, re, subprocess, sys

topic, branch = sys.argv[1], sys.argv[2]
repo = os.path.expanduser('~/code/graphops/cue')
ref = 'cjorge/' + branch

def git(*a):
    return subprocess.run(['git', '-C', repo, *a], capture_output=True, text=True, check=True).stdout

commits = git('log', '--reverse', '--format=%h %s', 'cjorge/master..' + ref).splitlines()
main = [c for c in commits if not c.split(' ', 1)[1].startswith('fixup! ')]
fixups = [c.split()[0] for c in commits if c.split(' ', 1)[1].startswith('fixup! ')]
assert len(main) == 1, commits
sha = main[0].split()[0]
subject = git('log', '-1', '--format=%s', sha).strip()
body = git('log', '-1', '--format=%b', sha).strip('\n')
base = git('rev-parse', '--short', 'cjorge/master').strip()
placeholders = sorted(f for f in git('grep', '-l', 'XXXX', ref, '--').splitlines()
                      if f.split(':', 1)[1] in git('diff', '--name-only', 'cjorge/master', ref).split())
placeholders = [f.split(':', 1)[1] for f in placeholders]

path = os.path.join('fork/upstream', topic, 'PR.md')
old = open(path).read() if os.path.exists(path) else ''
m = re.search(r'^## (At filing time|Reviewer note)', old, re.M)
kept = old[m.start():].rstrip('\n') + '\n' if m else ''

fix = (' plus fixup %s, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`)'
       % ', '.join(fixups)) if fixups else ''
out = ['# PR: %s' % topic, '',
       'Branch: cjorge-graphops/cue `%s`: commit %s%s. Based on `cjorge/master` %s (upstream master, v0.18.0-rc.3).' % (branch, sha, fix, base),
       '',
       'Before opening, replace `XXXX` with the issue number in the commit message and in: %s.'
       % ', '.join('`%s`' % p for p in placeholders) if placeholders else
       'Before opening, replace `XXXX` with the issue number in the commit message.',
       '', '## Title', '', '```', subject, '```', '',
       '## Body (plain text, as in the commit message)', '', '```', body, '```', '']
open(path, 'w').write('\n'.join(out) + ('\n' + kept if kept else ''))
print('%s: %s%s, placeholders in %d files, kept %d chars' % (topic, sha, ' + ' + ','.join(fixups) if fixups else '', len(placeholders), len(kept)))
