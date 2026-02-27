#!/bin/bash

# Test Runner Script for Mail Admin and Groupware
# Usage: ./run-tests.sh [mailadmin|groupware|all]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Environment variables
export DATABASE_URL='mailuser:PostFixCtl8149C?+@tcp(127.0.0.1:3306)/mailserver?parseTime=true'
export JWT_SECRET='test-secret-key-for-testing'
export ADMIN_USERS='admin:TestAdmin123'

echo "=================================="
echo "  Mail System Test Runner"
echo "=================================="
echo ""

run_mailadmin_tests() {
    echo -e "${YELLOW}Running Mail Admin Tests...${NC}"
    echo "-----------------------------------"
    cd /opt/mailadmin/backend

    if go test -v ./handlers/... 2>&1; then
        echo -e "${GREEN}Mail Admin Tests: PASSED${NC}"
        return 0
    else
        echo -e "${RED}Mail Admin Tests: FAILED${NC}"
        return 1
    fi
}

run_groupware_tests() {
    echo -e "${YELLOW}Running Groupware Tests...${NC}"
    echo "-----------------------------------"
    cd /opt/groupware

    if go test -v ./handlers/... 2>&1; then
        echo -e "${GREEN}Groupware Tests: PASSED${NC}"
        return 0
    else
        echo -e "${RED}Groupware Tests: FAILED${NC}"
        return 1
    fi
}

run_benchmarks() {
    echo -e "${YELLOW}Running Benchmarks...${NC}"
    echo "-----------------------------------"

    echo "Mail Admin Benchmarks:"
    cd /opt/mailadmin/backend
    go test -bench=. -benchmem ./handlers/... 2>&1 || true

    echo ""
    echo "Groupware Benchmarks:"
    cd /opt/groupware
    go test -bench=. -benchmem ./handlers/... 2>&1 || true
}

case "${1:-all}" in
    mailadmin)
        run_mailadmin_tests
        ;;
    groupware)
        run_groupware_tests
        ;;
    bench|benchmark)
        run_benchmarks
        ;;
    all)
        MAILADMIN_RESULT=0
        GROUPWARE_RESULT=0

        echo ""
        run_mailadmin_tests || MAILADMIN_RESULT=1

        echo ""
        run_groupware_tests || GROUPWARE_RESULT=1

        echo ""
        echo "=================================="
        echo "  Test Summary"
        echo "=================================="

        if [ $MAILADMIN_RESULT -eq 0 ]; then
            echo -e "  Mail Admin: ${GREEN}PASSED${NC}"
        else
            echo -e "  Mail Admin: ${RED}FAILED${NC}"
        fi

        if [ $GROUPWARE_RESULT -eq 0 ]; then
            echo -e "  Groupware:  ${GREEN}PASSED${NC}"
        else
            echo -e "  Groupware:  ${RED}FAILED${NC}"
        fi

        echo "=================================="

        if [ $MAILADMIN_RESULT -eq 0 ] && [ $GROUPWARE_RESULT -eq 0 ]; then
            echo -e "${GREEN}All tests passed!${NC}"
            exit 0
        else
            echo -e "${RED}Some tests failed!${NC}"
            exit 1
        fi
        ;;
    *)
        echo "Usage: $0 [mailadmin|groupware|bench|all]"
        echo ""
        echo "  mailadmin  - Run Mail Admin tests only"
        echo "  groupware  - Run Groupware tests only"
        echo "  bench      - Run benchmark tests"
        echo "  all        - Run all tests (default)"
        exit 1
        ;;
esac
