// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	user_model "github.com/neko233-com/gitsss/models/user"
	webhook_model "github.com/neko233-com/gitsss/models/webhook"
	"github.com/neko233-com/gitsss/modules/glob"
	"github.com/neko233-com/gitsss/modules/graceful"
	"github.com/neko233-com/gitsss/modules/hostmatcher"
	"github.com/neko233-com/gitsss/modules/log"
	"github.com/neko233-com/gitsss/modules/process"
	"github.com/neko233-com/gitsss/modules/proxy"
	"github.com/neko233-com/gitsss/modules/queue"
	"github.com/neko233-com/gitsss/modules/setting"
	"github.com/neko233-com/gitsss/modules/timeutil"
	"github.com/neko233-com/gitsss/modules/util"
	webhook_module "github.com/neko233-com/gitsss/modules/webhook"
)

func newDefaultRequest(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (req *http.Request, body []byte, err error) {
	switch w.HTTPMethod {
	case "", http.MethodPost:
		switch w.ContentType {
		case webhook_model.ContentTypeJSON:
			req, err = http.NewRequest(http.MethodPost, w.URL, strings.NewReader(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}

			req.Header.Set("Content-Type", "application/json")
		case webhook_model.ContentTypeForm:
			forms := url.Values{
				"payload": []string{t.PayloadContent},
			}

			req, err = http.NewRequest(http.MethodPost, w.URL, strings.NewReader(forms.Encode()))
			if err != nil {
				return nil, nil, err
			}

			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		default:
			return nil, nil, fmt.Errorf("invalid content type: %v", w.ContentType)
		}
	case http.MethodGet:
		u, err := url.Parse(w.URL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid URL: %w", err)
		}
		vals := u.Query()
		vals["payload"] = []string{t.PayloadContent}
		u.RawQuery = vals.Encode()
		req, err = http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, nil, err
		}
	case http.MethodPut:
		switch w.Type {
		case webhook_module.MATRIX: // used when t.Version == 1
			txnID, err := getMatrixTxnID([]byte(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}
			url := fmt.Sprintf("%s/%s", w.URL, url.PathEscape(txnID))
			req, err = http.NewRequest(http.MethodPut, url, strings.NewReader(t.PayloadContent))
			if err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, fmt.Errorf("invalid http method: %v", w.HTTPMethod)
		}
	default:
		return nil, nil, fmt.Errorf("invalid http method: %v", w.HTTPMethod)
	}

	body = []byte(t.PayloadContent)
	return req, body, addDefaultHeaders(req, []byte(w.Secret), w, t, body)
}

func addDefaultHeaders(req *http.Request, secret []byte, w *webhook_model.Webhook, t *webhook_model.HookTask, payloadContent []byte) error {
	var signatureSHA1 string
	var signatureSHA256 string
	if len(secret) > 0 {
		sig1 := hmac.New(sha1.New, secret)
		sig256 := hmac.New(sha256.New, secret)
		_, err := io.MultiWriter(sig1, sig256).Write(payloadContent)
		if err != nil {
			// this error should never happen, since the hashes are writing to []byte and always return a nil error.
			return fmt.Errorf("prepareWebhooks.sigWrite: %w", err)
		}
		signatureSHA1 = hex.EncodeToString(sig1.Sum(nil))
		signatureSHA256 = hex.EncodeToString(sig256.Sum(nil))
	}

	event := t.EventType.Event()
	eventType := string(t.EventType)
	targetType := "default"
	if w.IsSystemWebhook {
		targetType = "system"
	} else if w.RepoID != 0 {
		targetType = "repository"
	} else if w.OwnerID != 0 {
		owner, err := user_model.GetUserByID(req.Context(), w.OwnerID)
		if owner != nil && err == nil {
			if owner.IsOrganization() {
				targetType = "organization"
			} else {
				targetType = "user"
			}
		}
	}

	req.Header.Add("X-Gitea-Delivery", t.UUID)
	req.Header.Add("X-Gitea-Event", event)
	req.Header.Add("X-Gitea-Event-Type", eventType)
	req.Header.Add("X-Gitea-Signature", signatureSHA256)
	req.Header.Add("X-Gitea-Hook-Installation-Target-Type", targetType)
	req.Header.Add("X-Gogs-Delivery", t.UUID)
	req.Header.Add("X-Gogs-Event", event)
	req.Header.Add("X-Gogs-Event-Type", eventType)
	req.Header.Add("X-Gogs-Signature", signatureSHA256)
	req.Header.Add("X-Hub-Signature", "sha1="+signatureSHA1)
	req.Header.Add("X-Hub-Signature-256", "sha256="+signatureSHA256)
	req.Header["X-GitHub-Delivery"] = []string{t.UUID}
	req.Header["X-GitHub-Event"] = []string{event}
	req.Header["X-GitHub-Event-Type"] = []string{eventType}
	req.Header["X-GitHub-Hook-Installation-Target-Type"] = []string{targetType}
	return nil
}

// Deliver creates the [http.Request] (depending on the webhook type), sends it
// and records the status and response.
func Deliver(ctx context.Context, t *webhook_model.HookTask) error {
	w, err := webhook_model.GetWebhookByID(ctx, t.HookID)
	if err != nil {
		return err
	}

	defer func() {
		err := recover()
		if err == nil {
			return
		}
		// There was a panic whilst delivering a hook...
		log.Error("PANIC whilst trying to deliver webhook task[%d] to webhook %s Panic: %v\nStacktrace: %s", t.ID, notificationSafeURL(w.Type, w.URL), err, log.Stack(2))
	}()

	t.IsDelivered = true

	richNotification := false
	newRequest := webhookRequesters[w.Type]
	if w.Type == webhook_module.EMAIL {
		richNotification = true
		newRequest = notificationRichRequest
	} else if t.PayloadVersion == 1 || newRequest == nil {
		newRequest = newDefaultRequest
	} else if NotificationWebhookSupported(w.Type) {
		settings, parseErr := GetNotificationSettings(w.Meta)
		if parseErr != nil {
			return parseErr
		}
		if settings.Style != "legacy" {
			richNotification = true
			newRequest = notificationRichRequest
		}
	}

	var parts []notificationPart
	if richNotification {
		parts, err = notificationRichRequests(ctx, w, t)
	} else {
		req, body, requestErr := newRequest(ctx, w, t)
		err = requestErr
		parts = []notificationPart{{req, body}}
	}
	if err != nil {
		return fmt.Errorf("cannot create notification for webhook %s[%d]: %w", w.Type, w.ID, err)
	}
	req, body := parts[0].Request, parts[0].Body

	// Record delivery information.
	t.RequestInfo = &webhook_model.HookRequest{
		URL:        notificationSafeURL(w.Type, req.URL.String()),
		HTTPMethod: req.Method,
		Headers:    map[string]string{},
		Body:       string(body),
	}
	for k, vals := range req.Header {
		t.RequestInfo.Headers[k] = strings.Join(vals, ",")
	}

	if len(parts) > 1 {
		hashes := make([]string, 0, len(parts))
		for _, part := range parts {
			sum := sha256.Sum256(part.Body)
			hashes = append(hashes, hex.EncodeToString(sum[:]))
		}
		t.RequestInfo.Headers["X-GitSSS-Part-SHA256"] = strings.Join(hashes, ",")
		t.RequestInfo.Headers["X-GitSSS-Parts"] = strconv.Itoa(len(parts))
	}

	// Add Authorization Header
	authorization, err := w.HeaderAuthorization()
	if err != nil {
		return fmt.Errorf("cannot get Authorization header for webhook %s[%d %s]: %w", w.Type, w.ID, notificationSafeURL(w.Type, w.URL), err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
		t.RequestInfo.Headers["Authorization"] = "******"
	}

	t.ResponseInfo = &webhook_model.HookResponse{
		Headers: map[string]string{},
	}

	// OK We're now ready to attempt to deliver the task - we must double check that it
	// has not been delivered in the meantime
	updated, err := webhook_model.MarkTaskDelivered(ctx, t)
	if err != nil {
		log.Error("MarkTaskDelivered[%d]: %v", t.ID, err)
		return fmt.Errorf("unable to mark task[%d] delivered in the db: %w", t.ID, err)
	}
	if !updated {
		// This webhook task has already been attempted to be delivered or is in the process of being delivered
		log.Trace("Webhook Task[%d] already delivered", t.ID)
		return nil
	}

	// All code from this point will update the hook task
	defer func() {
		t.Delivered = timeutil.TimeStampNanoNow()
		if t.IsSucceed {
			log.Trace("Hook delivered: %s", t.UUID)
		} else if !w.IsActive {
			log.Trace("Hook delivery skipped as webhook is inactive: %s", t.UUID)
		} else {
			log.Trace("Hook delivery failed: %s", t.UUID)
		}

		if err := webhook_model.UpdateHookTask(ctx, t); err != nil {
			log.Error("UpdateHookTask [%d]: %v", t.ID, err)
		}

		// Update webhook last delivery status.
		if t.IsSucceed {
			w.LastStatus = webhook_module.HookStatusSucceed
		} else {
			w.LastStatus = webhook_module.HookStatusFail
		}
		if err = webhook_model.UpdateWebhookLastStatus(ctx, w); err != nil {
			log.Error("UpdateWebhookLastStatus: %v", err)
			return
		}
	}()

	if setting.DisableWebhooks {
		return fmt.Errorf("webhook task skipped (webhooks disabled): [%d]", t.ID)
	}

	if !w.IsActive {
		log.Trace("Webhook %s in Webhook Task[%d] is not active", notificationSafeURL(w.Type, w.URL), t.ID)
		return nil
	}
	completed := 0
	for index, part := range parts {
		if index > 0 {
			timer := time.NewTimer(notificationPartInterval(w.Type))
			select {
			case <-ctx.Done():
				timer.Stop()
				t.IsSucceed = false
				t.ResponseInfo.Body = fmt.Sprintf("Accepted %d/%d parts; delivery canceled.", completed, len(parts))
				return ctx.Err()
			case <-timer.C:
			}
		}
		if err := ctx.Err(); err != nil {
			t.IsSucceed = false
			t.ResponseInfo.Body = fmt.Sprintf("Accepted %d/%d parts; delivery canceled.", completed, len(parts))
			return err
		}
		if authorization != "" {
			part.Request.Header.Set("Authorization", authorization)
		}
		t.IsSucceed = false
		t.ResponseInfo.Headers["X-GitSSS-Parts"] = strconv.Itoa(len(parts))
		t.ResponseInfo.Headers["X-GitSSS-Parts-Accepted"] = strconv.Itoa(completed)
		if err := deliverNotificationPart(ctx, w, t, part); err != nil || !t.IsSucceed {
			t.ResponseInfo.Body = fmt.Sprintf("Accepted %d/%d parts; part %d failed. %s", completed, len(parts), index+1, t.ResponseInfo.Body)
			if err != nil {
				return err
			}
			return fmt.Errorf("notification part %d/%d rejected", index+1, len(parts))
		}
		completed++
		t.ResponseInfo.Headers["X-GitSSS-Parts-Accepted"] = strconv.Itoa(completed)
	}
	if len(parts) > 1 {
		t.ResponseInfo.Body = fmt.Sprintf("Accepted all %d parts. %s", len(parts), t.ResponseInfo.Body)
	}

	return nil
}

func deliverNotificationPart(ctx context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask, part notificationPart) error {
	if w.Type == webhook_module.EMAIL {
		t.RequestInfo.HTTPMethod = "SMTP"
		err := deliverNotificationEmail(w, part.Body)
		t.IsSucceed = err == nil
		if err != nil {
			t.ResponseInfo.Body = "SMTP delivery failed: " + err.Error()
			return err
		}
		t.ResponseInfo.Status = 250
		t.ResponseInfo.Body = "Accepted by the configured mail transport; inbox delivery is not confirmed."
		return nil
	}
	resp, err := webhookHTTPClient.Do(part.Request.WithContext(ctx))
	if err != nil {
		if NotificationWebhookSupported(w.Type) {
			if endpointError, ok := errors.AsType[*url.Error](err); ok {
				err = endpointError.Err
			}
		}
		t.ResponseInfo.Body = fmt.Sprintf("Delivery: %v", err)
		return fmt.Errorf("unable to deliver webhook task[%d]: %w", t.ID, err)
	}
	defer resp.Body.Close()
	t.IsSucceed = resp.StatusCode/100 == 2
	t.ResponseInfo.Status = resp.StatusCode
	for k, vals := range resp.Header {
		t.ResponseInfo.Headers[k] = strings.Join(vals, ",")
	}
	body, err := util.ReadWithLimit(resp.Body, 1024*1024)
	if err != nil {
		t.IsSucceed = false
		t.ResponseInfo.Body = "Unable to read provider response"
		return err
	}
	t.ResponseInfo.Body = string(body)
	if t.IsSucceed && NotificationWebhookSupported(w.Type) {
		if err := notificationResponseError(w.Type, body); err != nil {
			t.IsSucceed = false
			return err
		}
	}
	return nil
}

var (
	webhookHTTPClient *http.Client
	once              sync.Once
	hostMatchers      []glob.Glob
)

func webhookProxy(allowList *hostmatcher.HostMatchList) func(req *http.Request) (*url.URL, error) {
	if setting.Webhook.ProxyURL == "" {
		return proxy.Proxy()
	}

	once.Do(func() {
		for _, h := range setting.Webhook.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				if !allowList.MatchHostName(req.URL.Host) {
					return nil, fmt.Errorf("webhook can only call allowed HTTP servers (check your %s setting), deny '%s'", allowList.SettingKeyHint, req.URL.Host)
				}
				return http.ProxyURL(setting.Webhook.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}

// Init starts the hooks delivery thread
func Init() error {
	timeout := time.Duration(setting.Webhook.DeliverTimeout) * time.Second
	allowedHostMatcher := hostmatcher.ParseHostMatchList("security.ALLOWED_HOST_LIST", setting.Webhook.AllowedHostList)

	// NewHTTPTransport enforces the allow-list on direct connections; when webhookProxy routes a request
	// through a configured proxy, restricting the proxied target is the proxy server's responsibility.
	webhookHTTPClient = &http.Client{
		Timeout: timeout,
		Transport: hostmatcher.NewHTTPTransport("webhook", allowedHostMatcher, nil, webhookProxy(allowedHostMatcher), setting.Webhook.ProxyURLFixed,
			&tls.Config{InsecureSkipVerify: setting.Webhook.SkipTLSVerify}),
	}

	hookQueue = queue.CreateUniqueQueue(graceful.GetManager().ShutdownContext(), "webhook_sender", handler)
	if hookQueue == nil {
		return errors.New("unable to create webhook_sender queue")
	}
	go graceful.GetManager().RunWithCancel(hookQueue)

	go graceful.GetManager().RunWithShutdownContext(populateWebhookSendingQueue)

	return nil
}

func populateWebhookSendingQueue(ctx context.Context) {
	ctx, _, finished := process.GetManager().AddContext(ctx, "Webhook: Populate sending queue")
	defer finished()

	lowerID := int64(0)
	for {
		taskIDs, err := webhook_model.FindUndeliveredHookTaskIDs(ctx, lowerID)
		if err != nil {
			log.Error("Unable to populate webhook queue as FindUndeliveredHookTaskIDs failed: %v", err)
			return
		}
		if len(taskIDs) == 0 {
			return
		}
		lowerID = taskIDs[len(taskIDs)-1]

		for _, taskID := range taskIDs {
			select {
			case <-ctx.Done():
				log.Warn("Shutdown before Webhook Sending queue finishing being populated")
				return
			default:
			}
			if err := enqueueHookTask(taskID); err != nil {
				log.Error("Unable to push HookTask[%d] to the Webhook Sending queue: %v", taskID, err)
			}
		}
	}
}
