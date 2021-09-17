![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-21-37.png)
*Learning rate is typically scaled 0.1x per 50 epochs.*

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-22-04.png)

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-22-23.png)

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-22-53.png)

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-25-18.png)
*Small Hessian at initialization by design; Hessian then hovers around 20 (asymptote)?*

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-26-52.png)
*Y-axis is the spectral norm of the Hessian.*

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-30-25.png)
*As cross-entropy loss decreases from ~2.3 (for CIFAR10). At the beginning, we have picked the learning rate well—the optimization doesn’t diverge at the beginning, but the learning rate does not fit the curvature at the end.*

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-13-52-58.png)

![](jastrzebski-sgd-optimizers.md-assets/2021-09-17-14-00-21.png)
*Right plot is largest/smallest eigenvalue.*
