# chromeHistoryCleaner

A small, cross-platform CLI that **surgically removes browsing data for a specific
domain or keyword** from Google Chrome's local databases — URLs, visits, segments,
keyword search terms, search keywords, and autofill entries.

> ⚠️ Deletions are permanent. Always run with `-dry-run` first, and make sure
> Chrome is fully closed before cleaning.

## Install

Download the latest release for your platform from the
[Releases page](https://github.com/OpScaleHub/chromeHistoryCleaner/releases/latest),
or build from source:

```bash
go build -o chrome-cleaner .
```

## Usage

```bash
# Preview what would be deleted (no changes made)
chrome-cleaner -site example.com -dry-run

# Delete data for a domain (prompts for confirmation)
chrome-cleaner -site example.com

# Target a specific profile
chrome-cleaner -site example.com -profile "Profile 1"

# List available Chrome profiles
chrome-cleaner -list-profiles

# Print version
chrome-cleaner -version
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-site` | Domain or keyword to target for deletion (required) | — |
| `-profile` | Chrome profile directory name | `Default` |
| `-dry-run` | Show the impact report without deleting anything | `false` |
| `-list-profiles` | List all detected Chrome profiles | `false` |
| `-version` | Print version information and exit | `false` |

The `-site` value is matched as a literal substring; SQL `LIKE` wildcards
(`%`, `_`) in the input are escaped and treated literally.

## Notes

- Chrome **must be closed** while cleaning; the tool refuses to run otherwise.
- Supported platforms: Linux, macOS, Windows.
- A `VACUUM` is run after deletion to reclaim disk space.

## Development

```bash
go vet ./...
go test ./...
go build .
```

## License

[MIT](./LICENSE)
