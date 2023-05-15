In my studies I have encountered commonly only 4 discrete and 5 continuous distributions.

### Uniform (discrete and continuous)

There is little to be said of the uniform distributions, except that they are the most basic of the distributions. In fact, the uniform distribution forms the basis of scenarios which help derive the following distributions.

### Normal

### Geometric, Exponential, Poisson, and Gamma

Consider this scenario:

> A bakery sees on average $\lambda<1$ customers per minute, whose arrival times are distributed uniformly.

Questions on this scenario lead to the four distributions.

**Geometric**: One way this scenario could be fulfilled is by the following underlying mechanic: with probability $\lambda$, one customer arrives in each minute. Otherwise, no customer arrives in that minute. In which minute will the next customer arrive?

We take PDF $P(x)$ as the probability that the next customer arrives in the $x$-th minute. It must be then that for $x-1$ minutes prior, no customers arrive:

$$P(x)=(1-\lambda)^{x-1}\lambda.$$

**Exponential**: Dropping the geometric assumption earlier, how many minutes will pass before the next customer arrives?

The Exponential distribution is simply the continuous analogue of the geometric. Should we divide a minute into $n$ moments, we would expect $\lambda/n$ customers per moment. Alternatively, with large enough $n$, we expect a customer with probability $\lambda/n$.
For $x$ minutes to pass before the next customer arrives, then, we would see $xn$ moments with no customer and a final moment with a customer:

$$P(x)\approx \lim_{n\to\infty}(1-\lambda/n)^{xn}(\lambda/n)\propto e^{-\lambda x},$$

recalling that $e^x=\lim_{n\to\infty}(1+x/n)^n$.

Indeed, $P(x)$ must sum to $1$, and we have

$$\int_0^\infty e^{-\lambda x}=\Big[-e^{-\lambda x}/\lambda\Big]_0^\infty=1/\lambda$$

so $P(x)$ must equal $\lambda e^{-\lambda x}$.

**Poisson**: Suppose now that $\lambda \geq 1$, for ease of notation. How many customers will arrive in a minute?

Again, the approach of taking number of moments $n\to\infty$ is fruitful. We have for large $n$ moments per minute that with probability $\lambda/n$ a customer is slated to arrive. So for $x$ customers to arrive in the minute, $x$ of those $n$ moments must have a customer, and the rest must not.

$$\begin{aligned}
P(x)&= \lim_{n\to\infty}{n\choose x}(\lambda/n)^x(1-\lambda/n)^{n-x}\\
&=\lim_{n\to\infty}\frac{n!}{x!(n-x)!}(\lambda/n)^x(1-\lambda/n)^{-x}e^{-\lambda}\\
&=e^{-\lambda}\lim_{n\to\infty}\frac{n!}{x!(n-x)!}(\frac{\lambda}{n}\cdot\frac{n}{n-\lambda})^x\\
&=e^{-\lambda}\frac{\lambda^x}{x!}\lim_{n\to\infty}\frac{n\cdot(n-1)\cdots(n-x+1)}{(n-\lambda)^x}\\
&=\lambda^x e^{-\lambda}/x!.
\end{aligned}$$

**Gamma**: How many minutes will pass before the next $n$ customers arrive?

Immediately, this is simply the sum of $n$ exponential distributions, and by its name, must invoke the Gamma function. To intuit the Gamma function, consider how it must extend the factorial function with an integral. Certainly, this integral must be evaluated by cascading integration by parts:

$$\Gamma(n+1)= \int_0^\infty x^ne^{-x} dx.$$

Perhaps the $-x$ in the exponent is unintuitive. But consider this: all the *trash* terms in the integration by parts must vanish, leaving only the factorial term remaining. $x^n$ vanishes the $0$ limit, so the $e$ term must vanish the $\infty$ limit. Hence, the exponent must be negative.

Alternatively, consider the weak Sterling’s approximation derived earlier:

$$\Gamma(n+1)\approx f(n)=n!\approx (n/e)^n.$$

Given that $\Gamma$ must be an integral, the inside of the integral must then be similar to the derivative of $(n/e)^n$. Though weakly linked, there is indeed some similarity in form between $x^ne^{-x}dx$ and $(n/e)^n$.

### Binomial and Beta

There is little to be said of the Binomial: it is too common, and it is easily derived.

To derive the Beta distribution, consider the following scenario.

> A possibly unfair coin is flipped $14$ times, and it comes up heads $4$ times and tails $10$ times. Suppose if you guess the bias of the coin, you win $1$, and otherwise win nothing. What do you guess?

As we are penalized only on the wrongness of our guess, it is correct to use the maximum likelihood estimator on the coin bias. As one may expect, the most likely bias of the coin as a result of observing the $14$ flips is that it comes up heads $4/14$ times.

A (more natural) penalty invokes the Beta distribution.

> Suppose now, instead, that you will be given $1 if the next flip comes up heads and nothing otherwise. How much are you willing to pay to play this game?

And thus we are asked for the likelihood of a certain bias, given our observations, and the mean of this distribution. This is the mean of the beta distribution $Beta(\alpha=4,\beta=10)$.
