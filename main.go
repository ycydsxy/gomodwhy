package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"sort"
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

func hasCycle(path []string, node string) bool {
	for _, n := range path {
		if n == node {
			return true
		}
	}
	return false
}

func mergePaths(fromPath []string, toPath []string) []string {
	merged := make([]string, len(fromPath)+len(toPath))
	copy(merged, fromPath)
	for i := 0; i < len(toPath); i++ {
		merged[len(fromPath)+i] = toPath[i]
	}
	return merged
}

func trimAndUnique(paths [][]string, depth int) [][]string {
	set := make(map[string]struct{})
	res := make([][]string, 0)
	for _, path := range paths {
		if len(path) >= depth+1 {
			path = path[:depth+1]
		}
		key := strings.Join(path, "->")
		if _, ok := set[key]; ok {
			continue
		}
		set[key] = struct{}{}
		res = append(res, path)
	}
	return res
}

func reversePaths(paths [][]string) [][]string {
	reversed := make([][]string, 0, len(paths))
	for _, reversedPath := range paths {
		path := make([]string, 0, len(reversedPath))
		for i := len(reversedPath) - 1; i >= 0; i-- {
			path = append(path, reversedPath[i])
		}
		reversed = append(reversed, path)
	}
	return reversed
}

func allPaths(start string, end string, forward map[string][]string, depth int) [][]string {
	if depth <= 0 {
		depth = math.MaxInt32
	}

	// Build reversed graph
	reversedMap := make(map[string][]string)
	for k, v := range forward {
		for _, next := range v {
			reversedMap[next] = append(reversedMap[next], k)
		}
	}

	// Find all paths from end to start in reversed graph
	paths := doAllPaths(end, start, reversedMap, depth, map[string]*depthCache{})

	// Reverse paths to get from start to end
	paths = reversePaths(paths)

	// Sort paths by length and lexicographically
	sort.Slice(paths, func(i, j int) bool {
		if len(paths[i]) != len(paths[j]) {
			return len(paths[i]) < len(paths[j])
		}
		return strings.Join(paths[i], "->") < strings.Join(paths[j], "->")
	})

	return paths
}

type depthCache struct {
	depth int
	paths [][]string
}

func (c *depthCache) get(depth int) ([][]string, bool) {
	if c == nil || depth > c.depth {
		return nil, false
	}
	return trimAndUnique(c.paths, depth), true
}

func (c *depthCache) put(depth int, paths [][]string) {
	if depth <= c.depth {
		return
	}
	c.depth = depth
	c.paths = paths
}

// doAllPaths returns all paths from start to end in forward graph.
// Note: There is a premise that any path from the `start` node will eventually reach the `end` node.
func doAllPaths(start string, end string, forward map[string][]string, depthLeft int, cache map[string]*depthCache) [][]string {
	if start == end || depthLeft <= 0 {
		return [][]string{{start}}
	}
	if len(forward[start]) == 0 {
		return nil
	}
	if paths, ok := cache[start].get(depthLeft); ok {
		return paths
	}
	res := make([][]string, 0)
	for _, next := range forward[start] {
		paths := doAllPaths(next, end, forward, depthLeft-1, cache)
		var pathsToAppend [][]string
		for _, path := range paths {
			if hasCycle(path, start) {
				continue
			}
			pathsToAppend = append(pathsToAppend, mergePaths([]string{start}, path))
		}
		res = append(res, pathsToAppend...)
	}

	if cache[start] == nil {
		cache[start] = new(depthCache)
	}
	cache[start].put(depthLeft, res)

	return res
}

func printPaths(target string, paths [][]string) {
	fmt.Printf("# %s\n", target)
	if len(paths) == 0 {
		fmt.Println("no import chain found")
		return
	}
	for _, p := range paths {
		for _, item := range p {
			fmt.Println(item)
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
	forwardMap := buildForward(packages, opts.IncludeTest)
	opts.Printf("Dependency graph built successfully\n")

	opts.Printf("Analyzing dependency paths...\n")
	paths := allPaths(root, targetPkg, forwardMap, opts.Depth)
	opts.Printf("Successfully analyzed %d dependency paths\n\n", len(paths))
	printPaths(targetPkg, paths)
}
