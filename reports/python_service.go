package reports

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

//go:embed python/*.py python/requirements.txt
var pythonScripts embed.FS

// Timeouts for the helper scripts. Report generation uses Settings.JobTimeout.
const (
	dependencyCheckTimeout = 2 * time.Minute
	installTimeout         = 20 * time.Minute
)

// ErrInstallInProgress is returned when dependencies are already being installed.
var ErrInstallInProgress = errors.New("a dependency installation is already running")

// ErrPythonNotFound is returned when no Python 3 interpreter is on the PATH.
var ErrPythonNotFound = errors.New("Python 3 was not found in PATH")

// venvLock keeps report generation (readers) and dependency installation
// (writer, which may delete and recreate the venv) from overlapping.
var venvLock sync.RWMutex

var (
	venvDirOnce sync.Once
	venvDirPath string
)

// PythonService handles interaction with Python scripts for report generation
type PythonService struct {
	PythonPath string // system Python used to create the venv; "" when not found
}

// DependencyStatus represents the status of Python dependencies
type DependencyStatus struct {
	AllInstalled      bool   `json:"all_installed"`
	RequiredInstalled bool   `json:"required_installed"`
	PythonInstalled   bool   `json:"python_installed"`
	PythonVersion     string `json:"python_version"`
	VenvExists        bool   `json:"venv_exists"`
	VenvPython        string `json:"venv_python,omitempty"`
	VenvMessage       string `json:"venv_message,omitempty"`
	Dependencies      []struct {
		Name      string `json:"name"`
		Installed bool   `json:"installed"`
		Required  bool   `json:"required"`
		Version   string `json:"version"`
		Error     string `json:"error"`
	} `json:"dependencies"`
}

// NewPythonService creates a new Python service
func NewPythonService() *PythonService {
	return &PythonService{PythonPath: findPythonExecutable()}
}

// findPythonExecutable returns the system Python 3 to use, or "".
func findPythonExecutable() string {
	for _, name := range []string{"python3", "python"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	log.Warn("No Python executable found in PATH; report generation is unavailable")
	return ""
}

// VenvDir returns the absolute path of the persistent virtual environment
// (reports/python/venv under the working directory at first use).
func VenvDir() string {
	venvDirOnce.Do(func() {
		dir := filepath.Join("reports", "python", "venv")
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		venvDirPath = dir
	})
	return venvDirPath
}

// venvPython returns the venv's Python executable if the venv exists.
func venvPython() (string, bool) {
	var p string
	if runtime.GOOS == "windows" {
		p = filepath.Join(VenvDir(), "Scripts", "python.exe")
	} else {
		p = filepath.Join(VenvDir(), "bin", "python")
	}
	info, err := os.Stat(p)
	return p, err == nil && !info.IsDir()
}

// extractScripts writes the embedded Python scripts to a new temporary
// directory and returns it. The caller removes it.
func extractScripts() (string, error) {
	dir, err := os.MkdirTemp("", "anglerphish-report")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	err = fs.WalkDir(pythonScripts, "python", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, err := pythonScripts.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, filepath.Base(path)), content, 0600)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("failed to extract Python scripts: %v", err)
	}
	return dir, nil
}

// runScript runs a Python script with a timeout and returns stdout.
func runScript(ctx context.Context, python, script string, args []string, env []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, python, append([]string{script}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("%s timed out", filepath.Base(script))
	}
	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}
	if err != nil {
		if msg := stderr.String(); msg != "" {
			log.Errorf("Python script %s failed: %s", filepath.Base(script), msg)
			return nil, fmt.Errorf("Python error: %v\nDetails: %s", err, msg)
		}
		return nil, fmt.Errorf("Python error with no stderr output: %v", err)
	}
	if stderr.Len() > 0 {
		log.Debugf("Python script %s stderr: %s", filepath.Base(script), stderr.String())
	}
	return stdout.Bytes(), nil
}

