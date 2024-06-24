# utulek

```bash
# This may be aliased as `python3` on your system.
python -m venv .venv

# .venv\Scripts\activate on Windows.
activate

# Common pre-install requirements. `python -m` here avoids permissions errors.
python -m pip install --upgrade -r requirements/pre.txt

# `gpu.txt` should be used instead when a suitable CUDA and cuDNN install is available.
pip install --upgrade -r requirements/cpu.txt
```

`utulek` support is always pinned to a single version of all major dependencies. The table below describes the current pinned versions. These versions are guaranteed to be available on up-to-date Manjaro/ Arch.

Query the current CUDA version with `nvcc -V` or `pacman -Q cuda`. Query the current `cuDNN` version with `pacman -Q cudnn`. Query the current `python` version with `python -V`. Older python versions are usually available via the AUR on Arch and can be automatically built via `pamac`. It is recommended to not upgrade the full system via `pacman -Syu` while versions are correctly pinned.

Dependency|Version
-|-
`python`|`3.11.9`
`CUDA`|`12.4.1`
`cuDNN`|`8.9.7`
`tensorflow`|`2.16.1`
`torch`|`2.3.0`
`jax`|`0.4.28`

When version pinning is updated, corresponding pins in `requirements/*` should also be updated. Upgrade packages on Arch with something like `sudo pacman -Syu --ignore cuda --ignore cudnn` to avoid losing the pin.

Python should be formatted using `black`.
