# Publish Modules

Example HCL configuration using a module referenced from the registry:

```hcl
module "tls-private-key" {
  source = "boring-registry.example.com/acme/tls-private-key/aws"
  version = "~> 0.1"
}
```

## Uploading modules using the CLI

Modules can be published to the registry with the `upload` command.
The command expects a directory as argument, which is then walked recursively in search of `boring-registry.hcl` files.

The `boring-registry.hcl` file should be placed in the root directory of the module and should contain a `metadata` block like the following:

```hcl
metadata {
  namespace = "acme"
  name      = "tls-private-key"
  provider  = "aws"
  version   = "0.1.0"
}
```

When running the upload command, the module is then packaged up and published to the registry.

## Recursive vs. non-recursive upload

Walking the directory recursively is the default behavior of the `upload` command.
This way all modules underneath the current directory will be checked for `boring-registry.hcl` files and modules will be packaged and uploaded if they not already exist
However, this can be unwanted in certain situations e.g. if a `.terraform` directory is present containing other modules that have a configuration file.
The `--recursive=false` flag will omit this behavior.

## Fail early if module version already exists

By default the upload command will silently ignore already uploaded versions of a module and return exit code `0`.
For tagging mono-repositories this can become a problem as it is not clear if the module version is new or already uploaded.
The `--ignore-existing=false` parameter will force the upload command to return exit code `1` in such a case.
In combination with `--recursive=false` the exit code can be used to tag the Git repository only if a new version was uploaded.

```shell
for i in $(ls -d */); do
  printf "Operating on module \"${i%%/}\"\n"
  # upload the given directory
  ./boring-registry upload --type gcs -gcs-bucket=my-boring-registry-upload-bucket --recursive=false --ignore-existing=false ${i%%/}
  # tag the repo with a tag composed out of the boring-registry.hcl if not already exist
  if [ $? -eq 0 ]; then
    # git tag the repository with the version from boring-registry.hcl
    # hint: use mattolenik/hclq to parse the hcl file
  fi
done
```

## Module version constraints

The `--version-constraints-semver` flag lets you specify a range of acceptable semver versions for modules.
It expects a specially formatted string containing one or more conditions, which are separated by commas.
The syntax is similar to the [Terraform Version Constraint Syntax](https://www.terraform.io/docs/language/expressions/version-constraints.html#version-constraint-syntax).

In order to exclude all SemVer pre-releases, you can e.g. use `--version-constraints-semver=">=v0"`, which will instruct the boring-registry cli to only upload non-pre-releases to the registry.
This would for example be useful to restrict CI to only publish releases from the `main` branch.

The `--version-constraints-regex` flag lets you specify a regex that module versions have to match.
In order to only match pre-releases, you can e.g. use `--version-constraints-regex="^[0-9]+\.[0-9]+\.[0-9]+-|\d*[a-zA-Z-][0-9a-zA-Z-]*$"`.
This would for example be useful to prevent publishing releases from non-`main` branches, while allowing pre-releases to test out pull requests for example.

