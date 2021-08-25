# cmd/check

Check terraform code for modules and compare their version with a reference to suggest upgrade.
Can also patch code to latest versions.

# Build

Run `make build` to compile a binary for your machine.
Run the `make` command to see all other options.

# Install

Compile and install the binary: `make clean && make build && make install`. This puts the new binary into `bin/check`
Copy or symlink `./check` in you PATH (ex: /usr/local/bin)

# Manual usage:

```bash
Usage: check [-a action] [-c change_type] [files or directories...]
Checks or patches directories and/or files for obsolete terraform modules.
The source of truth is https://tfmodules.deliveroo.net/modules.json
Options are:
  -a string
    	Action to take on files: 'check' or 'patch' (default "check")
  -c string
    	Filter module version changes: only 'minor', 'major' or 'all' (default "all")
  -d	Enable debug

```

# Git hook usage


1/ Install the `check` binary in your PATH.
2/ Add a `pre-commit` hook in `.git/hooks/tfmodule-checker` (based on `pre-commit.sample`)

```bash
#!/bin/sh
if git rev-parse --verify HEAD >/dev/null 2>&1
then
	against=HEAD
else
	against=$(git hash-object -t tree /dev/null)
fi

exec 1>&2

# Report obsolete modules and force developer to update them (aka prevent commit)
exec git diff-index --name-only --cached $against -- | xargs check
```
