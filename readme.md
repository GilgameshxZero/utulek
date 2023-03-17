# útulek

ML environment setup:

```bash
python -m venv .venv
activate  # Or, .venv\Scripts\activate on Windows.
pip install --upgrade -r requirements/pre.txt

# If CUDA/cuDNN 8.0.2/11.1 is available, please use `gpu.txt` instead.
pip install --upgrade -r requirements/cpu.txt
```
