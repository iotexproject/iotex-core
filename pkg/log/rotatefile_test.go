package log

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFile(t *testing.T) {
	require := require.New(t)

	dir := makeTempDir("TestNewFile", t)
	defer os.RemoveAll(dir)
	l := &RotateFile{
		Filename: logFile(dir),
	}
	defer l.Close()
	b := []byte("boo!")
	n, err := l.Write(b)
	require.NoError(err)
	require.Equal(len(b), n)
	existsWithContent(logFile(dir), b, t)
}

func makeTempDir(name string, t testing.TB) string {
	require := require.New(t)
	dir := filepath.Join(os.TempDir(), name)
	require.NoError(os.Mkdir(dir, 0700))
	return dir
}

// logFile returns the log file name in the given directory for the current fake
// time.
func logFile(dir string) string {
	return filepath.Join(dir, "foobar.log")
}

// existsWithContent checks that the given file exists and has the correct content.
func existsWithContent(path string, content []byte, t testing.TB) {
	require := require.New(t)
	info, err := os.Stat(path)
	require.Nil(err)
	require.Equal(int64(len(content)), info.Size())

	b, err := ioutil.ReadFile(path)
	require.Nil(err)
	require.Equal(content, b)
}
