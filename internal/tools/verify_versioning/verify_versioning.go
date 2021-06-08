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

// This verify_versioning script is called after manually editing the
// versions.yaml file to verify that all modules have been correctly
// included in sets.
//
// This script must be called before running the
// pre-release and tagging scripts which update versions based on
// versions.yaml.

package main

import (
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/spf13/viper"
)

const (
	versionsConfigName = "versions"
	versionsConfigType = "yaml"
)

// versionConfig is needed to parse the versions.yaml file with viper
type versionConfig struct {
	ModuleSets moduleSetMap `mapstructure:"moduleSets"`
}

type moduleSetMap map[string]moduleSet

type moduleSet struct {
	Version	string			`mapstructure:"version"`
	Modules	[]modulePath	`mapstructure:"modules"`
}

type modulePath string

type moduleInfo struct {
	ModuleSetName	string
	Version 		string
}

type moduleInfoMap map[modulePath]moduleInfo

// moduleFilePath includes the base file name ("go.mod").
type moduleFilePath string

type modulePathMap map[modulePath]moduleFilePath

func findRepoRoot() (string, error) {
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

// buildModuleSetsMap creates a versionConfig struct holding all module sets.
func buildModuleSetsMap(root string) (moduleSetMap, error) {
	viper.AddConfigPath(root)
	viper.SetConfigName(versionsConfigName)
	viper.SetConfigType(versionsConfigType)

	var versionCfg versionConfig

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("Error reading versionsConfig file: %s", err)
	}

	if err := viper.Unmarshal(&versionCfg); err != nil {
		return nil, fmt.Errorf("Unable to decode versionsConfig: %s", err)
	}

	return versionCfg.ModuleSets, nil
}

// buildModuleMap creates a map with module paths as keys and their moduleInfo as values.
func buildModuleMap(modSetMap moduleSetMap) (moduleInfoMap, error) {
	modMap := make(moduleInfoMap)
	var modPath modulePath

	for setName, moduleSet := range modSetMap {
		for _, modPath = range moduleSet.Modules {
			// Check if module has already been added to the map
			if _, exists := modMap[modPath]; exists {
				return nil, fmt.Errorf("Module %v exists more than once. Exists in sets %v and %v.",
					modPath, modMap[modPath].ModuleSetName, setName)
			}
			modMap[modPath] = moduleInfo{setName, moduleSet.Version}
		}
	}

	return modMap, nil
}

// buildModulePathMap creates a map with module paths as keys and go.mod file paths as values.
func buildModulePathMap(root string) (modulePathMap, error) {
	// TODO: handle contrib repo
	modPathMap := make(modulePathMap)

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
			modPath := modulePath(mPath)
			modFilePath := moduleFilePath(fPath)

			modPathMap[modPath] = modFilePath
		}
		return nil
	}

	if err := filepath.Walk(string(root), findGoMod); err != nil {
		return nil, err
	}

	return modPathMap, nil
}

// verifyAllModulesInSet checks that every module (as defined by a go.mod file) is contained in exactly one module set.
func verifyAllModulesInSet(modPathMap modulePathMap, modInfoMap moduleInfoMap) error {
	// Note: This could be simplified by doing a set comparison between the keys in modInfoMap
	// and the values of modulePathMap.
	for modPath, modFilePath := range(modPathMap) {
		if _, exists := modInfoMap[modPath]; !exists {
			return fmt.Errorf("Module %v (defined in %v) is not contained in any module set.",
				modPath, string(modFilePath),
			)
		}
	}

	for modPath, modInfo := range modInfoMap {
		if _, exists := modPathMap[modPath]; !exists {
			// TODO: handle contrib repo
			return fmt.Errorf("Module %v in module set %v does not exist in the core repo.",
				modPath, modInfo.ModuleSetName,
			)
		}
	}

	return nil
}

// verifyVersions checks that module set versions conform to versioning semantics.
func verifyVersions(modSetMap moduleSetMap) error {
	// setMajorVersions keeps track of all sets' major versions, used to check for multiple sets
	// with the same non-zero major version.
	setMajorVersions := make(map[string]string)

	for modSetName, modSet := range modSetMap {
		// Check that module set versions conform to semver semantics
		if !semver.IsValid(modSet.Version) {
			return fmt.Errorf("Module set %v has invalid version string: %v",
				modSetName, modSet.Version,
			)
		}

		if isStableVersion(modSet.Version) {
			// Check that no more than one module exists for any given non-zero major version
			modSetVersionMajor := semver.Major(modSet.Version)
			if prevModSetName, exists := setMajorVersions[modSetVersionMajor]; exists {
				prevModSet := modSetMap[prevModSetName]
				return fmt.Errorf("Multiple module sets have the same major version (%v): " +
					"%v (version %v) and %v (version %v)",
					modSetVersionMajor,
					prevModSetName, prevModSet.Version,
					modSetName, modSet.Version,
				)
			}
			setMajorVersions[modSetVersionMajor] = modSetName
		}
	}

	return nil
}

// isStableVersion returns true if modSet.Version is stable (i.e. version major greater than
// or equal to v1), else false.
func isStableVersion(v string) bool {
	return semver.Compare(semver.Major(v), "v1") >= 0
}

func main() {
	repoRoot, err := findRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: #{err}")
	}

	modSetMap, err := buildModuleSetsMap(repoRoot)
	if err != nil {
		log.Fatalf("unable to build module sets map: #{err}")
	}

	modInfoMap, err := buildModuleMap(modSetMap)
	if err != nil {
		log.Fatalf("unable to build module info map: #{err}")
	}

	modPathMap, err := buildModulePathMap(repoRoot)
	if err != nil {
		log.Fatalf("unable to build module path map: #{err}")
	}

	if err = verifyAllModulesInSet(modPathMap, modInfoMap); err != nil {
		log.Fatalf("verifyAllModulesInSet failed: #{err}")
	}

	if err = verifyVersions(modSetMap); err != nil {
		log.Fatalf("verifyVersions failed: #{err}")
	}

	fmt.Println("PASS: Module sets successfully verified.")
}