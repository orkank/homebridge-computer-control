#!/usr/bin/osascript

-- Example AppleScript for Computer Control actions (macOS only)
-- Copy the line below into the Value field (Action type: AppleScript)
-- {status} is replaced with "on" or "off" when used as a toggle

-- This opens a warning dialog (Definitely visible)
display dialog "Action triggered! Status: {status}" with title "Computer Control" buttons {"OK"} default button "OK"