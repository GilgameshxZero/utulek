# Utuilities for visualization.

def show_img_ds(X, Y, lnames=None, samples=[[None] * 5] * 2, show_scale=2):
    """Plot a grid of images titled with their label/label names."""
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
                      "\"{}\" ({}, {})".format(lnames[Y[idx]], Y[idx], idx))
            plt.axis("off")
    plt.tight_layout()

def show_img_ds_cmp(X, Ys, lnames=None, samples=[[None] * 5] * 2, show_scale=2):
    """
    Plot a grid of images titled with their label/label names. Images where the
    Ys (ground truth vs. prediction) differ are colored differently (inverted).
    """
    import numpy as np
    import matplotlib.pyplot as plt

    c_samples = (len(samples), len(samples[0]))
    plt.figure(figsize=(c_samples[1] * show_scale, c_samples[0] * show_scale))
    for i in range(c_samples[0]):
        for j in range(c_samples[1]):
            idx = np.random.randint(len(Ys[0])) if samples[i][j] is None else \
                samples[i][j]
            is_wrong = Ys[0][idx] != Ys[1][idx]
            plt.subplot(*c_samples, i * c_samples[1] + j + 1)
            plt.imshow(np.clip(X[idx], 0, 1),
                       cmap="binary" if is_wrong else "gray")
            titles = ["{} ({})".format(Ys[k][idx], idx) if lnames is None else
                      "\"{}\" ({}, {})".format(
                          lnames[Ys[k][idx]], Ys[k][idx], idx)
                      for k in (0, 1)]
            plt.title(titles[0] + " / " + titles[1] if is_wrong else titles[0])
            plt.axis("off")
    plt.tight_layout()


def show_categ_heatmap(row_labels, col_labels, vals, show_scale=0.8):
    """Plot a heatmap for categorical data."""
    import numpy as np
    import matplotlib.pyplot as plt

    plt.figure(figsize=(len(col_labels) * show_scale,
               len(row_labels) * show_scale))
    plt.imshow(vals)
    plt.xticks(np.arange(len(col_labels)), col_labels)
    plt.yticks(np.arange(len(row_labels)), row_labels)
    plt.tick_params('x', labelrotation=45)
    for i in range(len(row_labels)):
        for j in range(len(col_labels)):
            plt.annotate("{:.2f}".format(vals[i][j]), (j, i), ha="center")
    plt.tight_layout()
