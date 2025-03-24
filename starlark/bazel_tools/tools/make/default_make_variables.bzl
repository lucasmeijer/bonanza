load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

def _default_make_variables_impl(ctx):
    return [platform_common.TemplateVariableInfo({
        "COMPILATION_MODE": ctx.attr._compilation_mode[BuildSettingInfo].value,
        "BINDIR": ctx.bin_dir.path,
        # TODO: Do we actually need to set this?
        "TARGET_CPU": "unknown",
    })]

default_make_variables = rule(
    _default_make_variables_impl,
    attrs = {
        "_compilation_mode": attr.label(default = "//command_line_option:compilation_mode"),
    },
)
