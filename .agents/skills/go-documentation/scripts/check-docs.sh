#!/usr/bin/env bash
set -euo pipefail

VERSION="1.0.0"
SCRIPT_NAME="$(basename "$0")"

usage() {
    cat <<EOF
$SCRIPT_NAME v$VERSION — Check for missing doc comments on exported Go symbols

USAGE
    bash $SCRIPT_NAME [options] [path]

DESCRIPTION
    Scans Go source files for exported functions, types, methods, constants,
    and variables that lack doc comments. Go convention requires all exported
    symbols to have a doc comment starting with the symbol name.

    Exits 0 if all exports are documented, 1 if undocumented exports found,
    2 on error.

OPTIONS
    -h, --help       Show this help message
    -v, --version    Show version
    --json           Output results as JSON
    --strict         Also check unexported types/functions with 5+ lines
    --limit N        Show at most N results (default: all)

ARGUMENTS
    path             Directory or file to check (default: ./...)

EXAMPLES
    bash $SCRIPT_NAME
    bash $SCRIPT_NAME ./pkg/api
    bash $SCRIPT_NAME --json .
    bash $SCRIPT_NAME --strict ./internal/server
EOF
}

JSON_OUTPUT=false
STRICT=false
LIMIT=0
TARGET=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)    usage; exit 0 ;;
        -v|--version) echo "$SCRIPT_NAME v$VERSION"; exit 0 ;;
        --json)       JSON_OUTPUT=true; shift ;;
        --strict)     STRICT=true; shift ;;
        --limit)      LIMIT="${2:?error: --limit requires a number}"; shift 2 ;;
        -*)           echo "error: unknown option: $1" >&2; usage >&2; exit 2 ;;
        *)            TARGET="$1"; shift ;;
    esac
done

TARGET="${TARGET:-./...}"

json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    s="${s//$'\t'/\\t}"
    s="${s//$'\r'/}"
    s="${s//$'\n'/\\n}"
    printf '%s' "$s"
}

find_go_files() {
    local t="$1"
    if [[ -f "$t" ]]; then
        echo "$t"
    elif [[ -d "$t" ]]; then
        find "$t" -name '*.go' ! -name '*_test.go' ! -path '*/vendor/*' ! -path '*/.git/*' 2>/dev/null
    else
        local dir="${t%%/...}"
        dir="${dir:-.}"
        if [[ -d "$dir" ]]; then
            find "$dir" -name '*.go' ! -name '*_test.go' ! -path '*/vendor/*' ! -path '*/.git/*' 2>/dev/null
        else
            echo "error: path not found: $t" >&2
            exit 2
        fi
    fi
}

MISSING=()

add_missing() {
    local file="$1" line="$2" kind="$3" name="$4"
    MISSING+=("${file}:${line}|${kind}|${name}")
}

