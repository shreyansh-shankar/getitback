package module

type MaturityLevel string

const (
	MaturityStable       MaturityLevel = "Stable"
	MaturityBeta         MaturityLevel = "Beta"
	MaturityExperimental MaturityLevel = "Experimental"
)

type Capability string

const (
	CapDetect  Capability = "Detect"
	CapBackup  Capability = "Backup"
	CapRestore Capability = "Restore"
	CapVerify  Capability = "Verify"
	CapDoctor  Capability = "Doctor"
)

type ModuleInfo struct {
	Name          string
	Description   string
	Category      string
	Maturity      MaturityLevel
	Platforms     []string
	Capabilities  []Capability
	DataCollected []string
	Dependencies  []string
	RecoveryValue string
}

var defaultCapabilities = []Capability{CapDetect, CapBackup, CapRestore, CapVerify, CapDoctor}

func DefaultCapabilities() []Capability {
	return defaultCapabilities
}

func GetModuleInfo(name string) *ModuleInfo {
	if info, ok := moduleInfoMap[name]; ok {
		return info
	}
	return nil
}

var moduleInfoMap = map[string]*ModuleInfo{
	"ssh": {
		Name: "SSH", Description: "Secure Shell keys, configuration, and known hosts",
		Category: "Identity", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Private keys (ed25519, RSA, ECDSA)",
			"Public keys",
			"SSH config file",
			"Known hosts",
			"Authorized keys",
		},
		Dependencies:  []string{"OpenSSH client"},
		RecoveryValue: "Critical",
	},
	"gpg": {
		Name: "GPG", Description: "GnuPG encryption keys and configuration",
		Category: "Identity", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Public and private key rings",
			"GPG configuration",
			"Ownertrust database",
		},
		Dependencies:  []string{"GnuPG"},
		RecoveryValue: "Critical",
	},
	"git": {
		Name: "Git", Description: "Git version control configuration and global settings",
		Category: "Configuration", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Global gitconfig",
			"Username and email",
			"Signing key configuration",
			"Git LFS settings",
			"Global gitignore",
		},
		Dependencies:  []string{"Git"},
		RecoveryValue: "Medium",
	},
	"shell": {
		Name: "Shell", Description: "Shell configuration, prompt, and frameworks",
		Category: "Configuration", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Shell rc files (.zshrc, .bashrc)",
			"Prompt configuration",
			"Shell framework (oh-my-zsh, starship)",
			"Environment variables",
		},
		Dependencies:  []string{"Zsh", "Bash", "Fish"},
		RecoveryValue: "Low",
	},
	"dotfiles": {
		Name: "Dotfiles", Description: "User dotfiles and application configuration files",
		Category: "Configuration", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Configuration files in home directory",
			"Application configs",
			"Custom scripts and aliases",
		},
		Dependencies:  nil,
		RecoveryValue: "High",
	},
	"vscode": {
		Name: "VS Code", Description: "Visual Studio Code editor configuration and extensions",
		Category: "Editors", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Installed extensions list",
			"User settings (settings.json)",
			"Keybindings",
			"Snippets",
			"Themes",
			"Workspace configurations",
		},
		Dependencies:  []string{"VS Code"},
		RecoveryValue: "High",
	},
	"neovim": {
		Name: "Neovim", Description: "Neovim editor configuration and plugins",
		Category: "Editors", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Init.lua or init.vim",
			"Plugin manager configuration",
			"Installed plugins",
		},
		Dependencies:  []string{"Neovim"},
		RecoveryValue: "Medium",
	},
	"firefox": {
		Name: "Firefox", Description: "Firefox browser profiles and settings",
		Category: "Browsers", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Extensions",
			"Saved passwords",
			"Preferences",
		},
		Dependencies:  []string{"Firefox"},
		RecoveryValue: "High",
	},
	"chrome": {
		Name: "Chrome", Description: "Google Chrome browser profiles and settings",
		Category: "Browsers", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Extensions",
			"Saved passwords",
			"Preferences",
		},
		Dependencies:  []string{"Google Chrome"},
		RecoveryValue: "High",
	},
	"chromium": {
		Name: "Chromium", Description: "Chromium browser profiles and settings",
		Category: "Browsers", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Extensions",
			"Preferences",
		},
		Dependencies:  []string{"Chromium"},
		RecoveryValue: "Medium",
	},
	"brave": {
		Name: "Brave", Description: "Brave browser profiles and settings",
		Category: "Browsers", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Extensions",
			"Preferences",
		},
		Dependencies:  []string{"Brave"},
		RecoveryValue: "Medium",
	},
	"vivaldi": {
		Name: "Vivaldi", Description: "Vivaldi browser profiles and settings",
		Category: "Browsers", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Preferences",
		},
		Dependencies:  []string{"Vivaldi"},
		RecoveryValue: "Low",
	},
	"edge": {
		Name: "Edge", Description: "Microsoft Edge browser profiles and settings",
		Category: "Browsers", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Preferences",
		},
		Dependencies:  []string{"Microsoft Edge"},
		RecoveryValue: "Low",
	},
	"opera": {
		Name: "Opera", Description: "Opera browser profiles and settings",
		Category: "Browsers", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Browser profiles",
			"Bookmarks",
			"Preferences",
		},
		Dependencies:  []string{"Opera"},
		RecoveryValue: "Low",
	},
	"golang": {
		Name: "Go", Description: "Go programming language toolchain and packages",
		Category: "Development", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Go version and GOROOT",
			"Installed Go packages",
			"GOPATH and workspace configuration",
		},
		Dependencies:  []string{"Go"},
		RecoveryValue: "Low",
	},
	"nodejs": {
		Name: "Node.js", Description: "Node.js runtime, npm packages, and global tools",
		Category: "Development", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Node.js version",
			"Global npm packages",
			"Bun installation",
			"Corepack configuration",
		},
		Dependencies:  []string{"Node.js", "npm"},
		RecoveryValue: "Low",
	},
	"python": {
		Name: "Python", Description: "Python runtime, pip packages, and virtual environments",
		Category: "Development", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Python versions",
			"pip global packages",
			"pipx installed tools",
			"Poetry configuration",
			"UV package manager",
		},
		Dependencies:  []string{"Python", "pip"},
		RecoveryValue: "Low",
	},
	"rust": {
		Name: "Rust", Description: "Rust toolchain, cargo packages, and build cache",
		Category: "Development", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Rust version",
			"Installed cargo tools",
			"Cargo cache size",
		},
		Dependencies:  []string{"Rust", "Cargo"},
		RecoveryValue: "Low",
	},
	"java": {
		Name: "Java", Description: "Java runtime, JDK, and build tools",
		Category: "Development", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"JVM versions (JDK, JRE)",
			"Gradle configuration and cache",
			"Maven configuration and cache",
			"SBT configuration",
		},
		Dependencies:  []string{"JDK", "Gradle", "Maven"},
		RecoveryValue: "Low",
	},
	"docker": {
		Name: "Docker", Description: "Docker Engine, containers, images, volumes, and compose",
		Category: "Containers", Maturity: MaturityStable,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"Docker daemon configuration",
			"Container list and status",
			"Image list and storage",
			"Volume list and data",
			"Network configuration",
			"Compose project files",
			"Build cache information",
		},
		Dependencies:  []string{"Docker CLI", "Docker Engine"},
		RecoveryValue: "Critical",
	},
	"postgres": {
		Name: "PostgreSQL", Description: "PostgreSQL database configuration and data",
		Category: "Databases", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"PostgreSQL version",
			"Database list",
			"Data directory",
			"Configuration files",
		},
		Dependencies:  []string{"PostgreSQL"},
		RecoveryValue: "Critical",
	},
	"mysql": {
		Name: "MySQL", Description: "MySQL/MariaDB database configuration and data",
		Category: "Databases", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"MySQL version",
			"Database list",
			"Data directory",
			"Configuration files",
		},
		Dependencies:  []string{"MySQL", "MariaDB"},
		RecoveryValue: "Critical",
	},
	"mongodb": {
		Name: "MongoDB", Description: "MongoDB database configuration and data",
		Category: "Databases", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"MongoDB version",
			"Database list",
			"Data directory",
			"Configuration files",
		},
		Dependencies:  []string{"MongoDB"},
		RecoveryValue: "Critical",
	},
	"redis": {
		Name: "Redis", Description: "Redis in-memory database configuration",
		Category: "Databases", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Redis version",
			"Configuration files",
			"Data directory",
		},
		Dependencies:  []string{"Redis"},
		RecoveryValue: "High",
	},
	"sqlite": {
		Name: "SQLite", Description: "SQLite database files",
		Category: "Databases", Maturity: MaturityBeta,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"SQLite version",
			"Database file locations",
		},
		Dependencies:  []string{"SQLite"},
		RecoveryValue: "Medium",
	},
	"cloud": {
		Name: "Cloud", Description: "Cloud CLI tools and authentication credentials",
		Category: "Cloud", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"AWS CLI configuration and credentials",
			"Azure CLI configuration",
			"Google Cloud SDK configuration",
			"GitHub CLI authentication",
			"Vercel, Netlify, Cloudflare configs",
			"Fly.io, Railway, Supabase configs",
		},
		Dependencies: []string{
			"AWS CLI", "Azure CLI", "gcloud",
			"GitHub CLI", "Vercel CLI", "Netlify CLI",
		},
		RecoveryValue: "Critical",
	},
	"kubernetes": {
		Name: "Kubernetes", Description: "Kubernetes configuration, contexts, and infrastructure tools",
		Category: "Infrastructure", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"Kubeconfig and current context",
			"Available contexts and namespaces",
			"Helm repositories",
			"Terraform plugin cache",
			"Kind, minikube, and other infra tools",
		},
		Dependencies: []string{"kubectl", "Helm", "Terraform", "Kind"},
		RecoveryValue: "Critical",
	},
	"repos": {
		Name: "Repositories", Description: "Git repository discovery and remote tracking",
		Category: "Projects", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS", "Windows"},
		DataCollected: []string{
			"Git repository locations",
			"Remote providers (GitHub, GitLab)",
			"Uncommitted changes",
			"Repositories without remotes",
		},
		Dependencies:  []string{"Git"},
		RecoveryValue: "High",
	},
	"certs": {
		Name: "Certificates", Description: "SSL/TLS certificate stores and custom CA bundles",
		Category: "Security", Maturity: MaturityBeta,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"System certificate stores",
			"Certificate files",
			"Expired and expiring certificates",
			"Custom CA bundles",
		},
		Dependencies:  nil,
		RecoveryValue: "High",
	},
	"virtualization": {
		Name: "Virtualization", Description: "Virtualization platforms and VM management",
		Category: "Virtualization", Maturity: MaturityBeta,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"Virtualization platform detection",
			"VM configuration",
		},
		Dependencies:  []string{"QEMU", "libvirt", "Vagrant"},
		RecoveryValue: "Low",
	},
	"apt": {
		Name: "APT", Description: "APT package manager and installed packages",
		Category: "Packages", Maturity: MaturityStable,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"Installed package list",
			"Manually installed packages",
			"Additional repositories",
			"Held packages",
		},
		Dependencies:  []string{"APT"},
		RecoveryValue: "Minimal",
	},
	"snap": {
		Name: "Snap", Description: "Snap package manager and installed snaps",
		Category: "Packages", Maturity: MaturityStable,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"Installed snap packages",
			"Snap version",
		},
		Dependencies:  []string{"Snap"},
		RecoveryValue: "Minimal",
	},
	"flatpak": {
		Name: "Flatpak", Description: "Flatpak package manager and installed applications",
		Category: "Packages", Maturity: MaturityStable,
		Platforms: []string{"Linux"},
		DataCollected: []string{
			"Installed Flatpak applications",
			"Flatpak version",
		},
		Dependencies:  []string{"Flatpak"},
		RecoveryValue: "Minimal",
	},
	"system": {
		Name: "System", Description: "Operating system information and hardware profile",
		Category: "System", Maturity: MaturityStable,
		Platforms: []string{"Linux", "macOS"},
		DataCollected: []string{
			"OS name and version",
			"Kernel version",
			"CPU and RAM information",
			"Disk usage",
			"Desktop environment",
			"System locale and timezone",
		},
		Dependencies:  nil,
		RecoveryValue: "Minimal",
	},
}
