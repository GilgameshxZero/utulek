# Utilities for training models.

def train_svc(C, X, Y, **kwargs):
    import utulek
    from sklearn import svm
    return svm.SVC(C=C, **kwargs).fit(utulek.transform_flatten_img_ds_X(X), Y)
