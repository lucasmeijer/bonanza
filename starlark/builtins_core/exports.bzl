AnalysisFailureInfo = provider()
AnalysisTestResultInfo = provider()
CcLauncherInfo = provider()
CcToolchainConfigInfo = provider()
CcToolchainInfo = provider()
ConfigSettingInfo = provider()
ConstraintSettingInfo = provider()
ConstraintValueInfo = provider()
DebugPackageInfo = provider()
DeclaredToolchainInfo = provider()
ExecutionInfo = provider()
FeatureFlagInfo = provider()
FilesToRunProvider = provider()
InstrumentedFilesInfo = provider()
JavaPluginInfo = provider()
OutputGroupInfo = provider(dict_like = True)
PackageSpecificationInfo = provider()
PlatformInfo = provider()
ProguardSpecProvider = provider()
PyInfo = provider()
StaticallyLinkedMarkerProvider = provider()
ToolchainInfo = provider()
ToolchainTypeInfo = provider()

def _cc_info_init(
        *,
        cc_native_library_info = None,
        compilation_context = None,
        debug_context = None,
        linking_context = None):
    if not debug_context:
        debug_context = builtins_internal_cc_common_create_debug_context()
    transitive_native_libraries = cc_native_library_info.libraries_to_link if cc_native_library_info else depset()
    return {
        "debug_context": lambda: debug_context,
        "compilation_context": compilation_context or builtins_internal_cc_common_create_compilation_context(),
        "linking_context": linking_context or builtins_internal_cc_common_create_linking_context(),
        "transitive_native_libraries": lambda: transitive_native_libraries,
    }

CcInfo, _CcInfoRaw = provider(init = _cc_info_init)

def _cc_native_library_info_init(*, libraries_to_link = None):
    return {
        "libraries_to_link": libraries_to_link or depset(),
    }

CcNativeLibraryInfo, _CcNativeLibraryInfoRaw = provider(init = _cc_native_library_info_init)

def _default_info_init(*, data_runfiles = None, default_runfiles = None, executable = None, files = None, runfiles = None):
    if runfiles:
        if data_runfiles or default_runfiles:
            fail("cannot specify \"runfiles\" together with \"data_runfiles\" or \"default_runfiles\"")
        default_runfiles = runfiles

    return {
        "data_runfiles": data_runfiles,
        "default_runfiles": default_runfiles,
        "files": files,
        "files_to_run": FilesToRunProvider(
            executable = executable,
            repo_mapping_manifest = None,
            runfiles_manifest = None,
        ),
    }

DefaultInfo, _DefaultInfoRaw = provider(init = _default_info_init)

def _run_environment_info_init(environment = {}, inherited_environment = []):
    return {
        "environment": environment,
        "inherited_environment": inherited_environment,
    }

RunEnvironmentInfo, _RunEnvironmentInfoRaw = provider(init = _run_environment_info_init)

def _template_variable_info_init(variables):
    return {"variables": variables}

TemplateVariableInfo, _TemplateVariableInfoRaw = provider(init = _template_variable_info_init)

def _cc_libc_top_alias_impl(ctx):
    fail("TODO")

cc_libc_top_alias = rule(
    implementation = _cc_libc_top_alias_impl,
    default_exec_group = False,
)

def _cc_proto_library_impl(ctx):
    fail("TODO")

cc_proto_library = rule(
    implementation = _cc_proto_library_impl,
    attrs = {
        "deps": attr.label_list(),
    },
    default_exec_group = False,
)

def cc_toolchain_suite(**kwargs):
    pass

def _get_effective_constraint_value(constraint_setting, value_label):
    if value_label == constraint_setting.default_constraint_value:
        # Constraint value is equal to its default value, meaning it
        # should not be encoded explicitly as part of the configuration.
        # Require that the constraint setting is absent.
        return None
    return value_label

def _config_setting_impl(ctx):
    return [ConfigSettingInfo(
        # Convert constraint values to a dictionary of constraint
        # settings to values. If the provided constraint value is the
        # default we set it to None, because it effectively means the
        # constraint setting should not be part of the configuration.
        constraints = {
            constraint_value[ConstraintValueInfo].constraint.label: _get_effective_constraint_value(
                constraint_value[ConstraintValueInfo].constraint,
                constraint_value[ConstraintValueInfo].label,
            )
            for constraint_value in ctx.attr.constraint_values
        },
        flag_values = {
            key.label: value
            for key, value in ctx.attr.flag_values.items()
        },
    )]

def _config_setting_init(**kwargs):
    # The "values" attr can be used to refer to command line options
    # that are integrated into Bazel. In our case we declare all of them
    # as build settings under @bazel_tools//command_line_option. This
    # allows us to simply remap "values" to "flag_values".
    kwargs["flag_values"] = kwargs.get("flag_values", {}) | {
        "@bazel_tools//command_line_option:" + option: value
        for option, value in kwargs.get("values", {}).items()
    }
    return kwargs

config_setting = rule(
    implementation = _config_setting_impl,
    attrs = {
        "constraint_values": attr.label_list(
            cfg = config.none(),
            providers = [ConstraintValueInfo],
        ),
        # TODO: Do we even want to support define_values?
        "define_values": attr.string_dict(),
        "flag_values": attr.label_keyed_string_dict(
            # We are only interested in obtaining the build setting
            # values, which doesn't require these targets to be
            # configured.
            cfg = config.unconfigured(),
        ),
        "values": attr.string_dict(),
    },
    default_exec_group = False,
    initializer = _config_setting_init,
    provides = [ConfigSettingInfo],
)

def configuration_field(fragment, name):
    # Don't provide actual support for late-bound defaults. Instead map
    # each of them to the respective command line option used by Bazel.
    if fragment == "apple":
        return Label("@bazel_tools//command_line_option:xcode_version_config")
    if fragment == "bazel_py":
        if name == "python_top":
            return Label("@bazel_tools//command_line_option:python_top")
    if fragment == "coverage":
        if name == "output_generator":
            # This configuration field should not map to lcov_merger if
            # coverage is disabled, as that would cause cyclic
            # dependencies otherwise. Let this map to an alias that only
            # points to lcov_merger if --collect_code_coverage is set.
            return Label("@bazel_tools//tools/coverage:coverage_output_generator")
    if fragment == "cpp":
        if name == "cs_fdo_profile":
            return Label("@bazel_tools//command_line_option:cs_fdo_profile")
        if name == "custom_malloc":
            return Label("@bazel_tools//command_line_option:custom_malloc")
        if name == "fdo_optimize":
            return Label("@bazel_tools//command_line_option:fdo_optimize")
        if name == "fdo_prefetch_hints":
            return Label("@bazel_tools//command_line_option:fdo_prefetch_hints")
        if name == "fdo_profile":
            return Label("@bazel_tools//command_line_option:fdo_profile")
        if name == "libc_top":
            return Label("@bazel_tools//command_line_option:grte_top")
        if name == "memprof_profile":
            return Label("@bazel_tools//command_line_option:memprof_profile")
        if name == "propeller_optimize":
            return Label("@bazel_tools//command_line_option:propeller_optimize")
        if name == "proto_profile_path":
            return Label("@bazel_tools//command_line_option:proto_profile_path")
        if name == "target_libc_top_DO_NOT_USE_ONLY_FOR_CC_TOOLCHAIN":
            return None
        if name == "xbinary_fdo":
            return Label("@bazel_tools//command_line_option:xbinary_fdo")
        if name == "zipper":
            return None
    if fragment == "java":
        if name == "launcher":
            return Label("@bazel_tools//command_line_option:java_launcher")
        if name == "java_toolchain_bytecode_optimizer":
            return Label("@bazel_tools//command_line_option:proguard_top")
        if name == "local_java_optimization_configuration":
            return Label("@bazel_tools//command_line_option:experimental_local_java_optimization_configuration")
    if fragment == "proto":
        if name == "proto_compiler":
            return Label("@bazel_tools//command_line_option:proto_compiler")
        if name == "proto_toolchain_for_cc":
            return Label("@bazel_tools//command_line_option:proto_toolchain_for_cc")
        if name == "proto_toolchain_for_java":
            return Label("@bazel_tools//command_line_option:proto_toolchain_for_java")
        if name == "proto_toolchain_for_java_lite":
            return Label("@bazel_tools//command_line_option:proto_toolchain_for_javalite")
    if fragment == "py":
        if name == "native_rules_allowlist":
            return Label("@bazel_tools//command_line_option:python_native_rules_allowlist")

    fail("this implementation of configuration_field() does not support fragment %s and name %s" % (fragment, name))

def _constraint_setting_impl(ctx):
    default_constraint_value = ctx.attr.default_constraint_value
    return [ConstraintSettingInfo(
        default_constraint_value = default_constraint_value.label if default_constraint_value else None,
        has_default_constraint_value = bool(default_constraint_value),
        label = ctx.label,
    )]

constraint_setting = rule(
    implementation = _constraint_setting_impl,
    attrs = {
        "default_constraint_value": attr.label(
            # Prevent cyclic dependency between the constraint_setting()
            # and the default constraint_value().
            cfg = config.unconfigured(),
            providers = [ConstraintValueInfo],
        ),
    },
    default_exec_group = False,
    provides = [ConstraintSettingInfo],
)

def _constraint_value_impl(ctx):
    constraint_setting = ctx.attr.constraint_setting[ConstraintSettingInfo]
    return [
        # Also provide a ConfigSettingInfo containing just this
        # constraint. This allows constraint values to be passed to
        # select() directly.
        ConfigSettingInfo(
            constraints = {
                constraint_setting.label: _get_effective_constraint_value(constraint_setting, ctx.label),
            },
            flag_values = {},
        ),
        ConstraintValueInfo(
            constraint = constraint_setting,
            label = ctx.label,
        ),
    ]

constraint_value = rule(
    implementation = _constraint_value_impl,
    attrs = {
        "constraint_setting": attr.label(
            mandatory = True,
            providers = [ConstraintSettingInfo],
        ),
    },
    default_exec_group = False,
    provides = [ConfigSettingInfo, ConstraintValueInfo],
)

def _filegroup_impl(ctx):
    files = []
    runfiles = []
    if ctx.attr.output_group:
        for src in ctx.attr.srcs:
            files.append(getattr(src[OutputGroupInfo], ctx.attr.output_group))
    elif len(ctx.attr.srcs) == 1:
        # If exactly one target is provided, return the original
        # DefaultInfo. This ensures that fields like files_to_run are
        # preserved.
        return ctx.attr.srcs[0][DefaultInfo]
    else:
        for src in ctx.attr.srcs:
            default_info = src[DefaultInfo]
            files.append(default_info.files)
            runfiles.append(default_info.default_runfiles)

    for data in ctx.attr.data:
        runfiles.append(data[DefaultInfo].default_runfiles)

    return [DefaultInfo(
        files = depset(direct = [], transitive = files),
        runfiles = ctx.runfiles(
            files = ctx.files.data,
        ).merge_all(runfiles),
    )]

