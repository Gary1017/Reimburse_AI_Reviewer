———
name: CountCodeBase
description: A skill to calculate the number of lines in the current project
———

When the user mentioned that he/she wants to know the exact lines of production code of the current project:

    1. You should call @count-codebase-scale.sh and show the shell output to the user.
    2. You should mention what are included and what are not included:
        - Included file types: .go, .sql, .sh, Dockerfile, Makefile, .tf
        - Excluded directories: .git, vendor, node_modules, bin, attachments, generated_vouchers
        - Excluded files: *_test.go