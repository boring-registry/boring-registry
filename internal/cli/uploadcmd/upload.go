package uploadcmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/TierMobility/boring-registry/internal/cli/help"
	"github.com/TierMobility/boring-registry/internal/cli/rootcmd"
	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/hashicorp/go-version"
	"github.com/peterbourgon/ff/v3"
	"github.com/peterbourgon/ff/v3/ffcli"
)

// Variables used to store flag values for further parsing and validation
var (
	versionConstraintsSemver string
	versionConstraintsRegex  string
)

type Config struct {
	*rootcmd.Config

	RegistryType string
	S3Bucket     string
	S3Prefix     string
	S3Region     string
	S3Endpoint   string
	S3PathStyle  bool

	GCSBucket string
	GCSPrefix string

	APIKey                 string
	ListenAddress          string
	TelemetryListenAddress string
	UploadRecursive        bool
	IgnoreExistingModule   bool

	// VersionConstraintsSemver holds semver constraints used to assess if discovered modules should be processed
	VersionConstraintsSemver version.Constraints

	// VersionConstraintsRegex holds regex constraints used to assess if discovered modules should be processed
	VersionConstraintsRegex *regexp.Regexp
}

func (c *Config) Exec(ctx context.Context, args []string) error {

	if len(args) < 1 {
		return errors.New("upload requires at least 1 args")
	}

	var registry module.Registry

	switch c.RegistryType {
	case "s3":
		if c.S3Bucket == "" {
			return errors.New("missing flag -s3-bucket")
		}
		if c.S3Region == "" {

		}
		reg, err := module.NewS3Registry(c.S3Bucket,
			module.WithS3RegistryBucketPrefix(c.S3Prefix),
			module.WithS3RegistryBucketRegion(c.S3Region),
			module.WithS3RegistryBucketEndpoint(c.S3Endpoint),
			module.WithS3RegistryPathStyle(c.S3PathStyle),
		)
		if err != nil {
			return err
		}
		registry = reg
	case "gcs":
		if c.GCSBucket == "" {
			return errors.New("missing flag -gcs-bucket")
		}

		reg, err := module.NewGCSRegistry(c.GCSBucket,
			module.WithGCSRegistryBucketPrefix(c.GCSPrefix),
		)
		if err != nil {
			return err
		}
		registry = reg
	default:
		return flag.ErrHelp
	}

	if _, err := os.Stat(args[0]); errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Validate the semver version constraints
	if versionConstraintsSemver != "" {
		constraints, err := version.NewConstraint(versionConstraintsSemver)
		if err != nil {
			return err
		}
		c.VersionConstraintsSemver = constraints
	}

	// Validate the regex version constraints
	if versionConstraintsRegex != "" {
		constraints, err := regexp.Compile(versionConstraintsRegex)
		if err != nil {
			return fmt.Errorf("invalid regex given: %v", err)
		}
		c.VersionConstraintsRegex = constraints
	}

	return c.archiveModules(args[0], registry)
}

func New(config *rootcmd.Config) *ffcli.Command {
	cfg := &Config{
		Config: config,
	}

	fs := flag.NewFlagSet("boring-registry upload", flag.ExitOnError)
	fs.StringVar(&cfg.RegistryType, "type", "", "Registry type to use (currently only \"s3\" and \"gcs\" is supported)")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "", "Bucket to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Prefix, "s3-prefix", "", "Prefix to use when using the S3 registry type")
	fs.StringVar(&cfg.S3Region, "s3-region", "", "Region of the S3 bucket when using the S3 registry type")
	fs.StringVar(&cfg.S3Endpoint, "s3-endpoint", "", "Endpoint of the S3 bucket when using the S3 registry type")
	fs.BoolVar(&cfg.S3PathStyle, "s3-pathstyle", false, "Use PathStyle for S3 bucket when using the S3 registry type")
	fs.StringVar(&cfg.GCSBucket, "gcs-bucket", "", "Bucket to use when using the GCS registry type")
	fs.StringVar(&cfg.GCSPrefix, "gcs-prefix", "", "Prefix to use when using the GCS registry type")
	fs.StringVar(&versionConstraintsSemver, "version-constraints-semver", "", "Limit the module versions that are eligible for upload with version constraints. The version string has to be formatted as a string literal containing one or more conditions, which are separated by commas. Can be combined with the -version-constrained-regex flag")
	fs.StringVar(&versionConstraintsRegex, "version-constraints-regex", "", "Limit the module versions that are eligible for upload with a regex that a version has to match. Can be combined with the -version-constraints-semver flag")
	fs.BoolVar(&cfg.UploadRecursive, "recursive", true, "Recursively traverse <dir> and upload all modules in subdirectories")
	fs.BoolVar(&cfg.IgnoreExistingModule, "ignore-existing", true, "Ignore already existing modules. If set to false upload will fail immediately if a module already exists in that version")

	config.RegisterFlags(fs)

	return &ffcli.Command{
		Name:       "upload",
		UsageFunc:  help.UsageFunc,
		ShortUsage: "boring-registry upload [flags] <dir>",
		ShortHelp:  "Uploads modules to a registry.",
		FlagSet:    fs,
		Options:    []ff.Option{ff.WithEnvVarPrefix(help.EnvVarPrefix)},
		LongHelp: fmt.Sprint(`  Uploads modules to a registry.

  This command requires some configuration, 
  such as which registry type to use and a directory to search for modules.

  The upload command walks the directory recursively and looks
  for modules with a boring-registry.hcl file in it. The file is then parsed
  to get the module metadata the module is then archived and uploaded to the given registry.

  Example Usage: boring-registry upload -type=s3 -s3-bucket=example-bucket modules/

  For more options see the available options below.`),
		Exec: cfg.Exec,
	}
}
