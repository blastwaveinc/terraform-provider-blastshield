#!/bin/bash
# Cleanup script for test resources that match name patterns
# Useful for cleaning up resources from old tests that don't have tags

# Don't exit on error - we want to try all resource types even if one fails
set +e

BLASTSHIELD_HOST="${BLASTSHIELD_HOST:-http://localhost:5000}"
BLASTSHIELD_TOKEN="${BLASTSHIELD_TOKEN:-dev}"

# Debug mode
if [ "$1" = "--debug" ] || [ "$2" = "--debug" ]; then
    DEBUG=true
else
    DEBUG=false
fi

if [ "$1" = "--dryrun" ] || [ "$2" = "--dryrun" ]; then
    DRYRUN=true
    echo "DRY RUN - Showing test resources that would be cleaned up"
else
    DRYRUN=false
    echo " WARNING: This will DELETE resources matching 'test-*' pattern!"
    read -p "Continue? [y/N] " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Cancelled."
        exit 1
    fi
fi

echo "Using API: $BLASTSHIELD_HOST"
echo ""

# Function to cleanup resources of a specific type
cleanup_resource() {
    local resource_type=$1
    local endpoint=$2

    echo "Checking ${resource_type}..."

    if [ "$DEBUG" = true ]; then
        echo "  [DEBUG] Endpoint: ${BLASTSHIELD_HOST}${endpoint}"
        echo "  [DEBUG] Token: ${BLASTSHIELD_TOKEN:0:20}..."
    fi

    # Get all resources and filter by name pattern
    response=$(curl -s -X GET "${BLASTSHIELD_HOST}${endpoint}" \
        -H "Authorization: Bearer ${BLASTSHIELD_TOKEN}")

    if [ "$DEBUG" = true ]; then
        echo "  [DEBUG] Response length: ${#response}"
        echo "  [DEBUG] Response preview: $(echo "$response" | head -c 200)"
    fi

    # Check if response is empty or invalid
    if [ -z "$response" ] || [ "$response" = "null" ]; then
        echo "  ERROR: No response from API"
        return
    fi

    # Check if we got an error
    error=$(echo "$response" | jq -r '.detail // empty' 2>/dev/null)
    if [ -n "$error" ]; then
        echo "  ERROR: $error"
        return
    fi

    # Try both .items[] (paginated) and .[] (direct array) response formats
    # Show id, name, and tags if available
    items=$(echo "$response" | jq -r '
        (if type == "object" then (.items // []) else . end)
        | .[]?
        | select(.name? // false | tostring | startswith("test-"))
        | "\(.id)|\(.name)|\(.tags // {} | to_entries | map("\(.key)=\(.value)") | join(","))"
    ' 2>&1)

    # Check if jq had an error
    if [ $? -ne 0 ]; then
        echo "  ERROR parsing response (jq failed)"
        echo "  Response: $(echo "$response" | head -c 100)..."
        return
    fi

    if [ -z "$items" ]; then
        echo "  (no test resources found)"
        return
    fi

    while IFS='|' read -r id name tags; do
        if [ -n "$id" ]; then
            if [ "$DRYRUN" = true ]; then
                if [ -n "$tags" ] && [ "$tags" != "" ]; then
                    echo "  Would delete: [$id] $name (tags: $tags)"
                else
                    echo "  Would delete: [$id] $name (no tags)"
                fi
            else
                echo "  Deleting: [$id] $name"
                curl -s -X DELETE "${BLASTSHIELD_HOST}${endpoint}${id}" \
                    -H "Authorization: Bearer ${BLASTSHIELD_TOKEN}" > /dev/null
            fi
        fi
    done <<< "$items"

    echo ""
}

# Clean up each resource type
cleanup_resource "Nodes" "/nodes/"
cleanup_resource "Endpoints" "/endpoints/"
cleanup_resource "Groups" "/groups/"
cleanup_resource "Services" "/services/"
cleanup_resource "Policies" "/policies/"
cleanup_resource "Egress Policies" "/egress_policies/"
cleanup_resource "Proxies" "/proxies/"
cleanup_resource "Event Log Rules" "/event_log_rules/"

if [ "$DRYRUN" = true ]; then
    echo "Run without --dryrun to delete these resources"
else
    echo " Cleanup complete!"
fi
