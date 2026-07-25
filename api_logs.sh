#!/bin/bash
# API Log Collection and Analysis Script
# Purpose: Extract only API logs (exclude health checks) and analyze API usage
# Usage: bash api_logs.sh [command] [filter]

set -e

COMPOSE_CMD="docker compose"
API_SERVICE="api"

print_usage() {
    echo "Usage: bash api_logs.sh [command] [options]"
    echo ""
    echo "Commands:"
    echo "  stream              - Stream live API logs (real-time)"
    echo "  latest [N]          - Show last N lines (default: 100)"
    echo "  errors              - Show only error responses (4xx, 5xx)"
    echo "  slow                - Show requests slower than 1 second"
    echo "  methods [METHOD]    - Filter by HTTP method (GET, POST, etc)"
    echo "  endpoints [PATH]    - Filter by endpoint path"
    echo "  analysis            - Show summary statistics"
    echo "  export [FILE]       - Export logs to file"
    echo ""
    echo "Examples:"
    echo "  bash api_logs.sh stream"
    echo "  bash api_logs.sh latest 50"
    echo "  bash api_logs.sh errors"
    echo "  bash api_logs.sh methods POST"
    echo "  bash api_logs.sh endpoints /auth"
}

# Stream real-time logs excluding health checks
stream_logs() {
    echo "Streaming API logs (excluding /health)..."
    echo "Press Ctrl+C to stop"
    echo ""
    $COMPOSE_CMD logs -f $API_SERVICE | grep -v "/health"
}

# Show latest N lines
latest_logs() {
    local lines=${1:-100}
    echo "Last $lines API logs (excluding /health):"
    echo ""
    $COMPOSE_CMD logs --tail=$lines $API_SERVICE | grep -v "/health"
}

# Show only error responses
show_errors() {
    echo "API Error Responses (4xx, 5xx status codes):"
    echo ""
    $COMPOSE_CMD logs --tail=1000 $API_SERVICE | grep -E "40[0-9]|50[0-9]" | grep -v "/health"
}

# Show slow requests (> 1 second)
show_slow() {
    echo "Slow Requests (> 1000ms):"
    echo ""
    $COMPOSE_CMD logs --tail=500 $API_SERVICE | grep -v "/health" | awk '
        {
            # Fiber log format: [timestamp] | status | method path | duration
            # Example: 2026-05-27 14:32:15 | 200 | POST /api/v1/auth/send-otp | 234ms
            if ($0 ~ /ms/) {
                # Extract duration in ms
                match($0, /([0-9]+)ms/, arr)
                duration = arr[1]
                if (duration > 1000) {
                    print $0
                }
            }
        }
    '
}

# Filter by HTTP method
filter_by_method() {
    local method=$1
    if [ -z "$method" ]; then
        echo "Available methods: GET, POST, PATCH, DELETE, PUT, OPTIONS"
        return 1
    fi
    
    echo "API Requests with method: $method"
    echo ""
    $COMPOSE_CMD logs --tail=500 $API_SERVICE | grep " $method " | grep -v "/health"
}

# Filter by endpoint path
filter_by_endpoint() {
    local path=$1
    if [ -z "$path" ]; then
        echo "Provide endpoint path to filter (e.g., /auth, /videos)"
        return 1
    fi
    
    echo "API Requests to endpoint containing: $path"
    echo ""
    $COMPOSE_CMD logs --tail=500 $API_SERVICE | grep "$path" | grep -v "/health"
}

# Show log statistics and analysis
analyze_logs() {
    echo "API Log Analysis (Last 1000 logs, excluding /health):"
    echo ""
    
    local logs=$($COMPOSE_CMD logs --tail=1000 $API_SERVICE | grep -v "/health")
    
    echo "=== Request Methods ==="
    echo "$logs" | grep -oE "\| (GET|POST|PATCH|DELETE|PUT|OPTIONS) " | sort | uniq -c
    echo ""
    
    echo "=== Response Codes ==="
    echo "$logs" | grep -oE "^\s*[0-9]{3}" | sort | uniq -c
    echo ""
    
    echo "=== Top 10 Endpoints ==="
    echo "$logs" | grep -oE "/api/v[0-9]/[^ ]+" | sort | uniq -c | sort -rn | head -10
    echo ""
    
    echo "=== Error Summary ==="
    echo "4xx Errors:"
    echo "$logs" | grep -cE "40[0-9]" || echo "0"
    echo "5xx Errors:"
    echo "$logs" | grep -cE "50[0-9]" || echo "0"
    echo ""
    
    local total=$(echo "$logs" | wc -l)
    echo "Total API calls (last 1000): $total"
}

# Export logs to file
export_logs() {
    local file=${1:-api_logs_$(date +%Y%m%d_%H%M%S).txt}
    
    echo "Exporting API logs to: $file"
    $COMPOSE_CMD logs --tail=1000 $API_SERVICE | grep -v "/health" > "$file"
    echo "✓ Exported $(wc -l < "$file") lines"
    echo "To analyze: cat $file | grep -E 'error|ERROR|Exception'"
}

# Main script
main() {
    if [ $# -eq 0 ]; then
        print_usage
        exit 0
    fi
    
    case "$1" in
        stream)
            stream_logs
            ;;
        latest)
            latest_logs "$2"
            ;;
        errors)
            show_errors
            ;;
        slow)
            show_slow
            ;;
        methods)
            filter_by_method "$2"
            ;;
        endpoints)
            filter_by_endpoint "$2"
            ;;
        analysis)
            analyze_logs
            ;;
        export)
            export_logs "$2"
            ;;
        help|-h|--help)
            print_usage
            ;;
        *)
            echo "Unknown command: $1"
            echo ""
            print_usage
            exit 1
            ;;
    esac
}

main "$@"
