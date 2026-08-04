#!/usr/bin/env bash
set -euo pipefail

VERSION="1.0.0"
SCRIPT_NAME="$(basename "$0")"

usage() {
    cat <<EOF
$SCRIPT_NAME v$VERSION — Run automated pre-review checks on Go code

USAGE
    bash $SCRIPT_NAME [options] [path]

DESCRIPTION
    Runs gofmt, go vet, and golangci-lint against the target path and
    reports any findings. Use before manual code review to catch
    mechanical issues early.

    Exits 0 if all checks pass, 1 if issues found, 2 on error.

OPTIONS
    -h, --help       Show this help message
    -v, --version    Show version
    --json           Output results as JSON
    --force          Run even if golangci-lint is not installed (skip it)
    --limit N        Max items reported per section (0 = unlimited, default: 0)

ARGUMENTS
    path             Package pattern to check (default: ./...)

EXAMPLES
    bash $SCRIPT_NAME
    bash $SCRIPT_NAME ./pkg/...
    bash $SCRIPT_NAME --json ./cmd/server/...
    bash $SCRIPT_NAME --force ./...
    bash $SCRIPT_NAME --json --limit 10 ./...
EOF
}

json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\t'/\\t}"
    s="${s//$'\r'/}"
    s="${s//$'\n'/\\n}"
    printf '%s' "$s"
}

JSON_OUTPUT=false
FORCE=false
LIMIT=0
TARGET=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)    usage; exit 0 ;;
        -v|--version) echo "$SCRIPT_NAME v$VERSION"; exit 0 ;;
        --json)       JSON_OUTPUT=true; shift ;;
        --force)      FORCE=true; shift ;;
        --limit)      LIMIT="${2:?error: --limit requires a number}"; shift 2 ;;
        -*)           echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
        *)            TARGET="$1"; shift ;;
    esac
done

TARGET="${TARGET:-./...}"

if ! command -v go &>/dev/null; then
    echo "error: go is not installed or not in PATH" >&2
    exit 2
fi

if ! command -v gofmt &>/dev/null; then
    echo "error: gofmt is not installed or not in PATH" >&2
    exit 2
fi

GOFMT_STATUS="pass"
GOFMT_FINDINGS=()
GOFMT_DIR="${TARGET%%/...}"
GOFMT_DIR="${GOFMT_DIR:-.}"
UNFORMATTED=$(gofmt -l "$GOFMT_DIR" 2>&1) || true
if [[ -n "$UNFORMATTED" ]]; then
    GOFMT_STATUS="fail"
    while IFS= read -r f; do
        [[ -n "$f" ]] && GOFMT_FINDINGS+=("$f")
    done <<< "$UNFORMATTED"
fi

GOVET_STATUS="pass"
GOVET_OUTPUT=""
if ! GOVET_OUTPUT=$(go vet "$TARGET" 2>&1); then
    GOVET_STATUS="fail"
fi

LINT_STATUS="skip"
LINT_OUTPUT=""
if command -v golangci-lint &>/dev/null; then
    LINT_STATUS="pass"
    if ! LINT_OUTPUT=$(golangci-lint run "$TARGET" 2>&1); then
        LINT_STATUS="fail"
    fi
elif ! $FORCE; then
    echo "error: golangci-lint not installed (use --force to skip)" >&2
    exit 2
fi

FAILED=0
[[ "$GOFMT_STATUS" == "fail" ]] && FAILED=1
[[ "$GOVET_STATUS" == "fail" ]] && FAILED=1
[[ "$LINT_STATUS" == "fail" ]] && FAILED=1

