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
// module set versioning file to verify that all modules have been correctly
// included in sets. If no versioning is specified, the default versioning
// file path will be used: (RepoRoot)/versions.yaml.
//
// This script must be called before running the pre-release and tagging
// scripts which update versions based on the versioning file.

package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/spf13/viper"

	"go.opentelemetry.io/otel/internal/tools/common"
)

const (
	defaultVersionsConfigName = "versions"
	defaultVersionsConfigType = "yaml"
)

type config struct {
	versioningFile	string
}

func validateConfig(cfg config) (config, error) {
	if cfg.versioningFile == "" {
		repoRoot, err := common.FindRepoRoot()
		if err != nil {
			return config{}, fmt.Errorf("Could not find repo root: %v", err)
		}
		cfg.versioningFile = filepath.Join(repoRoot,
			fmt.Sprintf("%v.%v", defaultVersionsConfigName, defaultVersionsConfigType))
	}
	return cfg, nil
}

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

// buildModuleSetsMap creates a versionConfig struct holding all module sets.
func buildModuleSetsMap(versioningFilename string) (moduleSetMap, error) {
	viper.AddConfigPath(filepath.Dir(versioningFilename))
	fileExt := filepath.Ext(versioningFilename)
	fileBaseWithoutExt := strings.TrimSuffix(filepath.Base(versioningFilename), fileExt)
	viper.SetConfigName(fileBaseWithoutExt)
	viper.SetConfigType(strings.TrimPrefix(fileExt, "."))

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
	// Plain log output, no timestamps.
	log.SetFlags(0)

	cfg := config{}

	flag.StringVarP(&cfg.versioningFile, "versioning-file", "v", "",
		"Path to versioning file that contains definitions of all module sets. " +
			fmt.Sprintf("If unspecified will default to (RepoRoot)/%v.%v",
				defaultVersionsConfigName, defaultVersionsConfigType,),
	)
	flag.Parse()

	cfg, err := validateConfig(cfg)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(-1)
	}

	repoRoot, err := common.FindRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: %v", err)
	}

	modSetMap, err := buildModuleSetsMap(cfg.versioningFile)
	if err != nil {
		log.Fatalf("unable to build module sets map: %v", err)
	}

	modInfoMap, err := buildModuleMap(modSetMap)
	if err != nil {
		log.Fatalf("unable to build module info map: %v", err)
	}

	modPathMap, err := buildModulePathMap(repoRoot)
	if err != nil {
		log.Fatalf("unable to build module path map: %v", err)
	}

	if err = verifyAllModulesInSet(modPathMap, modInfoMap); err != nil {
		log.Fatalf("verifyAllModulesInSet failed: %v", err)
	}

	if err = verifyVersions(modSetMap); err != nil {
		log.Fatalf("verifyVersions failed: %v", err)
	}

	fmt.Println("PASS: Module sets successfully verified.")
}