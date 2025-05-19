<link rel="stylesheet" href="../.vscode/default.css">
<link rel="stylesheet" href="../silver/selective/silverine.css">

<input type="checkbox" class="silver-theme-toggle" />

<section class="center">

# meditations on `silver`ine

and the rights of robots

❄ · 2025-05-18

</section><section>

## Introduction

![](silverine.md-assets/iris.png#small)
*Iris.EXE, from the hit game Mega Man Battle Network 6*

Does Iris deserve sympathy?

</section><section class="center">

<div class="silver-h0"></div>

167

lives lost in the volcanic collapse of Wily’s lab—but who to answer for it?

</section><section>

## What code does Iris run on?

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

Should Iris be the product of two ideals, $\alpha,\beta$, then she would be

$$\begin{aligned}
(AB)&=(\alpha\beta, \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{2}(\alpha\beta+\alpha\beta\delta), \frac{1}{4}(\alpha\beta-5\alpha\beta+2\alpha\beta\delta))\\
&=(\alpha\beta, \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(1+\delta), \frac{\alpha\beta}{2}(-2+\delta)).
\end{aligned}$$

From this it is clear $(AB)\subseteq (\alpha\beta/2).$ Furthermore, we observe that

$$\frac{\alpha\beta}{2}(1+\delta+2-\delta)=\frac{3\alpha\beta}{2}\in (AB)\tag{1.1}$$

but also $\alpha\beta\in (AB)$, so it must be that $\alpha\beta/2\in (AB)$. Thus,

$$(\alpha\beta/2)\subseteq (AB)\subseteq(\alpha\beta/2)
\tag{poggers}$$

and hence $(AB)=(\alpha\beta/2)$ and is a principal ideal.

</section><section>

## One might ask, what happens with long titles? Or what do I think of Iris’s thighs? Or why are there no right-aligned images?

1. I cannot say for sure about the images.
2. If I was Zero I’d cry too.

&nbsp;|Emilia|Rem|Beatrice|Elsa
-|-|-|-|-
Smile|5|5|1|5
Eyes|4|2|2|3
Dress|4|3|4|5

</section><section class="center">

> **Theorem: Iris $\Leftrightarrow$ cute.**
>
> For the forward direction, consider the contrapositive: If it is not cute, it cannot be Iris.
>
> Consider any homomorphism $\phi:\text{Cuteness}\to\text{Iris}$. Surely, *non-trivial statement*, so we have that all $\phi$ operate on the entire domain, so that $\phi$ is injective.
> 
> The reverse directly is similar. $\blacksquare$

</section>

</section><section class="bottom right tangerine">

Thank you.

</section>
