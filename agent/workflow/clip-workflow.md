# Clipboard workflow

Use `../../monochrome/bin/clip` to copy text to the desktop clipboard. Pass the text as an argument, for example:

```sh
../../monochrome/bin/clip 'text to copy'
```

Whenever giving the user commands to run, copy those exact commands to the clipboard with this tool.

If the script fails because sandboxing denies access to the desktop D-Bus, retry the same clipboard operation with elevated sandbox access. The user has explicitly authorized this retry for clipboard copies; do not assume the initial sandbox error means clipboard access is impossible.
