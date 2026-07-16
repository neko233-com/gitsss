package admin

import (
	"net/http"
	"strconv"

	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/templates"
	"github.com/neko233-com/gitsss/services/context"
	"github.com/neko233-com/gitsss/services/storageawareness"
)

const tplStorageAwareness templates.TplName = "admin/storage_awareness"

func StorageAwareness(ctx *context.Context) {
	ctx.Data["Title"] = ctx.Tr("admin.storage_awareness.title")
	ctx.Data["PageIsAdminStorageAwareness"] = true
	service, err := storageawareness.Get()
	if err != nil {
		ctx.ServerError("storage awareness", err)
		return
	}
	ctx.Data["StorageSnapshot"] = service.Snapshot()
	ctx.Data["StorageCleanupPlan"] = service.CleanupPlan()
	repositories, err := storageawareness.RepositoryUsages(ctx)
	if err != nil {
		ctx.ServerError("repository storage usage", err)
		return
	}
	ctx.Data["RepositoryUsages"] = repositories
	garbageService, err := storageawareness.Garbage()
	if err != nil {
		ctx.ServerError("garbage scanner", err)
		return
	}
	ctx.Data["GarbageReport"] = garbageService.Latest()
	ctx.HTML(http.StatusOK, tplStorageAwareness)
}

func ScanStorageGarbage(ctx *context.Context) {
	service, err := storageawareness.Garbage()
	if err != nil {
		ctx.ServerError("garbage scanner", err)
		return
	}
	report, err := service.Scan()
	if err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.storage_awareness.scan_success", report.Files, strconv.FormatInt(report.Bytes, 10)))
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/storage")
}

func CleanStorageGarbage(ctx *context.Context) {
	if ctx.FormString("confirm") != "yes" {
		ctx.Flash.Error(ctx.Tr("admin.storage_awareness.confirm_required"))
		ctx.Redirect(setting.AppSubURL + "/-/admin/storage")
		return
	}
	service, err := storageawareness.Garbage()
	if err != nil {
		ctx.ServerError("garbage scanner", err)
		return
	}
	bytes, files, err := service.Clean(ctx.FormString("scan_token"))
	if err != nil {
		ctx.Flash.Error(err.Error())
	} else {
		ctx.Flash.Success(ctx.Tr("admin.storage_awareness.clean_success", files, strconv.FormatInt(bytes, 10)))
	}
	ctx.Redirect(setting.AppSubURL + "/-/admin/storage")
}
