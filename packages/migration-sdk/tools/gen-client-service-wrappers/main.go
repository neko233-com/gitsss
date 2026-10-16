package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	wrappersOutputPath = "client_services_wrappers.go"
	migrationDocPath   = "docs/migrate-client-to-services.md"
)

type method struct {
	ClientName string
	Name       string
	Service    string
	Signature  string
	Args       string
	ImportUses map[string]struct{}
	SourceFile string
	SourceLine int
}

type serviceMetadata struct {
	Name        string
	Description string
	Order       int
}

var clientServiceFields = map[string]string{
	"ActionsService":       "Actions",
	"ActivityService":      "Activity",
	"ActivityPubService":   "ActivityPub",
	"AdminService":         "Admin",
	"GitService":           "Git",
	"HooksService":         "Hooks",
	"IssuesService":        "Issues",
	"RenderService":        "Render",
	"MetaService":          "Meta",
	"NotificationsService": "Notifications",
	"OAuth2Service":        "OAuth2",
	"OrganizationsService": "Organizations",
	"PackagesService":      "Packages",
	"PullRequestsService":  "PullRequests",
	"RepositoriesService":  "Repositories",
	"ReleasesService":      "Releases",
	"SettingsService":      "Settings",
	"TemplatesService":     "Templates",
	"UsersService":         "Users",
	"WikiService":          "Wiki",
}

var serviceMetadataByName = map[string]serviceMetadata{
	"Actions": {
		Name:        "Actions",
		Description: "Gitea Actions workflows, jobs, runners, secrets, and variables.",
		Order:       10,
	},
	"Activity": {
		Name:        "Activity",
		Description: "Activity feed APIs for users, organizations, teams, and repositories.",
		Order:       20,
	},
	"ActivityPub": {
		Name:        "ActivityPub",
		Description: "ActivityPub federation APIs.",
		Order:       30,
	},
	"Admin": {
		Name:        "Admin",
		Description: "Administrative APIs.",
		Order:       40,
	},
	"Git": {
		Name:        "Git",
		Description: "Low-level git data APIs such as blobs, refs, trees, notes, and git hooks.",
		Order:       50,
	},
	"Hooks": {
		Name:        "Hooks",
		Description: "Global, organization, user, and repository hook APIs.",
		Order:       60,
	},
	"Issues": {
		Name:        "Issues",
		Description: "Issues, comments, labels, reactions, subscriptions, and tracked time.",
		Order:       70,
	},
	"Render": {
		Name:        "Render",
		Description: "Markdown and markup rendering APIs.",
		Order:       80,
	},
	"Meta": {
		Name:        "Meta",
		Description: "Instance metadata APIs such as version, node info, and signing keys.",
		Order:       85,
	},
	"Notifications": {
		Name:        "Notifications",
		Description: "Notification APIs.",
		Order:       90,
	},
	"OAuth2": {
		Name:        "OAuth2",
		Description: "OAuth2 application APIs.",
		Order:       100,
	},
	"Organizations": {
		Name:        "Organizations",
		Description: "Organizations, teams, members, labels, blocks, and avatar helpers.",
		Order:       110,
	},
	"Packages": {
		Name:        "Packages",
		Description: "Package registry APIs.",
		Order:       120,
	},
	"PullRequests": {
		Name:        "PullRequests",
		Description: "Pull requests and reviews.",
		Order:       130,
	},
	"Repositories": {
		Name:        "Repositories",
		Description: "Repositories, branches, files, milestones, tags, stars, watches, and collaborators.",
		Order:       140,
	},
	"Releases": {
		Name:        "Releases",
		Description: "Repository releases and release attachments.",
		Order:       145,
	},
	"Settings": {
		Name:        "Settings",
		Description: "Global settings APIs.",
		Order:       150,
	},
	"Templates": {
		Name:        "Templates",
		Description: "Instance template APIs for gitignores, licenses, and labels.",
		Order:       155,
	},
	"Users": {
		Name:        "Users",
		Description: "Users, emails, keys, follows, settings, and other user-scoped APIs.",
		Order:       160,
	},
	"Wiki": {
		Name:        "Wiki",
		Description: "Repository wiki pages and revision history.",
		Order:       165,
	},
}

