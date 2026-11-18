# 09: `render` with a two-pass template

**What to build:** `render <name>` executes `templates/<name>.tmpl` (`text/template`) and writes the result to stdout. It makes two passes. The collection pass runs the template with a `get` that notes the Reference and returns empty, and with no prompt. Then comes a single prompt listing the collected References. Lastly, the real execution; if it asks for a Reference that did not appear in the collection pass, it is an error. The available functions are `get`, `json` and `base64`. Template errors are rewritten as "template <name>: line N: <kind>", without repeating the original text or values. The name must match `^[A-Za-z0-9._-]+$` and not start with `.`. The template goes through the Unix owner and permissions check. The refusal of a tty without `--show` applies here too. The acceptance test is the `paper-freeze` case: the `credential_process` JSON `{"Version":1,"AccessKeyId":…,"SecretAccessKey":…}`.

**Blocked by:** 01

**Status:** done

- [ ] The `aws-paper-freeze` template produces the exact JSON from the fixture, with a single prompt
- [ ] A Reference that only appears in a branch of the real execution triggers an error without a second prompt
- [ ] A template with a syntax or execution error produces "template X: line N: kind", without leaking a value
- [ ] Syntax errors in the collected References appear before the prompt
- [ ] Invalid name and nonexistent template are errors before the prompt
- [ ] `json` and `base64` escape and encode correctly
- [ ] If ticket 02 is already done, a group-writable template is refused on Unix