// GenerateReport runs the report generator on the given campaign data.
// It uses the venv Python when the venv exists, otherwise system Python.
func (s *PythonService) GenerateReport(ctx context.Context, format ReportFormat, data []byte, options ReportOptions, settings Settings) ([]byte, error) {
	venvLock.RLock()
	defer venvLock.RUnlock()

	pythonExe, ok := venvPython()
	if !ok {
		if s.PythonPath == "" {
			return nil, ErrPythonNotFound
		}
		pythonExe = s.PythonPath
	}

	dir, err := extractScripts()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	outputPath := filepath.Join(dir, "report."+format.Extension())
	args := []string{"--format", string(format), "--output", outputPath}
	if options.AnonymizeEmails {
		args = append(args, "--anonymize-emails")
	}
	if options.AnonymizeIPs {
		args = append(args, "--anonymize-ips")
	}
	if settings.OnlineGeolocation {
		args = append(args, "--online-geolocation")
	}
	env := []string{"PYTHONIOENCODING=utf-8"}
	if settings.GeoIPDatabasePath != "" {
		env = append(env, "ANGLERPHISH_GEOIP_DB="+settings.GeoIPDatabasePath)
	}

	log.Infof("Generating %s report using %s", format, pythonExe)
	if _, err := runScript(ctx, pythonExe, filepath.Join(dir, "generate_report.py"), args, env, data); err != nil {
		return nil, err
	}

	reportData, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("report file was not created: %v", err)
	}
	if len(reportData) == 0 {
		return nil, fmt.Errorf("report file was created but is empty")
	}
	return reportData, nil
}

// InstallDependencies installs the Python dependencies into the virtual
// environment, creating it if needed. Only one installation runs at a
// time, and none runs while a report is being generated.
func (s *PythonService) InstallDependencies(ctx context.Context) (map[string]interface{}, error) {
	if s.PythonPath == "" {
		return nil, ErrPythonNotFound
	}
	if !venvLock.TryLock() {
		return nil, ErrInstallInProgress
	}
	defer venvLock.Unlock()

	dir, err := extractScripts()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(ctx, installTimeout)
	defer cancel()
	log.Infof("Installing Python dependencies using %s (venv: %s)", s.PythonPath, VenvDir())
	out, err := runScript(ctx, s.PythonPath, filepath.Join(dir, "install_dependencies.py"), nil,
		[]string{"ANGLERPHISH_VENV_DIR=" + VenvDir()}, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("failed to parse dependency installation output: %v\nOutput: %s", err, out)
	}
	return result, nil
}

// CheckDependencies reports whether Python and the required packages are installed.
func (s *PythonService) CheckDependencies(ctx context.Context) (*DependencyStatus, error) {
	if s.PythonPath == "" {
		return &DependencyStatus{PythonInstalled: false, VenvMessage: ErrPythonNotFound.Error()}, nil
	}
	dir, err := extractScripts()
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	ctx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()
	out, err := runScript(ctx, s.PythonPath, filepath.Join(dir, "check_dependencies.py"), nil,
		[]string{"ANGLERPHISH_VENV_DIR=" + VenvDir()}, nil)
	if err != nil {
		return nil, err
	}
	var status DependencyStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("failed to parse dependency check output: %v\nOutput: %s", err, out)
	}
	status.PythonInstalled = true
	return &status, nil
}

// ValidationResult contains validation messages and warnings
type ValidationResult struct {
	Valid               bool
	Errors              []string
	Warnings            []string
	IncompleteCampaigns []string
	MissingTimelineData []string
}

func isCampaignComplete(status string) bool {
	return status == models.CampaignComplete || status == "Complete"
}

// ValidateCampaignData checks campaigns owned by uid before report generation.
func ValidateCampaignData(campaignIDs []int64, uid int64) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:               true,
		Errors:              []string{},
		Warnings:            []string{},
		IncompleteCampaigns: []string{},
		MissingTimelineData: []string{},
	}

	if len(campaignIDs) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No campaign IDs provided")
		return result, nil
	}

	validCampaigns := 0
	for _, id := range campaignIDs {
		campaign, err := models.GetCampaign(id, uid)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Campaign ID %d: not found or access denied", id))
			continue
		}

		if !isCampaignComplete(campaign.Status) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d) is not completed (status: %s)", campaign.Name, id, campaign.Status))
			result.IncompleteCampaigns = append(result.IncompleteCampaigns, fmt.Sprintf("%s (ID: %d)", campaign.Name, id))
		}

		campaignResults, err := models.GetCampaignResults(id, uid)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d): Unable to retrieve results - %v", campaign.Name, id, err))
		} else {
			if len(campaignResults.Events) == 0 && len(campaignResults.Results) > 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d): No timeline data available - report metrics may be incomplete", campaign.Name, id))
				result.MissingTimelineData = append(result.MissingTimelineData, fmt.Sprintf("%s (ID: %d)", campaign.Name, id))
			}
			locked := 0
			for _, e := range campaignResults.Events {
				if e.DetailsLocked {
					locked++
				}
			}
			if locked > 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d): %d event(s) could not be decrypted - submitted data, IPs and browsers for them will be missing", campaign.Name, id, locked))
			}
		}

		validCampaigns++
	}

	if validCampaigns == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No valid campaigns found for report generation")
	}
	return result, nil
}