filegroup = rule(
    implementation = _filegroup_impl,
    attrs = {
        "data": attr.label_list(allow_files = True),
        "output_group": attr.string(),
        "srcs": attr.label_list(allow_files = True),
    },
    default_exec_group = False,
)

def _genrule_impl(ctx):
    # TODO: Make this implementation more accurate.
    ctx.actions.run(
        executable = "sh",
        arguments = ["-c", ctx.expand_location(ctx.attr.cmd_bash or ctx.attr.cmd)],
        outputs = ctx.outputs.outs,
    )
    return [DefaultInfo(files = depset(ctx.outputs.outs))]

genrule = rule(
    implementation = _genrule_impl,
    attrs = {
        "cmd": attr.string(),
        "cmd_bash": attr.string(),
        "cmd_bat": attr.string(),
        "cmd_ps": attr.string(),
        "executable": attr.bool(),
        "local": attr.bool(),
        "message": attr.string(),
        "output_licenses": attr.string_list(),
        "output_to_bindir": attr.bool(),
        "outs": attr.output_list(mandatory = True),
        "srcs": attr.label_list(allow_files = True),
        "tools": attr.label_list(allow_files = True),
    },
    default_exec_group = False,
)

def _java_plugins_flag_alias_impl(ctx):
    return [JavaPluginInfo()]

java_plugins_flag_alias = rule(
    implementation = _java_plugins_flag_alias_impl,
    default_exec_group = False,
)

def _java_proto_library_impl(ctx):
    fail("TODO")

java_proto_library = rule(
    implementation = _java_proto_library_impl,
    attrs = {
        "deps": attr.label_list(),
    },
    default_exec_group = False,
)

def licenses(license_types):
    # This function is deprecated. Licenses can nowadays be attached in
    # the form of metadata. Provide a no-op stub.
    pass

def _platform_impl(ctx):
    # Convert all constraint values to a dict mapping the constraint
    # setting to the corresponding value.
    constraints = {}
    for value in ctx.attr.constraint_values:
        value_info = value[ConstraintValueInfo]
        setting_label = value_info.constraint.label
        value_label = value_info.label
        if setting_label in constraints:
            fail("constraint_values contains multiple values for constraint setting %s: %s and %s" % (
                setting_label,
                constraints[setting_label],
                value_label,
            ))
        constraints[setting_label] = _get_effective_constraint_value(value_info.constraint, value_label)

    exec_pkix_public_key = ctx.attr.exec_pkix_public_key
    repository_os_arch = ctx.attr.repository_os_arch
    repository_os_environ = ctx.attr.repository_os_environ
    repository_os_name = ctx.attr.repository_os_name

    # Inherit properties from the parent platform.
    if ctx.attr.parents:
        if len(ctx.attr.parents) != 1:
            fail("providing multiple parents is not supported")
        parent = ctx.attr.parents[0][PlatformInfo]
        constraints = parent.constraints | constraints
        exec_pkix_public_key = exec_pkix_public_key or parent.exec_pkix_public_key
        repository_os_arch = repository_os_arch or parent.repository_os_arch
        repository_os_environ = repository_os_environ or parent.repository_os_environ
        repository_os_name = repository_os_name or parent.repository_os_name

    return [PlatformInfo(
        constraints = {
            setting: value
            for setting, value in constraints.items()
            if value
        },
        exec_pkix_public_key = exec_pkix_public_key,
        repository_os_arch = repository_os_arch,
        repository_os_environ = repository_os_environ,
        repository_os_name = repository_os_name,
    )]

platform = rule(
    implementation = _platform_impl,
    attrs = {
        "constraint_values": attr.label_list(
            doc = """
            The combination of constraint choices that this platform
            comprises. In order for a platform to apply to a given
            environment, the environment must have at least the values
            in this list.

            Each constraint_value in this list must be for a different
            constraint_setting. For example, you cannot define a
            platform that requires the cpu architecture to be both
            @platforms//cpu:x86_64 and @platforms//cpu:arm.
            """,
            providers = [ConstraintValueInfo],
        ),
        "exec_pkix_public_key": attr.string(
            doc = """
            When the platform is used for execution, the X25519 public
            key in PKIX form that identifies the execution platform. The
            key needs to be provided in base64 encoded form, without the
            PEM header/footer.
            """,
        ),
        "parents": attr.label_list(
            doc = """
            The label of a platform target that this platform should
            inherit from. Although the attribute takes a list, there
            should be no more than one platform present. Any
            constraint_settings not set directly on this platform will
            be found in the parent platform. See the section on Platform
            Inheritance for details.
            """,
            providers = [PlatformInfo],
        ),
        "repository_os_arch": attr.string(
            doc = """
            If this platform is used as a platform for executing
            commands as part of module extensions or repository rules,
            the name of the architecture to announce via
            repository_os.arch.

            This attribute should match the value of the "os.arch" Java
            property converted to lower case (e.g., "aarch64" for ARM64,
            "amd64" for x86-64, "x86" for x86-32).
            """,
        ),
        "repository_os_environ": attr.string_dict(
            doc = """
            If this platform is used as a platform for executing
            commands as part of module extensions or repository rules,
            environment variables to announce via repository_os.environ.
            """,
        ),
        "repository_os_name": attr.string(
            doc = """
            If this platform is used as a platform for executing
            commands as part of module extensions or repository rules,
            the operating system name to announce via
            repository_os.name.

            This attribute should match the value of the "os.name" Java
            property converted to lower case (e.g., "linux", "mac os x",
            "windows 10").
            """,
        ),
    },
    default_exec_group = False,
    provides = [PlatformInfo],
)

def _sh_test_impl(ctx):
    fail("TODO: implement")

sh_test = rule(
    _sh_test_impl,
    attrs = {
        "data": attr.label_list(allow_files = True),
        "deps": attr.label_list(),
        "srcs": attr.label_list(allow_files = True),
    },
    default_exec_group = False,
    test = True,
)

def _test_suite_impl(ctx):
    fail("TODO: implement")

test_suite = rule(
    _test_suite_impl,
    attrs = {
        "tests": attr.string_list(),
    },
    default_exec_group = False,
)

def _toolchain_impl(ctx):
    return [DeclaredToolchainInfo(
        target_settings = [
            target_setting.label
            for target_setting in ctx.attr.target_settings
        ],
        toolchain = ctx.attr.toolchain.label,
        toolchain_type = ctx.attr.toolchain_type[ToolchainTypeInfo].type_label,
    )]

toolchain = rule(
    implementation = _toolchain_impl,
    attrs = {
        "target_settings": attr.label_list(
            providers = [ConfigSettingInfo],
        ),
        "toolchain": attr.label(
            # Prevent configuring toolchains that are not used.
            cfg = config.unconfigured(),
            mandatory = True,
            providers = [ToolchainInfo],
        ),
        "toolchain_type": attr.label(
            mandatory = True,
            providers = [ToolchainTypeInfo],
        ),
    },
    default_exec_group = False,
    provides = [DeclaredToolchainInfo],
)

def _toolchain_type_impl(ctx):
    return [ToolchainTypeInfo(
        type_label = ctx.label,
    )]

toolchain_type = rule(
    implementation = _toolchain_type_impl,
    default_exec_group = False,
    provides = [ToolchainTypeInfo],
)

def coverage_common_instrumented_files_info(
        ctx,
        *,
        coverage_environment = {},
        coverage_support_files = [],
        dependency_attributes = [],
        extensions = None,
        metadata_files = [],
        reported_to_actual_sources = None,
        source_attributes = []):
    return InstrumentedFilesInfo(
        # TODO: instrumented_files.
        metadata_files = depset(metadata_files),
    )

def proto_common_do_not_use_external_proto_infos():
    return []

def proto_common_do_not_use_incompatible_enable_proto_toolchain_resolution():
    # This option be controlled by command line option
    # --incompatible_enable_proto_toolchain_resolution.
    return False

def builtins_internal_apple_common_dotted_version(v):
    # TODO: Provide a proper implementation.
    return v

def builtins_internal_cc_common_action_is_enabled(*, feature_configuration, action_name):
    return feature_configuration.is_enabled(action_name)

def builtins_internal_cc_common_check_private_api(allowlist = []):
    pass

def _create_compilation_outputs(
        *,
        header_tokens,
        lto_compilation_context,
        objects,
        pic_objects):
    # TODO: Where do we get these from?
    dwo_files = depset()
    pic_dwo_files = depset()

    return struct(
        _header_tokens = header_tokens,
        _dwo_files = dwo_files,
        _objects = objects,
        _pic_dwo_files = pic_dwo_files,
        _pic_objects = pic_objects,
        lto_compilation_context = lambda: lto_compilation_context,

        # We unfortunately also need to provide access to these in the
        # form of lists.
        dwo_files = lambda: dwo_files.to_list(),
        objects = objects.to_list(),
        pic_dwo_files = lambda: pic_dwo_files.to_list(),
        pic_objects = pic_objects.to_list(),

        # TODO: Where do these come from?
        files_to_compile = lambda parse_headers = False, use_pic = False: depset(
            direct = [],
            transitive = [pic_objects if use_pic else objects] +
                         ([header_tokens] if parse_headers else []),
        ),
        gcno_files = lambda: [],
        header_tokens = lambda: header_tokens.to_list(),
        module_files = lambda: [],
        pic_gcno_files = lambda: [],
        temps = lambda: depset(),
    )

def feature_configuration_get_command_line(feature_configuration, action, variables):
    pass

