ML Paper/Essay|Summary
-|-
[Population Based Training of Neural Networks](https://arxiv.org/pdf/1711.09846.pdf)|Hyperparameter optimization/meta-learning similar to evolutionary strategies by discarding the least performant and copy-perturbing the most performant at each step, with applications in DL, RL, and even GANs.
[RenderNet: A deep convolutional network for differentiable rendering from 3D shapes](https://arxiv.org/pdf/1806.06575.pdf)|Neural network for different types of 3D rendering & shading, capable of single-image reconstruction (i.e. few-shot learning). Is it possible to perform pruning to create NN-based performant renderers?
[HoloGAN: Unsupervised learning of 3D representations from natural images](https://arxiv.org/pdf/1904.01326.pdf)|Training 3D representations via backprop on 2D renders. Is it possible to do few-shot learning?
[A New Interpretation of Information Rate](http://www.herrold.com/brokerage/kelly.pdf)|The famous Kelly criterion for risk management in maximizing log returns in the long run.
[An Impossibility Theorem for Clustering](https://www.cs.cornell.edu/home/kleinber/nips15.pdf)|Clustering algorithms cannot simultaneously satisfy three theoretical axioms of behavior. Consider which one we want to break. [See here for another summary](http://alexhwilliams.info/itsneuronalblog/2015/10/01/clustering2/).
[Invariant Risk Minimization](https://arxiv.org/pdf/1907.02893.pdf)|A way to enable better generalization to out-of-distribution data when training on a given distribution. Fundamental and theoretical.
[Model-Agnostic Meta-Learning for Fast Adaptation of Deep Networks](https://arxiv.org/pdf/1703.03400.pdf)|Task and model-agnostic method to optimize hyperparameters of models via a trainer model which performs backprop on average performance of child model over several unseen tasks.
[Learning Algorithms for Active Learning](https://arxiv.org/pdf/1708.00088.pdf)|Meta-learning the data selection for active learning based on only seeing the unlabeled data, useful for situations with high labeling cost.
[Snuba: Automating Weak Supervision to Label Training Data](http://www.vldb.org/pvldb/vol12/p223-varma.pdf)|Generate noisy labels for selected unlabeled data for further training by training on a smaller set of ground truth labels, outperforming heuristics of similar purpose in training the final model.
[Synthesizing Robust Adversarial Examples](https://arxiv.org/pdf/1707.07397.pdf)|Training 3D representations whose renders are all 2D adversarial examples. Student research group.
[Multi-Task Curriculum Transfer Deep Learning of Clothing Attributes](https://arxiv.org/pdf/1610.03670.pdf)|
[Imbalanced Deep Learning by Minority Class Incremental Rectification](https://arxiv.org/pdf/1804.10851.pdf)|
[Class Rectification Hard Mining for Imbalanced Deep Learning](https://arxiv.org/pdf/1712.03162.pdf)|
[When Deep Learning Met Code Search](https://arxiv.org/pdf/1905.03813.pdf)|
[Benign Overfitting in Linear Regression](https://arxiv.org/pdf/1906.11300v1.pdf)|
[Gated-SCNN: Gated Shape CNNs for Semantic Segmentation](https://arxiv.org/pdf/1907.05740.pdf)|
[A Baseline for 3D Multi-Object Tracking](https://arxiv.org/pdf/1907.03961.pdf)|
[Temporal Parallelization of Bayesian Filters and Smoothers](https://arxiv.org/pdf/1905.13002.pdf)|
[CNN-based synthesis of realistic high-resolution LiDAR data](https://arxiv.org/pdf/1907.00787.pdf)|
[Learning single-image 3D reconstruction by generative modeling of shape, pose and shading](https://arxiv.org/pdf/1901.06447.pdf)|
[GENESIS: Generative Scene Inference and Sampling with Object-Centric Latent Representations](https://arxiv.org/pdf/1907.13052.pdf)|
[ATOM: Accurate Tracking by Overlap Maximization](https://arxiv.org/pdf/1811.07628.pdf)|
[Fully-Convolutional Siamese Networks for Object Tracking](https://arxiv.org/pdf/1606.09549.pdf)|
[Fast Online Object Tracking and Segmentation: A Unifying Approach](https://arxiv.org/pdf/1812.05050.pdf)|
[Distractor-aware Siamese Networks for Visual Object Tracking](https://arxiv.org/pdf/1808.06048.pdf)|
[Introducing long-form question answering](https://ai.facebook.com/blog/longform-qa/)|
[ViLBERT: Pretraining Task-Agnostic Visiolinguistic Representations for Vision-and-Language Tasks](https://arxiv.org/pdf/1908.02265v1.pdf)|
[A Glimpse Far into the Future: Understanding Long-term Crowd Worker Quality](https://arxiv.org/pdf/1609.04855.pdf)|
[catch22: CAnonical Time-series CHaracteristics](https://arxiv.org/pdf/1901.10200.pdf)|
[SpatialSense: An Adversarially Crowdsourced Benchmark for Spatial Relation Recognition](https://arxiv.org/pdf/1908.02660.pdf)|
[Explainable AI for Trees: From Local Explanations to Global Understanding](https://arxiv.org/pdf/1905.04610.pdf)|
[A Unified Approach to Interpreting Model Predictions](http://papers.nips.cc/paper/7062-a-unified-approach-to-interpreting-model-predictions.pdf)|
[PointNet: Deep Learning on Point Sets for 3D Classification and Segmentation](https://arxiv.org/pdf/1612.00593.pdf)|
[Making Convolutional Networks Shift-Invariant Again](https://arxiv.org/pdf/1904.11486.pdf)|
[The Unreasonable Effectiveness of Deep Features as a Perceptual Metric](https://arxiv.org/pdf/1801.03924.pdf)|
[Split-Brain Autoencoders: Unsupervised Learning by Cross-Channel Prediction](https://arxiv.org/pdf/1611.09842.pdf)|
[Real-Time User-Guided Image Colorization with Learned Deep Priors](https://arxiv.org/pdf/1705.02999.pdf)|
[Unsupervised Deep Learning by Neighbourhood Discovery](https://arxiv.org/pdf/1904.11567.pdf)|
<https://arxiv.org/pdf/1911.05722.pdf>|
<https://arxiv.org/pdf/1911.04623.pdf>|

To read: <https://mp.weixin.qq.com/s/WOPGKz72S57TR6kD3nJkNw>.

HCI Paper/Essay|Summary
-|-
[Object Spreadsheets: A New Computational Model for End-User Development of Data-Centric Web Applications](https://sdg.csail.mit.edu/sites/default/files/documents/objsheets-onward2016.pdf)|Spreadsheets do not have strong or general data models like databases, and have trouble supporting changing hierarchal data. Object Spreadsheets allows for this expression by creation a spreadsheet-like web application with support for object-referencing formulas and hierarchal data.
[Mavo: Creating Interactive Data-Driven Web Applications by Authoring HTML](http://people.csail.mit.edu/karger/Papers/mavo.pdf)|Data-backed applications implemented via HTML with variable (possibly non-spreadsheet) sources allowing for CRUD operations and custom styling.
[Spreadsheet Driven Web Applications](https://dspace.mit.edu/bitstream/handle/1721.1/100512/Karger_-spreadsheet-driven-web-apps.pdf?sequence=1&isAllowed=y)|Users with basic HTML abilities and familiarity with spreadsheets should be able to create styled, spreadsheet-backed webpages with only minimal HTML coding experience.
[Up and Down the Ladder of Abstraction](http://worrydream.com/LadderOfAbstraction/)|

Chemistry Paper/Essay|Summary
-|-
Vertically-aligned nanostructures for electrochemical energy storage|
