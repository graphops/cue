#!/bin/bash
# Rebuild graphops/v0.18.0 from its two inputs:
#
#   1. the foundation: every commit of the dusk-to-dawn integration branch
#      integration/v0.18.0 (upstream-bound fixes and performance changes on
#      v0.18.0), ported with the fork's identity and trailers; the tree after
#      it must equal the integration branch's tree;
#   2. the GraphOps patches: every commit of each branch in patches.txt;
#
# then the commits of the previous line that follow the patches: the one
# whose subject is $GOLDENS (the recorded results that change only when the
# patches meet the foundation) and those whose subject starts with "fork:".
#
#   bash fork/rebuild.sh
#
# Environment:
#   FOUNDATION_REPO  repository holding the integration branch (default ~/code/personal/cue)
#   FOUNDATION_REF   the integration branch (default integration/v0.18.0)
#   BRANCH           the branch to point at the result (default graphops/v0.18.0)
#   PREV             the line to take the last commits from (default: BRANCH as it is)
#   REBUILD_WT       the worktree to build in (default: cue-rebuild-wt next to the repository)
#   KEEP_WT=1        keep the worktree after a successful build
#
# On a conflict without a recorded resolution the script stops and leaves the
# worktree in the middle of the cherry-pick. See "Rebuilding" in README.md.
set -euo pipefail

HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO=$(git -C "$HERE" rev-parse --path-format=absolute --git-common-dir)
REPO=${REPO%/.git}
BASE=v0.18.0
BRANCH=${BRANCH:-graphops/v0.18.0}
PREV=${PREV:-$BRANCH}
FOUNDATION_REPO=${FOUNDATION_REPO:-$HOME/code/personal/cue}
FOUNDATION_REF=${FOUNDATION_REF:-integration/v0.18.0}
FREF=refs/foundation/integration-v0.18.0
WT=${REBUILD_WT:-$(dirname "$REPO")/cue-rebuild-wt}
GOLDENS="cue/testdata: update recorded results for the fork patches combined"
FORK_ID="Carlos Jorge <carlos@graphops.xyz>"
D2D_SOB="Signed-off-by: Carlos Jorge <114352294+dusk-to-dawn@users.noreply.github.com>"
FORK_SOB="Signed-off-by: Carlos Jorge <carlos@graphops.xyz>"
COAUTHOR="Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"

# What the port of each upstream prerequisite of the foundation changes, by
# upstream commit. A prerequisite the foundation gains needs a note here.
declare -A NOTE=(
	[6b239f5554a456cd29509c688b7181fc140d6f82]="Backported to v0.18.0: its change to functions/tighten.txtar updates
cases that v0.18.0 lacks (added by 82a8938af), so that file stays as on
v0.18.0; its results do not change."
	[cd43cfbecef45e60b821ed6a7a0be0d5baa068b1]="Backported to v0.18.0, which lacks builtins/exists.txtar (a builtin of
master's predicates experiment): that file and its compiler sync mirror
are left out, and the stats of builtins/validators.txtar are those of
v0.18.0."
	[8c4aa86deda5a2999c357f16a40f6ccc488971c4]="Backported to v0.18.0: builtins/exists.txtar is left out; in
context.go only the comment that Validate replaces differs."
	[7d575eaaf686c411a7f2ba40ad68b9a5081e10b0]="Backported to v0.18.0: functions/native.txtar lacks master's cases
before the new ones; the new cases are added to v0.18.0's file and its
results regenerated on v0.18.0. They are upstream's, at other line
numbers."
	[f6483025ccbb2c95c8bc6ea08aace51930a694bc]="Backported to v0.18.0: the code change applies as it is; the stats of
functions/native.txtar are regenerated on v0.18.0, its results are
upstream's."
	[8fbe49d7d8f30a464d4f532625e2de03152c9129]="Backported to v0.18.0: the stats of comprehensions/pushdown.txtar are
regenerated on v0.18.0; the results of the new issue4595 cases are
upstream's."
	[5828251231984a1814dfc0741b5d6fe5f61874d7]="Backported to v0.18.0: applies as it is."
)

die() {
	printf '\nrebuild.sh: FAILED: %s\n' "$*" >&2
	exit 1
}
git_wt() { git -C "$WT" "$@"; }

# --- recorded resolutions (as in the integration branch's build.sh) -------

