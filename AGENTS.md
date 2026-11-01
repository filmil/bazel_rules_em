<!-- SPDX-License-Identifier: Apache-2.0 -->
# Instructions

Read `README.md` first.

The coding standard is the `ai-coding-sop` repository at
`https://github.com/filmil/ai-coding-sop`.
Its `AGENTS.md`, its `prose-readability` skill, its `git-commit-rules`
skill and its `hermetic`, `bazel` and `bazel-bzl-files` fragments apply
here in full.
Where this file and ai-coding-sop differ, ai-coding-sop wins; ask the
operator about a conflict that matters.

# Agent operating rules

A change is complete only when `bazel test //...` passes, the integration
module passes (`cd integration && bazel test //...`), and new tests cover
the change.

1. Edit text sources only: simulation scripts (`.py`), limit and stackup
   tables (`.json`), `BUILD.bazel`, `MODULE.bazel`, `.bzl`, Go sources. Do
   not commit generated outputs, except the docs that
   `bazel run //docs:update_docs` writes.
2. Keep every action hermetic. No host tools, no host files outside the
   workspace, no host environment variables, no network in actions. Add
   each external file with `http_archive` or `http_file` and a pinned
   `sha256`. `--action_env=PATH`, `use_default_shell_env = True` and the
   tags `local` and `no-sandbox` are forbidden. If a tool cannot run
   hermetically, stop and ask; do not make it work with a host install.
3. Find targets with `bazel query` and `bazel cquery`. Run
   `bazel run //tools:buildifier` on every change.
4. Once there are golden files, update them only with
   `bazel run //:update_goldens`, and give the reason in the commit
   message.
5. A change to an EM model or simulation attaches its plots (the
   impedance, TDR and S-parameter PNGs) and the report to the pull request.
6. One logical change per pull request: one rule, one feature, or one
   design change.
7. Stop and ask the operator before you:
   * delete a test, or loosen a tolerance;
   * change a toolchain version or a library version, the openEMS pin
     included;
   * run a release or publish workflow, or set a secret.
8. When a test fails and the cause is not clear after three attempts, stop.
   Write the failure and your hypotheses in the pull request.

# Rules particular to this repository

* **Tools come from toolchains only.** Every action runs its tool through
  `run_tool` in `//em/private`, with the tool from `ctx.toolchains`.
  No private attribute names an external tool.
* **Public packages name no dev dependency.** A user of the module loads
  `//em` and `//`, where `@stardoc`, `@bazel_lib`, `@buildifier_prebuilt`
  and `@hermetic_cc_toolchain` are not visible. Documentation targets live in
  `//docs` and development tools in `//tools`.
* **Documentation is generated.** After changing a `.bzl` file, run
  `bazel run //docs:update_docs`; `bazel test //...` fails until then.
* **Linux x86_64 only** in v1.
* **Python only where openEMS needs it.** openEMS's bindings and the
  post-processing that uses them are Python; every other helper is Go.
* **FDTD tests are slow.** A target or test that runs the solver gets
  the tag `fdtd` (a test also `size = "large"`). `.bazelrc` keeps them out
  of the default `bazel build //...` and `bazel test //...`;
  `bazel test --config=fdtd //...` runs only them, and the CI job `fdtd`
  runs that.
* **The openEMS wheels are built, not downloaded from PyPI.** A new
  openEMS or CSXCAD commit, or a change to `tools/openems_wheels`, is a
  toolchain change (rule 7): the operator runs
  `bazel run //tools:openems_wheels` twice, checks the two `SHA256SUMS`
  agree, and uploads with `bazel run //tools:mirror`.
* **Simulations are deterministic.** Run openEMS with a fixed engine and
  thread count (`engine='multithreaded'`, `numThreads=N`), never the
  defaults, which pick the fastest engine and use every core. Compare
  goldens with tolerances, never bit for bit.
* **Commits for a BCR pull request carry no `Co-Authored-By` trailer**
  (the `bazel-bcr-publish` skill says why). This is about the pull request
  on the Bazel Central Registry's repository that the release makes, not
  about commits in this repository.

`CLAUDE.md` and `GEMINI.md` are symlinks to this file.
