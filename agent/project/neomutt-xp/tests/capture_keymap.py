#!/usr/bin/env python3
"""External pager used only to retain NeoMutt's effective binding/macro dumps."""
from pathlib import Path
import sys
Path(sys.argv[1]).write_bytes(Path(sys.argv[-1]).read_bytes())
