# Fifty Challenging Problems in Probability

Frederick Mosteller
November 30, 2022
Annotated by Yang Yan

## 1

Let the number of red and black socks be $r$ and $b$, respectively. We are given

$$\begin{aligned}
2\cdot\frac{r}{r+b}&=\frac{r+b-1}{r-1}\\[3ex]
\implies r^2+br-r+br+b^2-b&=2r^2-2r
\end{aligned}$$

Originally it seems promising to solve for $b$, considering part (b), but given that this is a quadratic in $r$, it makes most sense to solve for $r$ instead:

$$\begin{aligned}
r^2-(2b+1)r+b-b^2&=0\\
\implies r&=\frac{2b+1\pm\sqrt{8b^2+1}}{2}.
\end{aligned}$$

It’s evident that $8b^2+1$ has to be a square:

$b$|$r$
-|-
$1$|$3$
$6$|$15$

which quickly gives our answers for both parts.

---

In fact, each square $8b^2+1$ is odd, and thus necessarily gives a valid solution for $r$. We can make then the inspired change of variables

$$\begin{aligned}
r&=\frac{2b+1\pm\sqrt{8b^2+1}}{2}\\[3ex]
\implies x^2-8y^2&=1\text{ with }r=\frac{2b+1\pm x}{2}\text{ and }b=y
\end{aligned}$$

which is a classic Pell equation of $n=8$.
