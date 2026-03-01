// Reports JavaScript Module
// Handles async report generation, history management, and tabbed interface

var reports = {
    // Properties
    campaigns: [],
    campaignSets: [],
    dependencyStatus: null,
    currentReports: [],
    statusPollingInterval: null,
    
    // Package descriptions for user understanding
    packageDescriptions: {
        "python-docx": "Create and modify Word documents",
        "openpyxl": "Generate Excel spreadsheets",
        "requests": "HTTP library for API calls and web operations",
        "traceback2": "Enhanced error reporting and debugging",
        "user-agents": "Parse and analyze user agent strings",
        "geoip2": "IP geolocation lookup functionality"
    },
    
    // Initialization
    init: function() {
        // Initialize UI state management
        reports.initializeUIState();
        
        // Initialize event handlers
        reports.initializeEventHandlers();
        
        // Check Python dependencies
        reports.checkDependencies();
        
        // Load campaigns and campaign sets
        reports.loadCampaigns();
        reports.loadCampaignSets();
        
        // Load report history
        reports.loadReportHistory();
        
        // Start polling for report status updates
        reports.startStatusPolling();
        
        // Setup dependency modal
        reports.setupDependencyModal();
    },
    
    // Initialize UI state from localStorage
    initializeUIState: function() {
        // Restore source selection first to prevent UI flicker
        var savedSource = localStorage.getItem("gophish.reports.source");
        if (savedSource) {
            $("#report_source").val(savedSource);
            reports.toggleSourceSelection();
        }
        
        // Restore report format selection
        var savedFormat = localStorage.getItem("gophish.reports.format");
        if (savedFormat) {
            $("#report_format").val(savedFormat);
        }
        
        // Restore GDPR options
        reports.restoreCheckboxState("anonymize_emails", true);
        reports.restoreCheckboxState("anonymize_ips", true);
    },
    
    // Initialize event handlers
    initializeEventHandlers: function() {
        // Report source selection (campaign vs campaign set)
        $("#report_source").on("change", function() {
            var selectedValue = $(this).val();
            localStorage.setItem("gophish.reports.source", selectedValue);
            reports.toggleSourceSelection();
        });
        
        // Report format selection
        $("#report_format").on("change", function() {
            var selectedValue = $(this).val();
            localStorage.setItem("gophish.reports.format", selectedValue);
        });
        
        // Generate report button
        $("#generate_report").on("click", function() {
            reports.generateReport();
        });
        
        // Refresh reports button
        $("#refresh-reports").on("click", function() {
            reports.loadReportHistory();
        });
        
        // Tab switching
        $('a[data-toggle="tab"]').on('shown.bs.tab', function (e) {
            var target = $(e.target).attr("href");
            if (target === "#history-tab") {
                reports.loadReportHistory();
            }
        });
        
        // Option save listeners
        $("#anonymize_emails, #anonymize_ips").on("change", function() {
            localStorage.setItem("gophish.reports." + this.id, this.checked);
        });
    },
    
    // Helper function to restore checkbox state
    restoreCheckboxState: function(elementId, defaultState) {
        var savedState = localStorage.getItem("gophish.reports." + elementId);
        if (savedState !== null) {
            $("#" + elementId).prop("checked", savedState === "true");
        } else {
            $("#" + elementId).prop("checked", defaultState);
        }
    },
    
    // Toggle between campaign and campaign set selection
    toggleSourceSelection: function() {
        var source = $("#report_source").val();
        if (source === "campaign") {
            $("#campaign_select_container").show();
            $("#campaign_set_select_container").hide();
        } else {
            $("#campaign_select_container").hide();
            $("#campaign_set_select_container").show();
        }
    },
    
    // Load campaigns via API
    loadCampaigns: function() {
        api.campaigns.get()
            .success(function(campaigns) {
                reports.campaigns = campaigns;
                $("#campaign_select").empty();
                
                if (campaigns.length === 0) {
                    $("#campaign_select").append('<option value="">No campaigns available</option>');
                    return;
                }
                
                // Sort campaigns by name
                campaigns.sort(function(a, b) {
                    return a.name.localeCompare(b.name);
                });
                
                // Add campaigns to select
                $.each(campaigns, function(i, campaign) {
                    $("#campaign_select").append('<option value="' + campaign.id + '">' + escapeHtml(campaign.name) + '</option>');
                });
                
                // Restore campaign selection from localStorage
                var savedCampaignIds = localStorage.getItem("gophish.reports.campaign_ids");
                if (savedCampaignIds) {
                    try {
                        var campaignIds = JSON.parse(savedCampaignIds);
                        $("#campaign_select").val(campaignIds);
                    } catch (e) {
                        // Ignore parse errors
                    }
                }
                
                // Save selection when it changes
                $("#campaign_select").on("change", function() {
                    var selectedIds = $(this).val();
                    localStorage.setItem("gophish.reports.campaign_ids", JSON.stringify(selectedIds));
                });
            })
            .error(function(error) {
                errorFlash("Error loading campaigns: " + error.message);
            });
    },
    
    // Load campaign sets via API
    loadCampaignSets: function() {
        api.campaignSets.get()
            .success(function(campaignSets) {
                reports.campaignSets = campaignSets;
                $("#campaign_set_select").empty();
                
                if (campaignSets.length === 0) {
                    $("#campaign_set_select").append('<option value="">No campaign sets available</option>');
                    return;
                }
                
                // Sort campaign sets by name
                campaignSets.sort(function(a, b) {
                    return a.name.localeCompare(b.name);
                });
                
                // Add campaign sets to select
                $.each(campaignSets, function(i, campaignSet) {
                    $("#campaign_set_select").append('<option value="' + campaignSet.id + '">' + escapeHtml(campaignSet.name) + '</option>');
                });
                
                // Restore campaign set selection from localStorage
                var savedCampaignSetId = localStorage.getItem("gophish.reports.campaign_set_id");
                if (savedCampaignSetId) {
                    $("#campaign_set_select").val(savedCampaignSetId);
                }
                
                // Save selection when it changes
                $("#campaign_set_select").on("change", function() {
                    var selectedId = $(this).val();
                    localStorage.setItem("gophish.reports.campaign_set_id", selectedId);
                });
            })
            .error(function(error) {
                errorFlash("Error loading campaign sets: " + error.message);
            });
    },
    
    // Load report history - always uses smart updates
    loadReportHistory: function() {
        var xhr = new XMLHttpRequest();
        xhr.open('GET', '/api/reports/', true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        var csrfToken = api.csrf || $('meta[name="csrf-token"]').attr('content');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRFToken', csrfToken);
        }
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                try {
                    var data = JSON.parse(xhr.responseText);
                    var newReports = data.reports || [];
                    var newStats = data.stats || {};
                    
                    // Update stats smoothly
                    reports.updateReportStats(newStats);
                    
                    // Always use smart update - it handles first load naturally
                    if (reports.currentReports.length === 0 && newReports.length === 0) {
                        // No reports at all - show message
                        $("#no-reports").show();
                        $("#loading-reports").hide();
                    } else {
                        $("#no-reports").hide();
                        $("#loading-reports").hide();
                        
                        if (reports.currentReports.length === 0) {
                            // First load with data - populate from scratch
                            reports.currentReports = newReports;
                            reports.populateReportsTable(newReports);
                        } else {
                            // Subsequent loads - use smart diff update
                            reports.updateReportsTable(newReports);
                            reports.currentReports = newReports;
                        }
                    }
                } catch (e) {
                    errorFlash("Error parsing report data");
                }
            } else {
                errorFlash("Error loading reports");
            }
        };
        
        xhr.onerror = function() {
            errorFlash("Network error loading reports");
        };
        
        xhr.send();
    },
    
    // Update report statistics display
    updateReportStats: function(stats) {
        $("#stats-total").text(stats.total || 0);
        $("#stats-queued").text(stats.queued || 0);
        $("#stats-processing").text(stats.processing || 0);
        $("#stats-completed").text(stats.completed || 0);
        $("#report-count").text(stats.total || 0);
    },
    
    // Populate reports table
    populateReportsTable: function(reportsList) {
        var $tbody = $("#reports-table tbody");
        $tbody.empty();
        
        if (reportsList.length === 0) {
            $("#no-reports").show();
            return;
        }
        
        $.each(reportsList, function(i, report) {
            var row = reports.createReportTableRow(report);
            $tbody.append(row);
        });
    },
    
    // Smart update of reports table - only changes what's different
    updateReportsTable: function(newReports) {
        var $tbody = $("#reports-table tbody");
        
        if (newReports.length === 0) {
            $tbody.empty();
            $("#no-reports").show();
            return;
        }
        
        $("#no-reports").hide();
        
        // Create a map of current reports by ID for quick lookup
        var currentMap = {};
        reports.currentReports.forEach(function(report) {
            currentMap[report.id] = report;
        });
        
        // Create a map of new reports by ID
        var newMap = {};
        newReports.forEach(function(report) {
            newMap[report.id] = report;
        });
        
        // Update or add rows
        newReports.forEach(function(report) {
            var $existingRow = $tbody.find('tr[data-report-id="' + report.id + '"]');
            var oldReport = currentMap[report.id];
            
            if ($existingRow.length === 0) {
                // New report - add it
                var newRow = reports.createReportTableRow(report);
                $tbody.prepend(newRow); // Add new reports at the top
                $tbody.find('tr[data-report-id="' + report.id + '"]').hide().fadeIn(300);
            } else if (oldReport && (oldReport.status !== report.status || oldReport.file_size !== report.file_size)) {
                // Report changed - update it with a brief highlight
                var updatedRow = reports.createReportTableRow(report);
                $existingRow.html($(updatedRow).html());
                $existingRow.addClass('highlight-update');
                setTimeout(function() {
                    $existingRow.removeClass('highlight-update');
                }, 1000);
            }
        });
        
        // Remove rows that no longer exist
        $tbody.find('tr[data-report-id]').each(function() {
            var reportId = $(this).data('report-id');
            if (!newMap[reportId]) {
                $(this).fadeOut(300, function() {
                    $(this).remove();
                });
            }
        });
    },
    
    // Create a table row for a report
    createReportTableRow: function(report) {
        var statusBadge = reports.getStatusBadge(report.status);
        var actions = reports.getReportActions(report);
        var fileSize = reports.formatFileSize(report.file_size);
        var createdAt = reports.formatDate(report.created_at);
        
        return '<tr data-report-id="' + report.id + '">' +
            '<td>' + report.id + '</td>' +
            '<td>' + report.format.charAt(0).toUpperCase() + report.format.slice(1) + '</td>' +
            '<td>' + statusBadge + '</td>' +
            '<td>' + (report.campaign_count || 0) + ' campaigns</td>' +
            '<td>' + createdAt + '</td>' +
            '<td>' + fileSize + '</td>' +
            '<td>' + actions + '</td>' +
            '</tr>';
    },
    
    // Get status badge HTML
    getStatusBadge: function(status) {
        var badgeClass = 'default';
        var icon = 'question';
        
        switch (status) {
            case 'queued':
                badgeClass = 'warning';
                icon = 'clock-o';
                break;
            case 'processing':
                badgeClass = 'info';
                icon = 'spinner fa-spin';
                break;
            case 'completed':
                badgeClass = 'success';
                icon = 'check';
                break;
            case 'failed':
                badgeClass = 'danger';
                icon = 'times';
                break;
        }
        
        return '<span class="label label-' + badgeClass + '">' +
            '<i class="fa fa-' + icon + '"></i> ' + status.charAt(0).toUpperCase() + status.slice(1) +
            '</span>';
    },
    
    // Get action buttons for a report
    getReportActions: function(report) {
        var actions = '';
        
        if (report.status === 'completed') {
            actions += '<button class="btn btn-success btn-sm" onclick="reports.downloadReport(' + report.id + ')" title="Download">' +
                '<i class="fa fa-download"></i></button> ';
        }
        
        if (report.status === 'processing' || report.status === 'queued') {
            actions += '<button class="btn btn-info btn-sm" onclick="reports.checkReportStatus(' + report.id + ')" title="Check Status">' +
                '<i class="fa fa-refresh"></i></button> ';
        }
        
        actions += '<button class="btn btn-danger btn-sm" onclick="reports.deleteReport(' + report.id + ')" title="Delete">' +
            '<i class="fa fa-trash"></i></button>';
        
        return actions;
    },
    
    // Format file size for display
    formatFileSize: function(bytes) {
        if (!bytes || bytes === 0) return '-';
        
        var sizes = ['B', 'KB', 'MB', 'GB'];
        var i = Math.floor(Math.log(bytes) / Math.log(1024));
        return Math.round(bytes / Math.pow(1024, i) * 100) / 100 + ' ' + sizes[i];
    },
    
    // Format date for display
    formatDate: function(dateString) {
        if (!dateString) return '-';
        
        var date = new Date(dateString);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    },
    
    // Start polling for status updates
    startStatusPolling: function() {
        // Poll every 10 seconds for status updates
        reports.statusPollingInterval = setInterval(function() {
            // Only poll if we're on the history tab and have active reports
            if ($('#history-tab').hasClass('active') && reports.hasActiveReports()) {
                reports.loadReportHistory();
            }
        }, 10000);
    },
    
    // Check if there are any active reports (queued or processing)
    hasActiveReports: function() {
        return reports.currentReports.some(function(report) {
            return report.status === 'queued' || report.status === 'processing';
        });
    },
    
    // Generate report (async)
    generateReport: function() {
        var reportOptions = {
            format: $("#report_format").val(),
            options: {
                anonymize_emails: $("#anonymize_emails").is(":checked"),
                anonymize_ips: $("#anonymize_ips").is(":checked")
            }
        };
        
        // Get campaign IDs or campaign set ID
        if ($("#report_source").val() === "campaign") {
            var selectedIds = $("#campaign_select").val();
            var campaignIds = Array.isArray(selectedIds) ? selectedIds : [selectedIds];
            reportOptions.campaign_ids = campaignIds.map(function(id) {
                return parseInt(id);
            });
            
            if (!reportOptions.campaign_ids || reportOptions.campaign_ids.length === 0) {
                errorFlash("Please select at least one campaign");
                return;
            }
        } else {
            var campaignSetId = $("#campaign_set_select").val();
            if (!campaignSetId) {
                errorFlash("Please select a campaign set");
                return;
            }
            reportOptions.campaign_set_id = parseInt(campaignSetId);
        }
        
        // Show loading modal
        $("#loading").modal({
            backdrop: 'static',
            keyboard: false
        });
        
        // Queue the report
        var xhr = new XMLHttpRequest();
        xhr.open('POST', '/api/reports/queue', true);
        xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        var csrfToken = api.csrf || $('meta[name="csrf-token"]').attr('content');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRFToken', csrfToken);
        }
        
        xhr.onload = function() {
            $("#loading").modal('hide');
            
            if (xhr.status === 200) {
                try {
                    var data = JSON.parse(xhr.responseText);
                    successFlash("Report queued successfully! Check the Report History tab for progress.");
                    
                    // Switch to history tab
                    $('a[href="#history-tab"]').tab('show');
                    
                    // Refresh the reports list
                    setTimeout(function() {
                        reports.loadReportHistory();
                    }, 1000);
                } catch (e) {
                    errorFlash("Error queuing report");
                }
            } else {
                try {
                    var response = JSON.parse(xhr.responseText);
                    errorFlash(response.message || "Error queuing report");
                } catch (e) {
                    errorFlash("Error queuing report: " + xhr.statusText);
                }
            }
        };
        
        xhr.onerror = function() {
            $("#loading").modal('hide');
            errorFlash("Network error while queuing report");
        };
        
        xhr.send(JSON.stringify(reportOptions));
    },
    
    // Download a completed report
    downloadReport: function(reportId) {
        // Use XHR with Authorization header instead of URL parameter for security
        var xhr = new XMLHttpRequest();
        xhr.open('GET', '/api/reports/' + reportId + '/download', true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        xhr.responseType = 'blob';
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                var blob = xhr.response;
                var link = document.createElement('a');
                link.href = window.URL.createObjectURL(blob);
                
                // Get filename from Content-Disposition header if available
                var disposition = xhr.getResponseHeader('Content-Disposition');
                var filename = 'report.docx';
                if (disposition) {
                    var filenameMatch = disposition.match(/filename[^;=\n]*=((['"]).*?\2|[^;\n]*)/);
                    if (filenameMatch && filenameMatch[1]) {
                        filename = filenameMatch[1].replace(/['"]/g, '');
                    }
                }
                
                link.download = filename;
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                window.URL.revokeObjectURL(link.href);
            } else {
                errorFlash('Error downloading report: ' + xhr.statusText);
            }
        };
        
        xhr.onerror = function() {
            errorFlash('Network error while downloading report');
        };
        
        xhr.send();
    },
    
    // Check report status
    checkReportStatus: function(reportId) {
        var xhr = new XMLHttpRequest();
        xhr.open('GET', '/api/reports/' + reportId + '/status', true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                try {
                    var data = JSON.parse(xhr.responseText);
                    var message = "Status: " + data.status;
                    if (data.message) {
                        message += " - " + data.message;
                    }
                    successFlash(message);
                    
                    // Refresh the reports list with smart update
                    reports.loadReportHistory();
                } catch (e) {
                    errorFlash("Error checking report status");
                }
            } else {
                errorFlash("Error checking report status");
            }
        };
        
        xhr.onerror = function() {
            errorFlash("Network error checking report status");
        };
        
        xhr.send();
    },
    
    // Delete a report
    deleteReport: function(reportId) {
        if (!confirm("Are you sure you want to delete this report?")) {
            return;
        }
        
        var xhr = new XMLHttpRequest();
        xhr.open('DELETE', '/api/reports/' + reportId, true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        var csrfToken = api.csrf || $('meta[name="csrf-token"]').attr('content');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRFToken', csrfToken);
        }
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                successFlash("Report deleted successfully");
                // Use smart refresh - deleted row will fade out
                reports.loadReportHistory();
            } else {
                try {
                    var response = JSON.parse(xhr.responseText);
                    errorFlash(response.message || "Error deleting report");
                } catch (e) {
                    errorFlash("Error deleting report");
                }
            }
        };
        
        xhr.onerror = function() {
            errorFlash("Network error deleting report");
        };
        
        xhr.send();
    },
    
    // Check Python dependencies
    checkDependencies: function() {
        var xhr = new XMLHttpRequest();
        xhr.open('GET', '/api/reports/dependencies', true);
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        var csrfToken = api.csrf || $('meta[name="csrf-token"]').attr('content');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRFToken', csrfToken);
        }
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                try {
                    var data = JSON.parse(xhr.responseText);
                    reports.dependencyStatus = data;
                    reports.updateDependencyStatus();
                } catch (e) {
                    reports.dependencyStatus = {
                        required_installed: false,
                        all_installed: false,
                        dependencies: []
                    };
                    reports.updateDependencyStatus();
                }
            } else {
                reports.dependencyStatus = {
                    required_installed: false,
                    all_installed: false,
                    dependencies: []
                };
                reports.updateDependencyStatus();
            }
        };
        
        xhr.onerror = function() {
            reports.dependencyStatus = {
                required_installed: false,
                all_installed: false,
                dependencies: []
            };
            reports.updateDependencyStatus();
        };
        
        xhr.send();
    },
    
    // Update dependency status indicator
    updateDependencyStatus: function() {
        var $status = $("#dependency-status");
        
        if (!reports.dependencyStatus) {
            $status.html('<span class="label label-default"><i class="fa fa-spinner fa-spin"></i> Checking Dependencies...</span> <i class="fa fa-info-circle" style="margin-left: 5px;"></i>');
            return;
        }
        
        var pythonVersion = reports.dependencyStatus.python_version || "Unknown";
        var pythonFound = reports.dependencyStatus.python_installed !== false;
        
        if (!pythonFound) {
            $status.html('<span class="label label-danger" data-toggle="tooltip" title="Python not found. Please install Python and ensure it\'s in your PATH.">Python Not Found</span> <i class="fa fa-info-circle" style="margin-left: 5px;"></i>');
            $("#generate_report").prop("disabled", true);
            return;
        }
        
        if (reports.dependencyStatus.required_installed) {
            $status.html('<span class="label label-success" data-toggle="tooltip" title="All required dependencies are installed.">Dependencies: Installed</span> <i class="fa fa-info-circle" style="margin-left: 5px;"></i>');
            $("#generate_report").prop("disabled", false);
        } else {
            $status.html('<span class="label label-danger" data-toggle="tooltip" title="Some required dependencies are missing. Using Python ' + pythonVersion + '">Dependencies: Not Installed</span> <i class="fa fa-info-circle" style="margin-left: 5px;"></i>');
            $("#generate_report").prop("disabled", true);
        }
        
        $('[data-toggle="tooltip"]').tooltip();
    },
    
    // Setup dependency modal
    setupDependencyModal: function() {
        $("#dependency-status").on("click", function() {
            reports.showDependencyModal();
        });
        
        $("#install-dependencies").on("click", function() {
            // Install all dependencies from requirements.txt
            reports.installDependencies();
        });
    },
    
    // Show dependency modal
    showDependencyModal: function() {
        var $tableBody = $("#dependency-table-body");
        $tableBody.empty();
        
        if (reports.dependencyStatus) {
            var pythonVersion = reports.dependencyStatus.python_version || "Unknown";
            var pythonFound = reports.dependencyStatus.python_installed !== false;
            
            if (pythonFound) {
                $("#python-info").removeClass("alert-danger").addClass("alert-info");
                $("#python-version").text(pythonVersion);
            } else {
                $("#python-info").removeClass("alert-info").addClass("alert-danger");
                $("#python-version").text("Not found! Please install Python and ensure it's in your PATH.");
            }
        } else {
            $("#python-version").text("Checking...");
        }
        
        if (reports.dependencyStatus && reports.dependencyStatus.dependencies && reports.dependencyStatus.dependencies.length > 0) {
            $.each(reports.dependencyStatus.dependencies, function(i, dep) {
                // Simple coloring: green for installed, red for not installed
                var statusClass = dep.installed ? "success" : "danger";
                var statusIcon = dep.installed ? "check" : "times";
                
                // Get description for the package
                var description = reports.packageDescriptions[dep.name] || "No description available";
                
                var row = '<tr>' +
                    '<td>' + escapeHtml(dep.name) + '</td>' +
                    '<td><span class="text-' + statusClass + '"><i class="fa fa-' + statusIcon + '"></i> ' + 
                    (dep.installed ? "Installed" : "Not Installed") + '</span></td>' +
                    '<td><small class="text-muted">' + escapeHtml(description) + '</small></td>' +
                    '</tr>';
                
                $tableBody.append(row);
            });
        } else {
            $tableBody.append('<tr><td colspan="3" class="text-center">No dependency information available</td></tr>');
        }
        
        // Show/hide install button based on whether all dependencies are installed
        if (reports.dependencyStatus && reports.dependencyStatus.all_installed) {
            $("#install-dependencies").hide();
        } else {
            $("#install-dependencies").show();
        }
        
        $("#dependency-modal").modal('show');
    },
    
    // Install dependencies with progress tracking
    installDependencies: function() {
        $("#dependency-details").hide();
        $("#dependency-install-progress").show();
        $("#install-dependencies").prop("disabled", true);
        
        // Initialize progress bar
        $("#install-progress-bar").css("width", "0%").attr("aria-valuenow", 0);
        $("#install-progress-text").text("0%");
        $("#install-package-name").text("Starting...");
        $("#install-log").empty();
        
        // Step 1: Initialize
        reports.updateInstallProgress(5, "Preparing...", "Starting dependency installation...");
        
        var xhr = new XMLHttpRequest();
        xhr.open('POST', '/api/reports/dependencies/install', true);
        xhr.setRequestHeader('Content-Type', 'application/json');
        xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        
        var csrfToken = api.csrf || $('meta[name="csrf-token"]').attr('content');
        if (csrfToken) {
            xhr.setRequestHeader('X-CSRFToken', csrfToken);
        }
        
        // Step 2: Show venv creation step (after 500ms)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(15, "Virtual Environment", "Creating Python virtual environment (venv)...");
            }
        }, 500);
        
        // Step 3: Show pip upgrade step (after 2s)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(25, "pip", "Upgrading pip in virtual environment...");
            }
        }, 2000);
        
        // Step 4: Show package installation (after 4s)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(35, "python-docx", "Installing python-docx (Word document support)...");
            }
        }, 4000);
        
        // Step 5: Show more packages (after 8s)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(50, "openpyxl", "Installing openpyxl (Excel spreadsheet support)...");
            }
        }, 8000);
        
        // Step 6: More packages (after 12s)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(65, "requests", "Installing requests and other dependencies...");
            }
        }, 12000);
        
        // Step 7: Still working message (after 20s)
        setTimeout(function() {
            if (xhr.readyState < 4) {
                reports.updateInstallProgress(75, "Utility packages", "Installing utility packages (geoip2, user-agents)...");
                reports.appendInstallLog("Still working... Large packages may take a while to download.", 'info');
            }
        }, 20000);
        
        
        xhr.onload = function() {
            if (xhr.status === 200) {
                try {
                    var data = JSON.parse(xhr.responseText);
                    
                    // Show completion progress
                    reports.updateInstallProgress(80, "All packages", "Finalizing installation...");
                    
                    // Display installation output in log
                    if (data.details) {
                        reports.appendInstallLog(data.details, data.success ? 'success' : 'error');
                    }
                    
                    // Complete progress
                    reports.updateInstallProgress(100, "Completed", data.success ? "Installation completed successfully!" : "Installation completed with errors");
                    
                    // Enable close button instead of auto-closing
                    reports.enableInstallCloseButton(data.success, data.success ? null : data.message);
                    
                } catch (e) {
                    reports.updateInstallProgress(100, "Error", "Failed to parse server response");
                    reports.appendInstallLog("Error: Could not parse server response", 'error');
                    
                    setTimeout(function() {
                        $("#dependency-install-progress").hide();
                        $("#dependency-details").show();
                        $("#install-dependencies").prop("disabled", false);
                        errorFlash("Error installing dependencies: Could not parse server response");
                    }, 1500);
                }
            } else {
                reports.updateInstallProgress(100, "Error", "Installation failed");
                
                try {
                    var response = JSON.parse(xhr.responseText);
                    reports.appendInstallLog("Error: " + response.message, 'error');
                    
                    setTimeout(function() {
                        $("#dependency-install-progress").hide();
                        $("#dependency-details").show();
                        $("#install-dependencies").prop("disabled", false);
                        errorFlash("Error installing dependencies: " + response.message);
                    }, 1500);
                } catch (e) {
                    reports.appendInstallLog("Error: " + xhr.statusText, 'error');
                    
                    setTimeout(function() {
                        $("#dependency-install-progress").hide();
                        $("#dependency-details").show();
                        $("#install-dependencies").prop("disabled", false);
                        errorFlash("Error installing dependencies: " + xhr.statusText);
                    }, 1500);
                }
            }
        };
        
        xhr.onerror = function() {
            reports.updateInstallProgress(100, "Error", "Network error");
            reports.appendInstallLog("Network error while installing dependencies", 'error');
            
            setTimeout(function() {
                $("#dependency-install-progress").hide();
                $("#dependency-details").show();
                $("#install-dependencies").prop("disabled", false);
                errorFlash("Network error while installing dependencies");
            }, 1500);
        };
        
        xhr.send(JSON.stringify({}));
    },
    
    // Update installation progress bar
    updateInstallProgress: function(percentage, packageName, message) {
        $("#install-progress-bar").css("width", percentage + "%").attr("aria-valuenow", percentage);
        $("#install-progress-text").text(percentage + "%");
        $("#install-package-name").text(packageName);
        
        if (message) {
            reports.appendInstallLog(message, 'info');
        }
    },
    
    // Append message to installation log
    appendInstallLog: function(message, type) {
        var $log = $("#install-log");
        var timestamp = new Date().toLocaleTimeString();
        var icon = '';
        var color = '';
        
        switch(type) {
            case 'success':
                icon = '<i class="fa fa-check-circle" style="color: #5cb85c;"></i>';
                color = '#5cb85c';
                break;
            case 'error':
                icon = '<i class="fa fa-times-circle" style="color: #d9534f;"></i>';
                color = '#d9534f';
                break;
            case 'warning':
                icon = '<i class="fa fa-exclamation-triangle" style="color: #f0ad4e;"></i>';
                color = '#f0ad4e';
                break;
            default:
                icon = '<i class="fa fa-info-circle" style="color: #5bc0de;"></i>';
                color = '#5bc0de';
        }
        
        var logEntry = '<div style="margin-bottom: 5px; color: ' + color + ';">' +
            '<span style="color: #999;">[' + timestamp + ']</span> ' +
            icon + ' ' +
            escapeHtml(message) +
            '</div>';
        
        $log.append(logEntry);
        
        // Auto-scroll to bottom
        var logContainer = $("#install-log-container")[0];
        logContainer.scrollTop = logContainer.scrollHeight;
    },
    
    // Enable close button after installation completes (prevents auto-close)
    enableInstallCloseButton: function(success, errorMessage) {
        // Add close button in the modal footer
        var closeButtonHtml = '<div style="text-align: center; margin-top: 20px;">' +
            '<button id="install-close-btn" class="btn btn-' + (success ? 'success' : 'warning') + ' btn-lg">' +
            '<i class="fa fa-' + (success ? 'check' : 'exclamation-triangle') + '"></i> ' +
            (success ? 'Close and Refresh' : 'Close and Review') +
            '</button></div>';
        
        $("#dependency-install-progress").append(closeButtonHtml);
        
        // Disable the install button while showing results
        $("#install-dependencies").prop("disabled", true);
        
        // Handle close button click
        $("#install-close-btn").on("click", function() {
            // Refresh dependencies
            reports.checkDependencies();
            
            // Clean up and return to dependency details
            $("#install-close-btn").parent().remove();
            $("#dependency-install-progress").hide();
            $("#dependency-details").show();
            $("#install-dependencies").prop("disabled", false);
            
            // Close modal if successful
            if (success) {
                $("#dependency-modal").modal('hide');
                successFlash("Dependencies installed successfully!");
            } else {
                // Keep modal open for error review but show flash message
                errorFlash("Installation completed with errors. Please review the log above. " + (errorMessage || ""));
            }
        });
        
        // Add a helpful message
        var reviewMsg = success ? 
            "Click the button to continue." :
            "Please review the installation log above for error details.";
        reports.appendInstallLog(reviewMsg, success ? 'info' : 'warning');
    }
};

// Initialize on document ready
$(document).ready(function() {
    reports.init();
});

// Cleanup on page unload
$(window).on('beforeunload', function() {
    if (reports.statusPollingInterval) {
        clearInterval(reports.statusPollingInterval);
    }
});
