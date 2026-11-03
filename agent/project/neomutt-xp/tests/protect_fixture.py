#!/usr/bin/env python3
"""Apply the production helper's private-storage policy to fixture tokens only."""
from pathlib import Path
import sys
from private_storage import protect_private
path=Path(sys.argv[1])
if path.name!='oauth-token.gpg' or 'neomutt-xp' not in str(path):
 raise SystemExit('Refusing a non-fixture token path')
protect_private(path)
