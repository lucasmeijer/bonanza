# Building bb-storage using Bonanza

Even though Bonanza should eventually become usable to build arbitrary
Bazel projects, we're currently mostly just aiming to get bb-storage
built. This document explains how to do this yourself, which is useful
for getting an idea of how Bonanza works.

## Step 1: Getting a patched version of bb-storage

A version of bb-storage that contains a number of local changes to
support building with Bonanza can be obtained as follows:

```sh
git clone git@github.com:buildbarn/bb-storage.git
cd bb-storage
git checkout eschouten/bonanza
```

Be sure to run a `git diff master...` to get an idea of the kinds of
changes that had to be made to support Bonanza.

## Step 2: Extending your `~/.bazelrc`

Let's extend our `~/.bazelrc`, so that we can use `--config=bonanza` to
perform builds against a locally running Bonanza cluster. Below is a
description of the flags to add, and an explanation of what they do.
Make sure to adjust any paths as needed (or change your username to
`ed`).

```
# Like Bazel, Bonanza has --remote_cache and --remote_executor flags.
# However, unlike Bazel they can't point to REv2 servers. Bonanza uses
# its own storage and execution protocols.
common:bonanza --remote_cache=unix:///Users/ed/bonanza_demo/bonanza_storage_frontend.sock
common:bonanza --remote_executor=unix:///Users/ed/bonanza_demo/bonanza_scheduler_clients.sock

# Bonanza supports encrypting CAS objects using AES. This key can be
# generated client side and is passed along to the cluster when a build
# is kicked off. If you change this key, you are effectively busting the
# entire cache. You can set this key to any 16-byte, 24-byte or 32-byte
# value you want: head -c ${size} /dev/urandom | base64
common:bonanza --remote_encryption_key=U3YDUwfejfiRDeD4aqoR7A==

# Analysis and orchestration of the build takes place on a process named
# bonanza_builder. Even though it does not run any build actions, it is
# modeled as a special kind of worker. This means that to kick off
# builds, we go through the bonanza_scheduler.
#
# Whereas REv2 identifies different types of workers by key-value pairs
# called platform properties, Bonanza requires that each type of worker
# has an X25519 private key. Clients can use the corresponding X25519
# public key to route requests. The key below is the public key
# corresponding to the private key used by the demo deployment.
#
# In production you should obviously generate your own key pair. This
# can be done by running the following commands:
#
#     openssl genpkey -algorithm x25519 -out private.pem
#     openssl pkey -in private.pem -pubout -out public.pem
common:bonanza --remote_executor_builder_pkix_public_key=MCowBQYDK2VuAyEAE+onXE9lGj+1ykKMdYJ7ORbbGvDg6mXwX9H90afmdDI=

# bonanza_builder only accepts build requests that are accompanied with
# a trusted X.509 certificate. We also need to provide the X25519
# private key that corresponds to the public key in the certificate, as
# that is used to encrypt the build request, so that only
# bonanza_builder (and not bonanza_scheduler itself) can read it.
#
# Generating such certificates yourself is still a bit hard, as OpenSSL
# is currently only able to generate certificates for key types that
# support signing. X25519 only supports elliptic-curve Diffie-Hellman
# (ECDH), not signing. Take a look at Bonanza's
# //cmd/create_x25519_client_certificate to see how you can use Go's
# "crypto/x509" package to craft your own certificates.
common:bonanza --remote_executor_client_private_key=/Users/ed/projects/bonanza/deployments/demo/bonanza_bazel.key.pem
common:bonanza --remote_executor_client_certificate_chain=/Users/ed/projects/bonanza/deployments/demo/bonanza_bazel.cert.pem

# Modules containing Starlark code that need to be uploaded in addition
# to the code belonging to the root module.
common:bonanza --override_module=bazel_tools=/Users/ed/projects/bonanza/starlark/bazel_tools
common:bonanza --override_module=builtins_bzl=/Users/ed/projects/bonanza/starlark/builtins_bzl
common:bonanza --override_module=builtins_core=/Users/ed/projects/bonanza/starlark/builtins_core

# Make DefaultInfo, filegroup(), platform(), etc. work without requiring
# any explicit load() directives. Bazel implements these as native types
# written in Java. In Bonanza even these are written in plain Starlark.
common:bonanza --builtins_module=builtins_core

# Make cc_binary(), etc. work without requiring any explicit load()
# directives.
common:bonanza --builtins_module=builtins_bzl

# bonanza_builder does not launch any subprocesses, meaning that it is
# incapable of running any repository rules itself. It uses remote
# execution to run these, meaning we need to provide a platform() for
# that. There is no requirement that this platform is compatible with
# the one running bonanza_bazel or bonanza_builder.
#
# This also ends up controlling what @platforms//host looks like. This
# means that if bonanza_bazel is run on Windows, bonanza_builder is run
# on Linux, and the --repo_platform points to workers running macOS, the
# build is expected to behave as if it is run on macOS.
common:bonanza --repo_platform=//platforms:repo
```

