package rsync

import (
	"io/ioutil"
	"os"
	"path"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"replicate.ai/cli/pkg/remote"
)

func TestUpload(t *testing.T) {
	mockRemote, err := remote.NewMockRemote()
	require.NoError(t, err)
	defer mockRemote.Kill()

	localDir, err := ioutil.TempDir("/tmp", "replicate-test-")
	require.NoError(t, err)
	defer os.RemoveAll(localDir)

	require.NoError(t, ioutil.WriteFile(path.Join(localDir, "foo.txt"), []byte("hello foo"), 0644))
	require.NoError(t, os.Mkdir(path.Join(localDir, "bar"), 0755))
	require.NoError(t, ioutil.WriteFile(path.Join(localDir, "bar/baz.txt"), []byte("hello baz"), 0644))

	remoteDir := "/tmp/upload"

	options := &remote.Options{
		Host:        "localhost",
		Port:        mockRemote.Port,
		Username:    "root",
		PrivateKeys: []string{mockRemote.PrivateKeyPath},
	}

	err = Upload(localDir, options, remoteDir)
	require.NoError(t, err)

	client, err := remote.NewClient(options)
	require.NoError(t, err)
	files, err := client.SFTP().ReadDir("/tmp/upload")
	require.NoError(t, err)

	names := []string{}
	for _, file := range files {
		names = append(names, file.Name())
	}
	sort.Strings(names)
	expectedNames := []string{"bar", "foo.txt"}

	require.Equal(t, expectedNames, names)

	contents, err := client.Command("cat", "/tmp/upload/foo.txt").Output()
	require.NoError(t, err)
	require.Equal(t, "hello foo", string(contents))

	contents, err = client.Command("cat", "/tmp/upload/bar/baz.txt").Output()
	require.NoError(t, err)
	require.Equal(t, "hello baz", string(contents))
}