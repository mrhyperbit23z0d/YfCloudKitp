package build

import "github.com/openshift/source-to-image/pkg/api"

// Builder is the interface that provides basic methods all implementation
// should have.
// Build method executes the build based on Request and returns the Result.
type Builder interface {
	Build(*api.Request) (*api.Result, error)
}

// Preparer provides the Prepare method for builders that need to prepare source
// code before it gets passed to the build.
type Preparer interface {
	Prepare(*api.Request) error
}

// Cleaner provides the Cleanup method for builders that need to cleanup
// temporary containers or directories after build execution finish.
type Cleaner interface {
	Cleanup(*api.Request)
}

// IncrementalBuilder provides methods that is used for builders that implements
// the 'incremental' build workflow.
// The Exists method checks if the artifacts from the previous build exists
// and if they can be used in the current build.
// The Save method stores the artifacts for the next build.
type IncrementalBuilder interface {
	Exists(*api.Request) bool
	Save(*api.Request) error
}

// ScriptsHandler provides an interface for executing the scripts
type ScriptsHandler interface {
	Execute(string, *api.Request) error
}

// Downloader provides methods for downloading the application source code
type Downloader interface {
	Download(*api.Request) error
}

// SourceHandler is a wrapper for STI strategy Downloader and Preparer which
// allows to use Download and Prepare functions from the STI strategy.
type SourceHandler interface {
	Downloader
	Preparer
}

// LayeredDockerBuilder represents a minimal Docker builder interface that is
// used to execute the layered Docker build with the application source.
type LayeredDockerBuilder interface {
	Builder
}
