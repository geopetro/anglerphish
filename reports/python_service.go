package reports

import (
	"bytes"
	_ "embed" // Required for go:embed
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	log "github.com/gophish/gophish/logger"
	"github.com/gophish/gophish/models"
)

//go:embed python/generate_report.py
var generateReportScript string

//go:embed python/word_generator.py
var wordGeneratorScript string

//go:embed python/excel_generator.py
var excelGeneratorScript string

//go:embed python/check_dependencies.py
var checkDependenciesScript string

//go:embed python/install_dependencies.py
var installDependenciesScript string

//go:embed python/venv_manager.py
var venvManagerScript string

//go:embed python/requirements.txt
var requirementsFile string

// PythonService handles interaction with Python scripts for report generation
type PythonService struct {
	PythonPath string // Path to Python executable (system Python for initial setup)
}

// DependencyStatus represents the status of Python dependencies
type DependencyStatus struct {
	AllInstalled      bool   `json:"all_installed"`
	RequiredInstalled bool   `json:"required_installed"`
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
	pythonPath := findPythonExecutable()
	return &PythonService{
		PythonPath: pythonPath,
	}
}

// findPythonExecutable attempts to find the best Python executable
func findPythonExecutable() string {
	// Check if python3 is available (common on Linux/Mac)
	if pythonPath, err := exec.LookPath("python3"); err == nil {
		log.Infof("Found Python 3 at: %s", pythonPath)
		return "python3"
	}

	// Check if python is available (common on Windows)
	if pythonPath, err := exec.LookPath("python"); err == nil {
		log.Infof("Found Python at: %s", pythonPath)
		return "python"
	}

	log.Warn("No Python executable found in PATH. Defaulting to 'python'")
	return "python"
}

// getPersistentVenvDir returns the path to the persistent venv directory
// This is reports/python/venv relative to the current working directory
func getPersistentVenvDir() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Warnf("Could not get working directory, using relative path: %v", err)
		return filepath.Join("reports", "python", "venv")
	}
	return filepath.Join(cwd, "reports", "python", "venv")
}

// extractAllScripts writes all embedded Python scripts to the temporary directory
func (s *PythonService) extractAllScripts(tempDir string) error {
	pythonDir := filepath.Join(tempDir, "python")

	// Write venv_manager script (must be first as others depend on it)
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "venv_manager.py"), []byte(venvManagerScript), 0500); err != nil {
		return fmt.Errorf("failed to write venv_manager.py: %v", err)
	}

	// Write main script
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "generate_report.py"), []byte(generateReportScript), 0500); err != nil {
		return fmt.Errorf("failed to write generate_report.py: %v", err)
	}

	// Write word generator script
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "word_generator.py"), []byte(wordGeneratorScript), 0500); err != nil {
		return fmt.Errorf("failed to write word_generator.py: %v", err)
	}

	// Write excel generator script
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "excel_generator.py"), []byte(excelGeneratorScript), 0500); err != nil {
		return fmt.Errorf("failed to write excel_generator.py: %v", err)
	}

	// Write dependency check script
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "check_dependencies.py"), []byte(checkDependenciesScript), 0500); err != nil {
		return fmt.Errorf("failed to write check_dependencies.py: %v", err)
	}

	// Write install dependencies script
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "install_dependencies.py"), []byte(installDependenciesScript), 0500); err != nil {
		return fmt.Errorf("failed to write install_dependencies.py: %v", err)
	}

	// Write requirements.txt
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "requirements.txt"), []byte(requirementsFile), 0500); err != nil {
		return fmt.Errorf("failed to write requirements.txt: %v", err)
	}

	return nil
}

