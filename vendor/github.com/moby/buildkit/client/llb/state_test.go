package llb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStateMeta(t *testing.T) {
	s := Source("foo")
	s = s.AddEnv("BAR", "abc").Dir("/foo/bar")

	v, ok := s.GetEnv("BAR")
	assert.True(t, ok)
	assert.Equal(t, "abc", v)

	assert.Equal(t, "/foo/bar", s.GetDir())

	s2 := Source("foo2")
	s2 = s2.AddEnv("BAZ", "def").Reset(s)

	_, ok = s2.GetEnv("BAZ")
	assert.False(t, ok)

	v, ok = s2.GetEnv("BAR")
	assert.True(t, ok)
	assert.Equal(t, "abc", v)
}
