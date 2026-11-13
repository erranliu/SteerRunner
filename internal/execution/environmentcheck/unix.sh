set -u -o pipefail
prefix='SLENV-@@ID@@-'
root=@@ROOT@@
remote=@@REMOTE@@
url=@@URL@@
script=@@SCRIPT@@
manifest=@@MANIFEST@@
scratch=
failure='environment preparation failed'
finish() {
    code=$?
    trap - EXIT
    if [ -n "$scratch" ] && ! rm -rf -- "$scratch"; then
        code=125
        failure='temporary directory cleanup failed'
    fi
    printf '%send:%s:' "$prefix" "$code"
    printf '%s' "$failure" | base64 | tr -d '\r\n'
    printf '\n'
    if [ @@SETTLE@@ -gt 0 ]; then sleep @@SETTLE@@; fi
    exit "$code"
}
trap finish EXIT
hash() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum; else shasum -a 256; fi | cut -d ' ' -f 1
}
hash_material() {
    local direct
    direct=$(hash < "$1") || return 1
    [ "$direct" = "$2" ] || [ "$(LC_ALL=C sed $'s/\r$//' "$1" | hash)" = "$2" ]
}
ordinary() {
    [ -f "$1" ] && [ "$(wc -c < "$1")" -le 1048576 ] || return 1
    local component="$1"
    while [ "$component" != "." ]; do
        [ ! -L "$component" ] || return 1
        case "$component" in */*) component=${component%/*} ;; *) component=. ;; esac
    done
}
verify() {
    failure='project checkout identity differs'
    [ "$(pwd -P)" = "$root" ] && [ "$(git rev-parse --show-toplevel 2>/dev/null)" = "$root" ] || return 1
    failure='project checkout remote differs'
    local actual
    actual=$(git config --get "remote.$remote.url" 2>/dev/null) || return 1
    if [ "$actual" != "$url" ]; then
        case "$actual" in http://*|https://*) ;; *) return 1 ;; esac
        case "$url" in http://*|https://*) ;; *) return 1 ;; esac
        [ "${actual%.git}" = "${url%.git}" ] || return 1
    fi
    failure='environment manifest differs'
    ordinary SteerlineFlow/environment.md || return 1
    local digest
    digest=$(awk '
        { sub(/\r$/, ""); if (index($0, "@@FENCE@@steerline-environment")) n++ }
        /^@@FENCE@@steerline-environment$/ { active=1; next }
        /^@@FENCE@@$/ && active { active=0; closed=1; next }
        active { if (line++) printf "\n"; printf "%s", $0 }
        END { if (n != 1 || !closed || active) exit 1 }
    ' SteerlineFlow/environment.md | hash) || return 1
    [ "$digest" = "$manifest" ] || return 1
@@CHECKS@@
}
verify || exit 125
failure='temporary directory is unavailable or inside the project'
temp_root=$(cd -- "${TMPDIR:-/tmp}" && pwd -P) || exit 125
case "$temp_root/" in "$root/"*) exit 125 ;; esac
scratch=$(mktemp -d "$temp_root/steerrunner-env.XXXXXXXXXX") || exit 125
export STEERLINE_RESULT_FILE="$scratch/result.json" TMPDIR="$scratch" TMP="$scratch" TEMP="$scratch"
failure='result initialization failed'
if [ "$(uname -s)" = Darwin ]; then decode=(-D); else decode=(-d); fi
printf '%s' @@INITIAL@@ | base64 "${decode[@]}" > "$STEERLINE_RESULT_FILE" || exit 125
bash "$script" >/dev/null 2>&1
code=$?
verify || exit 125
failure='result file is missing, redirected or exceeds bounds'
[ -f "$STEERLINE_RESULT_FILE" ] && [ ! -L "$STEERLINE_RESULT_FILE" ] && [ "$(wc -c < "$STEERLINE_RESULT_FILE")" -le 65536 ] || exit 125
base64 < "$STEERLINE_RESULT_FILE" | tr -d '\r\n' | fold -w 1024 | awk -v prefix="$prefix" '{print prefix "data:" $0}' || exit 125
failure=
exit "$code"
