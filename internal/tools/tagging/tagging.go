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

// This prerelease script creates a new branch pre_release_<module set name>_<new tag> that
// will contain all release changes, including updated version numbers.
// This is to be used after the verify_release script has successfully
// verified that the versioning of module sets is valid.

package main

import (
	"fmt"
	flag "github.com/spf13/pflag"
	"go.opentelemetry.io/otel/internal/tools"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultVersionsConfigName = "versions"
	defaultVersionsConfigType = "yaml"
)

type config struct {
	VersioningFile 	string
	ModuleSet      	string
	CommitHash		string
}

func validateConfig(cfg config) (config, error) {
	if cfg.VersioningFile == "" {
		repoRoot, err := tools.FindRepoRoot()
		if err != nil {
			return config{}, fmt.Errorf("no versioning file was given, and could not automatically find repo root: %v", err)
		}
		cfg.VersioningFile = filepath.Join(repoRoot,
			fmt.Sprintf("%v.%v", defaultVersionsConfigName, defaultVersionsConfigType))

		fmt.Printf("Using versioning file found at %v\n", cfg.VersioningFile)
	}

	if cfg.ModuleSet == "" {
		return config{}, fmt.Errorf("required argument module-set was empty")
	}

	if cfg.CommitHash == "" {
		return config{}, fmt.Errorf("required argument commit-hash was empty")
	}

	cmd := exec.Command("git", "rev-parse", "--quiet", "--verify", cfg.CommitHash)
	// output stores the complete SHA1 of the commit hash
	output, err := cmd.Output()
	if err != nil {
		return config{}, fmt.Errorf("could not retrieve commit hash %v: %v", cfg.CommitHash, err)
	}

	SHA := strings.TrimSpace(string((output)))

	cmd = exec.Command("git", "merge-base", SHA, "HEAD")
	// output should match SHA
	output, err = cmd.Output()
	if err != nil {
		return config{}, fmt.Errorf("command 'git merge-base %v HEAD' failed: %v", SHA, err)
	}
	if strings.TrimSpace(string(output)) != SHA {
		return config{}, fmt.Errorf("commit %v (complete SHA: %v) not found on this branch", cfg.CommitHash, SHA)
	}

	cfg.CommitHash = SHA

	return cfg, nil
}


func tagAllModules(newVersion string, modTagNames []tools.ModuleTagName, commitHash string) error {
	for _, modTagName := range modTagNames {
		var newFullTag string
		if modTagName == tools.REPOROOTTAG {
			newFullTag = newVersion
		} else {
			newFullTag = string(modTagName) + "/" + newVersion
		}
		fmt.Printf("git tag -a %v -s -m \"Version %v\" %v\n", newFullTag, newFullTag, commitHash)
		cmd := exec.Command("git", "tag", "-a", newFullTag, "-s", "-m", "Version " + newFullTag, commitHash)
		output, err := cmd.Output()
		if err != nil {
			fmt.Printf("Output of git tag command: %v\n", string(output))
			return fmt.Errorf("git tag failed: %v", err)
		}

		fmt.Println("Successfully tagged ", newFullTag)
	}

	return nil
}

// TODO: figure out if it is possible to print changes between release versions for specific modules
func printChanges(newVersion string) error {
	return nil
}

func main() {
	// Plain log output, no timestamps.
	log.SetFlags(0)

	cfg := config{}

	flag.StringVarP(&cfg.VersioningFile, "versioning-file", "v", "",
		"Path to versioning file that contains definitions of all module sets. " +
			fmt.Sprintf("If unspecified will default to (RepoRoot)/%v.%v",
				defaultVersionsConfigName, defaultVersionsConfigType,),
	)
	flag.StringVarP(&cfg.ModuleSet, "module-set", "m", "",
		"Name of module set whose version is being changed. Must be listed in the module set versioning YAML.",
	)
	flag.StringVarP(&cfg.CommitHash, "commit-hash", "c", "",
		"Git commit hash to tag.",
	)
	flag.Parse()

	cfg, err := validateConfig(cfg)
	if err != nil {
		fmt.Println(err)
		flag.Usage()
		os.Exit(-1)
	}

	fmt.Println(cfg)

	coreRepoRoot, err := tools.FindRepoRoot()
	if err != nil {
		log.Fatalf("unable to find repo root: %v", err)
	}

	fmt.Println("Changing to root directory...")
	os.Chdir(coreRepoRoot)

	// get new version and mod tags to update
	newVersion, _, newModTagNames, err := tools.VersionsAndModsToUpdate(cfg.VersioningFile, cfg.ModuleSet, coreRepoRoot)
	if err != nil {
		log.Fatalf("unable to get modules to update: %v", err)
	}

	if err := tagAllModules(newVersion, newModTagNames, cfg.CommitHash); err != nil {
		log.Fatalf("unable to tag modules: %v", err)
	}

	if err := printChanges; err != nil {
		log.Fatalf("unable to print changes: %v", err)
	}

}