def builtins_internal_cc_common_compile_fork(
        *,
        actions,
        cc_toolchain,
        feature_configuration,
        name,
        additional_exported_hdrs = None,
        additional_include_scanning_roots = [],
        additional_inputs = [],
        additional_module_maps = [],
        code_coverage_enabled = None,
        compilation_contexts = [],
        conly_flags = [],
        copts_filter = None,
        cxx_flags = [],
        defines = [],
        disallow_nopic_outputs = False,
        disallow_pic_outputs = False,
        do_not_generate_module_map = None,
        framework_includes = [],
        hdrs_checking_mode = None,
        implementation_compilation_contexts = [],
        include_prefix = "",
        includes = [],
        language = None,
        local_defines = [],
        loose_includes = [],
        module_interfaces = [],
        module_map = None,
        non_compilation_additional_inputs = [],
        private_hdrs = [],
        propagate_module_map_to_compile_action = None,
        public_hdrs = [],
        purpose = None,
        quote_includes = [],
        separate_module_headers = [],
        srcs = [],
        strip_include_prefix = "",
        system_includes = [],
        textual_hdrs = [],
        user_compile_flags = [],
        variables_extension = None):
    merged_compilation_context = builtins_internal_cc_common_merge_compilation_contexts(
        compilation_contexts,
        implementation_compilation_contexts,
    )

    objects = []
    pic_objects = []
    for src in srcs:
        p = src[0].path
        if p.endswith(".c"):
            action_name = "c-compile"
        elif p.endswith(".cc"):
            action_name = "c++-compile"
        else:
            fail(src)
        base = p.rsplit(".", 1)[0]

        object_file = actions.declare_file(base + ".o")
        variables = builtins_internal_cc_common_create_compile_variables(
            cc_toolchain = cc_toolchain,
            feature_configuration = feature_configuration,
            source_file = p,
            output_file = object_file.path,
            user_compile_flags = user_compile_flags,
            include_directories = merged_compilation_context.includes,
            quote_include_directories = merged_compilation_context.quote_includes,
            system_include_directories = merged_compilation_context.system_includes,
            framework_include_directories = merged_compilation_context.framework_includes,
            preprocessor_defines = merged_compilation_context.defines,
            use_pic = False,
            variables_extension = variables_extension,
        )
        actions.run(
            executable = builtins_internal_cc_common_get_tool_for_action(feature_configuration, action_name),
            arguments = builtins_internal_cc_common_get_memory_inefficient_command_line(feature_configuration, action_name, variables),
            inputs = [src[0]],
            outputs = [object_file],
        )
        objects.append(object_file)

        pic_object_file = actions.declare_file(base + ".pic.o")
        pic_variables = builtins_internal_cc_common_create_compile_variables(
            cc_toolchain = cc_toolchain,
            feature_configuration = feature_configuration,
            source_file = p,
            output_file = pic_object_file.path,
            user_compile_flags = user_compile_flags,
            include_directories = merged_compilation_context.includes,
            quote_include_directories = merged_compilation_context.quote_includes,
            system_include_directories = merged_compilation_context.system_includes,
            framework_include_directories = merged_compilation_context.framework_includes,
            preprocessor_defines = merged_compilation_context.defines,
            use_pic = True,
            variables_extension = variables_extension,
        )
        actions.run(
            executable = builtins_internal_cc_common_get_tool_for_action(feature_configuration, action_name),
            arguments = builtins_internal_cc_common_get_memory_inefficient_command_line(feature_configuration, action_name, pic_variables),
            inputs = [src[0]],
            outputs = [pic_object_file],
        )
        pic_objects.append(pic_object_file)

    # TODO: Fill this in properly.
    compilation_context = builtins_internal_cc_common_create_compilation_context(
        headers = depset(public_hdrs),
    )

    # TODO: Fill this in properly.
    srcs_compilation_outputs = _create_compilation_outputs(
        lto_compilation_context = struct(TODO_lto_compilation_context = True),
        header_tokens = depset(),
        objects = depset(objects),
        pic_objects = depset(pic_objects),
    )

    return compilation_context, srcs_compilation_outputs

def _selectable_get_name(selectable):
    if selectable.type_name == "action_config":
        return selectable.action_name
    return selectable.name

def feature_configuration_is_enabled(feature_configuration):
    enabled_feature_names = feature_configuration._enabled_feature_names
    return lambda feature: feature in enabled_feature_names

FeatureConfiguration = provider(
    computed_fields = {
        "is_enabled": feature_configuration_is_enabled,
    },
)

def _get_feature_configuration(
        requested_features,
        selectables_by_name,
        selectables,
        provides,
        implies,
        implied_by,
        requires,
        required_by,
        action_configs_by_action_name,
        cc_toolchain_path):
    requested_selectables = set([
        name
        for name in requested_features
        if name in selectables_by_name
    ])

    enabled = set()

    def enable_all_implied_by(selectable):
        # Bazel's implementation uses recursion, which Starlark does not
        # permit. Add some dummy loops to work around this.
        queue = set([selectable])
        for dummy1 in selectables_by_name:
            for dummy2 in selectables_by_name:
                if not queue:
                    return
                selectable = queue.pop()
                if selectable not in enabled:
                    enabled.add(selectable)
                    for implied in implies.get(selectable, set()):
                        queue.add(implied)
        fail("enable_all_implied_by failed to process all selectables")

    def is_implied_by_enabled_activatable(selectable):
        # TODO: Use set.isdisjoint(), which starlark-go does not support.
        return bool(implied_by[selectable].intersection(enabled))

    def all_implications_enabled(selectable):
        for implied in implies.get(selectable, set()):
            if implied not in enabled:
                return False
        return True

    def all_requirements_met(feature):
        if feature not in requires:
            return True
        for requires_all_of in requires[feature]:
            requirement_met = True
            for required in requires_all_of:
                if not required in enabled:
                    requirement_met = False
            if requirement_met:
                return True
        return False

    def is_satisfied(selectable):
        return (
            (selectable in requested_selectables or is_implied_by_enabled_activatable(selectable)) and
            all_implications_enabled(selectable) and
            all_requirements_met(selectable)
        )

    def check_activatable(selectable):
        if selectable not in enabled or is_satisfied(selectable):
            return
        enabled.remove(selectable)

        for implies_current in implied_by[selectable]:
            check_activatable(implies_current)
        for requires_current in required_by[selectable]:
            check_activatable(requires_current)
        for implied in implies[selectable]:
            check_activatable(implied)

    def disable_unsupported_activatables():
        check = set(enabled)
        for i in check:
            check_activatable(i)

    def is_feature(activatable):
        return {
            "action_config": False,
            "feature": True,
        }[activatable.type_name]

    def is_action_config(activatable):
        return {
            "action_config": True,
            "feature": False,
        }[activatable.type_name]

    # From FeatureSelection.run():
    for selectable in requested_selectables:
        enable_all_implied_by(selectable)
    disable_unsupported_activatables()
    enabled_activatables_in_order_builder = []
    for selectable in selectables:
        if _selectable_get_name(selectable) in enabled:
            enabled_activatables_in_order_builder.append(selectable)

    enabled_activatables_in_order = enabled_activatables_in_order_builder
    enabled_features_in_order = [
        activatable
        for activatable in enabled_activatables_in_order
        if is_feature(activatable)
    ]
    enabled_action_configs_in_order = [
        activatable
        for activatable in enabled_activatables_in_order
        if is_action_config(activatable)
    ]

    for provided in provides:
        conflicts = []
        for selectable_providing_string in provides[provided]:
            if selectable_providing_string in enabled_activatables_in_order:
                conflicts.append(selectable_providing_string.name)

        if len(conflicts) > 1:
            fail("Symbol %s is provided by all of the following features: %s" % (provided, " ".join(conflicts)))

    enabled_action_config_names = set([
        action_config.action_name
        for action_config in enabled_action_configs_in_order
    ])

    enabled_feature_names = set([
        feature.name
        for feature in enabled_features_in_order
    ])
    return FeatureConfiguration(
        _action_config_by_action_name = action_configs_by_action_name,
        _enabled_action_config_action_names = enabled_action_config_names,
        _enabled_feature_names = enabled_feature_names,
        is_requested = lambda feature: feature in requested_features,
    )

def builtins_internal_cc_common_configure_features(
        ctx,
        cc_toolchain = None,
        language = None,
        requested_features = [],
        unsupported_features = []):
    # From CcCommon.configureFeaturesOrThrowEvalException():
    cpp_configuration = cc_toolchain._cpp_configuration

    all_requested_features_builder = set()
    unsupported_features_builder = set(unsupported_features)
    if not cc_toolchain._supports_header_parsing:
        unsupported_features_builder.add("parse_headers")

    if (
        language not in ["objc", "objc++"] and
        not cc_toolchain._cc_info.compilation_context.module_map
    ):
        unsupported_features_builder.add("module_maps")

    if cpp_configuration.force_pic:
        if "supports_pic" in unsupported_features_builder:
            fail("PIC compilation is requested but the toolchain does not support it (feature named 'supports_pic' is not enabled)")

        all_requested_features_builder.add("supports_pic")

    if cpp_configuration.apple_generate_dsym:
        all_requested_features_builder.add("generate_dsym_file")
    else:
        all_requested_features_builder.add("no_generate_debug_symbols")

    if language in ["objc", "objc++"]:
        all_requested_features_builder.add("lang_objc")
        if cpp_configuration.objc_generate_linkmap:
            all_requested_features_builder.add("generate_linkmap")
        if cpp_configuration.objc_should_strip_binary:
            all_requested_features_builder.add("dead_strip")

    all_unsupported_features = unsupported_features_builder

    toolchain_features = cc_toolchain._toolchain_features
    all_features = (
        [
            cpp_configuration.compilation_mode(),
            # ALL_COMPILE_ACTIONS:
            "c-compile",
            "c++-compile",
            "c++-header-parsing",
            "c++-module-compile",
            "c++-module-codegen",
            "c++-module-deps-scanning",
            "c++20-module-compile",
            "c++20-module-codegen",
            "assemble",
            "preprocess-assemble",
            "clif-match",
            "linkstamp-compile",
            "cc-flags-make-variable",
            "lto-backend",
            "c++-header-analysis",
            # ALL_LINK_ACTIONS:
            "lto-index-for-executable",
            "lto-index-for-dynamic-library",
            "lto-index-for-nodeps-dynamic-library",
            "c++-link-executable",
            "c++-link-dynamic-library",
            "c++-link-nodeps-dynamic-library",
            # ALL_ARCHIVE_ACTIONS:
            "c++-link-static-library",
            # ALL_OTHER_ACTIONS:
            "strip",
        ] +
        requested_features +
        toolchain_features._default_selectables
    )

    if language in ["objc", "objc++"]:
        all_features += [
            "objc-compile",
            "objc++-compile",
            "objc-fully-link",
            "objc-executable",
        ]

    if not cpp_configuration.dont_enable_host_nonhost:
        if cc_toolchain._configuration.is_tool_configuration:
            all_features.append("host")
        else:
            all_features.append("host")

    if cpp_configuration.collect_code_coverage:
        all_features.append("coverage")
        if cpp_configuration.use_llvm_coverage_map_format:
            all_features.append("llvm_coverage_map_format")
        else:
            all_features.append("gcc_coverage_map_format")

    if "fdo_instrument" not in all_unsupported_features:
        if cpp_configuration.fdo_instrument:
            all_features += ["fdo_instrument"]
        elif cpp_configuration.cs_fdo_instrument:
            all_features += ["cs_fdo_instrument"]

    branch_fdo_provider = getattr(cc_toolchain._fdo_context, "branch_fdo_profile", None)

    propeller_optimize_info = getattr(cc_toolchain._fdo_context, "propeller_optimize_info", None)
    enable_propeller_optimize = (
        propeller_optimize_info and
        (
            propeller_optimize_info.cc_artifact or
            propeller_optimize_info.ld_artifact
        )
    )

    if branch_fdo_provider and cpp_configuration.compilation_mode == "opt":
        fail("TODO: add FDO related features")
    if cpp_configuration.fdo_prefetch_hints:
        all_requested_features_builder.add("fdo_prefetch_hints")

    if enable_propeller_optimize:
        all_requested_features_builder.add("propeller_optimize")

    for feature in all_features:
        if feature not in all_unsupported_features:
            all_requested_features_builder.add(feature)

    feature_configuration = _get_feature_configuration(
        all_requested_features_builder,
        toolchain_features._selectables_by_name,
        toolchain_features._selectables,
        toolchain_features._provides,
        toolchain_features._implies,
        toolchain_features._implied_by,
        toolchain_features._requires,
        toolchain_features._required_by,
        toolchain_features._action_configs_by_action_name,
        toolchain_features._cc_toolchain_path,
    )
    for feature in unsupported_features:
        if feature_configuration.is_enabled(feature):
            fail("The C++ toolchain '%s' unconditionally implies feature '%s', which is unsupported by this rule. This is most likely a misconfiguration in the C++ toolchain." % toolchain.get_cc_toolchain_label(), feature)
    if (
        cpp_configuration.force_pic and
        not feature_configuration.is_enabled("pic") and
        not feature_configuration.is_enabled("supports_pic")
    ):
        fail("PIC compilation is requested but the toolchain does not support it (feature named 'supports_pic' is not enabled)")
    return feature_configuration

