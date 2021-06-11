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
	"go.opentelemetry.io/otel/internal/tools"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultVersionsConfigName = "versions"
	defaultVersionsConfigType = "yaml"
)

type config struct {
	versioningFile string
}

func validateConfig(cfg config) (config, error) {
	if cfg.versioningFile == "" {
		repoRoot, err := tools.FindRepoRoot()
		if err != nil {
			return config{}, fmt.Errorf("Could not find repo root: %v", err)
		}
		cfg.versioningFile = filepath.Join(repoRoot,
			fmt.Sprintf("%v.%v", defaultVersionsConfigName, defaultVersionsConfigType))
	}
	return cfg, nil
}

// verifyAllModulesInSet checks that every module (as defined by a go.mod file) is contained in exactly one module set.
func verifyAllModulesInSet(modPathMap tools.ModulePathMap, modInfoMap tools.ModuleInfoMap) error {
	// Note: This could be simplified by doing a set comparison between the keys in modInfoMap
	// and the values of modulePathMap.
	for modPath, modFilePath := range modPathMap {
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

	fmt.Println("PASS: All modules exist in exactly one set.")

	return nil
}

// isStableVersion returns true if modSet.Version is stable (i.e. version major greater than
// or equal to v1), else false.
func isStableVersion(v string) bool {
	return semver.Compare(semver.Major(v), "v1") >= 0
}

// verifyVersions checks that module set versions conform to versioning semantics.
func verifyVersions(modSetMap tools.ModuleSetMap) error {
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
				return fmt.Errorf("Multiple module sets have the same major version (%v): "+
					"%v (version %v) and %v (version %v)",
					modSetVersionMajor,
					prevModSetName, prevModSet.Version,
					modSetName, modSet.Version,
				)
			}
			setMajorVersions[modSetVersionMajor] = modSetName
		}
	}

	fmt.Println("PASS: All module versions are valid, and no module sets have same non-zero major version.")

	return nil
}

// verifyDependencies checks that dependencies between modules conform to versioning semantics.
func verifyDependencies(modInfoMap tools.ModuleInfoMap, modPathMap tools.ModulePathMap) error {
	// Dependencies are defined by the require section of go.mod files.
	for modPath, modInfo := range modInfoMap {
		// check if the module is a stable
		if isStableVersion(modInfo.Version) {
			modFilePath := modPathMap[modPath]
			modData, err := ioutil.ReadFile(string(modFilePath))

			modFile, err := modfile.Parse("teststring", modData, nil)
			if err != nil {
				return err
			}

			// get dependencies as defined by the "requires" section
			requireDeps := modFile.Require

			for _, dep := range requireDeps {
				// check if dependency is an otel-go module (i.e. if it exists in the module versioning file)
				if depModInfo, exists := modInfoMap[tools.ModulePath(dep.Mod.Path)]; exists {
					// check if dependency is not stable
					if !isStableVersion(depModInfo.Version) {
						fmt.Printf(
							"WARNING: Stable module %v (%v) depends on unstable module %v (%v).\n",
							modPath, modInfoMap[modPath].Version,
							dep.Mod.Path, depModInfo.Version,
						)
					}
				}
			}
		}
	}

	fmt.Println("Finished checking all stable modules' dependencies.")

	return nil
}

func main() {
	// Plain log output, no timestamps.
	log.SetFlags(0)

	cfg := config{}

	flag.StringVarP(&cfg.versioningFile, "versioning-file", "v", "",
		"Path to versioning file that contains definitions of all module sets. "+
			fmt.Sprintf("If unspecified will default to (RepoRoot)/%v.%v",
				defaultVersionsConfigName, defaultVersionsConfigType),
	)
	flag.Parse()

	cfg, err := validateConfig(cfg)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(-1)
	}

	modSetMap, err := tools.BuildModuleSetsMap(cfg.versioningFile)
	if err != nil {
		log.Fatalf("unable to build module sets map: %v", err)
	}

	modInfoMap, err := tools.BuildModuleMap(cfg.versioningFile)
	if err != nil {
		log.Fatalf("unable to build module info map: %v", err)
	}

	coreRepoRoot, err := tools.FindRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: %v", err)
	}

	modPathMap, err := tools.BuildModulePathMap(cfg.versioningFile, coreRepoRoot)
	if err != nil {
		log.Fatalf("unable to build module path map: %v", err)
	}

	if err = verifyAllModulesInSet(modPathMap, modInfoMap); err != nil {
		log.Fatalf("verifyAllModulesInSet failed: %v", err)
	}

	if err = verifyVersions(modSetMap); err != nil {
		log.Fatalf("verifyVersions failed: %v", err)
	}

	if err = verifyDependencies(modInfoMap, modPathMap); err != nil {
		log.Fatalf("verifyDependencies failed: %v", err)
	}

	fmt.Println("PASS: Module sets successfully verified.")
}
