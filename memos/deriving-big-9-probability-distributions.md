In my studies I have encountered commonly only 4 discrete and 5 continuous distributions.

### Uniform (discrete and continuous)

There is little to be said of the uniform distributions, except that they are the most basic of the distributions. In fact, the uniform distribution forms the basis of scenarios which help derive the following distributions.

### Normal

The Normal distribution is forced by two requirements.

1. Its joint distribution, $P_{X_1,X_2}(x_1,x_2)$ sees independent $X_1,X_2$.
2. $P_{X_1,X_2}$ is also rotationally symmetric.

### Geometric, Exponential, Gamma, and Poisson

Consider this scenario:

> A bakery sees on average $\lambda<1$ customers per minute, whose arrival times are distributed uniformly.

Questions on this scenario lead to the four distributions.

**Geometric**: One way this scenario could be fulfilled is by the following underlying mechanic: with probability $\lambda$, one customer arrives in each minute. Otherwise, no customer arrives in that minute. In which minute will the next customer arrive?

We take PDF $P(x)$ as the probability that the next customer arrives in the $x$-th minute. It must be then that for $x-1$ minutes prior, no customers arrive:

$$P(x)=(1-\lambda)^{x-1}\lambda.$$

It is self-evident that the mean should be $1/\lambda$. The variance is not easy to compute, but is $(1-\lambda)/\lambda^2$, and is had by separating $E[X^2]=E[X(X-1)]+E[X]$.

**Exponential**: Dropping the geometric assumption earlier, how many minutes will pass before the next customer arrives?

The Exponential distribution is simply the continuous analogue of the geometric. Should we divide a minute into $n$ moments, we would expect $\lambda/n$ customers per moment. Alternatively, with large enough $n$, we expect a customer with probability $\lambda/n$.
For $x$ minutes to pass before the next customer arrives, then, we would see $xn$ moments with no customer and a final moment with a customer:

$$P(x)\approx \lim_{n\to\infty}(1-\lambda/n)^{xn}(\lambda/n)\propto e^{-\lambda x},$$

recalling that $e^x=\lim_{n\to\infty}(1+x/n)^n$.

Indeed, $P(x)$ must sum to $1$, and we have

$$\int_0^\infty e^{-\lambda x}=\Big[-e^{-\lambda x}/\lambda\Big]_0^\infty=1/\lambda$$

so $P(x)$ must equal $\lambda e^{-\lambda x}$.

As the continuous analogue of the geometric, the mean should remain the same, at $1/\lambda$. The variance is computed by a double integration by parts, and is $1/\lambda^2$.

**Gamma**: How many minutes will pass before the next $n$ customers arrive?

Immediately, this is simply the sum of $n$ exponential distributions, and by its name, must invoke the Gamma function. To intuit the Gamma function, consider how it must extend the factorial function with an integral. Certainly, this integral must be evaluated by cascading integration by parts:

$$\Gamma(n+1)= \int_0^\infty x^ne^{-x} dx.$$

Perhaps the $-x$ in the exponent is unintuitive. But consider this: all the *trash* terms in the integration by parts must vanish, leaving only the factorial term remaining. $x^n$ vanishes the $0$ limit, so the $e$ term must vanish the $\infty$ limit. Hence, the exponent must be negative.

Alternatively, consider the weak Sterling’s approximation derived earlier:

$$\Gamma(n+1)\approx f(n)=n!\approx (n/e)^n.$$

Given that $\Gamma$ must be an integral, the inside of the integral must then be similar to the derivative of $(n/e)^n$. Though weakly linked, there is indeed some similarity in form between $x^ne^{-x}dx$ and $(n/e)^n$.

To intuit the PDF $P_G(x)$ of the Gamma distribution, we take first the sum of two exponential RVs with PDF $P(x)$:

$$\begin{aligned}
&\mathbb{P}(Gamma(n=2,\lambda)=x)=P_G(x)\\
&\propto\int_{i=0}^x P(i)P(x-i)di\\
&=\int_{i=0}^x \lambda^2 e^{-\lambda i}e^{-\lambda(x-i)} di\\
&=\lambda^2\Big[e^{-\lambda x}i\Big]_{i=0}^x\\
&=\lambda^2 e^{-\lambda x}x\\
&=\lambda^n x^{n-1}e^{-\lambda x}
\end{aligned}$$

