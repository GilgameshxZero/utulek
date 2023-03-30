# útulek

```bash
# This may be aliased as `python3` on your system. Ensure that `python -V` is at least 3.9.10.
python -m venv .venv

# .venv\Scripts\activate on Windows.
activate

# Common pre-install requirements. `python -m` here avoids permissions errors.
python -m pip install --upgrade -r requirements/pre.txt

# If CUDA/cuDNN 8.0.2/11.1 is available, please use `gpu.txt` instead.
pip install --upgrade -r requirements/cpu.txt
```

The environment officially supports Python 3.9.10 with CUDA 11.1.1, cuDNN 8.1.0. Future version with enough back-compatibility may also be supported.
