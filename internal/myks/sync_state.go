package myks

import (
	"fmt"
	"sort"
)

// unsyncedReason reports why the vendored sources of an application look out of sync, or "" when they look fine.
// It only inspects the files the last vendir sync left behind, so a vendir config changed since that sync goes unnoticed.
func (a *Application) unsyncedReason() (string, error) {
	vendirFiles, err := a.vendirSourceFiles()
	if err != nil {
		return "", err
	}
	if len(vendirFiles) == 0 {
		return "", nil
	}

	linksMap, err := a.getLinksMap()
	if err != nil {
		return "", fmt.Errorf("reading vendir links map: %w", err)
	}
	// A valid vendir config always has contents, so an empty links map means no sync has ever run.
	if len(linksMap) == 0 {
		return "vendir sources were never synced", nil
	}

	var missing []string
	for vendorPath := range linksMap {
		// isExist follows the symlink, so a link to a removed cache entry counts as missing.
		ok, err := isExist(a.expandVendorPath(vendorPath))
		if err != nil {
			return "", err
		}
		if !ok {
			missing = append(missing, vendorPath)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Sprintf("vendored sources are missing: %v", missing), nil
	}
	return "", nil
}
