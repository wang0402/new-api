package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

const (
	playgroundImageTaskStatusPending   = "pending"
	playgroundImageTaskStatusRunning   = "running"
	playgroundImageTaskStatusSucceeded = "succeeded"
	playgroundImageTaskStatusFailed    = "failed"

	playgroundImageTaskTTL          = 30 * time.Minute
	playgroundImageMaxDownloadBytes = 50 << 20
)

type playgroundImageTask struct {
	ID        string    `json:"id"`
	UserID    int       `json:"-"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Response  any       `json:"response,omitempty"`
	Error     any       `json:"error,omitempty"`
}

type playgroundImageDownloadRequest struct {
	URL      string `json:"url"`
	Filename string `json:"filename,omitempty"`
}

var playgroundImageTasks = struct {
	sync.RWMutex
	items map[string]*playgroundImageTask
}{
	items: make(map[string]*playgroundImageTask),
}

func SubmitPlaygroundImageTask(c *gin.Context) {
	bodyStorage, err := common.GetBodyStorage(c)
	if err != nil {
		statusCode := http.StatusBadRequest
		if common.IsRequestBodyTooLargeError(err) {
			statusCode = http.StatusRequestEntityTooLarge
		}
		c.JSON(statusCode, gin.H{
			"error": types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, statusCode, types.ErrOptionWithSkipRetry()).ToOpenAIError(),
		})
		return
	}
	body, err := bodyStorage.Bytes()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry()).ToOpenAIError(),
		})
		return
	}

	task := &playgroundImageTask{
		ID:        common.GetRandomString(24),
		UserID:    c.GetInt("id"),
		Status:    playgroundImageTaskStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	storePlaygroundImageTask(task)

	headers := clonePlaygroundTaskHeaders(c.Request.Header)
	path := c.Request.URL.RequestURI()
	ctxValues := clonePlaygroundTaskContext(c)

	gopool.Go(func() {
		runPlaygroundImageTask(task.ID, body, headers, path, ctxValues)
	})

	c.JSON(http.StatusAccepted, gin.H{
		"id":         task.ID,
		"status":     task.Status,
		"created_at": task.CreatedAt,
		"updated_at": task.UpdatedAt,
	})
}

func GetPlaygroundImageTask(c *gin.Context) {
	taskID := c.Param("task_id")
	task, ok := loadPlaygroundImageTask(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "图片生成任务不存在或已过期",
				"type":    "invalid_request_error",
				"code":    "task_not_found",
			},
		})
		return
	}
	if task.UserID != c.GetInt("id") {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": "无权查看该图片生成任务",
				"type":    "access_denied",
				"code":    "access_denied",
			},
		})
		return
	}

	c.JSON(http.StatusOK, task)
}

func DownloadPlaygroundImage(c *gin.Context) {
	var req playgroundImageDownloadRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "参数错误"})
		return
	}

	imageURL := strings.TrimSpace(req.URL)
	if err := validatePlaygroundImageDownloadURL(c.Request.Context(), imageURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("图片地址重定向次数过多")
			}
			return validatePlaygroundImageDownloadURL(req.Context(), req.URL.String())
		},
	}
	downloadReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, imageURL, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "图片地址无效"})
		return
	}
	downloadReq.Header.Set("User-Agent", "new-api-playground-image-downloader/1.0")

	resp, err := httpClient.Do(downloadReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "下载图片失败"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("下载图片失败，状态码: %d", resp.StatusCode)})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	if !strings.HasPrefix(mediaType, "image/") {
		c.JSON(http.StatusBadGateway, gin.H{"message": "远程地址不是图片"})
		return
	}

	reader := io.LimitReader(resp.Body, playgroundImageMaxDownloadBytes+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取图片失败"})
		return
	}
	if len(body) > playgroundImageMaxDownloadBytes {
		c.JSON(http.StatusBadGateway, gin.H{"message": "图片文件过大"})
		return
	}

	filename := sanitizePlaygroundImageFilename(req.Filename, mediaType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Data(http.StatusOK, mediaType, body)
}

func runPlaygroundImageTask(taskID string, body []byte, headers http.Header, path string, ctxValues map[string]any) {
	updatePlaygroundImageTask(taskID, func(task *playgroundImageTask) {
		task.Status = playgroundImageTaskStatusRunning
	})

	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("图片生成任务异常: %v", r)
			common.SysError(errMsg)
			updatePlaygroundImageTask(taskID, func(task *playgroundImageTask) {
				task.Status = playgroundImageTaskStatusFailed
				task.Error = gin.H{
					"message": errMsg,
					"type":    "new_api_error",
					"code":    "internal_error",
				}
			})
		}
		cleanupExpiredPlaygroundImageTasks()
	}()

	recorder := httptest.NewRecorder()
	taskCtx, _ := gin.CreateTestContext(recorder)
	taskCtx.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	taskCtx.Request.Header = headers.Clone()

	for key, value := range ctxValues {
		taskCtx.Set(key, value)
	}
	if storage, err := common.CreateBodyStorage(body); err == nil {
		taskCtx.Set(common.KeyBodyStorage, storage)
		defer common.CleanupBodyStorage(taskCtx)
	}

	if newAPIError := preparePlaygroundRelayContext(taskCtx, types.RelayFormatOpenAIImage); newAPIError != nil {
		setPlaygroundImageTaskError(taskID, newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	Relay(taskCtx, types.RelayFormatOpenAIImage)

	var response any
	responseBody := recorder.Body.Bytes()
	if len(responseBody) > 0 {
		if err := common.Unmarshal(responseBody, &response); err != nil {
			response = string(responseBody)
		}
	}

	if recorder.Code >= http.StatusBadRequest {
		setPlaygroundImageTaskError(taskID, recorder.Code, response)
		return
	}

	updatePlaygroundImageTask(taskID, func(task *playgroundImageTask) {
		task.Status = playgroundImageTaskStatusSucceeded
		task.Response = response
	})
}

func setPlaygroundImageTaskError(taskID string, statusCode int, response any) {
	updatePlaygroundImageTask(taskID, func(task *playgroundImageTask) {
		task.Status = playgroundImageTaskStatusFailed
		task.Error = response
		if task.Error == nil {
			task.Error = gin.H{
				"message": fmt.Sprintf("图片生成失败，状态码: %d", statusCode),
				"type":    "new_api_error",
				"code":    "image_generation_failed",
			}
		}
	})
}

func storePlaygroundImageTask(task *playgroundImageTask) {
	playgroundImageTasks.Lock()
	defer playgroundImageTasks.Unlock()
	playgroundImageTasks.items[task.ID] = task
}

func loadPlaygroundImageTask(taskID string) (*playgroundImageTask, bool) {
	playgroundImageTasks.RLock()
	task, ok := playgroundImageTasks.items[taskID]
	playgroundImageTasks.RUnlock()
	if !ok {
		return nil, false
	}

	copyTask := *task
	return &copyTask, true
}

func updatePlaygroundImageTask(taskID string, update func(task *playgroundImageTask)) {
	playgroundImageTasks.Lock()
	defer playgroundImageTasks.Unlock()

	task, ok := playgroundImageTasks.items[taskID]
	if !ok {
		return
	}
	update(task)
	task.UpdatedAt = time.Now()
}

func cleanupExpiredPlaygroundImageTasks() {
	expiresBefore := time.Now().Add(-playgroundImageTaskTTL)

	playgroundImageTasks.Lock()
	defer playgroundImageTasks.Unlock()
	for taskID, task := range playgroundImageTasks.items {
		if task.UpdatedAt.Before(expiresBefore) {
			delete(playgroundImageTasks.items, taskID)
		}
	}
}

func clonePlaygroundTaskHeaders(headers http.Header) http.Header {
	cloned := headers.Clone()
	cloned.Set("Content-Type", "application/json")
	return cloned
}

func validatePlaygroundImageDownloadURL(ctx context.Context, rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("图片地址不能为空")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("图片地址无效")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅支持 http 或 https 图片地址")
	}

	hostname := parsed.Hostname()
	if ip := net.ParseIP(hostname); ip != nil {
		if !isPublicPlaygroundImageIP(ip) {
			return fmt.Errorf("不允许下载内网图片地址")
		}
		return nil
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, hostname)
	if err != nil || len(addrs) == 0 {
		return fmt.Errorf("无法解析图片地址")
	}
	for _, addr := range addrs {
		if !isPublicPlaygroundImageIP(addr.IP) {
			return fmt.Errorf("不允许下载内网图片地址")
		}
	}
	return nil
}

func isPublicPlaygroundImageIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsUnspecified() &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast()
}

func sanitizePlaygroundImageFilename(filename string, mediaType string) string {
	filename = filepath.Base(strings.TrimSpace(filename))
	if filename == "." || filename == "/" || filename == "" {
		filename = "playground-image"
	}
	filename = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		default:
			return r
		}
	}, filename)

	if filepath.Ext(filename) == "" {
		extensions, _ := mime.ExtensionsByType(mediaType)
		extension := ".png"
		if len(extensions) > 0 {
			extension = extensions[0]
		}
		filename += extension
	}
	return filename
}

func clonePlaygroundTaskContext(c *gin.Context) map[string]any {
	keys := []string{
		"id",
		"username",
		"role",
		"group",
		"user_group",
		"use_access_token",
		string(constant.ContextKeyRequestStartTime),
		string(constant.ContextKeyUserId),
		string(constant.ContextKeyUserName),
		string(constant.ContextKeyUserGroup),
		string(constant.ContextKeyUsingGroup),
		string(constant.ContextKeyLanguage),
		string(constant.ContextKeyChannelId),
		string(constant.ContextKeyChannelName),
		string(constant.ContextKeyChannelType),
		string(constant.ContextKeyChannelCreateTime),
		string(constant.ContextKeyChannelSetting),
		string(constant.ContextKeyChannelOtherSetting),
		string(constant.ContextKeyChannelParamOverride),
		string(constant.ContextKeyChannelHeaderOverride),
		string(constant.ContextKeyChannelOrganization),
		string(constant.ContextKeyChannelAutoBan),
		string(constant.ContextKeyChannelModelMapping),
		string(constant.ContextKeyChannelStatusCodeMapping),
		string(constant.ContextKeyChannelIsMultiKey),
		string(constant.ContextKeyChannelMultiKeyIndex),
		string(constant.ContextKeyChannelKey),
		string(constant.ContextKeyChannelBaseUrl),
		string(constant.ContextKeyOriginalModel),
		string(constant.ContextKeyTokenGroup),
		string(constant.ContextKeyTokenCrossGroupRetry),
	}

	values := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := c.Get(key); ok {
			values[key] = value
		}
	}
	return values
}