// GenerateReport generates a report using embedded Python scripts
// Uses venv Python if available, otherwise system Python
func (s *PythonService) GenerateReport(format ReportFormat, data []byte, options ReportOptions) ([]byte, error) {
	// Create temporary directory for scripts and output
	tempDir, err := ioutil.TempDir("", "gophish-report")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory structure
	pythonDir := filepath.Join(tempDir, "python")
	if err := os.MkdirAll(pythonDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create python directory: %v", err)
	}

	// Write all Python scripts to temporary directory
	if err := s.extractAllScripts(tempDir); err != nil {
		return nil, fmt.Errorf("failed to extract Python scripts: %v", err)
	}

	// Determine which Python to use (prefer venv)
	pythonExe := s.PythonPath
	venvDir := getPersistentVenvDir()

	// Check if venv exists by running venv_manager
	venvCheckCmd := exec.Command(s.PythonPath, filepath.Join(pythonDir, "venv_manager.py"), "--paths")
	venvCheckCmd.Env = append(os.Environ(), fmt.Sprintf("ANGLERPHISH_VENV_DIR=%s", venvDir))
	venvCheckOutput, err := venvCheckCmd.Output()
	if err == nil {
		var venvPaths struct {
			Exists bool   `json:"exists"`
			Python string `json:"python"`
		}
		if json.Unmarshal(venvCheckOutput, &venvPaths) == nil && venvPaths.Exists {
			// Use the venv Python from the actual venv location (not temp dir)
			pythonExe = venvPaths.Python
			log.Infof("Using venv Python for report generation: %s", pythonExe)
		}
	}

	// Determine output file extension
	extension := ".docx"
	if format == FormatExcel {
		extension = ".xlsx"
	}
	outputPath := filepath.Join(tempDir, "report"+extension)

	// Prepare command arguments
	mainScript := filepath.Join(pythonDir, "generate_report.py")
	args := []string{
		mainScript,
		"--format", string(format),
		"--output", outputPath,
	}

	// Add GDPR options
	if options.AnonymizeEmails {
		args = append(args, "--anonymize-emails")
	}
	if options.AnonymizeIPs {
		args = append(args, "--anonymize-ips")
	}
	if options.IncludeGDPRStatement {
		args = append(args, "--include-gdpr-statement")
	}

	// Add format-specific options
	if format == FormatWord && options.IncludeTOC {
		args = append(args, "--include-toc")
	}

	// Execute Python script
	cmd := exec.Command(pythonExe, args...)
	cmd.Stdin = bytes.NewReader(data)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Infof("Executing Python script for %s report generation using: %s", format, pythonExe)
	if err := cmd.Run(); err != nil {
		errOutput := stderr.String()
		if errOutput != "" {
			log.Errorf("Python script error output: %s", errOutput)
			return nil, fmt.Errorf("Python error: %v\nDetails: %s", err, errOutput)
		}
		return nil, fmt.Errorf("Python error with no stderr output: %v", err)
	}

	// Log any stderr output (not necessarily an error)
	if stderr.Len() > 0 {
		log.Infof("Python script stderr output (not an error): %s", stderr.String())
	}

	// Verify the report file exists and has content
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("report file was not created at %s", outputPath)
		}
		return nil, fmt.Errorf("error checking report file: %v", err)
	}

	if fileInfo.Size() == 0 {
		return nil, fmt.Errorf("report file was created but is empty: %s", outputPath)
	}

	log.Infof("Reading report file of size %d bytes from %s", fileInfo.Size(), outputPath)

	// Read the generated report
	reportData, err := ioutil.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("error reading report file: %v", err)
	}

	log.Infof("Successfully read report file of %d bytes", len(reportData))
	return reportData, nil
}

