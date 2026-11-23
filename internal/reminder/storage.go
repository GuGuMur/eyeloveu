package reminder

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "eyeloveu", "settings.json"), err
}

func loadConfig(path string) (Config, error) {
	config := DefaultConfig()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, config.Validate()
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".eyeloveu-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, writeErr := tmp.Write(data)
	if err := errors.Join(writeErr, tmp.Close()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func autostartPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "autostart", AppID+".desktop"), err
}

func autostartEnabled() bool {
	path, err := autostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func setAutostart(enabled bool) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	// The desktop-entry and Exec grammars each have their own escaping layer.
	arg := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", "\\`", "%", "%%").Replace(binary)
	arg = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(arg)
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=EyeLoveU\nExec=\"%s\"\nNoDisplay=true\n", arg)
	return atomicWrite(path, []byte(entry))
}
