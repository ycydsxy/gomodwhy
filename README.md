# gomodwhy

A tool to analyze why a specific package is included in your Go project's dependency graph, just like `go mod why`. 

- `go mod why` only shows the first import path, while this tool will find all possible import paths. 
- `go mod graph` always includes test dependencies, while this tool exclude test dependencies by default and provide an option to include them.

## What does it do?

`gomodwhy` helps you understand the dependency chain that leads to a particular package being included in your Go project. It:

- Builds a comprehensive dependency graph of your project
- Finds all possible import paths from your project to the target package
- Displays the dependency chains in a `go mod why` way
- Supports depth limiting for complex dependency trees
- Allows including test dependencies in the analysis
- Works with any Go module

## Installation

```bash
go install github.com/ycydsxy/gomodwhy@latest
```

## Usage

```bash
gomodwhy [options] <target-pkg>
```

### Options

- `-p, --pattern` - Go list package matching pattern (default: `.`)
- `-d, --depth` - Dependency path depth limit, 0 for unlimited (default: `0`)
- `-t, --include-test` - Include test dependencies
- `-v, --verbose` - Print verbose information

### Examples

#### Find why a package is imported in the current directory

```bash
gomodwhy fmt
# fmt
github.com/ycydsxy/gomodwhy
fmt

github.com/ycydsxy/gomodwhy
encoding/json
fmt

github.com/ycydsxy/gomodwhy
github.com/jessevdk/go-flags
fmt

github.com/ycydsxy/gomodwhy
github.com/jessevdk/go-flags
golang.org/x/sys/unix
fmt
```

#### Limit dependency path depth

```bash
gomodwhy -d 1 fmt
# fmt
encoding/json
fmt

github.com/jessevdk/go-flags
fmt

github.com/ycydsxy/gomodwhy
fmt

golang.org/x/sys/unix
fmt
```

#### Use with a specific package pattern

```bash
gomodwhy -p github.com/jessevdk/go-flags fmt
# fmt
github.com/jessevdk/go-flags
fmt

github.com/jessevdk/go-flags
golang.org/x/sys/unix
fmt
```

#### Verbose output

```bash
gomodwhy -v fmt
Executing go list command to get dependency information...
Successfully got dependency information for 69 packages
Building dependency graph...
Dependency graph built successfully
Analyzing dependency paths...
Successfully analyzed 4 dependency paths

# fmt
github.com/ycydsxy/gomodwhy
fmt

github.com/ycydsxy/gomodwhy
encoding/json
fmt

github.com/ycydsxy/gomodwhy
github.com/jessevdk/go-flags
fmt

github.com/ycydsxy/gomodwhy
github.com/jessevdk/go-flags
golang.org/x/sys/unix
fmt
```

#### Include test dependencies

```bash
gomodwhy -t crypto/sha256
# crypto/sha256
github.com/ycydsxy/gomodwhy
crypto/sha256
```

## How it works

1. **Dependency Collection**: Uses `go list -deps -json` (with `-test` flag if test dependencies are included) to gather dependency information
2. **Graph Construction**: Builds both forward and reverse dependency graphs, including test imports if requested
3. **Path Analysis**: Uses BFS (Breadth-First Search) to find all paths from your project to the target package
4. **Path Processing**: Handles depth limits and removes duplicate paths
5. **Output**: Displays the dependency chains in a clear format

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
