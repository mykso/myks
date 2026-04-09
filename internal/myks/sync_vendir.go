package myks

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	vendirconf "carvel.dev/vendir/pkg/vendir/config"
	"github.com/mykso/myks/internal/locker"
	"github.com/rs/zerolog/log"
	goyaml "gopkg.in/yaml.v3"
	"sigs.k8s.io/yaml"
)

const (
	VendirCacheDataDirName = "data"
	vendirConfigKindConfig = "Config"
)

// syncResult tracks the outcome of a vendir sync for a single cache entry.
// Used by the singleflight deduplication to let waiting goroutines reuse the result.
type syncResult struct {
	done chan struct{}
	err  error
}

// VendirSyncer is responsible for syncing application dependencies defined in vendir configuration.
type VendirSyncer struct {
	ident  string
	locker *locker.Locker
	// syncedCaches tracks cache entries being synced or already synced in this run.
	// Key: cache name, Value: *syncResult
	syncedCaches sync.Map
	// lazyCaches tracks whether each cache entry has lazy: true.
	// Key: cache name, Value: bool
	lazyCaches sync.Map
	// cacheConfigResults tracks per-cache vendir config writes in-flight or completed this run.
	// Key: cache name, Value: *syncResult
	// Used to ensure each cache's vendir.yaml is written exactly once across all concurrent apps.
	cacheConfigResults sync.Map

	// Dedup counters for observability
	SyncExecuted        atomic.Int64
	SyncSkippedInRun    atomic.Int64
	SyncSkippedCached   atomic.Int64
	ConfigWriteExecuted atomic.Int64
	ConfigWriteSkipped  atomic.Int64
}

// NewVendirSyncer creates a new VendirSyncer with the default identifier and the provided locker.
func NewVendirSyncer(lock *locker.Locker) *VendirSyncer {
	return &VendirSyncer{
		ident:  "vendir",
		locker: lock,
	}
}

func (v *VendirSyncer) Ident() string {
	return v.ident
}

// GetDedupStats returns a snapshot of the dedup counters.
func (v *VendirSyncer) GetDedupStats() *VendirDedupStats {
	return &VendirDedupStats{
		Executed:            v.SyncExecuted.Load(),
		SkippedInRun:        v.SyncSkippedInRun.Load(),
		SkippedCached:       v.SyncSkippedCached.Load(),
		ConfigWriteExecuted: v.ConfigWriteExecuted.Load(),
		ConfigWriteSkipped:  v.ConfigWriteSkipped.Load(),
	}
}

func (v *VendirSyncer) Sync(a *Application, vendirSecrets string) error {
	if err := v.renderVendirConfig(a); errors.Is(err, ErrNoVendirConfig) {
		log.Info().Msg(a.Msg(v.getStepName(), "No vendir config found"))
		return nil
	} else if err != nil {
		return fmt.Errorf("rendering vendir config for %s: %w", a.Name, err)
	}

	if err := v.extractCacheItems(a); err != nil {
		return fmt.Errorf("extracting cache items for %s: %w", a.Name, err)
	}

	if err := v.doSync(a, vendirSecrets); err != nil {
		return fmt.Errorf("syncing %s: %w", a.Name, err)
	}
	log.Info().Msg(a.Msg(v.getStepName(), "Synced"))
	return nil
}

// creates vendir yaml file
func (v *VendirSyncer) renderVendirConfig(a *Application) error {
	// Collect ytt arguments following the following steps:
	// 1. If exists, use the `apps/<prototype>/vendir` directory.
	// 2. If exists, for every level of environments use `<env>/_apps/<app>/vendir` directory.

	var yttFiles []string

	protoVendirDir := filepath.Join(a.Prototype, "vendir")
	if ok, err := isExist(protoVendirDir); err != nil {
		return fmt.Errorf("checking prototype vendir dir %s: %w", protoVendirDir, err)
	} else if ok {
		yttFiles = append(yttFiles, protoVendirDir)
	}

	appVendirDirs := a.e.collectBySubpath(filepath.Join(a.cfg.AppsDir, a.Name, "vendir"))
	yttFiles = append(yttFiles, appVendirDirs...)

	if len(yttFiles) == 0 {
		return ErrNoVendirConfig
	}

	baseDir := filepath.Join(a.cfg.PrototypesDir, "_vendir")
	if ok, err := isExist(baseDir); err != nil {
		return fmt.Errorf("checking vendir base dir %s: %w", baseDir, err)
	} else if ok {
		yttFiles = slices.Insert(yttFiles, 0, baseDir)
	}

	// add environment, prototype, and application data files
	yttFiles = slices.Insert(yttFiles, 0, a.yttDataFiles...)

	// create vendir config yaml
	vendirConfig, err := a.yttS(v.getStepName(), "creating vendir config", yttFiles, nil)
	if err != nil {
		return fmt.Errorf("rendering vendir config via ytt: %w", err)
	}

	if vendirConfig.Stdout == "" {
		return errors.New("rendered empty vendir config")
	}

	if err := validateVendirConfig(vendirConfig.Stdout); err != nil {
		return fmt.Errorf("invalid vendir config: %w", err)
	}

	vendirConfigFilePath := a.expandServicePath(a.cfg.VendirConfigFileName)
	// Create directory if it does not exist
	if err := createDirectory(filepath.Dir(vendirConfigFilePath)); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create directory for vendir config file"))
		return err
	}

	if err := os.WriteFile(vendirConfigFilePath, []byte(vendirConfig.Stdout), 0o600); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config file"))
		return err
	}

	return nil
}

