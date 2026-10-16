package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/neko233-com/gitsss/modules/giteahttpmigration"
	"github.com/neko233-com/gitsss/modules/json"

	"github.com/urfave/cli/v3"
)

func newImportGiteaHTTPCommand() *cli.Command {
	return &cli.Command{
		Name:  "import-gitea-http",
		Usage: "Import Gitea accounts, RBAC, Git data, issues, and pull requests over HTTP",
		Description: `Read-only access is used against the source. The target is changed only with --apply.
Authenticate with a site-admin token file, or with a username plus a password file. Do not put passwords on the command line.
Source password hashes, access tokens, SSH keys, and 2FA secrets are not available through the Gitea API.
New local accounts receive random temporary passwords and must change them on first login.
The initial import copies repository Git data, LFS objects, issues, comments, labels, milestones, pull requests, reviews, releases, wiki, and supported packages.
Use --sync-existing-git to incrementally fetch Git refs and LFS updates for repositories already imported. Fast-forward updates and source deletions are applied atomically; divergent target refs fail without being overwritten. That mode does not sync repository metadata, issues, pull requests, or packages.
Merged pull requests are closed on the target; the merge is not replayed.`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "source-url", Usage: "Source Gitea root URL", Required: true},
			&cli.StringFlag{Name: "source-user", Usage: "Source site-admin username, used with --source-password-file"},
			&cli.StringFlag{Name: "source-password-file", Usage: "File containing the source site-admin password", TakesFile: true},
			&cli.StringFlag{Name: "source-token-file", Usage: "File containing the source site-admin API token", TakesFile: true},
			&cli.StringFlag{Name: "target-url", Usage: "Target gitsss root URL", Required: true},
			&cli.StringFlag{Name: "target-user", Usage: "Target site-admin username, used with --target-password-file"},
			&cli.StringFlag{Name: "target-password-file", Usage: "File containing the target site-admin password", TakesFile: true},
			&cli.StringFlag{Name: "target-token-file", Usage: "File containing the target site-admin API token", TakesFile: true},
			&cli.StringSliceFlag{Name: "repo", Usage: "Repository owner/name or source URL; repeat for each repository"},
			&cli.BoolFlag{Name: "all-repos", Usage: "Import every repository visible to the source site administrator, with its owner accounts, organizations, teams, and permissions"},
			&cli.StringFlag{Name: "conflict", Usage: "Existing target object policy: fail or skip", Value: string(giteahttpmigration.ConflictFail)},
			&cli.BoolFlag{Name: "sync-existing-git", Usage: "Update Git refs and LFS objects on existing target repositories only"},
			&cli.StringFlag{Name: "mirror-cache-dir", Usage: "Persistent bare-repository cache used to fetch only new Git objects; defaults to the user's cache directory"},
			&cli.DurationFlag{Name: "timeout", Usage: "Whole discovery/import timeout", Value: 30 * time.Minute},
			&cli.BoolFlag{Name: "apply", Usage: "Write discovered objects to target; omitted means dry-run"},
			&cli.StringFlag{Name: "credentials-file", Usage: "New 0600 JSONL file for one-time temporary passwords", TakesFile: true},
		},
		Action: runImportGiteaHTTP,
	}
}

func runImportGiteaHTTP(ctx context.Context, c *cli.Command) (returnErr error) {
	sourceToken, err := optionalSecretFile(c.String("source-token-file"))
	if err != nil {
		return fmt.Errorf("read source token: %w", err)
	}
	sourcePassword, err := optionalSecretFile(c.String("source-password-file"))
	if err != nil {
		return fmt.Errorf("read source password: %w", err)
	}
	targetToken, err := optionalSecretFile(c.String("target-token-file"))
	if err != nil {
		return fmt.Errorf("read target token: %w", err)
	}
	targetPassword, err := optionalSecretFile(c.String("target-password-file"))
	if err != nil {
		return fmt.Errorf("read target password: %w", err)
	}
	if len(c.StringSlice("repo")) == 0 && !c.Bool("all-repos") {
		return errors.New("pass --repo for each repository or --all-repos")
	}
	migrator, err := giteahttpmigration.New(giteahttpmigration.Options{
		SourceURL:       c.String("source-url"),
		SourceUser:      c.String("source-user"),
		SourcePassword:  sourcePassword,
		SourceToken:     sourceToken,
		TargetURL:       c.String("target-url"),
		TargetUser:      c.String("target-user"),
		TargetPassword:  targetPassword,
		TargetToken:     targetToken,
		Repositories:    c.StringSlice("repo"),
		AllRepositories: c.Bool("all-repos"),
		ConflictPolicy:  giteahttpmigration.ConflictPolicy(c.String("conflict")),
		MirrorCacheDir:  c.String("mirror-cache-dir"),
		SyncExistingGit: c.Bool("sync-existing-git"),
	})
	if err != nil {
		return err
	}
	operationCtx, cancel := context.WithTimeout(ctx, c.Duration("timeout"))
	defer cancel()
	plan, err := migrator.Discover(operationCtx)
	if err != nil {
		return err
	}
	if !c.Bool("apply") {
		return writeIndentedJSON(c.Writer, plan)
	}
	if c.Bool("sync-existing-git") {
		result, err := migrator.SyncGitRepositories(operationCtx, plan)
		if err != nil {
			return err
		}
		return writeIndentedJSON(c.Writer, result)
	}
	if strings.TrimSpace(c.String("credentials-file")) == "" {
		return errors.New("--credentials-file is required with --apply")
	}
	credentials, err := giteahttpmigration.NewJSONLCredentialFile(c.String("credentials-file"))
	if err != nil {
		return err
	}
	defer func() {
		if err := credentials.Close(); returnErr == nil && err != nil {
			returnErr = fmt.Errorf("close credentials file: %w", err)
		}
	}()
	result, err := migrator.Apply(operationCtx, plan, credentials)
	if err != nil {
		return err
	}
	return writeIndentedJSON(c.Writer, result)
}

func writeIndentedJSON(writer io.Writer, value any) error {
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	_, err = writer.Write(content)
	return err
}

func optionalSecretFile(filePath string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", nil
	}
	return readSecretFile(filePath)
}

func readSecretFile(filePath string) (string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("secret path must be a regular file")
	}
	if info.Size() > 64*1024 {
		return "", errors.New("secret file exceeds 64 KiB")
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(content))
	if secret == "" {
		return "", errors.New("secret file is empty")
	}
	return secret, nil
}
