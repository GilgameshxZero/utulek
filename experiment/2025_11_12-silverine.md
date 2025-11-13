<link rel="stylesheet" href="../.vscode/default.css">
<link rel="stylesheet" href="../silver/selective/h1.subtitle.css">
<link rel="stylesheet" href="../silver/selective/silverine.css">

<section class="silver-center">

# meditations on `silver`ine

and the rights of robots

A collection of slides demonstrating `silverine`.

</section><section>

## Introduction

![](2025-05-18-silverine.0.md-assets/iris.png#small)
*Iris.EXE, from the game Mega Man Battle Network 6.*

Does Iris deserve sympathy, despite being a robot?

</section><section class="silver-center">

<div class="silver-h0"></div>

167

lives lost in the volcanic collapse of Wily’s lab—but who to answer for it?

</section><section>

## On what language does Iris run?

```c++
template <std::size_t INDEX>
class CTFibonacciExplicit;

template <>
class CTFibonacciExplicit<0> : public CTSizeT<0> {};

template <>
class CTFibonacciExplicit<1> : public CTSizeT<1> {};

template <std::size_t INDEX>
class CTFibonacciExplicit : public CTSizeT<
															CTFibonacciExplicit<INDEX - 1>::get() +
															CTFibonacciExplicit<INDEX - 2>::get()> {};
```


应该是 C++17. Or it could be Chinese!

</section><section>

Suppose Iris has two ideals: $A=(\alpha,(\alpha+\alpha\delta)/2),B=(\beta,(\beta+\beta\delta)/2),\delta=\sqrt{-5}$. She would find their product as

$$\begin{aligned}
(AB)&=(\alpha\beta, \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{4}(\alpha\beta-5\alpha\beta+2\alpha\beta\delta))\\
&=(\alpha\beta, \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(-2+\delta)).
\end{aligned}$$

From this it is clear $(AB)\subseteq (\alpha\beta/2).$ Furthermore, we observe that

$$\frac{\alpha\beta}{2}(1+\delta+2-\delta)=\frac{3\alpha\beta}{2}\in (AB)\tag{1.1}$$

but also $\alpha\beta\in (AB)$, so it must be that $\alpha\beta/2\in (AB)$. Thus,

$$(\alpha\beta/2)\subseteq (AB)\subseteq(\alpha\beta/2)
\tag{look at this tag!}$$

and hence $(AB)=(\alpha\beta/2)$ and is a principal ideal.

</section><section>

## One might ask, what happens with long titles? Or why are there no right-aligned images?

1. They wrap.
2. I cannot say for sure about the images.
3. Check out this table.

&nbsp;|Iris|LLMs
-|-|-
Sandbagging?|✗|✔
Deceptive?|✔|✔
Robust?|✗|✗

</section><section class="silver-center">

> **Theorem: 2D Maximum Subsquare $\Longleftrightarrow$ APSP**
>
> We know from Tamaki et al. [1998] that 2D Maximum Subarray reduces to APSP. Williams et al. [2016] shows the other direction for equivalence. Restriction to squares is not necessarily an easier problem, but iterating on diagonals shows that it satisfies a Min-Plus structure, which is well-known to be equivalent to APSP.$\blacksquare$

While printing, ensure that headers/footers are OFF and background graphics are ON. Using a Chromium browser also helps.

</section>

</section><section class="silver-bottom silver-right silver-tangerine">

You are awesome for coming to my presentation.
For more `silverine`, please visit <https://github.com/gilgameshxzero/silver>.
The Markdown source for these slides can be found at <>.
Thank you.

</section>
