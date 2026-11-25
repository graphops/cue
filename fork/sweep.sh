#!/bin/bash
# Render blueprint repository blueprint scenarios with two or more cue binaries and
# compare their output, wall time and peak memory.
#
#   IK=<blueprint repository checkout> BINS="new=/path/to/cue old=/path/to/cue" \
#   OUT=<results dir> bash fork/sweep.sh [SCENARIO...]
#
# A scenario is a blueprint directory under blueprints/{charts,raw,stacks}
# with a cue.mod, relative to IK, optionally followed by ":" and an overlay
# file in its testdata directory:
#
#   <kind>/<blueprint>                        cue export -t mockContext=true . -e out --out json
#   <kind>/<blueprint>:<overlay>.cue          cue export -t mockContext=true . testdata/<overlay>.cue -e out --out json
#
# Without arguments it renders every scenario: each blueprint without an
# overlay, and with each testdata/*.cue file in turn. SCENARIOS=<file> reads
# the list from a file, one scenario per line. LIST_SCENARIOS=1 prints the
# list and renders nothing.
#
# Rules, as for every heavy command on this host:
#   - one render at a time; the lock ($LOCK) is held for one render only;
#   - each render runs in its own systemd scope with MemoryMax=$MEM (4G) and
#     no swap;
#   - the sweep stops at the first render killed by a signal (an exit status
#     of 128 or more, such as an OOM kill) and reports it;
#   - more than 5 scenarios need APPROVED=1 (the owner's approval).
#
# Environment:
#   IK        blueprint repository checkout (required); its .envrc (direnv) gives
#             the CUE registry configuration. ENV_FROM names another checkout
#             to take it from, for a worktree direnv has not been allowed in.
#   BINS      NAME=PATH pairs, separated by spaces (required, at least one)
#   OUT       results directory (default: ./sweep-<date>); it holds
#             results.tsv (one line per render), summary.txt, and for every
#             scenario whose outputs differ, the outputs (diff/<scenario>/<NAME>.json.zst);
#             the stderr of every failing render is in fail/<scenario>/<NAME>.stderr
#   MEM       memory cap per render (default 4G)
#   TIMEOUT   seconds per render (default 600); a timeout is recorded, not a stop
#   LOCK      lock file (default /home/cjw.graphops/.claude/jobs/10b601ab/tmp/heavy.lock)
#   APPROVED  1 to allow more than 5 scenarios
#   RESUME=1  keep results.tsv and skip the scenarios it has for every binary
#
# Rendered outputs and the scenario list are GraphOps configuration: keep OUT
# outside this public repository, and record only aggregate numbers here.
set -uo pipefail

: "${IK:?IK must name an blueprint repository checkout}"
: "${BINS:?BINS must list NAME=PATH pairs}"
OUT=${OUT:-$PWD/sweep-$(date +%Y-%m-%d)}
MEM=${MEM:-4G}
TIMEOUT=${TIMEOUT:-600}
LOCK=${LOCK:-/home/cjw.graphops/.claude/jobs/10b601ab/tmp/heavy.lock}
ENV_FROM=${ENV_FROM:-$IK}

die() { printf 'sweep.sh: %s\n' "$*" >&2; exit 1; }

IK=$(cd "$IK" && pwd) || die "no $IK"
[ -d "$IK/blueprints" ] || die "$IK has no blueprints directory"

