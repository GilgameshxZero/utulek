# Utilities for fetching from the network.

def fetch_tfds_img_ds(name, split):
    import operator
    import numpy as np
    import tensorflow_datasets as tfds

    X, Y = operator.itemgetter("image", "label")(
        tfds.as_numpy(tfds.load(name=name, split=split,
                                data_dir=".data/", batch_size=-1)))
    X = X.astype(np.float64)

    # Normalize to [0, 1].
    X /= np.max(X)
    return X, Y


def fetch_tfds_img_ds_labels(name):
    import tensorflow_datasets as tfds

    _, info = tfds.load(name=name, split="train[:1]", data_dir=".data/",
                        batch_size=-1, with_info=True)
    return info.features["label"].names
