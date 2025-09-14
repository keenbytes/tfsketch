package tfpath

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

type Cache struct {
	path string
	regexpExternalModule *regexp.Regexp
	regexpVersion *regexp.Regexp
}

func NewCache(path string) *Cache {
	cache := &Cache{
		path: path,
		regexpExternalModule: regexp.MustCompile(`^[a-z]+.*$`),
		regexpVersion: regexp.MustCompile(`^[a-z0-9\.\-_]*$`),
	}

	return cache
}

func (c *Cache) DownloadModule(sourceVersion string) (string, error) {
	if !c.regexpExternalModule.MatchString(sourceVersion) {
		slog.Debug(
			fmt.Sprintf(
				"🚫 Skipped downloading module 📦%s as it is not external",
				sourceVersion,
			),
		)

		return "", nil
	}

	var source, version string
	split := strings.SplitN(sourceVersion, "@", 1)
	if len(split) == 2 {
		source = split[0]
		version = split[1]

		if !c.regexpVersion.MatchString(version) {
			slog.Debug(
				fmt.Sprintf(
					"🚫 Skipped downloading module 📦%s as it has invalid version",
					sourceVersion,
				),
			)

			return "", nil
		}
	} else {
		source = strings.Replace(sourceVersion, "@", "", 1)
		version = "latest"
	}

	slog.Debug(
		fmt.Sprintf(
			"🌍 Trying to download module 📦%s@%s",
			source,
			version,
		),
	)

	return "", nil
}
