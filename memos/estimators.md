<!-- emilia-snapshot-properties
Four commons estimators and properties
2023/05/29
utulek
emilia-snapshot-properties -->

# Four commons estimators and properties

May 30, 2023

## MSE

We have for estimator $\hat\Theta$ its mean squared error (MSE):

$$
\begin{aligned}
MSE(\hat\Theta)&=E[(\hat\Theta-\Theta)^2]\\
&=E[\hat\Theta^2]-2\Theta E[\hat\Theta]+\Theta^2\\
&=E[\hat\Theta^2]-E[\hat\Theta]^2+E[\hat\Theta]^2-2\Theta E[\hat\Theta]+\Theta^2\\
&=Var(\hat\Theta)+B(\hat\Theta)^2
\end{aligned}
$$

with bias $B(\hat\Theta)=E[\hat\Theta]-\Theta$. This is, of course, slightly counterintuitive—the variance of the estimator does not cancel out the bias, if they happen to be in the same direction.

As we will see later, having the MSE vanish as the number of samples is increased means that the estimator is necessarily consistent.

## Bias and consistency

Roughly, estimators are described by two sets of orthogonal adjectives:

1. Unbiased: the bias of the estimator, $B(\hat\Theta)$, equals $0$.
2. Consistent: for any $\epsilon>0$, we have
   $$\lim_{n\to\infty}P(|\hat\Theta_n-\Theta|\geq\epsilon)=0.$$
   That is, with more samples, the estimators get arbitrarily close to the true value.

Thus we have for examples of different types of estimators. For the population we choose distribution $U[0, 1]$. The following examples estimate the mean of the population based on a set of samples $X_i$ drawn from it.

**Biased and inconsistent**: The mean of the population is surely $\Theta=1/2$. We simply choose estimator $\hat\Theta=0$, which is a point distribution at point $x=0$.

**Unbiased and consistent** (a good estimator): We simply choose the sample mean $\bar X=(X_1+\cdots)/n$.

**Unbiased and inconsistent**: An unbiased estimator is easy to construct: simply a distribution spread 50% at $0$, and 50% at $1$ will do. For any $\epsilon<0.5$, the probability that anything estimated will be greater than that epsilon is exactly $1$, so it is inconsistent.

**Biased and consistent**: This is perhaps the most troubling of orthogonal combinations of adjectives. Let us consider an estimator for $\Theta$, the upper bound of a population with continuous distribution $U[0,\Theta]$. We take

$$\hat\Theta=\max(X_1,\ldots).$$

Notice this is simply distribution for $n$-th order statistic of $n$ samples. We recall from order statistics that

$$P(\hat\Theta=x)\propto {n\choose n-1}(1)f(x)F(x)^{n-1}(1-F(x))^0\propto Beta(n, 1)(x/\Theta).$$

In fact, this distribution is identical to the Beta distribution, scaled up $\Theta$ times to cover the entire support of the order statistic.

Then, the mean of the estimator distribution is proportional to the mean for the Beta distribution, at $n/(n+1)\cdot\Theta$. Of course, this is biased—intuitively, the estimator always approaches the true mean from the left. Alternatively, the true parameter $\Theta$ must obviously be greater than the $n$-th order statistic, so estimating exactly the that order statistic is almost certainly an underestimate, should there be any probability mass anywehre else.

This approach comes arbitrarily close to the true parameter, however, and for that we need two facts:

1. The mean, $E[\hat\Theta]=\frac{n}{n+1}\Theta$ gets arbitrarily close to $\Theta$.
2. The variance, same as the Beta distribution itself, potentially scaled with a constant, is $Var(\hat\Theta)\propto Var(Beta(n,1))=n/(n+1)^2/(n+2)$. Obviously, this goes to $0$. The variance may also be derived directly via the formula.

Hence, it is true that $\hat\Theta$ converges in probability to $\Theta$, and indeed, this estimator is consistent.

---

Intuitively, unbiased-ness is quite strong—it requires that for any $n$ number of samples, the estimator be unbiased, while consistency only requires convergence as $n\to\infty$.

Furthermore, notably, the convergence of the MSE of the estimator to $0$ as $n\to\infty$ also proves consistency. This is because the bias and the variance both must go to $0$ (speed irrelevant). This is nearly a tautology.<sup>[2]</sup>

## Methods of estimation<sup>[3]</sup>

In my studies I have seen no more than four types of estimators commonly employed. Consider observed data $X$ and estimator $\hat\Theta$ for the following discussions.

### Maximum likelihood (ML)

We have for some observations $X$ the likelihood of generating such $X$ based on some variable $\Theta$. Then, the following function (over variousvalues of $\Theta$)

$$P(X|\Theta)$$

