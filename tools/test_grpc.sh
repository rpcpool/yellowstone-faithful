#!/bin/bash

# Colors for better output readability
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if token is provided as a parameter
if [ -z "$1" ]; then
    echo -e "${RED}Error: Authentication token is required.${NC}"
    echo -e "Usage: $0 <auth_token> [server] [proto_path]"
    echo -e "Example: $0 4e730ca3-9e7b-4148-859e-6840173c58a9"
    exit 1
fi

# Set your authentication token and server
TOKEN="$1"
SERVER="${2:-lb-lon3.nodes.rpcpool.com:443}"  # Default server if not provided
PROTO_PATH="${3:-../old-faithful-proto/proto/old-faithful.proto}"   # Default proto path if not provided

# Validate requirements
if ! command -v grpcurl &> /dev/null; then
    echo -e "${RED}Error: grpcurl is not installed. Please install it first.${NC}"
    echo "You can install it from: https://github.com/fullstorydev/grpcurl"
    exit 1
fi

echo -e "${YELLOW}====== Testing Yellowstone gRPC Functionality ======${NC}"
echo -e "Using server: ${SERVER}"
echo -e "Using proto file: ${PROTO_PATH}"

# Function to run a gRPC call and check result
run_test() {
    local test_name="$1"
    local endpoint="$2"
    local data="$3"

    echo -e "\n${YELLOW}Testing $test_name...${NC}"
    echo "Command: grpcurl -proto $PROTO_PATH -H 'x-token: $TOKEN' -d '$data' $SERVER $endpoint"

    result=$(grpcurl -proto $PROTO_PATH -H "x-token: $TOKEN" -d "$data" $SERVER $endpoint 2>&1)

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Success!${NC}"
        echo "Response snippet (first 300 chars):"
        echo "${result:0:300}..."
    else
        echo -e "${RED}✗ Failed!${NC}"
        echo "Error: $result"
    fi
}

# 1. Test GetVersion
run_test "GetVersion" "OldFaithful.OldFaithful/GetVersion" '{}'

# 2. Test GetBlock (with a known slot)
run_test "GetBlock" "OldFaithful.OldFaithful/GetBlock" '{"slot": 307152000}'

# 3. Test GetTransaction (with a known signature)
# Note: You might need to replace this with a valid transaction signature
run_test "GetTransaction" "OldFaithful.OldFaithful/GetTransaction" '{"signature": "29mRGbKwAcd7azaLAFa6vg9SUQgJNaVnadqinfZfNn65bSjQxgEvU7EQqDSeYr89U96b5wBdvCAJiknLAoh65qdu"}'

# 4. Test StreamBlocks (a small range)
run_test "StreamBlocks" "OldFaithful.OldFaithful/StreamBlocks" '{"start_slot": 307152000, "end_slot": 307152010}'

# 5. Test StreamBlocks with account filter
run_test "StreamBlocks with account filter" "OldFaithful.OldFaithful/StreamBlocks" '{"start_slot": 307152000, "end_slot": 307152010, "filter": {"account_include": ["Vote111111111111111111111111111111111111111"]}}'

# 6. Test StreamTransactions
run_test "StreamTransactions" "OldFaithful.OldFaithful/StreamTransactions" '{"start_slot": 307152000, "end_slot": 307152010, "filter": {"vote": false, "failed": true}}'

# 7. Test StreamTransactions with account filters
run_test "StreamTransactions with account filters" "OldFaithful.OldFaithful/StreamTransactions" '{"start_slot": 307152000, "end_slot": 307152010, "filter": {"vote": false, "failed": true, "account_include": ["Vote111111111111111111111111111111111111111"], "account_exclude": [], "account_required": []}}'

# 8. Test bidirectional streaming using Get method
echo -e "\n${YELLOW}Testing bidirectional streaming with Get...${NC}"
temp_file=$(mktemp)
cat > $temp_file << EOF
{"id": 1, "block": {"slot": 307152000}}
{"id": 2, "transaction": {"signature": "29mRGbKwAcd7azaLAFa6vg9SUQgJNaVnadqinfZfNn65bSjQxgEvU7EQqDSeYr89U96b5wBdvCAJiknLAoh65qdu"}}
{"id": 3, "version": {}}
EOF

echo "Input data:"
cat $temp_file
echo

echo "Running bidirectional stream with Get..."
result=$(cat $temp_file | grpcurl -proto $PROTO_PATH -H "x-token: $TOKEN" -d @ $SERVER OldFaithful.OldFaithful/Get 2>&1)

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Bidirectional streaming successful!${NC}"
    echo "Response snippet (first 300 chars):"
    echo "${result:0:300}..."
else
    echo -e "${RED}✗ Bidirectional streaming failed!${NC}"
    echo "Error: $result"
fi

rm $temp_file

echo -e "\n${YELLOW}====== Testing Complete ======${NC}"
echo "Note: For failures, check if the server is accessible and your token is valid."
echo "You might also need to adjust slots or transaction signatures to match available data."
