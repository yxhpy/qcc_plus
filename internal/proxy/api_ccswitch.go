package proxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"qcc_plus/internal/importer/ccswitch"
)

func (p *Server) handleCCSwitchImport(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	if err := r.ParseMultipartForm(128 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	acc, ok := p.resolveCCSwitchAccount(w, r)
	if !ok {
		return
	}

	src, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file required"})
		return
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "qccplus-ccswitch-import-*.db")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, src); err != nil {
		tmpFile.Close()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("save upload failed: %v", err)})
		return
	}
	if err := tmpFile.Close(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	importProviders, err := parseCCSwitchBool(r.FormValue("import_providers"), true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid import_providers"})
		return
	}
	importPricing, err := parseCCSwitchBool(r.FormValue("import_pricing"), true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid import_pricing"})
		return
	}
	importLogs, err := parseCCSwitchBool(r.FormValue("import_logs"), true)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid import_logs"})
		return
	}

	weightOffset := 0
	if raw := strings.TrimSpace(r.FormValue("weight_offset")); raw != "" {
		weightOffset, err = strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid weight_offset"})
			return
		}
	}

	summary, err := ccswitch.RunWithTargetStore(r.Context(), p.store, ccswitch.Options{
		SourcePath:      tmpPath,
		AccountID:       acc.ID,
		WeightOffset:    weightOffset,
		ImportProviders: importProviders,
		ImportPricing:   importPricing,
		ImportLogs:      importLogs,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if summary.ProvidersImported > 0 {
		if err := p.reloadAccountNodesFromStore(r.Context(), acc.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("import completed but runtime reload failed: %v", err)})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"summary":   summary,
		"file_name": header.Filename,
	})
}

func (p *Server) handleCCSwitchExport(w http.ResponseWriter, r *http.Request) {
	if p.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	acc, ok := p.resolveCCSwitchAccount(w, r)
	if !ok {
		return
	}

	tmpFile, err := os.CreateTemp("", "qccplus-ccswitch-export-*.db")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer os.Remove(tmpPath)

	summary, err := ccswitch.ExportFromStore(r.Context(), p.store, ccswitch.ExportOptions{
		AccountID:  acc.ID,
		OutputPath: tmpPath,
		Overwrite:  true,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	file, err := os.Open(summary.OutputPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("cc-switch-%s.db", sanitizeCCSwitchFilename(acc.Name, acc.ID))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filename, stat.ModTime(), file)
}

func (p *Server) resolveCCSwitchAccount(w http.ResponseWriter, r *http.Request) (*Account, bool) {
	acc := accountFromCtx(r)
	if acc == nil {
		acc = p.defaultAccount
	}

	requested := strings.TrimSpace(r.FormValue("account_id"))
	if requested == "" {
		requested = strings.TrimSpace(r.URL.Query().Get("account_id"))
	}

	if isAdmin(r.Context()) {
		if requested != "" {
			target := p.getAccountByID(requested)
			if target == nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
				return nil, false
			}
			acc = target
		}
	} else if requested != "" && (acc == nil || requested != acc.ID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return nil, false
	}

	if acc == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account missing"})
		return nil, false
	}
	return acc, true
}

func parseCCSwitchBool(raw string, fallback bool) (bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func sanitizeCCSwitchFilename(parts ...string) string {
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z':
				return r
			case r >= 'A' && r <= 'Z':
				return r
			case r >= '0' && r <= '9':
				return r
			case r == '-' || r == '_' || r == '.':
				return r
			default:
				return '-'
			}
		}, part)
		part = strings.Trim(part, "-_.")
		if part != "" {
			return part
		}
	}
	return "export"
}
