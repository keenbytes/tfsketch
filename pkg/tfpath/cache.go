package tfpath

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrDownloadingModule               = errors.New("error downloading module")
	ErrGettingModuleDir                = errors.New("errors getting module directory")
	ErrModuleDirAlreadyExistsAndNotDir = errors.New("module directory already exists and not a dir")
	ErrCreatingModuleDir               = errors.New("error creating module directory")
	ErrGitCloneFailed                  = errors.New("error running 'git clone' command")
	ErrGitCheckoutFailed               = errors.New("error running 'git checkout' command")
)

const (
	headerWithSource = "X-Terraform-Get"
	gitCloneTimeout = 120
)

type Cache struct {
	path string
	regexpExternalModule *regexp.Regexp
	regexpVersion *regexp.Regexp
	regexpGit *regexp.Regexp
	downloaded map[string]struct{}
}

func NewCache(path string) *Cache {
	cache := &Cache{
		path: path,
		regexpExternalModule: regexp.MustCompile(`^[a-z]+.*$`),
		regexpVersion: regexp.MustCompile(`^[a-z0-9\.\-_]*$`),
		regexpGit: regexp.MustCompile(`^git::.*$`),
		downloaded: map[string]struct{}{},
	}

	return cache
}

func (c *Cache) WasDownloaded(sourceVersion string) bool {
	_, exists := c.downloaded[sourceVersion]

	return exists
}

func (c *Cache) DownloadModule(sourceVersion string) (string, error) {
	// mark module as one that has already been downloaded
	c.downloaded[sourceVersion] = struct{}{}

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
	}

	slog.Debug(
		fmt.Sprintf(
			"🌍 Trying to download module 📦%s@%s",
			source,
			version,
		),
	)

	var url string
	if version == "" {
		// http.Get will follow redirect
		url = fmt.Sprintf("https://registry.terraform.io/v1/modules/%s/download", source)
	} else {
		url = fmt.Sprintf("https://registry.terraform.io/v1/modules/%s/%s/download", source, version)
	}

	resp, err := http.Get(url)
	if err != nil {
		slog.Error(fmt.Sprintf("❌ Error downloading module: %s", err.Error()))

		return "", fmt.Errorf("%w: %w", ErrDownloadingModule, err)
	}

	defer resp.Body.Close()

	sourceGitRepository := resp.Header.Get(headerWithSource)
	if sourceGitRepository == "" {
		slog.Debug(
			fmt.Sprintf(
				"🚫 '%s' header not found in response from %s",
				headerWithSource,
				url,
			),
		)

		return "", nil
	}

	if !c.regexpGit.MatchString(sourceGitRepository) {
		slog.Error(fmt.Sprintf("🚫 %s not supported: %s", headerWithSource, sourceGitRepository))

		return "", nil
	}

	gitUrl := strings.Replace(sourceGitRepository, "git::", "", 1)

	split = strings.SplitN(gitUrl, "?", 2)
	if len(split) != 2 {
		return "", nil
	}

	gitUrl = split[0]
	gitCommit := strings.Replace(split[1], "ref=", "", 1)
	slog.Debug(
		fmt.Sprintf(
			"🌎 Cloning module 📦%s repository %s commit %s",
			source,
			gitUrl,
			gitCommit,
		),
	)

	moduleDirName := strings.ReplaceAll(source+"@"+version, "/", "__")
	moduleDirPath := filepath.Join(c.path, moduleDirName)

	var nextStepGitClone bool
	// Check if module directory already exists
	dirStat, err := os.Stat(moduleDirPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the directory
			if err := os.MkdirAll(moduleDirPath, 0755); err != nil {
				slog.Error(fmt.Sprintf("🚫 Error creating dir module %s: %s", moduleDirPath, err.Error()))

				return "", fmt.Errorf("%w: %w", ErrCreatingModuleDir, err)
			}
			nextStepGitClone = true
		} else {
			slog.Error(fmt.Sprintf("🚫 Error checking dir module: %s", err.Error()))

			return "", fmt.Errorf("%w: %w", ErrGettingModuleDir, err)
		}
	} else {
		if !dirStat.IsDir() {
			slog.Error("🚫 Cache module directory already exists and it is not a directory")

			return "", fmt.Errorf("%w: %w", ErrModuleDirAlreadyExistsAndNotDir, err)
		}
	}

	if !nextStepGitClone {
		slog.Debug(fmt.Sprintf("🔸 Found cached module directory for 📦%s@%s at 📁%s", source, version, moduleDirPath))
	}

	cmdName := "git"

	if nextStepGitClone {
		cmdArgs := []string{"clone", gitUrl, moduleDirPath}

		ctx, cancel := context.WithTimeout(context.Background(), gitCloneTimeout*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
		if err := cmd.Run(); err != nil {
			slog.Error(fmt.Sprintf("🚫 Command '%s %s' failed: %s", cmdName, strings.Join(cmdArgs, " "), err.Error()))

			return "", fmt.Errorf("%w: %w", ErrGitCloneFailed, err)
		}
	}

	cmdArgsMatrix := [][]string{
		{"fetch", "--all"},
		{"checkout", gitCommit},
	}

	for _, cmdArgs := range cmdArgsMatrix {
		ctx, cancel := context.WithTimeout(context.Background(), gitCloneTimeout*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, cmdName, cmdArgs...)
		cmd.Dir = moduleDirPath
		if err := cmd.Run(); err != nil {
			slog.Error(fmt.Sprintf("🚫 Command '%s %s' failed in 📁%s: %s", cmdName, strings.Join(cmdArgs, " "), moduleDirPath, err.Error()))

			return "", fmt.Errorf("%w: %w", ErrGitCheckoutFailed, err)
		}
	}

	slog.Info(fmt.Sprintf("🔸 Changed ref for cached module 📦%s@%s in 📁%s to %s", source, version, moduleDirPath, gitCommit))

	return moduleDirPath, nil
}
