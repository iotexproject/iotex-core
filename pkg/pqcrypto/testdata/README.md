# NIST ACVP vectors for SLH-DSA-SHA2-192s

`acvp-slhdsa-sha2-192s.json` holds the FIPS 205 test vectors that apply to
IoTeX's use of SLH-DSA (IIP-65 §2): the SLH-DSA-SHA2-192s groups of

| ACVP mode | Group | Used by |
|---|---|---|
| `keyGen` | AFT, all 10 cases | `signer.TestACVPKeyGen` |
| `sigGen` | external interface, pure, deterministic (7 cases) | `signer.TestACVPSigGen` |
| `sigVer` | external interface, pure (14 cases, 6 of them invalid) | `pqcrypto.TestACVPSigVer` |

They come from `usnistgov/ACVP-Server` at commit
`112690e8484dba7077709a05b1f3af58ddefdd5d`,
`gen-val/json-files/SLH-DSA-{keyGen,sigGen,sigVer}-FIPS205/internalProjection.json`
(git blob `bd99c97086…`, `5ff182bc29…`, `0805107118…`). `acvp_extract.py`
selects the groups:

```sh
python3 -I acvp_extract.py <dir with the three files> acvp-slhdsa-sha2-192s.json
```

The pre-hash and internal interfaces, and non-deterministic signing, are not
used by IoTeX and are left out. Hedged signing is covered by
`signer.TestSignIsHedged`.
