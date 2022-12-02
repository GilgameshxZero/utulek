# Utilities for fetching from the network.

from .show import TqdmByteTransfer


def fetch_ds_cifar_10(cache_dir=".data/cifar-10"):
    """
    Returns a tuple of (X, Y, labels) for the CIFAR-10 dataset.
    X := (32, 32, 3) of uint8.
    Y := Scalar in [0, 10) of uint8.
    labels := Scalar of str.
    """
    
    import numpy as np
    import os
    import pathlib
    import pickle
    import tqdm
    import urllib
    import tarfile
    
    REMOTE_PATH = "https://www.cs.toronto.edu/~kriz/cifar-10-python.tar.gz"
    LOCAL_PATH = os.path.join(cache_dir, "cifar-10-python.tar.gz")
    
    if not os.path.exists(LOCAL_PATH):
        os.makedirs(cache_dir, exist_ok=True)
        try:
            with TqdmByteTransfer() as tqdm_byte_transfer:
                urllib.request.urlretrieve(REMOTE_PATH, filename=LOCAL_PATH, reporthook=tqdm_byte_transfer.up_to)
        except:
            pathlib.Path(LOCAL_PATH).unlink(missing_ok=True)
            raise
    
    Xs, Ys = [], []
            
    with tarfile.open(LOCAL_PATH) as tar:        
        for filename in [
            "cifar-10-batches-py/data_batch_1",
            "cifar-10-batches-py/data_batch_2",
            "cifar-10-batches-py/data_batch_3",
            "cifar-10-batches-py/data_batch_4",
            "cifar-10-batches-py/data_batch_5",
            "cifar-10-batches-py/test_batch"
        ]:
            with tar.extractfile(filename) as file:
                obj = pickle.load(file, encoding="bytes")
                Xs.append(obj[b"data"])
                Ys.append(obj[b"labels"])
            
        with tar.extractfile("cifar-10-batches-py/batches.meta") as file:
            obj = pickle.load(file, encoding="bytes")
            labels = np.array(obj[b"label_names"]).astype(str)
    
    X = np.vstack(Xs).reshape((-1, 3, 32, 32)).transpose((0, 2, 3, 1))
    Y = np.hstack(Ys).astype(np.uint8)
    return X, Y, labels


def fetch_tfds_img_ds(name, split):
    """
    Use TFDS to fetch a split for a given dataset.
    """
    
    import operator
    import numpy as np
    import tensorflow_datasets as tfds

    X, Y = operator.itemgetter("image", "label")(
        tfds.as_numpy(tfds.load(name=name, split=split,
                                data_dir=".data/", batch_size=-1)))
    X = X.astype(np.float32)

    # Normalize to [0, 1].
    X /= np.max(X)
    return X, Y


def fetch_tfds_img_ds_labels(name):
    """
    Use TFDS to fetch the labels for a given dataset name.
    """
    
    import tensorflow_datasets as tfds

    _, info = tfds.load(name=name, split="train[:1]", data_dir=".data/",
                        batch_size=-1, with_info=True)
    return info.features["label"].names
