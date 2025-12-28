#!/usr/bin/env bash
# =============================================================================
# E2E Test Report Generator
# =============================================================================
# Automatically generates TEST_RESULTS.md from E2E test execution
# =============================================================================

set -e

# Input: test output log file
TEST_LOG="${1:-/tmp/e2e-test-output.log}"
OUTPUT_FILE="${2:-tests/e2e/TEST_RESULTS.md}"

# Check if test log exists
if [ ! -f "$TEST_LOG" ]; then
    echo "Error: Test log not found: $TEST_LOG"
    echo "Usage: $0 <test-log-file> [output-file]"
    exit 1
fi

# =============================================================================
# Parse Test Results
# =============================================================================

# Count test results
TOTAL_TESTS=$(grep -c "^=== RUN" "$TEST_LOG" || echo 0)
PASSED=$(grep -c "^--- PASS:" "$TEST_LOG" || echo 0)
FAILED=$(grep -c "^--- FAIL:" "$TEST_LOG" || echo 0)
SKIPPED=$(grep -c "^--- SKIP:" "$TEST_LOG" || echo 0)

# Calculate percentages
if [ "$TOTAL_TESTS" -gt 0 ]; then
    PASS_PCT=$(awk "BEGIN {printf \"%.1f\", ($PASSED / $TOTAL_TESTS) * 100}")
    FAIL_PCT=$(awk "BEGIN {printf \"%.1f\", ($FAILED / $TOTAL_TESTS) * 100}")
    SKIP_PCT=$(awk "BEGIN {printf \"%.1f\", ($SKIPPED / $TOTAL_TESTS) * 100}")
else
    PASS_PCT="0.0"
    FAIL_PCT="0.0"
    SKIP_PCT="0.0"
fi

# Get binary version
BINARY_VERSION=$(./devnet-builder version 2>&1 | head -1 || echo "unknown")

# Get execution time from final FAIL/PASS line
EXEC_TIME=$(grep "^FAIL\|^PASS" "$TEST_LOG" | tail -1 | awk '{print $NF}' || echo "unknown")

# Current date
DATE=$(date +"%Y-%m-%d %H:%M:%S")

# =============================================================================
# Generate Report
# =============================================================================

cat > "$OUTPUT_FILE" <<EOF
# E2E Test Suite - Execution Report

**Date**: $DATE
**Binary**: $BINARY_VERSION
**Total Execution Time**: $EXEC_TIME
**Environment**: $(uname -s) ($(uname -m))

## Executive Summary

| Metric | Value | Percentage |
|--------|-------|------------|
| Total Tests | $TOTAL_TESTS | 100% |
| ✅ Passed | $PASSED | $PASS_PCT% |
| ❌ Failed | $FAILED | $FAIL_PCT% |
| ⏭️ Skipped | $SKIPPED | $SKIP_PCT% |

EOF

# =============================================================================
# Passed Tests Section
# =============================================================================

if [ "$PASSED" -gt 0 ]; then
    cat >> "$OUTPUT_FILE" <<EOF
## ✅ Passing Tests ($PASSED)

EOF

    grep "^--- PASS:" "$TEST_LOG" | sed 's/^--- PASS: //' | sort | while read -r line; do
        test_name=$(echo "$line" | awk '{print $1}')
        duration=$(echo "$line" | grep -o '([0-9.]*s)' || echo '')
        echo "- ✅ \`$test_name\` $duration" >> "$OUTPUT_FILE"
    done

    echo "" >> "$OUTPUT_FILE"
fi

# =============================================================================
# Failed Tests Section
# =============================================================================

if [ "$FAILED" -gt 0 ]; then
    cat >> "$OUTPUT_FILE" <<EOF
## ❌ Failed Tests ($FAILED)

