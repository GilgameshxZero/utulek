# Utilities for training models.


def eval_svc_acc(svc, X, Y):
    import utulek
    import numpy as np
    return np.count_nonzero(
        Y == svc.predict(utulek.transform_flatten_img_ds_X(X))) / len(Y)


def train_svc(C, X, Y, **kwargs):
    import utulek
    from sklearn import svm
    return svm.SVC(C=C, **kwargs).fit(utulek.transform_flatten_img_ds_X(X), Y)


def process_dataloader_async(dataloader, device, process):
    """
    Steps through a dataloader, and calls `process` on each batch.
    Preloads the next batch onto the specified device. For this to be effective, requires pin_memory=True.
    """
    
    batches, c_batches = iter(dataloader), len(dataloader)
    batch_next = next(batches)
    batch_next = [i.to(device, non_blocking=True) for i in batch_next]
    
    for i in range(c_batches):
        batch = batch_next
        if i != c_batches - 1:
            batch_next = next(batches)
            batch_next = [i.to(device, non_blocking=True) for i in batch_next]
        
        process(batch)
            