package git

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/locker"
	"github.com/boltdb/bolt"
	"github.com/moby/buildkit/cache"
	"github.com/moby/buildkit/cache/metadata"
	"github.com/moby/buildkit/identity"
	"github.com/moby/buildkit/snapshot"
	"github.com/moby/buildkit/source"
	"github.com/moby/buildkit/util/progress/logs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var validHex = regexp.MustCompile(`^[a-f0-9]{40}$`)

type Opt struct {
	CacheAccessor cache.Accessor
	MetadataStore *metadata.Store
}

type gitSource struct {
	md     *metadata.Store
	cache  cache.Accessor
	locker *locker.Locker
}

func NewSource(opt Opt) (source.Source, error) {
	gs := &gitSource{
		md:     opt.MetadataStore,
		cache:  opt.CacheAccessor,
		locker: locker.NewLocker(),
	}

	if err := exec.Command("git", "version").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to find git binary")
	}

	return gs, nil
}

func (gs *gitSource) ID() string {
	return source.GitScheme
}

// needs to be called with repo lock
func (gs *gitSource) mountRemote(ctx context.Context, remote string) (target string, release func(), retErr error) {
	remoteKey := "git-remote::" + remote

	sis, err := gs.md.Search(remoteKey)
	if err != nil {
		return "", nil, errors.Wrapf(err, "failed to search metadata for %s", remote)
	}

	var remoteRef cache.MutableRef
	for _, si := range sis {
		remoteRef, err = gs.cache.GetMutable(ctx, si.ID())
		if err != nil {
			if cache.IsLocked(err) {
				// should never really happen as no other function should access this metadata, but lets be graceful
				logrus.Warnf("mutable ref for %s  %s was locked: %v", remote, si.ID(), err)
				continue
			}
			return "", nil, errors.Wrapf(err, "failed to get mutable ref for %s", remote)
		}
		break
	}

	initializeRepo := false
	if remoteRef == nil {
		remoteRef, err = gs.cache.New(ctx, nil)
		if err != nil {
			return "", nil, errors.Wrapf(err, "failed to create new mutable for %s", remote)
		}
		initializeRepo = true
	}

	releaseRemoteRef := func() {
		s, err := remoteRef.Freeze() // TODO: remove this
		if err == nil {
			s.Release(context.TODO())
		}
	}

	defer func() {
		if retErr != nil && remoteRef != nil {
			releaseRemoteRef()
		}
	}()

	mount, err := remoteRef.Mount(ctx)
	if err != nil {
		return "", nil, err
	}

	lm := snapshot.LocalMounter(mount)
	dir, err := lm.Mount()
	if err != nil {
		return "", nil, err
	}

	defer func() {
		if retErr != nil {
			lm.Unmount()
		}
	}()

	if initializeRepo {
		if _, err := gitWithinDir(ctx, dir, "", "init", "--bare"); err != nil {
			return "", nil, errors.Wrapf(err, "failed to init repo at %s", dir)
		}

		if _, err := gitWithinDir(ctx, dir, "", "remote", "add", "origin", remote); err != nil {
			return "", nil, errors.Wrapf(err, "failed add origin repo at %s", dir)
		}

		// same new remote metadata
		si, _ := gs.md.Get(remoteRef.ID())
		v, err := metadata.NewValue(remoteKey)
		v.Index = remoteKey
		if err != nil {
			return "", nil, err
		}

		if err := si.Update(func(b *bolt.Bucket) error {
			return si.SetValue(b, "git-remote", *v)
		}); err != nil {
			return "", nil, err
		}
	}
	return dir, func() {
		lm.Unmount()
		releaseRemoteRef()
	}, nil
}

type gitSourceHandler struct {
	*gitSource
	src      source.GitIdentifier
	cacheKey string
}

func (gs *gitSource) Resolve(ctx context.Context, id source.Identifier) (source.SourceInstance, error) {
	gitIdentifier, ok := id.(*source.GitIdentifier)
	if !ok {
		return nil, errors.Errorf("invalid git identifier %v", id)
	}

	return &gitSourceHandler{
		src:       *gitIdentifier,
		gitSource: gs,
	}, nil
}

func (gs *gitSourceHandler) CacheKey(ctx context.Context) (string, error) {
	remote := gs.src.Remote
	ref := gs.src.Ref
	if ref == "" {
		ref = "master"
	}
	gs.locker.Lock(remote)
	defer gs.locker.Unlock(remote)

	if isCommitSHA(ref) {
		gs.cacheKey = ref
		return ref, nil
	}

	gitDir, unmountGitDir, err := gs.mountRemote(ctx, remote)
	if err != nil {
		return "", err
	}
	defer unmountGitDir()

	// TODO: should we assume that remote tag is immutable? add a timer?

	buf, err := gitWithinDir(ctx, gitDir, "", "ls-remote", "origin", ref)
	if err != nil {
		return "", errors.Wrapf(err, "failed to fetch remote %s", remote)
	}
	out := buf.String()
	idx := strings.Index(out, "\t")
	if idx == -1 {
		return "", errors.Errorf("failed to find commit SHA from output: %s", string(out))
	}

	sha := string(out[:idx])
	if !isCommitSHA(sha) {
		return "", errors.Errorf("invalid commit sha %q", sha)
	}
	gs.cacheKey = sha
	return sha, nil
}

