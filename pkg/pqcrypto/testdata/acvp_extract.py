"""Extract the SLH-DSA-SHA2-192s ACVP groups IoTeX uses (pure, external
interface; key generation) from NIST ACVP-Server internalProjection files."""
import json
import sys
from pathlib import Path

base = Path(__file__).resolve().parent
src_dir = Path(sys.argv[1]).resolve(strict=True)
if not src_dir.is_dir():
    raise ValueError('input must be a directory')
filename = sys.argv[2] if len(sys.argv) > 2 else 'acvp-slhdsa-sha2-192s.json'
if not filename or filename in ('.', '..') or '/' in filename or '\\' in filename:
    raise ValueError('output must be a plain basename')
out = (base / filename).resolve()
if out.parent != base:
    raise ValueError('output must remain in the script directory')
ps = 'SLH-DSA-SHA2-192s'

def load(name):
    if name not in ('keyGen', 'sigGen', 'sigVer'):
        raise ValueError('unknown input')
    path = (src_dir / f'{name}-internalProjection.json').resolve(strict=True)
    if path.parent != src_dir:
        raise ValueError('input must remain in the source directory')
    with path.open() as f:
        return json.load(f)

def groups(j, **want):
    return [g for g in j['testGroups'] if g['parameterSet'] == ps and all(g.get(k) == v for k, v in want.items())]

res = {'source': 'usnistgov/ACVP-Server@112690e8484dba7077709a05b1f3af58ddefdd5d gen-val/json-files/SLH-DSA-*-FIPS205/internalProjection.json',
       'parameterSet': ps}
kg = groups(load('keyGen'))
res['keyGen'] = [{k: t[k] for k in ('tcId', 'skSeed', 'skPrf', 'pkSeed', 'sk', 'pk')} for g in kg for t in g['tests']]
sg = groups(load('sigGen'), signatureInterface='external', preHash='pure', deterministic=True)
res['sigGen'] = [{k: t[k] for k in ('tcId', 'sk', 'pk', 'message', 'context', 'signature')} for g in sg for t in g['tests']]
sv = groups(load('sigVer'), signatureInterface='external', preHash='pure')
res['sigVer'] = [{k: t[k] for k in ('tcId', 'pk', 'message', 'context', 'signature', 'testPassed', 'reason')} for g in sv for t in g['tests']]
with out.open('w') as f:
    json.dump(res, f, indent=1)
print({k: len(v) for k, v in res.items() if isinstance(v, list)})
print(sorted({(t['testPassed'], t['reason']) for t in res['sigVer']}))
