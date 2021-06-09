// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package common provides helper functions used in scripts within the
// internal/tools module.
package common

import (
	"errors"
	"fmt"
	"github.com/spf13/viper"
	"golang.org/x/mod/modfile"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)


// VersionConfig is needed to parse the versions.yaml file with viper.
type VersionConfig struct {
	ModuleSets ModuleSetMap `mapstructure:"moduleSets"`
}

// ModuleSetMap maps the name of a module set to a ModuleSet struct.
type ModuleSetMap map[string]ModuleSet

// ModuleSet holds the version that the specified modules within the set will have.
type ModuleSet struct {
	Version	string			`mapstructure:"version"`
	Modules	[]ModulePath	`mapstructure:"modules"`
}

// ModulePath holds the module import path, such as "go.opentelemetry.io/otel".
type ModulePath string

// ModuleInfoMap is a mapping from a module's import path to its ModuleInfo struct.
type ModuleInfoMap map[ModulePath]ModuleInfo

// ModuleInfo is a reverse of the ModuleSetMap, to allow for quick lookup from module
// path to its set and version.
type ModuleInfo struct {
	ModuleSetName	string
	Version 		string
}

// ModuleFilePath holds the file path to the go.mod file within the repo,
// including the base file name ("go.mod").
type ModuleFilePath string

// ModulePathMap is a mapping from a module's import path to its file path.
type ModulePathMap map[ModulePath]ModuleFilePath


// BuildModulePathMap creates a map with module paths as keys and go.mod file paths as values.
func BuildModulePathMap(root string) (ModulePathMap, error) {
	// TODO: handle contrib repo
	modPathMap := make(ModulePathMap)

	findGoMod := func(fPath string, info fs.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("Warning: file could not be read during filepath.Walk(): %v", err)
			return nil
		}
		if filepath.Base(fPath) == "go.mod" {
			// read go.mod file into mod []byte
			mod, err := ioutil.ReadFile(fPath)
			if err != nil {
				return err
			}

			// read path of module from go.mod file
			mPath := modfile.ModulePath(mod)

			// convert mPath, fPath string to modulePath and moduleFilePath
			modPath := ModulePath(mPath)
			modFilePath := ModuleFilePath(fPath)

			modPathMap[modPath] = modFilePath
		}
		return nil
	}

	if err := filepath.Walk(string(root), findGoMod); err != nil {
		return nil, err
	}

	return modPathMap, nil
}

// BuildModuleSetsMap creates a versionConfig struct holding all module sets.
func BuildModuleSetsMap(versioningFilename string) (ModuleSetMap, error) {
	viper.AddConfigPath(filepath.Dir(versioningFilename))
	fileExt := filepath.Ext(versioningFilename)
	fileBaseWithoutExt := strings.TrimSuffix(filepath.Base(versioningFilename), fileExt)
	viper.SetConfigName(fileBaseWithoutExt)
	viper.SetConfigType(strings.TrimPrefix(fileExt, "."))

	var versionCfg VersionConfig

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("Error reading versionsConfig file: %s", err)
	}

	if err := viper.Unmarshal(&versionCfg); err != nil {
		return nil, fmt.Errorf("Unable to decode versionsConfig: %s", err)
	}

	return versionCfg.ModuleSets, nil
}

// BuildModuleMap creates a map with module paths as keys and their moduleInfo as values
// by creating and "reversing" a ModuleSetsMap.
func BuildModuleMap(versioningFilename string) (ModuleInfoMap, error) {
	modSetMap, err := BuildModuleSetsMap(versioningFilename)
	if err != nil {
		return ModuleInfoMap{}, err
	}

	modMap := make(ModuleInfoMap)
	var modPath ModulePath

	for setName, moduleSet := range modSetMap {
		for _, modPath = range moduleSet.Modules {
			// Check if module has already been added to the map
			if _, exists := modMap[modPath]; exists {
				return nil, fmt.Errorf("Module %v exists more than once. Exists in sets %v and %v.",
					modPath, modMap[modPath].ModuleSetName, setName)
			}
			modMap[modPath] = ModuleInfo{setName, moduleSet.Version}
		}
	}

	return modMap, nil
}

func FindRepoRoot() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := start
	for {
		_, err := os.Stat(filepath.Join(dir, ".git"))
		if errors.Is(err, os.ErrNotExist) {
			dir = filepath.Dir(dir)
			// From https://golang.org/pkg/path/filepath/#Dir:
			// The returned path does not end in a separator unless it is the root directory.
			if strings.HasSuffix(dir, string(filepath.Separator)) {
				return "", fmt.Errorf("unable to find git repository enclosing working dir %s", start)
			}
			continue
		}

		if err != nil {
			return "", err
		}

		return dir, nil
	}
}
