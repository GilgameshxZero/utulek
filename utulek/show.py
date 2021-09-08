# Utuilities for visualization.

def show_img_ds(X, Y, lnames=None, samples=[[None] * 5] * 2, show_scale=2):
    import numpy as np
    import matplotlib.pyplot as plt

    c_samples = (len(samples), len(samples[0]))
    plt.figure(figsize=(c_samples[1] * show_scale, c_samples[0] * show_scale))
    for i in range(c_samples[0]):
        for j in range(c_samples[1]):
            idx = np.random.randint(len(Y)) if samples[i][j] is None else \
                samples[i][j]
            plt.subplot(*c_samples, i * c_samples[1] + j + 1)
            plt.imshow(np.clip(X[idx], 0, 1), cmap="gray")
            plt.title("{} ({})".format(Y[idx], idx) if lnames is None else
                      "\"{}\" / {} ({})".format(lnames[Y[idx]], Y[idx], idx))
            plt.axis("off")
    plt.tight_layout()