var legacyClientMethodNames = map[string]map[string]string{
	"Actions": {
		"CreateGlobalRunnerRegistrationToken": "CreateAdminActionRunnerRegistrationToken",
		"CreateOrgRunnerRegistrationToken":    "CreateOrgActionRunnerRegistrationToken",
		"CreateOrgSecret":                     "CreateOrgActionSecret",
		"CreateOrgVariable":                   "CreateOrgActionVariable",
		"CreateRepoRunnerRegistrationToken":   "CreateRepoActionRunnerRegistrationToken",
		"CreateRepoSecret":                    "CreateRepoActionSecret",
		"CreateRepoVariable":                  "CreateRepoActionVariable",
		"CreateUserRunnerRegistrationToken":   "CreateUserActionRunnerRegistrationToken",
		"CreateUserSecret":                    "CreateUserActionSecret",
		"CreateUserVariable":                  "CreateUserActionVariable",
		"DeleteGlobalRunner":                  "DeleteAdminActionRunner",
		"DeleteOrgRunner":                     "DeleteOrgActionRunner",
		"DeleteOrgSecret":                     "DeleteOrgActionSecret",
		"DeleteOrgVariable":                   "DeleteOrgActionVariable",
		"DeleteRepoArtifact":                  "DeleteRepoActionArtifact",
		"DeleteRepoRun":                       "DeleteRepoActionRun",
		"DeleteRepoRunner":                    "DeleteRepoActionRunner",
		"DeleteRepoSecret":                    "DeleteRepoActionSecret",
		"DeleteRepoVariable":                  "DeleteRepoActionVariable",
		"DeleteUserRunner":                    "DeleteUserActionRunner",
		"DeleteUserSecret":                    "DeleteUserActionSecret",
		"DeleteUserVariable":                  "DeleteUserActionVariable",
		"DisableRepoWorkflow":                 "DisableRepoActionWorkflow",
		"DispatchRepoWorkflow":                "DispatchRepoActionWorkflow",
		"EnableRepoWorkflow":                  "EnableRepoActionWorkflow",
		"GetGlobalRunner":                     "GetAdminActionRunner",
		"GetOrgRunner":                        "GetOrgActionRunner",
		"GetOrgVariable":                      "GetOrgActionVariable",
		"GetRepoArtifact":                     "GetRepoActionArtifact",
		"GetRepoArtifactArchive":              "GetRepoActionArtifactArchive",
		"GetRepoArtifactArchiveReader":        "GetRepoActionArtifactArchiveReader",
		"GetRepoRunJob":                       "GetRepoActionJob",
		"GetRepoRunJobLogs":                   "GetRepoActionJobLogs",
		"GetRepoRun":                          "GetRepoActionRun",
		"GetRepoRunner":                       "GetRepoActionRunner",
		"GetRepoVariable":                     "GetRepoActionVariable",
		"GetRepoWorkflow":                     "GetRepoActionWorkflow",
		"GetUserRunner":                       "GetUserActionRunner",
		"GetUserVariable":                     "GetUserActionVariable",
		"ListRunJobs":                         "ListAdminActionJobs",
		"ListGlobalRunners":                   "ListAdminActionRunners",
		"ListRuns":                            "ListAdminActionRuns",
		"ListOrgRunJobs":                      "ListOrgActionJobs",
		"ListOrgRunners":                      "ListOrgActionRunners",
		"ListOrgRuns":                         "ListOrgActionRuns",
		"ListOrgSecrets":                      "ListOrgActionSecret",
		"ListOrgVariables":                    "ListOrgActionVariable",
		"ListRepoArtifacts":                   "ListRepoActionArtifacts",
		"ListRepoRunJobs":                     "ListRepoActionJobs",
		"ListRepoRunArtifacts":                "ListRepoActionRunArtifacts",
		"ListRepoJobsByRun":                   "ListRepoActionRunJobs",
		"ListRepoRunners":                     "ListRepoActionRunners",
		"ListRepoRuns":                        "ListRepoActionRuns",
		"ListRepoSecrets":                     "ListRepoActionSecret",
		"ListRepoTasks":                       "ListRepoActionTasks",
		"ListRepoVariables":                   "ListRepoActionVariable",
		"ListRepoWorkflows":                   "ListRepoActionWorkflows",
		"ListUserRunJobs":                     "ListUserActionJobs",
		"ListUserRunners":                     "ListUserActionRunners",
		"ListUserRuns":                        "ListUserActionRuns",
		"ListUserVariables":                   "ListUserActionVariable",
		"RerunRepoRunJob":                     "RerunRepoActionJob",
		"RerunRepoRun":                        "RerunRepoActionRun",
		"RerunRepoRunFailedJobs":              "RerunRepoActionRunFailedJobs",
		"UpdateGlobalRunner":                  "UpdateAdminActionRunner",
		"UpdateOrgRunner":                     "UpdateOrgActionRunner",
		"UpdateOrgVariable":                   "UpdateOrgActionVariable",
		"UpdateRepoRunner":                    "UpdateRepoActionRunner",
		"UpdateRepoVariable":                  "UpdateRepoActionVariable",
		"UpdateUserRunner":                    "UpdateUserActionRunner",
		"UpdateUserVariable":                  "UpdateUserActionVariable",
	},
	"Activity": {
		"ListOrgFeeds":  "ListOrgActivityFeeds",
		"ListRepoFeeds": "ListRepoActivityFeeds",
		"ListTeamFeeds": "ListTeamActivityFeeds",
		"ListUserFeeds": "ListUserActivityFeeds",
	},
	"Admin": {
		"CreateOrg":           "AdminCreateOrg",
		"CreateRepo":          "AdminCreateRepo",
		"CreateUser":          "AdminCreateUser",
		"CreateUserPublicKey": "AdminCreateUserPublicKey",
		"DeleteUser":          "AdminDeleteUser",
		"DeleteUserPublicKey": "AdminDeleteUserPublicKey",
		"EditUser":            "AdminEditUser",
		"ListEmails":          "ListAdminEmails",
		"ListOrgs":            "AdminListOrgs",
		"ListUsers":           "AdminListUsers",
		"RenameUser":          "AdminRenameUser",
		"SearchEmails":        "SearchAdminEmails",
	},
	"ActivityPub": {
		"GetPerson":         "GetActivityPubPerson",
		"GetPersonResponse": "GetActivityPubPersonResponse",
		"SendInbox":         "SendActivityPubInbox",
	},
	"Hooks": {
		"CreateGlobalHook": "CreateAdminHook",
		"DeleteGlobalHook": "DeleteAdminHook",
		"EditGlobalHook":   "EditAdminHook",
		"GetGlobalHook":    "GetAdminHook",
		"ListGlobalHooks":  "ListAdminHooks",
	},
	"Render": {
		"RenderMarkdown":    "RenderMarkdown",
		"RenderMarkdownRaw": "RenderMarkdownRaw",
	},
	"OAuth2": {
		"CreateApplication": "CreateOauth2",
		"DeleteApplication": "DeleteOauth2",
		"GetApplication":    "GetOauth2",
		"ListApplications":  "ListOauth2",
		"UpdateApplication": "UpdateOauth2",
	},
	"Repositories": {
		"ListMilestones": "ListRepoMilestones",
	},
	"Wiki": {
		"CreatePage":       "CreateWikiPage",
		"DeletePage":       "DeleteWikiPage",
		"EditPage":         "EditWikiPage",
		"GetPage":          "GetWikiPage",
		"GetPageRevisions": "GetWikiPageRevisions",
		"ListPages":        "ListWikiPages",
	},
	"Notifications": {
		"Check":          "CheckNotifications",
		"GetByID":        "GetNotification",
		"List":           "ListNotifications",
		"ListByRepo":     "ListRepoNotifications",
		"MarkRead":       "ReadNotifications",
		"MarkReadByID":   "ReadNotification",
		"MarkReadByRepo": "ReadRepoNotifications",
	},
	"Templates": {
		"GetGitignore":   "GetGitignoreTemplateInfo",
		"GetLabel":       "GetLabelTemplate",
		"GetLicense":     "GetLicenseTemplateInfo",
		"ListGitignores": "ListGitignoresTemplates",
		"ListLabels":     "ListLabelTemplates",
		"ListLicenses":   "ListLicenseTemplates",
	},
}

