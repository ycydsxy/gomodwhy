package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jessevdk/go-flags"
)

type Package struct {
	ImportPath  string
	Imports     []string
	TestImports []string
}

func runGoList(pattern string, includeTest bool) ([]Package, error) {
	args := []string{"list", "-deps", "-json"}
	// if includeTest {
	// 	args = append(args, "-test")
	// }
	args = append(args, pattern)
	cmd := exec.Command("go", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Read stderr to buffer
	var stderrBuf strings.Builder
	go func() { io.Copy(&stderrBuf, stderr) }()

	var packages []Package
	dec := json.NewDecoder(stdout)
	for {
		var p Package
		if err := dec.Decode(&p); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("go list failed: %v\n\n%s\n%s", err, cmd.String(), stderrBuf.String())
		}
		packages = append(packages, p)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("go list failed: %v\n\n%s\n%s", err, cmd.String(), stderrBuf.String())
	}
	return packages, nil
}

func buildForward(packages []Package, includeTest bool) map[string][]string {
	forward := make(map[string][]string)
	for _, p := range packages {
		forward[p.ImportPath] = append(forward[p.ImportPath], p.Imports...)
		if includeTest {
			forward[p.ImportPath] = append(forward[p.ImportPath], p.TestImports...)
		}
	}
	return forward
}

func allPaths(from string, to string, forward map[string][]string) [][]string {
	// Build reverse dependency graph, search up from target package
	reverseMap := make(map[string][]string)
	for u, vs := range forward {
		for _, v := range vs {
			reverseMap[v] = append(reverseMap[v], u)
		}
	}

	// Use BFS to search up from target package
	var paths [][]string
	queue := [][]string{{to}}

	for len(queue) > 0 {
		currentPath := queue[0]
		queue = queue[1:]

		lastNode := currentPath[0]

		// If reached the starting point, reverse path and add to results
		if lastNode == from {
			reversedPath := make([]string, len(currentPath))
			for i, node := range currentPath {
				reversedPath[len(currentPath)-1-i] = node
			}
			paths = append(paths, reversedPath)
			continue
		}

		// Iterate through all possible predecessor nodes
		for _, prev := range reverseMap[lastNode] {
			// Check if path already contains this node to avoid cycles
			duplicate := false
			for _, node := range currentPath {
				if node == prev {
					duplicate = true
					break
				}
			}
			if duplicate {
				continue
			}

			// Create new path and add to queue
			newPath := make([]string, len(currentPath)+1)
			newPath[0] = prev
			copy(newPath[1:], currentPath)
			queue = append(queue, newPath)
		}
	}

	return paths
}

func uniquePaths(paths [][]string) [][]string {
	seen := make(map[string]bool)
	var out [][]string
	for _, p := range paths {
		k := strings.Join(p, "->")
		if !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	return out
}

func handleDepth(paths [][]string, depth int) [][]string {
	if depth <= 0 {
		return paths
	}
	var out [][]string
	for _, p := range paths {
		if len(p) <= depth {
			out = append(out, p)
		} else {
			out = append(out, p[:depth+1])
		}
	}
	return out
}

func printPaths(target string, paths [][]string) {
	fmt.Printf("# %s\n", target)
	if len(paths) == 0 {
		fmt.Println("no import chain found")
		return
	}
	for _, p := range paths {
		for i := len(p) - 1; i >= 0; i-- {
			fmt.Println(p[i])
		}
		fmt.Println()
	}
}

type Opts struct {
	Pattern     string `long:"pattern" short:"p" description:"go list package matching pattern" default:"."`
	Depth       int    `long:"depth" short:"d" description:"dependency path depth limit, 0 for unlimited" default:"0"`
	IncludeTest bool   `long:"include-test" short:"t" description:"include test dependencies"`
	Verbose     bool   `long:"verbose" short:"v" description:"print verbose information"`
}

func (o Opts) Printf(format string, a ...interface{}) {
	if o.Verbose {
		fmt.Printf(format, a...)
	}
}

func main() {
	var opts Opts
	parser := flags.NewParser(&opts, flags.Default)
	parser.Name = "gomodwhy"
	parser.Usage = "[options] <target-pkg>"

	args, err := parser.Parse()
	if err != nil {
		os.Exit(1)
	}

	if len(args) != 1 {
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}

	targetPkg := args[0]

	opts.Printf("Executing go list command to get dependency information...\n")
	packages, err := runGoList(opts.Pattern, opts.IncludeTest)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	if len(packages) == 0 {
		fmt.Fprintf(os.Stderr, "no package found\n\n")
		parser.WriteHelp(os.Stderr)
		os.Exit(1)
	}
	opts.Printf("Successfully got dependency information for %d packages\n", len(packages))
	root := packages[len(packages)-1].ImportPath // go list use post-order traversal

	opts.Printf("Building dependency graph...\n")
	forward := buildForward(packages, opts.IncludeTest)
	opts.Printf("Dependency graph built successfully\n")

	opts.Printf("Analyzing dependency paths...\n")
	paths := allPaths(root, targetPkg, forward)
	paths = handleDepth(paths, opts.Depth)
	paths = uniquePaths(paths)

	opts.Printf("Analysis completed, found %d dependency paths\n\n", len(paths))
	printPaths(targetPkg, paths)
}
