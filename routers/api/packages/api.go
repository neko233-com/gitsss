// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"net/http"

	auth_model "github.com/neko233-com/gitsss/models/auth"
	"github.com/neko233-com/gitsss/models/perm"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/routers/api/packages/alpine"
	"github.com/neko233-com/gitsss/routers/api/packages/arch"
	"github.com/neko233-com/gitsss/routers/api/packages/chef"
	"github.com/neko233-com/gitsss/routers/api/packages/conan"
	"github.com/neko233-com/gitsss/routers/api/packages/conda"
	"github.com/neko233-com/gitsss/routers/api/packages/container"
	"github.com/neko233-com/gitsss/routers/api/packages/cran"
	"github.com/neko233-com/gitsss/routers/api/packages/debian"
	"github.com/neko233-com/gitsss/routers/api/packages/generic"
	"github.com/neko233-com/gitsss/routers/api/packages/helm"
	"github.com/neko233-com/gitsss/routers/api/packages/pub"
	"github.com/neko233-com/gitsss/routers/api/packages/rpm"
	"github.com/neko233-com/gitsss/routers/api/packages/rubygems"
	"github.com/neko233-com/gitsss/routers/api/packages/swift"
	"github.com/neko233-com/gitsss/routers/api/packages/terraform"
	"github.com/neko233-com/gitsss/routers/api/packages/vagrant"
	"github.com/neko233-com/gitsss/services/auth"
	"github.com/neko233-com/gitsss/services/context"
)

