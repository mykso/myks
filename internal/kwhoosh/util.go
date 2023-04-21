package kwhoosh

import (
	cmdtpl "github.com/vmware-tanzu/carvel-ytt/pkg/cmd/template"
	"github.com/vmware-tanzu/carvel-ytt/pkg/cmd/ui"
	"github.com/vmware-tanzu/carvel-ytt/pkg/files"
)

// Process files from the given paths using ytt
func YttFiles(paths []string) (string, error) {
	filesToProcess, err := files.NewSortedFilesFromPaths(paths, files.SymlinkAllowOpts{})
	if err != nil {
		return "", err
	}

	ui := ui.NewTTY(false)
	opts := cmdtpl.NewOptions()

	out := opts.RunWithFiles(cmdtpl.Input{Files: filesToProcess}, ui)

	if out.Err != nil {
		return "", out.Err
	}

	// FIXME: Use library function to output results
	strOut := ""

	for _, file := range out.Files {
		strOut += string(file.Bytes())
	}

	return strOut, nil
}
