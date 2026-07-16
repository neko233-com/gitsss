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
		Usage: "Import Gitea accounts, RBAC, and selected Git repositories over HTTP",
		Description: `Read-only access is used against the source. The target is changed only with --apply.
Source passwords, password hashes, access tokens, SSH keys, and 2FA secrets are not available through the Gitea API.
New local accounts receive random temporary passwords and must change them on first login.`,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "source-url", Usage: "Source Gitea root URL", Required: true},
			&cli.StringFlag{Name: "source-token-file", Usage: "File containing source site-admin API token", Required: true, TakesFile: true},
			&cli.StringFlag{Name: "target-url", Usage: "Target gitsss root URL", Required: true},
			&cli.StringFlag{Name: "target-token-file", Usage: "File containing target site-admin API token", Required: true, TakesFile: true},
			&cli.StringSliceFlag{Name: "repo", Usage: "Repository owner/name or source URL; repeat for each repository", Required: true},
			&cli.StringFlag{Name: "conflict", Usage: "Existing target object policy: fail or skip", Value: string(giteahttpmigration.ConflictFail)},
			&cli.DurationFlag{Name: "timeout", Usage: "Whole discovery/import timeout", Value: 30 * time.Minute},
			&cli.BoolFlag{Name: "apply", Usage: "Write discovered objects to target; omitted means dry-run"},
			&cli.StringFlag{Name: "credentials-file", Usage: "New 0600 JSONL file for one-time temporary passwords", TakesFile: true},
		},
		Action: runImportGiteaHTTP,
	}
}

func runImportGiteaHTTP(ctx context.Context, c *cli.Command) (returnErr error) {
	sourceToken, err := readSecretFile(c.String("source-token-file"))
	if err != nil {
		return fmt.Errorf("read source token: %w", err)
	}
	targetToken, err := readSecretFile(c.String("target-token-file"))
	if err != nil {
		return fmt.Errorf("read target token: %w", err)
	}
	migrator, err := giteahttpmigration.New(giteahttpmigration.Options{
		SourceURL:      c.String("source-url"),
		SourceToken:    sourceToken,
		TargetURL:      c.String("target-url"),
		TargetToken:    targetToken,
		Repositories:   c.StringSlice("repo"),
		ConflictPolicy: giteahttpmigration.ConflictPolicy(c.String("conflict")),
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
