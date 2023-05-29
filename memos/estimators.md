<!-- emilia-snapshot-properties
Estimators
2023/05/29
utulek
emilia-snapshot-properties -->

# Estimators

May 29, 2023

## MSE

We have for estimator $\hat\Theta$ its mean squared error (MSE):

$$
\begin{aligned}
MSE(\hat\Theta)&=E[(\hat\Theta-\theta)^2]\\
&=E[\hat\Theta^2]-2\theta E[\hat\Theta]+\theta^2\\
&=E[\hat\Theta^2]-E[\hat\Theta]^2+E[\hat\Theta]^2-2\theta E[\hat\Theta]+\theta^2\\
&=Var(\hat\Theta)+B(\hat\Theta)^2
\end{aligned}
$$

with bias $B(\hat\Theta)=E[\hat\Theta]-\theta$. This is, of course, slightly counterintuitive—the variance of the estimator does not cancel out the bias, if they happen to be in the same direction.

## Bias and consistency

Roughly, estimators are described by two sets of orthogonal adjectives:

1. Unbiased: the bias of the estimator, $B(\hat\Theta)$, equals $0$.
2. Consistent: for any $\epsilon>0$, we have
   $$\lim_{n\to\infty}P(|\hat\Theta_n-\theta|\geq\epsilon)=0.$$
   That is, with more samples, the estimators get arbitrarily close to the true value.

Thus we have for examples of different types of estimators. For the population we choose distribution $U[0, 1]$. The following examples estimate the mean of the population based on a set of samples $X_i$ drawn from it.

**Biased and inconsistent**: The mean of the population is surely $\theta=1/2$. We simply choose estimator $\hat\Theta=0$, which is a point distribution at point $x=0$.

**Unbiased and consistent** (a good estimator): We simply choose the sample mean $\bar X=(X_1+\cdots)/n$.

**Unbiased and inconsistent**: An unbiased estimator is easy to construct: simply a distribution spread 50% at $0$, and 50% at $1$ will do. For any $\epsilon<0.5$, the probability that anything estimated will be greater than that epsilon is exactly $1$, so it is inconsistent.

**Biased and consistent**: This is perhaps the most troubling of orthogonal combinations of adjectives. Let us consider an estimator for $\theta$, the upper bound of a population with continuous distribution $U[0,\theta]$. We take

$$\hat\Theta=\max(X_1,\ldots).$$

Notice this is simply distribution for $n$-th order statistic of $n$ samples. We recall from order statistics that

$$P(\hat\Theta=x)\propto {n\choose n-1}(1)f(x)F(x)^{n-1}(1-F(x))^0\propto Beta(n, 1)(x/\theta).$$

In fact, this distribution is identical to the Beta distribution, scaled up $\theta$ times to cover the entire support of the order statistic.

Then, the mean of the estimator distribution is proportional to the mean for the Beta distribution, at $n/(n+1)\cdot\theta$. Of course, this is biased—intuitively, the estimator always approaches the true mean from the left. Alternatively, the true parameter $\theta$ must obviously be greater than the $n$-th order statistic, so estimating exactly the that order statistic is almost certainly an underestimate, should there be any probability mass anywehre else.

This approach comes arbitrarily close to the true parameter, however, and for that we need two facts:

1. The mean, $E[\hat\Theta]=\frac{n}{n+1}\theta$ gets arbitrarily close to $\theta$.
2. The variance, same as the Beta distribution itself, potentially scaled with a constant, is $Var(\hat\Theta)\propto Var(Beta(n,1))=n/(n+1)^2/(n+2)$. Obviously, this goes to $0$. The variance may also be derived directly via the formula.

Hence, it is true that $\hat\Theta$ converges in probability to $\theta$, and indeed, this estimator is consistent.

---

Intuitively, unbiased-ness is quite strong—it requires that for any $n$ number of samples, the estimator be unbiased, while consistency only requires convergence as $n\to\infty$.

Furthermore, notably, the convergence of the MSE of the estimator to $0$ as $n\to\infty$ also proves consistency. This is because the bias and the variance both must go to $0$ (speed irrelevant). This is nearly a tautology.<sup>[2]</sup>

---

References:

1. <https://stats.stackexchange.com/questions/88800/intuitive-reasoning-behind-biased-maximum-likelihood-estimators>.
2. <https://stats.stackexchange.com/questions/280684/intuitive-understanding-of-the-difference-between-consistent-and-asymptotically>.