// InstallDependencies installs Python dependencies into a virtual environment
func (s *PythonService) InstallDependencies(packages []string) (map[string]interface{}, error) {
	// Create temporary directory for scripts
	tempDir, err := ioutil.TempDir("", "gophish-dependency-install")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory structure
	pythonDir := filepath.Join(tempDir, "python")
	if err := os.MkdirAll(pythonDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create python directory: %v", err)
	}

	// Write all required scripts
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "venv_manager.py"), []byte(venvManagerScript), 0500); err != nil {
		return nil, fmt.Errorf("failed to write venv_manager.py: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(pythonDir, "install_dependencies.py"), []byte(installDependenciesScript), 0500); err != nil {
		return nil, fmt.Errorf("failed to write install_dependencies.py: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(pythonDir, "requirements.txt"), []byte(requirementsFile), 0500); err != nil {
		return nil, fmt.Errorf("failed to write requirements.txt: %v", err)
	}

	// Run install script - it will handle venv creation and package installation
	installScript := filepath.Join(pythonDir, "install_dependencies.py")
	cmd := exec.Command(s.PythonPath, installScript)

	// Set environment variable to point to persistent venv location
	venvDir := getPersistentVenvDir()
	cmd.Env = append(os.Environ(), fmt.Sprintf("ANGLERPHISH_VENV_DIR=%s", venvDir))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Infof("Installing Python dependencies using system Python: %s (venv: %s)", s.PythonPath, venvDir)
	if err := cmd.Run(); err != nil {
		errOutput := stderr.String()
		if errOutput != "" {
			log.Errorf("Python dependency installation error: %s", errOutput)
			return nil, fmt.Errorf("Python error: %v\nDetails: %s", err, errOutput)
		}
		return nil, fmt.Errorf("Python error with no stderr output: %v", err)
	}

	// Parse the JSON output
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse dependency installation output: %v\nOutput: %s", err, stdout.String())
	}

	return result, nil
}

// CheckDependencies checks if all required Python dependencies are installed
func (s *PythonService) CheckDependencies() (*DependencyStatus, error) {
	// Create temporary directory for scripts
	tempDir, err := ioutil.TempDir("", "gophish-dependency-check")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create directory structure
	pythonDir := filepath.Join(tempDir, "python")
	if err := os.MkdirAll(pythonDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create python directory: %v", err)
	}

	// Write required scripts
	if err := ioutil.WriteFile(filepath.Join(pythonDir, "venv_manager.py"), []byte(venvManagerScript), 0500); err != nil {
		return nil, fmt.Errorf("failed to write venv_manager.py: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(pythonDir, "check_dependencies.py"), []byte(checkDependenciesScript), 0500); err != nil {
		return nil, fmt.Errorf("failed to write check_dependencies.py: %v", err)
	}

	if err := ioutil.WriteFile(filepath.Join(pythonDir, "requirements.txt"), []byte(requirementsFile), 0500); err != nil {
		return nil, fmt.Errorf("failed to write requirements.txt: %v", err)
	}

	// Run check script
	checkScript := filepath.Join(pythonDir, "check_dependencies.py")
	cmd := exec.Command(s.PythonPath, checkScript)

	// Set environment variable to point to persistent venv location
	venvDir := getPersistentVenvDir()
	cmd.Env = append(os.Environ(), fmt.Sprintf("ANGLERPHISH_VENV_DIR=%s", venvDir))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Infof("Checking Python dependencies (venv: %s)", venvDir)
	if err := cmd.Run(); err != nil {
		errOutput := stderr.String()
		if errOutput != "" {
			log.Errorf("Python dependency check error: %s", errOutput)
			return nil, fmt.Errorf("Python error: %v\nDetails: %s", err, errOutput)
		}
		return nil, fmt.Errorf("Python error with no stderr output: %v", err)
	}

	// Parse the JSON output
	var status DependencyStatus
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		return nil, fmt.Errorf("failed to parse dependency check output: %v\nOutput: %s", err, stdout.String())
	}

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

// ValidateCampaignData validates campaigns before report generation
func ValidateCampaignData(campaignIDs []int64) (*ValidationResult, error) {
	result := &ValidationResult{
		Valid:               true,
		Errors:              []string{},
		Warnings:            []string{},
		IncompleteCampaigns: []string{},
		MissingTimelineData: []string{},
	}

	log.Infof("ValidateCampaignData: Validating %d campaign IDs", len(campaignIDs))

	if len(campaignIDs) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No campaign IDs provided")
		return result, nil
	}

	validCampaigns := 0
	for _, id := range campaignIDs {
		campaign, err := models.GetCampaign(id, 1)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("Campaign ID %d: Failed to retrieve - %v", id, err))
			continue
		}

		if campaign.Status != "Complete" && campaign.Status != "Completed" {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d) is not completed (status: %s)", campaign.Name, id, campaign.Status))
			result.IncompleteCampaigns = append(result.IncompleteCampaigns, fmt.Sprintf("%s (ID: %d)", campaign.Name, id))
		}

		campaignResults, err := models.GetCampaignResults(id, 1)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d): Unable to retrieve results - %v", campaign.Name, id, err))
		} else {
			if len(campaignResults.Events) == 0 && len(campaignResults.Results) > 0 {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Campaign '%s' (ID: %d): No timeline data available - report metrics may be incomplete", campaign.Name, id))
				result.MissingTimelineData = append(result.MissingTimelineData, fmt.Sprintf("%s (ID: %d)", campaign.Name, id))
			}
		}

		validCampaigns++
	}

	if validCampaigns == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "No valid campaigns found for report generation")
	}

	if len(result.Errors) > 0 {
		log.Warnf("Campaign validation completed with %d errors", len(result.Errors))
	}
	if len(result.Warnings) > 0 {
		log.Infof("Campaign validation completed with %d warnings", len(result.Warnings))
	}
	if result.Valid && len(result.Errors) == 0 && len(result.Warnings) == 0 {
		log.Infof("Campaign validation completed successfully - all %d campaigns are ready", validCampaigns)
	}

	return result, nil
}