func (v *VendirSyncer) doSync(a *Application, vendirSecrets string) error {
	linksMap, err := a.getLinksMap()
	if err != nil {
		return fmt.Errorf("reading vendir links map: %w", err)
	}

	if err := os.RemoveAll(a.expandVendorPath("")); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to remove vendor directory"))
		return err
	}

	for contentPath, cacheName := range linksMap {
		if err := v.syncCacheEntry(a, cacheName, vendirSecrets); err != nil {
			return err
		}
		if err := v.linkVendorToCache(a, contentPath, cacheName); err != nil {
			log.Error().Err(err).Msg(a.Msg(v.getStepName(), "Unable to create link to cache"))
			return err
		}
	}

	return nil
}

// isCachePopulated checks if a cache entry already has synced data from a previous run.
func (v *VendirSyncer) isCachePopulated(a *Application, cacheName string) bool {
	cacheDir := a.expandVendirCache(cacheName)
	lockPath := filepath.Join(cacheDir, a.cfg.VendirLockFileName)
	dataDir := filepath.Join(cacheDir, VendirCacheDataDirName)

	lockStat, err := os.Stat(lockPath)
	if err != nil || !lockStat.Mode().IsRegular() {
		return false
	}
	dataStat, err := os.Stat(dataDir)
	if err != nil || !dataStat.IsDir() {
		return false
	}
	return true
}

// syncCacheEntry ensures a cache entry is synced, using two levels of deduplication:
// 1. Within-run: only one goroutine syncs each cache entry; others wait and reuse the result.
// 2. Cross-run: if the cache is already populated on disk and the content is lazy, skip vendir entirely.
func (v *VendirSyncer) syncCacheEntry(a *Application, cacheName, vendirSecrets string) error {
	// Within-run dedup: try to claim ownership of this cache entry
	result := &syncResult{done: make(chan struct{})}
	if existing, loaded := v.syncedCaches.LoadOrStore(cacheName, result); loaded {
		existingResult := existing.(*syncResult)
		<-existingResult.done
		v.SyncSkippedInRun.Add(1)
		if existingResult.err != nil {
			log.Debug().Str("cache", cacheName).Msg(a.Msg(v.getStepName(), "Skipped vendir sync (failed by another app)"))
		} else {
			log.Debug().Str("cache", cacheName).Msg(a.Msg(v.getStepName(), "Skipped vendir sync (already synced this run)"))
		}
		return existingResult.err
	}

	// We own this cache entry — perform the sync
	defer close(result.done)

	// Cross-run dedup: check if cache is already populated on disk
	lazyVal, _ := v.lazyCaches.Load(cacheName)
	isLazy, _ := lazyVal.(bool)
	if isLazy && v.isCachePopulated(a, cacheName) {
		v.SyncSkippedCached.Add(1)
		log.Debug().Str("cache", cacheName).Msg(a.Msg(v.getStepName(), "Skipped vendir sync (cache already populated)"))
		return nil
	}

	// Acquire lock and run vendir
	unlock := v.locker.LockNames(slices.Values([]string{cacheName}), true)
	defer unlock()

	cacheDir := a.expandVendirCache(cacheName)
	vendirConfigPath := filepath.Join(cacheDir, a.cfg.VendirConfigFileName)
	vendirLockPath := filepath.Join(cacheDir, a.cfg.VendirLockFileName)

	v.SyncExecuted.Add(1)
	syncErr := v.runVendirSync(a, vendirConfigPath, vendirLockPath, vendirSecrets)

	if syncErr != nil {
		log.Error().Err(syncErr).Msg(a.Msg(v.getStepName(), "Vendir sync failed, cleaning up the cache entry"))
		if e := os.RemoveAll(cacheDir); e != nil {
			log.Warn().Err(e).Msg(a.Msg(v.getStepName(), "Unable to remove cache directory"))
		}
		result.err = syncErr
		return syncErr
	}

	return nil
}

