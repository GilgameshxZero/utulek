# Questions

It seems productive to keep a list of technical questions that I haven’t be able to fully solve somewhere.

1. “Tetration” function $f(x)=x^x$: why is it so hard to solve for $f(x)=n$ for $x>0,n>0$ when this is easily graphable, and determinable computationally (e.g. via estimation)?
2. Twofive (<https://usaco.training/usacoprob2?a=WKwbb6ahvPc&S=twofive>) extended: suppose all 26 letters are allowd (with the choice of which letter to omit).
	1. Can we keep the same DP as in `twofive`, and instead of simply considering one choice (with the ommitance of `Z`), consider the ommitting of any of the 26 letters, to determine the total number of placements with a certain prefix?
		1. I think this is correct, probably resolved.
3. HSP (<gilgamesh.cc/snapshots/altair/homochromatic-square-perimeters.html?noscript>) quadratic time: I don’t yet fully understand the nature of the offline linear-time DSU yet. Perhaps it’s time to read the paper.
