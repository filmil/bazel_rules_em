<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Public API of hdlf_rules_em.

Load every rule from this file:

```starlark
load(
    "@hdlf_rules_em//em:defs.bzl",
    "determinism_test",
    "swap_test",
    "tool_toolchain",
)
```

This is the scaffold. The EM rules, `kicad_em_model`, `kicad_em_sim` and
`kicad_em_test`, arrive after the openEMS toolchain, one pull request
each, as https://github.com/filmil/bazel_rules_em/issues/1 lays out. This
file is the only public entry point; everything under `//em/private` may
change without notice.

<a id="tool_toolchain"></a>

## tool_toolchain

<pre>
load("@hdlf_rules_em//em:defs.bzl", "tool_toolchain")

tool_toolchain(<a href="#tool_toolchain-name">name</a>, <a href="#tool_toolchain-tools">tools</a>)
</pre>

A toolchain that offers tools by name, for `run_tool`.

Example, a toolchain for a tool of your own:

```starlark
tool_toolchain(
    name = "my_tool_impl",
    tools = {"//third_party/my_tool": "my_tool"},
)

toolchain(
    name = "my_tool",
    toolchain = ":my_tool_impl",
    toolchain_type = "//my:toolchain_type",
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="tool_toolchain-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="tool_toolchain-tools"></a>tools |  Executable targets, each mapped to the name rules use for it.   | <a href="https://bazel.build/rules/lib/core/dict">Dictionary: Label -> String</a> | required |  |


<a id="determinism_test"></a>

## determinism_test

<pre>
load("@hdlf_rules_em//em:defs.bzl", "determinism_test")

determinism_test(<a href="#determinism_test-name">name</a>, <a href="#determinism_test-target">target</a>, <a href="#determinism_test-kwargs">**kwargs</a>)
</pre>

Checks that two builds of `target` give byte-identical outputs.

The target is built twice in one build, under two configurations that
differ only in a setting no rule reads, so every action runs twice.
The test compares the SHA-256 of every output file, directories walked,
and fails on any difference (spec D1).

Two runs in the same second can hide a time stamp with a one-second
resolution, so pair this with a header scan where a format has one. CI
also builds every determinism_test target in two fresh output bases and
compares the results.

Example:

```starlark
determinism_test(
    name = "microstrip_model_determinism_test",
    target = ":microstrip_model",
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="determinism_test-name"></a>name |  the test's name.   |  none |
| <a id="determinism_test-target"></a>target |  the target whose outputs must not change between builds.   |  none |
| <a id="determinism_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


<a id="swap_test"></a>

## swap_test

<pre>
load("@hdlf_rules_em//em:defs.bzl", "swap_test")

swap_test(<a href="#swap_test-name">name</a>, <a href="#swap_test-targets">targets</a>, <a href="#swap_test-kwargs">**kwargs</a>)
</pre>

Builds targets with every tool swapped for a stub, and checks them.

The targets are built with `--@hdlf_rules_em//em/toolchains:use_stub`,
so each tool toolchain that honors the flag resolves to the stub tool.
The stub writes a record into every output, files and directories
alike. The test fails if any output file of the targets lacks the
record: some action made it without a toolchain tool.

Example:

```starlark
swap_test(
    name = "swap_test",
    targets = [":microstrip_model", ":microstrip_sim"],
)
```


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="swap_test-name"></a>name |  the test's name.   |  none |
| <a id="swap_test-targets"></a>targets |  targets whose outputs must all come from the stub.   |  none |
| <a id="swap_test-kwargs"></a>kwargs |  common test attributes, such as `size` and `tags`.   |  none |


