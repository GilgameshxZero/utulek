# Utuilities for visualization.

from tqdm.auto import tqdm


class TqdmByteTransfer(tqdm):
    """
    Tqdm progress bar with sensible defaults for file transfers.
    """
    
    def __init__(self):
        super().__init__(self, unit='B', unit_scale=True, unit_divisor=1024)
        
    
    def up_to(self, block_idx, block_size, total_size):
        if total_size is not None:
            self.total = total_size
        return self.update(block_idx * block_size - self.n)


def show_img_ds(X, Y, pred=None, labels=None, shape=None, samples=None, scale=2):
    """
    Plot a grid of images titled with their label/label names. Images where the
    Ys (ground truth vs. prediction) differ are colored differently.
    """
    import numpy as np
    import matplotlib.pyplot as plt

    if shape is None:
        shape = (len(samples), len(samples[0]))

    if samples is None:
        # Take unique random indices.
        samples = np.random.choice(len(X), shape, replace=False)

    if labels is None:
        labels = [i for i in range(Y.argmax())]

    plt.figure(figsize=(shape[1] * scale, shape[0] * scale))
    for i in range(shape[0]):
        for j in range(shape[1]):
            idx = samples[i][j]
            plt.subplot(*shape, i * shape[1] + j + 1)
            plt.imshow(np.clip(X[idx], 0, 1), cmap="gray")
            plt.title("{} ({}) [{}]".format(
                labels[Y[idx]], Y[idx], idx) if pred is None else
                "{} ({}) : {} ({}) [{}]".format(
                labels[Y[idx]], Y[idx], labels[pred[idx]], pred[idx], idx))
            plt.axis("off")
            if pred is not None and Y[idx] != pred[idx]:
                plt.axhspan(0, len(X[idx]), color="#ff000060")
    plt.tight_layout()


def show_categ_heatmap(row_labels, col_labels, vals, scale=0.8):
    """Plot a heatmap for categorical data."""
    import numpy as np
    import matplotlib.pyplot as plt

    plt.figure(figsize=(len(col_labels) * scale,
               len(row_labels) * scale))
    plt.imshow(vals)
    plt.xticks(np.arange(len(col_labels)), col_labels)
    plt.yticks(np.arange(len(row_labels)), row_labels)
    plt.tick_params('x', labelrotation=45)
    for i in range(len(row_labels)):
        for j in range(len(col_labels)):
            plt.annotate("{:.2f}".format(vals[i][j]), (j, i), ha="center")
    plt.tight_layout()