def _feature(
        name,
        enabled = False,
        flag_sets = [],
        env_sets = [],
        requires = [],
        implies = [],
        provides = []):
    return struct(
        name = name,
        enabled = enabled,
        flag_sets = flag_sets,
        env_sets = env_sets,
        requires = requires,
        implies = implies,
        provides = provides,
        type_name = "feature",
    )

def _flag_group(
        flags = [],
        flag_groups = [],
        iterate_over = None,
        expand_if_available = None,
        expand_if_not_available = None,
        expand_if_true = None,
        expand_if_false = None,
        expand_if_equal = None):
    return struct(
        flags = flags,
        flag_groups = flag_groups,
        iterate_over = iterate_over,
        expand_if_available = expand_if_available,
        expand_if_not_available = expand_if_not_available,
        expand_if_true = expand_if_true,
        expand_if_false = expand_if_false,
        expand_if_equal = expand_if_equal,
        type_name = "flag_group",
    )

def _flag_set(
        actions = [],
        with_features = [],
        flag_groups = []):
    return struct(
        actions = actions,
        with_features = with_features,
        flag_groups = flag_groups,
        type_name = "flag_set",
    )

def _variable_with_value(name, value):
    return struct(
        name = name,
        value = value,
        type_name = "variable_with_value",
    )

def _with_feature_set(features = [], not_features = []):
    return struct(
        features = features,
        not_features = not_features,
        type_name = "with_feature_set",
    )

def _action_config(
        action_name,
        enabled = False,
        tools = [],
        flag_sets = [],
        implies = []):
    return struct(
        action_name = action_name,
        enabled = enabled,
        tools = tools,
        flag_sets = flag_sets,
        implies = implies,
        type_name = "action_config",
    )

def _tool(path = None, with_features = [], execution_requirements = [], tool = None):
    return struct(
        path = path,
        tool = tool,
        with_features = with_features,
        execution_requirements = execution_requirements,
        type_name = "tool",
    )

def _path_relative_to_package(ctx, path):
    if path.startswith("/"):
        return path

    return "/".join(
        [
            component
            for component in (
                ctx.label.workspace_root.split("/") +
                ctx.label.package.split("/") +
                path.split("/")
            )
            if component
        ],
    )

