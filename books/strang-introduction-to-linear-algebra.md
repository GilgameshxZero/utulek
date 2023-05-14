## Orthonormal matrices

A matrix $A$

$$A=\begin{bmatrix}
\vdots & \vdots & & \vdots\\
\mathbf{a_1} & \mathbf{a_2} & \cdots & \mathbf{a_n}\\
\vdots & \vdots & & \vdots
\end{bmatrix}$$

is orthonormal iff

1. Columns $\mathbf{a_i}$ have unit norm: $\mathbf{a_i}^T\mathbf{a_i}=1$.
2. Columns $\mathbf{a_i}$ are orthogonal with each other: $\mathbf{a_i}^T\mathbf{a_j}=0$ for $i\neq j$.
3. Thus, it must be that $A^TA=I$, and similarly $AA^T=I$. Recall that multiplication from the left acts on the rows, and multiplication from the right acts on the columns.
4. Hence, $A^T=A^{-1}$. Intuitively, $A^T$ solves

	 $$A\mathbf{x}=\mathbf{b}\iff \mathbf{x}=A^T\mathbf{b}$$

	 by taking $\mathbf{x}$ as the dot products $\mathbf{a_i}^T\mathbf{b}$. This is simply the projection of $\mathbf{b}$ onto orthonormal basis $A$, which necessarily solves $A\mathbf{x}=\mathbf{b}$.

## Projections

For projection $\mathbf{p}$ of $\mathbf{b}$ onto line $\mathbf{a}$ we have with *scaling factor* $\hat{x}$

$$\mathbf{p}=\mathbf{a}\hat{x}=P\mathbf{b}.$$

Recall that dot product $\mathbf{a}^T\mathbf{b}$ is the norm of the projection of $\mathbf{b}$ onto $\mathbf{a}$, scaled by the norm of $\mathbf{a}$. Hence, the real norm of the projection can be found by scaling back:

$$\hat{x}=\mathbf{a}^T\mathbf{b}/\mathbf{a}^T\mathbf{a}$$

giving projection matrix $P$ onto a line as

$$P=\mathbf{a}(\mathbf{a}^T\mathbf{a})^{-1}\mathbf{a}^T.$$

Projections onto orthonormal column space $A$ follow a similar form due to similar reasoning:

$$P=\mathbf{A}(\mathbf{A}^T\mathbf{A})^{-1}\mathbf{A}^T.$$

$A$ needs not be orthonormal.

The order here obviously matters now. We have intuitively that $P^2=P$, but also that $P^T=P$ (symmetry), from the following reasoning: for vectors $\mathbf{v},\mathbf{w}$, we have their dot product in space $A$ as

$$(P\mathbf{v})\cdot\mathbf{w}=\mathbf{v}\cdot(P\mathbf{w})\implies P\mathbf{v}=\mathbf{v}P.$$

Recalling that multiplying $P$ from the right gives linear combinations of its columns, and multiplying from the left gives linear combinations of its rows. Hence, the rows and columns of $P$ must be equal and in the same order, and $P$ must be symmetric.

Then, the only order of the terms above which also preserve symmetry is the $P$ given.

## Least-squares

Estimating $A\hat{\mathbf{x}}=\mathbf{b}$ is akin to projecting $\mathbf{b}$ onto $A$. A solvable equation means that $\mathbf{b}$ lies in the column space of $A$.

A linear least-squares regression line (LSRL) of points $(t_1,b_1),(t_2,b_2),\ldots$ is then a projection of $\mathbf{b}$ onto a space $A$ spanned by the $0$-th order  approximation column $(1,\ldots,1)^T$ and the $1$-st order approximation column $(t_1,\ldots,t_m)^T$.

## The big 4 decompositions

### $LDU$

Gaussian elimination gives this, as long as $A$ has full rank.

### $QR$

Acquired by the Gram-Schmidt process, which essentially gives orthonormal $Q$ by repeatedly taking out non-orthogonal components of each column, and tracking in upper-triangular $R$.

### $X\lambda X^{-1}$ (Eigenvalue)

### $U\Sigma V^T$ (SVD)

## Cramer’s rule

$$A^{-1}=C^T/\det A$$

where $C$ is the cofactor matrix, of determinants of each of the submatrices of $A$ with some row $i$ and column $j$ removed, scaled by $-1$ if *odd* on a checkerboard.
