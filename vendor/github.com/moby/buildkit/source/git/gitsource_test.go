package git

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/snapshot/naive"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepeatedFetch(t *testing.T) {
	testRepeatedFetch(t, false)
}
func TestRepeatedFetchKeepGitDir(t *testing.T) {
	testRepeatedFetch(t, true)
}

func testRepeatedFetch(t *testing.T, keepGitDir bool) {
	ctx := namespaces.WithNamespace(context.Background(), "buildkit-test")

	tmpdir, err := ioutil.TempDir("", "buildkit-state")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	gs := setupGitSource(t, tmpdir)

	repodir, err := ioutil.TempDir("", "buildkit-gitsource")
	require.NoError(t, err)
	defer os.RemoveAll(repodir)

	setupGitRepo(t, repodir)

	id := &source.GitIdentifier{Remote: repodir, KeepGitDir: keepGitDir}

	g, err := gs.Resolve(ctx, id)
	require.NoError(t, err)

	key1, err := g.CacheKey(ctx)
	require.NoError(t, err)

	require.Equal(t, 40, len(key1))

	ref1, err := g.Snapshot(ctx)
	require.NoError(t, err)
	defer ref1.Release(context.TODO())

	mount, err := ref1.Mount(ctx)
	require.NoError(t, err)

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	require.NoError(t, err)
	defer lm.Unmount()

	dt, err := ioutil.ReadFile(filepath.Join(dir, "def"))
	require.NoError(t, err)

	require.Equal(t, "bar\n", string(dt))

	_, err = os.Lstat(filepath.Join(dir, "ghi"))
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))

	// second fetch returns same dir
	id = &source.GitIdentifier{Remote: repodir, Ref: "master", KeepGitDir: keepGitDir}

	g, err = gs.Resolve(ctx, id)
	require.NoError(t, err)

	key2, err := g.CacheKey(ctx)
	require.NoError(t, err)

	require.Equal(t, key1, key2)

	ref2, err := g.Snapshot(ctx)
	require.NoError(t, err)
	defer ref2.Release(context.TODO())

	require.Equal(t, ref1.ID(), ref2.ID())

	id = &source.GitIdentifier{Remote: repodir, Ref: "feature", KeepGitDir: keepGitDir}

	g, err = gs.Resolve(ctx, id)
	require.NoError(t, err)

	key3, err := g.CacheKey(ctx)
	require.NoError(t, err)
	require.NotEqual(t, key1, key3)

	ref3, err := g.Snapshot(ctx)
	require.NoError(t, err)
	defer ref3.Release(context.TODO())

	mount, err = ref3.Mount(ctx)
	require.NoError(t, err)

	lm = snapshot.LocalMounter(mount)
	dir, err = lm.Mount()
	require.NoError(t, err)
	defer lm.Unmount()

	dt, err = ioutil.ReadFile(filepath.Join(dir, "ghi"))
	require.NoError(t, err)

	require.Equal(t, "baz\n", string(dt))
}

func TestFetchBySHA(t *testing.T) {
	testFetchBySHA(t, false)
}
func TestFetchBySHAKeepGitDir(t *testing.T) {
	testFetchBySHA(t, true)
}

func testFetchBySHA(t *testing.T, keepGitDir bool) {
	ctx := namespaces.WithNamespace(context.Background(), "buildkit-test")

	tmpdir, err := ioutil.TempDir("", "buildkit-state")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	gs := setupGitSource(t, tmpdir)

	repodir, err := ioutil.TempDir("", "buildkit-gitsource")
	require.NoError(t, err)
	defer os.RemoveAll(repodir)

	setupGitRepo(t, repodir)

	cmd := exec.Command("git", "rev-parse", "feature")
	cmd.Dir = repodir

	out, err := cmd.Output()
	require.NoError(t, err)

	sha := strings.TrimSpace(string(out))
	require.Equal(t, 40, len(sha))

	id := &source.GitIdentifier{Remote: repodir, Ref: sha, KeepGitDir: keepGitDir}

	g, err := gs.Resolve(ctx, id)
	require.NoError(t, err)

	key1, err := g.CacheKey(ctx)
	require.NoError(t, err)

	require.Equal(t, 40, len(key1))

	ref1, err := g.Snapshot(ctx)
	require.NoError(t, err)
	defer ref1.Release(context.TODO())

	mount, err := ref1.Mount(ctx)
	require.NoError(t, err)

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	require.NoError(t, err)
	defer lm.Unmount()

	dt, err := ioutil.ReadFile(filepath.Join(dir, "ghi"))
	require.NoError(t, err)

	require.Equal(t, "baz\n", string(dt))
}