Noteworthy is the `--repo_platform` flag pointing to `//platforms:repo`.
Notice how bb-storage's `platforms/BUILD.bazel` contains some
`platform()` targets that have some attributes that are specific to
Bonanza.

- The `exec_pkix_public_key` attribute is a replacement for Bazel's
  `exec_properties` and `remote_execution_properties` attributes. It
  contains the X25519 public key that identifies the type of worker.

- The `repository_os_*` attributes contain some additional information
  about the platform that's needed to make it usable for running
  repository rules.

# Step 3: Build and install bonanza\_bazel

In order to launch builds, we need to install Bonanza's command line
utility. This can be done as follows:

```sh
install -m 555 \
    $(bazel run --run_under echo //cmd/bonanza_bazel) \
    /usr/local/bin/bonanza_bazel
```

# Step 4: Launch the demo cluster

A demo cluster can be launched on your local system by running the
following command:

```sh
bazel run //deployments/demo
```

This ends up launching a bunch of processes:

```
$ ps u | grep -e '\<bonanza_' -e '\<bb_' | sort -k 11
ed   38907   0.0  0.0 416177744  30368 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_builder/bonanza_builder_/bonanza_builder ...
ed   38914   0.0  0.0 411957840  24704 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_scheduler/bonanza_scheduler_/bonanza_scheduler ...
ed   38904   0.0  0.0 411949360  25056 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_storage_frontend/bonanza_storage_frontend_/bonanza_storage_frontend ...
ed   38869   0.0  0.0 411890496  21040 s002  S+    2:09PM   0:00.02 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38870   0.0  0.0 411943392  23280 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38872   0.0  0.0 411899968  21552 s002  S+    2:09PM   0:00.02 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38874   0.0  0.0 411916352  22624 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38877   0.0  0.0 411909184  21920 s002  S+    2:09PM   0:00.02 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38881   0.0  0.0 411914624  22464 s002  S+    2:09PM   0:00.02 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38890   0.0  0.0 411916368  22736 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38898   0.0  0.0 411917136  22832 s002  S+    2:09PM   0:00.03 /.../cmd/bonanza_storage_shard/bonanza_storage_shard_/bonanza_storage_shard ...
ed   38922   0.0  0.1 412952448  35936 s002  S+    2:09PM   0:00.05 /.../cmd/bonanza_worker/bonanza_worker_/bonanza_worker ...
ed   38936   0.0  0.0 411851360  23920 s002  S+    2:09PM   0:00.03 /.../external/com_github_buildbarn_bb_remote_execution+/cmd/bb_runner/bb_runner_/bb_runner ...
```

Notice how bonanza\_worker still has the same architecture as Buildbarn,
where it uses a separate runner process to actually launch build
actions. In fact, it is still the same protocol for this, meaning that
the demo deployment also runs a stock copy of bb\_runner coming from the
bb-remote-execution repository.

# Step 5: Run the actual build

Now that we've spun up a cluster, go to the bb-storage checkout we
created previously. In there, you can run a command like the following
to build it:

```sh
bonanza_bazel build --config=bonanza //cmd/bb_storage
```

Enjoy!
