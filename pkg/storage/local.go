package storage

import (
	"context"
	"fmt"
	"io"
	llog "log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
)

type LocalFileSystem interface {
	OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error)
	ReadFile(name string) ([]byte, error)
	Stat(name string) (os.FileInfo, error)
	ReadDir(name string) ([]os.DirEntry, error)
	MkdirAll(name string, perm os.FileMode) error
}

type FileServer interface {
	ListenAndServe() error
	Addr() string
}

type LocalStorage struct {
	fs                  LocalFileSystem
	server              FileServer
	serverEndpoint      string
	storageDir          string
	moduleArchiveFormat string
}

func NewLocalStorage(fs LocalFileSystem, server FileServer, storageDir, serverEndpoint, moduleArchiveFormat string) *LocalStorage {
	// when use client to upload or migrate, no need to set up the http file server
	if server != nil && !reflect.ValueOf(server).IsNil() {
		go func() {
			if err := server.ListenAndServe(); err != nil {
				llog.Printf("error: %+v", err)
			}
		}()
	}

	return &LocalStorage{
		fs:                  fs,
		server:              server,
		serverEndpoint:      serverEndpoint,
		storageDir:          storageDir,
		moduleArchiveFormat: moduleArchiveFormat,
	}
}

func NewDefaultLocalStorage(storageDir, moduleArchiveFormat, endpoint string, serverAddr string) *LocalStorage {
	var server *fileServer
	if len(serverAddr) != 0 {
		server = &fileServer{
			addr: serverAddr,
			path: storageDir,
		}
	}

	return NewLocalStorage(&fs{}, server, storageDir, endpoint, moduleArchiveFormat)
}

func (ls *LocalStorage) GetProvider(ctx context.Context, namespace, name, version, os_, arch string) (core.Provider, error) {
	if len(namespace) == 0 {
		return core.Provider{}, errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return core.Provider{}, errors.New("name argument is empty")
	}

	if len(version) == 0 {
		return core.Provider{}, errors.New("namespace argument is empty")
	}

	if len(os_) == 0 {
		return core.Provider{}, errors.New("os argument is empty")
	}

	if len(arch) == 0 {
		return core.Provider{}, errors.New("arch argument is empty")
	}

	if ls.server == nil || reflect.ValueOf(ls.server).IsNil() {
		return core.Provider{}, errors.New("http file server is not set up")
	}

	providerPrefix, err := providerStoragePrefix(ls.storageDir, internalProviderType, "", namespace, name)
	if err != nil {
		return core.Provider{}, err
	}

	provider := core.Provider{
		Namespace: namespace,
		Name:      name,
		Version:   version,
		OS:        os_,
		Arch:      arch,
	}

	archive, err := provider.ArchiveFileName()
	if err != nil {
		return core.Provider{}, err
	}

	archivePath := filepath.Join(providerPrefix, archive)
	if exist, _ := ls.isLocalFileExist(archivePath); !exist {
		return core.Provider{}, errors.New("archive file not exist")
	}

	httpProviderPrefix, _ := providerStoragePrefix("", internalProviderType, "", namespace, name)
	httpArchivePath := path.Join(httpProviderPrefix, archive)

	provider.Filename = archive
	provider.DownloadURL = fmt.Sprintf("http://%s/%s", ls.serverEndpoint, httpArchivePath)

	shaSum, err := provider.ShasumFileName()
	if err != nil {
		return core.Provider{}, err
	}

	shaSumPath := filepath.Join(providerPrefix, shaSum)
	if exist, _ := ls.isLocalFileExist(shaSumPath); !exist {
		return core.Provider{}, errors.New("shaSum file not exist")
	}

	httpSHASumPath := path.Join(httpProviderPrefix, shaSum)
	provider.SHASumsURL = fmt.Sprintf("http://%s/%s", ls.serverEndpoint, httpSHASumPath)

	f, err := ls.fs.OpenFile(shaSumPath, os.O_RDWR, 0644)
	if err != nil {
		return core.Provider{}, err
	}

	sha, err := readSHASums(f, archive)
	if err != nil {
		return core.Provider{}, err
	}
	provider.SHASum = sha

	sig, err := provider.ShasumSignatureFileName()
	if err != nil {
		return core.Provider{}, err
	}

	sigPath := filepath.Join(providerPrefix, sig)
	if exist, _ := ls.isLocalFileExist(sigPath); !exist {
		return core.Provider{}, errors.New("sig file not exist")
	}

	httpSigPath := path.Join(httpProviderPrefix, sig)
	provider.SHASumsSignatureURL = fmt.Sprintf("http://%s/%s", ls.serverEndpoint, httpSigPath)

	keyPath := signingKeysPath(ls.storageDir, namespace)
	if exist, _ := ls.isLocalFileExist(keyPath); !exist {
		return core.Provider{}, errors.New("key file not exist")
	}

	keysRaw, err := ls.fs.ReadFile(keyPath)
	if err != nil {
		return core.Provider{}, err
	}

	keys, err := unmarshalSigningKeys(keysRaw)
	if err != nil {
		return core.Provider{}, err
	}
	provider.SigningKeys = *keys

	return provider, nil
}

