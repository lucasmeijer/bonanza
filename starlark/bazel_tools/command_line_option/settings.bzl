load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

def _impl(ctx):
    return [BuildSettingInfo(value = ctx.build_setting_value)]

label_list_flag = rule(
    implementation = _impl,
    build_setting = config.label_list(flag = True),
)