func (v *VendirSyncer) linkVendorToCache(a *Application, vendorPath, cacheName string) error {
	linkFullPath := a.expandVendorPath(vendorPath)
	linkDir := filepath.Dir(linkFullPath)
	cacheDataPath := path.Join(a.expandVendirCache(cacheName), VendirCacheDataDirName)

	relCacheDataPath, err := filepath.Rel(linkDir, cacheDataPath)
	if err != nil {
		return fmt.Errorf("computing relative cache path from %s to %s: %w", linkDir, cacheDataPath, err)
	}

	if err := createDirectory(linkDir); err != nil {
		return fmt.Errorf("creating link directory %s: %w", linkDir, err)
	}

	if err := os.Symlink(relCacheDataPath, linkFullPath); err != nil {
		return fmt.Errorf("creating symlink %s -> %s: %w", linkFullPath, relCacheDataPath, err)
	}
	return nil
}

func (v *VendirSyncer) runVendirSync(a *Application, vendirConfig, vendirLock, vendirSecrets string) error {
	// TODO sync retry - maybe as vendir MR
	args := []string{
		"vendir",
		"sync",
		"--file=" + vendirConfig,
		"--lock-file=" + vendirLock,
		"--file=-",
	}
	_, err := a.runCmd(v.getStepName(), "vendir sync", myksFullPath(), strings.NewReader(vendirSecrets), args)
	return err
}

func (v *VendirSyncer) getStepName() string {
	return fmt.Sprintf("%s-%s", syncStepName, v.Ident())
}

func (v *VendirSyncer) extractCacheItems(a *Application) error {
	configPath := a.expandServicePath(a.cfg.VendirConfigFileName)
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read vendir config: %w", err)
	}

	// Unmarshal directly without validation (NewConfigFromBytes validates which
	// may reject certain valid-for-our-use configs like those with "." paths)
	var vendirConfig vendirconf.Config
	if err := yaml.Unmarshal(configBytes, &vendirConfig); err != nil {
		return fmt.Errorf("failed to parse vendir config: %w", err)
	}

	vendorDirToCacheMap := map[string]string{}
	cacheVendirConfigs := map[string]vendirconf.Config{}

	for _, dir := range vendirConfig.Directories {
		for i := range dir.Contents {
			content := &dir.Contents[i]
			vendorDirPath := filepath.Join(dir.Path, content.Path)
			cacheName, err := genCacheName(*content)
			if err != nil {
				return err
			}
			vendorDirToCacheMap[vendorDirPath] = cacheName
			cacheDir := a.expandVendirCache(cacheName)
			cacheVendirConfigs[cacheName] = buildCacheVendirConfig(cacheDir, vendirConfig, dir, content)
			v.lazyCaches.Store(cacheName, content.Lazy)
		}
	}

	// Save the per-app links map first — it writes to an app-specific path,
	// so no cross-app coordination is needed.
	if err := v.saveLinksMap(a, vendorDirToCacheMap); err != nil {
		return err
	}

	// Write per-cache vendir configs using a singleflight: the first goroutine to
	// encounter a given cache name writes the file; others wait and reuse the result.
	// This eliminates the previous bulk write lock, which serialized all apps sharing
	// any common cache entry even though they produce identical config content.
	for cacheName, cacheVendirConfig := range cacheVendirConfigs {
		if err := v.ensureCacheConfig(a, cacheName, cacheVendirConfig); err != nil {
			return err
		}
	}

	return nil
}

// ensureCacheConfig writes the per-cache vendir config file exactly once per run using a
// singleflight pattern. The first goroutine per cache name writes the file; subsequent
// goroutines wait for the write to complete and reuse the result (success or error).
func (v *VendirSyncer) ensureCacheConfig(a *Application, cacheName string, config vendirconf.Config) error {
	result := &syncResult{done: make(chan struct{})}
	if existing, loaded := v.cacheConfigResults.LoadOrStore(cacheName, result); loaded {
		existingResult := existing.(*syncResult)
		<-existingResult.done
		v.ConfigWriteSkipped.Add(1)
		return existingResult.err
	}
	defer close(result.done)

	v.ConfigWriteExecuted.Add(1)
	result.err = v.saveCacheVendirConfig(a, cacheName, config)
	return result.err
}

