<!-- emilia-snapshot-properties
PDF transformations
2023/05/26
utulek
emilia-snapshot-properties -->

# PDF transformations

May 26, 2023

Consider the behavior of some function $f$ under change of variable $y$, which we call $g$. We have

$$\begin{aligned}
g(y(x))&=f(x)\text{ by definition}\\
\implies g(x)&=f(y^{-1}(x))\\
\implies g'(x)&=f'(z(x))z'(x)\text{, with }z=y^{-1}\\
\end{aligned}$$

Importantly, recall that the PDF is defined as the rate of change of the CDF. When PDF $f'$ undergoes change of variable $y'$, its altered PDF $g'$ is correspondingly multiplied by the *Jacobian factor*, which has its parallels for random vectors $\mathbf{x}$.

For example, suppose for RV $X$ we have its PDF $f'$ and a change of variable $y(X)=X^2\implies y^{-1}(Y)=z(Y)=\sqrt{Y}\implies z'(Y)=1/(2\sqrt{Y})$. The new CDF is then $g(x)=f(y^{-1}(x))=f(z(x))$ and thus the PDF is

$$g'(x)=(f'(\sqrt{x})+f'(-\sqrt{x}))/(2\sqrt{x})$$

as desired.

The mode, or maximum likelihood estimator, is similarly transformed, and due to the properties of the second derivative, may not remain trivial. However, should the transformation be linear, the new maximum likelihood estimator $\hat y$ is simply $y(\hat x)$, the trivial one.

---

References:

1. <https://stats.stackexchange.com/questions/236840/different-probability-density-transformations-due-to-jacobian-factor>.
2. <https://stats.stackexchange.com/questions/14483/intuitive-explanation-for-density-of-transformed-variable>.
