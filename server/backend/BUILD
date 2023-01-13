load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "backend.go",
        "livegrep_backend.go",
        "zoekt_backend.go",
    ],
    importpath = "github.com/livegrep/livegrep/server/backend",
    visibility = ["//visibility:public"],
    deps = [
        "//server/log:go_default_library",
        "//src/proto:go_proto",
        "@com_github_grafana_regexp//:regexp",
        "@com_github_sourcegraph_zoekt//:go_default_library",
        "@com_github_sourcegraph_zoekt//query:go_default_library",
        "@com_github_sourcegraph_zoekt//rpc:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
