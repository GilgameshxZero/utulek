# útulek

ML environment setup:

```bash
python -m venv .venv
activate
pip install --upgrade -r requirements/pre.txt

# If CUDA/cuDNN 8.0.2/11.1 is available, please use `gpu.txt` instead.
pip install --upgrade -r requirements/cpu.txt
```
