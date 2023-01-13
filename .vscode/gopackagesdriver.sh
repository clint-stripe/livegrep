#!/usr/bin/env bash
bazel=bazel
if type bazelisk >/dev/null; then
    bazel=bazelisk
fi
exec $bazel run --tool_tag=gopackagesdriver -- @io_bazel_rules_go//go/tools/gopackagesdriver "${@}"