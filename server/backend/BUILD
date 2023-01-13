load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "backend.go",
        "livegrep_backend.go",
    ],
    importpath = "github.com/livegrep/livegrep/server/backend",
    visibility = ["//visibility:public"],
    deps = [
        "//src/proto:go_proto",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