func TestMultipleRepos(t *testing.T) {
	testMultipleRepos(t, false)
}

func TestMultipleReposKeepGitDir(t *testing.T) {
	testMultipleRepos(t, true)
}

func testMultipleRepos(t *testing.T, keepGitDir bool) {
	ctx := namespaces.WithNamespace(context.Background(), "buildkit-test")

	tmpdir, err := ioutil.TempDir("", "buildkit-state")
	require.NoError(t, err)
	defer os.RemoveAll(tmpdir)

	gs := setupGitSource(t, tmpdir)

	repodir, err := ioutil.TempDir("", "buildkit-gitsource")
	require.NoError(t, err)
	defer os.RemoveAll(repodir)

	setupGitRepo(t, repodir)

	repodir2, err := ioutil.TempDir("", "buildkit-gitsource")
	require.NoError(t, err)
	defer os.RemoveAll(repodir2)

	runShell(t, repodir2,
		"git init",
		"git config --local user.email test",
		"git config --local user.name test",
		"echo xyz > xyz",
		"git add xyz",
		"git commit -m initial",
	)

	id := &source.GitIdentifier{Remote: repodir, KeepGitDir: keepGitDir}
	id2 := &source.GitIdentifier{Remote: repodir2, KeepGitDir: keepGitDir}

	g, err := gs.Resolve(ctx, id)
	require.NoError(t, err)

	g2, err := gs.Resolve(ctx, id2)
	require.NoError(t, err)

	key1, err := g.CacheKey(ctx)
	require.NoError(t, err)
	require.Equal(t, 40, len(key1))

	key2, err := g2.CacheKey(ctx)
	require.NoError(t, err)
	require.Equal(t, 40, len(key2))

	require.NotEqual(t, key1, key2)

	ref1, err := g.Snapshot(ctx)
	require.NoError(t, err)
	defer ref1.Release(context.TODO())

	mount, err := ref1.Mount(ctx)
	require.NoError(t, err)

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	require.NoError(t, err)
	defer lm.Unmount()

	ref2, err := g2.Snapshot(ctx)
	require.NoError(t, err)
	defer ref2.Release(context.TODO())

	mount, err = ref2.Mount(ctx)
	require.NoError(t, err)

	lm = snapshot.LocalMounter(mount)
	dir2, err := lm.Mount()
	require.NoError(t, err)
	defer lm.Unmount()

	dt, err := ioutil.ReadFile(filepath.Join(dir, "def"))
	require.NoError(t, err)

	require.Equal(t, "bar\n", string(dt))

	dt, err = ioutil.ReadFile(filepath.Join(dir2, "xyz"))
	require.NoError(t, err)

	require.Equal(t, "xyz\n", string(dt))
}

func setupGitSource(t *testing.T, tmpdir string) source.Source {
	snapshotter, err := naive.NewSnapshotter(filepath.Join(tmpdir, "snapshots"))
	assert.NoError(t, err)

	md, err := metadata.NewStore(filepath.Join(tmpdir, "metadata.db"))
	assert.NoError(t, err)

	cm, err := cache.NewManager(cache.ManagerOpt{
		Snapshotter:   snapshotter,
		MetadataStore: md,
	})
	assert.NoError(t, err)

	repodir, err := ioutil.TempDir("", "buildkit-gitsource")
	require.NoError(t, err)
	defer os.RemoveAll(repodir)

	gs, err := NewSource(Opt{
		CacheAccessor: cm,
		MetadataStore: md,
	})
	require.NoError(t, err)

	return gs
}

func setupGitRepo(t *testing.T, dir string) {
	runShell(t, dir,
		"git init",
		"git config --local user.email test",
		"git config --local user.name test",
		"echo foo > abc",
		"git add abc",
		"git commit -m initial",
		"echo bar > def",
		"git add def",
		"git commit -m second",
		"git checkout -B feature",
		"echo baz > ghi",
		"git add ghi",
		"git commit -m feature",
	)
}

func runShell(t *testing.T, dir string, cmds ...string) {
	for _, args := range cmds {
		cmd := exec.Command("sh", "-c", args)
		cmd.Dir = dir
		dt, err := cmd.CombinedOutput()
		require.NoError(t, err, "command %v returned %s", args, dt)
	}
}
