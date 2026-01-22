———
name: RestartServer
description: This skill automates killing the running server and restarting it for testing purposes.This skill provides functionality to:
- Terminate the currently running server process
- Start a fresh server instance
- Support automated test workflows

———


# Restart Server Skill


## Usage

```bash
./restart_server.sh
```

## Implementation

The batch script is located at: `restart-server.sh`

### Script Details

Refer to [`restart-server.sh`]for the implementation.

The script handles:
1. Finding and terminating the active server process
2. Waiting for graceful shutdown
3. Clearing temporary files (optional)
4. Starting the new server instance

## Integration

Include this skill in your test automation pipeline to ensure a clean server state between test runs.
