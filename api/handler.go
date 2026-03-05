package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"suanzi/db"
)

// CountRequest 支持单类型和批量类型两种模式
type CountRequest struct {
	Domain string   `json:"domain"`
	Type   string   `json:"type"`    // 单类型模式
	Path   string   `json:"path"`
	UUID   string   `json:"uuid"`
	Types  []string `json:"types"`   // 批量模式
}

type InitRequest struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Count  int    `json:"count"`
}

type CountResponse struct {
	Count  int            `json:"count"`
	Counts map[string]int `json:"counts,omitempty"` // 批量模式返回
}

// validateReferer 检查 Referer 是否合法
// 规则：Referer 的域名必须与请求参数中的 domain 一致
// 例如：domain=example.com 时，Referer 必须是 https://example.com/xxx
func validateReferer(r *http.Request, domain string) bool {
	referer := r.Referer()
	if referer == "" {
		// 没有 Referer 的请求可能是直接调用，拒绝
		return false
	}

	u, err := url.Parse(referer)
	if err != nil {
		return false
	}

	// 提取 Referer 的主机名
	refererHost := u.Hostname()
	
	// Referer 主机名必须与 domain 参数一致
	if domain != "" && refererHost == domain {
		return true
	}

	return false
}

// incrementCounter 增加计数器
func incrementCounter(dbConn *sql.DB, siteID int, counterType string, path string) error {
	_, err := dbConn.Exec("INSERT INTO counters (site_id, type, path, count) VALUES (?, ?, ?, 1) ON CONFLICT(site_id, type, path) DO UPDATE SET count = count + 1, updated_at = CURRENT_TIMESTAMP", siteID, counterType, path)
	return err
}

// recordUV 记录 UV（按天去重）
// 返回 isNewUV 表示是否是新访客（今天第一次访问）
func recordUV(dbConn *sql.DB, siteID int, uuid string) (bool, error) {
	today := time.Now().Format("2006-01-02")
	
	// 检查是否已存在
	var exists bool
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM uv_records WHERE site_id = ? AND uuid = ? AND date = ?)", siteID, uuid, today).Scan(&exists)
	if err != nil {
		return false, err
	}

	if !exists && uuid != "" {
		// 记录新 UV
		_, err := dbConn.Exec("INSERT INTO uv_records (site_id, uuid, date) VALUES (?, ?, ?)", siteID, uuid, today)
		if err != nil {
			return false, err
		}
		return true, nil // 新访客
	}
	return false, nil // 老访客，今天已经计过 UV 了
}

// getCounterValue 获取计数值
func getCounterValue(dbConn *sql.DB, siteID int, counterType string, path string) (int, error) {
	var count int
	err := dbConn.QueryRow("SELECT count FROM counters WHERE site_id = ? AND type = ? AND path = ?", siteID, counterType, path).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return count, err
}

// getOrCreateSite 获取或创建站点
func getOrCreateSite(dbConn *sql.DB, domain string) (int, error) {
	var siteID int
	err := dbConn.QueryRow("SELECT id FROM sites WHERE domain = ?", domain).Scan(&siteID)
	if err == sql.ErrNoRows {
		// Create new site
		result, err := dbConn.Exec("INSERT INTO sites (domain) VALUES (?)", domain)
		if err != nil {
			return 0, err
		}
		siteID64, _ := result.LastInsertId()
		return int(siteID64), nil
	}
	return siteID, err
}