// FetchCampaignData retrieves the campaign data needed for report generation
func FetchCampaignData(campaignIDs []int64) ([]byte, error) {
	data := ReportData{
		Campaigns: make([]map[string]interface{}, 0),
	}

	log.Infof("FetchCampaignData: Fetching data for %d campaign IDs", len(campaignIDs))

	for _, id := range campaignIDs {
		campaign, err := models.GetCampaign(id, 1)
		if err != nil {
			log.Errorf("Failed to get campaign for ID %d: %v", id, err)
			continue
		}

		if campaign.Status != "Complete" && campaign.Status != "Completed" {
			log.Warnf("Campaign ID %d (%s) is not completed (status: %s) - including with partial data", id, campaign.Name, campaign.Status)
		}

		log.Infof("Processing campaign ID %d (%s) for report", id, campaign.Name)

		campaignResults, err := models.GetCampaignResults(id, 1)
		if err != nil {
			log.Errorf("Error getting campaign results for ID %d: %v", id, err)

			campaignData := map[string]interface{}{
				"id":             campaign.Id,
				"name":           campaign.Name,
				"status":         campaign.Status,
				"created_date":   campaign.CreatedDate,
				"launch_date":    campaign.LaunchDate,
				"completed_date": campaign.CompletedDate,
				"results":        []interface{}{},
				"timeline":       []interface{}{},
			}

			data.Campaigns = append(data.Campaigns, campaignData)
			continue
		}

		campaignData := map[string]interface{}{
			"id":       campaignResults.Id,
			"name":     campaignResults.Name,
			"status":   campaignResults.Status,
			"results":  campaignResults.Results,
			"timeline": campaignResults.Events,
		}

		campaignData["created_date"] = campaign.CreatedDate
		campaignData["launch_date"] = campaign.LaunchDate
		campaignData["completed_date"] = campaign.CompletedDate
		campaignData["type"] = campaign.Type

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

		campaignData["page_details"] = map[string]interface{}{
			"redirect_url":        campaign.Page.RedirectURL,
			"capture_credentials": campaign.Page.CaptureCredentials,
			"capture_passwords":   campaign.Page.CapturePasswords,
		}

		campaignData["phish_url"] = campaign.URL
		campaignData["urlparam"] = campaign.URLParam

		data.Campaigns = append(data.Campaigns, campaignData)
	}

	if len(data.Campaigns) == 0 {
		log.Error("No valid campaigns were found for report generation")
		return nil, fmt.Errorf("no valid campaigns found - please ensure selected campaigns exist and you have access to them")
	}

	log.Infof("Successfully collected data for %d campaigns for report generation", len(data.Campaigns))

	return json.Marshal(data)
}

