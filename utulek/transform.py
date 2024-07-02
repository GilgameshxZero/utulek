# Data transformation.


def transform_flatten_img_ds_X(X):
    import numpy as np

    return np.reshape(X, (X.shape[0], -1))


def transform_batched_iterable(iterable, batch_size=1):
    c_iterable = len(iterable)
    for ndx in range(0, c_iterable, batch_size):
        yield iterable[ndx : min(ndx + batch_size, c_iterable)]
