# boring-registry

Boring Registry is an open source Terraform Module Registry.

The registry is designed to be simple and only implements the "Module Registry Protocol" and apart from the registry storage backend (currently only S3 is supported), there are no external dependencies, it also does not ship with a UI.

## Getting Started

The registry supports two modes:
  * Server - The server runs the registry API
  * Upload - Uploads modules to the configured registry

To run the server you need to specify which registry to use (currently only S3 is supported):

**Example:**
```bash
boring-registry server -type=s3 -s3.bucket=terraform-registry-test
```

To upload modules to the registry you need to specify which registry to use (currently only S3 is supported) and which local directory to work from.

**Example:**
```bash
boring-registry upload -type=s3 -s3.bucket=terraform-registry-test <DIR> 
```

## Configuration

The boring-registry does not rely on any configuration files. Instead, everything can be configured using flags or environment variables.
**Important Note**: Flags have higher priority than environment variables. Environment variables are always prefixed with `BORING_REGISTRY`.

**Example:**
To enable debug logging you can either pass the flag: `-debug` or set the environment variable: `BORING_REGISTRY_DEBUG=true`.

## Roadmap

The project is in its very early stages and there is a lot of things we want to tackle. This may mean some breaking changes in the future, but once the project is stable enough we will put quite heavy focus on keeping changes backwards compatible. This project started out as a single server (just serving the Module Registry Protocol), but is now becoming a single binary that can host the server and allow operators to manage the registry using a streamlined interface.

* Module maintenance - The CLI should be able to inspect/view, modify and delete existing modules.
* Migration helpers - We want the CLI to be able to provide some automation when migrating to the boring-registry. This is currently a manual task and is quite time consuming.
