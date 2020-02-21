package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// MOduleRepo URL where modules are sources in terraform code
// MOduleJSONURL Where to get modules.json index from
var (
	ModuleRepo    = "https://tfmodules.deliveroo.net/"
	ModuleJSONURL = ModuleRepo + "modules.json"
	DEBUG         = false
)

type moduleInfo struct {
	ID        string
	Namespace string
	Provider  string
	Version   string
	Name      string
	Source    string
}

type modulesInfo struct {
	Modules []moduleInfo
}

type moduleIndex map[string]moduleInfo

// generic debug wrapper
func debug(msg string) {
	if DEBUG {
		fmt.Printf("DEBUG: %s", msg)
	}
}

// downloadJSON fetches a document from a URL and returns it a sa string
func downloadFile(url string) (moduleJSON []byte, err error) {

	debug(fmt.Sprintf("Getting index file from %s", url))
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("FAILED to download file from %s: %s\n", url, err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("FAILED to read response body: %s", err)
		return nil, err
	}
	return body, nil
}

// decodeJSON parses modules.json data into a modulesInfo struct
func decodeJSON(buf []byte) (m modulesInfo, err error) {
	var modulesJSON modulesInfo
	if err := json.Unmarshal(buf, &modulesJSON); err != nil {
		fmt.Printf("FAILED to decode JSON: %s", err)
	}
	return modulesJSON, err
}

// getTerraformFiles walks the given path and returns a list of *.tf files found
func getTerraformFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		m, _ := regexp.MatchString(`.*\.tf$`, path)
		if m == true {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// checkTerraformModules compare module version in a terraform file with latest known
func checkTerraformModules(path string, modules moduleIndex) (changes []string, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", path, err)
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	moduleSourcePtn := `^\s*source\s*=\s*"` + ModuleRepo + `([^/]+)/(.*)\.zip"\s*$`
	re := regexp.MustCompile(moduleSourcePtn)
	for n, line := range lines {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if checkModuleVersion(m[1], m[2], modules) {
			changes = append(changes, fmt.Sprintf("%s:%d `%s` version %s (latest %s)", path, n, m[1], m[2], modules[m[1]].Version))
		}
	}
	return changes, nil
}

// checkModuleVersion report if a module version in code is older than latest available
func checkModuleVersion(name, version string, modules moduleIndex) bool {

	latest := "n/a" // a dummy string that is higher than any version ;)
	if v, ok := modules[name]; !ok {
		latest = v.Version
	}
	debug(fmt.Sprintf("%s: %s vs %s\n", name, version, latest))
	return version < latest
}

// makeModuleInfoHash turns a modulesInfo array into a hash using the `name` field of individual moduleInfo structures
func makeModuleInfoHash(data []moduleInfo) (map[string]moduleInfo, error) {
	var modulesInfoHash = make(map[string]moduleInfo)
	for _, k := range data {
		modulesInfoHash[k.Name] = k
	}
	return modulesInfoHash, nil
}

func main() {

	var root string

	flag.BoolVar(&DEBUG, "debug", false, "Enable debug")
	flag.StringVar(&root, "root", "", "Root of local directory to scan")
	flag.Parse()

	if len(root) == 0 {
		fmt.Printf("path to scan not set")
		os.Exit(1)
	}

	if DEBUG {
		debug("Debug mode is on")
	}

	buf, err := downloadFile(ModuleJSONURL)
	if err != nil {
		os.Exit(1)
	}

	var modulesJSON modulesInfo
	modulesJSON, err = decodeJSON(buf)
	if err != nil {
		os.Exit(1)
	}

	var modules = make(moduleIndex)
	modules, err = makeModuleInfoHash(modulesJSON.Modules)

	files, err := getTerraformFiles(root)
	if err != nil {
		os.Exit(1)
	}
	var changes []string
	for _, file := range files {
		changes, err = checkTerraformModules(file, modules)

		for _, change := range changes {
			fmt.Printf("%s\n", change)
		}
	}
}
