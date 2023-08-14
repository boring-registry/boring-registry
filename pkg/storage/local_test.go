package storage

import (
	"bytes"
	"context"
	"io"
	iofs "io/fs"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/TierMobility/boring-registry/pkg/core"
	"github.com/stretchr/testify/assert"
)

type localStorageAttrsForTest struct {
	lfs                 LocalFileSystem
	server              FileServer
	storageDir          string
	serverEndpoint      string
	moduleArchiveFormat string
}

type mockLocalFileInfo struct {
	isDir bool
}

func (ml *mockLocalFileInfo) Name() string {
	return ""
}

func (ml *mockLocalFileInfo) Size() int64 {
	return 0
}

func (ml *mockLocalFileInfo) Mode() iofs.FileMode {
	return iofs.ModeAppend
}

func (ml *mockLocalFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (ml *mockLocalFileInfo) IsDir() bool {
	return ml.isDir
}

func (ml *mockLocalFileInfo) Sys() any {
	return nil
}

type mockFile struct {
	buf *bytes.Buffer
}

func (mf *mockFile) Read(p []byte) (n int, err error) {
	return mf.buf.Read(p)
}

func (mf *mockFile) Write(p []byte) (n int, err error) {
	return mf.buf.Write(p)
}

func (mf *mockFile) Close() error {
	return nil
}

type mockDirEntry struct {
	name  string
	isDir bool
	info  os.FileInfo
	mode  os.FileMode
}

func (m *mockDirEntry) Name() string {
	return m.name
}

func (m *mockDirEntry) IsDir() bool {
	return m.isDir
}

func (m *mockDirEntry) Info() (os.FileInfo, error) {
	return m.info, nil
}

func (m *mockDirEntry) Type() os.FileMode {
	return m.mode
}

type mockLocalFileSystem struct {
	files     map[string]io.ReadWriteCloser
	fileStats map[string]os.FileInfo
	dirs      map[string][]os.DirEntry
	mkdirErr  bool
}

func (fs mockLocalFileSystem) OpenFile(name string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if file, exist := fs.files[name]; !exist {
		return nil, errors.New("file not exist")
	} else {
		return file, nil
	}
}

func (fs mockLocalFileSystem) ReadFile(name string) ([]byte, error) {
	file, exist := fs.files[name]
	if !exist {
		return nil, errors.New("file not exist")
	}

	return ioutil.ReadAll(file)
}

func (fs mockLocalFileSystem) Stat(name string) (os.FileInfo, error) {
	stat, exist := fs.fileStats[name]
	if !exist {
		return nil, errors.New("file not exist")
	}

	return stat, nil
}

func (fs mockLocalFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	dir, exist := fs.dirs[name]
	if !exist {
		return nil, errors.New("dir not exist")
	}

	return dir, nil
}

func (fs mockLocalFileSystem) MkdirAll(name string, perm os.FileMode) error {
	if fs.mkdirErr {
		return errors.New("mkdir failed")
	}

	return nil
}

type mockFileServer struct {
	addr string
}

func (m *mockFileServer) ListenAndServe() error {
	return nil
}

func (m *mockFileServer) Addr() string {
	return m.addr
}

func TestLocalStorage_GetProvider(t *testing.T) {
	type args struct {
		namespace string
		name      string
		version   string
		os_       string
		arch      string
	}

	cases := []struct {
		name      string
		attrs     localStorageAttrsForTest
		args      args
		expected  core.Provider
		expectErr bool
	}{
		{
			name: "emtpy namespaces",
			args: args{
				namespace: "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy name",
			args: args{
				namespace: "xxx",
				name:      "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy version",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy os",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "xxx",
				os_:       "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "emtpy arch",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				version:   "xxx",
				os_:       "xxx",
				arch:      "",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
			},
			expectErr: true,
		},
		{
			name: "provider not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
				lfs:        mockLocalFileSystem{},
			},
			expectErr: true,
		},
		{
			name: "shasum not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			attrs: localStorageAttrsForTest{
				storageDir:     "/test",
				serverEndpoint: "localhost:8080",
				server: &fileServer{
					addr: ":8080",
				},
				lfs: mockLocalFileSystem{
					fileStats: map[string]os.FileInfo{
						"/test/providers/a/b/terraform-provider-b_c_d_e.zip": &mockLocalFileInfo{},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "shasum sig not exist in local file system",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "/test",
				lfs: mockLocalFileSystem{
					fileStats: map[string]os.FileInfo{
						"/test/providers/a/b/terraform-provider-b_c_d_e.zip":    &mockLocalFileInfo{},
						"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockLocalFileInfo{},
					},
					files: map[string]io.ReadWriteCloser{
						"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockFile{
							buf: bytes.NewBuffer([]byte(`198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120  terraform-provider-b_c_d_e.zip`)),
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "get provider success",
			args: args{
				namespace: "a",
				name:      "b",
				version:   "c",
				os_:       "d",
				arch:      "e",
			},
			attrs: localStorageAttrsForTest{
				storageDir:     "/test",
				serverEndpoint: "localhost",
				server: &mockFileServer{
					addr: ":8080",
				},
				lfs: mockLocalFileSystem{
					fileStats: map[string]os.FileInfo{
						"/test/providers/a/b/terraform-provider-b_c_d_e.zip":        &mockLocalFileInfo{},
						"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS":     &mockLocalFileInfo{},
						"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS.sig": &mockLocalFileInfo{},
						"/test/providers/a/signing-keys.json":                       &mockLocalFileInfo{},
					},
					files: map[string]io.ReadWriteCloser{
						"/test/providers/a/b/terraform-provider-b_c_SHA256SUMS": &mockFile{
							buf: bytes.NewBuffer([]byte(`198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120  terraform-provider-b_c_d_e.zip`)),
						},
						"/test/providers/a/signing-keys.json": &mockFile{
							buf: bytes.NewBuffer([]byte(`
                            {
                              "gpg_public_keys": [
                                {
                                  "key_id": "51852D87348FFC4C",
                                  "ascii_armor": "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n..."
                                }
                              ]
                            }`)),
						},
					},
				},
			},
			expectErr: false,
			expected: core.Provider{
				Namespace:           "a",
				Name:                "b",
				Version:             "c",
				OS:                  "d",
				Arch:                "e",
				Filename:            "terraform-provider-b_c_d_e.zip",
				DownloadURL:         "http://localhost/providers/a/b/terraform-provider-b_c_d_e.zip",
				SHASum:              "198e1bb88df52eb1201953d0b1e6c4ac48eac2440e395887cb4eca655d68b120",
				SHASumsURL:          "http://localhost/providers/a/b/terraform-provider-b_c_SHA256SUMS",
				SHASumsSignatureURL: "http://localhost/providers/a/b/terraform-provider-b_c_SHA256SUMS.sig",
				SigningKeys: core.SigningKeys{
					GPGPublicKeys: []core.GPGPublicKey{
						{
							KeyID:      "51852D87348FFC4C",
							ASCIIArmor: "-----BEGIN PGP PUBLIC KEY BLOCK-----\nVersion: GnuPG v1\n...",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ls := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			p, err := ls.GetProvider(
				context.Background(),
				tc.args.namespace,
				tc.args.name,
				tc.args.version,
				tc.args.os_,
				tc.args.arch,
			)

			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, p)
			}
		})
	}
}

func TestLocalStorage_ListProviderVersions(t *testing.T) {
	type args struct {
		namespace string
		name      string
	}

	cases := []struct {
		name      string
		args      args
		attrs     localStorageAttrsForTest
		expected  []core.ProviderVersion
		expectErr bool
	}{
		{
			name:      "empty namespace",
			expectErr: true,
		},
		{
			name: "empty name",
			args: args{
				namespace: "xx",
			},
			expectErr: true,
		},
		{
			name: "local storage dir not exist",
			args: args{
				namespace: "xx",
				name:      "yy",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: &mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{},
				},
			},
			expectErr: false,
			expected:  []core.ProviderVersion{},
		},
		{
			name: "no valid provider versions",
			args: args{
				namespace: "xx",
				name:      "yy",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: &mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{
						"zz/providers/xx/yy": &mockLocalFileInfo{
							isDir: true,
						},
					},
					dirs: map[string][]iofs.DirEntry{
						"zz/providers/xx/yy": {
							&mockDirEntry{
								name:  "invalid.zip",
								isDir: false,
							},
							&mockDirEntry{
								name:  "invalid-dir",
								isDir: true,
							},
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "have provider versions",
			args: args{
				namespace: "xx",
				name:      "yy",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: &mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{
						"zz/providers/xx/yy": &mockLocalFileInfo{
							isDir: true,
						},
					},
					dirs: map[string][]iofs.DirEntry{
						"zz/providers/xx/yy": {
							&mockDirEntry{
								name:  "invalid.zip",
								isDir: false,
							},
							&mockDirEntry{
								name:  "invalid-dir",
								isDir: true,
							},
							&mockDirEntry{
								name:  "terraform-provider-yy_0.0.1+xsdfasd_darwin_arm64.zip",
								isDir: false,
							},
						},
					},
				},
			},
			expected: []core.ProviderVersion{
				{
					Name:      "yy",
					Namespace: "",
					Version:   "0.0.1+xsdfasd",
					Platforms: []core.Platform{
						{
							OS:   "darwin",
							Arch: "arm64",
						},
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			got, err := storage.ListProviderVersions(context.Background(), tc.args.namespace, tc.args.name)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestLocalStorage_UploadProviderReleaseFiles(t *testing.T) {
	type args struct {
		namespace string
		name      string
		filename  string
		file      io.Reader
	}

	var dst = new(bytes.Buffer)
	cases := []struct {
		name      string
		args      args
		attrs     localStorageAttrsForTest
		expected  []byte
		expectErr bool
	}{
		{
			name: "empty namespace",
			args: args{
				namespace: "",
			},
			expectErr: true,
		},
		{
			name: "empty name",
			args: args{
				namespace: "xx",
				name:      "",
			},
			expectErr: true,
		},
		{
			name: "empty filename",
			args: args{
				namespace: "xx",
				name:      "yy",
				filename:  "",
			},
			expectErr: true,
		},
		{
			name: "nil file",
			args: args{
				namespace: "xx",
				name:      "yy",
				filename:  "terraform-provider-yy_0.0.1+xxx_darwin_arm64.zip",
				file:      nil,
			},
			expectErr: true,
		},
		{
			name: "upload success",
			args: args{
				namespace: "xx",
				name:      "yy",
				filename:  "terraform-provider-yy_0.0.1+xxx_darwin_arm64.zip",
				file:      bytes.NewBufferString("hi"),
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{
						"zz/providers/xx/yy": &mockLocalFileInfo{
							isDir: true,
						},
					},
					files: map[string]io.ReadWriteCloser{
						"zz/providers/xx/yy/terraform-provider-yy_0.0.1+xxx_darwin_arm64.zip": &mockFile{
							buf: dst,
						},
					},
				},
			},
			expected:  []byte("hi"),
			expectErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			err := storage.UploadProviderReleaseFiles(context.Background(), tc.args.namespace, tc.args.name, tc.args.filename, tc.args.file)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, dst.Bytes())
			}
		})
	}
}

func TestLocalStorage_SigningKeys(t *testing.T) {
	type args struct {
		namespace string
	}

	cases := []struct {
		name      string
		args      args
		attrs     localStorageAttrsForTest
		expected  core.SigningKeys
		expectErr bool
	}{
		{
			name:      "empty namespace",
			expectErr: true,
		},
		{
			name: "read key file failed",
			args: args{
				namespace: "xx",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: mockLocalFileSystem{
					files: map[string]io.ReadWriteCloser{},
				},
			},
			expectErr: true,
		},
		{
			name: "success",
			args: args{
				namespace: "xx",
			},
			attrs: localStorageAttrsForTest{
				storageDir: "zz",
				lfs: mockLocalFileSystem{
					files: map[string]io.ReadWriteCloser{
						"zz/providers/xx/signing-keys.json": &mockFile{
							buf: bytes.NewBufferString(`
                            {
                                "gpg_public_keys": [
                                    {
                                        "key_id": "aaaa",
                                        "ascii_armor": "bbbb"
                                    }
                                ]
                            }
                            `),
						},
					},
				},
			},
			expected: core.SigningKeys{
				GPGPublicKeys: []core.GPGPublicKey{
					{
						KeyID:      "aaaa",
						ASCIIArmor: "bbbb",
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			got, err := storage.SigningKeys(context.Background(), tc.args.namespace)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, *got)
			}
		})
	}
}

func TestLocalStorage_GetModule(t *testing.T) {
	type args struct {
		namespace string
		name      string
		provider  string
		version   string
	}

	cases := []struct {
		name      string
		args      args
		attrs     localStorageAttrsForTest
		expected  core.Module
		expectErr bool
	}{
		{
			name: "emtpy namespaces",
			args: args{
				namespace: "",
			},
			expectErr: true,
		},
		{
			name: "emtpy name",
			args: args{
				namespace: "xxx",
				name:      "",
			},
			expectErr: true,
		},
		{
			name: "emtpy provider",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				provider:  "",
			},
			expectErr: true,
		},
		{
			name: "emtpy version",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				provider:  "xxx",
				version:   "",
			},
			expectErr: true,
		},
		{
			name: "module not exist",
			args: args{
				namespace: "a",
				name:      "b",
				provider:  "c",
				version:   "0.0.1",
			},
			attrs: localStorageAttrsForTest{
				storageDir:          "x",
				moduleArchiveFormat: "zip",
				lfs: mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{},
				},
			},
			expectErr: true,
		},
		{
			name: "success",
			args: args{
				namespace: "a",
				name:      "b",
				provider:  "c",
				version:   "0.0.1",
			},
			attrs: localStorageAttrsForTest{
				storageDir:          "x",
				moduleArchiveFormat: "zip",
				serverEndpoint:      "localhost",
				lfs: mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{
						"x/modules/a/b/c/a-b-c-0.0.1.zip": &mockLocalFileInfo{
							isDir: false,
						},
					},
				},
			},
			expected: core.Module{
				Namespace:   "a",
				Name:        "b",
				Provider:    "c",
				Version:     "0.0.1",
				DownloadURL: "http://localhost/modules/a/b/c/a-b-c-0.0.1.zip",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			got, err := storage.GetModule(context.Background(), tc.args.namespace, tc.args.name, tc.args.provider, tc.args.version)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestLocalStorage_ListModuleVersions(t *testing.T) {
	type args struct {
		namespace string
		name      string
		provider  string
	}

	cases := []struct {
		name      string
		args      args
		attrs     localStorageAttrsForTest
		expected  []core.Module
		expectErr bool
	}{
		{
			name: "emtpy namespaces",
			args: args{
				namespace: "",
			},
			expectErr: true,
		},
		{
			name: "emtpy name",
			args: args{
				namespace: "xxx",
				name:      "",
			},
			expectErr: true,
		},
		{
			name: "emtpy provider",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				provider:  "",
			},
			expectErr: true,
		},
		{
			name: "success",
			args: args{
				namespace: "a",
				name:      "b",
				provider:  "c",
			},
			attrs: localStorageAttrsForTest{
				storageDir:          "x",
				moduleArchiveFormat: "tar.gz",
				serverEndpoint:      "localhost:8080",
				lfs: mockLocalFileSystem{
					dirs: map[string][]iofs.DirEntry{
						"x/modules/a/b/c": {
							&mockDirEntry{
								isDir: false,
								name:  "a-b-c-0.0.1.tar.gz",
							},
							&mockDirEntry{
								isDir: false,
								name:  "a-b-c-0.0.2+kjslx.tar.gz",
							},
						},
					},
				},
			},
			expected: []core.Module{
				{
					Namespace:   "a",
					Name:        "b",
					Provider:    "c",
					Version:     "0.0.1",
					DownloadURL: "http://localhost:8080/modules/a/b/c/a-b-c-0.0.1.tar.gz",
				},
				{
					Namespace:   "a",
					Name:        "b",
					Provider:    "c",
					Version:     "0.0.2+kjslx",
					DownloadURL: "http://localhost:8080/modules/a/b/c/a-b-c-0.0.2+kjslx.tar.gz",
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			got, err := storage.ListModuleVersions(context.Background(), tc.args.namespace, tc.args.name, tc.args.provider)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}

func TestLocalStorage_UploadModule(t *testing.T) {
	type args struct {
		namespace string
		name      string
		provider  string
		version   string
		body      io.Reader
	}

	var dst = new(bytes.Buffer)
	cases := []struct {
		name       string
		args       args
		attrs      localStorageAttrsForTest
		expected   core.Module
		expectFile []byte
		expectErr  bool
	}{
		{
			name: "emtpy namespaces",
			args: args{
				namespace: "",
			},
			expectErr: true,
		},
		{
			name: "emtpy name",
			args: args{
				namespace: "xxx",
				name:      "",
			},
			expectErr: true,
		},
		{
			name: "emtpy provider",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				provider:  "",
			},
			expectErr: true,
		},
		{
			name: "emtpy version",
			args: args{
				namespace: "xxx",
				name:      "xxx",
				provider:  "xxx",
				version:   "",
			},
			expectErr: true,
		},
		{
			name: "success",
			args: args{
				namespace: "a",
				name:      "b",
				provider:  "c",
				version:   "0.0.1",
				body:      bytes.NewBufferString("hi"),
			},
			attrs: localStorageAttrsForTest{
				storageDir:          "x",
				moduleArchiveFormat: "tar.gz",
				serverEndpoint:      "localhost:8080",
				lfs: mockLocalFileSystem{
					fileStats: map[string]iofs.FileInfo{
						"x/modules/a/b/c": &mockLocalFileInfo{
							isDir: true,
						},
					},
					files: map[string]io.ReadWriteCloser{
						"x/modules/a/b/c/a-b-c-0.0.1.tar.gz": &mockFile{
							buf: dst,
						},
					},
				},
			},
			expectFile: []byte("hi"),
			expected: core.Module{
				Namespace:   "a",
				Name:        "b",
				Provider:    "c",
				Version:     "0.0.1",
				DownloadURL: "http://localhost:8080/modules/a/b/c/a-b-c-0.0.1.tar.gz",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewLocalStorage(tc.attrs.lfs, tc.attrs.server, tc.attrs.storageDir, tc.attrs.serverEndpoint, tc.attrs.moduleArchiveFormat)
			got, err := storage.UploadModule(context.Background(), tc.args.namespace, tc.args.name, tc.args.provider, tc.args.version, tc.args.body)
			assert.Equal(t, tc.expectErr, err != nil)
			if err == nil {
				assert.Equal(t, tc.expected, got)
				assert.Equal(t, tc.expectFile, dst.Bytes())
			}
		})
	}
}
