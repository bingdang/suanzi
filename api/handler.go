package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"suanzi/db"
)

type CountRequest struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	UUID   string `json:"uuid"`
}

type InitRequest struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Path   string `json:"path"`
	Count  int    `json:"count"`
}

type CountResponse struct {
	Count int `json:"count"`
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

func CountHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		QueryHandler(w, r)
		return
	}
	
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Domain == "" || req.Type == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
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
	var siteID int
	err := dbConn.QueryRow("SELECT id FROM sites WHERE domain = ?", req.Domain).Scan(&siteID)
	if err == sql.ErrNoRows {
		// Create new site
		result, err := dbConn.Exec("INSERT INTO sites (domain) VALUES (?)", req.Domain)
		if err != nil {
			http.Error(w, "Failed to create site", http.StatusInternalServerError)
			return
		}
		siteID64, _ := result.LastInsertId()
		siteID = int(siteID64)
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Handle UV counting
	if req.Type == "site_uv" {
		// Check if this UUID already counted today
		today := time.Now().Format("2006-01-02")
		var exists bool
		err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM uv_records WHERE site_id = ? AND uuid = ? AND date = ?)", siteID, req.UUID, today).Scan(&exists)
		if err != nil {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		if !exists && req.UUID != "" {
			// Record new UV
			_, err := dbConn.Exec("INSERT INTO uv_records (site_id, uuid, date) VALUES (?, ?, ?)", siteID, req.UUID, today)
			if err != nil {
				http.Error(w, "Failed to record UV", http.StatusInternalServerError)
				return
			}
			// Increment counter
			_, err = dbConn.Exec("INSERT INTO counters (site_id, type, path, count) VALUES (?, ?, ?, 1) ON CONFLICT(site_id, type, path) DO UPDATE SET count = count + 1, updated_at = CURRENT_TIMESTAMP", siteID, req.Type, req.Path)
			if err != nil {
				http.Error(w, "Failed to update counter", http.StatusInternalServerError)
				return
			}
		}
	} else {
		// Handle PV counting
		_, err := dbConn.Exec("INSERT INTO counters (site_id, type, path, count) VALUES (?, ?, ?, 1) ON CONFLICT(site_id, type, path) DO UPDATE SET count = count + 1, updated_at = CURRENT_TIMESTAMP", siteID, req.Type, req.Path)
		if err != nil {
			http.Error(w, "Failed to update counter", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func QueryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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

	var count int
	err = dbConn.QueryRow("SELECT count FROM counters WHERE site_id = ? AND type = ? AND path = ?", siteID, counterType, path).Scan(&count)
	if err == sql.ErrNoRows {
		count = 0
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	response := CountResponse{Count: count}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func InitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	var siteID int
	err := dbConn.QueryRow("SELECT id FROM sites WHERE domain = ?", req.Domain).Scan(&siteID)
	if err == sql.ErrNoRows {
		result, err := dbConn.Exec("INSERT INTO sites (domain) VALUES (?)", req.Domain)
		if err != nil {
			http.Error(w, "Failed to create site", http.StatusInternalServerError)
			return
		}
		siteID64, _ := result.LastInsertId()
		siteID = int(siteID64)
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
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