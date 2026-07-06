package report

type Report struct {
	Header       ReportHeader        `json:"header" yaml:"header"`
	Overview     RecoveryOverview    `json:"overview" yaml:"overview"`
	Summary      ExecutiveSummary    `json:"summary" yaml:"summary"`
	Machine      MachineProfile      `json:"machine" yaml:"machine"`
	Development  DevelopmentStack    `json:"development" yaml:"development"`
	Identity     IdentitySection     `json:"identity" yaml:"identity"`
	Browsers     []BrowserInfo       `json:"browsers" yaml:"browsers"`
	Editors      []EditorInfo        `json:"editors" yaml:"editors"`
	Databases    []DatabaseInfo      `json:"databases" yaml:"databases"`
	Containers   *ContainerInfo      `json:"containers,omitempty" yaml:"containers,omitempty"`
	Cloud        *CloudInfo          `json:"cloud,omitempty" yaml:"cloud,omitempty"`
	Infra        *InfrastructureInfo `json:"infrastructure,omitempty" yaml:"infrastructure,omitempty"`
	Packages     PackageInfo         `json:"packages" yaml:"packages"`
	Security     *SecurityInfo       `json:"security,omitempty" yaml:"security,omitempty"`
	Projects     *ProjectsInfo       `json:"projects,omitempty" yaml:"projects,omitempty"`
	Virtualization *VirtualizationInfo `json:"virtualization,omitempty" yaml:"virtualization,omitempty"`
	Backups      BackupSummary       `json:"backups" yaml:"backups"`
	Gaps         []RecoveryGap       `json:"recovery_gaps" yaml:"recovery_gaps"`
	Statistics   AssetStats          `json:"statistics" yaml:"statistics"`
	NotDetected  []string            `json:"not_detected" yaml:"not_detected"`
	Coverage     CoverageInfo        `json:"coverage" yaml:"coverage"`
	Verdict      RecoveryVerdict     `json:"verdict" yaml:"verdict"`
	Meta         ReportMetadata      `json:"metadata" yaml:"metadata"`
}

type RecoveryOverview struct {
	ConfidenceScore  int    `json:"confidence_score" yaml:"confidence_score"`
	ConfidenceGrade  string `json:"confidence_grade" yaml:"confidence_grade"`
	MachineStatus    string `json:"machine_status" yaml:"machine_status"`
	ProtectedAssets  int    `json:"protected_assets" yaml:"protected_assets"`
	UnprotectedAssets int   `json:"unprotected_assets" yaml:"unprotected_assets"`
	LatestBackup     string `json:"latest_backup,omitempty" yaml:"latest_backup,omitempty"`
	BackupCount     int    `json:"backup_count" yaml:"backup_count"`
	HighestRisks    []string `json:"highest_risks" yaml:"highest_risks"`
}

type RecoveryGap struct {
	Name     string `json:"name" yaml:"name"`
	Category string `json:"category" yaml:"category"`
	Issue    string `json:"issue" yaml:"issue"`
}

type RecoveryVerdict struct {
	Summary       string   `json:"summary" yaml:"summary"`
	Confidence    int      `json:"confidence" yaml:"confidence"`
	TargetConfidence int   `json:"target_confidence" yaml:"target_confidence"`
	CriticalActions []string `json:"critical_actions" yaml:"critical_actions"`
}

type ReportHeader struct {
	Hostname        string `json:"hostname" yaml:"hostname"`
	GeneratedAt     string `json:"generated_at" yaml:"generated_at"`
	GeneratedAtUnix int64  `json:"generated_at_unix" yaml:"generated_at_unix"`
	OperatingSystem string `json:"operating_system" yaml:"operating_system"`
	GetItBackVersion string `json:"getitback_version" yaml:"getitback_version"`
	RecoverySnapshot string `json:"recovery_snapshot,omitempty" yaml:"recovery_snapshot,omitempty"`
	SnapshotCount   int    `json:"snapshot_count" yaml:"snapshot_count"`
}

