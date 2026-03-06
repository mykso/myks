package myks

// Config contains naming conventions and paths for the myks application.
type Config struct {
	// Global vendir cache dir
	VendirCache string `default:"vendir-cache"`
	// Project root directory
	RootDir string `default:"."`
	// Base directory for environments
	EnvironmentBaseDir string `default:"envs" mapstructure:"environment-base-dir"`
	// Application prototypes directory
	PrototypesDir string `default:"prototypes" mapstructure:"prototypes-dir"`
	// Ytt library directory name
	YttLibraryDirName string `default:"lib" mapstructure:"ytt-library-dir-name"`
	// Rendered kubernetes manifests directory
	RenderedEnvsDir string `default:"rendered/envs" mapstructure:"rendered-envs-dir"`
	// Rendered argocd manifests directory
	RenderedArgoDir string `default:"rendered/argocd" mapstructure:"rendered-argo-dir"`

	// Directory of application-specific configuration
	AppsDir string `default:"_apps" mapstructure:"apps-dir"`
	// Directory of environment-specific configuration
	EnvsDir string `default:"_env" mapstructure:"envs-dir"`
	// Directory of application-specific prototype overwrites
	PrototypeOverrideDir string `default:"_proto" mapstructure:"prototype-override-dir"`

	// Data values schema file name
	DataSchemaFileName string `default:"data-schema.ytt.yaml"`
	// Application data file name
	ApplicationDataFileName string `default:"app-data*.yaml" mapstructure:"application-data-file-name"`
	// Environment data file name
	EnvironmentDataFileName string `default:"env-data*.yaml" mapstructure:"environment-data-file-name"`
	// Rendered environment data file path, keep in sync with YttLibApiDir
	RenderedEnvironmentDataLibFileName string `default:"_api/_ytt_lib/myks/data.lib.yaml"`
	// Directory where myks' ytt libraries are stored
	YttLibAPIDir string `default:"_api"`
	// Myks runtime data file name
	MyksDataFileName string `default:"myks-data.ytt.yaml"`
	// Service directory name
	ServiceDirName string `default:".myks"`
	// Temporary directory name
	TempDirName string `default:"tmp"`

	// Rendered vendir config file name
	VendirConfigFileName string `default:"vendir.yaml"`
	// Rendered vendir lock file name
	VendirLockFileName string `default:"vendir.lock.yaml"`
	// Name of the file with directory-to-cache-dir mappings
	VendirLinksMapFileName string `default:"vendir-links.yaml"`
	// Prefix for vendir secret environment variables
	VendirSecretEnvPrefix string `default:"VENDIR_SECRET_"`

	// Downloaded third-party sources
	VendorDirName string `default:"vendor"`
	// Helm charts directory name
	HelmChartsDirName string `default:"charts"`

	// Plugin subdirectories
	// ArgoCD data directory name
	ArgoCDDataDirName string `default:"argocd" mapstructure:"plugin-argocd-dir-name"`
	// Helm step directory name
	HelmStepDirName string `default:"helm" mapstructure:"plugin-helm-dir-name"`
	// Static files directory name
	StaticFilesDirName string `default:"static" mapstructure:"plugin-static-dir-name"`
	// Vendir step directory name
	VendirStepDirName string `default:"vendir" mapstructure:"plugin-vendir-dir-name"`
	// Ytt step directory name (deprecated, not used)
	YttPkgStepDirName string `default:"ytt-pkg"`
	// Ytt step directory name
	YttStepDirName string `default:"ytt" mapstructure:"plugin-ytt-dir-name"`

	// Running in a git repository
	WithGit bool
	// Git repository path prefix (non-empty if running in a subdirectory of a git repository)
	GitPathPrefix string
	// Git repository branch
	GitRepoBranch string
	// Git repository URL
	GitRepoURL string

	// Extra ytt file paths
	ExtraYttPaths []string

	Metrics *MetricsManager
}
