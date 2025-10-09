#!/bin/bash

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
YELLOW='\033[1;33m'

# PostgreSQL connection settings - these will be prompted
echo -n "Enter PostgreSQL host (default: localhost): "
read PGHOST
PGHOST=${PGHOST:-localhost}

echo -n "Enter PostgreSQL port (default: 5432): "
read PGPORT
PGPORT=${PGPORT:-5432}

echo -n "Enter PostgreSQL database name (default: concourse): "
read PGDATABASE
PGDATABASE=${PGDATABASE:-concourse}

echo -n "Enter PostgreSQL username: "
read PGUSER

echo -n "Enter PostgreSQL password: "
read -s PGPASSWORD
echo

# Export for psql
export PGHOST PGPORT PGDATABASE PGUSER PGPASSWORD

echo -e "\n${YELLOW}Running cleanup verification tests...${NC}\n"

# Function to run SQL and check results
run_sql_test() {
    local test_name=$1
    local sql=$2
    local expected_condition=$3
    
    result=$(psql -t -A -c "$sql")
    
    if [ $? -ne 0 ]; then
        echo -e "${RED}✗ $test_name - Failed to execute SQL${NC}"
        return 1
    fi
    
    if eval "$expected_condition"; then
        echo -e "${GREEN}✓ $test_name - PASS${NC}"
        return 0
    else
        echo -e "${RED}✗ $test_name - FAIL (got: $result)${NC}"
        return 1
    fi
}

# 1. Check for orphaned inputs
echo -e "\n${YELLOW}1. Checking for orphaned inputs...${NC}"
run_sql_test "Orphan Check" "
    SELECT COUNT(*) AS orphan_inputs
    FROM build_resource_config_version_inputs bi
    LEFT JOIN builds b ON bi.build_id = b.id
    WHERE b.id IS NULL;" "[ \"$result\" -eq 0 ]"

# 2. Verify retention window (90 days)
echo -e "\n${YELLOW}2. Verifying build retention window...${NC}"
run_sql_test "Retention Window" "
    SELECT COUNT(*) 
    FROM builds 
    WHERE reap_time < NOW() - INTERVAL '90 days';" "[ \"$result\" -eq 0 ]"

# 3. Check cascade deletion
echo -e "\n${YELLOW}3. Checking cascade deletion...${NC}"
run_sql_test "Cascade Deletion" "
    SELECT COUNT(*)
    FROM build_resource_config_version_inputs bi
    LEFT JOIN builds b ON bi.build_id = b.id
    WHERE b.id IS NULL;" "[ \"$result\" -eq 0 ]"

# 4. Verify indexes
echo -e "\n${YELLOW}4. Verifying required indexes...${NC}"
run_sql_test "Index Check" "
    SELECT COUNT(*)
    FROM pg_indexes
    WHERE tablename = 'build_resource_config_version_inputs'
    AND (indexdef LIKE '%resource_id%version_md5%'
         OR indexdef LIKE '%build_id%');" "[ \"$result\" -ge 2 ]"

# 5. Get table statistics
echo -e "\n${YELLOW}5. Current table statistics:${NC}"
psql -c "
    SELECT 
        relname as table_name,
        n_live_tup as row_count,
        pg_size_pretty(pg_total_relation_size(C.oid)) as total_size
    FROM pg_class C
    LEFT JOIN pg_namespace N ON (N.oid = C.relnamespace)
    WHERE relname = 'build_resource_config_version_inputs';"

# 6. Check cleanup job logs (if running locally)
echo -e "\n${YELLOW}6. Checking ATC logs for cleanup job (last 1 hour):${NC}"
if [ -f "/var/log/concourse/atc.log" ]; then
    grep -i "cleanup-old-builds" /var/log/concourse/atc.log | tail -n 5
else
    echo -e "${YELLOW}⚠ Could not check ATC logs - log file not found${NC}"
fi

echo -e "\n${YELLOW}Verification complete!${NC}"

# Print summary of build counts
echo -e "\n${YELLOW}Summary:${NC}"
psql -c "
    SELECT 
        COUNT(*) as total_builds,
        COUNT(*) FILTER (WHERE reap_time IS NOT NULL) as reaped_builds,
        COUNT(*) FILTER (WHERE reap_time < NOW() - INTERVAL '90 days') as old_reaped_builds
    FROM builds;"

echo -e "\nTo test API endpoint performance, run:"
echo -e "${YELLOW}time curl -s http://<concourse-url>/api/v1/teams/<team>/pipelines/<pipeline>/resources/<resource>/versions/<version>/input_to${NC}"