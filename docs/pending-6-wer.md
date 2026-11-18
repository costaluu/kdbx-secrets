# Pending item 6: Windows crash dumps (WER)

State after ticket 08: **open, manual verification.** Windows hardening is just `VirtualLock` on the binary's own buffers (password and output), via `hardening.LockBuffer`. Nothing excludes the process from Windows Error Reporting dumps.

## What was investigated

Documentation reading only, no test on a real Windows:

- WER writes a dump of a crashing process only if the user or the machine configured `LocalDumps` (`HKLM\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps`). Without that key, the default is not to write a local dump.
- A Go process that exits through `panic` or `os.Exit` does not trigger WER; only fatal runtime failures or native exceptions (access violation) do.
- Candidates to exclude the process: `WerAddExcludedApplication` (writes to the registry, requires privilege or affects the whole user, too invasive for a short-lived CLI) and `SetErrorMode(SEM_NOGPFAULTERRORBOX)` (suppresses the dialog box, does not guarantee the absence of a dump).

## Pending manual verification

1. On a Windows with `LocalDumps` configured (`DumpType=2`), force a fatal failure in the binary after reading the password and check whether the dump contains the password or the Database.
2. If it does, evaluate `WerAddExcludedApplication` or document the recommendation not to enable `LocalDumps` on machines that use the binary.

## Limits already documented

No `mlockall`: the decrypted Database (`gokeepasslib` `string`s) may go to the pagefile, which is in cleartext without BitLocker; hibernation writes RAM to disk. See `CONTEXT.md`.