def builtins_internal_cc_common_create_cc_toolchain_config_info(
        ctx,
        toolchain_identifier,
        compiler,
        features = [],
        action_configs = [],
        artifact_name_patterns = [],
        cxx_builtin_include_directories = [],
        host_system_name = None,
        target_system_name = None,
        target_cpu = None,
        target_libc = None,
        abi_version = None,
        abi_libc_version = None,
        tool_paths = [],
        make_variables = [],
        builtin_sysroot = None):
    feature_names = set([feature.name for feature in features])
    if "no_legacy_features" not in feature_names:
        gcc_tool_path = "DUMMY_GCC_TOOL"
        linker_tool_path = "DUMMY_LINKER_TOOL"
        ar_tool_path = "DUMMY_AR_TOOL"
        strip_tool_path = "DUMMY_STRIP_TOOL"
        for tool in tool_paths:
            if tool.name == "gcc":
                gcc_tool_path = tool.path
                linker_tool_path = _path_relative_to_package(ctx, tool.path)
            elif tool.name == "ar":
                ar_tool_path = tool.path
            elif tool.name == "strip":
                strip_tool_path = tool.path

        legacy_features_builder = [
            feature
            for feature in features
            if feature.name == "legacy_compile_flags"
        ][:1] + [
            feature
            for feature in features
            if feature.name == "default_compile_flags"
        ][:1]

        if "legacy_compile_flags" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "legacy_compile_flags",
                enabled = True,
                flag_sets = [_flag_set(
                    actions = [
                        "assemble",
                        "c-compile",
                        "c++-compile",
                        "c++-header-parsing",
                        "c++-module-codegen",
                        "c++-module-compile",
                        "clif-match",
                        "linkstamp-compile",
                        "lto-backend",
                        "preprocess-assemble",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "legacy_compile_flags",
                        iterate_over = "legacy_compile_flags",
                        flags = ["%{legacy_compile_flags}"],
                    )],
                )],
            ))
        if "dependency_file" not in feature_names:
            fail("TODO: dependency_file")
        if "random_seed" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "random_seed",
                enabled = True,
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                        "c++-module-codegen",
                        "c++-module-compile",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "output_file",
                        flags = ["-frandom-seed=%{output_file}"],
                    )],
                )],
            ))
        if "pic" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "pic",
                enabled = True,
                flag_sets = [_flag_set(
                    actions = [
                        "assemble",
                        "c-compile",
                        "c++-compile",
                        "c++-module-codegen",
                        "c++-module-compile",
                        "linkstamp-compile",
                        "preprocess-assemble",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "pic",
                        flags = ["-fPIC"],
                    )],
                )],
            ))
        if "per_object_debug_info" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "per_object_debug_info",
                flag_sets = [_flag_set(
                    actions = [
                        "assemble",
                        "c-compile",
                        "c++-compile",
                        "c++-module-codegen",
                        "preprocess-assemble",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "per_object_debug_info_file",
                        flags = ["-gsplit-dwarf", "-g"],
                    )],
                )],
            ))
        if "preprocessor_defines" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "preprocessor_defines",
                enabled = True,
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                        "c++-header-parsing",
                        "c++-module-compile",
                        "clif-match",
                        "linkstamp-compile",
                        "preprocess-assemble",
                    ],
                    flag_groups = [_flag_group(
                        iterate_over = "preprocessor_defines",
                        flags = ["-D%{preprocessor_defines}"],
                    )],
                )],
            ))
        if "includes" not in feature_names:
            fail("TODO: includes")
        if "include_paths" not in feature_names:
            fail("TODO: include_paths")
        if "fdo_instrument" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "fdo_instrument",
                provides = ["profile"],
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "fdo_instrument_path",
                        flags = [
                            "-fprofile-generate=%{fdo_instrument_path}",
                            "-fno-data-sections",
                        ],
                    )],
                )],
            ))
        if "fdo_optimize" not in feature_names:
            fail("TODO: fdo_optimize")
        if "cs_fdo_instrument" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "cs_fdo_instrument",
                provides = ["csprofile"],
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-backend",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "cs_fdo_instrument_path",
                        flags = ["-fcs-profile-generate=%{cs_fdo_instrument_path}"],
                    )],
                )],
            ))
        if "cs_fdo_optimize" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "cs_fdo_optimize",
                provides = ["csprofile"],
                flag_sets = [_flag_set(
                    actions = ["lto-backend"],
                    flag_groups = [_flag_group(
                        expand_if_available = "fdo_profile_path",
                        flags = [
                            "-fprofile-use=%{fdo_profile_path}",
                            "-Wno-profile-instr-unprofiled",
                            "-Wno-profile-instr-out-of-date",
                            "-fprofile-correction",
                        ],
                    )],
                )],
            ))
        if "fdo_prefetch_hints" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "fdo_prefetch_hints",
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                        "lto-backend",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "fdo_prefetch_hints_path",
                        flags = [
                            "-mllvm",
                            "-prefetch-hints-file=%{fdo_prefetch_hints_path}",
                        ],
                    )],
                )],
            ))
        if "autofdo" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "autofdo",
                provides = ["profile"],
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "fdo_profile_path",
                        flags = [
                            "-fauto-profile=%{fdo_profile_path}",
                            "-fprofile-correction",
                        ],
                    )],
                )],
            ))
        if "propeller_optimize_thinlto_compile_actions" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "propeller_optimize_thinlto_compile_actions",
            ))
        if "propeller_optimize" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "propeller_optimize",
                flag_sets = [
                    _flag_set(
                        actions = [
                            "c-compile",
                            "c++-compile",
                            "lto-compile",
                        ],
                        flag_groups = [_flag_group(
                            expand_if_available = "propeller_optimize_cc_path",
                            flags = [
                                "-fbasic-block-sections=list=%{propeller_optimize_cc_path}",
                                "-DBUILD_PROPELLER_ENABLED=1",
                            ],
                        )],
                    ),
                    _flag_set(
                        actions = ["c++-link-executable"],
                        flag_groups = [_flag_group(
                            flags = ["-Wl,--symbol-ordering-file=%{propeller_optimize_ld_path}"],
                        )],
                    ),
                ],
            ))
        if "memprof_optimize" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "memprof_optimize",
                flag_sets = [_flag_set(
                    actions = [
                        "c-compile",
                        "c++-compile",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "memprof_profile_path",
                        flags = ["-memprof-profile-file=%{memprof_profile_path}"],
                    )],
                )],
            ))
        if "build_interface_libraries" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "build_interface_libraries",
                flag_sets = [_flag_set(
                    with_features = [_with_feature_set(
                        features = ["supports_interface_shared_libraries"],
                    )],
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "generate_interface_library",
                        flags = [
                            "%{generate_interface_library}",
                            "%{interface_library_builder_path}",
                            "%{interface_library_input_path}",
                            "%{interface_library_output_path}",
                        ],
                    )],
                )],
            ))
        if "dynamic_library_linker_tool" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "dynamic_library_linker_tool",
                flag_sets = [_flag_set(
                    with_features = [_with_feature_set(
                        features = ["supports_interface_shared_libraries"],
                    )],
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "generate_interface_library",
                        flags = [linker_tool_path],
                    )],
                )],
            ))
        if "shared_flag" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "shared_flag",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        flags = ["-shared"],
                    )],
                )],
            ))
        if "linkstamps" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "linkstamps",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "linkstamp_paths",
                        iterate_over = "linkstamp_paths",
                        flags = ["%{linkstamp_paths}"],
                    )],
                )],
            ))
        if "output_execpath_flags" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "output_execpath_flags",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "output_execpath",
                        flags = ["-o", "%{output_execpath}"],
                    )],
                )],
            ))
        if "runtime_library_search_directories" not in feature_names:
            fail("TODO: runtime_library_search_directories")
        if "library_search_directories" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "library_search_directories",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "library_search_directories",
                        iterate_over = "library_search_directories",
                        flags = ["-L%{library_search_directories}"],
                    )],
                )],
            ))
        if "archiver_flags" not in feature_names:
            fail("TODO: archiver_flags")
        if "libraries_to_link" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "libraries_to_link",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [
                        _flag_group(
                            expand_if_true = "thinlto_param_file",
                            flags = ["-Wl,@%{thinlto_param_file}"],
                        ),
                        _flag_group(
                            expand_if_available = "libraries_to_link",
                            iterate_over = "libraries_to_link",
                            flag_groups = [
                                _flag_group(
                                    expand_if_equal = _variable_with_value(
                                        name = "libraries_to_link.type",
                                        value = "object_file_group",
                                    ),
                                    expand_if_false = "libraries_to_link.is_whole_archive",
                                    flags = ["-Wl,--start-lib"],
                                ),
                            ] + (
                                ([
                                    _flag_group(
                                        expand_if_true = "libraries_to_link.is_whole_archive",
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "static_library",
                                        ),
                                        flags = ["-Wl,-whole-archive"],
                                    ),
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "object_file_group",
                                        ),
                                        iterate_over = "libraries_to_link.object_files",
                                        flags = ["%{libraries_to_link.object_files}"],
                                    ),
                                ] + [
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = libraries_to_link_type,
                                        ),
                                        flags = ["%{libraries_to_link.name}"],
                                    )
                                    for libraries_to_link_type in [
                                        "object_file",
                                        "interface_library",
                                        "static_library",
                                    ]
                                ] + [
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "dynamic_library",
                                        ),
                                        flags = ["-l%{libraries_to_link.name}"],
                                    ),
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "versioned_dynamic_library",
                                        ),
                                        flags = ["-l:%{libraries_to_link.name}"],
                                    ),
                                    _flag_group(
                                        expand_if_true = "libraries_to_link.is_whole_archive",
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "static_library",
                                        ),
                                        flags = ["-Wl,-no-whole-archive"],
                                    ),
                                ]) if target_cpu != "macosx" else ([
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "object_file_group",
                                        ),
                                        iterate_over = "libraries_to_link.object_files",
                                        flag_groups = [
                                            _flag_group(
                                                expand_if_false = "libraries_to_link.is_whole_archive",
                                                flags = ["%{libraries_to_link.object_files}"],
                                            ),
                                            _flag_group(
                                                expand_if_true = "libraries_to_link.is_whole_archive",
                                                flags = ["-Wl,-force_load,%{libraries_to_link.object_files}"],
                                            ),
                                        ],
                                    ),
                                ] + [
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = libraries_to_link_type,
                                        ),
                                        iterate_over = "libraries_to_link.object_files",
                                        flag_groups = [
                                            _flag_group(
                                                expand_if_false = "libraries_to_link.is_whole_archive",
                                                flags = ["%{libraries_to_link.name}"],
                                            ),
                                            _flag_group(
                                                expand_if_true = "libraries_to_link.is_whole_archive",
                                                flags = ["-Wl,-force_load,%{libraries_to_link.name}"],
                                            ),
                                        ],
                                    )
                                    for libraries_to_link_type in [
                                        "object_file",
                                        "interface_library",
                                        "static_library",
                                    ]
                                ] + [
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "dynamic_library",
                                        ),
                                        flags = ["-l%{libraries_to_link.name}"],
                                    ),
                                    _flag_group(
                                        expand_if_equal = _variable_with_value(
                                            name = "libraries_to_link.type",
                                            value = "versioned_dynamic_library",
                                        ),
                                        flags = ["%{libraries_to_link.path}"],
                                    ),
                                ])
                            ) + [
                                _flag_group(
                                    expand_if_equal = _variable_with_value(
                                        name = "libraries_to_link.type",
                                        value = "object_file_group",
                                    ),
                                    expand_if_false = "libraries_to_link.is_whole_archive",
                                    flags = ["-Wl,--end-lib"],
                                ),
                            ],
                        ),
                    ],
                )],
            ))
        if "force_pic_flags" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "force_pic_flags",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-executable",
                        "lto-index-for-executable",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "force_pic",
                        flags = ["-pie" if target_cpu != "macosx" else "-Wl,-pie"],
                    )],
                )],
            ))
        if "user_link_flags" not in feature_names:
            fail("TODO: user_link_flags")
        if "legacy_link_flags" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "legacy_link_flags",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "legacy_link_flags",
                        iterate_over = "legacy_link_flags",
                        flags = ["%{legacy_link_flags}"],
                    )],
                )],
            ))
        if "static_libgcc" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "static_libgcc",
                enabled = True,
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                    ],
                    with_features = [_with_feature_set(
                        features = ["static_link_cpp_runtimes"],
                    )],
                    flag_groups = [_flag_group(
                        flags = ["-static-libgcc"],
                    )],
                )],
            ))
        if "fission_support" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "fission_support",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "is_using_fission",
                        flags = ["-Wl,--gdb-index"],
                    )],
                )],
            ))
        if "strip_debug_symbols" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "strip_debug_symbols",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "c++-link-nodeps-dynamic-library",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                        "lto-index-for-nodeps-dynamic-library",
                    ],
                    flag_groups = [_flag_group(
                        expand_if_available = "strip_debug_symbols",
                        flags = ["-Wl,-S"],
                    )],
                )],
            ))
        if "coverage" not in feature_names:
            fail("TODO: coverage")

        legacy_features_builder += [
            feature
            for feature in features
            if feature.name not in ["legacy_compile_flags", "default_compile_flags"]
        ]

        if "fully_static_link" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "fully_static_link",
                flag_sets = [_flag_set(
                    actions = [
                        "c++-link-dynamic-library",
                        "c++-link-executable",
                        "lto-index-for-dynamic-library",
                        "lto-index-for-executable",
                    ],
                    flag_groups = [_flag_group(
                        flags = ["-static"],
                    )],
                )],
            ))
        if "user_compile_flags" not in feature_names:
            fail("TODO: user_compile_flags")
        if "sysroot" not in feature_names:
            fail("TODO: sysroot")
        if "sysroot" not in feature_names:
            fail("TODO: sysroot")
        if "unfiltered_compile_flags" not in feature_names:
            fail("TODO: unfiltered_compile_flags")
        if "linker_param_file" not in feature_names:
            legacy_features_builder.append(_feature(
                name = "linker_param_file",
                flag_sets = [
                    _flag_set(
                        actions = [
                            "c++-link-dynamic-library",
                            "c++-link-executable",
                            "c++-link-nodeps-dynamic-library",
                            "lto-index-for-dynamic-library",
                            "lto-index-for-executable",
                            "lto-index-for-nodeps-dynamic-library",
                        ],
                        flag_groups = [_flag_group(
                            expand_if_available = "linker_param_file",
                            flags = ["@%{linker_param_file}"],
                        )],
                    ),
                    _flag_set(
                        actions = ["c++-link-static-library"],
                        flag_groups = [_flag_group(
                            expand_if_available = "linker_param_file",
                            flags = ["@%{linker_param_file}"],
                        )],
                    ),
                ],
            ))
        if "compiler_input_flags" not in feature_names:
            fail("TODO: compiler_input_flags")
        if "compiler_output_flags" not in feature_names:
            fail("TODO: compiler_output_flags")

        features = legacy_features_builder

        legacy_action_config_builder = []

        existing_action_config_names = set([action_config.action_name for action_config in action_configs])
        for action_name in [
            "assemble",
            "preprocess-assemble",
            "linkstamp-compile",
            "lto-backend",
            "c-compile",
            "c++-compile",
            "c++-header-parsing",
            "c++-module-compile",
            "c++-module-codegen",
        ]:
            if action_name not in existing_action_config_names:
                legacy_action_config_builder.append(_action_config(
                    action_name = action_name,
                    tools = [_tool(path = gcc_tool_path)],
                    implies = [
                        "legacy_compile_flags",
                        "user_compile_flags",
                        "sysroot",
                        "unfiltered_compile_flags",
                        "compiler_input_flags",
                        "compiler_output_flags",
                    ],
                ))
        for action_name in [
            "c++-link-executable",
            "lto-index-for-executable",
        ]:
            if action_name not in existing_action_config_names:
                legacy_action_config_builder.append(_action_config(
                    action_name = action_name,
                    tools = [_tool(path = gcc_tool_path)],
                    implies = [
                        "strip_debug_symbols",
                        "linkstamps",
                        "output_execpath_flags",
                        "runtime_library_search_directories",
                        "library_search_directories",
                        "libraries_to_link",
                        "force_pic_flags",
                        "user_link_flags",
                        "legacy_link_flags",
                        "linker_param_file",
                        "fission_support",
                        "sysroot",
                    ],
                ))
        for action_name in [
            "c++-link-nodeps-dynamic-library",
            "lto-index-for-nodeps-dynamic-library",
            "c++-link-dynamic-library",
            "lto-index-for-dynamic-library",
        ]:
            if action_name not in existing_action_config_names:
                legacy_action_config_builder.append(_action_config(
                    action_name = action_name,
                    tools = [_tool(path = gcc_tool_path)],
                    implies = [
                        "build_interface_libraries",
                        "dynamic_library_linker_tool",
                        "strip_debug_symbols",
                        "shared_flag",
                        "linkstamps",
                        "output_execpath_flags",
                        "runtime_library_search_directories",
                        "library_search_directories",
                        "libraries_to_link",
                        "user_link_flags",
                        "legacy_link_flags",
                        "linker_param_file",
                        "fission_support",
                        "sysroot",
                    ],
                ))
        if "c++-link-static-library" not in existing_action_config_names:
            legacy_action_config_builder.append(_action_config(
                action_name = "c++-link-static-library",
                tools = [_tool(path = ar_tool_path)],
                implies = ["archiver_flags", "linker_param_file"],
            ))
        if "strip" not in existing_action_config_names:
            legacy_action_config_builder.append(_action_config(
                action_name = "strip",
                tools = [_tool(path = strip_tool_path)],
                flag_sets = [_flag_set(
                    flag_groups = [
                        _flag_group(
                            flags = ["-S"] +
                                    (["-p"] if target_cpu != "macosx" else []) +
                                    ["-o", "%{output_file}"],
                        ),
                        _flag_group(
                            iterate_over = "stripopts",
                            flags = ["%{stripopts}"],
                        ),
                        _flag_group(
                            flags = ["%{input_file}"],
                        ),
                    ],
                )],
            ))

        legacy_action_config_builder += action_configs
        action_configs = legacy_action_config_builder

    tool_paths_tuples = [
        [tool.name, tool.path]
        for tool in tool_paths
    ]
    return CcToolchainConfigInfo(
        # Tools provided by the caller contain paths that are relative
        # to the current package, but CcToolchainConfigInfo needs to
        # report paths relative to the input root. Rewrite all action
        # configs and tools to fix up the paths.
        _action_configs = [
            _action_config(
                action_name = action_config.action_name,
                enabled = action_config.enabled,
                tools = [
                    _tool(
                        path = _path_relative_to_package(ctx, tool.path),
                        tool = tool.tool,
                        with_features = tool.with_features,
                        execution_requirements = tool.execution_requirements,
                    )
                    for tool in action_config.tools
                ],
                flag_sets = action_config.flag_sets,
                implies = action_config.implies,
            )
            for action_config in action_configs
        ],
        _artifact_name_patterns = {
            pattern.category_name: struct(prefix = pattern.prefix, extension = pattern.extension)
            for pattern in artifact_name_patterns
        },
        _features = features,
        abi_libc_version = lambda: abi_libc_version,
        abi_version = lambda: abi_version,
        builtin_sysroot = lambda: builtin_sysroot,
        compiler = lambda: compiler,
        cxx_builtin_include_directories = lambda: cxx_builtin_include_directories,
        make_variables = lambda: make_variables,
        target_cpu = lambda: target_cpu,
        target_libc = lambda: target_libc,
        target_system_name = lambda: target_system_name,
        tool_paths = lambda: tool_paths_tuples,
        toolchain_id = lambda: toolchain_identifier,
    )

