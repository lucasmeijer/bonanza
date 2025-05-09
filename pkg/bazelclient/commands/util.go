package commands

import (
	"github.com/buildbarn/bb-storage/pkg/filesystem/path"
	"github.com/buildbarn/bonanza/pkg/bazelclient/formatted"
	"github.com/buildbarn/bonanza/pkg/bazelclient/logging"
)

func ValidateInsideWorkspace(logger logging.Logger, commandName string, workspacePath path.Parser) {
	if workspacePath == nil {
		logger.Fatal(formatted.Textf("The %#v command is only supported from within a workspace (below a directory having a MODULE.bazel file)", commandName))
	}
}
