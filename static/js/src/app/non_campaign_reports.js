// Non-Campaign Reports JS

var nonCampaignTable = null;
var currentImapId = 0;
var selectedReports = {};  // Map of report id -> true for selected reports

// Update the selection count display and button visibility
function updateReportSelectionUI() {
    var count = Object.keys(selectedReports).length;
    $('#selectedReportCount').text(count);
    if (count > 0) {
        $('#deleteSelectedReports').show();
    } else {
        $('#deleteSelectedReports').hide();
    }
}

// Clear all selections
function clearReportSelections() {
    selectedReports = {};
    $('input.report-checkbox').prop('checked', false);
    $('#selectAllReports').prop('checked', false).prop('indeterminate', false);
    updateReportSelectionUI();
}

// Handle individual checkbox change
function handleReportCheckboxChange(reportId) {
    var checkbox = $('input.report-checkbox[data-id="' + reportId + '"]');
    if (checkbox.is(':checked')) {
        selectedReports[reportId] = true;
    } else {
        delete selectedReports[reportId];
    }
    updateReportSelectionUI();
    updateSelectAllCheckbox();
}

// Update select all checkbox state
function updateSelectAllCheckbox() {
    if (!nonCampaignTable) return;
    var allCheckboxes = $(nonCampaignTable.table().body()).find('input.report-checkbox');
    var checkedCount = allCheckboxes.filter(':checked').length;
    var totalCount = allCheckboxes.length;
    
    if (totalCount === 0 || checkedCount === 0) {
        $('#selectAllReports').prop('checked', false).prop('indeterminate', false);
    } else if (checkedCount === totalCount) {
        $('#selectAllReports').prop('checked', true).prop('indeterminate', false);
    } else {
        $('#selectAllReports').prop('checked', false).prop('indeterminate', true);
    }
}

// Handle select all checkbox
function handleSelectAllReports() {
    if (!nonCampaignTable) return;
    var isChecked = $('#selectAllReports').is(':checked');
    var allCheckboxes = $(nonCampaignTable.table().body()).find('input.report-checkbox');
    
    allCheckboxes.each(function() {
        $(this).prop('checked', isChecked);
        var reportId = $(this).data('id');
        if (isChecked) {
            selectedReports[reportId] = true;
        } else {
            delete selectedReports[reportId];
        }
    });
    updateReportSelectionUI();
}

// Delete selected reports
function deleteSelectedReports() {
    var ids = Object.keys(selectedReports).map(function(id) { return parseInt(id); });
    if (ids.length === 0) return;
    
    var confirmText = ids.length === 1 
        ? "Delete 1 report?" 
        : "Delete " + ids.length + " reports?";
    
    Swal.fire({
        title: "Are you sure?",
        text: confirmText + " This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#d9534f",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.IMAP.bulkDeleteReports(ids)
                    .success(function (msg) { resolve(msg) })
                    .error(function (data) { reject(data.responseJSON ? data.responseJSON.message : "An error occurred") })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Reports Deleted!', result.value.message || 'Selected reports have been deleted.', 'success');
            selectedReports = {};
            // Reload the reports
            load(currentImapId);
        }
    })
}
window.deleteSelectedReports = deleteSelectedReports;

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
            // Build the API URL with query parameters if needed
            var apiUrl = "/api/imap/non_campaign_reports";
            var currentImapId = $("#imap_filter").val() || 0;
            
            if (currentImapId > 0) {
                apiUrl += "?imap_id=" + currentImapId;
            }
            
            // Send the DELETE request
            $.ajax({
                url: apiUrl,
                method: "DELETE",
                beforeSend: function(xhr) {
                    xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
                }
            })
            .done(function(response) {
                Swal.fire({
                    title: "Deleted!",
                    text: "The reports have been deleted.",
                    type: "success"
                }).then(function() {
                    // Reload the reports and ensure UI elements are updated properly
                    $("#stats_div").hide();
                    $("#non_campaign_table_div").hide();
                    $("#filter_div").hide();
                    
                    // Reset all elements
                    nonCampaignTable.clear().draw();
                    $("#no_reports_message").hide();
                    
                    // Reload the reports
                    load(currentImapId);
                });
            })
            .fail(function(xhr, status, error) {
                // console.error("API error:", error, xhr.responseText);
                Swal.fire({
                    title: "Error!",
                    text: "An error occurred while deleting the reports. Please try again.",
                    type: "error"
                });
            });
        }
    });
}