declare -a NAMES PATHS
for p in $BINS; do
	n=${p%%=*} b=${p#*=}
	[ -x "$b" ] || die "binary $b ($n) is not executable"
	NAMES+=("$n")
	PATHS+=("$(readlink -f -- "$b")")
done
[ ${#NAMES[@]} -ge 1 ] || die "BINS is empty"

# --- scenarios ----------------------------------------------------------------

all_scenarios() {
	local d f
	(cd "$IK/blueprints" && for d in charts/*/ raw/*/ stacks/*/; do
		d=${d%/}
		[ -f "$d/cue.mod/module.cue" ] || continue
		echo "$d"
		for f in "$d"/testdata/*.cue; do
			[ -f "$f" ] && echo "$d:${f##*/}"
		done
	done)
}
if [ -n "${SCENARIOS:-}" ]; then
	mapfile -t LIST < <(grep -v -e '^#' -e '^[[:space:]]*$' "$SCENARIOS")
elif [ $# -gt 0 ]; then
	LIST=("$@")
else
	mapfile -t LIST < <(all_scenarios)
fi
if [ "${LIST_SCENARIOS:-}" = 1 ]; then printf '%s\n' "${LIST[@]}"; exit 0; fi
[ ${#LIST[@]} -gt 0 ] || die "no scenarios"
if [ ${#LIST[@]} -gt 5 ] && [ "${APPROVED:-}" != 1 ]; then
	die "${#LIST[@]} scenarios: more than 5 need the owner's approval (APPROVED=1)"
fi

# --- environment --------------------------------------------------------------

command -v direnv >/dev/null || die "direnv is needed for the CUE registry configuration"
eval "$(cd "$ENV_FROM" && direnv export bash 2>/dev/null)"
[ -n "${CUE_REGISTRY:-}" ] || die "no CUE_REGISTRY from $ENV_FROM (direnv allow there?)"
export CUE_REGISTRY CUE_CACHE_DIR

mkdir -p "$OUT"
RES=$OUT/results.tsv
[ -f "$RES" ] && [ "${RESUME:-}" = 1 ] || printf 'scenario\tbinary\texit\twall_s\tpeak_rss_kib\tsha256\tbytes\n' >"$RES"
TMP=$(mktemp -d "$OUT/.tmp.XXXX")
trap 'rm -rf "$TMP"' EXIT

# render SCENARIO INDEX: one capped, locked render; prints the result line.
render() {
	local s=$1 i=$2 dir ov args rc wall rss sum bytes
	dir=${s%%:*}
	ov=
	[ "$s" != "$dir" ] && ov=testdata/${s#*:}
	args=(export -t mockContext=true .)
	[ -n "$ov" ] && args+=("$ov")
	args+=(-e out --out json)
	(cd "$IK/blueprints/$dir" &&
		flock "$LOCK" systemd-run --user --scope -q -p MemoryMax="$MEM" -p MemorySwapMax=0 -- \
			/usr/bin/time -f '%e %M' -o "$TMP/time" \
			timeout -s KILL "$TIMEOUT" "${PATHS[$i]}" "${args[@]}" >"$TMP/out.$i" 2>"$TMP/err.$i")
	rc=$?
	read -r wall rss < <(tail -1 "$TMP/time" 2>/dev/null || echo "- -")
	sum=$(sha256sum <"$TMP/out.$i" | cut -c1-64)
	bytes=$(stat -c %s "$TMP/out.$i")
	printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$s" "${NAMES[$i]}" "$rc" "$wall" "$rss" "$sum" "$bytes" | tee -a "$RES"
	if [ "$rc" != 0 ]; then
		mkdir -p "$OUT/fail/${s//[\/:]/_}"
		cp "$TMP/err.$i" "$OUT/fail/${s//[\/:]/_}/${NAMES[$i]}.stderr"
	fi
	return $rc
}

n=0
for s in "${LIST[@]}"; do
	n=$((n + 1))
	printf '# [%d/%d] %s\n' "$n" "${#LIST[@]}" "$s" >&2
	if [ "${RESUME:-}" = 1 ] &&
		[ "$(awk -F'\t' -v s="$s" '$1==s' "$RES" | wc -l)" -ge ${#NAMES[@]} ]; then
		continue
	fi
	SUMS=()
	for i in "${!NAMES[@]}"; do
		render "$s" "$i"
		rc=$?
		if [ $rc -ge 128 ]; then
			# 137 is a SIGKILL: the memory cap (OOM) or the timeout.
			why="killed (exit $rc)"
			wall=$(tail -1 "$TMP/time" 2>/dev/null | cut -d' ' -f1)
			if [ "${wall%.*}" -ge "$TIMEOUT" ] 2>/dev/null; then
				echo "# $s with ${NAMES[$i]}: timeout after ${TIMEOUT}s, continuing" >&2
				continue
			fi
			sed -n '1,20p' "$TMP/err.$i" >&2
			die "$s with ${NAMES[$i]}: $why; stopping at the first kill (see $RES)"
		fi
		SUMS[$i]=$(tail -1 "$RES" | cut -f3,6)
	done
	# Keep the outputs of every scenario whose exit status or output differs.
	if [ "$(printf '%s\n' "${SUMS[@]}" | sort -u | wc -l)" -gt 1 ]; then
		d=$OUT/diff/${s//[\/:]/_}
		mkdir -p "$d"
		for i in "${!NAMES[@]}"; do
			zstd -q -f "$TMP/out.$i" -o "$d/${NAMES[$i]}.json.zst"
			cp "$TMP/err.$i" "$d/${NAMES[$i]}.stderr"
		done
	fi
done

# --- summary ------------------------------------------------------------------

awk -F'\t' -v names="${NAMES[*]}" '
NR == 1 { next }
{
	key = $1; b = $2
	rc[key, b] = $3; sum[key, b] = $6; wall[b] += $4; if ($5 > peak[b]) { peak[b] = $5; peaks[b] = key }
	rss[b] += $5; cnt[b]++; if (!(key in seen)) { seen[key] = 1; order[++ns] = key }
	if ($3 != 0) fail[b]++
}
END {
	split(names, N, " ")
	same = 0; differ = 0; status = 0
	for (k = 1; k <= ns; k++) {
		s = order[k]; ref = N[1]; d = 0; st = 0
		for (j = 2; j <= length(N); j++) {
			if (rc[s, N[j]] != rc[s, ref]) st = 1
			else if (sum[s, N[j]] != sum[s, ref]) d = 1
		}
		if (st) status++; else if (d) differ++; else same++
	}
	printf "scenarios: %d\nidentical (exit status and output): %d\noutput differs, same exit status: %d\nexit status differs: %d\n", ns, same, differ, status
	for (j = 1; j <= length(N); j++) {
		b = N[j]
		printf "%s: %d renders, %d failing, total wall %.1f s, mean peak RSS %.0f MiB, max peak RSS %.0f MiB (%s)\n", \
			b, cnt[b], fail[b], wall[b], rss[b] / cnt[b] / 1024, peak[b] / 1024, peaks[b]
	}
}' "$RES" | tee "$OUT/summary.txt"
