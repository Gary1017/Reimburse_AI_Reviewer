#!/bin/bash

# This script calculates the lines of code for the project.
# It only includes source code files and excludes test files.
# It also ignores empty lines.
#
# Included file types: .go, .sql, .sh, Dockerfile, Makefile, .tf
# Excluded directories: .git, vendor, node_modules, bin, attachments, generated_vouchers
# Excluded files: *_test.go

find . \
    -path './.git' -prune -o \
    -path './vendor' -prune -o \
    -path './node_modules' -prune -o \
    -path './bin' -prune -o \
    -path './attachments' -prune -o \
    -path './generated_vouchers' -prune -o \
    -type f \( \
        -name "*.go" -o \
        -name "*.sql" -o \
        -name "*.sh" -o \
        -name "Dockerfile" -o \
        -name "Makefile" -o \
        -name "*.tf" \
    \) -a -not -name "*_test.go" \
    -print0 | \
    xargs -0 sed '/^\s*$/d' | \
    wc -l