if $JSON_OUTPUT; then
    GOFMT_TRUNCATED=false
    GOFMT_DISPLAY=("${GOFMT_FINDINGS[@]+"${GOFMT_FINDINGS[@]}"}")
    if [[ $LIMIT -gt 0 && ${#GOFMT_DISPLAY[@]} -gt $LIMIT ]]; then
        GOFMT_DISPLAY=("${GOFMT_FINDINGS[@]:0:$LIMIT}")
        GOFMT_TRUNCATED=true
    fi

    GOFMT_JSON="["
    first=true
    for f in "${GOFMT_DISPLAY[@]+"${GOFMT_DISPLAY[@]}"}"; do
        $first || GOFMT_JSON+=","
        first=false
        GOFMT_JSON+="\"$(json_escape "$f")\""
    done
    GOFMT_JSON+="]"

    GOVET_TRUNCATED=false
    GOVET_DISPLAY="$GOVET_OUTPUT"
    if [[ $LIMIT -gt 0 && -n "$GOVET_OUTPUT" ]]; then
        GOVET_ARR=()
        while IFS= read -r line; do
            GOVET_ARR+=("$line")
        done <<< "$GOVET_OUTPUT"
        if [[ ${#GOVET_ARR[@]} -gt $LIMIT ]]; then
            GOVET_DISPLAY=""
            for (( i=0; i<LIMIT; i++ )); do
                [[ -n "$GOVET_DISPLAY" ]] && GOVET_DISPLAY+=$'\n'
                GOVET_DISPLAY+="${GOVET_ARR[$i]}"
            done
            GOVET_TRUNCATED=true
        fi
    fi
    GOVET_ESC="$(json_escape "$GOVET_DISPLAY")"

    LINT_TRUNCATED=false
    LINT_DISPLAY="$LINT_OUTPUT"
    if [[ $LIMIT -gt 0 && -n "$LINT_OUTPUT" ]]; then
        LINT_ARR=()
        while IFS= read -r line; do
            LINT_ARR+=("$line")
        done <<< "$LINT_OUTPUT"
        if [[ ${#LINT_ARR[@]} -gt $LIMIT ]]; then
            LINT_DISPLAY=""
            for (( i=0; i<LIMIT; i++ )); do
                [[ -n "$LINT_DISPLAY" ]] && LINT_DISPLAY+=$'\n'
                LINT_DISPLAY+="${LINT_ARR[$i]}"
            done
            LINT_TRUNCATED=true
        fi
    fi
    LINT_ESC="$(json_escape "$LINT_DISPLAY")"

    GOFMT_TRUNC=""
    $GOFMT_TRUNCATED && GOFMT_TRUNC=',"truncated":true'
    GOVET_TRUNC=""
    $GOVET_TRUNCATED && GOVET_TRUNC=',"truncated":true'
    LINT_TRUNC=""
    $LINT_TRUNCATED && LINT_TRUNC=',"truncated":true'

    cat <<EOF
{"gofmt":{"status":"$GOFMT_STATUS","files":$GOFMT_JSON$GOFMT_TRUNC},"govet":{"status":"$GOVET_STATUS","output":"$GOVET_ESC"$GOVET_TRUNC},"golangci_lint":{"status":"$LINT_STATUS","output":"$LINT_ESC"$LINT_TRUNC},"passed":$( [[ $FAILED -eq 0 ]] && echo true || echo false )}
EOF
else
    echo "=== gofmt ==="
    if [[ "$GOFMT_STATUS" == "fail" ]]; then
        echo "Unformatted files:"
        GOFMT_COUNT=0
        for f in "${GOFMT_FINDINGS[@]}"; do
            GOFMT_COUNT=$((GOFMT_COUNT + 1))
            if [[ $LIMIT -gt 0 && $GOFMT_COUNT -gt $LIMIT ]]; then
                echo "  ... ($(( ${#GOFMT_FINDINGS[@]} - LIMIT )) more items truncated)"
                break
            fi
            echo "  $f"
        done
    else
        echo "OK"
    fi

    echo ""
    echo "=== go vet ==="
    if [[ "$GOVET_STATUS" == "fail" ]]; then
        if [[ $LIMIT -gt 0 ]]; then
            GOVET_ARR=()
            while IFS= read -r line; do
                GOVET_ARR+=("$line")
            done <<< "$GOVET_OUTPUT"
            for (( i=0; i<${#GOVET_ARR[@]} && i<LIMIT; i++ )); do
                echo "${GOVET_ARR[$i]}"
            done
            if [[ ${#GOVET_ARR[@]} -gt $LIMIT ]]; then
                echo "... ($(( ${#GOVET_ARR[@]} - LIMIT )) more items truncated)"
            fi
        else
            echo "$GOVET_OUTPUT"
        fi
    else
        echo "OK"
    fi

    echo ""
    echo "=== golangci-lint ==="
    if [[ "$LINT_STATUS" == "skip" ]]; then
        echo "Skipped (not installed)"
    elif [[ "$LINT_STATUS" == "fail" ]]; then
        if [[ $LIMIT -gt 0 ]]; then
            LINT_ARR=()
            while IFS= read -r line; do
                LINT_ARR+=("$line")
            done <<< "$LINT_OUTPUT"
            for (( i=0; i<${#LINT_ARR[@]} && i<LIMIT; i++ )); do
                echo "${LINT_ARR[$i]}"
            done
            if [[ ${#LINT_ARR[@]} -gt $LIMIT ]]; then
                echo "... ($(( ${#LINT_ARR[@]} - LIMIT )) more items truncated)"
            fi
        else
            echo "$LINT_OUTPUT"
        fi
    else
        echo "OK"
    fi

    echo ""
    if [[ $FAILED -eq 1 ]]; then
        echo "Pre-review checks FAILED — fix issues before manual review."
    else
        echo "All pre-review checks passed."
    fi
fi

exit $FAILED