func (ls *LocalStorage) ListProviderVersions(ctx context.Context, namespace, name string) ([]core.ProviderVersion, error) {
	if len(namespace) == 0 {
		return nil, errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return nil, errors.New("name argument is empty")
	}

	dir, err := providerStoragePrefix(ls.storageDir, internalProviderType, "", namespace, name)
	if err != nil {
		return nil, err
	}

	if exist, _ := ls.isLocalDirExist(dir); !exist {
		return []core.ProviderVersion{}, nil
	}

	entries, err := ls.fs.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrap(err, "read provider dir failed")
	}

	collection := NewCollection()
	for _, entry := range entries {
		provider, err := core.NewProviderFromArchive(entry.Name())
		if err != nil {
			continue
		}

		collection.Add(provider)
	}

	result := collection.List()
	if len(result) == 0 {
		return nil, fmt.Errorf("no provider versions found for %s/%s", namespace, name)
	}

	return result, nil
}

func (ls *LocalStorage) UploadProviderReleaseFiles(ctx context.Context, namespace, name, filename string, file io.Reader) error {
	if len(namespace) == 0 {
		return errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return errors.New("name argument is empty")
	}

	if len(filename) == 0 {
		return errors.New("filename argument is empty")
	}

	if file == nil {
		return errors.New("nil file reader")
	}

	dir, err := providerStoragePrefix(ls.storageDir, internalProviderType, "", namespace, name)
	if err != nil {
		return err
	}

	exist, err := ls.isLocalDirExist(dir)
	if err != nil {
		return err
	}

	if !exist {
		if err := ls.fs.MkdirAll(dir, 0744); err != nil {
			return err
		}
	}

	path := filepath.Join(dir, filename)
	dst, err := ls.fs.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	if _, err := io.Copy(dst, file); err != nil {
		return err
	}

	return nil
}

func (ls *LocalStorage) SigningKeys(ctx context.Context, namespace string) (*core.SigningKeys, error) {
	if len(namespace) == 0 {
		return nil, errors.New("namespace arguement is empty")
	}

	keyPath := signingKeysPath(ls.storageDir, namespace)
	raw, err := ls.fs.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	return unmarshalSigningKeys(raw)
}

func (ls *LocalStorage) MigrateProviders(ctx context.Context, logger log.Logger, dryRun bool) error {
	return nil
}

func (ls *LocalStorage) isLocalFileExist(path string) (bool, error) {
	s, err := ls.fs.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if s.IsDir() {
		return false, errors.New("local file path is a directory")
	}

	return true, nil
}

func (ls *LocalStorage) isLocalDirExist(path string) (bool, error) {
	s, err := ls.fs.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !s.IsDir() {
		return false, errors.New("local dir path is a file")
	}

	return true, nil
}

