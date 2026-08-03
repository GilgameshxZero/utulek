# Reverse-engineering implicit regularization due to large learning rates in deep learning

Stanisław Jastrzębski
September 17, 2021
<https://rosanneliu.com/dlctfs/dlct_210917.pdf>

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-21-37.avif)
*Learning rate is typically scaled 0.1x per 50 epochs.*

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-22-04.avif)

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-22-23.avif)

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-22-53.avif)

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-25-18.avif)
*Small Hessian at initialization by design; Hessian then hovers around 20 (asymptote)?*

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-26-52.avif)
*Y-axis is the spectral norm of the Hessian.*

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-30-25.avif)
*As cross-entropy loss decreases from ~2.3 (for CIFAR10). At the beginning, we have picked the learning rate well—the optimization doesn’t diverge at the beginning, but the learning rate does not fit the curvature at the end.*

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-13-52-58.avif)

![](2021_09_17-jastrzebski_dlct_sgd_optimizers.md.assets/2021-09-17-14-00-21.avif)
*Right plot is largest/smallest eigenvalue.*
