<link rel="stylesheet" href="../.vscode/default.css">
<link rel="stylesheet" href="../silver/selective/silverine.css">
<link rel="stylesheet" href="../silver/selective/h1.subtitle.css">

<input type="checkbox" class="silver-theme-toggle" />

<section class="silver-center">

# meditations on `silver`ine

and the rights of robots

❄ · 2025-05-18

</section><section>

## Introduction

![](2025_05_18-silverine.0.md.assets/iris.png#small)
*Iris.EXE, from the hit game Mega Man Battle Network 6*

Does Iris deserve sympathy?

</section><section class="silver-center">

<div class="silver-h0"></div>

167

lives lost in the volcanic collapse of Wily’s lab—but who to answer for it?

</section><section>

## On what code does Iris run?

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

Probably C++17. 尽然通过compiler能够算出Fibonacci数字！

</section><section>

Should Iris be the product of two ideals, $A=(\alpha,(\alpha+\alpha\delta)/2),B=(\beta,(\beta+\beta\delta)/2),\delta=\sqrt{-5}$, then she would be

$$\begin{aligned}
(AB)&=(\alpha\beta, \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{4}(\alpha\beta-5\alpha\beta+2\alpha\beta\delta))\\
&=(\alpha\beta, \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(-2+\delta)).
\end{aligned}$$

From this it is clear $(AB)\subseteq (\alpha\beta/2).$ Furthermore, we observe that

$$\frac{\alpha\beta}{2}(1+\delta+2-\delta)=\frac{3\alpha\beta}{2}\in (AB)\tag{1.1}$$

but also $\alpha\beta\in (AB)$, so it must be that $\alpha\beta/2\in (AB)$. Thus,

$$(\alpha\beta/2)\subseteq (AB)\subseteq(\alpha\beta/2)
\tag{poggers}$$

and hence $(AB)=(\alpha\beta/2)$ and is a principal ideal. That’s pretty cute.

</section><section>

## One might ask, what happens with long titles? Or what do I think of Iris’s thighs? Or why are there no right-aligned images?

1. I think they’re quite nice.
2. I cannot say for sure about the images.

&nbsp;|Emilia|Rem|Beatrice|Elsa
-|-|-|-|-
Smile|5|5|1|5
Eyes|4|2|2|3
Dress|4|3|4|5

</section><section class="silver-center">

> **Theorem: $x\in\text{Iris}\implies x\in\text{cute}$.**
>
> **Proof**: Take any $x\not\in\text{cute}$. Then it must be ugly. But nothing in $\text{Iris}$ is ugly. Thus the contrapositive is true and the original claim is as well. $\blacksquare$

> **Theorem: 2D Maximum Subsquare $\Longleftrightarrow$ APSP**
>
> We know from Tamaki et al. [1998] that 2D Maximum Subarray reduces to APSP. Williams et al. [2016] shows the other direction for equivalence. Restriction to squares is not necessarily an easier problem, but iterating on diagonals shows that it satisfies a Min-Plus structure, which is well-known to be equivalent to APSP.

</section>

</section><section class="silver-bottom silver-right tangerine">

You are awesome for coming to my presentation.
For more, please visit <https://github.com/gilgameshxzero/silver> (experimental).
Thank you.

</section>
