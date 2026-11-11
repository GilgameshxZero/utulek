#!/usr/bin/env python3
"""Validate serialized headers independently of terminal rendering."""
import email, email.utils, json, sys
from pathlib import Path
root=Path(sys.argv[1]); results=[]
for case in json.loads((root/'cases.json').read_text()):
    msg=email.message_from_bytes(Path(case['draft']).read_bytes())
    addresses=sorted(a.lower() for _,a in email.utils.getaddresses(msg.get_all('To',[])+msg.get_all('Cc',[])))
    results.append(dict(name=case['name'],passed=addresses==sorted(case['expected']),addresses=addresses,expected=case['expected']))
(root/'address_results.json').write_text(json.dumps(results,indent=2));print(json.dumps(results,indent=2));sys.exit(not all(r['passed'] for r in results))
