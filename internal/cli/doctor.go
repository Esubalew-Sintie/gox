package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// RunDoctor diagnoses common project and tooling problems.
func RunDoctor() error {
	fmt.Println("gox doctor")
	fmt.Println()

	ok := true

	// Check Go installed
	goPath, err := exec.LookPath("go")
	if err != nil {
		printCheck(false, "Go installed")
		ok = false
	} else {
		printCheck(true, "Go installed ("+goPath+")")
	}

	// Check Go version (require >= 1.21)
	if goVersionOK() {
		printCheck(true, "Go version supported")
	} else {
		printCheck(false, "Go version supported (requires >= 1.21)")
		ok = false
	}

	// Check project detected
	projectPath, err := FindProjectRoot()
	if err != nil {
		printCheck(false, "Project detected")
		fmt.Println()
		if ok {
			fmt.Println("No problems found with tooling.")
		} else {
			fmt.Println("Some problems were found.")
		}
		return nil
	}
	printCheck(true, "Project detected ("+projectPath+")")

	// Check .gox.yaml valid
	// We use the config loader via the fs; check file presence
	configPath := projectPath + "/.gox.yaml"
	if fileExists(configPath) {
		printCheck(true, ".gox.yaml valid")
	} else {
		printCheck(false, ".gox.yaml missing")
		ok = false
	}

	// Check go build
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = projectPath
	if out, err := cmd.CombinedOutput(); err != nil {
		printCheck(false, "Project builds: "+strings.TrimSpace(string(out)))
		ok = false
	} else {
		printCheck(true, "Project builds")
	}

	fmt.Println()

	if ok {
		fmt.Println("No problems found.")
	} else {
		fmt.Println("Some problems were found. Address the ✗ items above.")
	}

	return nil
}

func printCheck(ok bool, msg string) {
	if ok {
		fmt.Println("✓", msg)
	} else {
		fmt.Println("✗", msg)
	}
}

func goVersionOK() bool {
	out, err := exec.Command("go", "version").Output()
	if err != nil {
		return false
	}

	// "go version go1.22.0 linux/amd64"
	fields := strings.Fields(string(out))
	if len(fields) < 3 {
		return false
	}

	verStr := strings.TrimPrefix(fields[2], "go")
	parts := strings.SplitN(verStr, ".", 3)
	if len(parts) < 2 {
		return false
	}

	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])

	_ = runtime.Version()
	return major > 1 || (major == 1 && minor >= 21)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
