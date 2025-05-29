load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

def _simple_impl(ctx):
    return [BuildSettingInfo(value = ctx.build_setting_value)]

bool_flag = rule(
    implementation = _simple_impl,
    build_setting = config.bool(flag = True),
    needs_default_exec_group = False,
    needs_make_variables = False,
    needs_target_platform = False,
)

int_flag = rule(
    implementation = _simple_impl,
    build_setting = config.int(flag = True),
    needs_default_exec_group = False,
    needs_make_variables = False,
    needs_target_platform = False,
)

label_list_flag = rule(
    implementation = _simple_impl,
    build_setting = config.label_list(flag = True),
    needs_default_exec_group = False,
    needs_make_variables = False,
    needs_target_platform = False,
)

string_list_flag = rule(
    implementation = _simple_impl,
    build_setting = config.string_list(flag = True),
    needs_default_exec_group = False,
    needs_make_variables = False,
    needs_target_platform = False,
)

def _string_impl(ctx):
    if ctx.attr.values and ctx.build_setting_value not in ctx.attr.values:
        fail("value '%s' is not one of %s" % ctx.build_setting_value, ctx.attr.values)
    return [BuildSettingInfo(value = ctx.build_setting_value)]

string_flag = rule(
    implementation = _string_impl,
    attrs = {
        "values": attr.string_list(),
    },
    build_setting = config.string(flag = True),
    needs_default_exec_group = False,
    needs_make_variables = False,
    needs_target_platform = False,
)