func main() {
	rootDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	fset := token.NewFileSet()
	files, err := parsePackageFiles(fset, rootDir)
	if err != nil {
		panic(err)
	}

	importPaths := collectImportPaths(files)
	methods := collectMethods(fset, files)

	wrapperContent := renderWrappers(importPaths, methods)
	formattedWrappers, err := format.Source(wrapperContent)
	if err != nil {
		fmt.Fprintln(os.Stderr, string(wrapperContent))
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, wrappersOutputPath), formattedWrappers, 0o644); err != nil {
		panic(err)
	}

	wrapperLines := collectWrapperLines(formattedWrappers)
	migrationDoc := renderMigrationDoc(methods, wrapperLines)
	if err := os.WriteFile(filepath.Join(rootDir, migrationDocPath), migrationDoc, 0o644); err != nil {
		panic(err)
	}
}

func parsePackageFiles(fset *token.FileSet, rootDir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}

	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		filenames = append(filenames, filepath.Join(rootDir, name))
	}
	sort.Strings(filenames)

	files := make([]*ast.File, 0, len(filenames))
	for _, filename := range filenames {
		file, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		if file.Name.Name != "gitea" {
			continue
		}
		files = append(files, file)
	}
	return files, nil
}

func collectImportPaths(files []*ast.File) map[string]string {
	importPaths := map[string]string{}
	for _, file := range files {
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, "\"")
			if spec.Name != nil {
				if spec.Name.Name == "_" || spec.Name.Name == "." {
					continue
				}
				importPaths[spec.Name.Name] = path
				continue
			}

			alias := filepath.Base(path)
			importPaths[alias] = path
		}
	}
	return importPaths
}

