def get_notebook_name(session_name: str = "") -> str:
    """Guess the name of an `.ipynb` based on the session name of the kernel."""
    import os

    if session_name == "":
        session_name = os.getenv("JPY_SESSION_NAME")
    # 1 + 36 + 6 characters for `-{UUID}.ipynb`.
    return os.path.basename(session_name)[:-43] + ".ipynb"


def get_notebook_assets_path(session_name: str = "") -> str:
    """Notebooks of the name `NAME.ID.ipynb` all share an assets directory of `NAME.ipynb.assets`."""
    import os

    dirpath = os.path.join(os.getcwd(), "experiment")
    basename = get_notebook_name(session_name=session_name)
    basename_prefix = basename.rsplit(".", 2)[0]
    return os.path.join(dirpath, basename_prefix + ".ipynb.assets/")
