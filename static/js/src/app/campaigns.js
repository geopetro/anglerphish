// labels is a map of campaign statuses to
// CSS classes
var labels = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger"
}

var campaigns = []
var campaign = {}
var activeCampaignsTable = null
var archivedCampaignsTable = null
var selectedCampaigns = {}  // Map of campaign id -> true for selected campaigns
var currentTab = 'active'   // Track current active tab

// Update the selection count display and button visibility
function updateSelectionUI() {
    var count = Object.keys(selectedCampaigns).length;
    
    $('#selectedCount').text(count);
    $('#selectedCompleteCount').text(count);
    if (count > 0) {
        $('#deleteSelectedCampaigns').show();
    } else {
        $('#deleteSelectedCampaigns').hide();
    }

    // Completing only applies to active (non-completed) campaigns, so the
    // Complete button is limited to the Active tab.
    if (count > 0 && currentTab === 'active') {
        $('#completeSelectedCampaigns').show();
    } else {
        $('#completeSelectedCampaigns').hide();
    }
}

// Clear all selections and update UI
function clearSelections() {
    selectedCampaigns = {};
    
    // Uncheck all checkboxes
    $('input.campaign-checkbox').prop('checked', false);
    $('#selectAllActive').prop('checked', false).prop('indeterminate', false);
    $('#selectAllArchived').prop('checked', false).prop('indeterminate', false);
    
    updateSelectionUI();
}

// Handle individual checkbox change
function handleCheckboxChange(campaignId) {
    var checkbox = $('input.campaign-checkbox[data-id="' + campaignId + '"]');
    
    if (checkbox.is(':checked')) {
        selectedCampaigns[campaignId] = true;
    } else {
        delete selectedCampaigns[campaignId];
    }
    
    updateSelectionUI();
    updateSelectAllCheckbox();
}

// Update the "select all" checkbox state based on individual selections
function updateSelectAllCheckbox() {
    var table = currentTab === 'active' ? activeCampaignsTable : archivedCampaignsTable;
    var selectAll = currentTab === 'active' ? '#selectAllActive' : '#selectAllArchived';
    
    if (!table) return;
    
    var allCheckboxes = $(table.table().body()).find('input.campaign-checkbox');
    var checkedCount = allCheckboxes.filter(':checked').length;
    var totalCount = allCheckboxes.length;
    
    if (totalCount === 0) {
        $(selectAll).prop('checked', false);
        $(selectAll).prop('indeterminate', false);
    } else if (checkedCount === 0) {
        $(selectAll).prop('checked', false);
        $(selectAll).prop('indeterminate', false);
    } else if (checkedCount === totalCount) {
        $(selectAll).prop('checked', true);
        $(selectAll).prop('indeterminate', false);
    } else {
        $(selectAll).prop('checked', false);
        $(selectAll).prop('indeterminate', true);
    }
}

// Handle "select all" checkbox click
function handleSelectAll() {
    var table = currentTab === 'active' ? activeCampaignsTable : archivedCampaignsTable;
    var selectAll = currentTab === 'active' ? '#selectAllActive' : '#selectAllArchived';
    
    if (!table) return;
    
    var isChecked = $(selectAll).is(':checked');
    var allCheckboxes = $(table.table().body()).find('input.campaign-checkbox');
    
    allCheckboxes.each(function() {
        $(this).prop('checked', isChecked);
        var campaignId = $(this).data('id');
        if (isChecked) {
            selectedCampaigns[campaignId] = true;
        } else {
            delete selectedCampaigns[campaignId];
        }
    });
    
    updateSelectionUI();
}

// Delete selected campaigns
function deleteSelectedCampaigns() {
    var ids = Object.keys(selectedCampaigns).map(function(id) { return parseInt(id); });
    
    if (ids.length === 0) {
        return;
    }
    
    // Get campaign names for the confirmation message
    var names = [];
    ids.forEach(function(id) {
        var campaign = campaigns.find(function(c) { return c.id === id; });
        if (campaign) {
            names.push(campaign.name);
        }
    });
    
    var confirmText = ids.length === 1 
        ? "Delete campaign: " + names[0] + "?" 
        : "Delete " + ids.length + " campaigns?";
    
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
                api.campaigns.bulkDelete(ids)
                    .success(function (msg) {
                        resolve(msg)
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'Campaigns Deleted!',
                result.value.message,
                'success'
            );
            // Clear selection and reload page
            selectedCampaigns = {};
            $('button:contains("OK")').on('click', function () {
                location.reload()
            })
        }
    })
}
window.deleteSelectedCampaigns = deleteSelectedCampaigns;