func collectMethods(fset *token.FileSet, files []*ast.File) []method {
	methods := make([]method, 0, 256)
	for _, file := range files {
		base := filepath.Base(fset.Position(file.Package).Filename)
		if base == "client.go" || base == "httpsign.go" || base == "services.go" || base == wrappersOutputPath {
			continue
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || !fn.Name.IsExported() {
				continue
			}
			if len(fn.Recv.List) != 1 {
				continue
			}

			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			recv, ok := star.X.(*ast.Ident)
			if !ok {
				continue
			}

			service := clientServiceFields[recv.Name]
			if service == "" {
				continue
			}

			args := collectArgs(fn.Type.Params)
			position := fset.Position(fn.Pos())
			methods = append(methods, method{
				ClientName: legacyClientMethodName(service, fn.Name.Name),
				Name:       fn.Name.Name,
				Service:    service,
				Signature:  strings.TrimPrefix(renderNode(fset, fn.Type), "func"),
				Args:       strings.Join(args, ", "),
				ImportUses: collectSignatureImports(fn.Type),
				SourceFile: filepath.Base(position.Filename),
				SourceLine: position.Line,
			})
		}
	}

	sort.Slice(methods, func(i, j int) bool {
		if methods[i].Service == methods[j].Service {
			return methods[i].Name < methods[j].Name
		}
		return methods[i].Service < methods[j].Service
	})

	return methods
}

func collectArgs(fields *ast.FieldList) []string {
	if fields == nil {
		return nil
	}

	args := make([]string, 0, len(fields.List))
	for i, field := range fields.List {
		if len(field.Names) == 0 {
			panic(fmt.Sprintf("unnamed parameter in exported service method at index %d", i))
		}

		for _, name := range field.Names {
			arg := name.Name
			if _, ok := field.Type.(*ast.Ellipsis); ok {
				arg += "..."
			}
			args = append(args, arg)
		}
	}
	return args
}

func collectSignatureImports(fnType *ast.FuncType) map[string]struct{} {
	imports := map[string]struct{}{}
	ast.Inspect(fnType, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		imports[ident.Name] = struct{}{}
		return true
	})
	return imports
}

func legacyClientMethodName(service, methodName string) string {
	if serviceMethods, ok := legacyClientMethodNames[service]; ok {
		if legacyName, ok := serviceMethods[methodName]; ok {
			return legacyName
		}
	}
	return methodName
}