// FetchCampaignSetData retrieves the campaign set data needed for report generation
func FetchCampaignSetData(campaignSetID int64) ([]byte, error) {
	data := ReportData{
		Campaigns: make([]map[string]interface{}, 0),
	}

	log.Infof("FetchCampaignSetData: Fetching data for campaign set ID %d", campaignSetID)

	campaignSet, err := models.GetCampaignSet(campaignSetID, 1)
	if err != nil {
		log.Errorf("Failed to get campaign set for ID %d: %v", campaignSetID, err)
		return nil, err
	}

	data.SetName = campaignSet.Name
	data.SetID = campaignSet.Id
	data.SetCreatedDate = campaignSet.CreatedDate
	data.SetLaunchDate = campaignSet.LaunchDate
	data.SetCompletedDate = campaignSet.CompletedDate
	data.SetStatus = campaignSet.Status
	data.SetURL = campaignSet.URL

	for _, campaign := range campaignSet.Campaigns {
		if campaign.Status != "Complete" && campaign.Status != "Completed" {
			log.Infof("Campaign ID %d (%s) in set is not completed (status: %s) - including anyway", campaign.Id, campaign.Name, campaign.Status)
		}

		log.Infof("Processing campaign ID %d (%s) from set for report", campaign.Id, campaign.Name)

		campaignResults, err := models.GetCampaignResults(campaign.Id, 1)
		if err != nil {
			log.Errorf("Error getting campaign results for ID %d: %v", campaign.Id, err)

			campaignData := map[string]interface{}{
				"id":             campaign.Id,
				"name":           campaign.Name,
				"status":         campaign.Status,
				"created_date":   campaign.CreatedDate,
				"launch_date":    campaign.LaunchDate,
				"completed_date": campaign.CompletedDate,
				"results":        []interface{}{},
				"timeline":       []interface{}{},
			}

			data.Campaigns = append(data.Campaigns, campaignData)
			continue
		}

		campaignData := map[string]interface{}{
			"id":       campaignResults.Id,
			"name":     campaignResults.Name,
			"status":   campaignResults.Status,
			"results":  campaignResults.Results,
			"timeline": campaignResults.Events,
		}

		campaignData["created_date"] = campaign.CreatedDate
		campaignData["launch_date"] = campaign.LaunchDate
		campaignData["completed_date"] = campaign.CompletedDate
		campaignData["type"] = campaign.Type

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

		campaignData["page_details"] = map[string]interface{}{
			"redirect_url":        campaign.Page.RedirectURL,
			"capture_credentials": campaign.Page.CaptureCredentials,
			"capture_passwords":   campaign.Page.CapturePasswords,
		}

		campaignData["phish_url"] = campaign.URL
		campaignData["urlparam"] = campaign.URLParam

		data.Campaigns = append(data.Campaigns, campaignData)
	}

	if len(data.Campaigns) == 0 {
		log.Error("No campaigns were found in the campaign set for report generation")
		return nil, fmt.Errorf("no campaigns found in campaign set - please ensure the set exists and contains campaigns")
	}

	log.Infof("Successfully collected data for %d campaigns in set %d for report generation", len(data.Campaigns), campaignSetID)

	return json.Marshal(data)
}

// GenerateReport creates a report for the specified campaign(s)
func GenerateReport(format ReportFormat, campaignIDs []int64, options ReportOptions) ([]byte, error) {
	campaignData, err := FetchCampaignData(campaignIDs)
	if err != nil {
		return nil, err
	}

	service := NewPythonService()
	return service.GenerateReport(format, campaignData, options)
}

// GenerateReportForCampaignSet creates a report for the specified campaign set
func GenerateReportForCampaignSet(format ReportFormat, campaignSetID int64, options ReportOptions) ([]byte, error) {
	campaignSetData, err := FetchCampaignSetData(campaignSetID)
	if err != nil {
		return nil, err
	}

	service := NewPythonService()
	return service.GenerateReport(format, campaignSetData, options)
}
