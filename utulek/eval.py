# Utilities for evaluating models.

def eval_svc_acc(svc, X, Y):
    import utulek
    import numpy as np
    return np.count_nonzero(
        Y == svc.predict(utulek.transform_flatten_img_ds_X(X))) / len(Y)
