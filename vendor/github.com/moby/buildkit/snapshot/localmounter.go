package snapshot

import (
	"io/ioutil"
	"os"
	"sync"

	"github.com/containerd/containerd/mount"
	"github.com/pkg/errors"
)

type Mounter interface {
	Mount() (string, error)
	Unmount() error
}

// LocalMounter is a helper for mounting to temporary path. In addition it can
// mount binds without privileges
func LocalMounter(m []mount.Mount) Mounter {
	return &localMounter{m: m}
}

type localMounter struct {
	mu     sync.Mutex
	m      []mount.Mount
	target string
}

func (lm *localMounter) Mount() (string, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if len(lm.m) == 1 && lm.m[0].Type == "bind" {
		return lm.m[0].Source, nil
	}

	dir, err := ioutil.TempDir("", "buildkit-mount")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp dir")
	}

	if err := mount.MountAll(lm.m, dir); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	lm.target = dir
	return dir, nil
}

func (lm *localMounter) Unmount() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.target != "" {
		if err := mount.Unmount(lm.target, 0); err != nil {
			return err
		}
		os.RemoveAll(lm.target)
		lm.target = ""
	}

	return nil
}