EOF

    grep "^--- FAIL:" "$TEST_LOG" | sed 's/^--- FAIL: //' | sort | while read -r line; do
        test_name=$(echo "$line" | awk '{print $1}')
        duration=$(echo "$line" | grep -o '([0-9.]*s)' || echo '')

        # Try to extract error message for this test
        error_msg=$(awk "/^=== RUN.*$test_name$/,/^--- (PASS|FAIL|SKIP):/" "$TEST_LOG" | \
                    grep "Error:" | head -1 | sed 's/^[[:space:]]*//' || echo "")

        if [ -n "$error_msg" ]; then
            echo "- ❌ \`$test_name\` $duration" >> "$OUTPUT_FILE"
            echo "  - $error_msg" >> "$OUTPUT_FILE"
        else
            echo "- ❌ \`$test_name\` $duration" >> "$OUTPUT_FILE"
        fi
    done

    echo "" >> "$OUTPUT_FILE"
fi

# =============================================================================
# Skipped Tests Section
# =============================================================================

if [ "$SKIPPED" -gt 0 ]; then
    cat >> "$OUTPUT_FILE" <<EOF
## ⏭️ Skipped Tests ($SKIPPED)

EOF

    grep "^--- SKIP:" "$TEST_LOG" | sed 's/^--- SKIP: //' | sort | while read -r line; do
        test_name=$(echo "$line" | awk '{print $1}')

        # Try to extract skip reason
        skip_reason=$(awk "/^=== RUN.*$test_name$/,/^--- (PASS|FAIL|SKIP):/" "$TEST_LOG" | \
                      grep -E "Blockchain binary|Docker" | head -1 | sed 's/^[[:space:]]*//' || echo "")

        if [ -n "$skip_reason" ]; then
            echo "- ⏭️ \`$test_name\`" >> "$OUTPUT_FILE"
            echo "  - Reason: $skip_reason" >> "$OUTPUT_FILE"
        else
            echo "- ⏭️ \`$test_name\`" >> "$OUTPUT_FILE"
        fi
    done

    echo "" >> "$OUTPUT_FILE"
fi

# =============================================================================
# Test Categories Breakdown
# =============================================================================

cat >> "$OUTPUT_FILE" <<EOF
## Test Coverage by Category

EOF

# Extract test categories from test names
declare -A categories

while IFS= read -r line; do
    test_name=$(echo "$line" | sed 's/^--- [A-Z]*: //' | awk '{print $1}')
    category=$(echo "$test_name" | cut -d'_' -f1)

    status="unknown"
    if echo "$line" | grep -q "^--- PASS:"; then
        status="passed"
    elif echo "$line" | grep -q "^--- FAIL:"; then
        status="failed"
    elif echo "$line" | grep -q "^--- SKIP:"; then
        status="skipped"
    fi

    # Count by category
    if [ -z "${categories[$category]}" ]; then
        categories[$category]="0:0:0"  # passed:failed:skipped
    fi

    IFS=':' read -r p f s <<< "${categories[$category]}"

    case "$status" in
        passed)  p=$((p + 1)) ;;
        failed)  f=$((f + 1)) ;;
        skipped) s=$((s + 1)) ;;
    esac

    categories[$category]="$p:$f:$s"
done < <(grep "^--- " "$TEST_LOG")

# Print category breakdown
echo "| Category | ✅ Passed | ❌ Failed | ⏭️ Skipped | Total |" >> "$OUTPUT_FILE"
echo "|----------|-----------|-----------|------------|-------|" >> "$OUTPUT_FILE"

for category in $(echo "${!categories[@]}" | tr ' ' '\n' | sort); do
    IFS=':' read -r p f s <<< "${categories[$category]}"
    total=$((p + f + s))
    echo "| $category | $p | $f | $s | $total |" >> "$OUTPUT_FILE"
done

echo "" >> "$OUTPUT_FILE"

# =============================================================================
# Footer
# =============================================================================

cat >> "$OUTPUT_FILE" <<EOF
---

**Report generated**: $(date +"%Y-%m-%d %H:%M:%S")
**Test log**: \`$TEST_LOG\`
**Generated by**: \`tests/e2e/scripts/generate-test-report.sh\`

> This report is automatically generated. Do not edit manually.
EOF

echo "Test report generated: $OUTPUT_FILE"
echo ""
echo "Summary:"
echo "  Total:   $TOTAL_TESTS tests"
echo "  Passed:  $PASSED ($PASS_PCT%)"
echo "  Failed:  $FAILED ($FAIL_PCT%)"
echo "  Skipped: $SKIPPED ($SKIP_PCT%)"