must have a maximum at $\Theta=\hat\Theta$ which maximizes the likelihood of seeing $X$ under that schema. This is the maximum likelihood estimator.

To compute the ML estimator, usually one takes the likelihood function $L(\Theta)$ and maximizes it via derivatives. To make this easier, sometimes the log-likelihood is used, as $\log$ is monotonic.

### Method of moments (MoM)

### Maximum a posteriori (MAP)

The MAP estimator is very related to the ML estimator. Notably, for data $X$, it is the value $\hat\Theta$ which maximizes

$$P(\Theta|X)=P(X|\Theta)P(\Theta)/P(X)$$

which is the same value of $\hat\Theta$ which maximizes

$$P(\Theta|X)\propto P(X|\Theta)P(\Theta).$$

Should the a priori distribution $P(\Theta)$ be uniform, then the ML estimator, which maximizes $P(X|\Theta)$ is necessarily the same as the MAP estimator. Thus—MAP can be seen as a generalization on ML.

For example, in estimating the bias $p$ of a coin given $14$ flips of which $10$ resulted in heads, the ML estimator is obviously $\hat p_{ML}=10/14$, and because we have no prior distribution, the reasonable conjugate prior to choose here is the Beta distribution, which must mean that the prior is uniform. As such $\hat p_{MAP}=10/14$ as well.

### Minimum mean squared error (MMSE)<sup>[4]</sup>/Expected a posteriori (EAP)<sup>[5]</sup>

Consider again the distribution $\Theta|X$, defined over the support of $\Theta$: instead of choosing the mode of this distribution, which gives the MAP estimator, we choose the mean. This gives the MMSE estimator:

$$\hat\Theta_{MMSE}=E[\Theta|X].$$

Very conveniently, this estimator $\hat\Theta$ minimizes the mean squared error from the real statistic $\Theta$:

$$\hat\Theta=\min_\theta E[(\Theta-\theta)^2].$$

This may be verified directly via calculus and the use of the law of iterated expectations (LIE) near the end:

$$E[X]=E[E[X|Y]].$$

## Example: stimator for uniform RV

We have for some population generated from $U[0,\Theta]$ samples $X_1,\ldots,X_n$. The estimator $\hat\Theta=\max(X_i)=M$ is trivially the ML estimator. That is, the likelihood of achieving samples $X_i$ is

$$L(\hat\Theta)=(1/\hat\Theta)^n\text{ when }\hat\Theta\geq\max(X_i)$$

which is minimized when $dL/d\hat\Theta=0=n\ln\hat\Theta(1/\hat\Theta)^{n-1}\implies\hat\Theta=-\infty$, but considering the lower bound on likelihood, we must have $\hat\Theta=\max(X_i)$.

Before moving forward, let us also compute the unbiased MMSE estimator. We have from Bayes’ rule

$$P(\Theta|X)\propto P(X|\Theta)P(\Theta).$$

But of course we have no reason to assume any particular prior $P(\Theta)$ so we let it be uniform, and thus

$$P(\Theta|X)(\theta)\propto P(X|\Theta)(\theta)=(1/\theta)^n.$$

Let us quickly compute the scaling constant $c$:

$$\begin{aligned}
\int_M^\infty(1/\theta)^nd\theta&=1/c\\
1/c&=\Big[\frac{1}{-n+1}\theta^{-n+1}\Big]_M^\infty\\
&=\frac{M^{1-n}}{n-1}\\
\implies c&=\frac{n-1}{M^{1-n}}\\
P(\Theta|X)(\theta)&=\frac{(n-1)M^{n-1}}{\theta^n}.
\end{aligned}$$

So now we can directly compute the expected value:

$$\begin{aligned}
E[P(\Theta|X)]&=\hat\Theta_{MMSE}\\
&=\int_M^\infty\frac{(n-1)M^{n-1}}{\theta^n}\theta d\theta\\
&=\Big[\frac{1}{-n+2}\theta^{-n+2}(n-1)M^{n-1}\Big]_M^\infty\\
&=\frac{n-1}{n-2}\frac{M^{n-1}}{M^{n-2}}=\frac{n-1}{n-2}M.
\end{aligned}$$

---

References:

1. <https://stats.stackexchange.com/questions/88800/intuitive-reasoning-behind-biased-maximum-likelihood-estimators>.
2. <https://stats.stackexchange.com/questions/280684/intuitive-understanding-of-the-difference-between-consistent-and-asymptotically>.
3. <https://math.mit.edu/~rmd/650/estimation.pdf>.
4. <https://www.probabilitycourse.com/chapter9/9_1_5_mean_squared_error_MSE.php>.
5. <https://towardsdatascience.com/mle-map-and-bayesian-inference-3407b2d6d4d9>.
