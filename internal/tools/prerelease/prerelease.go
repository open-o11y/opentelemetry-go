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

// TODO: describe this script.

package main
//
//import "io/ioutil"
//
//// verifyDependencies checks that dependencies between modules conform to versioning semantics.
//func verifyDependencies(modInfoMap moduleInfoMap, modPathMap modulePathMap) error {
//	// Dependencies are defined by the require section of go.mod files.
//	for modPath, modInfo := range modInfoMap {
//		if isStableVersion(modInfo.Version) {
//			modFilePath := modPathMap[modPath]
//			modData, err := ioutil.ReadFile(string(modFilePath))
//
//			modFile, err := modfile.Parse(string(modFilePath), modData, nil)
//			if err != nil {
//				return err
//			}
//
//			requireDeps := modFile.Require
//			for _, dep := range requireDeps {
//
//				fmt.Println(dep.Mod)
//			}
//		}
//	}
//
//	// No dependencies on any experimental module exist in stable modules, based on version numbers.
//	// TODO: Dependencies within a set of modules must be the most current version. (?)
//	// No module should list a dependent module version that does not yet exist.
//	return nil
//}
//
//func main() {
//	if err = verifyDependencies(modInfoMap, modPathMap); err != nil {
//		panic(err)
//	}
//
//	fmt.Println("PASS: Module sets successfully verified.")
//}