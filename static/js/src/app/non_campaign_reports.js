// non_campaign_reports.js

$(document).ready(function() {
    // Initialize DataTable
    var reportsTable = $("#non_campaign_table").DataTable({
        columnDefs: [
            {
                orderable: false,
                targets: "no-sort"
            }
        ],
        order: [[2, "desc"]], // Sort by reported date by default
        dom: "<'row'<'col-sm-6'l><'col-sm-6'f>>" +
             "<'row'<'col-sm-12'<'clear-reports-button'>>>" +
             "<'row'<'col-sm-12'tr>>" +
             "<'row'<'col-sm-5'i><'col-sm-7'p>>",
        initComplete: function() {
            // Add clear button to the designated spot
            $('.clear-reports-button').html(
                '<button id="clear_reports_btn" class="btn btn-danger pull-right" style="margin-bottom: 10px;">' +
                '<i class="fa fa-trash"></i> Clear Reports</button>'
            );
            
            // Attach the event handler for the button
            $("#clear_reports_btn").on('click', clearReportsHandler);
        }
    });

    // Since we're not supporting multiple IMAP configurations, we don't need to load them
    function loadIMAPConfigs() {
        // No need to load IMAP configurations as we're using a single configuration
        $("#filter_div").hide(); // Hide the filter div since we don't need it
    }

    // Load non-campaign reports
    function loadReports() {
        $("#loading").show();
        $("#stats_div").hide();
        $("#non_campaign_table_div").hide();
        $("#no_reports_message").hide();

        // Use query function directly instead of api.request
        query("/imap/non_campaign_reports", "GET", {}, true)
            .success(function(response) {
                $("#loading").hide();
                $("#filter_div").show();

                // Update stats
                if (response.stats && response.stats.report_count > 0) {
                    $("#report_count").text(response.stats.report_count);
                    
                    // Format the last reported date
                    var lastReported = "Never";
                    if (response.stats.last_reported_at && response.stats.last_reported_at !== "0001-01-01T00:00:00Z") {
                        lastReported = moment(response.stats.last_reported_at).format("MMMM Do YYYY, h:mm:ss a");
                    }
                    $("#last_reported").text(lastReported);
                    
                    $("#stats_div").show();
                }

                // Clear and reload the table
                reportsTable.clear();

                if (response.reports && response.reports.length > 0) {
                    // Add reports to the table
                    $.each(response.reports, function(i, report) {
                        var reportedAt = moment(report.reported_at).format("MMMM Do YYYY, h:mm:ss a");
                        
                        reportsTable.row.add([
                            escapeHtml(report.reporter_email),
                            escapeHtml(report.subject),
                            reportedAt
                        ]);
                    });
                    
                    reportsTable.draw();
                    $("#non_campaign_table_div").show();
                } else {
                    $("#no_reports_message").show();
                }

                // We don't need to set the selected IMAP ID since we're not supporting multiple configurations
            })
            .error(function(xhr, status, error) {
                $("#loading").hide();
                console.error("Error loading non-campaign reports:", error);
                errorFlash("Failed to load non-campaign reports");
            });
    }

    // We don't need to handle filter form submission since we're not supporting multiple configurations

    // Handler for clearing reports
    function clearReportsHandler() {
        Swal.fire({
            title: "Are you sure?",
            text: "This will permanently delete all displayed non-campaign phishing reports. This action cannot be undone.",
            type: "warning",
            showCancelButton: true,
            confirmButtonColor: "#d9534f",
            confirmButtonText: "Yes, delete them!",
            closeOnConfirm: false,
            showLoaderOnConfirm: true
        }).then(function(result) {
            if (result.value) {
                // Send the DELETE request
                query("/imap/non_campaign_reports", "DELETE", {}, true)
                    .success(function(response) {
                        Swal.fire({
                            title: "Deleted!",
                            text: "The reports have been deleted.",
                            type: "success"
                        }).then(function() {
                            // Reload the reports and ensure UI elements are updated properly
                            $("#stats_div").hide();
                            $("#non_campaign_table_div").hide();
                            
                            // Reset all elements
                            reportsTable.clear().draw();
                            $("#no_reports_message").hide();
                            
                            // Reload the reports
                            loadReports();
                        });
                    })
                    .error(function(xhr, status, error) {
                        console.error("API error:", error, xhr.responseText);
                        Swal.fire({
                            title: "Error!",
                            text: "An error occurred while deleting the reports. Please try again.",
                            type: "error"
                        });
                    });
            }
        });
    }

    // Helper function to escape HTML
    function escapeHtml(str) {
        return $("<div>").text(str).html();
    }

    // Initialize the page
    loadIMAPConfigs();
    loadReports();
    
    // Load active campaign RIDs for monitoring
    function loadActiveCampaignRIDs() {
        // Use the correct API function
        api.campaigns.get()
            .success(function(campaigns) {
                var activeCampaigns = campaigns.filter(function(campaign) {
                    return campaign.status === "In progress";
                });
                
                if (activeCampaigns.length > 0) {
                    var campaignList = $("<div class='alert alert-info'><strong>Active Campaign RIDs:</strong> ");
                    
                    activeCampaigns.forEach(function(campaign, index) {
                        campaignList.append("<span class='label label-primary'>" + campaign.name + " (RID: " + campaign.id + ")</span>");
                        if (index < activeCampaigns.length - 1) {
                            campaignList.append(" ");
                        }
                    });
                    
                    campaignList.append("</div>");
                    $("#stats_div").after(campaignList);
                }
            })
            .error(function(xhr, status, error) {
                console.error("Error loading active campaigns:", error);
            });
    }
    
    // Load active campaign RIDs
    loadActiveCampaignRIDs();
});
