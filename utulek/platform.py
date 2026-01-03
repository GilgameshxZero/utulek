import typing


# <https://stackoverflow.com/a/64791087>.
def import_global(
    global_scope: typing.Dict[str, str],
    object_name: str,
    short_name: str = "",
    context_module_name: str = "",
):
    """Use this statement to import inside a function,
    but effective as import at the top of the module.

    Args:
        object_name: the object name want to import,
                     could be module or function
        short_name: the short name for the import
        context_module_name: the context module name in the import

    example usage:
    import os -> global_imports("os")
    import numpy as np -> global_imports("numpy", "np")
    from collections import Counter ->
        global_imports("Counter", None, "collections")
    from google.cloud import storage ->
        global_imports("storage", None, "google.cloud")

    """
    if short_name == "":
        short_name = object_name
    if context_module_name == "":
        global_scope[short_name] = __import__(object_name)
    else:
        context_module = __import__(context_module_name, fromlist=[object_name])
        global_scope[short_name] = getattr(context_module, object_name)


def import_globals(global_scope: typing.Dict[str, str]):
    """Import commonly used globals under standard namespaces. Should pass `globals()` as the first argument."""
    from .filesystem import get_notebook_name, get_notebook_assets_path

    import_global(global_scope, "os")
    import_global(global_scope, "glob")
    import_global(global_scope, "pathlib")
    import_global(global_scope, "pp", "", "pprint")
    import_global(global_scope, "json")
    import_global(global_scope, "tqdm", "", "tqdm.auto")
    import_global(global_scope, "dill")
    import_global(global_scope, "pyplot", "plt", "matplotlib")
    import_global(global_scope, "display", "idisplay", "IPython")
    import_global(global_scope, "typing")
    import_global(global_scope, "numpy", "np")

    import_global(global_scope, "transformers")
    import_global(global_scope, "torch")
    import_global(global_scope, "whisper")

    import_global(global_scope, "librosa")
    import_global(global_scope, "soundfile")

    # Global variables.
    global_scope["NOTEBOOK_NAME"] = get_notebook_name()
    global_scope["ASSETS_PATH"] = get_notebook_assets_path()

    # Debug.
    print(global_scope["ASSETS_PATH"])
    print(
        [
            global_scope["torch"].cuda.get_device_name(i)
            for i in range(global_scope["torch"].cuda.device_count())
        ]
    )
