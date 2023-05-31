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

### Reduction to Pell’s equation

The desire to generalize such solutions is not unfamiliar. In fact, each square $8b^2+1$ is odd, and thus necessarily gives a valid positive integral solution for $r$. We are then inspired to make the change of variables $x=\mp(2r-2b-1)$ and $y=b$ to obtain a classic Pell’s equation with $n=8$:

$$x^2-8y^2=1$$

whose solutions in the positive integers are equivalent to solutions to the original problem.

## 19

I thought this was a particularly nice intuitive problem. Note that this is the binomial distribution with $n=6,12,18$ and $p=1/6$. It is obvious that the mean of the binomial distribution is $np$, but less obvious that the median is also $np$ when $np$ is an integer. Knowing this, however, leads to the intuition for this problem.

The median divides the probability mass into equal parts. Adding the median mass and the right tail is bound to be at least 50% of the mass. If the mass at the median is large, then the sum of it and the right tail is bound to be larger. Obviously, the smaller $n$ is, the more concentrated the mass is at the median (and everywhere else). Hence, the sum of the right tail and the median mass is largest when $n=6$, as desired.

## 24

This problem reminds me of the *primary clustering* phenomenon in hashing—where large gaps are over-represented in some way, simply because they are larger.

## 30

Interestingly, the Poisson distribution alwas has higher mass on even numbers than the odds. The series itself is not difficult analytically. The intuitive reason is still a mystery to me.
