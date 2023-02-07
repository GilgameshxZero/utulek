# articles

Articles are moved offline with Firefox’s Reader View, with the following settings:

1. Serif
2. 4 (font size)
3. 8 (column width)
4. 5 (line spacing)

The articles are subsequently printed to PDF with custom margins of

1. Top: 0.50in
2. Bottom: 0.50in
3. Left: 0.00in (left and right margins seem to have some default to them).
4. Right: 0.00in

For Codeforces articles, fonts must be loaded beforehand (wait on page a bit before going into Reader View), and the spoiler sections must be expanded:

```javascript
document.querySelectorAll(`.spoiler-content`).forEach((i) => {i.style = `display: block;`;});
```
