### $LDU$

Gaussian elimination gives this decomposition with $L$ lower triangular, $D$ diagonal, and $U$ upper triangular, as long as $A$ has full rank.

### $QR$

Acquired by the Gram-Schmidt process, which essentially gives orthonormal $Q$ by repeatedly taking out non-orthogonal components of each column, and tracking in upper-triangular $R$. The normalization of each column may be tracked by the diagonal values in $R$.

### $X\Lambda X^{-1}$ (Eigenvalue)

$X$ has columns the eigenvectors of $A$ and $\Lambda$ is diagonal with the corresponding eigenvalues, in order. 

### $U\Sigma V^T$ (SVD)

Notably, $U,T$ are orthonormal matrices generated from $A^TA$ and $AA^T$ respectively, which renders $V^T=V^{-1}$.
