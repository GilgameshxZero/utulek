#!/usr/bin/env python3
"""Exercise release-evidence rejection using a supplied real checkpoint bundle."""
import argparse
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile

ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location('gate', ROOT / 'scripts/check_release.py')
gate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gate)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence-root', type=Path, required=True)
    args = parser.parse_args()
    checks = []
    files = gate.load_json(args.evidence_root / 'checkpoints/04_validation/checkpoint.json')['files']

    def rejects(label, action):
        try:
            action()
        except (ValueError, FileNotFoundError):
            checks.append(label)
        else:
            raise AssertionError('Gate accepted ' + label)

    with tempfile.TemporaryDirectory(dir=ROOT / '.obj', prefix='release_gate_test_') as temp:
        root = Path(temp)
        for name in ('02_persistence', '03_large_library', '04_validation'):
            source = args.evidence_root / 'checkpoints' / name
            destination = root / 'checkpoints' / name
            destination.mkdir(parents=True)
            for filename in ('checkpoint.json', 'source.tar.gz'):
                shutil.copy2(source / filename, destination / filename)
            for original in gate.load_json(source / 'checkpoint.json')['evidence']:
                path = gate.relocated_evidence(root, original)
                path.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(gate.relocated_evidence(args.evidence_root, original), path)
        gate.verify_evidence(root, files)
        checks.append('complete_bundle_accepted')
        rejects('mismatched_binary_hash_rejected', lambda: gate.verify_evidence(root, dict(files, cmus='0' * 64)))
        rejects('path_traversal_rejected', lambda: gate.relocated_evidence(root, '/project/.obj/../../escape'))
        manifest = root / 'checkpoints/04_validation/checkpoint.json'
        saved_manifest = manifest.read_bytes()
        manifest.unlink()
        rejects('missing_checkpoint_rejected', lambda: gate.verify_evidence(root, files))
        manifest.write_bytes(saved_manifest)
        source = root / 'checkpoints/04_validation/source.tar.gz'
        saved_source = source.read_bytes()
        source.write_bytes(saved_source + b'tampered')
        rejects('modified_checkpoint_source_rejected', lambda: gate.verify_evidence(root, files))
        source.write_bytes(saved_source)
        evidence = root / 'milestones/validation/windows/results.json'
        saved_evidence = evidence.read_bytes()
        evidence.write_bytes(saved_evidence + b' ')
        rejects('modified_evidence_rejected', lambda: gate.verify_evidence(root, files))
        evidence.write_bytes(saved_evidence)
        rows = gate.load_json(evidence)
        evidence.write_text(json.dumps(rows[:-1]))
        altered_manifest = json.loads(saved_manifest)
        for key in altered_manifest['evidence']:
            if key.endswith('validation/windows/results.json'):
                altered_manifest['evidence'][key] = gate.digest(evidence)
        manifest.write_text(json.dumps(altered_manifest))
        rejects('truncated_decoder_coverage_rejected', lambda: gate.verify_evidence(root, files))
        manifest.write_bytes(saved_manifest)
        evidence.write_bytes(saved_evidence)
        saved_root = gate.ROOT
        gate.ROOT = root / 'changed_runtime'
        gate.ROOT.mkdir()
        (gate.ROOT / 'player.c').write_text('/* unvalidated runtime */\n')
        try:
            rejects('changed_runtime_rejected', lambda: gate.verify_evidence(root, files))
        finally:
            gate.ROOT = saved_root
        gate.verify_evidence(root, files)
        checks.append('restored_bundle_accepted')
    print(json.dumps({'passed': True, 'checks': checks}, indent=2))


if __name__ == '__main__':
    main()
