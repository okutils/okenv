package platform

import "fmt"

type Condition struct {
	OS   string   `json:"os"`
	Arch []string `json:"arch"`
}

func (condition Condition) IsMatch(operatingSystem, architecture string) bool {
	if condition.OS != operatingSystem {
		return false
	}
	for _, supportedArchitecture := range condition.Arch {
		if supportedArchitecture == architecture {
			return true
		}
	}
	return false
}

// Validate adds the condition to the owning override list's set.
func (condition Condition) Validate(seen map[string]bool) error {
	if len(condition.Arch) == 0 {
		return fmt.Errorf("arch must contain at least one architecture")
	}
	for _, supportedArchitecture := range condition.Arch {
		var isSupported bool
		switch condition.OS {
		case "windows", "linux":
			isSupported = supportedArchitecture == "arm64" || supportedArchitecture == "amd64"
		case "darwin":
			isSupported = supportedArchitecture == "arm64"
		}
		if !isSupported {
			return fmt.Errorf("unsupported platform %q/%q", condition.OS, supportedArchitecture)
		}
		entryName := condition.OS + "/" + supportedArchitecture
		if seen[entryName] {
			return fmt.Errorf("overlapping platform %q", entryName)
		}
		seen[entryName] = true
	}
	return nil
}
