load("@bazel_tools//tools/build_defs/pkg:pkg.bzl", "pkg_tar")
load("@com_grail_bazel_compdb//:aspects.bzl", "compilation_database")
load("@bazel_gazelle//:def.bzl", "gazelle")

compilation_database(
    name = "compilation_db",
    targets = [
        "//src/tools:codesearch",
        "//src/tools:codesearchtool",
    ],
)

# gazelle:prefix github.com/livegrep/livegrep
gazelle(name = "gazelle")

gazelle(
    name = "gazelle-update-repos",
    args = [
        "-from_file=go.mod",
        "-to_macro=tools/build_defs/go_externals.bzl%go_externals",
    ],
    command = "update-repos",
)

filegroup(
    name = "docs",
    srcs = glob([
        "doc/**/*",
    ]),
)

pkg_tar(
    name = "livegrep",
    srcs = [
        ":COPYING",
        ":README.md",
        ":docs",
    ],
    strip_prefix = ".",
    deps = [
        "//cmd:go_tools",
        "//src/tools:cc_tools",
        "//web:assets",
    ],
)