// setCORSHeaders 设置 CORS 跨域头
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func CountHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 OPTIONS 预检请求
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == http.MethodGet {
		QueryHandler(w, r)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 设置 CORS 头
	setCORSHeaders(w)

	var req CountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Domain == "" {
		http.Error(w, "Missing required field: domain", http.StatusBadRequest)
		return
	}

	// 校验 Referer，防止恶意刷量
	if !validateReferer(r, req.Domain) {
		http.Error(w, "Invalid referer", http.StatusForbidden)
		return
	}

	dbConn := db.GetDB()
	if dbConn == nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get or create site
	siteID, err := getOrCreateSite(dbConn, req.Domain)
	if err != nil {
		http.Error(w, "Failed to get or create site", http.StatusInternalServerError)
		return
	}

	// 批量模式：处理 types 数组
	if len(req.Types) > 0 {
		counts := make(map[string]int)
		
		// 检查是否有 page_pv，如果有则需要同时增加 site_pv
		hasPagePV := false
		for _, t := range req.Types {
			if t == "page_pv" {
				hasPagePV = true
				break
			}
		}
		
		// 如果有 page_pv 但没有 site_pv，自动添加 site_pv
		typesToProcess := req.Types
		if hasPagePV {
			hasSitePV := false
			for _, t := range req.Types {
				if t == "site_pv" {
					hasSitePV = true
					break
				}
			}
			if !hasSitePV {
				// 复制数组并添加 site_pv
				typesToProcess = make([]string, len(req.Types)+1)
				copy(typesToProcess, req.Types)
				typesToProcess[len(req.Types)] = "site_pv"
			}
		}
		
		for _, counterType := range typesToProcess {
			path := req.Path
			if counterType != "page_pv" {
				path = ""
			}
			
			// UV 特殊处理：只有新访客才增加计数
			if counterType == "site_uv" {
				isNewUV, err := recordUV(dbConn, siteID, req.UUID)
				if err != nil {
					http.Error(w, "Failed to record UV", http.StatusInternalServerError)
					return
				}
				if isNewUV {
					// 新访客，增加 UV 计数
					if err := incrementCounter(dbConn, siteID, counterType, path); err != nil {
						http.Error(w, "Failed to update counter", http.StatusInternalServerError)
						return
					}
				}
			} else {
				// PV 计数：直接增加
				if err := incrementCounter(dbConn, siteID, counterType, path); err != nil {
					http.Error(w, "Failed to update counter", http.StatusInternalServerError)
					return
				}
			}
			
			// 获取计数值（只返回原始请求的类型）
			for _, originalType := range req.Types {
				if counterType == originalType {
					count, err := getCounterValue(dbConn, siteID, counterType, path)
					if err != nil {
						http.Error(w, "Failed to get counter value", http.StatusInternalServerError)
						return
					}
					counts[counterType] = count
					break
				}
			}
		}

		// 返回批量结果
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CountResponse{Counts: counts})
		return
	}

	// 单类型模式：向后兼容
	if req.Type == "" {
		http.Error(w, "Missing required field: type or types", http.StatusBadRequest)
		return
	}

	// UV 特殊处理：只有新访客才增加计数
	if req.Type == "site_uv" {
		isNewUV, err := recordUV(dbConn, siteID, req.UUID)
		if err != nil {
			http.Error(w, "Failed to record UV", http.StatusInternalServerError)
			return
		}
		if !isNewUV {
			// 老访客，今天已经计过 UV 了，不增加计数
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// 增加计数器
	if err := incrementCounter(dbConn, siteID, req.Type, req.Path); err != nil {
		http.Error(w, "Failed to update counter", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func QueryHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 OPTIONS 预检请求
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 设置 CORS 头
	setCORSHeaders(w)

	domain := r.URL.Query().Get("domain")
	counterType := r.URL.Query().Get("type")
	path := r.URL.Query().Get("path")

	if domain == "" || counterType == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	dbConn := db.GetDB()
	if dbConn == nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	var siteID int
	err := dbConn.QueryRow("SELECT id FROM sites WHERE domain = ?", domain).Scan(&siteID)
	if err == sql.ErrNoRows {
		// Site doesn't exist, return 0
		response := CountResponse{Count: 0}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	count, err := getCounterValue(dbConn, siteID, counterType, path)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := CountResponse{Count: count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func InitHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 OPTIONS 预检请求
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 设置 CORS 头
	setCORSHeaders(w)

	var req InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Domain == "" || req.Type == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	dbConn := db.GetDB()
	if dbConn == nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Get or create site
	siteID, err := getOrCreateSite(dbConn, req.Domain)
	if err != nil {
		http.Error(w, "Failed to get or create site", http.StatusInternalServerError)
		return
	}

	// Initialize counter
	_, err = dbConn.Exec("INSERT INTO counters (site_id, type, path, count) VALUES (?, ?, ?, ?) ON CONFLICT(site_id, type, path) DO UPDATE SET count = excluded.count, updated_at = CURRENT_TIMESTAMP", siteID, req.Type, req.Path, req.Count)
	if err != nil {
		http.Error(w, "Failed to initialize counter", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// StatsHandler 返回统计信息（站点总数等）
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	// 处理 OPTIONS 预检请求
	if r.Method == http.MethodOptions {
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// 设置 CORS 头
	setCORSHeaders(w)

	dbConn := db.GetDB()
	if dbConn == nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 获取站点总数
	var siteCount int
	err := dbConn.QueryRow("SELECT COUNT(*) FROM sites").Scan(&siteCount)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 获取总 PV 数
	var totalPV int
	err = dbConn.QueryRow("SELECT COALESCE(SUM(count), 0) FROM counters WHERE type IN ('site_pv', 'page_pv')").Scan(&totalPV)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 获取总 UV 数
	var totalUV int
	err = dbConn.QueryRow("SELECT COALESCE(SUM(count), 0) FROM counters WHERE type = 'site_uv'").Scan(&totalUV)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := map[string]int{
		"sites": siteCount,
		"total_pv": totalPV,
		"total_uv": totalUV,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