// Complete selected campaigns
function completeSelectedCampaigns() {
    var ids = Object.keys(selectedCampaigns).map(function(id) { return parseInt(id); });

    if (ids.length === 0) {
        return;
    }

    var confirmText = ids.length === 1
        ? "Complete this campaign?"
        : "Complete " + ids.length + " campaigns?";

    Swal.fire({
        title: "Are you sure?",
        text: confirmText + " Gophish will stop processing events for them.",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Complete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            // Complete each selected campaign independently, tolerating
            // individual failures so one bad campaign doesn't block the rest.
            return Promise.all(ids.map(function (id) {
                return new Promise(function (resolve) {
                    api.campaignId.complete(id)
                        .success(function () { resolve({ id: id, ok: true }); })
                        .error(function () { resolve({ id: id, ok: false }); })
                });
            })).then(function (results) {
                return results.filter(function (r) { return r.ok; }).length;
            });
        }
    }).then(function (result) {
        if (typeof result.value !== "undefined") {
            var succeeded = result.value;
            Swal.fire(
                'Campaigns Completed!',
                succeeded + ' of ' + ids.length + ' campaign(s) marked as complete.',
                succeeded === ids.length ? 'success' : 'warning'
            );
            selectedCampaigns = {};
            $('button:contains("OK")').on('click', function () {
                location.reload()
            })
        }
    })
}
window.completeSelectedCampaigns = completeSelectedCampaigns;

// generateCampaignSummary creates a formatted summary of campaign settings
function generateCampaignSummary() {
    // Get the campaign type
    var campaignType = $("#campaign_type").val();
    
    // Format dates
    var launchDate = $("#launch_date").val();
    var sendByDate = $("#send_by_date").val();
    
    // Build common summary parts
    var summary = "<div style='text-align: left;'>";
    summary += "<strong>Campaign Name:</strong> " + $("#name").val() + "<br>";
    
    // Display campaign type
    var typeDisplay = campaignType === "email" ? "Email" : (campaignType === "sms" ? "SMS" : "Generic (Landing Page Only)");
    summary += "<strong>Campaign Type:</strong> " + typeDisplay + "<br>";
    summary += "<strong>Landing Page:</strong> " + ($("#page").select2("data")[0] ? $("#page").select2("data")[0].text : "None") + "<br>";
    
    // Add type-specific details
    if (campaignType === "email") {
        summary += "<strong>Email Template:</strong> " + ($("#template").select2("data")[0] ? $("#template").select2("data")[0].text : "None") + "<br>";
        summary += "<strong>Sending Profile:</strong> " + ($("#profile").select2("data")[0] ? $("#profile").select2("data")[0].text : "None") + "<br>";
        
        // Add target groups for email
        summary += "<strong>Target Groups:</strong> ";
        var groups = [];
        $("#users").select2("data").forEach(function (group) {
            groups.push(group.text);
        });
        summary += groups.join(", ") + "<br>";
        
        // Get the number of recipients
        var totalRecipients = 0;
        $("#users").select2("data").forEach(function (group) {
            var match = group.title && group.title.match(/(\d+) targets/);
            if (match && match[1]) {
                totalRecipients += parseInt(match[1]);
            }
        });
        summary += "<strong>Total Recipients:</strong> " + totalRecipients + "<br>";
    } else if (campaignType === "sms") {
        summary += "<strong>SMS Template:</strong> " + ($("#sms_template").select2("data")[0] ? $("#sms_template").select2("data")[0].text : "None") + "<br>";
        summary += "<strong>SMS Sending Profile:</strong> " + ($("#sms_profile").select2("data")[0] ? $("#sms_profile").select2("data")[0].text : "None") + "<br>";
        
        // Add target groups for SMS
        summary += "<strong>Target Groups:</strong> ";
        var groups = [];
        $("#users").select2("data").forEach(function (group) {
            groups.push(group.text);
        });
        summary += groups.join(", ") + "<br>";
        
        // Get the number of recipients
        var totalRecipients = 0;
        $("#users").select2("data").forEach(function (group) {
            var match = group.title && group.title.match(/(\d+) targets/);
            if (match && match[1]) {
                totalRecipients += parseInt(match[1]);
            }
        });
        summary += "<strong>Total Recipients:</strong> " + totalRecipients + "<br>";
    } else if (campaignType === "generic") {
        summary += "<strong>Distribution:</strong> Manual (via tracking URL)<br>";
        summary += "<strong>Note:</strong> No emails/SMS will be sent. You will receive a tracking URL after launch.<br>";
    }
    
    // Add URL and other settings
    summary += "<strong>URL:</strong> " + $("#url").val() + "<br>";
    summary += "<strong>Launch Date:</strong> " + launchDate + "<br>";
    if (sendByDate && campaignType !== "generic") {
        summary += "<strong>Send By Date:</strong> " + sendByDate + "<br>";
    }
    
    // Add HTTP Basic Auth if enabled
    if ($('#basicauth').is(":checked")) {
        summary += "<strong>HTTP Basic Auth:</strong> Enabled<br>";
    }

    // Add randomized send order if enabled
    if ($('#randomize_send_order').is(":checked")) {
        summary += "<strong>Randomize Send Order:</strong> Enabled<br>";
    }
    
    // Add QR code size if specified
    var qrSize = $("#qrsize").val();
    if (qrSize) {
        summary += "<strong>QR Code Size:</strong> " + qrSize + "<br>";
    }
    
    summary += "</div>";
    return summary;
}

