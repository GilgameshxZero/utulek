## $A=LDU$

Gaussian elimination gives this decomposition with $L$ lower triangular, $D$ diagonal, and $U$ upper triangular, as long as $A$ has full rank.

## $A=QR$

Acquired by the Gram-Schmidt process, which essentially gives orthonormal $Q$ by repeatedly taking out non-orthogonal components of each column, and tracking in upper-triangular $R$. The normalization of each column may be tracked by the diagonal values in $R$.

Let

$$A=\begin{bmatrix}
A_1 & A_2 & \cdots & A_n
\end{bmatrix}=QR=\begin{bmatrix}
Q_1\\
Q_2\\
\vdots\\
Q_n\\
\end{bmatrix}R$$

with the definition that $Q$ is the orthonormal decomposition of $A$. We have then, by definition,

$$Q_1=A_1/||A_1||.$$

Secondly, we compute the projection of $A_2$

## $A=X\Lambda X^{-1}$ (Eigenvalue)

$X$ has columns the eigenvectors of $A$ and $\Lambda$ is diagonal with the corresponding eigenvalues, in order. 

## $A=U\Sigma V^T$ (SVD)

Notably, $U,T$ are orthonormal matrices generated from $A^TA$ and $AA^T$ respectively, which renders $V^T=V^{-1}$. Intuitively, the first few terms of the SVD are those with the highest influence on the rank of $A$—and thus a truncated SVD is known as PCA.

## Cholesky
