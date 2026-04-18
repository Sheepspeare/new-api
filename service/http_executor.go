package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	// DisableSSRFProtection 控制是否完全禁用SSRF保护
	// 可通过环境变量 MESSAGE_HOOK_DISABLE_SSRF_PROTECTION=true 设置
	DisableSSRFProtection bool

	// AllowHTTPProtocol 控制是否允许HTTP协议（默认只允许HTTPS）
	// 可通过环境变量 MESSAGE_HOOK_ALLOW_HTTP=true 设置
	AllowHTTPProtocol bool

	// AllowedHosts 白名单：允许的主机名或IP地址
	// 可通过环境变量 MESSAGE_HOOK_ALLOWED_HOSTS 设置，逗号分隔
	// 例如: MESSAGE_HOOK_ALLOWED_HOSTS=localhost,127.0.0.1,192.168.1.100
	AllowedHosts []string
)

func init() {
	// 从环境变量读取SSRF保护配置
	if val := os.Getenv("MESSAGE_HOOK_DISABLE_SSRF_PROTECTION"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			DisableSSRFProtection = b
			if DisableSSRFProtection {
				common.SysLog("WARNING: Message Hook SSRF protection is DISABLED. All IPs are allowed.")
			}
		}
	}

	// 从环境变量读取HTTP协议配置
	if val := os.Getenv("MESSAGE_HOOK_ALLOW_HTTP"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			AllowHTTPProtocol = b
			if AllowHTTPProtocol {
				common.SysLog("WARNING: Message Hook allows HTTP protocol (insecure).")
			}
		}
	}

	// 从环境变量读取白名单
	if val := os.Getenv("MESSAGE_HOOK_ALLOWED_HOSTS"); val != "" {
		hosts := strings.Split(val, ",")
		for _, host := range hosts {
			host = strings.TrimSpace(host)
			if host != "" {
				AllowedHosts = append(AllowedHosts, host)
			}
		}
		if len(AllowedHosts) > 0 {
			common.SysLog(fmt.Sprintf("Message Hook allowed hosts: %v", AllowedHosts))
		}
	}
}

// HTTPExecutor interface for calling external HTTP services
type HTTPExecutor interface {
	Execute(hookURL string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error)
}

// httpExecutor implements HTTPExecutor with connection pooling
type httpExecutor struct {
	client *http.Client
}

// NewHTTPExecutor creates a new HTTP executor with connection pooling
func NewHTTPExecutor() HTTPExecutor {
	return &httpExecutor{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				TLSClientConfig: &tls.Config{
					// 允许自签名证书和各种免费/官方证书
					// InsecureSkipVerify: true 会跳过所有验证，包括主机名验证
					// 这对于开发环境和自签名证书是必要的
					InsecureSkipVerify: true,
					// 支持更广泛的TLS版本以提高兼容性
					MinVersion: tls.VersionTLS10,
					MaxVersion: tls.VersionTLS13,
				},
			},
		},
	}
}

// Execute calls an external HTTP service with the given input and timeout
func (e *httpExecutor) Execute(hookURL string, input *dto.HookInput, timeout time.Duration) (*dto.HookOutput, error) {
	if hookURL == "" {
		return nil, errors.New("URL is empty")
	}

	if input == nil {
		return nil, errors.New("input is nil")
	}

	// Validate input
	if err := dto.ValidateHookInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Validate URL
	if err := validateHookURL(hookURL); err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Marshal input to JSON using common.Marshal
	jsonData, err := common.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", hookURL, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "new-api-message-hook/1.0")

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("HTTP request timeout: %w", err)
		}
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Parse response JSON using common.Unmarshal
	var output dto.HookOutput
	if err := common.Unmarshal(body, &output); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	// Validate output
	if err := dto.ValidateHookOutput(&output); err != nil {
		return nil, fmt.Errorf("invalid output: %w", err)
	}

	return &output, nil
}

// validateHookURL validates the URL for security
func validateHookURL(hookURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(hookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check protocol
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return errors.New("only HTTP and HTTPS URLs are allowed")
	}

	// Get hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		return errors.New("URL must have a hostname")
	}

	// Get port
	port := parsedURL.Port()
	var portNum int
	if port != "" {
		portNum, err = strconv.Atoi(port)
		if err != nil {
			return fmt.Errorf("invalid port number: %w", err)
		}
	}

	// Check if SSRF protection is disabled
	if DisableSSRFProtection {
		return nil
	}

	// Resolve hostname to IP addresses
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname: %w", err)
	}

	// Check if any resolved IP is private
	isPrivate := false
	for _, ip := range ips {
		if isPrivateIP(ip) {
			isPrivate = true
			break
		}
	}

	// If it's a private IP, check special cases
	if isPrivate {
		// 默认白名单：允许内网/本地地址使用端口 55566-55569
		// 这些端口通常用于内部webhook服务
		if portNum >= 55566 && portNum <= 55569 {
			// 端口在白名单范围内，允许HTTP和HTTPS
			return nil
		}

		// Check if host is in whitelist
		if len(AllowedHosts) > 0 {
			for _, allowedHost := range AllowedHosts {
				if hostname == allowedHost {
					// Host is whitelisted, but still need to check HTTP protocol
					if parsedURL.Scheme == "http" && !AllowHTTPProtocol {
						return errors.New("only HTTPS URLs are allowed for whitelisted hosts (set MESSAGE_HOOK_ALLOW_HTTP=true to allow HTTP)")
					}
					return nil
				}
			}
		}

		// Private IP not in whitelist and not using default ports
		return fmt.Errorf("private IP addresses are not allowed: %s (use ports 55566-55569 for local webhooks, or add to MESSAGE_HOOK_ALLOWED_HOSTS, or set MESSAGE_HOOK_DISABLE_SSRF_PROTECTION=true)", ips[0].String())
	}

	// For public IPs, require HTTPS unless HTTP is explicitly allowed
	if parsedURL.Scheme == "http" && !AllowHTTPProtocol {
		return errors.New("only HTTPS URLs are allowed (set MESSAGE_HOOK_ALLOW_HTTP=true to allow HTTP)")
	}

	return nil
}

// isPrivateIP checks if an IP address is private (SSRF protection)
func isPrivateIP(ip net.IP) bool {
	// Check for IPv4 private ranges
	if ip.To4() != nil {
		// 10.0.0.0/8
		if ip[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip[0] == 192 && ip[1] == 168 {
			return true
		}
		// 127.0.0.0/8 (localhost)
		if ip[0] == 127 {
			return true
		}
		// 169.254.0.0/16 (link-local)
		if ip[0] == 169 && ip[1] == 254 {
			return true
		}
	}

	// Check for IPv6 private ranges
	if ip.To16() != nil {
		// ::1 (localhost)
		if ip.IsLoopback() {
			return true
		}
		// fe80::/10 (link-local)
		if ip.IsLinkLocalUnicast() {
			return true
		}
		// fc00::/7 (unique local)
		if len(ip) >= 1 && (ip[0]&0xfe) == 0xfc {
			return true
		}
	}

	return false
}

// ValidateHTTPHookURL is a public wrapper for validateHookURL
func ValidateHTTPHookURL(hookURL string) error {
	return validateHookURL(hookURL)
}
