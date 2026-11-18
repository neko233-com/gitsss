// Copyright 2026 GitSSS contributors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"errors"
	"net/http"
	"time"

	api "github.com/neko233-com/gitsss/modules/structs"
	"github.com/neko233-com/gitsss/modules/web"
	"github.com/neko233-com/gitsss/services/context"
	dailyreport_service "github.com/neko233-com/gitsss/services/dailyreport"
)

// AnalyzeDailyReport analyzes only the authenticated user's own commits.
func AnalyzeDailyReport(ctx *context.APIContext) {
	// swagger:operation POST /user/daily-report/analyze user userAnalyzeDailyReport
	// ---
	// summary: Analyze the authenticated user's own commits with an organization agent
	// consumes:
	// - application/json
	// produces:
	// - application/json
	// parameters:
	// - name: body
	//   in: body
	//   required: true
	//   schema:
	//     type: object
	//     required: [organization_id]
	//     properties:
	//       organization_id:
	//         type: integer
	//         format: int64
	// responses:
	//   "200":
	//     description: Analysis of the authenticated user's own commits
	//     schema:
	//       type: object
	//       properties:
	//         organization_id:
	//           type: integer
	//           format: int64
	//         account:
	//           type: string
	//         full_name:
	//           type: string
	//         summary:
	//           type: string
	//   "400":
	//     "$ref": "#/responses/error"
	//   "401":
	//     "$ref": "#/responses/error"
	//   "403":
	//     "$ref": "#/responses/forbidden"
	//   "409":
	//     "$ref": "#/responses/conflict"
	//   "503":
	//     "$ref": "#/responses/error"

	form := web.GetForm(ctx).(*api.DailyReportAnalyzeOption)
	if form.OrganizationID <= 0 {
		ctx.APIError(http.StatusBadRequest, "organization_id must be positive")
		return
	}
	result, err := dailyreport_service.AnalyzeOwnOrganization(ctx, form.OrganizationID, ctx.Doer, time.Now())
	if err != nil {
		switch {
		case errors.Is(err, dailyreport_service.ErrDailyReportNotOrganizationMember):
			ctx.APIError(http.StatusForbidden, "organization membership is required")
		case errors.Is(err, dailyreport_service.ErrDailyReportAgentNotConfigured):
			ctx.APIError(http.StatusServiceUnavailable, "daily report agent is not configured")
		case errors.Is(err, dailyreport_service.ErrDailyReportAnalysisInProgress):
			ctx.APIError(http.StatusConflict, "daily report analysis is already running")
		default:
			ctx.APIErrorInternal(err)
		}
		return
	}
	ctx.JSON(http.StatusOK, api.DailyReportAnalysis{
		OrganizationID: form.OrganizationID,
		Account:        result.Account,
		FullName:       result.FullName,
		Summary:        result.Summary,
	})
}