type ExecutiveSummary struct {
	OperatingSystem  string   `json:"operating_system" yaml:"operating_system"`
	PrimaryShell     string   `json:"primary_shell,omitempty" yaml:"primary_shell,omitempty"`
	PrimaryEditor    string   `json:"primary_editor,omitempty" yaml:"primary_editor,omitempty"`
	PrimaryBrowser   string   `json:"primary_browser,omitempty" yaml:"primary_browser,omitempty"`
	Languages        []string `json:"languages" yaml:"languages"`
	Containers       int      `json:"containers" yaml:"containers"`
	DockerVolumes    int      `json:"docker_volumes" yaml:"docker_volumes"`
	Repositories     int      `json:"repositories" yaml:"repositories"`
	CloudProviders   []string `json:"cloud_providers" yaml:"cloud_providers"`
	BackupSnapshots  int      `json:"backup_snapshots" yaml:"backup_snapshots"`
	Databases        []string `json:"databases" yaml:"databases"`
}

type MachineProfile struct {
	Architecture string `json:"architecture,omitempty" yaml:"architecture,omitempty"`
	CPU          string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	RAM          string `json:"ram,omitempty" yaml:"ram,omitempty"`
	Storage      string `json:"storage,omitempty" yaml:"storage,omitempty"`
	StorageUsed  string `json:"storage_used,omitempty" yaml:"storage_used,omitempty"`
	StorageAvail string `json:"storage_avail,omitempty" yaml:"storage_avail,omitempty"`
	Kernel       string `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	Desktop      string `json:"desktop,omitempty" yaml:"desktop,omitempty"`
	Session      string `json:"session,omitempty" yaml:"session,omitempty"`
	Timezone     string `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	Locale       string `json:"locale,omitempty" yaml:"locale,omitempty"`
}

type DevelopmentStack struct {
	Languages      []LanguageInfo      `json:"languages" yaml:"languages"`
	VersionControl []VCInfo            `json:"version_control" yaml:"version_control"`
	Editors        []EditorInfo        `json:"editors" yaml:"editors"`
	PackageManagers []PackageManagerInfo `json:"package_managers" yaml:"package_managers"`
}

type LanguageInfo struct {
	Name         string `json:"name" yaml:"name"`
	Version      string `json:"version,omitempty" yaml:"version,omitempty"`
	PackageCount int    `json:"package_count,omitempty" yaml:"package_count,omitempty"`
	GlobalPkgs   int    `json:"global_packages,omitempty" yaml:"global_packages,omitempty"`
}

type VCInfo struct {
	Name      string `json:"name" yaml:"name"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	Configured bool   `json:"configured" yaml:"configured"`
	Details   string `json:"details,omitempty" yaml:"details,omitempty"`
}

type IdentitySection struct {
	SSH *SSHInfo `json:"ssh,omitempty" yaml:"ssh,omitempty"`
	GPG *GPGInfo `json:"gpg,omitempty" yaml:"gpg,omitempty"`
}

type SSHInfo struct {
	Version       string `json:"version,omitempty" yaml:"version,omitempty"`
	IdentityCount int    `json:"identity_count" yaml:"identity_count"`
	Keys          int    `json:"keys" yaml:"keys"`
	BackedUp      bool   `json:"backed_up" yaml:"backed_up"`
}

type GPGInfo struct {
	Version     string `json:"version,omitempty" yaml:"version,omitempty"`
	KeyCount    int    `json:"key_count" yaml:"key_count"`
	BackedUp    bool   `json:"backed_up" yaml:"backed_up"`
}

type BrowserInfo struct {
	Name           string `json:"name" yaml:"name"`
	Version        string `json:"version,omitempty" yaml:"version,omitempty"`
	ProfileCount   int    `json:"profile_count" yaml:"profile_count"`
	DefaultProfile string `json:"default_profile,omitempty" yaml:"default_profile,omitempty"`
	Storage        string `json:"storage,omitempty" yaml:"storage,omitempty"`
	InstallMethod  string `json:"install_method,omitempty" yaml:"install_method,omitempty"`
	BackedUp       bool   `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel  int    `json:"recovery_level" yaml:"recovery_level"`
}

