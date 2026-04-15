# stripcolor

Simple program to strip ansi color sequences when running a program.

Usage: `stripcolor COMAND ...`

This program executes `COMMAND` under a pseudo-terminal and removes
all escape sequences that attempt to change foreground colors,
background colors, or text intensity (bold, dim, etc.).

It is particularly useful for programs like *OpenCode* whose TUI
has poor light-mode support and forces low-contrast colors (especially
light gray text on light backgrounds).   While these programs may
claim to support *themes* those themes require specialized terminal
applications (e.g., they do not work on macOS's Terminal.app)  Many
modern AI-written terminal applications scatter color logic throughout
the codebase and only properly test against dark-themed terminals.

Dark themes and heavy use of color can significantly reduce readability
as we age.  This problem becomes even worse with *ER IOLs* (Extended
Range Intraocular Lenses) after cataract surgery.