func (gs *gitSourceHandler) Snapshot(ctx context.Context) (out cache.ImmutableRef, retErr error) {
	ref := gs.src.Ref
	if ref == "" {
		ref = "master"
	}

	cacheKey := gs.cacheKey
	if cacheKey == "" {
		var err error
		cacheKey, err = gs.CacheKey(ctx)
		if err != nil {
			return nil, err
		}
	}

	snapshotKey := "git-snapshot::" + cacheKey + ":" + gs.src.Subdir
	gs.locker.Lock(snapshotKey)
	defer gs.locker.Unlock(snapshotKey)

	sis, err := gs.md.Search(snapshotKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to search metadata for %s", snapshotKey)
	}
	if len(sis) > 0 {
		return gs.cache.Get(ctx, sis[0].ID())
	}

	gitDir, unmountGitDir, err := gs.mountRemote(ctx, gs.src.Remote)
	if err != nil {
		return nil, err
	}
	defer unmountGitDir()

	doFetch := true
	if isCommitSHA(ref) {
		// skip fetch if commit already exists
		if _, err := gitWithinDir(ctx, gitDir, "", "cat-file", "-e", ref+"^{commit}"); err == nil {
			doFetch = false
		}
	}

	if doFetch {
		args := []string{"fetch", "--recurse-submodules=yes"}
		if !isCommitSHA(ref) { // TODO: find a branch from ls-remote?
			args = append(args, "--depth=1", "--no-tags")
		} else {
			if _, err := os.Lstat(filepath.Join(gitDir, "shallow")); err == nil {
				args = append(args, "--unshallow")
			}
		}
		args = append(args, "origin")
		if !isCommitSHA(ref) {
			args = append(args, ref+":tags/"+ref)
			// local refs are needed so they would be advertised on next fetches
			// TODO: is there a better way to do this?
		}
		if _, err := gitWithinDir(ctx, gitDir, "", args...); err != nil {
			return nil, errors.Wrapf(err, "failed to fetch remote %s", gs.src.Remote)
		}
	}

	checkoutRef, err := gs.cache.New(ctx, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new mutable for %s", gs.src.Remote)
	}

	defer func() {
		if retErr != nil && checkoutRef != nil {
			s, err := checkoutRef.Freeze() // TODO: remove this
			if err != nil {
				s.Release(context.TODO())
			}
		}
	}()

	mount, err := checkoutRef.Mount(ctx)
	if err != nil {
		return nil, err
	}
	lm := snapshot.LocalMounter(mount)
	checkoutDir, err := lm.Mount()
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil && lm != nil {
			lm.Unmount()
		}
	}()

	if gs.src.KeepGitDir {
		_, err = gitWithinDir(ctx, checkoutDir, "", "init")
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDir, "", "remote", "add", "origin", gitDir)
		if err != nil {
			return nil, err
		}
		pullref := ref
		if isCommitSHA(ref) {
			pullref = "refs/buildkit/" + identity.NewID()
			_, err = gitWithinDir(ctx, gitDir, "", "update-ref", pullref, ref)
			if err != nil {
				return nil, err
			}
		}
		_, err = gitWithinDir(ctx, checkoutDir, "", "fetch", "--recurse-submodules=yes", "--depth=1", "origin", pullref)
		if err != nil {
			return nil, err
		}
		_, err = gitWithinDir(ctx, checkoutDir, checkoutDir, "checkout", "FETCH_HEAD")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", gs.src.Remote)
		}
	} else {
		_, err = gitWithinDir(ctx, gitDir, checkoutDir, "checkout", ref, "--", ".")
		if err != nil {
			return nil, errors.Wrapf(err, "failed to checkout remote %s", gs.src.Remote)
		}
	}
	lm.Unmount()
	lm = nil

	snap, err := checkoutRef.ReleaseAndCommit(ctx)
	if err != nil {
		return nil, err
	}
	checkoutRef = nil

	defer func() {
		if retErr != nil {
			snap.Release(context.TODO())
		}
	}()

	si, _ := gs.md.Get(snap.ID())
	v, err := metadata.NewValue(snapshotKey)
	v.Index = snapshotKey
	if err != nil {
		return nil, err
	}
	if err := si.Update(func(b *bolt.Bucket) error {
		return si.SetValue(b, "git-snapshot", *v)
	}); err != nil {
		return nil, err
	}

	return snap, nil
}

func isCommitSHA(str string) bool {
	return validHex.MatchString(str)
}

func gitWithinDir(ctx context.Context, gitDir, workDir string, args ...string) (*bytes.Buffer, error) {
	a := []string{"--git-dir", gitDir}
	if workDir != "" {
		a = append(a, "--work-tree", workDir)
	}
	return git(ctx, append(a, args...)...)
}

func git(ctx context.Context, args ...string) (*bytes.Buffer, error) {
	stdout, stderr := logs.NewLogStreams(ctx)
	defer stdout.Close()
	defer stderr.Close()
	cmd := exec.CommandContext(ctx, "git", args...)
	buf := bytes.NewBuffer(nil)
	cmd.Stdout = io.MultiWriter(stdout, buf)
	cmd.Stderr = stderr
	return buf, cmd.Run()
}
