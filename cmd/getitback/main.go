package main

import (
	"log/slog"
	"os"

	"github.com/shreyansh-shankar/getitback/internal/cli"
	"github.com/shreyansh-shankar/getitback/internal/module"
	"github.com/shreyansh-shankar/getitback/internal/modules/apt"
	"github.com/shreyansh-shankar/getitback/internal/modules/brave"
	"github.com/shreyansh-shankar/getitback/internal/modules/certs"
	"github.com/shreyansh-shankar/getitback/internal/modules/chrome"
	"github.com/shreyansh-shankar/getitback/internal/modules/chromium"
	"github.com/shreyansh-shankar/getitback/internal/modules/cloud"
	"github.com/shreyansh-shankar/getitback/internal/modules/docker"
	"github.com/shreyansh-shankar/getitback/internal/modules/dotfiles"
	"github.com/shreyansh-shankar/getitback/internal/modules/edge"
	"github.com/shreyansh-shankar/getitback/internal/modules/firefox"
	"github.com/shreyansh-shankar/getitback/internal/modules/flatpak"
	"github.com/shreyansh-shankar/getitback/internal/modules/git"
	"github.com/shreyansh-shankar/getitback/internal/modules/golang"
	"github.com/shreyansh-shankar/getitback/internal/modules/gpg"
	"github.com/shreyansh-shankar/getitback/internal/modules/java"
	"github.com/shreyansh-shankar/getitback/internal/modules/kubernetes"
	"github.com/shreyansh-shankar/getitback/internal/modules/mongodb"
	"github.com/shreyansh-shankar/getitback/internal/modules/mysql"
	"github.com/shreyansh-shankar/getitback/internal/modules/neovim"
	"github.com/shreyansh-shankar/getitback/internal/modules/node"
	"github.com/shreyansh-shankar/getitback/internal/modules/opera"
	"github.com/shreyansh-shankar/getitback/internal/modules/postgres"
	"github.com/shreyansh-shankar/getitback/internal/modules/python"
	"github.com/shreyansh-shankar/getitback/internal/modules/redis"
	"github.com/shreyansh-shankar/getitback/internal/modules/repos"
	"github.com/shreyansh-shankar/getitback/internal/modules/rust"
	"github.com/shreyansh-shankar/getitback/internal/modules/shell"
	"github.com/shreyansh-shankar/getitback/internal/modules/snap"
	"github.com/shreyansh-shankar/getitback/internal/modules/sqlite"
	"github.com/shreyansh-shankar/getitback/internal/modules/ssh"
	"github.com/shreyansh-shankar/getitback/internal/modules/system"
	"github.com/shreyansh-shankar/getitback/internal/modules/virtualization"
	"github.com/shreyansh-shankar/getitback/internal/modules/vivaldi"
	"github.com/shreyansh-shankar/getitback/internal/modules/vscode"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	manager := module.NewManager()
	manager.Register(system.NewModule())
	manager.Register(git.NewModule())
	manager.Register(ssh.NewModule())
	manager.Register(shell.NewModule())
	manager.Register(dotfiles.NewModule())
	manager.Register(vscode.NewModule())
	manager.Register(neovim.NewModule())
	manager.Register(firefox.NewModule())
	manager.Register(chromium.NewModule())
	manager.Register(node.NewModule())
	manager.Register(golang.NewModule())
	manager.Register(python.NewModule())
	manager.Register(rust.NewModule())
	manager.Register(postgres.NewModule())
	manager.Register(mongodb.NewModule())
	manager.Register(redis.NewModule())
	manager.Register(sqlite.NewModule())
	manager.Register(apt.NewModule())
	manager.Register(snap.NewModule())
	manager.Register(flatpak.NewModule())
	manager.Register(gpg.NewModule())
	manager.Register(chrome.NewModule())
	manager.Register(brave.NewModule())
	manager.Register(vivaldi.NewModule())
	manager.Register(edge.NewModule())
	manager.Register(opera.NewModule())
	manager.Register(docker.NewModule())
	manager.Register(mysql.NewModule())
	manager.Register(cloud.NewModule())
	manager.Register(kubernetes.NewModule())
	manager.Register(virtualization.NewModule())
	manager.Register(certs.NewModule())
	manager.Register(repos.NewModule())
	manager.Register(java.NewModule())

	cli.Execute(manager)
}
