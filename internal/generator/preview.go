package generator

import (
	"path/filepath"

	"github.com/theaiteam-dev/commandspec/internal/model"
)

// Preview returns the list of file paths that Generate would create for the
// given spec and CLI name, without writing anything to disk. The returned
// paths are relative to the output directory (i.e. the same relative paths
// that Generate passes to writeFile).
func Preview(spec *model.APISpec, name string) ([]string, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	var files []string

	collect := func(path string) {
		files = append(files, path)
	}

	collect("main.go")
	collect("go.mod")
	collect(filepath.Join("cmd", "root.go"))
	collect(filepath.Join("cmd", "completion.go"))

	reservedCmdNames := map[string]bool{"root": true, "completion": true}
	for _, resource := range spec.Resources {
		if reservedCmdNames[resource.Name] {
			continue
		}
		collect(filepath.Join("cmd", resource.Name+".go"))
		for _, cmd := range resource.Commands {
			filename := resource.Name + "_" + cmd.Name + ".go"
			collect(filepath.Join("cmd", filename))
		}
	}

	collect(filepath.Join("internal", "client", "client.go"))
	collect(filepath.Join("internal", "client", "pagination.go"))
	collect(filepath.Join("internal", "config.go"))
	collect(filepath.Join("internal", "output", "output.go"))
	collect(filepath.Join("internal", "errors.go"))
	collect(filepath.Join("internal", "validate", "validate.go"))

	return files, nil
}
