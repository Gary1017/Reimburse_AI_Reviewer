#!/bin/bash

# This script is designed to automate the process of restarting the Go server
# for the AI_Reimbursement project. It simplifies development by providing a
# single command to kill the currently running server and start a new instance.

# --- Configuration ---
# The main command to run the server.
# We use `go run` for simplicity during development. For production, you might
# use a compiled binary.
SERVER_COMMAND="go run cmd/server/main.go"

# Log file for the server output. Using `tee` allows viewing logs in real-time
# while also saving them to a file.
LOG_FILE="server.log"

# --- Script Logic ---

echo "Attempting to restart the server..."

# 1. Find and terminate the active server process
# We use `pkill -f` to find the process by its full command line argument.
# This is more reliable than matching just the process name.
# The `|| true` prevents the script from exiting if no process is found.
echo "--> Searching for and terminating existing server process..."
pkill -f "$SERVER_COMMAND" || true
# Brief pause to allow the OS to terminate the process gracefully.
sleep 1

# 2. Start the new server instance
# We run the server in the background using `&` so that this script can exit
# while the server continues to run.
# `nohup` ensures the process isn't terminated when the shell session ends.
# Output is redirected to a log file for debugging and monitoring.
echo "--> Starting new server instance in the background..."
nohup $SERVER_COMMAND > $LOG_FILE 2>&1 &

# 3. Confirmation
# Get the Process ID (PID) of the newly started server for confirmation.
# `pgrep` is used again to find the PID of the new process.
# We add a small delay to give the process time to start.
sleep 1
NEW_PID=$(pgrep -f "$SERVER_COMMAND")

if [ -n "$NEW_PID" ]; then
  echo "--> Server restarted successfully!"
  echo "    New Process ID (PID): $NEW_PID"
  echo "    Logs are being written to: $LOG_FILE"
  echo "    To view live logs, run: tail -f $LOG_FILE"
else
  echo "--> Error: Server failed to start."
  echo "    Check '$LOG_FILE' for error messages."
fi

echo "Restart script finished."