func (ls *LocalStorage) GetModule(ctx context.Context, namespace, name, provider, version string) (core.Module, error) {
	if len(namespace) == 0 {
		return core.Module{}, errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return core.Module{}, errors.New("name argument is empty")
	}

	if len(provider) == 0 {
		return core.Module{}, errors.New("provider argument is empty")
	}

	if len(version) == 0 {
		return core.Module{}, errors.New("version argument is empty")
	}

	path := modulePath(ls.storageDir, namespace, name, provider, version, ls.moduleArchiveFormat)
	if exist, err := ls.isLocalFileExist(path); err != nil {
		return core.Module{}, err
	} else if !exist {
		return core.Module{}, ErrModuleNotFound
	}

	httpPrefix := fmt.Sprintf("http://%s", ls.serverEndpoint)
	downloadURL := fmt.Sprintf("%s/%s",
		httpPrefix,
		modulePath("", namespace, name, provider, version, ls.moduleArchiveFormat),
	)
	return core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: downloadURL,
	}, nil
}

func (ls *LocalStorage) ListModuleVersions(ctx context.Context, namespace, name, provider string) ([]core.Module, error) {
	if len(namespace) == 0 {
		return []core.Module{}, errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return []core.Module{}, errors.New("name argument is empty")
	}

	if len(provider) == 0 {
		return []core.Module{}, errors.New("provider argument is empty")
	}

	dir := modulePathPrefix(ls.storageDir, namespace, name, provider)
	entries, err := ls.fs.ReadDir(dir)
	if err != nil {
		return []core.Module{}, errors.Wrap(ErrModuleListFailed, err.Error())
	}

	var (
		ms         []core.Module
		httpPrefix = fmt.Sprintf("http://%s", ls.serverEndpoint)
	)
	for _, entry := range entries {
		m, err := moduleFromObject(filepath.Join(dir, entry.Name()), ls.moduleArchiveFormat)
		if err != nil {
			continue
		}

		m.DownloadURL = fmt.Sprintf("%s/%s", httpPrefix,
			modulePath("", m.Namespace, m.Name, m.Provider, m.Version, ls.moduleArchiveFormat),
		)
		ms = append(ms, *m)
	}

	return ms, nil
}

func (ls *LocalStorage) UploadModule(ctx context.Context, namespace, name, provider, version string, body io.Reader) (core.Module, error) {
	if len(namespace) == 0 {
		return core.Module{}, errors.New("namespace argument is empty")
	}

	if len(name) == 0 {
		return core.Module{}, errors.New("name argument is empty")
	}

	if len(provider) == 0 {
		return core.Module{}, errors.New("provider argument is empty")
	}

	if len(version) == 0 {
		return core.Module{}, errors.New("version argument is empty")
	}

	dir := modulePathPrefix(ls.storageDir, namespace, name, provider)
	if exist, err := ls.isLocalDirExist(dir); err != nil {
		return core.Module{}, err
	} else if !exist {
		if err := ls.fs.MkdirAll(dir, 0744); err != nil {
			return core.Module{}, err
		}
	}

	path := modulePath(ls.storageDir, namespace, name, provider, version, ls.moduleArchiveFormat)
	dst, err := ls.fs.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return core.Module{}, err
	}

	if _, err := io.Copy(dst, body); err != nil {
		return core.Module{}, err
	}

	httpPrefix := fmt.Sprintf("http://%s", ls.serverEndpoint)
	downloadURL := fmt.Sprintf("%s/%s", httpPrefix,
		modulePath("", namespace, name, provider, version, ls.moduleArchiveFormat),
	)
	return core.Module{
		Namespace:   namespace,
		Name:        name,
		Provider:    provider,
		Version:     version,
		DownloadURL: downloadURL,
	}, nil
}

func (ls *LocalStorage) MigrateModules(ctx context.Context, logger log.Logger, dryRun bool) error {
	return nil
}

type fs struct{}

func (fs *fs) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	return os.OpenFile(name, flag, perm)
}

func (fs *fs) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (fs *fs) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (fs *fs) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func (fs *fs) MkdirAll(name string, perm os.FileMode) error {
	return os.MkdirAll(name, perm)
}

type fileServer struct {
	addr string
	path string
}

func (s *fileServer) ListenAndServe() error {
	return http.ListenAndServe(s.addr, http.FileServer(http.Dir(s.path)))
}

func (s *fileServer) Addr() string {
	return s.addr
}
