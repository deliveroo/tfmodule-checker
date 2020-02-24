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
	"strconv"
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
		fmt.Printf("DEBUG: %s\n", msg)
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

// TODO: refactor
// checkTerraformModules compare module version in a terraform file with latest known
func checkTerraformModules(path string, modules moduleIndex, reportMode string) (changes []string, err error) {
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
		if checkModuleVersion(m[1], m[2], modules, reportMode) {
			changes = append(changes, fmt.Sprintf("%s:%d `%s` version %s (latest %s)", path, n, m[1], m[2], modules[m[1]].Version))
		}
	}
	return changes, nil
}

// TODO: refactor
// patchModules updates terraform files with updated module files
func patchModules(path string, modules moduleIndex, reportMode string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Printf("Failed to read %s: %s\n", path, err)
		return err
	}
	var buf []byte
	lines := strings.Split(string(data), "\n")
	moduleSourceFindPtn := `^\s*source\s*=\s*"` + ModuleRepo + `([^/]+)/([^/]+)\.zip"\s*$`
	moduleSourceReplacePtn := `/[^/]+\.zip`
	re := regexp.MustCompile(moduleSourceFindPtn)
	repl := regexp.MustCompile(moduleSourceReplacePtn)
	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if m != nil {
			if checkModuleVersion(m[1], m[2], modules, reportMode) {
				line = repl.ReplaceAllString(line, "/"+modules[m[1]].Version+".zip")
			}
		}
		line = line + "\n"
		buf = append(buf, line...)
	}
	// overwrite file (we want something we can push as a PR)
	if err = ioutil.WriteFile(path, buf, os.FileMode(0644)); err != nil {
		fmt.Printf("Failed to write to %s: %s\n", path, err)
		return err
	}

	return nil
}

// checkModuleVersion report if a module version in code is older than latest available
// reportMode can be used to filter out Major/Minor change or return all cases
func checkModuleVersion(name, version string, modules moduleIndex, reportMode string) bool {

	v := strings.Split(version, ".")
	vMajor, _ := strconv.Atoi(v[0])
	vMinor, _ := strconv.Atoi(v[1])
	mMajor, mMinor := 0, 0 // a dummy string that is higher than any version ;)
	mVersion, ok := modules[name]
	if ok {
		parts := strings.Split(mVersion.Version, ".")
		mMajor, _ = strconv.Atoi(parts[0])
		mMinor, _ = strconv.Atoi(parts[1])
	}
	check := false
	switch reportMode {
	case "major":
		check = (vMajor < mMajor)
	case "minor":
		check = (vMajor == mMajor) && (vMinor < mMinor)
	default:
		check = (vMajor <= mMajor) && (vMinor < mMinor)
	}
	debug(fmt.Sprintf("%s: %s vs %s (%t)", name, version, mVersion.Version, check))
	return check
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

	var root, action, report string

	flag.BoolVar(&DEBUG, "d", false, "Enable debug")
	flag.StringVar(&root, "r", "", "Root of local directory to scan")
	flag.StringVar(&action, "a", "check", "Action to take on files: check or patch")
	flag.StringVar(&report, "c", "all", "Report selector 'minor', 'major' or 'all' version changes")
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

	var reportMode string
	switch report {
	case "minor":
		fallthrough
	case "major":
		fallthrough
	case "all":
		reportMode = report
	default:
		fmt.Printf("Unknown report selector: %s\n", report)
		os.Exit(1)
	}
	debug(reportMode)

	switch action {
	case "check":
		var changes []string
		for _, file := range files {
			changes, err = checkTerraformModules(file, modules, reportMode)

			for _, change := range changes {
				fmt.Printf("%s\n", change)
			}
		}
	case "patch":
		for _, file := range files {
			err = patchModules(file, modules, reportMode)
		}
	default:
		fmt.Printf("Unknown action %s\n", action)
		os.Exit(1)
	}

}