of which the final line may be inferred from the general form on the line prior. It remains to scale this PDF to sum to unity. To do this, of course, we invoke the $\Gamma$ function, with a clever substitution $y=\lambda x$:

$$\begin{aligned}
P_G(x)&=\lambda^n (y/\lambda)^{n-1}e^{-y}\\
\int_{x=0}^\infty P_G(x)dx&=\int_{x=0}^\infty y^ne^{-y}(1/x)dx\\
&=\int_{y=0}^\infty y^ne^{-y}(1/x)(1/\lambda)dy\\
&=\int_{y=0}^\infty y^{n-1}e^{-y}dy\\
&=\Gamma(n).
\end{aligned}$$

And thus we scale the proportional PDF earlier by $1/\Gamma(n)$:

$$P_G(x)=\lambda^nx^{n-1}e^{-\lambda x}/\Gamma(n).$$

In other literature, you may see the variables $n$ replaced with $\alpha$ and $\lambda$ replaced with $\beta$.

As the Gamma distribution is the sum of i.i.d. exponential distributions, its mean is necessarily $n/\lambda$ and its variance $n/\lambda^2$.

**Poisson**: Suppose now that $\lambda \geq 1$, for ease of notation. How many customers will arrive in a minute?

Again, the approach of taking number of moments $n\to\infty$ is fruitful. We have for large $n$ moments per minute that with probability $\lambda/n$ a customer is slated to arrive. So for $x$ customers to arrive in the minute, $x$ of those $n$ moments must have a customer, and the rest must not.

$$\begin{aligned}
P(x)&= \lim_{n\to\infty}{n\choose x}(\lambda/n)^x(1-\lambda/n)^{n-x}\\
&=\lim_{n\to\infty}\frac{n!}{x!(n-x)!}(\lambda/n)^x(1-\lambda/n)^{-x}e^{-\lambda}\\
&=e^{-\lambda}\lim_{n\to\infty}\frac{n!}{x!(n-x)!}(\frac{\lambda}{n}\cdot\frac{n}{n-\lambda})^x\\
&=e^{-\lambda}\frac{\lambda^x}{x!}\lim_{n\to\infty}\frac{n\cdot(n-1)\cdots(n-x+1)}{(n-\lambda)^x}\\
&=\lambda^x e^{-\lambda}/x!.
\end{aligned}$$

By definition, the mean is $\lambda$. The variance, perhaps surprisingly, is also $\lambda$. The intuition to this lies in the binomial distribution: that when $n$ intervals are used, the mean is $n(\lambda/n)=\lambda$, but the variance is $n(\lambda/n)(1-\lambda/n)$, but $1-\lambda/n$ vanishes to unity as $n$ goes to $\infty$, and so the variance is the same as the mean, at $\lambda$.

### Binomial and Beta

There is little to be said of the Binomial: it is too common, and it is easily derived. The mean is $np$, and the variance $np(1-p)$, which is useful in gaining intuition for the Poisson distribution.

To derive the Beta distribution, consider the following scenario.

> A possibly unfair coin is flipped $14$ times, and it comes up heads $10$ times and tails $4$ times. Suppose if you guess the bias of the coin, you win $1$, and otherwise win nothing. What do you guess?

As we are penalized only on the wrongness of our guess, it is correct to use the maximum likelihood estimator on the coin bias. As one may expect, the most likely bias of the coin as a result of observing the $14$ flips is that it comes up heads $10/14$ times.

A (more natural) penalty invokes the Beta distribution.

> Suppose now, instead, that you will be given $1 if the next flip comes up heads and nothing otherwise. How much are you willing to pay to play this game?

And thus we are asked for the likelihood of a certain bias, given our observations, and the mean of this distribution. This is the mean of the Beta distribution $Beta(\alpha=10,\beta=4)$.

---

References:

1. <https://www.behind-the-enemy-lines.com/2008/01/are-you-bayesian-or-frequentist-or.html>: A discussion about differences between Bayesian and frequentist interpretations, which leads naturally into the intuition for the Beta distribution.
