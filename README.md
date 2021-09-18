# shed

shed is a simple CLI tool that makes it easy to install and manage Go tool dependencies.
It is built on top of Go Modules, and allows for reproducible dev environments.

## Installation

#### Homebrew

```
brew install cszatmary/tap/shed
```

#### Binary Release

```
curl -sSfL https://raw.githubusercontent.com/cszatmary/shed/main/scripts/install.sh | sh -s -- -b /usr/local/bin
```

This will install it to `/usr/local/bin/shed`. You can specify a different path to `-b` to customize the install location.

You can also install a specific version by providing the git tag as an argument.

```
curl -sSfL https://raw.githubusercontent.com/cszatmary/shed/main/scripts/install.sh | sh -s -- -b /usr/local/bin v0.1.0
```

#### Install from source

You can also install shed using `go get`:

```
go get github.com/cszatmary/shed
```

Note: Installing from source requires a minimum Go version of 1.17.

## Usage

shed requires a minimum Go version of `1.11` to be installed. This is because it builds tools from source using Go modules.

### Installing tools

shed uses the import path with an optional version to install tools, just like `go get` in module mode.

```
shed get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.33.0
```

If the version is omitted, the latest version will be installed.

```
shed get github.com/golangci/golangci-lint/cmd/golangci-lint
```

If no arguments are provided, shed will install all tools in the `shed.lock` file.

```
shed get
```

To uninstall a tool use the special `@none` version suffix.

```
shed get github.com/golangci/golangci-lint/cmd/golangci-lint@none
```

### Running tools

Once a tool is installed it can be run using `shed run`. This can take either the name of the tool binary,
or the full import path.

**Note**: The binary name must be unique to use it directly.

```
shed run golangci-lint run
```

Or

```
shed run github.com/golangci/golangci-lint/cmd/golangci-lint run
```

All additional arguments are passed to the tool being run. Any flags after the tool name are passed to the
tool directly and are not parsed by shed.

```
shed run stringer -type=Pill
```

## `shed.lock`

shed will generate a `shed.lock` file in the current directory if one does not already exists. This contains a list of all
installed tools and their verions. If new tools are installed, or tools are uninstalled, the lockfile is updated accordingly.

The `shed.lock` file allows shed to have reproducible installs. It ensures that the same version of each tool is always installed.
For this reason, it is recommended that you check this into source control.
