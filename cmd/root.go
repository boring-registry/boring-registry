package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

const (
	projectName = "boring-registry"
	envPrefix   = "BORING_REGISTRY"
)

const (
	logKeyCaller    = "caller"
	logKeyHostname  = "hostname"
	logKeyTimestamp = "timestamp"
)

var (
	flagJSON  bool
	flagDebug bool

	// S3 options.
	flagS3Bucket    string
	flagS3Prefix    string
	flagS3Region    string
	flagS3Endpoint  string
	flagS3PathStyle bool

	// GCS options.
	flagGCSBucket          string
	flagGCSPrefix          string
	flagGCSServiceAccount  string
	flagGCSSignedURL       bool
	flagGCSSignedURLExpiry time.Duration
)

var (
	logger log.Logger
)

var rootCmd = &cobra.Command{
	Use: projectName,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		logger = setupLogger(os.Stdout)
		return initializeConfig(cmd)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "enable json logging")
	rootCmd.PersistentFlags().BoolVar(&flagDebug, "debug", false, "enable debug logging")
	rootCmd.PersistentFlags().StringVar(&flagS3Bucket, "storage-s3-bucket", "", "S3 bucket to use for the registry")
	rootCmd.PersistentFlags().StringVar(&flagS3Prefix, "storage-s3-prefix", "", "S3 bucket prefix to use for the registry")
	rootCmd.PersistentFlags().StringVar(&flagS3Region, "storage-s3-region", "", "S3 bucket region to use for the registry")
	rootCmd.PersistentFlags().StringVar(&flagS3Endpoint, "storage-s3-endpoint", "", "S3 bucket endpoit URL (required for MINIO)")
	rootCmd.PersistentFlags().BoolVar(&flagS3PathStyle, "storage-s3-pathstyle", false, "S3 use PathStyle (required for MINIO)")

	rootCmd.PersistentFlags().StringVar(&flagGCSBucket, "storage-gcs-bucket", "", "Bucket to use when using the GCS registry type")
	rootCmd.PersistentFlags().StringVar(&flagGCSPrefix, "storage-gcs-prefix", "", "Prefix to use when using the GCS registry type")
	rootCmd.PersistentFlags().StringVar(&flagGCSServiceAccount, "storage-gcs-sa-email", "", "Google service account email to be used for Application Default Credentials (ADC).\n"+
		"GOOGLEPersistent_APPLICATION_CREDENTIALS environment variable might be used as alternative.\n"+
		"For GCPersistentS presigned URLs this SA needs the `iam.serviceAccountTokenCreator` role")
	rootCmd.PersistentFlags().BoolVar(&flagGCSSignedURL, "storage-gcs-signedurl", false, "Generate GCS signedURL (public) instead of relying on GCP credentials being set on terraform init.\n"+
		"WARNINPersistentG: only use in combination with `api-key` option")
	rootCmd.PersistentFlags().DurationVar(&flagGCSSignedURLExpiry, "storage-gcs-signedurl-expiry", 30*time.Second, "Generate GCS signed URL valid for X seconds. Only meaningful if used in combination with `gcs-signedurl`")
}

func initializeConfig(cmd *cobra.Command) error {
	v := viper.New()
	v.SetEnvPrefix(envPrefix)
	v.AutomaticEnv()
	bindFlags(cmd, v)
	return nil
}

func setupLogger(w io.Writer) log.Logger {
	logger := log.NewLogfmtLogger(w)

	if flagJSON {
		logger = log.NewJSONLogger(w)
	}

	logger = log.With(logger,
		logKeyCaller, log.Caller(5),
		logKeyTimestamp, log.DefaultTimestampUTC,
	)

	logLevel := level.AllowInfo()
	{
		if flagDebug {
			logLevel = level.AllowDebug()
		}
		logger = level.NewFilter(logger, logLevel)
	}

	if hostname, err := os.Hostname(); err == nil {
		logger = log.With(logger, logKeyHostname, hostname)
	}

	return logger
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if strings.Contains(f.Name, "-") {
			envVarSuffix := strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
			v.BindEnv(f.Name, fmt.Sprintf("%s_%s", envPrefix, envVarSuffix))
		}

		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func setupS3Registry() (module.Registry, error) {
	return module.NewS3Registry(flagS3Bucket,
		module.WithS3RegistryBucketPrefix(flagS3Prefix),
		module.WithS3RegistryBucketRegion(flagS3Region),
		module.WithS3RegistryBucketEndpoint(flagS3Endpoint),
		module.WithS3RegistryPathStyle(flagS3PathStyle),
	)
}

func setupGCSRegistry() (module.Registry, error) {
	return module.NewGCSRegistry(flagGCSBucket,
		module.WithGCSRegistryBucketPrefix(flagGCSPrefix),
		module.WithGCSRegistrySignedURL(flagGCSSignedURL),
		module.WithGCSServiceAccount(flagGCSServiceAccount),
		module.WithGCSSignedUrlExpiry(int64(flagGCSSignedURLExpiry.Seconds())),
	)
}