// campaignReportData builds the report data for one campaign owned by uid.
func campaignReportData(id, uid int64) (map[string]interface{}, error) {
	campaign, err := models.GetCampaign(id, uid)
	if err != nil {
		return nil, err
	}
	if !isCampaignComplete(campaign.Status) {
		log.Infof("Campaign ID %d (%s) is not completed (status: %s) - including with partial data", id, campaign.Name, campaign.Status)
	}

	campaignData := map[string]interface{}{
		"id":             campaign.Id,
		"name":           campaign.Name,
		"status":         campaign.Status,
		"type":           campaign.Type,
		"created_date":   campaign.CreatedDate,
		"launch_date":    campaign.LaunchDate,
		"completed_date": campaign.CompletedDate,
		"results":        []interface{}{},
		"timeline":       []interface{}{},
		"phish_url":      campaign.URL,
		"urlparam":       campaign.URLParam,
		"page_details": map[string]interface{}{
			"redirect_url":        campaign.Page.RedirectURL,
			"capture_credentials": campaign.Page.CaptureCredentials,
			"capture_passwords":   campaign.Page.CapturePasswords,
		},
	}
	if campaign.Type == "sms" {
		campaignData["sms_template"] = map[string]interface{}{
			"text": campaign.SMSTemplate.Text,
			"name": campaign.SMSTemplate.Name,
		}
		campaignData["sms"] = map[string]interface{}{
			"from":     campaign.SMS.From,
			"provider": campaign.SMS.Provider,
		}
	} else {
		campaignData["template_details"] = map[string]interface{}{
			"envelope_sender": campaign.Template.EnvelopeSender,
			"subject":         campaign.Template.Subject,
		}
	}

	campaignResults, err := models.GetCampaignResults(id, uid)
	if err != nil {
		log.Errorf("Error getting campaign results for ID %d: %v", id, err)
		return campaignData, nil
	}
	campaignData["results"] = campaignResults.Results
	campaignData["timeline"] = campaignResults.Events
	return campaignData, nil
}

// FetchCampaignData retrieves the report data for campaigns owned by uid.
func FetchCampaignData(campaignIDs []int64, uid int64) (*ReportData, error) {
	data := &ReportData{Campaigns: make([]map[string]interface{}, 0, len(campaignIDs))}
	for _, id := range campaignIDs {
		campaignData, err := campaignReportData(id, uid)
		if err != nil {
			log.Errorf("Failed to get campaign %d for user %d: %v", id, uid, err)
			continue
		}
		data.Campaigns = append(data.Campaigns, campaignData)
	}
	if len(data.Campaigns) == 0 {
		return nil, fmt.Errorf("no valid campaigns found - please ensure selected campaigns exist and you have access to them")
	}
	return data, nil
}

// FetchCampaignSetData retrieves the report data for a campaign set owned by uid.
func FetchCampaignSetData(campaignSetID, uid int64) (*ReportData, error) {
	campaignSet, err := models.GetCampaignSet(campaignSetID, uid)
	if err != nil {
		return nil, fmt.Errorf("campaign set %d not found or access denied", campaignSetID)
	}
	ids := make([]int64, len(campaignSet.Campaigns))
	for i, c := range campaignSet.Campaigns {
		ids[i] = c.Id
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("campaign set %d has no campaigns", campaignSetID)
	}
	// Load each campaign in full; the set only carries summaries
	data, err := FetchCampaignData(ids, uid)
	if err != nil {
		return nil, err
	}
	data.SetName = campaignSet.Name
	data.SetID = campaignSet.Id
	data.SetCreatedDate = campaignSet.CreatedDate
	data.SetLaunchDate = campaignSet.LaunchDate
	data.SetCompletedDate = campaignSet.CompletedDate
	data.SetStatus = campaignSet.Status
	data.SetURL = campaignSet.URL
	return data, nil
}

// reportTitle is a short human-readable description of the report contents.
func (d *ReportData) reportTitle() string {
	if d.SetName != "" {
		return d.SetName
	}
	if len(d.Campaigns) == 1 {
		if name, ok := d.Campaigns[0]["name"].(string); ok && name != "" {
			return name
		}
	}
	return fmt.Sprintf("%d campaigns", len(d.Campaigns))
}