function load(imapId) {
    $("#loading").show();
    // console.log("Attempting to load non-campaign reports...");
    
    // Set the current IMAP ID
    currentImapId = imapId || 0;
    
    // Build the API URL with query parameters if needed
    var apiUrl = "/api/imap/non_campaign_reports";
    if (currentImapId > 0) {
        apiUrl += "?imap_id=" + currentImapId;
    }
    
    $.ajax({
        url: apiUrl,
        method: "GET",
        dataType: "json",
        contentType: "application/json",
        beforeSend: function(xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(response) {
        // console.log("API response received:", response);
        $("#loading").hide();
        
        if (!response) {
            // console.error("Empty response from API");
            $("#no_reports_message").show();
            return;
        }
        
        // Set stats summary
        if (response.stats && response.stats.report_count > 0) {
            $("#stats_div").show();
            $("#report_count").text(response.stats.report_count);
            $("#last_reported").text(moment(response.stats.last_reported_at).format('MMMM Do YYYY, h:mm:ss a'));
        } else {
            $("#no_reports_message").show();
            return;
        }
        
        // Populate IMAP filter dropdown if we have configurations
        if (response.imap_configs && response.imap_configs.length > 0) {
            $("#filter_div").show();
            var filterDropdown = $("#imap_filter");
            
            // Clear all options except the first one
            filterDropdown.find('option:not(:first)').remove();
            
            // Add options for each IMAP configuration
            $.each(response.imap_configs, function(i, config) {
                filterDropdown.append(
                    $('<option></option>')
                        .val(config.id)
                        .text(config.name)
                );
            });
            
            // Set the selected value based on the response
            filterDropdown.val(response.selected_imap_id);
        }
        
        if (!response.reports || response.reports.length === 0) {
            $("#no_reports_message").show();
            return;
        }
        
        $("#non_campaign_table_div").show();
        
        // Build the reports table
        nonCampaignTable = $("#non_campaign_table").DataTable({
            destroy: true,
            columnDefs: [{
                orderable: false,
                targets: "no-sort"
            }],
            order: [
                [4, "desc"]
            ],
            data: response.reports,
            columns: [{
                title: "<input type='checkbox' id='selectAllReports' title='Select All'>",
                data: function(row) {
                    return "<input type='checkbox' class='report-checkbox' data-id='" + row.id + "'>";
                },
                className: "no-sort",
                orderable: false
            }, {
                title: "Reporter",
                data: "reporter_email",
                className: "reporter_email"
            }, {
                title: "Subject",
                data: "subject",
                className: "subject"
            }, {
                title: "",
                className: "no-sort",
                orderable: false,
                data: function(row) {
                    if (!row.imap_uid && !row.message_id) {
                        return '<button class="btn btn-xs btn-default" disabled ' +
                               'title="Not available for reports created before this feature">' +
                               '<i class="fa fa-envelope-o"></i> View</button>';
                    }
                    return '<button class="btn btn-xs btn-primary view-message-btn" data-id="' + row.id + '">' +
                           '<i class="fa fa-envelope-o"></i> View</button>';
                }
            }, {
                title: "Reported At",
                data: function(row) {
                    return moment(row.reported_at).format('MMMM Do YYYY, h:mm:ss a');
                },
                className: "reported_at"
            }],
            // Add the action buttons to the DataTable dom
            dom: "<'row'<'col-sm-6'l><'col-sm-6'f>>" +
                 "<'row'<'col-sm-12'<'action-buttons'>>>" +
                 "<'row'<'col-sm-12'tr>>" +
                 "<'row'<'col-sm-5'i><'col-sm-7'p>>",
            // Initialize callback to add our buttons to the custom container
            initComplete: function() {
                // Add action buttons
                $('.action-buttons').html(
                    '<button id="deleteSelectedReports" class="btn btn-danger" style="display:none; margin-right: 10px; margin-bottom: 10px;" onclick="deleteSelectedReports()">' +
                    '<i class="fa fa-trash-o"></i> Delete Selected (<span id="selectedReportCount">0</span>)</button>' +
                    '<button id="clear_reports_btn" class="btn btn-danger pull-right" style="margin-bottom: 10px;">' +
                    '<i class="fa fa-trash"></i> Clear All Reports</button>'
                );
                
                // Reattach the event handler for the clear button
                $("#clear_reports_btn").on('click', clearReportsHandler);
                
                // Set up checkbox event handlers
                $('#selectAllReports').off('change').on('change', function() {
                    handleSelectAllReports();
                });
                $(document).off('change', 'input.report-checkbox').on('change', 'input.report-checkbox', function() {
                    var reportId = $(this).data('id');
                    handleReportCheckboxChange(reportId);
                });

                $(document).off('click', '.view-message-btn').on('click', '.view-message-btn', function() {
                    var reportId = $(this).data('id');
                    showMessageModal({ url: "/api/imap/non_campaign_reports/" + reportId + "/message" });
                });

                // Clear selections when loading
                clearReportSelections();
            }
        });
    })
    .fail(function(xhr, status, error) {
        // console.error("API error:", error, xhr.responseText);
        $("#loading").hide();
        errorFlash("Error loading non-campaign reports: " + error);
    });
}

$(document).ready(function() {
    // Setup error div
    $('#modal').on('hidden.bs.modal', function(event) {
        $(this).removeClass('modal-error');
        $('.modal-content', this).removeClass('modal-error');
    });
    
    // Initialize datatable
    nonCampaignTable = $("#non_campaign_table").DataTable({
        destroy: true,
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }],
        order: [
            [2, "desc"]
        ],
    });
    
    // Directly use the query function instead of going through api object
    // This avoids any issues with the api object registration
    api.nonCampaignReports = {
        get: function() {
            // Don't show the no_reports_message by default
            $("#no_reports_message").hide();
            return $.ajax({
                url: "/api/imap/non_campaign_reports",
                method: "GET",
                dataType: "json",
                contentType: "application/json",
                beforeSend: function(xhr) {
                    xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
                }
            }).fail(function(xhr, status, error) {
                // console.error("API error:", xhr.status, error);
                $("#loading").hide();
                errorFlash("Error loading non-campaign reports. Status: " + xhr.status);
            });
        }
    };
    
    // Add event handler for the IMAP filter dropdown
    $("#imap_filter").on('change', function() {
        var selectedImapId = $(this).val();
        load(selectedImapId);
    });
    
    // Load the reports
    load();
    
    // If still seeing the loading spinner after 5 seconds, show an error
    setTimeout(function() {
        if ($("#loading").is(":visible")) {
            $("#loading").hide();
            $("#no_reports_message").show();
            errorFlash("Timed out waiting for report data. API endpoint may not be responding.");
        }
    }, 5000);
});

