package tfpath

import (
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"gopkg.gen64.pl/tfsketch/internal/overrides"
)

var (
	ErrWalkingOverrides      = errors.New("error walking overrides")
	ErrParsingContainerPaths = errors.New("error parsing container paths")
	ErrLinkingContainerPaths = errors.New("error linking container paths")
)

const (
	parsePathMaxDepth = 6
)

// Container contains TfPath instances for external modules and the root of the local path that is scanned.
type Container struct {
	// Paths contains mapping of module or path to an TfPath object
	Paths map[string]*TfPath

	// Overrides contains regular expressions against which module or path can be matched and have its local path assigned
	Overrides map[string]*Override
}

// Override represents remote-to-local mapping
type Override struct {
	// Remote is a regular expression to which module or path is matched against.
	Remote *regexp.Regexp

	// Local represents a template for a local directory when module or path matches Remote.
	Local string

	// Cache is a source URL for the module that it needs to be downloaded from. It takes
	// precedence before Local.
	Cache string
}

// NewContainer returns a new Container.
func NewContainer() *Container {
	container := &Container{
		Paths:     map[string]*TfPath{},
		Overrides: map[string]*Override{},
	}

	return container
}

// AddPath adds a new TfPath to the container (external module).
func (c *Container) AddPath(name string, tfPath *TfPath) {
	c.Paths[name] = tfPath

	slog.Info(fmt.Sprintf("🔸 Module added: 📦%s in 📁%s", name, tfPath.Path))
}

// WalkOverrides runs traverser's WalkPath on each entry from Override object
func (c *Container) WalkOverrides(overrides *overrides.Overrides, traverser *Traverser, cache *Cache) error {
	for _, externalModule := range overrides.ExternalModules {
		remoteField := strings.TrimSpace(externalModule.Remote)
		localField := strings.TrimSpace(externalModule.Local)
		cacheField := strings.TrimSpace(externalModule.Cache)

		// Either local or cache must be present
		if remoteField == "" || (localField == "" && cacheField == "") {
			continue
		}

		// Remote starting with ^ are considered regular expression and modules sources should be matched
		// against those
		if strings.HasPrefix(remoteField, "^") {
			_, exists := c.Overrides[remoteField]
			if exists {
				continue
			}

			// add these regular expressions to the container
			c.Overrides[remoteField] = &Override{
				Remote: regexp.MustCompile(remoteField),
				Local:  localField,
				Cache:  cacheField,
			}

			continue
		}

		// If cache is passed then we need to try to download the module first and then pass its local path
		if cacheField != "" && cache != nil {
			// If there was an attempt to download this module already then try to get it
			// from the container.
			if cache.WasDownloaded(remoteField) {
				localTfPath, exists := c.Paths[remoteField]
				if !exists {
					continue
				}

				localField = localTfPath.Path
			} else {
				// Download the module source code and save it in the cache
				downloadedPath, err := cache.DownloadModule(remoteField, cacheField)
				if err != nil {
					slog.Error(
						fmt.Sprintf(
							"❌ Error downloading module (📦%s): %s",
							remoteField,
							err.Error(),
						),
					)

					continue
				}

				if downloadedPath == "" {
					continue
				}

				localField = downloadedPath
			}
		}

		if localField == "" {
			continue
		}

		tfPath := NewTfPath(localField, remoteField)
		c.AddPath(tfPath.TraverseName, tfPath)

		isSubModule := c.isExternalModuleASubModule(remoteField)

		if tfPath.Walked {
			continue
		}

		err := traverser.WalkPath(tfPath, !isSubModule)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error walking dirs in overrides local path 📁%s: %s",
					localField,
					err.Error(),
				),
			)

			return fmt.Errorf("%w: %w", ErrWalkingOverrides, err)
		}
	}

	return nil
}

// ParsePaths runs traverser's ParsePath on each path
func (c *Container) ParsePaths(traverser *Traverser, cache *Cache, depth int) error {
	// Limit number of recursive calls
	if depth == parsePathMaxDepth {
		return nil
	}

	foundModules := []string{}

	for pathName, tfPath := range c.Paths {
		if tfPath.Parsed {
			continue
		}

		err := traverser.ParsePath(tfPath, &foundModules)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error parsing container terraform path 📁%s (%s) : %s",
					tfPath.Path,
					pathName,
					err.Error(),
				),
			)

			return fmt.Errorf("%w: %w", ErrParsingContainerPaths, err)
		}
	}

	if cache != nil && len(foundModules) > 0 {
		overrides := &overrides.Overrides{}

		for _, containerPathKey := range foundModules {
			_, exists := c.Paths[containerPathKey]
			if exists {
				continue
			}

			if cache.WasDownloaded(containerPathKey) {
				continue
			}

			var localPath, cacheUrl string

			localPath, cacheUrl = c.MatchesOverride(containerPathKey)
			if localPath != "" {
				overrides.AddExternalModule(containerPathKey, localPath)
				continue
			}

			downloadedPath, err := cache.DownloadModule(containerPathKey, cacheUrl)
			if err != nil {
				slog.Error(
					fmt.Sprintf(
						"❌ Error downloading module (📦%s): %s",
						containerPathKey,
						err.Error(),
					),
				)

				continue
			}

			if downloadedPath == "" {
				continue
			}

			overrides.AddExternalModule(containerPathKey, downloadedPath)
		}

		if len(overrides.ExternalModules) > 0 {
			err := c.WalkOverrides(overrides, traverser, cache)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrWalkingOverrides, err)
			}

			err = c.ParsePaths(traverser, cache, depth+1)
			if err != nil {
				return fmt.Errorf("%w: %w", ErrParsingContainerPaths, err)
			}
		}
	}

	return nil
}

// LinkPaths will run traverser's LinkPath on each path
func (c *Container) LinkPaths(traverser *Traverser) error {
	for pathName, tfPath := range c.Paths {
		err := traverser.LinkPath(tfPath)
		if err != nil {
			slog.Error(
				fmt.Sprintf(
					"❌ Error linking local modules in terraform path 📁%s (%s) : %s",
					tfPath.Path,
					pathName,
					err.Error(),
				),
			)

			return fmt.Errorf("%w: %w", ErrLinkingContainerPaths, err)
		}
	}

	return nil
}

// MatchesOverride checks if specific module/path is found in overrides.
func (c *Container) MatchesOverride(containerPathKey string) (string, string) {
	for regexpString, override := range c.Overrides {
		matches := override.Remote.FindStringSubmatch(containerPathKey)
		if len(matches) == 0 {
			continue
		}

		localPath := override.Local
		cacheUrl := override.Cache

		if cacheUrl != "" {
			for i, sub := range matches {
				cacheUrl = strings.ReplaceAll(cacheUrl, fmt.Sprintf("{%d}", i), sub)
			}

			slog.Debug(
				fmt.Sprintf(
					"🧩 Module 📦%s matches override regexp 📦%s and points to source 📁%s",
					containerPathKey,
					regexpString,
					cacheUrl,
				),
			)

			return "", cacheUrl
		}

		if localPath == "" {
			return "", ""
		}

		for i, sub := range matches {
			localPath = strings.ReplaceAll(localPath, fmt.Sprintf("{%d}", i), sub)
		}

		slog.Debug(
			fmt.Sprintf(
				"🧩 Module 📦%s matches override regexp 📦%s and points to local path 📁%s",
				containerPathKey,
				regexpString,
				localPath,
			),
		)

		return localPath, ""
	}

	return "", ""
}

func (c *Container) isExternalModuleASubModule(module string) bool {
	return strings.Contains(module, "//modules/")
}