def _create_compilation_context(
        *,
        additional_inputs,
        defines,
        framework_includes,
        headers,
        includes,
        module_map,
        quote_includes,
        system_includes,
        transitive_modules,
        virtual_to_original_headers):
    return struct(
        additional_inputs = lambda: additional_inputs,
        defines = defines,
        framework_includes = framework_includes,
        headers = headers,
        includes = includes,
        module_map = module_map,
        quote_includes = quote_includes,
        system_includes = system_includes,
        transitive_modules = transitive_modules,
        validation_artifacts = depset(),
        virtual_to_original_headers = lambda: virtual_to_original_headers,
    )

def builtins_internal_cc_common_create_compilation_context(
        *,
        actions = None,
        add_public_headers_to_modular_headers = None,
        defines = None,
        dependent_cc_compilation_contexts = [],
        direct_private_headers = [],
        direct_public_headers = [],
        direct_textual_headers = [],
        exported_dependent_cc_compilation_contexts = [],
        external_includes = None,
        framework_includes = None,
        header_module = None,
        headers = None,
        headers_checking_mode = None,
        includes = None,
        label = None,
        local_defines = None,
        loose_hdrs_dirs = [],
        module_map = None,
        non_code_inputs = [],
        pic_header_module = None,
        propagate_module_map_to_compile_action = False,
        purpose = None,
        quote_includes = None,
        separate_module = None,
        separate_module_headers = [],
        separate_pic_module = None,
        system_includes = None,
        virtual_to_original_headers = None):
    # TODO: We should also add direct_module_maps.
    additional_inputs_transitive = []
    if non_code_inputs:
        additional_inputs_transitive.append(non_code_inputs)
    additional_inputs = depset(transitive = additional_inputs_transitive)

    modules_direct = []
    if header_module:
        modules_direct.append(header_module)
    if separate_module:
        modules_direct.append(separate_module)
    modules = depset(modules_direct)

    pic_modules_direct = []
    if pic_header_module:
        pic_modules_direct.append(pic_header_module)
    if separate_pic_module:
        pic_modules_direct.append(separate_pic_module)
    pic_modules = depset(pic_modules_direct)

    return _create_compilation_context(
        additional_inputs = additional_inputs,
        defines = defines or depset(),
        framework_includes = framework_includes or depset(),
        headers = headers or depset(),
        includes = includes or depset(),
        module_map = module_map,
        quote_includes = quote_includes or depset(),
        system_includes = system_includes or depset(),
        transitive_modules = lambda use_pic: pic_modules if use_pic else modules,
        virtual_to_original_headers = virtual_to_original_headers or depset(),
    )

def _create_lto_compilation_context():
    return struct(
        lto_bitcode_inputs = lambda: {},
    )

def builtins_internal_cc_common_create_compilation_outputs(
        *,
        dwo_objects,
        pic_dwo_objects,
        lto_compilation_context = None,
        objects = None,
        pic_objects = None):
    return _create_compilation_outputs(
        header_tokens = depset(),
        lto_compilation_context = lto_compilation_context or _create_lto_compilation_context(),
        objects = objects or depset(),
        pic_objects = pic_objects or depset(),
    )

def builtins_internal_cc_common_create_compile_variables(
        cc_toolchain,
        feature_configuration,
        source_file = None,
        output_file = None,
        user_compile_flags = None,
        include_directories = None,
        quote_include_directories = None,
        system_include_directories = None,
        framework_include_directories = None,
        preprocessor_defines = None,
        thinlto_index = None,
        thinlto_input_bitcode_file = None,
        thinlto_output_object_file = None,
        use_pic = False,
        add_legacy_cxx_options = False,
        variables_extension = None,
        strip_opts = None,
        input_file = None):
    return struct(_todo_is_compile_variables = True)

def _create_debug_context(dwo_files, pic_dwo_files):
    return struct(
        dwo_files = dwo_files,
        pic_dwo_files = pic_dwo_files,
        # TODO: How do we set these?
        files = depset(),
        pic_files = depset(),
    )

def builtins_internal_cc_common_create_debug_context(compilation_outputs = None):
    return _create_debug_context(
        dwo_files = compilation_outputs._dwo_files if compilation_outputs else depset(),
        pic_dwo_files = compilation_outputs._pic_dwo_files if compilation_outputs else depset(),
    )

def builtins_internal_cc_common_create_linker_input(
        *,
        owner,
        additional_inputs = None,
        libraries = None,
        linkstamps = None,
        user_link_flags = None):
    return struct(
        additional_inputs = tuple(additional_inputs.to_list()) if additional_inputs else (),
        libraries = tuple(libraries.to_list()) if libraries else (),
        linkstamps = tuple(linkstamps.to_list()) if linkstamps else (),
        user_link_flags = tuple(user_link_flags) if user_link_flags else (),
    )

def builtins_internal_cc_common_create_linking_context(
        *,
        additional_inputs = None,
        extra_link_time_library = None,
        libraries_to_link = None,
        linker_inputs = None,
        owner = None,
        user_link_flags = None):
    return struct(
        extra_link_time_libraries = lambda: struct(
            # TODO: Return proper depsets.
            build_libraries = lambda ctx, static_mode, for_dynamic_library: (depset(), depset()),
        ),
        linker_inputs = linker_inputs or depset(),
    )

def builtins_internal_cc_common_create_module_map(
        *,
        file,
        name,
        umbrella_header):
    return struct(
        file = lambda: file,
        umbrella_header = lambda: umbrella_header,
    )

def builtins_internal_cc_common_get_environment_variables(
        feature_configuration,
        action_name,
        variables):
    return {}

def builtins_internal_cc_common_get_execution_requirements(
        *,
        action_name,
        feature_configuration):
    return []

def builtins_internal_cc_common_get_memory_inefficient_command_line(
        feature_configuration,
        action_name,
        variables):
    return ["TODO", "get_memory_inefficient_command_line"]

def _tool_get_tool_path_string(tool):
    fail(tool)

def _tool_is_with_features_satisfied(with_feature_sets, enabled_feature_names):
    fail(with_feature_sets)

def _action_config_get_tool(action_config, enabled_feature_names):
    for tool in action_config.tools:
        if _tool_is_with_features_satisfied(tool.with_features, enabled_feature_names):
            return tool
    fail("Matching tool for action %s not found for given feature configuration" % action_config.action_name)

def builtins_internal_cc_common_get_tool_for_action(feature_configuration, action_name):
    action_config = feature_configuration._action_config_by_action_name[action_name]
    return _tool_get_tool_path_string(
        _action_config_get_tool(action_config, feature_configuration._enabled_feature_names),
    )

def builtins_internal_cc_common_get_tool_requirement_for_action(*, action_name, feature_configuration):
    return []

def builtins_internal_cc_common_merge_compilation_contexts(compilation_contexts = [], non_exported_compilation_contexts = []):
    additional_inputs = depset(transitive = [cc.additional_inputs() for cc in compilation_contexts])
    defines = depset(transitive = [cc.defines for cc in compilation_contexts])
    framework_includes = depset(transitive = [cc.framework_includes for cc in compilation_contexts])
    headers = depset(transitive = [cc.headers for cc in compilation_contexts])
    includes = depset(transitive = [cc.includes for cc in compilation_contexts])
    modules = depset(transitive = [cc.transitive_modules(use_pic = False) for cc in compilation_contexts])
    pic_modules = depset(transitive = [cc.transitive_modules(use_pic = True) for cc in compilation_contexts])
    quote_includes = depset(transitive = [cc.quote_includes for cc in compilation_contexts])
    system_includes = depset(transitive = [cc.system_includes for cc in compilation_contexts])
    virtual_to_original_headers = depset(transitive = [cc.virtual_to_original_headers() for cc in compilation_contexts])

    return _create_compilation_context(
        additional_inputs = additional_inputs,
        defines = defines,
        framework_includes = framework_includes,
        headers = headers,
        includes = includes,
        module_map = None,
        quote_includes = quote_includes,
        system_includes = system_includes,
        transitive_modules = lambda use_pic: pic_modules if use_pic else modules,
        virtual_to_original_headers = virtual_to_original_headers,
    )

