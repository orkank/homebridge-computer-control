# Action Examples

Example scripts for Computer Control actions. Use the **full path** in the client.

## Shell (example.sh)

- **macOS/Linux:** Use full path, e.g. `/path/to/examples/example.sh`
- Add `{status}` for toggle: `/path/to/examples/example.sh {status}` → receives "on" or "off"
- Make executable: `chmod +x example.sh`

## Batch (example.bat)

- **Windows:** Use full path, e.g. `C:\path\to\examples\example.bat`
- Add `{status}` for toggle: `C:\path\to\examples\example.bat {status}` → receives %1

## AppleScript (example.applescript)

- **macOS only:** Use the file path (e.g. `~/Downloads/example.applescript`) or paste inline script
- File path: runs `osascript <path>`
- Inline: runs `osascript -e "<your script>"`
- Add `{status}` in the script for toggle support