RRBACKUP=$HERE/rr-cache
CACHE=$(git -C "$REPO" rev-parse --path-format=absolute --git-path rr-cache)
git -C "$REPO" config rerere.enabled true
restore_resolutions() {
	local d id f
	[ -d "$RRBACKUP" ] || return 0
	for d in "$RRBACKUP"/*/; do
		[ -d "$d" ] || continue
		id=$(basename "$d")
		mkdir -p "$CACHE/$id"
		for f in "$d"preimage* "$d"postimage*; do
			[ -f "$f" ] && [ ! -f "$CACHE/$id/$(basename "$f")" ] && cp "$f" "$CACHE/$id/"
		done
	done
	return 0
}
note_conflicts() {
	local rr
	rr=$(git_wt rev-parse --git-path MERGE_RR)
	[ -s "$rr" ] || return 0
	mkdir -p "$RRBACKUP"
	tr '\0' '\n' <"$rr" | cut -f1 | grep . >>"$RRBACKUP/PENDING" || true
}
backup_resolutions() {
	local id dir sfx
	[ -s "$RRBACKUP/PENDING" ] || return 0
	sort -u "$RRBACKUP/PENDING" | while read -r id; do
		dir=${id%%.*}
		sfx=${id#"$dir"}
		if [ -f "$CACHE/$dir/postimage$sfx" ]; then
			mkdir -p "$RRBACKUP/$dir"
			cp "$CACHE/$dir/preimage$sfx" "$CACHE/$dir/postimage$sfx" "$RRBACKUP/$dir/"
		else
			echo "$id"
		fi
	done >"$RRBACKUP/PENDING.new"
	mv "$RRBACKUP/PENDING.new" "$RRBACKUP/PENDING"
	[ -s "$RRBACKUP/PENDING" ] || rm -f "$RRBACKUP/PENDING"
}

# pick COMMIT: cherry-pick COMMIT with its message as it is, using recorded
# resolutions for conflicts.
pick() {
	local c=$1 left
	if git_wt -c rerere.autoUpdate=true cherry-pick --allow-empty "$c" >"$WT.log" 2>&1; then
		return 0
	fi
	note_conflicts
	git_wt rev-parse -q --verify CHERRY_PICK_HEAD >/dev/null || { cat "$WT.log" >&2; die "cherry-pick of $c failed"; }
	left=$(git_wt diff --name-only --diff-filter=U)
	if [ -n "$left" ]; then
		die "unresolved conflict in $(git -C "$REPO" log -1 --format='%h %s' "$c"):
$left
Resolve it in $WT (README.md, Rebuilding), git add the files,
git commit -C $c --cleanup=verbatim, then run rebuild.sh again."
	fi
	if git_wt grep -q --cached -e '^<<<<<<< ' -e '^>>>>>>> ' -- $(git_wt diff --cached --name-only); then
		die "conflict markers left after a recorded resolution of $c"
	fi
	git_wt commit -q --cleanup=verbatim -C "$c" || die "committing the resolved cherry-pick of $c failed"
	echo "     (recorded resolution)"
}

# fork_message COMMIT: the message of a foundation commit with the fork's
# trailers: Assisted-by and Co-Authored-By lines dropped, the Signed-off-by
# of the dusk-to-dawn address replaced by the fork's, Co-Authored-By last.
fork_message() {
	git -C "$REPO" log -1 --format=%B "$1" |
		grep -v -e '^Assisted-by:' -e '^Co-Authored-By:' |
		sed "s|^$D2D_SOB\$|$FORK_SOB|" |
		sed -e ':a' -e '/^\n*$/{$d;N;ba' -e '}'
	echo "$COAUTHOR"
}

# upstream_of COMMIT: the upstream master commit a foundation prerequisite
# ports, by author, author date and subject.
upstream_of() {
	local key
	key=$(git -C "$REPO" log -1 --format='%ae %at %s' "$1")
	git -C "$REPO" log --format='%H %ae %at %s' upstream/master | grep -F -- " $key" | head -1 | cut -d' ' -f1
}

# --- checks ---------------------------------------------------------------

restore_resolutions
backup_resolutions
git -C "$REPO" rev-parse -q --verify "$BASE^{commit}" >/dev/null || die "no $BASE"
git -C "$REPO" fetch -q "$FOUNDATION_REPO" "+refs/heads/$FOUNDATION_REF:$FREF" ||
	die "cannot fetch $FOUNDATION_REF from $FOUNDATION_REPO"
git -C "$REPO" merge-base --is-ancestor "$BASE" "$FREF" || die "$FOUNDATION_REF is not on $BASE"
mapfile -t PATCHES < <(grep -v -e '^#' -e '^[[:space:]]*$' "$HERE/patches.txt")
for b in "${PATCHES[@]}"; do
	git -C "$REPO" rev-parse -q --verify "refs/heads/$b^{commit}" >/dev/null || die "no branch $b"
	git -C "$REPO" merge-base --is-ancestor "$BASE" "$b" || die "$b is not on $BASE"
done
git -C "$REPO" rev-parse -q --verify "$PREV^{commit}" >/dev/null || die "no $PREV to take the last commits from"
mapfile -t TAIL < <(git -C "$REPO" log --reverse --format='%H %s' "$BASE..$PREV" |
	awk -v g="$GOLDENS" '{h=$1; $1=""; s=substr($0,2)} s==g || s ~ /^fork:/ {print h}')
[ ${#TAIL[@]} -gt 0 ] || die "$PREV has no commits to take after the patches"
if git -C "$REPO" worktree list --porcelain | grep -qx "branch refs/heads/$BRANCH"; then
	die "$BRANCH is checked out in a worktree; check out something else there first"
fi

# --- worktree ---------------------------------------------------------------

if [ -e "$WT" ]; then
	[ -e "$WT/.git" ] || die "$WT exists and is not a worktree"
	if git_wt rev-parse -q --verify CHERRY_PICK_HEAD >/dev/null; then
		die "a cherry-pick is in progress in $WT; finish or abort it first"
	fi
	[ -z "$(git_wt status --porcelain)" ] || die "$WT has local changes"
	git_wt checkout -q --detach "$BASE"
else
	git -C "$REPO" twig new --detach "$WT" "$BASE" >/dev/null
fi
echo "building on $(git_wt log -1 --format='%h %s' HEAD) in $WT"

# --- 1. foundation ----------------------------------------------------------

msg=$WT.msg
for c in $(git -C "$REPO" rev-list --reverse "$BASE..$FREF"); do
	git_wt cherry-pick --allow-empty "$c" >"$WT.log" 2>&1 ||
		{ cat "$WT.log" >&2; die "foundation commit $c does not apply on the foundation before it"; }
	if [ "$(git -C "$REPO" log -1 --format=%an "$c")" = "Carlos Jorge" ]; then
		fork_message "$c" >"$msg"
		git_wt commit -q --amend --allow-empty --cleanup=verbatim --author="$FORK_ID" -F "$msg"
	else
		# An upstream commit: its author and message, and where it comes from.
		u=$(upstream_of "$c")
		[ -n "$u" ] || die "foundation commit $c is by $(git -C "$REPO" log -1 --format=%an "$c") but is not on upstream/master (fetch upstream)"
		[ -n "${NOTE[$u]:-}" ] || die "no port note for upstream $u; add one to NOTE in rebuild.sh"
		{ git -C "$REPO" log -1 --format=%B "$c"; printf '(cherry picked from commit %s)\n\n%s\n' "$u" "${NOTE[$u]}"; } >"$msg"
		git_wt commit -q --amend --allow-empty --cleanup=verbatim -F "$msg"
	fi
	[ "$(git_wt rev-parse HEAD^{tree})" = "$(git -C "$REPO" rev-parse "$c^{tree}")" ] ||
		die "tree differs from the foundation after $c"
	echo "found  $(git_wt log -1 --format='%h %an: %s')"
done
rm -f "$msg"
[ "$(git_wt rev-parse HEAD^{tree})" = "$(git -C "$REPO" rev-parse "$FREF^{tree}")" ] ||
	die "the foundation's tree differs from $FOUNDATION_REF"
echo "       tree $(git_wt rev-parse --short HEAD^{tree}) = $FOUNDATION_REF ($(git -C "$REPO" rev-parse --short "$FREF"))"

# --- 2. GraphOps patches, then the last commits of PREV ---------------------

for b in "${PATCHES[@]}"; do
	for c in $(git -C "$REPO" rev-list --reverse "$BASE..$b"); do
		echo "patch  $b $(git -C "$REPO" log -1 --format='%h %s' "$c")"
		pick "$c"
	done
done
for c in "${TAIL[@]}"; do
	echo "tail   $(git -C "$REPO" log -1 --format='%h %s' "$c")"
	pick "$c"
done

# --- result -----------------------------------------------------------------

backup_resolutions
head=$(git_wt rev-parse HEAD)
old=$(git -C "$REPO" rev-parse -q --verify "refs/heads/$BRANCH" || true)
git -C "$REPO" update-ref "refs/heads/$BRANCH" "$head" ${old:+"$old"}
echo
echo "$BRANCH: ${old:+${old:0:12} -> }${head:0:12}"
if [ -n "$old" ] && [ "$(git -C "$REPO" rev-parse "$old^{tree}")" = "$(git -C "$REPO" rev-parse "$head^{tree}")" ]; then
	echo "same tree as before"
fi
if [ "${KEEP_WT:-}" != 1 ]; then
	git -C "$REPO" twig remove "$WT" >/dev/null
fi
rm -f "$WT.log"
