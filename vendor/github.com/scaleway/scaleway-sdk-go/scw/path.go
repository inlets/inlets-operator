package scw

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	unixHomeDirEnv    = "HOME"
	windowsHomeDirEnv = "USERPROFILE"
	xdgConfigDirEnv   = "XDG_CONFIG_HOME"

	defaultConfigFileName = "config.yaml"
)

var (
	// ErrNoHomeDir errors when no user directory is found
	ErrNoHomeDir = errors.New("user home directory not found")
)

// GetConfigPath returns the default path.
// Default path is base on the following priority order:
// - $SCW_CONFIG_PATH
// - $XDG_CONFIG_HOME/scw/config.yaml
// - $HOME/.config/scw/config.yaml
// - $USERPROFILE/.config/scw/config.yaml
func GetConfigPath() string {
	configPath := os.Getenv(scwConfigPathEnv)
	if configPath == "" {
		configPath, _ = getConfigV2FilePath()
	}
	return filepath.Clean(configPath)
}

// getConfigV2FilePath returns the path to the Scaleway CLI config file
func getConfigV2FilePath() (string, bool) {
	configDir, err := getScwConfigDir()
	if err != nil {
		return "", false
	}
	return filepath.Clean(filepath.Join(configDir, defaultConfigFileName)), true
}

// getConfigV1FilePath returns the path to the Scaleway CLI config file
func getConfigV1FilePath() (string, bool) {
	path, err := getHomeDir()
	if err != nil {
		return "", false
	}
	return filepath.Clean(filepath.Join(path, ".scwrc")), true
}

// getScwConfigDir returns the path to scw config folder
func getScwConfigDir() (string, error) {
	if xdgPath := os.Getenv(xdgConfigDirEnv); xdgPath != "" {
		return filepath.Join(xdgPath, "scw"), nil
	}

	homeDir, err := getHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "scw"), nil
}

// getHomeDir returns the path to your home directory
func getHomeDir() (string, error) {
	switch {
	case os.Getenv(unixHomeDirEnv) != "":
		return os.Getenv(unixHomeDirEnv), nil
	case os.Getenv(windowsHomeDirEnv) != "":
		return os.Getenv(windowsHomeDirEnv), nil
	default:
		return "", ErrNoHomeDir
	}
}

func fileExist(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}
