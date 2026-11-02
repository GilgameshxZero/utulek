#!/usr/bin/env python3
"""Compare contact identities against installed khard without writing contacts."""
import argparse
import json
from pathlib import Path
import subprocess


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--config", required=True)
    p.add_argument("--reference", default="khard")
    p.add_argument("--candidate", default=".bin/khard")
    p.add_argument("--report", default=".obj/readonly_comparison.json")
    args = p.parse_args()
    report = {"read_only": True, "commands": {}}
    for command in ("list", "phone", "email"):
        outputs = []
        for executable in (args.reference, args.candidate):
            result = subprocess.run([executable, "-c", args.config, command, "-p"], capture_output=True, text=True, check=True)
            # Upstream email prints a status line even in parsable mode.
            outputs.append(sorted(line for line in result.stdout.splitlines() if "\t" in line))
        # Phone/email labels are intentionally blank for untyped properties;
        # upstream displays implicit voice/internet labels for those properties.
        identities = [sorted("\t".join(row.split("\t")[:2]) for row in rows) for rows in outputs]
        report["commands"][command] = {"reference_rows": len(outputs[0]), "candidate_rows": len(outputs[1]), "identities_equal": identities[0] == identities[1], "exact_rows_equal": outputs[0] == outputs[1]}
    Path(args.report).write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))
    if not all(result["identities_equal"] for result in report["commands"].values()):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
