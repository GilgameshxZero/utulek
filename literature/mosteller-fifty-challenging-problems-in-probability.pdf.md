# Fifty Challenging Problems in Probability

Frederick Mosteller
November 30, 2022
Annotated by Yang Yan

## 1

Let the number of red and black socks be $r$ and $b$, respectively. We are given

$$\begin{aligned}
2\cdot\frac{r}{r+b}&=\frac{r+b-1}{r-1}\\[3ex]
\implies r^2+br-r+br+b^2-b&=2r^2-2r.
\end{aligned}$$

Given that this is a quadratic in $r$, it makes most sense to solve for $r$:

$$\begin{aligned}
r^2-(2b+1)r+b-b^2&=0\\
\implies r&=\frac{2b+1\pm\sqrt{8b^2+1}}{2}.
\end{aligned}$$

It’s evident that $8b^2+1$ must be a square. Trying smaller values for $b$, we have

$b$|$r$
-|-
$1$|$3$
$6$|$15$

which quickly gives our answers for both parts.

### Solving Pell’s equation

The desire to generalize such solutions is not unfamiliar. In fact, each square $8b^2+1$ is odd, and thus necessarily gives a valid positive integral solution for $r$. We are then inspired to make the change of variables $x=\mp(2r-2b-1)$ and $y=b$ to obtain a classic Pell’s equation with $n=8$:

$$x^2-8y^2=1$$

whose solutions in the positive integers are equivalent to solutions to the original problem.
