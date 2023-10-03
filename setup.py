# Shared library for utulek Python code.
import setuptools

setuptools.setup(
    name="utulek",
    packages=["utulek"],

    # Default requires are are skimped versions of what is required from the utulek library environment.
    install_requires=["numpy", "matplotlib", "scikit-learn", "tensorflow-datasets"]
)