func (v *VendirSyncer) saveCacheVendirConfig(a *Application, cacheName string, vendirConfig vendirconf.Config) error {
	data, err := vendirConfig.AsBytes()
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal vendir config"))
		return err
	}
	vendirConfigPath := filepath.Join(a.expandVendirCache(cacheName), a.cfg.VendirConfigFileName)
	if err = writeFile(vendirConfigPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write vendir config"))
		return err
	}
	return nil
}

func buildCacheVendirConfig(cacheDir string, vendirConfig vendirconf.Config, vendirDir vendirconf.Directory, vendirContent *vendirconf.DirectoryContents) vendirconf.Config {
	// Create a copy of the content with path set to "."
	contentCopy := *vendirContent
	contentCopy.Path = "."

	// Ensure required fields have default values
	apiVersion := vendirConfig.APIVersion
	if apiVersion == "" {
		apiVersion = vendirAPIVersion
	}
	kind := vendirConfig.Kind
	if kind == "" {
		kind = vendirConfigKindConfig
	}

	newConfig := vendirconf.Config{
		APIVersion:             apiVersion,
		Kind:                   kind,
		MinimumRequiredVersion: vendirConfig.MinimumRequiredVersion,
		Directories: []vendirconf.Directory{
			{
				Path:        filepath.Join(cacheDir, VendirCacheDataDirName),
				Permissions: vendirDir.Permissions,
				Contents:    []vendirconf.DirectoryContents{contentCopy},
			},
		},
	}
	return newConfig
}

const vendirAPIVersion = "vendir.k14s.io/v1alpha1"

// validateVendirConfig validates the rendered vendir configuration YAML.
// It checks for:
// - Single YAML document (no multiple documents separated by ---)
// - Required apiVersion field with correct value
// - Required kind field with correct value
// - At least one directory defined
// - Each directory has a path and at least one content entry
func validateVendirConfig(configYaml string) error {
	// Check for multiple YAML documents
	// A document separator is "\n---" which indicates the start of a new document after content
	// We don't allow any document separators (only one document allowed)
	if strings.Contains(configYaml, "\n---") {
		return errors.New("vendir config contains multiple YAML documents, expected exactly 1")
	}

	// Parse and validate structure
	var config vendirconf.Config
	if err := yaml.Unmarshal([]byte(configYaml), &config); err != nil {
		return fmt.Errorf("failed to parse vendir config: %w", err)
	}

	// Validate apiVersion
	if config.APIVersion == "" {
		return errors.New("vendir config missing required field: apiVersion")
	}
	if config.APIVersion != vendirAPIVersion {
		return fmt.Errorf("vendir config has invalid apiVersion: %q, expected %q", config.APIVersion, vendirAPIVersion)
	}

	// Validate kind
	if config.Kind == "" {
		return errors.New("vendir config missing required field: kind")
	}
	if config.Kind != vendirConfigKindConfig {
		return fmt.Errorf("vendir config has invalid kind: %q, expected %q", config.Kind, vendirConfigKindConfig)
	}

	// Validate directories
	if len(config.Directories) == 0 {
		return errors.New("vendir config has no directories defined")
	}

	for i, dir := range config.Directories {
		if dir.Path == "" {
			return fmt.Errorf("vendir config directory[%d] missing required field: path", i)
		}
		if len(dir.Contents) == 0 {
			return fmt.Errorf("vendir config directory[%d] (%s) has no contents defined", i, dir.Path)
		}
	}

	return nil
}

func (a *Application) getLinksMap() (map[string]string, error) {
	if a.linksMap == nil {
		linksMapRaw, err := unmarshalYamlToMap(a.getLinksMapPath())
		if err != nil {
			return nil, err
		}
		a.linksMap = make(map[string]string, len(linksMapRaw))
		for k, v := range linksMapRaw {
			a.linksMap[k] = v.(string)
		}
	}
	return a.linksMap, nil
}

func (a *Application) getLinksMapPath() string {
	return a.expandServicePath(a.cfg.VendirLinksMapFileName)
}

func (v *VendirSyncer) saveLinksMap(a *Application, linksMap map[string]string) error {
	linksMapPath := a.getLinksMapPath()
	data, err := goyaml.Marshal(linksMap)
	if err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to marshal links map"))
		return err
	}
	if err = writeFile(linksMapPath, data); err != nil {
		log.Warn().Err(err).Msg(a.Msg(v.getStepName(), "Unable to write links map"))
		return err
	}
	a.linksMap = linksMap
	return nil
}