// Launch attempts to POST to /campaigns/
function launch() {
    // Generate campaign summary
    var campaignSummary = generateCampaignSummary();
    
    Swal.fire({
        title: "Campaign Summary",
        html: "Please review the campaign settings before launching:<br><br>" + campaignSummary,
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                // Get the campaign type
                var campaignType = $("#campaign_type").val();
                
                // Get groups (only for non-generic campaigns)
                var groups = [];
                if (campaignType !== "generic") {
                    $("#users").select2("data").forEach(function (group) {
                        groups.push({
                            name: group.text
                        });
                    });
                }
                
                // Validate our fields
                var send_by_date = $("#send_by_date").val()
                if (send_by_date != "" && campaignType !== "generic") {
                    send_by_date = moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format()
                } else {
                    send_by_date = null;
                }

                var urlParamValue = $("#urlparam").val();
                var urlparam = urlParamValue !== '' ? urlParamValue : 'rid';
                
                // Common campaign properties
                campaign = {
                    name: $("#name").val(),
                    type: campaignType,
                    url: $("#url").val(),
                    urlparam: urlparam,
                    qrsize: $("#qrsize").val(),
                    page: {
                        name: $("#page").select2("data")[0].text
                    },
                    basicauth: $('#basicauth').is(":checked"),
                    launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
                    send_by_date: send_by_date,
                    groups: groups,
                    randomize_send_order: $('#randomize_send_order').is(":checked"),
                }
                
                // Add type-specific properties
                if (campaignType === "email") {
                    campaign.template = {
                        name: $("#template").select2("data")[0].text
                    };
                    campaign.smtp = {
                        name: $("#profile").select2("data")[0].text
                    };
                } else if (campaignType === "sms") {
                    campaign.sms_template = {
                        name: $("#sms_template").select2("data")[0].text
                    };
                    campaign.sms = {
                        name: $("#sms_profile").select2("data")[0].text
                    };
                }
                // Generic campaigns don't need template, smtp, sms_template, or sms
                
                // Submit the campaign
                api.campaigns.post(campaign)
                    .success(function (data) {
                        resolve()
                        campaign = data
                    })
                    .error(function (data) {
                        $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
                        Swal.close()
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            // For generic campaigns, show the tracking URL
            if (campaign.type === "generic" && campaign.results && campaign.results.length > 0) {
                // Close the campaign creation modal first
                $("#modal").modal('hide');
                
                var urlParam = campaign.urlparam || 'rid';
                var trackingUrl = campaign.url + '?' + urlParam + '=' + campaign.results[0].id;
                
                Swal.fire({
                    title: 'Generic Campaign Launched!',
                    html: '<p>Your tracking URL is ready:</p>' +
                          '<div class="input-group" style="margin-top: 10px;">' +
                          '<input type="text" class="form-control" id="trackingUrlInput" value="' + escapeHtml(trackingUrl) + '" readonly>' +
                          '<span class="input-group-btn">' +
                          '<button class="btn btn-primary" type="button" onclick="copyTrackingUrl()"><i class="fa fa-copy"></i> Copy</button>' +
                          '</span></div>' +
                          '<p style="margin-top: 15px; font-size: 12px; color: #666;">You can generate more links from the campaign results page.</p>',
                    type: 'success',
                    showCancelButton: false,
                    confirmButtonText: 'View Campaign',
                    confirmButtonColor: '#428bca'
                }).then(function() {
                    window.location = "/campaigns/" + campaign.id.toString()
                });
            } else {
                Swal.fire(
                    'Campaign Scheduled!',
                    'This campaign has been scheduled for launch!',
                    'success'
                );
                $('button:contains("OK")').on('click', function () {
                    window.location = "/campaigns/" + campaign.id.toString()
                })
            }
        }
    })
}

// Helper function to copy tracking URL to clipboard
function copyTrackingUrl() {
    var input = document.getElementById('trackingUrlInput');
    input.select();
    input.setSelectionRange(0, 99999);
    document.execCommand('copy');
    
    // Show feedback by changing button text temporarily
    var btn = $(event.target).closest('button');
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-check"></i> Copied!');
    btn.removeClass('btn-primary').addClass('btn-success');
    
    setTimeout(function() {
        btn.html(originalHtml);
        btn.removeClass('btn-success').addClass('btn-primary');
    }, 2000);
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    var test_email_request = {
        template: {
            name: $("#template").select2("data")[0].text
        },
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        custom: $("input[name=to_custom]").val(),
        url: $("#url").val(),
        page: {
            name: $("#page").select2("data")[0].text
        },
        smtp: {
            name: $("#profile").select2("data")[0].text
        }
    }
    btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    // Send the test email
    api.send_test_email(test_email_request)
        .success(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
            <i class=\"fa fa-check-circle\"></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#campaign_type").val("email");
    
    // Reset campaign type buttons to default (Email)
    $(".campaign-type-btn").removeClass("btn-primary active").addClass("btn-default");
    $(".campaign-type-btn[data-type='email']").removeClass("btn-default").addClass("btn-primary active");
    
    // Show email fields, hide SMS and generic fields
    $("#email_template_div").show();
    $("#sms_template_div").hide();
    $("#email_profile_div").show();
    $("#sms_profile_div").hide();
    $("#groups_div").show();
    $("#generic_info_div").hide();
    
    $("#template").val("").change();
    $("#sms_template").val("").change();
    $("#page").val("").change();
    $("#url").val("");
    $("#urlLengthIndicator").html("");
    $("#profile").val("").change();
    $("#sms_profile").val("").change();
    $("#users").val("").change();
    $("#modal").modal('hide');
}

function deleteCampaign(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + campaigns[idx].name,
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaigns[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Deleted!',
                'This campaign has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function resendFailed(campaignId) {
    Swal.fire({
        title: "Resend failed messages?",
        text: "All recipients with send errors will be re-queued. They should be sent within the next minute.",
        type: "warning",
        showCancelButton: true,
        confirmButtonText: "Resend",
        confirmButtonColor: "#f0ad4e",
        reverseButtons: true,
        showLoaderOnConfirm: true,
        preConfirm: function() {
            return new Promise(function(resolve) {
                api.campaignId.resendFailed(campaignId)
                    .success(function(msg) { resolve(msg) })
                    .error(function(data) {
                        var msg = (data.responseJSON && data.responseJSON.message) || 'An error occurred';
                        Swal.showValidationMessage(msg);
                        resolve(false);
                    })
            })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire({
                title: 'Queued!',
                text: result.value.message,
                type: 'success',
                timer: 2000,
                showConfirmButton: false
            }).then(function() {
                location.reload();
            });
        }
    })
}
window.resendFailed = resendFailed;

function setupOptions() {
    api.groups.summary()
        .success(function (summaries) {
            groups = summaries.groups.filter(function (g) { return !g.locked; })
            if (groups.length == 0) {
                modalError("No groups found!")
                return false;
            } else {
                var group_s2 = $.map(groups, function (obj) {
                    obj.text = obj.name
                    obj.title = obj.num_targets + " targets"
                    return obj
                });
                $("#users.form-control").select2({
                    placeholder: "Select Groups",
                    data: group_s2,
                });
            }
        });
    
    // Load email templates
    api.templates.get()
        .success(function (templates) {
            if (templates.length == 0) {
                modalError("No email templates found!")
                return false
            } else {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var template_select = $("#template.form-control")
                template_select.select2({
                    placeholder: "Select an Email Template",
                    data: template_s2,
                });
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        });
    
    // Load SMS templates
    api.smsTemplates.get()
        .success(function (templates) {
            // Only store the templates, don't show error here
            // Error will be shown when switching to SMS campaign type
            if (templates.length > 0) {
                var template_s2 = $.map(templates, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var template_select = $("#sms_template.form-control")
                template_select.select2({
                    placeholder: "Select an SMS Template",
                    data: template_s2,
                });
                if (templates.length === 1) {
                    template_select.val(template_s2[0].id)
                    template_select.trigger('change.select2')
                }
            }
        });
    
    api.pages.get()
        .success(function (pages) {
            if (pages.length == 0) {
                modalError("No pages found!")
                return false
            } else {
                var page_s2 = $.map(pages, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var page_select = $("#page.form-control")
                page_select.select2({
                    placeholder: "Select a Landing Page",
                    data: page_s2,
                });
                if (pages.length === 1) {
                    page_select.val(page_s2[0].id)
                    page_select.trigger('change.select2')
                }
            }
        });
    
    // Load email sending profiles
    api.SMTP.get()
        .success(function (profiles) {
            if (profiles.length == 0) {
                modalError("No email sending profiles found!")
                return false
            } else {
                var profile_s2 = $.map(profiles, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var profile_select = $("#profile.form-control")
                profile_select.select2({
                    placeholder: "Select an Email Sending Profile",
                    data: profile_s2,
                }).select2("val", profile_s2[0]);
                if (profiles.length === 1) {
                    profile_select.val(profile_s2[0].id)
                    profile_select.trigger('change.select2')
                }
            }
        });
    
    // Load SMS sending profiles
    api.SMS.get()
        .success(function (profiles) {
            if (profiles.length == 0) {
                return false
            } else {
                var profile_s2 = $.map(profiles, function (obj) {
                    obj.text = obj.name
                    return obj
                });
                var profile_select = $("#sms_profile.form-control")
                profile_select.select2({
                    placeholder: "Select an SMS Sending Profile",
                    data: profile_s2,
                }).select2("val", profile_s2[0]);
                if (profiles.length === 1) {
                    profile_select.val(profile_s2[0].id)
                    profile_select.trigger('change.select2')
                }
            }
        });
}

function edit(campaign) {
    setupOptions();
}

function copy(idx) {
    setupOptions();
    // Set our initial values
    api.campaignId.get(campaigns[idx].id)
        .success(function (campaign) {
            $("#name").val("Copy of " + campaign.name)
            
            // Set the campaign type from the original campaign
            var campaignType = campaign.type || "email";
            $("#campaign_type").val(campaignType);
            
            // Update the UI buttons to reflect the correct campaign type
            $(".campaign-type-btn").removeClass("btn-primary active").addClass("btn-default");
            $(".campaign-type-btn[data-type='" + campaignType + "']")
                .removeClass("btn-default").addClass("btn-primary active");
            
            // Show/hide appropriate fields based on campaign type
            if (campaignType === "sms") {
                // SMS campaign - show SMS fields, hide email fields
                $("#email_template_div").hide();
                $("#sms_template_div").show();
                $("#email_profile_div").hide();
                $("#sms_profile_div").show();
                $("#groups_div").show();
                $("#generic_info_div").hide();
                
                // Populate SMS-specific fields
                if (campaign.sms_template && campaign.sms_template.id) {
                    $("#sms_template").val(campaign.sms_template.id.toString());
                    $("#sms_template").trigger("change.select2");
                } else if (campaign.sms_template && campaign.sms_template.name) {
                    $("#sms_template").val("").change();
                    $("#sms_template").select2({
                        placeholder: campaign.sms_template.name
                    });
                }
                
                if (campaign.sms && campaign.sms.id) {
                    $("#sms_profile").val(campaign.sms.id.toString());
                    $("#sms_profile").trigger("change.select2");
                } else if (campaign.sms && campaign.sms.name) {
                    $("#sms_profile").val("").change();
                    $("#sms_profile").select2({
                        placeholder: campaign.sms.name
                    });
                }
            } else if (campaignType === "generic") {
                // Generic campaign - hide all template/profile fields
                $("#email_template_div").hide();
                $("#sms_template_div").hide();
                $("#email_profile_div").hide();
                $("#sms_profile_div").hide();
                $("#groups_div").hide();
                $("#generic_info_div").show();
            } else {
                // Email campaign (default) - show email fields, hide SMS fields
                $("#email_template_div").show();
                $("#sms_template_div").hide();
                $("#email_profile_div").show();
                $("#sms_profile_div").hide();
                $("#groups_div").show();
                $("#generic_info_div").hide();
                
                // Populate email-specific fields
                if (campaign.template && campaign.template.id) {
                    $("#template").val(campaign.template.id.toString());
                    $("#template").trigger("change.select2");
                } else if (campaign.template && campaign.template.name) {
                    $("#template").val("").change();
                    $("#template").select2({
                        placeholder: campaign.template.name
                    });
                }
                
                if (campaign.smtp && campaign.smtp.id) {
                    $("#profile").val(campaign.smtp.id.toString());
                    $("#profile").trigger("change.select2");
                } else if (campaign.smtp && campaign.smtp.name) {
                    $("#profile").val("").change();
                    $("#profile").select2({
                        placeholder: campaign.smtp.name
                    });
                }
            }
            
            // Set common fields (page, URL, groups)
            if (campaign.page) {
                if (!campaign.page.id) {
                    $("#page").val("").change();
                    $("#page").select2({
                        placeholder: campaign.page.name
                    });
                } else {
                    $("#page").val(campaign.page.id.toString());
                    $("#page").trigger("change.select2")
                }
            }
            
            // Set URL and related fields
            $("#url").val(campaign.url);
            if (campaign.urlparam) {
                $("#urlparam").val(campaign.urlparam);
            }
            if (campaign.qrsize) {
                $("#qrsize").val(campaign.qrsize);
            }
            if (campaign.basicauth) {
                $("#basicauth").prop("checked", true);
            } else {
                $("#basicauth").prop("checked", false);
            }
            $("#randomize_send_order").prop("checked", !!campaign.randomize_send_order);

            // Update URL length indicator
            updateURLLengthIndicator();
            
            // Populate groups (for non-generic campaigns)
            if (campaignType !== "generic" && campaign.groups && campaign.groups.length > 0) {
                var groupIds = campaign.groups.map(function(g) { return g.id.toString(); });
                $("#users").val(groupIds);
                $("#users").trigger("change.select2");
            }
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
            <i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>")
        })
}

// URL Template functionality
var urlTemplates = [];

function loadURLTemplates() {
    api.urlTemplates.get()
        .success(function (templates) {
            urlTemplates = templates;
            displayURLTemplates();
        })
        .error(function (data) {
            modalError("Error loading URL templates");
        });
}

function displayURLTemplates() {
    var templateList = $("#urlTemplateList");
    templateList.empty();
    $("#urlTemplateSearch").val('');

    if (urlTemplates.length === 0) {
        templateList.append('<div class="alert alert-info">No URL templates available. Click "Add Custom" to create one.</div>');
        return;
    }

    // Custom templates at the top
    var customTemplates = urlTemplates.filter(function(t) { return !t.is_preset; });
    if (customTemplates.length > 0) {
        templateList.append(
            '<div class="url-template-divider" style="padding:4px 2px;margin-bottom:2px;font-size:11px;font-weight:bold;color:#f0ad4e;text-transform:uppercase;letter-spacing:1px;">' +
            '<i class="fa fa-star"></i> Your Custom Templates</div>'
        );
        customTemplates.forEach(function(template) {
            var item = $('<div class="list-group-item url-template-item" style="cursor:pointer;padding:8px 12px;">' +
                '<div style="display:flex;justify-content:space-between;align-items:center;">' +
                '<div><i class="fa fa-bookmark"></i> <strong>' + escapeHtml(template.name) + '</strong></div>' +
                '<button class="btn btn-xs btn-danger delete-template-btn" style="flex-shrink:0;margin-left:10px;"><i class="fa fa-trash"></i></button>' +
                '</div>' +
                '<small class="text-muted" style="display:block;margin-top:3px;word-break:break-all;">' + escapeHtml(template.url) + '</small>' +
                '</div>');
            item.find('.delete-template-btn').click(function(e) {
                e.stopPropagation();
                deleteURLTemplate(template.id);
            });
            item.click(function() {
                $("#url").val(template.url);
                $("#urlTemplateModal").modal('hide');
                updateURLLengthIndicator();
            });
            templateList.append(item);
        });
    }

    // Preset templates grouped by category
    var grouped = {};
    urlTemplates.filter(function(t) { return t.is_preset; }).forEach(function(t) {
        if (!grouped[t.category]) grouped[t.category] = [];
        grouped[t.category].push(t);
    });
    Object.keys(grouped).sort().forEach(function(category) {
        templateList.append(
            '<div class="url-template-divider" style="padding:4px 2px;margin-top:10px;margin-bottom:2px;font-size:11px;font-weight:bold;color:#888;text-transform:uppercase;letter-spacing:1px;">' +
            '<i class="fa fa-folder"></i> ' + escapeHtml(category) + '</div>'
        );
        grouped[category].forEach(function(template) {
            var item = $('<div class="list-group-item url-template-item" style="cursor:pointer;padding:8px 12px;">' +
                '<div><i class="fa fa-link"></i> <strong>' + escapeHtml(template.name) + '</strong></div>' +
                '<small class="text-muted" style="display:block;margin-top:3px;word-break:break-all;">' + escapeHtml(template.url) + '</small>' +
                '</div>');
            item.click(function() {
                $("#url").val(template.url);
                $("#urlTemplateModal").modal('hide');
                updateURLLengthIndicator();
            });
            templateList.append(item);
        });
    });
}

function deleteURLTemplate(id) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the custom URL template.",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete Template",
        confirmButtonColor: "#d9534f",
        reverseButtons: true
    }).then(function (result) {
        if (result.value) {
            api.urlTemplateId.delete(id)
                .success(function () {
                    loadURLTemplates();
                    Swal.fire("Deleted!", "The template has been deleted.", "success");
                })
                .error(function (data) {
                    Swal.fire("Error!", data.responseJSON.message, "error");
                });
        }
    });
}

function saveCustomURLTemplate() {
    var name = $("#customTemplateName").val().trim();
    var url = $("#customTemplateUrl").val().trim();
    
    if (!name) {
        $("#inlineAddTemplateFlashes").empty().append(
            '<div class="alert alert-danger"><i class="fa fa-exclamation-circle"></i> Template name is required</div>'
        );
        return;
    }

    if (!url) {
        $("#inlineAddTemplateFlashes").empty().append(
            '<div class="alert alert-danger"><i class="fa fa-exclamation-circle"></i> URL is required</div>'
        );
        return;
    }
    
    var template = {
        name: name,
        url: url,
        category: "Custom"
    };
    
    api.urlTemplates.post(template)
        .success(function () {
            $("#customTemplateName").val('');
            $("#customTemplateUrl").val('');
            $("#inlineAddTemplateFlashes").empty();
            $("#inlineAddTemplateForm").slideUp(150);
            $("#addCustomTemplateBtn").show();
            loadURLTemplates();
            successFlashFade("Custom template saved successfully!", 3);
        })
        .error(function (data) {
            $("#inlineAddTemplateFlashes").empty().append(
                '<div class="alert alert-danger"><i class="fa fa-exclamation-circle"></i> ' +
                data.responseJSON.message + '</div>'
            );
        });
}

// Calculate and display URL length with parameter and RID
function updateURLLengthIndicator() {
    var url = $("#url").val();
    var urlParam = $("#urlparam").val() || 'rid';
    
    // RID length is typically 8 characters in Gophish
    var ridLength = 8;
    
    // Calculate total length: URL + separator + param + = + RID
    var separator = url.indexOf('?') !== -1 ? '&' : '?';
    var totalLength = url.length + separator.length + urlParam.length + 1 + ridLength; // +1 for '='
    
    // Standard URL length limit (most browsers support up to 2048)
    var maxLength = 2048;
    var remaining = maxLength - totalLength;
    
    var indicator = $("#urlLengthIndicator");
    
    if (url.length === 0) {
        indicator.html('');
        return;
    }
    
    var color = '';
    var icon = '';
    
    if (remaining < 0) {
        color = '#d9534f'; // red
        icon = '<i class="fa fa-exclamation-triangle"></i> ';
    } else if (remaining < 100) {
        color = '#f0ad4e'; // orange
        icon = '<i class="fa fa-exclamation-circle"></i> ';
    } else {
        color = '#5cb85c'; // green
        icon = '<i class="fa fa-check-circle"></i> ';
    }
    
    var fullUrl = url + separator + urlParam + '=' + '[RID]';
    indicator.html(
        icon + 
        '<span style="color: ' + color + '; font-weight: bold;">' + 
        totalLength + '/' + maxLength + ' chars</span> ' +
        '<span class="text-muted">(with ?' + urlParam + '=[RID])</span>'
    );
}

$(document).ready(function () {
    // URL length indicator updates
    $("#url").on('input', updateURLLengthIndicator);
    $("#urlparam").on('input', updateURLLengthIndicator);
    
    // URL Template button click
    $("#urlTemplateBtn").click(function (e) {
        e.preventDefault();
        loadURLTemplates();
        $("#urlTemplateModal").modal('show');
    });
    
    // Add custom template button — show inline form
    $("#addCustomTemplateBtn").click(function () {
        var currentUrl = $("#url").val();
        if (currentUrl) {
            $("#customTemplateUrl").val(currentUrl);
        }
        $("#inlineAddTemplateForm").slideDown(150);
        $("#addCustomTemplateBtn").hide();
    });

    // Cancel inline add form
    $("#cancelAddTemplateBtn").click(function () {
        $("#inlineAddTemplateForm").slideUp(150);
        $("#addCustomTemplateBtn").show();
        $("#customTemplateName").val('');
        $("#customTemplateUrl").val('');
        $("#inlineAddTemplateFlashes").empty();
    });

    // Save custom template button
    $("#saveCustomTemplateBtn").click(function () {
        saveCustomURLTemplate();
    });

    // Reset inline form when URL template modal closes
    $("#urlTemplateModal").on('hidden.bs.modal', function () {
        $("#customTemplateName").val('');
        $("#customTemplateUrl").val('');
        $("#inlineAddTemplateFlashes").empty();
        $("#inlineAddTemplateForm").hide();
        $("#addCustomTemplateBtn").show();
        $("#urlTemplateSearch").val('');
        $('.url-template-item, .url-template-divider').show();
    });

    // Live search for URL templates
    $(document).on('input', '#urlTemplateSearch', function () {
        var q = $(this).val().toLowerCase();
        if (!q) {
            $('.url-template-item, .url-template-divider').show();
            return;
        }
        $('.url-template-item').each(function () {
            $(this).toggle($(this).text().toLowerCase().indexOf(q) !== -1);
        });
        $('.url-template-divider').each(function () {
            var divider = $(this);
            var next = divider.next();
            var hasVisible = false;
            while (next.length && !next.hasClass('url-template-divider')) {
                if (next.hasClass('url-template-item') && next.is(':visible')) { hasVisible = true; break; }
                next = next.next();
            }
            divider.toggle(hasVisible);
        });
    });
    
    $("#launch_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "defaultDate": moment(),
        "format": "MMMM Do YYYY, h:mm a"
    })
    $("#send_by_date").datetimepicker({
        "widgetPositioning": {
            "vertical": "bottom"
        },
        "showTodayButton": true,
        "useCurrent": false,
        "format": "MMMM Do YYYY, h:mm a"
    })
    
    // Handle campaign type icon button clicks
    $(".campaign-type-btn").click(function() {
        var type = $(this).data("type");
        
        // Update hidden input
        $("#campaign_type").val(type);
        
        // Update button styles
        $(".campaign-type-btn").removeClass("btn-primary active").addClass("btn-default");
        $(this).removeClass("btn-default").addClass("btn-primary active");
        
        // Show/hide appropriate fields
        if (type === "email") {
            $("#email_template_div").show();
            $("#sms_template_div").hide();
            $("#email_profile_div").show();
            $("#sms_profile_div").hide();
            $("#groups_div").show();
            $("#generic_info_div").hide();
            // Clear any previous error messages
            $("#modal\\.flashes").empty();
        } else if (type === "sms") {
            $("#email_template_div").hide();
            $("#sms_template_div").show();
            $("#email_profile_div").hide();
            $("#sms_profile_div").show();
            $("#groups_div").show();
            $("#generic_info_div").hide();
            
            // Clear any previous error messages
            $("#modal\\.flashes").empty();
            
            // Check if SMS templates exist
            api.smsTemplates.get()
                .success(function (templates) {
                    if (templates.length == 0) {
                        modalError("No SMS templates found!");
                    }
                });
            
            // Check if SMS profiles exist
            api.SMS.get()
                .success(function (profiles) {
                    if (profiles.length == 0) {
                        modalError("No SMS sending profiles found!");
                    }
                });
        } else if (type === "generic") {
            // Hide all template and profile fields for generic campaigns
            $("#email_template_div").hide();
            $("#sms_template_div").hide();
            $("#email_profile_div").hide();
            $("#sms_profile_div").hide();
            $("#groups_div").hide();
            $("#generic_info_div").show();
            
            // Clear any previous error messages
            $("#modal\\.flashes").empty();
        }
    });
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    api.campaigns.summary()
        .success(function (data) {
            campaigns = data.campaigns
            $("#loading").hide()
            if (campaigns.length > 0) {
                $("#campaignTable").show()
                $("#campaignTableArchive").show()

                activeCampaignsTable = $("#campaignTable").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                archivedCampaignsTable = $("#campaignTableArchive").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });
                rows = {
                    'active': [],
                    'archived': []
                }
                $.each(campaigns, function (i, campaign) {
                    label = labels[campaign.status] || "label-default";

                    //section for tooltips on the status of a campaign to show some quick stats
                    var launchDate;
                    var quickStats;
                    if (moment(campaign.launch_date).isAfter(moment())) {
                        launchDate = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        if (campaign.type === 'generic') {
                            quickStats = launchDate + "<br><br>" + "Number of links: " + campaign.stats.total
                        } else {
                            quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total
                        }
                    } else {
                        launchDate = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        
                        // Customize stats based on campaign type
                        if (campaign.type === 'generic') {
                            // Generic campaigns: links, clicks, submissions only
                            quickStats = launchDate + 
                                "<br><br>" + "Number of links: " + campaign.stats.total + 
                                "<br><br>" + "Links clicked: " + campaign.stats.clicked + 
                                "<br><br>" + "Submitted Credentials: " + campaign.stats.submitted_data
                        } else if (campaign.type === 'sms') {
                            // SMS campaigns: no opens, no reports
                            quickStats = launchDate + 
                                "<br><br>" + "Number of recipients: " + campaign.stats.total + 
                                "<br><br>" + "SMS sent: " + campaign.stats.sent + 
                                "<br><br>" + "Links clicked: " + campaign.stats.clicked + 
                                "<br><br>" + "Submitted Credentials: " + campaign.stats.submitted_data + 
                                "<br><br>" + "Errors: " + campaign.stats.error
                        } else {
                            // Email campaigns: full stats
                            quickStats = launchDate + 
                                "<br><br>" + "Number of recipients: " + campaign.stats.total + 
                                "<br><br>" + "Emails opened: " + campaign.stats.opened + 
                                "<br><br>" + "Emails clicked: " + campaign.stats.clicked + 
                                "<br><br>" + "Submitted Credentials: " + campaign.stats.submitted_data + 
                                "<br><br>" + "Errors: " + campaign.stats.error + 
                                "<br><br>" + "Reported: " + campaign.stats.email_reported
                        }
                    }

                    // Get campaign type icon
                    var typeIcon = '';
                    var typeTooltip = '';
                    if (campaign.type === 'sms') {
                        typeIcon = '<i class="fa fa-mobile text-info" data-toggle="tooltip" title="SMS Campaign"></i> ';
                        typeTooltip = 'SMS Campaign';
                    } else if (campaign.type === 'generic') {
                        typeIcon = '<i class="fa fa-link text-warning" data-toggle="tooltip" title="Generic Campaign (Landing Page Only)"></i> ';
                        typeTooltip = 'Generic Campaign';
                    } else {
                        typeIcon = '<i class="fa fa-envelope text-primary" data-toggle="tooltip" title="Email Campaign"></i> ';
                        typeTooltip = 'Email Campaign';
                    }

                    // Determine if this is active or archived for the checkbox
                    var tableType = campaign.status == 'Completed' ? 'archived' : 'active';
                    
                    var resendBtn = '';
                    if ((campaign.type === 'email' || campaign.type === 'sms') && campaign.status === 'In progress' && campaign.stats && campaign.stats.error > 0) {
                        var resendTooltip = campaign.type === 'sms' ? 'Resend Failed Messages' : 'Resend Failed Emails';
                        resendBtn = "<button class='btn btn-warning' onclick='resendFailed(" + campaign.id + ")' data-toggle='tooltip' data-placement='left' title='" + resendTooltip + "'><i class='fa fa-repeat'></i></button>";
                    }

                    var row = [
                        "<input type='checkbox' class='campaign-checkbox' data-id='" + campaign.id + "' data-type='" + tableType + "'>",
                        typeIcon + escapeHtml(campaign.name),
                        moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        "<div class='pull-right'>" + resendBtn + "\
                    <a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>\
                    <i class='fa fa-bar-chart'></i>\
                    </a>\
                    <span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Campaign' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]
                    if (campaign.status == 'Completed') {
                        rows['archived'].push(row)
                    } else {
                        rows['active'].push(row)
                    }
                })
                activeCampaignsTable.rows.add(rows['active']).draw()
                archivedCampaignsTable.rows.add(rows['archived']).draw()
                $('[data-toggle="tooltip"]').tooltip()
                
                // Set up checkbox event handlers
                $('#selectAllActive').on('change', function() {
                    currentTab = 'active';
                    handleSelectAll();
                });
                
                $('#selectAllArchived').on('change', function() {
                    currentTab = 'archived';
                    handleSelectAll();
                });
                
                // Delegate checkbox change events
                $(document).on('change', 'input.campaign-checkbox', function() {
                    var campaignId = $(this).data('id');
                    handleCheckboxChange(campaignId);
                });
                
                // Clear selections when switching tabs
                $('a[data-toggle="tab"]').on('shown.bs.tab', function (e) {
                    var target = $(e.target).attr("href");
                    if (target === '#activeCampaigns') {
                        currentTab = 'active';
                    } else if (target === '#archivedCampaigns') {
                        currentTab = 'archived';
                    }
                    // Clear all selections when changing tabs
                    clearSelections();
                });
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching campaigns")
        })
    // Select2 Defaults
    $.fn.select2.defaults.set("width", "100%");
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"));
    $.fn.select2.defaults.set("theme", "bootstrap");
    $.fn.select2.defaults.set("sorter", function (data) {
        return data.sort(function (a, b) {
            if (a.text.toLowerCase() > b.text.toLowerCase()) {
                return 1;
            }
            if (a.text.toLowerCase() < b.text.toLowerCase()) {
                return -1;
            }
            return 0;
        });
    })
})
