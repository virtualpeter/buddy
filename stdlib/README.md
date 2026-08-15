# stdlib

Reserved for shared buddyscript modules loaded as `load("stdlib/<name>.buddy", …)`.

There are no modules yet. Multi-step recipes live in [`examples/`](../examples/) (copy and adapt):

- [`examples/bump-release.buddy`](../examples/bump-release.buddy) — policy package / install / Self Service bump
- [`examples/setup-app-classic.buddy`](../examples/setup-app-classic.buddy) — Classic greenfield change-batch steps

To add a module, drop `<name>.buddy` here (lowercase, no subdirs) and add `//go:embed *.buddy` next to the README embed in `fs.go`.
