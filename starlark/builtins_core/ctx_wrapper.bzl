def wrapped_ctx_configuration(ctx):
    return ctx._real_ctx.fragments.configuration

def wrapped_ctx_var(ctx):
    return ctx._real_ctx.var

WrappedCtx = provider(
    computed_fields = {
        "configuration": wrapped_ctx_configuration,
        "var": wrapped_ctx_var,
    },
)

def wrap_ctx(ctx):
    def ctx_actions_declare_shareable_artifact(path, artifact_root = None):
        if artifact_root and artifact_root != ctx.bin_dir:
            fail("artifact_root %s is not equal to ctx.bin_dir %s, which is not supported by this implementation" % (artifact_root, ctx.bin_dir))

        expected_path_prefix = ctx.label.workspace_root + "/"
        if ctx.label.package:
            expected_path_prefix += ctx.label.package
            expected_path_prefix += "/"
        if not path.startswith(expected_path_prefix):
            fail("path %s does not start with %s, which is not supported by this implementation" % (path, expected_path_prefix))

        return ctx.actions.declare_file(path.removeprefix(expected_path_prefix))

    def ctx_actions_run_shell(*, command, **kwargs):
        ctx.actions.run(
            executable = "/bin/bash",
            arguments = ["-c", command],
            **kwargs
        )

    def ctx_coverage_instrumented(target = None):
        return False

    def ctx_expand_location(input, targets = []):
        # TODO: Actually expand locations!
        return input

    actions = ctx.actions
    ctx_fields = {
        "_real_ctx": ctx,
        "actions": struct(
            args = actions.args,
            declare_directory = actions.declare_directory,
            declare_file = actions.declare_file,
            declare_shareable_artifact = ctx_actions_declare_shareable_artifact,
            declare_symlink = actions.declare_symlink,
            expand_template = actions.expand_template,
            run = actions.run,
            run_shell = ctx_actions_run_shell,
            symlink = actions.symlink,
            transform_info_file = actions.transform_info_file,
            transform_version_file = actions.transform_version_file,
            write = actions.write,
        ),
        "attr": ctx.attr,
        "bin_dir": ctx.bin_dir,
        "coverage_instrumented": ctx_coverage_instrumented,
        "disabled_features": [],
        "exec_groups": ctx.exec_groups,
        "executable": ctx.executable,
        "expand_location": ctx_expand_location,
        "features": [],
        "file": ctx.file,
        "files": ctx.files,
        "fragments": ctx.fragments,
        "info_file": ctx.info_file,
        "label": ctx.label,
        "outputs": ctx.outputs,
        "runfiles": ctx.runfiles,
        "target_platform_has_constraint": ctx.target_platform_has_constraint,
        "version_file": ctx.version_file,
        "workspace_name": "_main",
    }

    # Build settings are only available for rules for which
    # build_setting was set.
    if hasattr(ctx, "build_setting_value"):
        ctx_fields["build_setting_value"] = ctx.build_setting_value

    # If the rule has a default exec group, expose its toolchains
    # through ctx.toolchains.
    if "" in ctx.exec_groups:
        ctx_fields["toolchains"] = ctx.exec_groups[""].toolchains

    return WrappedCtx(**ctx_fields)
