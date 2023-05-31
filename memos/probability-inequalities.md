<!-- emilia-snapshot-properties
Six proability distribution inequalities
2023/05/30
utulek
emilia-snapshot-properties -->

# Six proability distribution inequalities

May 30, 2023

It occurs to me that the probability bounds most commonly used in both randomized algorithms as well as in quantitative analysis are few in number. I briefly summarize their intended usage here.

## Union bound (and subsequent Bonferroni inequalities)

Take the principle of inclusion-exclusion for the set of events $X_i$. We have

$$\begin{aligned}P(X_1\cup\ldots\cup X_N)&=P(X_1)+\ldots+P(X_N)-\ldots\\
&\leq\sum P(X_i).\end{aligned}$$

## Markov bound

For a non-negatively valued distribution, take any integer $x$. Should we collapse all the mass above $x$ into $x$, and discard the mass below $x$, we would have expected value $xP(X\geq x)$. Necessarily, this is less than the original expected value $E[X]$ due to the shifting and removal of masses. Hence,

$$P(X\geq x)\leq E[X]/x.$$

## Chebyshev bound

And thus from the Markov bound we have upper-bounds on the tails of any distribution, simply by taking its square to make it non-negative.

## Jensen’s inequality

For some function $f$ whose second-derivative is monotone—i.e. it is either convex or concave—we have for some two points on the function, any point in between must always lie below or above the curve, depending on if $f$ is convex or concave.

For concave function $f(x)=\sqrt{x}$, we have that every point on a line between two points on $f$ must lie below $f$. Thus,

$$E[f(X)]\leq f(E[X])\iff \sqrt{E[X]}\geq E[\sqrt{X}].$$

This is expectation over any distribution—and thus covers any point on the line between the two points.

Notably, the sample standard deviation is seen on the LHS—and necessarily has non-negative bias.

## Chernoff bounds

Similar to the Chebyshev, the Chernoff bounds provide a series of exponential bounds for distribution tails. These are derived from the Markov bound on the moment generating functions, noting that they are monotone with respect to the exponent $n$.

Let us then recall the moment generating function $M_X(s)$ of a distribution $X$:

$$M_X(s)=E[e^{sX}].$$

## Cauchy-Schwarz

Terence Tao has an in-depth blog on the method used to derive Cauchy-Schwarz.