func renderWrappers(importPaths map[string]string, methods []method) []byte {
	usedImports := map[string]string{}
	for _, method := range methods {
		for name := range method.ImportUses {
			path, ok := importPaths[name]
			if ok {
				usedImports[name] = path
			}
		}
	}

	aliases := make([]string, 0, len(usedImports))
	for alias := range usedImports {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	var out bytes.Buffer
	out.WriteString("// Code generated by go generate; DO NOT EDIT.\n\n")
	out.WriteString("package gitea\n\n")
	if len(aliases) > 0 {
		out.WriteString("import (\n")
		for _, alias := range aliases {
			path := usedImports[alias]
			if filepath.Base(path) == alias {
				fmt.Fprintf(&out, "\t%q\n", path)
			} else {
				fmt.Fprintf(&out, "\t%s %q\n", alias, path)
			}
		}
		out.WriteString(")\n\n")
	}

	for _, method := range methods {
		fmt.Fprintf(&out, "// Deprecated: use `client.%s.%s` instead.\n", method.Service, method.Name)
		fmt.Fprintf(&out, "func (c *Client) %s%s {\n", method.ClientName, method.Signature)
		out.WriteString("\tc.initServices()\n")
		fmt.Fprintf(&out, "\treturn c.%s.%s(%s)\n", method.Service, method.Name, method.Args)
		out.WriteString("}\n\n")
	}

	return out.Bytes()
}

func collectWrapperLines(content []byte) map[string]int {
	lines := strings.Split(string(content), "\n")
	wrapperLines := map[string]int{}
	const prefix = "func (c *Client) "
	for i, line := range lines {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		nameAndRest := strings.TrimPrefix(line, prefix)
		name, _, ok := strings.Cut(nameAndRest, "(")
		if !ok {
			continue
		}
		wrapperLines[name] = i + 1
	}
	return wrapperLines
}

func renderMigrationDoc(methods []method, wrapperLines map[string]int) []byte {
	serviceMethods := map[string][]method{}
	serviceOrder := make([]string, 0, len(clientServiceFields))
	serviceSeen := map[string]struct{}{}
	for _, method := range methods {
		serviceMethods[method.Service] = append(serviceMethods[method.Service], method)
		if _, ok := serviceSeen[method.Service]; !ok {
			serviceSeen[method.Service] = struct{}{}
			serviceOrder = append(serviceOrder, method.Service)
		}
	}

	sort.Slice(serviceOrder, func(i, j int) bool {
		left := serviceMetadataByName[serviceOrder[i]]
		right := serviceMetadataByName[serviceOrder[j]]
		if left.Order == right.Order {
			return left.Name < right.Name
		}
		return left.Order < right.Order
	})

	var out bytes.Buffer
	out.WriteString("# Migration Guide: Client Methods to Services\n\n")
	out.WriteString("This file is generated by `go generate ./...`. Do not edit it manually.\n\n")
	out.WriteString("The SDK now exposes business APIs through service-specific entry points on `Client`,\n")
	out.WriteString("following the `go-github` style.\n\n")
	out.WriteString("The old `Client` business methods are still available in the current module, but they are now\n")
	out.WriteString("generated compatibility wrappers in `client_services_wrappers.go` and are marked\n")
	out.WriteString("as deprecated. They are intended to be removed in the next major version.\n\n")
	out.WriteString("## What changed\n\n")
	out.WriteString("Instead of calling business methods directly on `Client`:\n\n")
	out.WriteString("```go\nrepo, _, err := client.GetRepo(ctx, \"owner\", \"repo\")\n```\n\n")
	out.WriteString("Call them through the matching service:\n\n")
	out.WriteString("```go\nrepo, _, err := client.Repositories.GetRepo(ctx, \"owner\", \"repo\")\n```\n\n")
	out.WriteString("The underlying implementations stay in their existing files. This migration only\n")
	out.WriteString("changes the public entry points and grouping.\n\n")
	out.WriteString("## Service overview\n\n")
	out.WriteString("| Service | Scope | Methods |\n")
	out.WriteString("| --- | --- | ---: |\n")
	for _, service := range serviceOrder {
		methodsForService := serviceMethods[service]
		metadata := serviceMetadataByName[service]
		fmt.Fprintf(&out, "| [`client.%s`](#client%s) | %s | %d |\n", service, strings.ToLower(service), metadata.Description, len(methodsForService))
	}
	out.WriteString("\n## Full mapping\n\n")
	for _, service := range serviceOrder {
		methodsForService := serviceMethods[service]
		metadata := serviceMetadataByName[service]
		fmt.Fprintf(&out, "### `client.%s`\n\n", service)
		fmt.Fprintf(&out, "%s\n\n", metadata.Description)
		out.WriteString("| Deprecated `Client` method | Service method |\n")
		out.WriteString("| --- | --- |\n")
		for _, method := range methodsForService {
			legacyLink := fmt.Sprintf("../%s#L%d", wrappersOutputPath, wrapperLines[method.ClientName])
			serviceLink := fmt.Sprintf("../%s#L%d", method.SourceFile, method.SourceLine)
			fmt.Fprintf(&out, "| [`client.%s`](%s) | [`client.%s.%s`](%s) |\n", method.ClientName, legacyLink, method.Service, method.Name, serviceLink)
		}
		out.WriteString("\n")
	}

	out.WriteString("## What stays on Client\n\n")
	out.WriteString("Low-level client setup and transport helpers still live on `Client`, for example:\n\n")
	out.WriteString("- `gitea.NewClient(...)`\n")
	out.WriteString("- `client.SetHTTPClient(...)`\n")
	out.WriteString("- `client.SetBasicAuth(...)`\n")
	out.WriteString("- `client.SetOTP(...)`\n")
	out.WriteString("- `client.SetSudo(...)`\n")
	out.WriteString("- `client.SetUserAgent(...)`\n")
	out.WriteString("- `client.CheckServerVersionConstraint(...)`\n\n")
	out.WriteString("## Compatibility wrappers\n\n")
	out.WriteString("The deprecated wrappers are generated. If you change exported service methods,\n")
	out.WriteString("regenerate them with:\n\n")
	out.WriteString("```bash\nmake generate\n```\n\n")
	out.WriteString("or:\n\n")
	out.WriteString("```bash\ngo generate ./...\n```\n")
	return out.Bytes()
}

func renderNode(fset *token.FileSet, node any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, node); err != nil {
		panic(err)
	}
	return buf.String()
}