def builtins_internal_cc_common_merge_compilation_outputs(*, compilation_outputs = []):
    return _create_compilation_outputs(
        header_tokens = depset(
            transitive = [
                co._header_tokens
                for co in compilation_outputs
            ],
        ),
        objects = depset(
            transitive = [
                co._objects
                for co in compilation_outputs
            ],
        ),
        pic_objects = depset(
            transitive = [
                co._pic_objects
                for co in compilation_outputs
            ],
        ),
        lto_compilation_context = _create_lto_compilation_context(),
    )

def builtins_internal_cc_common_merge_debug_context(debug_contexts = []):
    return _create_debug_context(
        dwo_files = depset(
            transitive = [
                dc.dwo_files
                for dc in debug_contexts
            ],
        ),
        pic_dwo_files = depset(
            transitive = [
                dc.pic_dwo_files
                for dc in debug_contexts
            ],
        ),
    )

def builtins_internal_cc_common_merge_linking_contexts(linking_contexts = []):
    return builtins_internal_cc_common_create_linking_context(
        linker_inputs = depset(
            transitive = [
                lc.linker_inputs
                for lc in linking_contexts
            ],
        ),
    )

def builtins_internal_cc_common_validate_starlark_compile_api_call(
        *,
        actions,
        include_prefix,
        strip_include_prefix,
        additional_include_scanning_roots):
    pass

def builtins_internal_cc_internal_actions2ctx_cheat(actions):
    return native.current_ctx()

def builtins_internal_cc_internal_cc_toolchain_features(*, toolchain_config_info, tools_directory):
    selectables = []
    selectables_by_name = {}
    action_configs_by_action_name = {}
    default_selectables = []
    for feature in toolchain_config_info._features:
        selectables.append(feature)
        selectables_by_name[feature.name] = feature
        if feature.enabled:
            default_selectables.append(feature.name)

    for action_config in toolchain_config_info._action_configs:
        selectables.append(action_config)
        selectables_by_name[action_config.action_name] = action_config
        action_configs_by_action_name[action_config.action_name] = action_config
        if action_config.enabled:
            default_selectables.append(action_config.action_name)

    implies = {}
    requires = {}
    provides = {}
    implied_by = {}
    required_by = {}

    for feature in toolchain_config_info._features:
        name = feature.name
        for required_features in feature.requires:
            all_of = set()
            for required_name in required_features.features:
                all_of.add(required_name)
                required_by.setdefault(required_name, set()).add(name)
            requires.setdefault(name, set()).union(all_of)
        for implied_name in feature.implies:
            implied_by.setdefault(implied_name, set()).add(name)
            implies.setdefault(name, set()).add(implied_name)
        for provides_name in feature.provides:
            provides.setdefault(provides_name, set()).add(name)

    for action_config in toolchain_config_info._action_configs:
        name = action_config.action_name
        for implied_name in action_config.implies:
            implied_by.setdefault(implied_name, set()).add(name)
            implies.setdefault(name, set()).add(implied_name)

    return struct(
        _action_configs_by_action_name = action_configs_by_action_name,
        _artifact_name_patterns = toolchain_config_info._artifact_name_patterns,
        _cc_toolchain_path = tools_directory,
        _default_selectables = default_selectables,
        _implied_by = implied_by,
        _implies = implies,
        _provides = provides,
        _required_by = required_by,
        _requires = requires,
        _selectables = selectables,
        _selectables_by_name = selectables_by_name,
    )

def builtins_internal_cc_internal_cc_toolchain_variables(vars):
    return "TODO"

def builtins_internal_cc_internal_collect_libraries_to_link(
        libraries_to_link,
        cc_toolchain,
        feature_configuration,
        output,
        dynamic_library_solib_symlink_output,
        link_type,
        linking_mode,
        is_native_deps,
        solib_dir,
        toolchain_libraries_solib_dir,
        workspace_name):
    return struct(
        all_runtime_library_search_directories = depset(),
        library_search_directories = depset(),
    )

def builtins_internal_cc_internal_convert_library_to_link_list_to_linker_input_list(libraries_to_link, static_mode, for_dynamic_library, support_dynamic_linker):
    library_inputs = []
    for library_to_link in libraries_to_link.to_list():
        fail("TODO: implement!")
    return library_inputs

def builtins_internal_cc_internal_create_cc_launcher_info(*, cc_info, compilation_outputs):
    return CcLauncherInfo(
        cc_info = lambda: cc_info,
        compilation_outputs = lambda: compilation_outputs,
    )

def library_to_link_disable_whole_archive(lib):
    return lambda: lib._disable_whole_archive

def library_to_link_must_keep_debug(lib):
    return lambda: lib._must_keep_debug

def library_to_link_pic_objects_private(lib):
    return lambda: lib._pic_object_files.to_list()

LibraryToLink = provider(
    computed_fields = {
        "disable_whole_archive": library_to_link_disable_whole_archive,
        "must_keep_debug": library_to_link_must_keep_debug,
        "pic_objects_private": library_to_link_pic_objects_private,
    },
)

def builtins_internal_cc_internal_create_library_to_link(library_to_link):
    return LibraryToLink(
        _disable_whole_archive = getattr(library_to_link, "disable_whole_archive", False),
        _must_keep_debug = getattr(library_to_link, "must_keep_debug", False),
        _pic_object_files = depset(getattr(library_to_link, "pic_object_files", [])),
        alwayslink = getattr(library_to_link, "dynamic_library", False),
        dynamic_library = getattr(library_to_link, "dynamic_library", None),
        interface_library = getattr(library_to_link, "interface_library", None),
        pic_static_library = getattr(library_to_link, "pic_static_library", None),
        resolved_symlink_dynamic_library = getattr(library_to_link, "resolved_symlink_dynamic_library", None),
        # Notice "resolve_" instead of "resolved_".
        resolved_symlink_interface_library = getattr(library_to_link, "resolve_symlink_interface_library", None),
        static_library = getattr(library_to_link, "static_library", None),
    )

def builtins_internal_cc_internal_create_shared_non_lto_artifacts(
        actions,
        lto_compilation_context,
        is_linker,
        feature_configuration,
        cc_toolchain,
        use_pic,
        object_files):
    shared_non_lto_backends = {}
    lto_bitcode_inputs = lto_compilation_context.lto_bitcode_inputs()
    for input_artifact in object_files:
        if input_artifact in lto_bitcode_inputs:
            fail("TODO")
    return shared_non_lto_backends

def builtins_internal_cc_internal_dynamic_library_soname(actions, path, preserve_name):
    if preserve_name:
        return path.rsplit("/", 1)[-1]
    fail("TODO: implement!")

def builtins_internal_cc_internal_empty_compilation_outputs():
    return _create_compilation_outputs(
        header_tokens = depset(),
        objects = depset(),
        pic_objects = depset(),
        lto_compilation_context = _create_lto_compilation_context(),
    )

def builtins_internal_cc_internal_escape_label(label):
    return "".join([
        "_U" if c == "_" else "_S" if c == "/" else "_B" if c == "\\" else "_C" if c == ":" else "_A" if c == "@" else c
        for c in (label.repo_name + "@" + label.package + ":" + label.name).elems()
    ])

def builtins_internal_cc_internal_for_object_file(name, is_whole_archive):
    return struct(__todo_is_for_object_file = True)

def builtins_internal_cc_internal_for_static_library(name, is_whole_archive):
    return struct(__todo_is_for_static_library = True)

# Artifact name patterns that are registered by default.
# Obtained from ArtifactCategory.java.
default_artifact_name_patterns = {
    "STATIC_LIBRARY": struct(prefix = "lib", extension = ".a"),
    "ALWAYSLINK_STATIC_LIBRARY": struct(prefix = "lib", extension = ".lo"),
    "DYNAMIC_LIBRARY": struct(prefix = "lib", extension = ".so"),
    "EXECUTABLE": struct(prefix = "", extension = ""),
    "INTERFACE_LIBRARY": struct(prefix = "lib", extension = ".ifso"),
    "PIC_FILE": struct(prefix = "", extension = ".pic"),
    "INCLUDED_FILE_LIST": struct(prefix = "", extension = ".d"),
    "SERIALIZED_DIAGNOSTICS_FILE": struct(prefix = "", extension = ".dia"),
    "OBJECT_FILE": struct(prefix = "", extension = ".o"),
    "PIC_OBJECT_FILE": struct(prefix = "", extension = ".pic.o"),
    "CPP_MODULE": struct(prefix = "", extension = ".pcm"),
    "CPP_MODULE_GCM": struct(prefix = "", extension = ".gcm"),
    "CPP_MODULE_IFC": struct(prefix = "", extension = ".ifc"),
    "CPP_MODULES_INFO": struct(prefix = "", extension = ".CXXModules.json"),
    "CPP_MODULES_DDI": struct(prefix = "", extension = ".ddi"),
    "CPP_MODULES_MODMAP": struct(prefix = "", extension = ".modmap"),
    "CPP_MODULES_MODMAP_INPUT": struct(prefix = "", extension = ".modmap.input"),
    "GENERATED_ASSEMBLY": struct(prefix = "", extension = ".s"),
    "PROCESSED_HEADER": struct(prefix = "", extension = ".processed"),
    "GENERATED_HEADER": struct(prefix = "", extension = ".h"),
    "PREPROCESSED_C_SOURCE": struct(prefix = "", extension = ".i"),
    "PREPROCESSED_CPP_SOURCE": struct(prefix = "", extension = ".ii"),
    "COVERAGE_DATA_FILE": struct(prefix = "", extension = ".gcno"),
    "CLIF_OUTPUT_PROTO": struct(prefix = "", extension = ".opb"),
}

def builtins_internal_cc_internal_get_artifact_name_for_category(cc_toolchain, category, output_name):
    pattern = cc_toolchain._toolchain_features._artifact_name_patterns.get(category)
    if not pattern:
        pattern = default_artifact_name_patterns[category]

    output_parts = output_name.split("/")
    output_parts[-1] = pattern.prefix + output_parts[-1] + pattern.extension
    return "/".join(output_parts)

def builtins_internal_cc_internal_get_link_args(
        *,
        action_name,
        build_variables,
        feature_configuration,
        parameter_file_type):
    return native.current_ctx().actions.args()

def builtins_internal_cc_internal_licenses(ctx):
    return None

def builtins_internal_cc_internal_wrap_link_actions(actions, build_config = None, use_shareable_artifact_factory = False):
    return actions

