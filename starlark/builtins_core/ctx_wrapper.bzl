load("@bazel_tools//fragments:fragment_info.bzl", "FragmentInfo")

def wrap_actions(actions, bin_dir, label):
    def actions_declare_shareable_artifact(path, artifact_root = None):
        if artifact_root and artifact_root != bin_dir:
            fail("artifact_root %s is not equal to bin_dir %s, which is not supported by this implementation" % (artifact_root, bin_dir))

        expected_path_prefix = label.workspace_root + "/"
        if label.package:
            expected_path_prefix += label.package
            expected_path_prefix += "/"
        if not path.startswith(expected_path_prefix):
            fail("path %s does not start with %s, which is not supported by this implementation" % (path, expected_path_prefix))

        return actions.declare_file(path.removeprefix(expected_path_prefix))

    def actions_run_shell(*, command, **kwargs):
        actions.run(
            executable = "/bin/bash",
            arguments = ["-c", command],
            **kwargs
        )

    actions_fields = {
        field: getattr(actions, field)
        for field in dir(actions)
    } | {
        "declare_shareable_artifact": actions_declare_shareable_artifact,
        "run_shell": actions_run_shell,
    }
    return struct(**actions_fields)

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
    def ctx_coverage_instrumented(target = None):
        return False

    def ctx_expand_location(input, targets = []):
        # TODO: Actually expand locations!
        return input

    ctx_fields = {
        field: getattr(ctx, field)
        for field in dir(ctx)
        # TODO: Remove this once they are gone.
        if field not in ["configuration", "var"]
    } | {
        "_real_ctx": ctx,
        "actions": wrap_actions(ctx.actions, ctx.bin_dir, ctx.label),
        "coverage_instrumented": ctx_coverage_instrumented,
        "disabled_features": [],
        "expand_location": ctx_expand_location,
        "features": [],
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

    # If the rule depends on one or more fragments, an attribute with
    # name "_ctx_fragments" of type attr.label_list() is injected. The
    # default value of this attribute will refer to targets offering a
    # FragmentInfo. Make these available through ctx.fragments.
    if hasattr(ctx.attr, "_ctx_fragments"):
        ctx_fields["fragments"] = struct(**{
            fragment.label.name: fragment[FragmentInfo]
            for fragment in ctx.attr._ctx_fragments
        })

    return WrappedCtx(**ctx_fields)