func reqPackageAccess(accessMode perm.AccessMode) func(ctx *context.Context) {
	return func(ctx *context.Context) {
		if ctx.Data["IsApiToken"] == true {
			scope, ok := ctx.Data["ApiTokenScope"].(auth_model.AccessTokenScope)
			if ok { // it's a personal access token but not oauth2 token
				scopeMatched := false
				var err error
				switch accessMode {
				case perm.AccessModeRead:
					scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeReadPackage)
					if err != nil {
						ctx.HTTPError(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				case perm.AccessModeWrite:
					scopeMatched, err = scope.HasScope(auth_model.AccessTokenScopeWritePackage)
					if err != nil {
						ctx.HTTPError(http.StatusInternalServerError, "HasScope", err.Error())
						return
					}
				}
				if !scopeMatched {
					ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
					ctx.HTTPError(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
					return
				}

				// check if scope only applies to public resources
				publicOnly, err := scope.PublicOnly()
				if err != nil {
					ctx.HTTPError(http.StatusForbidden, "tokenRequiresScope", "parsing public resource scope failed: "+err.Error())
					return
				}

				if publicOnly {
					// a public-only token must not reach limited-visibility owners either,
					// matching how orgs/users are enforced elsewhere in this file
					if ctx.Package != nil && !ctx.Package.Owner.Visibility.IsPublic() {
						ctx.HTTPError(http.StatusForbidden, "reqToken", "token scope is limited to public packages")
						return
					}
				}
			}
		}

		if ctx.Package.AccessMode < accessMode && !ctx.IsUserSiteAdmin() {
			ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm="Gitea Package API"`)
			ctx.HTTPError(http.StatusUnauthorized, "reqPackageAccess", "user should have specific permission or be a site admin")
			return
		}
	}
}

type verifyAuthOptions struct {
	afterAuthCallback func(ctx *context.Context, err error)
}

func verifyAuth(r *web.Router, authMethods []auth.Method, opts verifyAuthOptions) {
	if setting.Service.EnableReverseProxyAuth {
		authMethods = append(authMethods, &auth.ReverseProxy{})
	}
	authGroup := auth.NewGroup(authMethods...)

	r.AfterRouting(func(ctx *context.Context) {
		var err error
		ctx.Doer, err = authGroup.Verify(ctx.Req, ctx.Resp, ctx, ctx.Session)
		ctx.IsSigned = ctx.Doer != nil
		if opts.afterAuthCallback != nil {
			opts.afterAuthCallback(ctx, err)
		} else if err != nil {
			log.Error("Failed to verify user: %v", err)
			ctx.HTTPError(http.StatusUnauthorized, "Failed to authenticate user")
		}
	})
}

// CommonRoutes provide endpoints for most package managers (except containers - see below)
// These are mounted on `/api/packages` (not `/api/v1/packages`)
func CommonRoutes() *web.Router {
	r := web.NewRouter()

	r.AfterRouting(context.PackageContexter())

	verifyAuth(r, []auth.Method{
		&auth.OAuth2{},
		&auth.Basic{},
		&Auth{},
		&chef.Auth{},
	}, verifyAuthOptions{})

	r.Group("/{username}", func() {
		r.Group("/alpine", func() {
			r.Get("/key", alpine.GetRepositoryKey)
			r.Group("/{branch}/{repository}", func() {
				r.Put("", reqPackageAccess(perm.AccessModeWrite), alpine.UploadPackageFile)
				r.Group("/{architecture}", func() {
					r.Get("/APKINDEX.tar.gz", alpine.GetRepositoryFile)
					r.Group("/{filename}", func() {
						r.Get("", alpine.DownloadPackageFile)
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), alpine.DeletePackageFile)
					})
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/arch", func() {
			r.Methods("HEAD,GET", "/repository.key", arch.GetRepositoryKey)
			r.Methods("PUT", "" /* no repository */, reqPackageAccess(perm.AccessModeWrite), arch.UploadPackageFile)
			r.PathGroup("/*", func(g *web.RouterPathGroup) {
				g.MatchPath("PUT", "/<repository:*>", reqPackageAccess(perm.AccessModeWrite), arch.UploadPackageFile)
				g.MatchPath("HEAD,GET", "/<repository:*>/<architecture>/<filename>", arch.GetPackageOrRepositoryFile)
				g.MatchPath("DELETE", "/<repository:*>/<name>/<version>/<architecture>", reqPackageAccess(perm.AccessModeWrite), arch.DeletePackageVersion)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/chef", func() {
			r.Group("/api/v1", func() {
				r.Get("/universe", chef.PackagesUniverse)
				r.Get("/search", chef.EnumeratePackages)
				r.Group("/cookbooks", func() {
					r.Get("", chef.EnumeratePackages)
					r.Post("", reqPackageAccess(perm.AccessModeWrite), chef.UploadPackage)
					r.Group("/{name}", func() {
						r.Get("", chef.PackageMetadata)
						r.Group("/versions/{version}", func() {
							r.Get("", chef.PackageVersionMetadata)
							r.Delete("", reqPackageAccess(perm.AccessModeWrite), chef.DeletePackageVersion)
							r.Get("/download", chef.DownloadPackage)
						})
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), chef.DeletePackage)
					})
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/conan", func() {
			r.Group("/v1", func() {
				r.Get("/ping", conan.Ping)
				r.Group("/users", func() {
					r.Get("/authenticate", conan.Authenticate)
					r.Get("/check_credentials", conan.CheckCredentials)
				})
				r.Group("/conans", func() {
					r.Get("/search", conan.SearchRecipes)
					r.Group("/{name}/{version}/{user}/{channel}", func() {
						r.Get("", conan.RecipeSnapshot)
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV1)
						r.Get("/search", conan.SearchPackagesV1)
						r.Get("/digest", conan.RecipeDownloadURLs)
						r.Post("/upload_urls", reqPackageAccess(perm.AccessModeWrite), conan.RecipeUploadURLs)
						r.Get("/download_urls", conan.RecipeDownloadURLs)
						r.Group("/packages", func() {
							r.Post("/delete", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV1)
							r.Group("/{package_reference}", func() {
								r.Get("", conan.PackageSnapshot)
								r.Get("/digest", conan.PackageDownloadURLs)
								r.Post("/upload_urls", reqPackageAccess(perm.AccessModeWrite), conan.PackageUploadURLs)
								r.Get("/download_urls", conan.PackageDownloadURLs)
							})
						})
					}, conan.ExtractPathParameters)
				})
				r.Group("/files/{name}/{version}/{user}/{channel}/{recipe_revision}", func() {
					r.Group("/recipe/{filename}", func() {
						r.Get("", conan.DownloadRecipeFile)
						r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadRecipeFile)
					})
					r.Group("/package/{package_reference}/{package_revision}/{filename}", func() {
						r.Get("", conan.DownloadPackageFile)
						r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadPackageFile)
					})
				}, conan.ExtractPathParameters)
			})
			r.Group("/v2", func() {
				r.Get("/ping", conan.Ping)
				r.Group("/users", func() {
					r.Get("/authenticate", conan.Authenticate)
					r.Get("/check_credentials", conan.CheckCredentials)
				})
				r.Group("/conans", func() {
					r.Get("/search", conan.SearchRecipes)
					r.Group("/{name}/{version}/{user}/{channel}", func() {
						r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV2)
						r.Get("/search", conan.SearchPackagesV2)
						r.Get("/latest", conan.LatestRecipeRevision)
						r.Group("/revisions", func() {
							r.Get("", conan.ListRecipeRevisions)
							r.Group("/{recipe_revision}", func() {
								r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeleteRecipeV2)
								r.Get("/search", conan.SearchPackagesV2)
								r.Group("/files", func() {
									r.Get("", conan.ListRecipeRevisionFiles)
									r.Group("/{filename}", func() {
										r.Get("", conan.DownloadRecipeFile)
										r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadRecipeFile)
									})
								})
								r.Group("/packages", func() {
									r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
									r.Group("/{package_reference}", func() {
										r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
										r.Get("/latest", conan.LatestPackageRevision)
										r.Group("/revisions", func() {
											r.Get("", conan.ListPackageRevisions)
											r.Group("/{package_revision}", func() {
												r.Delete("", reqPackageAccess(perm.AccessModeWrite), conan.DeletePackageV2)
												r.Group("/files", func() {
													r.Get("", conan.ListPackageRevisionFiles)
													r.Group("/{filename}", func() {
														r.Get("", conan.DownloadPackageFile)
														r.Put("", reqPackageAccess(perm.AccessModeWrite), conan.UploadPackageFile)
													})
												})
											})
										})
									})
								})
							})
						})
					}, conan.ExtractPathParameters)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.PathGroup("/conda/*", func(g *web.RouterPathGroup) {
			g.MatchPath("GET", "/<architecture>/<filename>", conda.ListOrGetPackages)
			g.MatchPath("GET", "/<channel:*>/<architecture>/<filename>", conda.ListOrGetPackages)
			g.MatchPath("PUT", "/<channel:*>/<filename>", reqPackageAccess(perm.AccessModeWrite), conda.UploadPackageFile)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/cran", func() {
			r.Group("/src", func() {
				r.Group("/contrib", func() {
					r.Get("/PACKAGES", cran.EnumerateSourcePackages)
					r.Get("/PACKAGES{format}", cran.EnumerateSourcePackages)
					r.Get("/{filename}", cran.DownloadSourcePackageFile)
					r.Get("/Archive/{packagename}/{filename}", cran.DownloadSourcePackageFile)
				})
				r.Put("", reqPackageAccess(perm.AccessModeWrite), cran.UploadSourcePackageFile)
			})
			r.Group("/bin", func() {
				r.Group("/{platform}/contrib/{rversion}", func() {
					r.Get("/PACKAGES", cran.EnumerateBinaryPackages)
					r.Get("/PACKAGES{format}", cran.EnumerateBinaryPackages)
					r.Get("/{filename}", cran.DownloadBinaryPackageFile)
				})
				r.Put("", reqPackageAccess(perm.AccessModeWrite), cran.UploadBinaryPackageFile)
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/debian", func() {
			r.Get("/repository.key", debian.GetRepositoryKey)
			r.Group("/dists/{distribution}", func() {
				r.Get("/{filename}", debian.GetRepositoryFile)
				r.Get("/by-hash/{algorithm}/{hash}", debian.GetRepositoryFileByHash)
				r.Group("/{component}/{architecture}", func() {
					r.Get("/{filename}", debian.GetRepositoryFile)
					r.Get("/by-hash/{algorithm}/{hash}", debian.GetRepositoryFileByHash)
				})
			})
			r.Group("/pool/{distribution}/{component}", func() {
				r.Get("/{name}_{version}_{architecture}.deb", debian.DownloadPackageFile)
				r.Group("", func() {
					r.Put("/upload", debian.UploadPackageFile)
					r.Delete("/{name}/{version}/{architecture}", debian.DeletePackageFile)
				}, reqPackageAccess(perm.AccessModeWrite))
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/generic", func() {
			r.Group("/{packagename}/{packageversion}", func() {
				r.Delete("", reqPackageAccess(perm.AccessModeWrite), generic.DeletePackage)
				r.Group("/{filename}", func() {
					r.Methods("HEAD,GET", "", generic.DownloadPackageFile)
					r.Group("", func() {
						r.Put("", generic.UploadPackage)
						r.Delete("", generic.DeletePackageFile)
					}, reqPackageAccess(perm.AccessModeWrite))
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/helm", func() {
			r.Get("/index.yaml", helm.Index)
			r.Get("/{filename}", helm.DownloadPackageFile)
			r.Post("/api/charts", reqPackageAccess(perm.AccessModeWrite), helm.UploadPackage)
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/pub", func() {
			r.Group("/api/packages", func() {
				r.Group("/versions/new", func() {
					r.Get("", pub.RequestUpload)
					r.Post("/upload", pub.UploadPackageFile)
					r.Get("/finalize/{id}/{version}", pub.FinalizePackage)
				}, reqPackageAccess(perm.AccessModeWrite))
				r.Group("/{id}", func() {
					r.Get("", pub.EnumeratePackageVersions)
					r.Get("/files/{version}", pub.DownloadPackageFile)
					r.Get("/{version}", pub.PackageVersionMetadata)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))

		r.Methods("HEAD,GET", "/rpm.repo", reqPackageAccess(perm.AccessModeRead), rpm.GetRepositoryConfig)
		r.PathGroup("/rpm/*", func(g *web.RouterPathGroup) {
			g.MatchPath("HEAD,GET", "/repository.key", rpm.GetRepositoryKey)
			g.MatchPath("HEAD,GET", "/<group:*>.repo", rpm.GetRepositoryConfig)
			g.MatchPath("HEAD", "/<group:*>/repodata/<filename>", rpm.CheckRepositoryFileExistence)
			g.MatchPath("GET", "/<group:*>/repodata/<filename>", rpm.GetRepositoryFile)
			g.MatchPath("PUT", "/<group:*>/upload", reqPackageAccess(perm.AccessModeWrite), rpm.UploadPackageFile)
			g.MatchPath("POST", "/<group:*>/package/<name>/<version>/errata", reqPackageAccess(perm.AccessModeWrite), rpm.UploadErrata)
			// this URL pattern is only used internally in the RPM index, it is generated by us, the filename part is not really used (can be anything)
			g.MatchPath("HEAD,GET", "/<group:*>/package/<name>/<version>/<architecture>/<filename>", rpm.DownloadPackageFile)
			g.MatchPath("HEAD,GET", "/<group:*>/package/<name>/<version>/<architecture>", rpm.DownloadPackageFile)
			g.MatchPath("DELETE", "/<group:*>/package/<name>/<version>/<architecture>", reqPackageAccess(perm.AccessModeWrite), rpm.DeletePackageFile)
		}, reqPackageAccess(perm.AccessModeRead))

		r.Group("/rubygems", func() {
			r.Get("/specs.4.8.gz", rubygems.EnumeratePackages)
			r.Get("/latest_specs.4.8.gz", rubygems.EnumeratePackagesLatest)
			r.Get("/prerelease_specs.4.8.gz", rubygems.EnumeratePackagesPreRelease)
			r.Get("/quick/Marshal.4.8/{filename}", rubygems.ServePackageSpecification)
			r.Get("/gems/{filename}", rubygems.DownloadPackageFile)
			r.Get("/info/{packagename}", rubygems.GetPackageInfo)
			r.Get("/versions", rubygems.GetAllPackagesVersions)
			r.Group("/api/v1/gems", func() {
				r.Post("/", rubygems.UploadPackageFile)
				r.Delete("/yank", rubygems.DeletePackage)
			}, reqPackageAccess(perm.AccessModeWrite))
		}, reqPackageAccess(perm.AccessModeRead))

		r.Group("/swift", func() {
			r.Group("", func() { // Needs to be unauthenticated.
				r.Post("", swift.CheckAuthenticate)
				r.Post("/login", swift.CheckAuthenticate)
			})
			r.Group("", func() {
				r.Group("/{scope}/{name}", func() {
					r.Group("", func() {
						r.Get("", swift.EnumeratePackageVersions)
						r.Get(".json", swift.EnumeratePackageVersions)
					}, swift.CheckAcceptMediaType(swift.AcceptJSON))
					r.PathGroup("/*", func(g *web.RouterPathGroup) {
						g.MatchPath("GET", "/<version>.json", swift.CheckAcceptMediaType(swift.AcceptJSON), swift.PackageVersionMetadata)
						g.MatchPath("GET", "/<version>.zip", swift.CheckAcceptMediaType(swift.AcceptZip), swift.DownloadPackageFile)
						g.MatchPath("GET", "/<version>/Package.swift", swift.CheckAcceptMediaType(swift.AcceptSwift), swift.DownloadManifest)
						g.MatchPath("GET", "/<version>", swift.CheckAcceptMediaType(swift.AcceptJSON), swift.PackageVersionMetadata)
						g.MatchPath("PUT", "/<version>", reqPackageAccess(perm.AccessModeWrite), swift.CheckAcceptMediaType(swift.AcceptJSON), swift.UploadPackageFile)
					})
				})
				r.Get("/identifiers", swift.CheckAcceptMediaType(swift.AcceptJSON), swift.LookupPackageIdentifiers)
			}, reqPackageAccess(perm.AccessModeRead))
		})
		// See https://docs.gitlab.com/ci/jobs/fine_grained_permissions/#terraform-state-endpoints
		// For endpoint and permission reference
		r.Group("/terraform/state/{name}", func() {
			r.Get("", terraform.GetTerraformState)
			r.Get("/versions/{serial}", terraform.GetTerraformStateBySerial)
			r.Group("", func() {
				r.Post("", terraform.UploadState)
				r.Delete("", terraform.DeleteState)
				r.Delete("/versions/{serial}", terraform.DeleteStateBySerial)
			}, reqPackageAccess(perm.AccessModeWrite))
			r.Group("/lock", func() {
				r.Post("", terraform.LockState)
				r.Delete("", terraform.UnlockState)
			}, reqPackageAccess(perm.AccessModeWrite))
		}, reqPackageAccess(perm.AccessModeRead))
		r.Group("/vagrant", func() {
			r.Group("/authenticate", func() {
				r.Get("", vagrant.CheckAuthenticate)
			})
			r.Group("/{name}", func() {
				r.Head("", vagrant.CheckBoxAvailable)
				r.Get("", vagrant.EnumeratePackageVersions)
				r.Group("/{version}/{provider}", func() {
					r.Get("", vagrant.DownloadPackageFile)
					r.Put("", reqPackageAccess(perm.AccessModeWrite), vagrant.UploadPackageFile)
				})
			})
		}, reqPackageAccess(perm.AccessModeRead))
	}, context.UserAssignmentWeb(), context.PackageAssignment())

	return r
}

// ContainerRoutes provides endpoints that implement the OCI API to serve containers
// These have to be mounted on `/v2/...` to comply with the OCI spec:
// https://github.com/opencontainers/distribution-spec/blob/main/spec.md
func ContainerRoutes() *web.Router {
	r := web.NewRouter()

	r.AfterRouting(context.PackageContexter())

	verifyAuth(r, []auth.Method{
		&auth.Basic{},
		// container auth requires token, so container.Authenticate issues a Ghost user token for anonymous access
		&Auth{AllowGhostUser: true},
	}, verifyAuthOptions{
		afterAuthCallback: func(ctx *context.Context, err error) {
			if err != nil {
				log.Error("Failed to verify container user: %v", err)
				container.APIUnauthorizedError(ctx)
			}
		},
	})

	// TODO: Content Discovery / References (not implemented yet)

	r.Get("", container.ReqContainerAccess, container.DetermineSupport)
	r.Group("/token", func() {
		r.Get("", container.Authenticate)
		r.Post("", container.AuthenticateNotImplemented)
	})
	r.Get("/_catalog", container.ReqContainerAccess, container.GetRepositoryList)
	r.Group("/{username}", func() {
		r.PathGroup("/*", func(g *web.RouterPathGroup) {
			g.MatchPath("POST", "/<image:*>/blobs/uploads", reqPackageAccess(perm.AccessModeWrite), container.VerifyImageName, container.PostBlobsUploads)
			g.MatchPath("GET", "/<image:*>/tags/list", container.VerifyImageName, container.GetTagsList)

			patternBlobsUploadsUUID := g.PatternRegexp(`/<image:*>/blobs/uploads/<uuid:[-.=\w]+>`, reqPackageAccess(perm.AccessModeWrite), container.VerifyImageName)
			g.MatchPattern("GET", patternBlobsUploadsUUID, container.GetBlobsUpload)
			g.MatchPattern("PATCH", patternBlobsUploadsUUID, container.PatchBlobsUpload)
			g.MatchPattern("PUT", patternBlobsUploadsUUID, container.PutBlobsUpload)
			g.MatchPattern("DELETE", patternBlobsUploadsUUID, container.DeleteBlobsUpload)

			g.MatchPath("HEAD", `/<image:*>/blobs/<digest>`, container.VerifyImageName, container.HeadBlob)
			g.MatchPath("GET", `/<image:*>/blobs/<digest>`, container.VerifyImageName, container.GetBlob)
			g.MatchPath("DELETE", `/<image:*>/blobs/<digest>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.DeleteBlob)

			g.MatchPath("HEAD", `/<image:*>/manifests/<reference>`, container.VerifyImageName, container.HeadManifest)
			g.MatchPath("GET", `/<image:*>/manifests/<reference>`, container.VerifyImageName, container.GetManifest)
			g.MatchPath("PUT", `/<image:*>/manifests/<reference>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.PutManifest)
			g.MatchPath("DELETE", `/<image:*>/manifests/<reference>`, container.VerifyImageName, reqPackageAccess(perm.AccessModeWrite), container.DeleteManifest)
		})
	}, container.ReqContainerAccess, context.UserAssignmentWeb(), context.PackageAssignment(), reqPackageAccess(perm.AccessModeRead))

	return r
}
