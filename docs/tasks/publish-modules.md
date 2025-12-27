# Publish Modules

Example HCL configuration using a module referenced from the registry:

```hcl
module "tls-private-key" {
  source = "boring-registry.example.com/acme/tls-private-key/aws"
  version = "~> 0.1"
}
```

## Uploading modules using the CLI

Modules can be published to the registry with the `upload module` command.
The command:

1. Discovers `boring-registry.hcl` files by walking the directory recursively (by default)
2. Packages each module into an archive
3. Uploads the archive to the configured storage backend

The `boring-registry.hcl` file should be placed in the root directory of the module and must contain a `metadata` block with the following fields:

```hcl
metadata {
  namespace = "acme"           # required: namespace for the module
  name      = "tls-private-key" # required: module name
  provider  = "aws"             # required: provider name
  version   = "0.1.0"           # required by default, must be omitted when using --version flag
}
```

## Module versioning

Modules use version numbers [following the Semantic Versioning 2.0 conventions](https://opentofu.org/docs/internals/module-registry-protocol/#module-versions).
The version of a module to be published can be specified in two ways:

1. In the `boring-registry.hcl` file - allows for recursive discovery of multiple (nested) modules from a root directory
2. Via the `--version` CLI flag - useful for dynamic versioning, for example in CI/CD pipelines.
  Allows to set the version depending on git tags, build metadata, or semantic release tools.

When using the `--version` flag, the `version` field in the `boring-registry.hcl` must not be set:

```hcl
metadata {
  namespace = "acme"
  name      = "tls-private-key"
  provider  = "aws"
  # version is omitted when using --version flag
}
```

The `--version` flag is mutually exclusive with recursive module discovery (`--recursive`)!

An example usage to upload a module with `--versioning` is `boring-registry upload module --version "0.1.0" ./path/to/tls-private-key`.

## Recursive vs. non-recursive upload

By default, the `upload module` command walks the directory tree recursively, searching for all `boring-registry.hcl` files and uploading each discovered module.
This is convenient for uploading multiple modules at once.

Use `--recursive=false` to disable recursive discovery and only upload the module in the specified directory.

## Handling Existing Module Versions

By default, the `upload module` command will silently skip modules that already exist in the registry.
This ensures that re-running uploads is idempotent and doesn't fail.

This behavior can be disabled by setting the `--ignore-existing=false` flag.

## Module version constraints

Version constraints allow you to filter which module versions get uploaded based on their version string.
This is useful for implementing branch-specific upload policies in CI/CD pipelines, for example.

!!! note

    Version constraints work with both versioning approaches (version specified in `boring-registry.hcl` or via the `--version` flag).

### Semantic Version Constraints

The `--version-constraints-semver` flag filters modules based on semantic versioning rules.
It uses the [Version Constraint Syntax](https://opentofu.org/docs/language/expressions/version-constraints/) used in OpenTofu/Terraform.

The following showcases some common use cases:

#### Exclude pre-releases (production releases only)

```bash
# Only uploads releases like 0.1.0, 2.1.5
# Skips pre-releases like 1.0.0-beta, 2.0.0-rc1
boring-registry upload module --version-constraints-semver=">=v0" ./modules/
```
This is useful for restricting CI to only publish releases from the `main` branch.

Multiple version constraints can be passed as well:

```bash
# Only publishes release versions >= 1.0.0 and < 3.0.0
boring-registry upload module --version-constraints-semver=">=1.0.0,<3.0.0" ./modules/
```

### Regular Expression Constraints

The `--version-constraints-regex` flag allows you to filter versions using a regular expression pattern.
This provides more flexible matching than semantic version constraints.

#### Only upload pre-releases

```bash
# Match only pre-release versions like 1.0.0-beta, 2.0.0-rc.1
boring-registry upload module --version-constraints-regex="^[0-9]+\.[0-9]+\.[0-9]+-|\d*[a-zA-Z-][0-9a-zA-Z-]*$" ./modules/`
```
This is useful for allowing pre-releases to be published from feature branches for testing, while preventing publishing releases from non-main branches.

## Examples

### Fail early if module version already exists

By default the `upload module` command will silently ignore already uploaded versions of a module and return exit code `0`.
This can become a problem for tagging mono-repositories as it is not clear if the module version is new or already uploaded.

Setting the `--ignore-existing=false` flag will force the upload command to return exit code `1` in such case.
In combination with `--recursive=false` the exit code can be used to create a Git tag only if a new version was uploaded.

```shell
for i in $(ls -d */); do
  printf "Operating on module \"${i%%/}\"\n"
  # upload the given directory
  ./boring-registry upload module --recursive=false --ignore-existing=false ${i%%/}
  # tag the repo with a tag composed out of the boring-registry.hcl if not already exist
  if [ $? -eq 0 ]; then
    # git tag the repository with the version from boring-registry.hcl
    # hint: use mattolenik/hclq to parse the hcl file
  fi
done
```