check_file() {
    local file="$1"
    local prev_line=""
    local prev_prev_line=""
    local line_num=0

    local in_grouped_block=false
    local grouped_kind=""

    local re_method='^func[[:space:]]+\([^)]+\)[[:space:]]+([A-Z][a-zA-Z0-9]*)\('
    local re_func='^func[[:space:]]+([A-Z][a-zA-Z0-9]*)\('
    local re_unexported_func='^func[[:space:]]+([a-z][a-zA-Z0-9]*)\('
    local re_grouped_open='^(const|var|type)[[:space:]]*\($'
    local re_exported_type='^type[[:space:]]+([A-Z][a-zA-Z0-9]*)[[:space:]]'
    local re_unexported_type='^type[[:space:]]+([a-z][a-zA-Z0-9]*)[[:space:]]'
    local re_exported_const='^const[[:space:]]+([A-Z][a-zA-Z0-9]*)[[:space:]]'
    local re_exported_var='^var[[:space:]]+([A-Z][a-zA-Z0-9]*)[[:space:]]'
    local re_grouped_exported='^[[:space:]]+([A-Z][a-zA-Z0-9]*)'
    local re_grouped_unexported='^[[:space:]]+([a-z][a-zA-Z0-9]*)'

    while IFS= read -r line; do
        line_num=$((line_num + 1))

        # Check exported function/method declarations
        if [[ "$line" =~ ^func[[:space:]] ]]; then
            local name=""
            local kind=""
            # Method: func (r *Type) Name(
            if [[ "$line" =~ $re_method ]]; then
                name="${BASH_REMATCH[1]}"
                kind="method"
            # Function: func Name(
            elif [[ "$line" =~ $re_func ]]; then
                name="${BASH_REMATCH[1]}"
                kind="function"
            fi

            if [[ -n "$name" ]]; then
                if ! is_documented "$prev_line" "$prev_prev_line"; then
                    add_missing "$file" "$line_num" "$kind" "$name"
                fi
            fi

            # Strict mode: also check unexported functions
            if $STRICT && [[ -z "$name" ]] && [[ "$line" =~ $re_unexported_func ]]; then
                name="${BASH_REMATCH[1]}"
                if ! is_documented "$prev_line" "$prev_prev_line"; then
                    add_missing "$file" "$line_num" "function" "$name"
                fi
            fi
        fi

        # Check exported type declarations
        if [[ "$line" =~ $re_exported_type ]]; then
            local name="${BASH_REMATCH[1]}"
            if ! is_documented "$prev_line" "$prev_prev_line"; then
                add_missing "$file" "$line_num" "type" "$name"
            fi
        fi

        # Strict mode: also check unexported type declarations
        if $STRICT && [[ "$line" =~ $re_unexported_type ]]; then
            local name="${BASH_REMATCH[1]}"
            if ! is_documented "$prev_line" "$prev_prev_line"; then
                add_missing "$file" "$line_num" "type" "$name"
            fi
        fi

        # Check exported const (single-line, not in block)
        if [[ "$line" =~ $re_exported_const ]]; then
            local name="${BASH_REMATCH[1]}"
            if ! is_documented "$prev_line" "$prev_prev_line"; then
                add_missing "$file" "$line_num" "const" "$name"
            fi
        fi

        # Check exported var (single-line, not blank identifier)
        if [[ "$line" =~ $re_exported_var ]]; then
            local name="${BASH_REMATCH[1]}"
            if ! is_documented "$prev_line" "$prev_prev_line"; then
                add_missing "$file" "$line_num" "var" "$name"
            fi
        fi

        # Check package comment
        if [[ "$line" =~ ^package[[:space:]]+ ]]; then
            if ! is_documented "$prev_line" "$prev_prev_line"; then
                local pkg_name
                pkg_name=$(echo "$line" | sed 's/^package[[:space:]]*//;s/[[:space:]]*$//')
                add_missing "$file" "$line_num" "package" "$pkg_name"
            fi
        fi

        # Track grouped declaration blocks: const ( ... ), var ( ... ), type ( ... )
        if [[ "$line" =~ $re_grouped_open ]]; then
            in_grouped_block=true
            grouped_kind="${BASH_REMATCH[1]}"
        fi
        if $in_grouped_block && [[ "$line" =~ ^\)[[:space:]]*$ ]]; then
            in_grouped_block=false
            grouped_kind=""
        fi
        if $in_grouped_block && [[ -n "$grouped_kind" ]]; then
            # Check for exported names inside grouped block
            if [[ "$line" =~ $re_grouped_exported ]]; then
                local gname="${BASH_REMATCH[1]}"
                if ! is_documented "$prev_line" "$prev_prev_line"; then
                    add_missing "$file" "$line_num" "$grouped_kind" "$gname"
                fi
            fi
            # Strict: also check unexported names in grouped blocks
            if $STRICT && [[ "$line" =~ $re_grouped_unexported ]]; then
                local gname="${BASH_REMATCH[1]}"
                if ! is_documented "$prev_line" "$prev_prev_line"; then
                    add_missing "$file" "$line_num" "$grouped_kind" "$gname"
                fi
            fi
        fi

        prev_prev_line="$prev_line"
        prev_line="$line"
    done < "$file"
}

is_documented() {
    local prev="$1"
    local prev_prev="$2"
    # Previous line is a comment (// or end of block comment */)
    if [[ "$prev" =~ ^[[:space:]]*//.* ]] || [[ "$prev" =~ \*/[[:space:]]*$ ]]; then
        return 0
    fi
    # Previous line might be empty but line before is comment (allow one blank line)
    if [[ -z "${prev// /}" ]] && [[ "$prev_prev" =~ ^[[:space:]]*//.* ]]; then
        return 0
    fi
    return 1
}

FILES=()
while IFS= read -r f; do
    [[ -n "$f" ]] && FILES+=("$f")
done < <(find_go_files "$TARGET")

if [[ ${#FILES[@]} -eq 0 ]]; then
    if $JSON_OUTPUT; then
        echo '{"missing":[],"count":0,"status":"no_go_files"}'
    else
        echo "No Go files found in: $TARGET"
    fi
    exit 0
fi

for file in "${FILES[@]}"; do
    check_file "$file"
done

# Truncation
TOTAL=${#MISSING[@]}
TRUNCATED=false
if [[ $LIMIT -gt 0 && $TOTAL -gt $LIMIT ]]; then
    MISSING=("${MISSING[@]:0:$LIMIT}")
    TRUNCATED=true
fi

if $JSON_OUTPUT; then
    echo "{"
    echo '  "missing": ['
    first=true
    for entry in "${MISSING[@]+"${MISSING[@]}"}"; do
        IFS='|' read -r location kind name <<< "$entry"
        file="${location%%:*}"
        line="${location#*:}"
        $first || echo ","
        first=false
        printf '    {"file":"%s","line":%s,"kind":"%s","name":"%s"}' \
            "$(json_escape "$file")" "$line" "$(json_escape "$kind")" "$(json_escape "$name")"
    done
    echo ""
    echo "  ],"
    printf '  "total": %d,\n' "$TOTAL"
    printf '  "truncated": %s\n' "$TRUNCATED"
    echo "}"
else
    if [[ $TOTAL -eq 0 ]]; then
        echo "All exported symbols are documented."
        exit 0
    fi

    echo "Undocumented exported symbols:"
    echo ""
    for entry in "${MISSING[@]}"; do
        IFS='|' read -r location kind name <<< "$entry"
        printf "  %s  [%s] %s\n" "$location" "$kind" "$name"
    done
    if $TRUNCATED; then
        echo "  ... and $((TOTAL - LIMIT)) more (use --limit to adjust)"
    fi
    echo ""
    echo "Total: $TOTAL undocumented symbol(s)"
fi

if [[ $TOTAL -gt 0 ]]; then
    exit 1
fi
exit 0