def builtins_internal_java_common_internal_do_not_use__check_java_toolchain_is_declared_on_rule():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use__google_legacy_api_enabled():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use__incompatible_java_info_merge_runtime_module_flags():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_check_provider_instances(providers, what, provider_type):
    # TODO.
    pass

def builtins_internal_java_common_internal_do_not_use_collect_native_deps_dirs():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_create_compilation_action():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_create_header_compilation_action():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_expand_java_opts():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_get_runtime_classpath_for_archive():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_incompatible_disable_non_executable_java_binary():
    return False

def builtins_internal_java_common_internal_do_not_use_target_kind():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_tokenize_javacopts():
    return "TODO"

def builtins_internal_java_common_internal_do_not_use_wrap_java_info():
    return "TODO"

def builtins_internal_py_builtins_are_action_listeners_enabled(ctx):
    return False

def builtins_internal_py_builtins_create_repo_mapping_manifest(ctx, runfiles, output):
    ctx.actions.write(output, "TODO")

def builtins_internal_py_builtins_get_current_os_name():
    return "unknown"

def builtins_internal_py_builtins_get_label_repo_runfiles_path(label):
    return "/".join(["..", label.repo_name] + label.package.split("/"))

def builtins_internal_py_builtins_get_legacy_external_runfiles(ctx):
    return False

def builtins_internal_py_builtins_is_bzlmod_enabled(ctx):
    return True

def builtins_internal_py_builtins_make_runfiles_respect_legacy_external_runfiles(ctx, runfiles):
    return runfiles

def builtins_internal_py_builtins_merge_runfiles_with_generated_inits_empty_files_supplier(ctx, runfiles):
    return runfiles

def builtins_json_encode_indent(x, **kwargs):
    return json.indent(json.encode(x), **kwargs)

exported_rules = {
    "alias": native.alias,
    "cc_libc_top_alias": cc_libc_top_alias,
    "cc_proto_library": cc_proto_library,
    "cc_toolchain_suite": cc_toolchain_suite,
    "config_setting": config_setting,
    "constraint_setting": constraint_setting,
    "constraint_value": constraint_value,
    "exports_files": native.exports_files,
    "filegroup": filegroup,
    "genrule": genrule,
    "glob": native.glob,
    "java_plugins_flag_alias": java_plugins_flag_alias,
    "java_proto_library": java_proto_library,
    "label_flag": native.label_flag,
    "label_setting": native.label_setting,
    "licenses": licenses,
    "package_group": native.package_group,
    "platform": platform,
    "sh_test": sh_test,
    "test_suite": test_suite,
    "toolchain": toolchain,
    "toolchain_type": toolchain_type,
}
exported_toplevels = {
    "AnalysisFailureInfo": AnalysisFailureInfo,
    "AnalysisTestResultInfo": AnalysisTestResultInfo,
    "DefaultInfo": DefaultInfo,
    "OutputGroupInfo": OutputGroupInfo,
    "RunEnvironmentInfo": RunEnvironmentInfo,
    "CcInfo": CcInfo,
    "CcToolchainConfigInfo": CcToolchainConfigInfo,
    "DebugPackageInfo": DebugPackageInfo,
    "InstrumentedFilesInfo": InstrumentedFilesInfo,
    "PackageSpecificationInfo": PackageSpecificationInfo,
    "ProguardSpecProvider": ProguardSpecProvider,
    "PyInfo": PyInfo,
    "config_common": struct(
        FeatureFlagInfo = FeatureFlagInfo,
        toolchain_type = config_common.toolchain_type,
    ),
    "configuration_field": configuration_field,
    "coverage_common": struct(
        instrumented_files_info = coverage_common_instrumented_files_info,
    ),
    "exec_transition": transition,
    "json": struct(
        decode = json.decode,
        encode = json.encode,
        # starlark-go does not support json.encode_indent().
        # TODO: Should we get it added?
        encode_indent = builtins_json_encode_indent,
        indent = json.indent,
    ),
    "platform_common": struct(
        ConstraintValueInfo = ConstraintValueInfo,
        TemplateVariableInfo = TemplateVariableInfo,
        ToolchainInfo = ToolchainInfo,
    ),
    "proto_common_do_not_use": struct(
        external_proto_infos = proto_common_do_not_use_external_proto_infos,
        incompatible_enable_proto_toolchain_resolution = proto_common_do_not_use_incompatible_enable_proto_toolchain_resolution,
    ),
    "testing": struct(
        ExecutionInfo = ExecutionInfo,
    ),
}

exported_toplevels["_builtins"] = struct(
    internal = struct(
        CcNativeLibraryInfo = CcNativeLibraryInfo,
        StaticallyLinkedMarkerProvider = StaticallyLinkedMarkerProvider,
        apple_common = struct(
            dotted_version = builtins_internal_apple_common_dotted_version,
        ),
        cc_common = struct(
            CcToolchainInfo = CcToolchainInfo,
            action_is_enabled = builtins_internal_cc_common_action_is_enabled,
            check_private_api = builtins_internal_cc_common_check_private_api,
            compile_fork = builtins_internal_cc_common_compile_fork,
            configure_features = builtins_internal_cc_common_configure_features,
            create_cc_toolchain_config_info = builtins_internal_cc_common_create_cc_toolchain_config_info,
            create_compilation_context = builtins_internal_cc_common_create_compilation_context,
            create_compilation_outputs = builtins_internal_cc_common_create_compilation_outputs,
            create_compile_variables = builtins_internal_cc_common_create_compile_variables,
            create_debug_context = builtins_internal_cc_common_create_debug_context,
            create_linker_input = builtins_internal_cc_common_create_linker_input,
            create_linking_context = builtins_internal_cc_common_create_linking_context,
            create_module_map = builtins_internal_cc_common_create_module_map,
            do_not_use_tools_cpp_compiler_present = None,
            get_environment_variables = builtins_internal_cc_common_get_environment_variables,
            get_execution_requirements = builtins_internal_cc_common_get_execution_requirements,
            get_memory_inefficient_command_line = builtins_internal_cc_common_get_memory_inefficient_command_line,
            get_tool_for_action = builtins_internal_cc_common_get_tool_for_action,
            get_tool_requirement_for_action = builtins_internal_cc_common_get_tool_requirement_for_action,
            merge_compilation_contexts = builtins_internal_cc_common_merge_compilation_contexts,
            merge_compilation_outputs = builtins_internal_cc_common_merge_compilation_outputs,
            merge_debug_context = builtins_internal_cc_common_merge_debug_context,
            merge_linking_contexts = builtins_internal_cc_common_merge_linking_contexts,
            validate_starlark_compile_api_call = builtins_internal_cc_common_validate_starlark_compile_api_call,
        ),
        cc_internal = struct(
            actions2ctx_cheat = builtins_internal_cc_internal_actions2ctx_cheat,
            cc_toolchain_features = builtins_internal_cc_internal_cc_toolchain_features,
            cc_toolchain_variables = builtins_internal_cc_internal_cc_toolchain_variables,
            collect_libraries_to_link = builtins_internal_cc_internal_collect_libraries_to_link,
            convert_library_to_link_list_to_linker_input_list = builtins_internal_cc_internal_convert_library_to_link_list_to_linker_input_list,
            create_cc_launcher_info = builtins_internal_cc_internal_create_cc_launcher_info,
            create_library_to_link = builtins_internal_cc_internal_create_library_to_link,
            create_shared_non_lto_artifacts = builtins_internal_cc_internal_create_shared_non_lto_artifacts,
            dynamic_library_soname = builtins_internal_cc_internal_dynamic_library_soname,
            empty_compilation_outputs = builtins_internal_cc_internal_empty_compilation_outputs,
            escape_label = builtins_internal_cc_internal_escape_label,
            for_object_file = builtins_internal_cc_internal_for_object_file,
            for_static_library = builtins_internal_cc_internal_for_static_library,
            get_artifact_name_for_category = builtins_internal_cc_internal_get_artifact_name_for_category,
            get_link_args = builtins_internal_cc_internal_get_link_args,
            launcher_provider = CcLauncherInfo,
            licenses = builtins_internal_cc_internal_licenses,
            wrap_link_actions = builtins_internal_cc_internal_wrap_link_actions,
        ),
        java_common_internal_do_not_use = struct(
            _check_java_toolchain_is_declared_on_rule = builtins_internal_java_common_internal_do_not_use__check_java_toolchain_is_declared_on_rule,
            _google_legacy_api_enabled = builtins_internal_java_common_internal_do_not_use__google_legacy_api_enabled,
            _incompatible_java_info_merge_runtime_module_flags = builtins_internal_java_common_internal_do_not_use__incompatible_java_info_merge_runtime_module_flags,
            check_provider_instances = builtins_internal_java_common_internal_do_not_use_check_provider_instances,
            collect_native_deps_dirs = builtins_internal_java_common_internal_do_not_use_collect_native_deps_dirs,
            create_compilation_action = builtins_internal_java_common_internal_do_not_use_create_compilation_action,
            create_header_compilation_action = builtins_internal_java_common_internal_do_not_use_create_header_compilation_action,
            expand_java_opts = builtins_internal_java_common_internal_do_not_use_expand_java_opts,
            get_runtime_classpath_for_archive = builtins_internal_java_common_internal_do_not_use_get_runtime_classpath_for_archive,
            incompatible_disable_non_executable_java_binary = builtins_internal_java_common_internal_do_not_use_incompatible_disable_non_executable_java_binary,
            target_kind = builtins_internal_java_common_internal_do_not_use_target_kind,
            tokenize_javacopts = builtins_internal_java_common_internal_do_not_use_tokenize_javacopts,
            wrap_java_info = builtins_internal_java_common_internal_do_not_use_wrap_java_info,
        ),
        objc_internal = struct(),
        py_builtins = struct(
            are_action_listeners_enabled = builtins_internal_py_builtins_are_action_listeners_enabled,
            create_repo_mapping_manifest = builtins_internal_py_builtins_create_repo_mapping_manifest,
            get_current_os_name = builtins_internal_py_builtins_get_current_os_name,
            get_label_repo_runfiles_path = builtins_internal_py_builtins_get_label_repo_runfiles_path,
            get_legacy_external_runfiles = builtins_internal_py_builtins_get_legacy_external_runfiles,
            is_bzlmod_enabled = builtins_internal_py_builtins_is_bzlmod_enabled,
            make_runfiles_respect_legacy_external_runfiles = builtins_internal_py_builtins_make_runfiles_respect_legacy_external_runfiles,
            merge_runfiles_with_generated_inits_empty_files_supplier = builtins_internal_py_builtins_merge_runfiles_with_generated_inits_empty_files_supplier,
        ),
    ),
    toplevel = struct(**exported_toplevels),
)
