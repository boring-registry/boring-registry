package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TierMobility/boring-registry/pkg/module"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
)

const (
	apiVersion         = "v1"
	moduleSpecFileName = "boring-registry.hcl"
)

var (
	prefix = fmt.Sprintf(`/%s`, apiVersion)
)

func main() {
	var (
		flagDebug = flag.Bool("debug", func() bool {
			if v := os.Getenv("REGISTRY_DEBUG"); v != "" {
				val, err := strconv.ParseBool(v)
				if err != nil {
					return false
				}

				return val
			}

			return false
		}(), "Enables debug logging")

		flagRegistry = flag.String("registry", func() string {
			if v := os.Getenv("REGISTRY_TYPE"); v != "" {
				return v
			}

			return "s3"
		}(), "Registry type to use for the registry")

		flagRegistryS3Bucket       = flag.String("registry.s3.bucket", os.Getenv("REGISTRY_S3_BUCKET"), "Bucket to use for the S3 registry type")
		flagRegistryS3BucketPrefix = flag.String("registry.s3.bucket-prefix", os.Getenv("REGISTRY_S3_BUCKET_PREFIX"), "Bucket prefix to  use for the S3 registry type")
		flagRegistryS3BucketRegion = flag.String("registry.s3.bucket-region", os.Getenv("REGISTRY_S3_BUCKET_REGION"), "Region of the bucket to use for the S3 registry type")
	)
	flag.Parse()

	var logger log.Logger
	{
		logLevel := level.AllowInfo()
		if *flagDebug {
			logLevel = level.AllowAll()
		}
		logger = log.With(
			log.NewJSONLogger(os.Stdout),
			"timestamp", log.DefaultTimestampUTC,
		)
		logger = level.NewFilter(logger, logLevel)
	}

	hostname, err := os.Hostname()
	if err != nil {
		abort(logger, err)
	}

	logger = log.With(logger, "hostname", hostname)

	if len(flag.Args()) < 1 {
		abort(logger, errors.New("missing DIR argument"))
	}

	var registry module.Registry

	switch *flagRegistry {
	case "s3":
		registry, err = module.NewS3Registry(*flagRegistryS3Bucket, module.WithS3RegistryBucketPrefix(*flagRegistryS3BucketPrefix), module.WithS3RegistryBucketRegion(*flagRegistryS3BucketRegion))
		if err != nil {
			abort(logger, err)
		}
	default:
		abort(logger, fmt.Errorf("invalid registry type '%s'", *flagRegistry))
	}

	ctx := context.Background()

	abort(logger, filepath.Walk(flag.Args()[0], func(path string, fi os.FileInfo, err error) error {
		if fi.Name() == moduleSpecFileName {
			spec, err := module.ParseFile(path)
			if err != nil {
				return err
			}

			body, err := archive(filepath.Dir(path))
			if err != nil {
				return err
			}

			res, err := registry.UploadModule(ctx, spec.Metadata.Namespace, spec.Metadata.Name, spec.Metadata.Provider, spec.Metadata.Version, body)
			if err != nil {
				if errors.Cause(err) == module.ErrAlreadyExists {
					level.Debug(logger).Log(
						"msg", "skipping already existing module",
						"err", err,
					)
				} else {
					return err
				}
			} else {
				level.Info(logger).Log(
					"msg", "successfully uploaded module",
					"module", res,
				)
			}
		}
		return nil
	}))
}

func abort(logger log.Logger, err error) {
	if err == nil {
		return
	}

	logger.Log("err", err)
	os.Exit(1)
}

func archive(dir string) (io.Reader, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}

	var baseDir string
	if info.IsDir() && info.Name() != "." {
		baseDir = filepath.Base(dir)
	}

	buf := new(bytes.Buffer)

	gw := gzip.NewWriter(buf)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err = filepath.Walk(dir, func(file string, fi os.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(file, dir))
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}

			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return buf, nil
}
