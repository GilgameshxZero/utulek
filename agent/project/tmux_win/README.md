# tmux_win console wrapping fix

This project maintains a small patch to the pinned native Windows tmux release
[3.6a-win32.7](https://github.com/arndawg/tmux-windows/releases/tag/v3.6a-win32.7).
The upstream source is reconstructed from the hash-pinned archive in
`dependencies.json`; `patches/console_wrap.patch` is the only source change.

The client advertised automatic margins while disabling Windows console line
wrapping. Full-width redraws could overwrite the final column and leave stale
rows/colors on screen, despite a correct application console and tmux pane grid.
The fix includes `ENABLE_WRAP_AT_EOL_OUTPUT` in the output console mode.

`scripts/build_windows.ps1 -Root <private-build-directory>` verifies the pinned
source/libevent/CMake archives, extracts into that directory, applies the exact
patch and builds with existing Visual Studio 2022 MSVC and static libevent.
Stage the portable CMake zip/extracted directory if the optional VS CMake
component is absent. Use this project's `.obj/windows` for local build staging,
`.bin/windows/tmux.exe` for the finished executable, and
`monochrome/app/tmux.exe` for installation. No additional runtime DLLs are needed.
The Windows test staging convention is `%USERPROFILE%/.obj/tmux_win`.

The regression harness in `../neomutt_xp/tests/check_windows_tmux.js` exercises
scrolling, index/pager view transitions, detach/reattach and resize with two
color themes on system and bundled ConPTY. Original release: stale pager on
both themes. Patched release: all 40 visual-state checks pass. The installed
cmus_xp also passes all 20 of its existing terminal/default-color assertions
with this tmux release. Neither its player nor user settings were changed.

The resulting executable imports only Windows WS2_32, ADVAPI32 and KERNEL32.
The upstream license is retained in `COPYING`.