var repliesTable = null;
var replyRows = {};

function loadReplies(campaignId) {
    var apiUrl = "/api/imap/replies";
    if (campaignId && campaignId > 0) {
        apiUrl += "?campaign_id=" + campaignId;
    }

    $.ajax({
        url: apiUrl,
        method: "GET",
        dataType: "json",
        beforeSend: function(xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
    .done(function(response) {
        var filterDropdown = $("#campaign_filter");
        filterDropdown.find('option:not(:first)').remove();
        $.each(response.campaigns || [], function(i, c) {
            filterDropdown.append($('<option></option>').val(c.id).text(c.name));
        });
        filterDropdown.val(response.selected_campaign_id || 0);

        var replies = response.replies || [];
        replyRows = {};
        $.each(replies, function(i, r) { replyRows[r.event_id] = r; });

        if (replies.length === 0) {
            $("#no_replies_message").show();
            $("#replies_table").hide();
            return;
        }
        $("#no_replies_message").hide();
        $("#replies_table").show();

        repliesTable = $("#replies_table").DataTable({
            destroy: true,
            columnDefs: [{ orderable: false, targets: "no-sort" }],
            order: [[2, "desc"]],
            data: replies,
            columns: [{
                title: "Campaign",
                data: "campaign_name"
            }, {
                title: "Recipient",
                data: "email"
            }, {
                title: "Replied At",
                data: function(row) {
                    return moment(row.time).format('MMMM Do YYYY, h:mm:ss a');
                }
            }, {
                title: "",
                className: "no-sort",
                orderable: false,
                data: function(row) {
                    if (!row.message) {
                        return '<button class="btn btn-xs btn-default" disabled ' +
                               'title="No content was captured for this reply">' +
                               '<i class="fa fa-envelope-o"></i> View</button>';
                    }
                    return '<button class="btn btn-xs btn-primary view-reply-btn" data-id="' + row.event_id + '">' +
                           '<i class="fa fa-envelope-o"></i> View</button>';
                }
            }]
        });
    })
    .fail(function(xhr, status, error) {
        errorFlash("Error loading replies: " + error);
    });
}

$(document).ready(function() {
    $('#monitorTabs a[href="#repliesPane"]').on('shown.bs.tab', function() {
        loadReplies($("#campaign_filter").val() || 0);
    });

    $("#campaign_filter").on('change', function() {
        loadReplies($(this).val());
    });

    $(document).on('click', '.view-reply-btn', function() {
        var row = replyRows[$(this).data('id')];
        if (!row || !row.message) { return; }
        // text and html are omitempty on the wire, so either may be absent.
        // Concatenating an absent text yields the string "undefined", which
        // defeats the modal's own empty-content fallback.
        var replyText = row.message.text || "";
        if (row.message.truncated) {
            replyText += "\n\n[Content truncated at 256KB]";
        }
        showMessageModal({
            message: {
                subject: "Reply from " + row.email,
                from: row.email,
                date: moment(row.time).format('MMMM Do YYYY, h:mm:ss a'),
                text: replyText,
                html: row.message.html || "",
                headers: row.message.headers || []
            }
        });
    });
});
