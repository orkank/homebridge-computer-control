#!/bin/sh
# Example shell script for Computer Control actions
# Use the full path in the client, e.g. /path/to/examples/example.sh
# {status} is replaced with "on" or "off" when used as a toggle

echo "Action triggered: $1"
# Example: show a notification (macOS)
# osascript -e "display notification \"Action: $1\" with title \"Computer Control\""