type EditorInfo struct {
	Name         string `json:"name" yaml:"name"`
	Version      string `json:"version,omitempty" yaml:"version,omitempty"`
	Extensions   int    `json:"extensions,omitempty" yaml:"extensions,omitempty"`
	Settings     int    `json:"settings,omitempty" yaml:"settings,omitempty"`
	Themes       int    `json:"themes,omitempty" yaml:"themes,omitempty"`
	Snippets     int    `json:"snippets,omitempty" yaml:"snippets,omitempty"`
	BackedUp     bool   `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel int   `json:"recovery_level" yaml:"recovery_level"`
}

type PackageManagerInfo struct {
	Name      string `json:"name" yaml:"name"`
	Version   string `json:"version,omitempty" yaml:"version,omitempty"`
	Count     int    `json:"count" yaml:"count"`
}

type DatabaseInfo struct {
	Name        string `json:"name" yaml:"name"`
	Version     string `json:"version,omitempty" yaml:"version,omitempty"`
	Databases   int    `json:"databases" yaml:"databases"`
	DataDir     string `json:"data_dir,omitempty" yaml:"data_dir,omitempty"`
	ConfigFile  string `json:"config_file,omitempty" yaml:"config_file,omitempty"`
	Storage     string `json:"storage,omitempty" yaml:"storage,omitempty"`
	BackedUp    bool   `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel int  `json:"recovery_level" yaml:"recovery_level"`
}

type ContainerInfo struct {
	Version         string `json:"version,omitempty" yaml:"version,omitempty"`
	Containers      int    `json:"containers" yaml:"containers"`
	Running         int    `json:"running" yaml:"running"`
	Stopped         int    `json:"stopped" yaml:"stopped"`
	Images          int    `json:"images" yaml:"images"`
	Volumes         int    `json:"volumes" yaml:"volumes"`
	Networks        int    `json:"networks" yaml:"networks"`
	CustomNetworks  int    `json:"custom_networks" yaml:"custom_networks"`
	ComposeProjects int    `json:"compose_projects" yaml:"compose_projects"`
	BuildCache      string `json:"build_cache,omitempty" yaml:"build_cache,omitempty"`
	DanglingImages  int    `json:"dangling_images" yaml:"dangling_images"`
	ImageStorage    string `json:"image_storage,omitempty" yaml:"image_storage,omitempty"`
	VolumeStorage   string `json:"volume_storage,omitempty" yaml:"volume_storage,omitempty"`
	RootDir         string `json:"root_dir,omitempty" yaml:"root_dir,omitempty"`
	Rootless        bool   `json:"rootless" yaml:"rootless"`
	BackedUp        bool   `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel   int    `json:"recovery_level" yaml:"recovery_level"`
}

type CloudInfo struct {
	Providers []CloudProviderInfo `json:"providers" yaml:"providers"`
}

type CloudProviderInfo struct {
	Name          string `json:"name" yaml:"name"`
	CliInstalled  bool   `json:"cli_installed" yaml:"cli_installed"`
	Authenticated bool   `json:"authenticated" yaml:"authenticated"`
	AccountID     string `json:"account_id,omitempty" yaml:"account_id,omitempty"`
	Credentials   bool   `json:"credentials" yaml:"credentials"`
	BackedUp      bool   `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel int    `json:"recovery_level" yaml:"recovery_level"`
}

type InfrastructureInfo struct {
	Kubernetes       *KubernetesInfo `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Tools            []string        `json:"tools" yaml:"tools"`
	BackedUp         bool            `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel    int             `json:"recovery_level" yaml:"recovery_level"`
	KubeconfigFound  bool            `json:"kubeconfig_found" yaml:"kubeconfig_found"`
}

type KubernetesInfo struct {
	Version        string   `json:"version,omitempty" yaml:"version,omitempty"`
	CurrentContext string   `json:"current_context,omitempty" yaml:"current_context,omitempty"`
	Contexts       []string `json:"contexts" yaml:"contexts"`
	Namespaces     []string `json:"namespaces" yaml:"namespaces"`
	HelmRepos      []string `json:"helm_repos" yaml:"helm_repos"`
}

