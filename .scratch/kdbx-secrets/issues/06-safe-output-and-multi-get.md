# 06: Safe output and `get` with several References

**What to build:** The spec's single output points. A sanitizer replaces control characters with `?` in everything that goes to stderr or the tty (process names, Entries, References). A guard refuses to write a secret to a stdout that is a terminal without `--show`. Both will be used by `get`, `render` and `decrypt`. `get` accepts several References with a single prompt and returns one value per line, in the requested order, and fails if any value has a line break. The password and the binary's output buffers are wiped after use, as best effort.

**Blocked by:** 01

**Status:** done

- [ ] With stdout as a tty and without `--show`, `get` fails before the prompt; with `--show`, it writes
- [ ] A title with a control character (fixture) shows up sanitized on stderr and in the prompt
- [ ] No error message contains the secret's value (assertion in the error tests)
- [ ] `get a b c` uses one prompt and returns 3 lines in the requested order
- [ ] With several References and a multiline value, the command fails without writing anything to stdout
