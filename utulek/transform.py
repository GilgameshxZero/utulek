def transform_flatten_img_ds_X(X):
    import numpy as np
    return np.reshape(X, (X.shape[0], -1))