type PackageInfo struct {
	Apt     *PackageManagerInfo `json:"apt,omitempty" yaml:"apt,omitempty"`
	Snap    *PackageManagerInfo `json:"snap,omitempty" yaml:"snap,omitempty"`
	Flatpak *PackageManagerInfo `json:"flatpak,omitempty" yaml:"flatpak,omitempty"`
}

type SecurityInfo struct {
	CertStores int `json:"certificate_stores" yaml:"certificate_stores"`
	ValidCerts int `json:"valid_certificates" yaml:"valid_certificates"`
	Expiring   int `json:"expiring" yaml:"expiring"`
	Expired    int `json:"expired" yaml:"expired"`
	CABundles  int `json:"custom_ca_bundles" yaml:"custom_ca_bundles"`
}

type ProjectsInfo struct {
	TotalRepos  int `json:"total_repositories" yaml:"total_repositories"`
	DirtyRepos  int `json:"dirty_repositories" yaml:"dirty_repositories"`
	NoRemote    int `json:"without_remote" yaml:"without_remote"`
	GitHubRepos int `json:"github_repos" yaml:"github_repos"`
	GitLabRepos int `json:"gitlab_repos" yaml:"gitlab_repos"`
	LocalOnly   int `json:"local_only" yaml:"local_only"`
}

type VirtualizationInfo struct {
	Platforms     []string `json:"platforms" yaml:"platforms"`
	BackedUp      bool     `json:"backed_up" yaml:"backed_up"`
	RecoveryLevel int      `json:"recovery_level" yaml:"recovery_level"`
}

type BackupSummary struct {
	LatestSnapshot  string `json:"latest_snapshot,omitempty" yaml:"latest_snapshot,omitempty"`
	CreatedAt       string `json:"created_at,omitempty" yaml:"created_at,omitempty"`
	SnapshotCount   int    `json:"snapshot_count" yaml:"snapshot_count"`
	TotalSize       string `json:"total_size,omitempty" yaml:"total_size,omitempty"`
	Encryption      string `json:"encryption" yaml:"encryption"`
	StorageProvider string `json:"storage_provider" yaml:"storage_provider"`
	RecoverableCount int   `json:"recoverable_modules" yaml:"recoverable_modules"`
	TotalCount      int    `json:"total_detected_modules" yaml:"total_detected_modules"`
	RestoreTime     string `json:"estimated_restore_time,omitempty" yaml:"estimated_restore_time,omitempty"`
}

type AssetStats struct {
	Languages       int `json:"languages" yaml:"languages"`
	Browsers        int `json:"browsers" yaml:"browsers"`
	Editors         int `json:"editors" yaml:"editors"`
	Databases       int `json:"databases" yaml:"databases"`
	Containers      int `json:"containers" yaml:"containers"`
	DockerVolumes   int `json:"docker_volumes" yaml:"docker_volumes"`
	Repositories    int `json:"repositories" yaml:"repositories"`
	Certificates    int `json:"certificates" yaml:"certificates"`
	SSHKeys         int `json:"ssh_keys" yaml:"ssh_keys"`
	GPGKeys         int `json:"gpg_keys" yaml:"gpg_keys"`
	CloudProviders  int `json:"cloud_providers" yaml:"cloud_providers"`
	ComposeProjects int `json:"compose_projects" yaml:"compose_projects"`
}

type CoverageInfo struct {
	DetectedModules int    `json:"detected_modules" yaml:"detected_modules"`
	TotalModules    int    `json:"total_modules" yaml:"total_modules"`
	MissingModules  int    `json:"missing_modules" yaml:"missing_modules"`
	CoveragePercent int    `json:"coverage_percent" yaml:"coverage_percent"`
}

type ReportMetadata struct {
	GeneratedBy string `json:"generated_by" yaml:"generated_by"`
	Version     string `json:"version" yaml:"version"`
	GeneratedAt string `json:"generated_at" yaml:"generated_at"`
	MachineID   string `json:"machine_id,omitempty" yaml:"machine_id,omitempty"`
	Checksum    string `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	Format      string `json:"format" yaml:"format"`
}
