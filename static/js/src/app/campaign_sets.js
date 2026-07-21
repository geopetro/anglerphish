// campaign_sets.js

// URL Template functionality
var urlTemplates = [];
var currentUrlFieldTarget = null; // Track which URL field triggered the template modal

// Cache of per-campaign stats from the set summary endpoint, keyed by campaign
// id. Lets showCampaignSummary render target counts without its own API call.
var campaignSetSummaryById = {};

// The most recent set summary, retained so the Totals/Unique toggle can
// re-render without refetching.
var currentSetSummary = null;

// Renders the Overview funnel for one of the two stat modes.
//
// Sent → Opened → Clicked → Submitted is a funnel: each stage is a subset of
// the one before, so a shared track whose full width is the contact count shows
// drop-off directly. Replied and Reported are not funnel stages — Reported is
// the metric where a target caught the phish — so they sit below a rule as
// separate signals.
//
// "totals" sums across campaigns (someone in two campaigns counts twice);
// "unique" dedups by contact. The base (100% of the track) is that mode's total.
function renderOverviewKpis(summary, mode) {
    var stats = mode === "unique" ? summary.unique_stats : summary.stats;
    var base = stats.total || 0;
    var contactWord = mode === "unique" ? "unique contacts" : "targets";

    var stages = [
        { value: stats.sent, label: "Sent", icon: "fa-envelope-o", color: "cs-stat-sent", hex: "#1abc9c" },
        { value: stats.opened, label: "Opened", icon: "fa-envelope-open-o", color: "cs-stat-opened", hex: "#f9bf3b" },
        { value: stats.clicked, label: "Clicked", icon: "fa-mouse-pointer", color: "cs-stat-clicked", hex: "#F39C12" },
        { value: stats.submitted_data, label: "Submitted", icon: "fa-exclamation-circle", color: "cs-stat-submitted", hex: "#f05b4f", outcome: true }
    ];
    var signals = [
        { value: stats.replied, label: "Replied", icon: "fa-reply", color: "cs-stat-replied", hex: "#E67E22" },
        { value: stats.email_reported, label: "Reported", icon: "fa-bullhorn", color: "cs-stat-reported", hex: "#45d6ef", good: true }
    ];

    function row(item) {
        var v = item.value || 0;
        var pct = base > 0 ? (v / base) * 100 : 0;
        var width = Math.max(0, Math.min(100, pct));
        // Zero rows show a hairline tick and an em dash, so an empty set reads as
        // "nothing yet" rather than a broken wall of zeros.
        var fill = v > 0
            ? '<span class="cs-fn-fill" style="width:' + width.toFixed(1) + '%;background:' + item.hex + '"></span>'
            : '<span class="cs-fn-zero"></span>';
        var pctLabel = (base > 0 && v > 0) ? (pct < 1 ? pct.toFixed(1) : Math.round(pct)) + "%" : "—";
        var nCls = "cs-fn-n" + (item.outcome ? " cs-fn-n-outcome" : "") + (item.good ? " cs-fn-n-good" : "");
        return '<div class="cs-fn-row">' +
            '<div class="cs-fn-label"><i class="fa ' + item.icon + ' ' + item.color + '"></i>' + item.label + '</div>' +
            '<div class="cs-fn-track">' + fill + '</div>' +
            '<div class="' + nCls + '">' + v + '</div>' +
            '<div class="cs-fn-pct">' + pctLabel + '</div>' +
            '</div>';
    }

    var html = '<div class="cs-fn-scale"><span>Progression of ' + contactWord + '</span>' +
        '<span>' + base + ' ' + contactWord + '</span></div>';
    html += stages.map(row).join("");
    html += '<hr class="cs-fn-rule">';
    html += '<div class="cs-fn-sig-hd">Signals</div>';
    html += signals.map(row).join("");

    $("#overviewKpis").html(html);
}

// Renders the whole Overview tab from a campaign set summary response.
function renderCampaignSetOverview(summary) {
    currentSetSummary = summary;

    var launched = summary.launch_date && summary.launch_date !== "0001-01-01T00:00:00Z"
        ? moment(summary.launch_date).format("MMM Do YYYY, h:mm a")
        : "Not scheduled";
    var sendBy = summary.send_by_date && summary.send_by_date !== "0001-01-01T00:00:00Z"
        ? " · Send by " + moment(summary.send_by_date).format("MMM Do YYYY, h:mm a")
        : "";

    $("#overviewHeader").html(
        '<span class="cs-overview-title">' + escapeHtml(summary.name) + '</span>' +
        '<span class="label label-default">' + escapeHtml(summary.status || "Unknown") + '</span>' +
        '<div class="cs-overview-meta">' +
        summary.campaign_count + ' campaign' + (summary.campaign_count === 1 ? '' : 's') +
        ' · Launched ' + launched + sendBy +
        '</div>'
    );

    // Reset the toggle to Totals on every open.
    $("#overviewStatsToggle button").removeClass("active btn-primary").addClass("btn-default");
    $('#overviewStatsToggle button[data-mode="totals"]').addClass("active btn-primary").removeClass("btn-default");
    renderOverviewKpis(summary, "totals");

    // Initialize the stats info tooltip. This is static template markup, so there
    // is no page-load init for it; init here (idempotent) when the tab is populated.
    $("#overviewStatsInfo").tooltip();

    var rows = "";
    (summary.campaigns || []).forEach(function (campaign) {
        var stats = campaign.stats || {};
        var isEmail = campaign.type === "email";
        rows += '<tr>' +
            '<td><div class="cs-overview-campaign-name" title="' + escapeHtml(campaign.name) + '">' +
            '<i class="fa ' + (isEmail ? "fa-envelope" : "fa-mobile") + '"></i> ' +
            escapeHtml(campaign.name) + '</div></td>' +
            '<td>' + escapeHtml(campaign.status || "") + '</td>' +
            '<td class="text-right">' + (stats.total || 0) + '</td>' +
            '<td class="text-right">' + (stats.sent || 0) + '</td>' +
            '<td class="text-right">' + (isEmail ? (stats.opened || 0) : "—") + '</td>' +
            '<td class="text-right">' + (stats.clicked || 0) + '</td>' +
            '<td class="text-right">' + (isEmail ? (stats.replied || 0) : "—") + '</td>' +
            '<td class="text-right">' + (stats.submitted_data || 0) + '</td>' +
            '<td class="text-right">' + (isEmail ? (stats.email_reported || 0) : "—") + '</td>' +
            '<td class="text-right"><a class="btn btn-xs btn-primary" href="/campaigns/' + campaign.id + '">' +
            '<i class="fa fa-bar-chart"></i></a></td>' +
            '</tr>';
    });
    $("#overviewBreakdown tbody").html(rows);
}

// Toggle handler — bound once at load, not per modal open, to avoid stacking
// duplicate handlers each time a set is viewed.
$(document).on("click", "#overviewStatsToggle button", function () {
    if (!currentSetSummary) {
        return;
    }
    var mode = $(this).data("mode");
    $("#overviewStatsToggle button").removeClass("active btn-primary").addClass("btn-default");
    $(this).addClass("active btn-primary").removeClass("btn-default");
    renderOverviewKpis(currentSetSummary, mode);
});

// Calculate and display URL length with parameter and RID
function updateURLLengthIndicator(urlFieldId, urlParamFieldId, indicatorId) {
    var url = $(urlFieldId).val();
    var urlParam = $(urlParamFieldId).val() || 'rid';
    
    // RID length is typically 8 characters in Gophish
    var ridLength = 8;
    
    // Calculate total length: URL + separator + param + = + RID
    var separator = url.indexOf('?') !== -1 ? '&' : '?';
    var totalLength = url.length + separator.length + urlParam.length + 1 + ridLength;
    
    // Standard URL length limit
    var maxLength = 2048;
    var remaining = maxLength - totalLength;
    
    var indicator = $(indicatorId);
    
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
    
    indicator.html(
        icon + 
        '<span style="color: ' + color + '; font-weight: bold;">' + 
        totalLength + '/' + maxLength + ' chars</span> ' +
        '<span class="text-muted">(with ?' + urlParam + '=[RID])</span>'
    );
}

// ── Pill toggle sync helpers ──────────────────────────────────────────────

/**
 * Reads the hidden checkbox states and updates the pill toggle visuals
 * and field dimming to match. Call this any time checkboxes are set
 * programmatically (edit / view / copy mode).
 */
function syncPillsFromCheckboxes() {
    var mapping = {
        "useSharedPage":     "sharedPageRow",
        "useSharedURL":      "sharedURLRow",
        "useSharedURLParam": "sharedURLParamRow",
        "useSharedQRSize":   "sharedQRSizeRow",
        "useSharedHTTPAuth": "sharedHTTPAuthRow",
        "useSharedSchedule": "sharedScheduleRow"
    };

    $.each(mapping, function(checkboxId, rowId) {
        var isChecked  = $("#" + checkboxId).is(":checked");
        var $group     = $("#" + rowId + " .shared-toggle-group");
        var $fieldCol  = $("#" + rowId + " .inline-setting-field-col");

        $group.find(".shared-toggle-btn").removeClass("active");

        if (isChecked) {
            $group.find(".shared-toggle-shared").addClass("active");
            $fieldCol.removeClass("field-disabled");
        } else {
            $group.find(".shared-toggle-percampaign").addClass("active");
            $fieldCol.addClass("field-disabled");
        }
    });
}

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

    function makeClickHandler(template) {
        return function () {
            if (currentUrlFieldTarget) {
                $(currentUrlFieldTarget).val(template.url);
                if (currentUrlFieldTarget === '#url') {
                    updateURLLengthIndicator('#url', '#urlparam', '#urlLengthIndicator');
                } else {
                    var idx = $(currentUrlFieldTarget).attr('id').match(/\d+/);
                    if (idx) {
                        var urlParamField = $('#useSharedURLParam').is(':checked') ? '#urlparam' : '#campaign_urlparam_' + idx[0];
                        updateURLLengthIndicator(currentUrlFieldTarget, urlParamField, currentUrlFieldTarget + '_length');
                    }
                }
            }
            $("#urlTemplateModal").modal('hide');
        };
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
            item.click(makeClickHandler(template));
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
            item.click(makeClickHandler(template));
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

$(document).ready(function () {
    // Set Select2 defaults to match the campaigns page
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
    });
    
    // URL Template button click handlers
    $("#urlTemplateBtn").click(function (e) {
        e.preventDefault();
        currentUrlFieldTarget = '#url';
        loadURLTemplates();
        $("#urlTemplateModal").modal('show');
    });
    
    // Add custom template button — show inline form
    $("#addCustomTemplateBtn").click(function () {
        var currentUrl = $(currentUrlFieldTarget).val();
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
    
    // Store original body padding when campaign set modal opens
    $("#modal").on('show.bs.modal', function () {
        if (!$(this).data('originalPadding')) {
            $(this).data('originalPadding', $('body').css('padding-right'));
        }
    });
    
    // Comprehensive fix for nested modal scroll bar issue
    $("#urlTemplateModal, #addUrlTemplateModal").on('hidden.bs.modal', function () {
        // Check if the parent modal is still open
        var $parentModal = $("#modal");
        if ($parentModal.hasClass('in') || $parentModal.hasClass('show')) {
            // Force restore modal-open class
            $('body').addClass('modal-open');
            
            // Force restore padding-right
            var scrollbarWidth = window.innerWidth - $(document).width();
            if (scrollbarWidth > 0) {
                $('body').css('padding-right', scrollbarWidth + 'px');
            }
            
            // Ensure parent modal has correct z-index
            $parentModal.css('overflow-y', 'auto');
        }
    });
    
    // Additional safety net - monitor body classes
    var bodyObserver = new MutationObserver(function(mutations) {
        mutations.forEach(function(mutation) {
            if (mutation.attributeName === 'class') {
                var $modal = $("#modal");
                // If campaign set modal is open but body doesn't have modal-open
                if (($modal.hasClass('in') || $modal.hasClass('show')) && 
                    !$('body').hasClass('modal-open')) {
                    $('body').addClass('modal-open');
                    
                    // Restore padding
                    var scrollbarWidth = window.innerWidth - $(document).width();
                    if (scrollbarWidth > 0) {
                        $('body').css('padding-right', scrollbarWidth + 'px');
                    }
                }
            }
        });
    });
    
    // Start observing body class changes
    bodyObserver.observe(document.body, {
        attributes: true,
        attributeFilter: ['class']
    });
    
    // URL length indicator updates for shared URL
    $("#url").on('input', function() {
        updateURLLengthIndicator('#url', '#urlparam', '#urlLengthIndicator');
    });
    $("#urlparam").on('input', function() {
        updateURLLengthIndicator('#url', '#urlparam', '#urlLengthIndicator');
    });

    // ── Initialize all Select2 dropdowns up front ──
    $("#page, .campaign-page").select2({ placeholder: "Select a Landing Page" });
    $(".email-template").select2({ placeholder: "Select an Email Template" });
    $(".sms-template").select2({ placeholder: "Select an SMS Template" });
    $(".email-profile").select2({ placeholder: "Select a Sending Profile" });
    $(".sms-profile").select2({ placeholder: "Select an SMS Sending Profile" });
    $(".groups").select2({ placeholder: "Select Groups" });

    // Setup the DataTable
    campaignSetTable = $("#campaignSetTable").DataTable({
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }],
        order: [
            [1, "desc"]
        ],
        autoWidth: false
    });

    // Load the campaign sets
    loadCampaignSets();

    // Setup the "New Campaign Set" button
    $("#new-campaign-set-btn").on("click", function () {
        // Clear any previous error messages
        $("#modal\\.flashes").empty();

        // Reset the modal title
        $("#modalLabel").text("New Campaign Set");
        
        // Reset the tabs
        $(".nav-tabs a[href='#generalSettings']").parent().show();
        $(".nav-tabs a[href='#generalSettings']").tab("show"); // Activate general settings tab
        $("#generalSettings").addClass("active");
        $("#campaignsTab").removeClass("active");
        
        // Clear any previous summary views
        $(".campaign-summary").remove();

        // Reset the modal to its initial state
        $("#modal").modal("show");
        $("#name").val("");
        $("#page").select2({
            placeholder: "Select a Landing Page"
        });
        $("#url").val("");
        $("#urlparam").val("");
        $("#qrsize").val("");
        $("#basicauth").prop("checked", false);
        // Set launch date to current day
        $("#launch_date").val(moment().format("MMMM Do YYYY, h:mm a"));
        $("#send_by_date").val("");
        $("#campaignList").html("");
        $("#campaignDetail").html("");
        $(".campaign-detail-placeholder").show();

        // Reset the shared settings checkbox
        $("#useSharedSettings").prop("checked", true);
        $("#sharedSettingsSection").show();

        // Reset the buttons
        $("#saveDraftButton").text("Save as Draft");
        $("#saveDraftButton").off("click").on("click", function () {
            saveDraftCampaignSet();
        });

        $("#launchButton").text("Launch");
        $("#launchButton").off("click").on("click", function () {
            launchCampaignSet();
        });

        // Make sure all form fields are enabled
        $("#modal input, #modal select, #modal textarea").prop("disabled", false);
        $(".btn-remove-campaign, #addCampaignButton").show();
        $("#saveDraftButton").show();
        $("#launchButton").show();

        // Add a new campaign entry
        addCampaignEntry(0);
    });

    // Clear modal flashes when the modal is hidden
    $("#modal").on("hidden.bs.modal", function () {
        $("#modal\\.flashes").empty();
    });

    // Handle the "Save as Draft" button
    $("#saveDraftButton").on("click", function () {
        saveDraftCampaignSet();
    });

    // Setup the "Add Campaign" button
    $("#addCampaignButton").on("click", function () {
        const campaignCount = $(".campaign-list-item").length;
        addCampaignEntry(campaignCount);

        // Select the newly added campaign
        $(`.campaign-list-item[data-index="${campaignCount}"]`).trigger("click");
    });

    // Handle campaign list item click
    $(document).on("click", ".campaign-list-item", function (e) {
        e.preventDefault();

        // Get the campaign index
        const index = $(this).data("index");

        // Update active state in list
        $(".campaign-list-item").removeClass("active");
        $(this).addClass("active");

        // Hide all campaign detail forms
        $(".campaign-detail-form").hide();

        // Show the selected campaign detail form
        $(`#campaign_form_${index}`).show();

        // Show the delete button
        $(".btn-remove-campaign").show();

        // Hide the placeholder
        $(".campaign-detail-placeholder").hide();
        
        // Update campaign settings visibility based on shared settings
        updateCampaignSettingsVisibility();
    });

    // Handle campaign name input to update the list item text
    $(document).on("input", ".campaign-name", function () {
        const campaignForm = $(this).closest(".campaign-detail-form");
        const index = campaignForm.attr("id").replace("campaign_form_", "");
        const name = $(this).val().trim();

        // Update the list item text
        const listItemText = name || `New Campaign ${parseInt(index) + 1}`;
        $(`.campaign-list-item[data-index="${index}"] .campaign-list-item-text`).text(listItemText);
    });

    // Handle campaign type change
    $(document).on("change", ".campaign-type", function () {
        const campaignForm = $(this).closest(".campaign-detail-form");
        const type = $(this).val();
        const index = campaignForm.attr("id").replace("campaign_form_", "");

        // Sync btn-group type buttons (handles programmatic .trigger("change") in edit/copy mode)
        campaignForm.find(".cs-type-btn").removeClass("btn-primary active").addClass("btn-default");
        campaignForm.find(`.cs-type-btn[data-type="${type}"]`)
            .removeClass("btn-default").addClass("btn-primary active");

        // Update the icon in the list
        const icon = type === "email" ? "fa-envelope" : "fa-mobile";
        const $listItem = $(`.campaign-list-item[data-index="${index}"]`);
        $listItem.find(".campaign-list-item-icon i").attr("class", `fa ${icon}`);
        
        // Update the data-type attribute
        $listItem.attr("data-type", type);
        
        // Show/hide appropriate stats based on campaign type
        if (type === "email") {
            $listItem.find(".email-stat").show();
            // Update the sent icon for email
            $listItem.find(".campaign-stat[title='Sent'] i").attr("class", "fa fa-envelope-o");
        } else if (type === "sms") {
            $listItem.find(".email-stat").hide();
            // Update the sent icon for SMS
            $listItem.find(".campaign-stat[title='Sent'] i").attr("class", "fa fa-mobile");
        }

        if (type === "email") {
            campaignForm.find(".email-template-group").show();
            campaignForm.find(".sms-template-group").hide();
            campaignForm.find(".email-profile-group").show();
            campaignForm.find(".sms-profile-group").hide();
        } else if (type === "sms") {
            campaignForm.find(".email-template-group").hide();
            campaignForm.find(".sms-template-group").show();
            campaignForm.find(".email-profile-group").hide();
            campaignForm.find(".sms-profile-group").show();
        }
    });

    // ── "Set All" global buttons ──────────────────────────────────────────
    // Toggles btn-primary/btn-default to show which is active, then sets all checkboxes
    $("#setAllShared").on("click", function () {
        $("#setAllShared").removeClass("btn-default").addClass("btn-primary active");
        $("#setAllPerCampaign").removeClass("btn-primary active").addClass("btn-default");
        $("#useSharedPage, #useSharedURL, #useSharedURLParam, #useSharedQRSize, #useSharedHTTPAuth, #useSharedSchedule")
            .prop("checked", true).trigger("change");
    });

    $("#setAllPerCampaign").on("click", function () {
        $("#setAllPerCampaign").removeClass("btn-default").addClass("btn-primary active");
        $("#setAllShared").removeClass("btn-primary active").addClass("btn-default");
        $("#useSharedPage, #useSharedURL, #useSharedURLParam, #useSharedQRSize, #useSharedHTTPAuth, #useSharedSchedule")
            .prop("checked", false).trigger("change");
    });

    // ── Campaign type btn-group click (scoped to campaign-detail-form) ────
    // Uses .cs-type-btn to avoid conflict with campaigns.js .campaign-type-btn
    $(document).on("click", ".cs-type-btn", function () {
        var $btn  = $(this);
        var type  = $btn.data("type");
        var $form = $btn.closest(".campaign-detail-form");

        // Update btn-group active state (same pattern as campaigns.html)
        $form.find(".cs-type-btn").removeClass("btn-primary active").addClass("btn-default");
        $btn.removeClass("btn-default").addClass("btn-primary active");

        // Drive the hidden select so all existing change-handler logic fires
        $form.find(".campaign-type").val(type).trigger("change");
    });

    // ── Pill toggle click handler ──────────────────────────────────────────
    // Delegates to the hidden checkbox which fires all existing change handlers
    $(document).on("click", ".shared-toggle-btn", function () {
        var $btn   = $(this);
        var $group = $btn.closest(".shared-toggle-group");
        var $row   = $btn.closest(".inline-setting-row");

        // Update visual active state
        $group.find(".shared-toggle-btn").removeClass("active");
        $btn.addClass("active");

        // Dim / undim the field column
        var $fieldCol = $row.find(".inline-setting-field-col");
        if ($btn.data("value") === "shared") {
            $fieldCol.removeClass("field-disabled");
        } else {
            $fieldCol.addClass("field-disabled");
        }

        // Update the hidden checkbox and let existing handlers fire
        var checkboxId = $group.data("target-checkbox");
        var isShared   = $btn.data("value") === "shared";
        $("#" + checkboxId).prop("checked", isShared).trigger("change");
    });

    // Handle the global shared settings checkbox
    $("#useSharedSettings").on("change", function() {
        const isChecked = $(this).is(":checked");
        
        if (isChecked) {
            // Show the shared settings section
            $("#sharedSettingsSection").show();
        } else {
            // Hide the shared settings section
            $("#sharedSettingsSection").hide();
        }
        
        // Update campaign settings visibility
        updateCampaignSettingsVisibility();
    });
    
    // Handle individual shared setting checkboxes
    // NOTE: syncPillsFromCheckboxes is also called here so that programmatic
    // .trigger("change") calls (in edit / copy mode) keep the pills in sync.
    $("#useSharedPage, #useSharedURL, #useSharedURLParam, #useSharedQRSize, #useSharedHTTPAuth, #useSharedSchedule").on("change", function() {
        const settingType = $(this).attr('id').replace('useShared', '');
        const isChecked = $(this).is(":checked");
        
        // Show/hide the corresponding section based on checkbox state
        if (isChecked) {
            $(`#shared${settingType}Section`).show();
        } else {
            $(`#shared${settingType}Section`).hide();
        }
        
        // Sync the pill toggle visuals to match the new checkbox state
        syncPillsFromCheckboxes();
        
        // Update campaign settings visibility
        updateCampaignSettingsVisibility();
    });
    
    // Initialize the granular settings sections
    function initializeGranularSettings() {
        // Check if the global shared settings checkbox is checked
        const useSharedSettings = $("#useSharedSettings").is(":checked");
        
        if (useSharedSettings) {
            // Show the legacy shared settings section
            $("#sharedSettingsSection").show();
        } else {
            // Hide the legacy shared settings section
            $("#sharedSettingsSection").hide();
        }
        
        // Show the granular settings section
        $("#granularSettingsSection").show();
        
        // Initialize each granular setting section based on its checkbox
        $("#useSharedPage, #useSharedURL, #useSharedURLParam, #useSharedQRSize, #useSharedHTTPAuth, #useSharedSchedule").each(function() {
            const settingType = $(this).attr('id').replace('useShared', '');
            const isChecked = $(this).is(":checked");
            
            if (isChecked) {
                $(`#shared${settingType}Section`).show();
            } else {
                $(`#shared${settingType}Section`).hide();
            }
        });
        
        // Update campaign settings visibility
        updateCampaignSettingsVisibility();
    }
    
    // Call the initialization function
    initializeGranularSettings();

    // Handle tab change
    $('a[data-toggle="tab"]').on('shown.bs.tab', function (e) {
        const target = $(e.target).attr("href");
        if (target === "#campaignsTab") {
            // Update campaign settings visibility when switching to campaigns tab
            updateCampaignSettingsVisibility();
        }
    });
    
    // Handle campaign URL template button clicks (delegated event)
    $(document).on("click", ".campaign-url-template-btn", function(e) {
        e.preventDefault();
        const index = $(this).data("campaign-index");
        currentUrlFieldTarget = '#campaign_url_' + index;
        loadURLTemplates();
        $("#urlTemplateModal").modal('show');
    });
    
    // Handle campaign URL input changes for length indicator (delegated event)
    $(document).on("input", ".campaign-url", function() {
        const index = $(this).attr('id').match(/\d+/);
        if (index) {
            // Use shared URL param if enabled, otherwise use campaign-specific
            const urlParamField = $('#useSharedURLParam').is(':checked') ? '#urlparam' : '#campaign_urlparam_' + index[0];
            updateURLLengthIndicator('#campaign_url_' + index[0], urlParamField, '#campaign_url_' + index[0] + '_length');
        }
    });
    
    // Handle campaign URL parameter input changes for length indicator (delegated event)
    $(document).on("input", ".campaign-urlparam", function() {
        const index = $(this).attr('id').match(/\d+/);
        if (index) {
            // Always use campaign-specific since this event means we're NOT using shared param
            updateURLLengthIndicator('#campaign_url_' + index[0], '#campaign_urlparam_' + index[0], '#campaign_url_' + index[0] + '_length');
        }
    });
    
    // Handle shared URL parameter changes - update all campaign URL length indicators
    $("#urlparam").on('input', function() {
        // If shared URL param is enabled, update all campaign URL length indicators
        if ($('#useSharedURLParam').is(':checked')) {
            $('.campaign-url').each(function() {
                const index = $(this).attr('id').match(/\d+/);
                if (index) {
                    updateURLLengthIndicator('#campaign_url_' + index[0], '#urlparam', '#campaign_url_' + index[0] + '_length');
                }
            });
        }
        // Also update the shared URL indicator
        updateURLLengthIndicator('#url', '#urlparam', '#urlLengthIndicator');
    });

    // Function to update campaign settings visibility based on granular checkboxes
    function updateCampaignSettingsVisibility() {
        // For each campaign form, show/hide specific override fields based on granular settings
        $(".campaign-detail-form").each(function() {
            const $form = $(this);
            
            // First check if we should show the overall override section
            if ($("#useSharedSettings").is(":checked")) {
                $form.find(".campaign-settings-override").hide();
                return; // Skip the rest if using all shared settings
            } else {
                $form.find(".campaign-settings-override").show();
            }
            
            // Handle page visibility
            if ($("#useSharedPage").is(":checked")) {
                $form.find(".campaign-page").closest(".form-group").hide();
            } else {
                $form.find(".campaign-page").closest(".form-group").show();
            }
            
            // Handle URL visibility
            if ($("#useSharedURL").is(":checked")) {
                $form.find(".campaign-url").closest(".form-group").hide();
            } else {
                $form.find(".campaign-url").closest(".form-group").show();
            }
            
            // Handle URL parameter visibility
            if ($("#useSharedURLParam").is(":checked")) {
                $form.find(".campaign-urlparam").closest(".form-group").hide();
            } else {
                $form.find(".campaign-urlparam").closest(".form-group").show();
            }
            
            // Handle QR size visibility
            if ($("#useSharedQRSize").is(":checked")) {
                $form.find(".campaign-qrsize").closest(".form-group").hide();
            } else {
                $form.find(".campaign-qrsize").closest(".form-group").show();
            }
            
            // Handle HTTP auth visibility
            if ($("#useSharedHTTPAuth").is(":checked")) {
                $form.find(".campaign-basicauth").closest(".form-group").hide();
            } else {
                $form.find(".campaign-basicauth").closest(".form-group").show();
            }
            
            // Handle schedule visibility
            if ($("#useSharedSchedule").is(":checked")) {
                $form.find(".campaign-launch-date").closest(".form-group").hide();
                $form.find(".campaign-send-by-date").closest(".form-group").hide();
            } else {
                $form.find(".campaign-launch-date").closest(".form-group").show();
                $form.find(".campaign-send-by-date").closest(".form-group").show();
            }
        });
    }

    // Handle remove campaign button
    $(document).on("click", ".btn-remove-campaign", function () {
        // Check if we're in view mode for an existing campaign set
        if ($("#modal").data("viewMode")) {
            // Get the active campaign and campaign set IDs
            const campaignId = $(".campaign-list-item.active").data("campaign-id");
            const campaignSetId = $("#modal").data("campaignSetId");
            const isLastCampaign = $(".campaign-list-item").length === 1;
            
            // Use the deleteCampaignFromSet function
            deleteCampaignFromSet(campaignId, campaignSetId, isLastCampaign);
        } else {
            // We're in edit mode for a new or draft campaign set
            const campaignCount = $(".campaign-list-item").length;
            if (campaignCount > 1) {
                // Get the index of the campaign to remove
                const index = $(".campaign-list-item.active").data("index");

                // Remove the campaign list item and detail form
                $(`.campaign-list-item[data-index="${index}"]`).remove();
                $(`#campaign_form_${index}`).remove();

                // Select the first campaign in the list
                $(".campaign-list-item").first().trigger("click");
            } else {
                modalError("You must have at least one campaign in the set");
            }
        }
    });

    // Setup the date pickers
    $("#launch_date").datetimepicker({
        widgetPositioning: {
            vertical: 'bottom'
        },
        showTodayButton: true,
        defaultDate: moment(),
        format: "MMMM Do YYYY, h:mm a"
    });
    $("#send_by_date").datetimepicker({
        widgetPositioning: {
            vertical: 'bottom'
        },
        showTodayButton: true,
        useCurrent: false,
        format: "MMMM Do YYYY, h:mm a"
    });

    // Handle the launch button
    $("#launchButton").on("click", function () {
        launchCampaignSet();
    });

    // Load the available pages, templates, profiles, and groups
    loadAvailableOptions();
    
    // Initialize the launch draft button handler
    $("#launchDraftButton").on("click", function() {
        launchDraftCampaignSet();
    });
});

// Loads the campaign sets from the API and populates the table
function loadCampaignSets() {
    // First load regular campaign sets
    api.campaignSets.get()
        .success(function (cs) {
            // console.log("Campaign sets API response:", cs);
            campaignSetTable.clear();

            // Now load draft campaign sets
            api.draftCampaignSets.get()
                .success(function (dcs) {
                    // console.log("Draft campaign sets API response:", dcs);

                    // If we have any campaign sets or drafts, show them
                    if (cs.length > 0 || dcs.length > 0) {
                        // Add regular campaign sets
                        $.each(cs, function (i, campaignSet) {
                            // Check if the campaign set is already completed or if all campaigns are completed
                            let isCompleted = campaignSet.status === "Completed";
                            let allCampaignsCompleted = true;

                            // Check if all campaigns are completed
                            if (campaignSet.campaigns && campaignSet.campaigns.length > 0) {
                                $.each(campaignSet.campaigns, function (j, campaign) {
                                    if (campaign.status !== "Completed") {
                                        allCampaignsCompleted = false;
                                        return false; // Break the loop
                                    }
                                });
                            } else {
                                // If there are no campaigns, consider it not completed
                                allCampaignsCompleted = false;
                            }

                            // Disable the complete button if the campaign set is already completed or all campaigns are completed
                            let completeButtonDisabled = (isCompleted || allCampaignsCompleted) ? "disabled" : "";

                            // Add the row to the table
                            const rowNode = campaignSetTable.row.add([
                                escapeHtml(campaignSet.name),
                                moment(campaignSet.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                                campaignSet.launch_date ? moment(campaignSet.launch_date).format('MMMM Do YYYY, h:mm:ss a') : "Not scheduled",
                                campaignSet.send_by_date ? moment(campaignSet.send_by_date).format('MMMM Do YYYY, h:mm:ss a') : "Not set",
                                campaignSet.status,
                                campaignSet.campaigns ? campaignSet.campaigns.length : 0,
                                "<div class='pull-right'><button class='btn btn-info' onclick='viewCampaignSet(" + campaignSet.id + ")'><i class='fa fa-eye'></i></button>&nbsp;<button class='btn btn-warning' onclick='copyCampaignSet(" + campaignSet.id + ")'><i class='fa fa-copy'></i></button>&nbsp;<button class='btn btn-success' onclick='completeCampaignSet(" + campaignSet.id + ")' " + completeButtonDisabled + "><i class='fa fa-check'></i></button>&nbsp;<button class='btn btn-danger' onclick='deleteCampaignSet(" + campaignSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
                            ]).draw().node();

                            // Store the row index for later reference
                            $(rowNode).attr('data-campaign-set-id', campaignSet.id);

                            // If all campaigns are completed but the campaign set is not marked as completed,
                            // automatically mark the campaign set as completed
                            if (allCampaignsCompleted && !isCompleted) {
                                // Call the API to mark the campaign set as completed
                                api.campaignSetId.complete(campaignSet.id)
                                    .success(function () {
                                        // console.log(`Campaign set ${campaignSet.id} automatically marked as completed because all campaigns are completed`);
                                        // Update the status for the UI
                                        isCompleted = true;
                                        campaignSet.status = "Completed";

                                        // Update the row in the table to show "Completed" status
                                        const rowIndex = $(`tr[data-campaign-set-id="${campaignSet.id}"]`).index();
                                        if (rowIndex !== -1) {
                                            campaignSetTable.cell(rowIndex, 4).data("Completed").draw(false);
                                            // Also disable the complete button
                                            const actionsCell = campaignSetTable.cell(rowIndex, 6).node();
                                            $(actionsCell).find('button.btn-success').prop('disabled', true);
                                        }

                                        // Force a refresh of the campaign sets to ensure the status is updated
                                        setTimeout(function () {
                                            loadCampaignSets();
                                        }, 1000);
                                    })
                                    .error(function (data) {
                                        // console.error(`Error automatically completing campaign set ${campaignSet.id}:`, data.responseJSON);
                                    });
                            }
                        });

                        // Add draft campaign sets
                        $.each(dcs, function (i, draftSet) {
                            campaignSetTable.row.add([
                                escapeHtml(draftSet.name) + " <span class='label label-default'>DRAFT</span>",
                                moment(draftSet.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                                draftSet.launch_date ? moment(draftSet.launch_date).format('MMMM Do YYYY, h:mm:ss a') : "Not scheduled",
                                draftSet.send_by_date ? moment(draftSet.send_by_date).format('MMMM Do YYYY, h:mm:ss a') : "Not set",
                                "Draft",
                                draftSet.campaigns ? draftSet.campaigns.length : 0,
                                "<div class='pull-right'><button class='btn btn-success' data-toggle='modal' data-target='#launchDraftModal' onclick='prepareLaunchDraft(" + draftSet.id + ")'><i class='fa fa-rocket'></i></button>&nbsp;<button class='btn btn-warning' onclick='copyDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-copy'></i></button>&nbsp;<button class='btn btn-primary' onclick='editDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-pencil'></i></button>&nbsp;<button class='btn btn-danger' onclick='deleteDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
                            ]).draw();
                        });
                    } else {
                        // Add a message to the table when there are no campaign sets
                        campaignSetTable.row.add([
                            "No campaign sets created yet",
                            "",
                            "",
                            "",
                            "",
                            "",
                            ""
                        ]).draw();
                    }

                    $("#campaignSetTable").show();
                })
                .error(function () {
                    errorFlash("Error fetching draft campaign sets");

                    // Still show regular campaign sets if we have them
                    if (cs.length > 0) {
                        $.each(cs, function (i, campaignSet) {
                            // Check if the campaign set is already completed or if all campaigns are completed
                            let isCompleted = campaignSet.status === "Completed";
                            let allCampaignsCompleted = true;

                            // Check if all campaigns are completed
                            if (campaignSet.campaigns && campaignSet.campaigns.length > 0) {
                                $.each(campaignSet.campaigns, function (j, campaign) {
                                    if (campaign.status !== "Completed") {
                                        allCampaignsCompleted = false;
                                        return false; // Break the loop
                                    }
                                });
                            } else {
                                // If there are no campaigns, consider it not completed
                                allCampaignsCompleted = false;
                            }

                            // Disable the complete button if the campaign set is already completed or all campaigns are completed
                            let completeButtonDisabled = (isCompleted || allCampaignsCompleted) ? "disabled" : "";

                            // Add the row to the table
                            const rowNode = campaignSetTable.row.add([
                                escapeHtml(campaignSet.name),
                                moment(campaignSet.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                                campaignSet.launch_date ? moment(campaignSet.launch_date).format('MMMM Do YYYY, h:mm:ss a') : "Not scheduled",
                                campaignSet.send_by_date ? moment(campaignSet.send_by_date).format('MMMM Do YYYY, h:mm:ss a') : "Not set",
                                campaignSet.status,
                                campaignSet.campaigns ? campaignSet.campaigns.length : 0,
                                "<div class='pull-right'><button class='btn btn-info' onclick='viewCampaignSet(" + campaignSet.id + ")'><i class='fa fa-eye'></i></button>&nbsp;<button class='btn btn-success' onclick='completeCampaignSet(" + campaignSet.id + ")' " + completeButtonDisabled + "><i class='fa fa-check'></i></button>&nbsp;<button class='btn btn-danger' onclick='deleteCampaignSet(" + campaignSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
                            ]).draw().node();

                            // Store the row index for later reference
                            $(rowNode).attr('data-campaign-set-id', campaignSet.id);

                            // If all campaigns are completed but the campaign set is not marked as completed,
                            // automatically mark the campaign set as completed
                            if (allCampaignsCompleted && !isCompleted) {
                                // Call the API to mark the campaign set as completed
                                api.campaignSetId.complete(campaignSet.id)
                                    .success(function () {
                                        console.log(`Campaign set ${campaignSet.id} automatically marked as completed because all campaigns are completed`);
                                        // Update the status for the UI
                                        isCompleted = true;
                                        campaignSet.status = "Completed";

                                        // Update the row in the table to show "Completed" status
                                        const rowIndex = $(`tr[data-campaign-set-id="${campaignSet.id}"]`).index();
                                        if (rowIndex !== -1) {
                                            campaignSetTable.cell(rowIndex, 4).data("Completed").draw(false);
                                            // Also disable the complete button
                                            const actionsCell = campaignSetTable.cell(rowIndex, 6).node();
                                            $(actionsCell).find('button.btn-success').prop('disabled', true);
                                        }

                                        // Force a refresh of the campaign sets to ensure the status is updated
                                        setTimeout(function () {
                                            loadCampaignSets();
                                        }, 1000);
                                    })
                                    .error(function (data) {
                                        console.error(`Error automatically completing campaign set ${campaignSet.id}:`, data.responseJSON);
                                    });
                            }
                        });
                    } else {
                        // Add a message to the table when there are no campaign sets
                        campaignSetTable.row.add([
                            "No campaign sets created yet",
                            "",
                            "",
                            "",
                            "",
                            "",
                            ""
                        ]).draw();
                    }

                    $("#campaignSetTable").show();
                });
        })
        .error(function () {
            errorFlash("Error fetching campaign sets");
            campaignSetTable.clear();

            // Try to load just draft campaign sets
            api.draftCampaignSets.get()
                .success(function (dcs) {
                    console.log("Draft campaign sets API response:", dcs);

                    if (dcs.length > 0) {
                        // Add draft campaign sets
                        $.each(dcs, function (i, draftSet) {
                            campaignSetTable.row.add([
                                escapeHtml(draftSet.name) + " <span class='label label-default'>DRAFT</span>",
                                moment(draftSet.created_date).format('MMMM Do YYYY, h:mm:ss a'),
                                draftSet.launch_date ? moment(draftSet.launch_date).format('MMMM Do YYYY, h:mm:ss a') : "Not scheduled",
                                draftSet.send_by_date ? moment(draftSet.send_by_date).format('MMMM Do YYYY, h:mm:ss a') : "Not set",
                                "Draft",
                                draftSet.campaigns ? draftSet.campaigns.length : 0,
                                "<div class='pull-right'><button class='btn btn-success' data-toggle='modal' data-target='#launchDraftModal' onclick='prepareLaunchDraft(" + draftSet.id + ")'><i class='fa fa-rocket'></i></button>&nbsp;<button class='btn btn-primary' onclick='editDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-pencil'></i></button>&nbsp;<button class='btn btn-danger' onclick='deleteDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
                            ]).draw();
                        });
                    } else {
                        // Add a message to the table when there are no campaign sets
                        campaignSetTable.row.add([
                            "No campaign sets created yet",
                            "",
                            "",
                            "",
                            "",
                            "",
                            ""
                        ]).draw();
                    }

                    $("#campaignSetTable").show();
                })
                .error(function () {
                    errorFlash("Error fetching draft campaign sets");

                    // Add a message to the table when there are no campaign sets
                    campaignSetTable.row.add([
                        "No campaign sets created yet",
                        "",
                        "",
                        "",
                        "",
                        "",
                        ""
                    ]).draw();

                    $("#campaignSetTable").show();
                });
        });
}

// Adds a new campaign entry to the campaign set form
function addCampaignEntry(index) {
    // Add the campaign to the list
    const campaignListItem = `
        <a href="#" class="list-group-item campaign-list-item" data-index="${index}" data-type="email">
            <span class="campaign-list-item-icon"><i class="fa fa-envelope"></i></span>
            <span class="campaign-list-item-text">New Campaign ${index + 1}</span>
            <div class="campaign-stats-container" style="display: none;">
                <span class="campaign-stat campaign-stat-sent email-stat" title="Sent"><i class="fa fa-envelope-o"></i> <span class="stat-sent">0</span></span>
                <span class="campaign-stat campaign-stat-opened email-stat" title="Opened"><i class="fa fa-envelope-open-o"></i> <span class="stat-opened">0</span></span>
                <span class="campaign-stat campaign-stat-clicked" title="Clicked"><i class="fa fa-mouse-pointer"></i> <span class="stat-clicked">0</span></span>
                <span class="campaign-stat campaign-stat-replied email-stat" title="Replied"><i class="fa fa-reply"></i> <span class="stat-replied">0</span></span>
                <span class="campaign-stat campaign-stat-submitted" title="Submitted Data"><i class="fa fa-exclamation-circle"></i> <span class="stat-submitted">0</span></span>
                <span class="campaign-stat campaign-stat-reported email-stat" title="Reported"><i class="fa fa-bullhorn"></i> <span class="stat-reported">0</span></span>
            </div>
        </a>
    `;
    $("#campaignList").append(campaignListItem);

    // Create the campaign detail form (it will be shown when the campaign is selected)
    const campaignDetailForm = `
        <div class="campaign-detail-form" id="campaign_form_${index}" style="display: none;">
            <div class="form-group">
                <label class="control-label" for="campaign_name_${index}">Name:</label>
                <input type="text" class="form-control campaign-name" placeholder="Campaign name" id="campaign_name_${index}" />
            </div>
            
            <!-- Type: hidden select (drives all logic) + btn-group identical to campaigns.html -->
            <select class="campaign-type" id="campaign_type_${index}" style="display:none;">
                <option value="email">Email</option>
                <option value="sms">SMS</option>
            </select>
            <div class="form-group">
                <label class="control-label">Type:</label>
                <div>
                    <div class="btn-group" role="group">
                        <button type="button" class="btn btn-sm btn-primary cs-type-btn active" data-type="email">
                            <i class="fa fa-envelope"></i> Email
                        </button>
                        <button type="button" class="btn btn-sm btn-default cs-type-btn" data-type="sms">
                            <i class="fa fa-mobile"></i> SMS
                        </button>
                    </div>
                </div>
            </div>
            
            <div class="form-group email-template-group">
                <label class="control-label" for="template_${index}">Email Template:</label>
                <select class="form-control email-template" placeholder="Template" id="template_${index}">
                </select>
            </div>
            
            <div class="form-group sms-template-group" style="display:none;">
                <label class="control-label" for="sms_template_${index}">SMS Template:</label>
                <select class="form-control sms-template" placeholder="SMS Template" id="sms_template_${index}">
                </select>
            </div>
            
            <div class="form-group email-profile-group">
                <label class="control-label" for="profile_${index}">Sending Profile:</label>
                <select class="form-control email-profile" placeholder="Sending Profile" id="profile_${index}">
                </select>
            </div>
            
            <div class="form-group sms-profile-group" style="display:none;">
                <label class="control-label" for="sms_profile_${index}">SMS Sending Profile:</label>
                <select class="form-control sms-profile" placeholder="SMS Sending Profile" id="sms_profile_${index}">
                </select>
            </div>
            
            <div class="form-group">
                <label class="control-label" for="users_${index}">Groups:</label>
                <select class="form-control groups" id="users_${index}" multiple="multiple">
                </select>
            </div>
            
            <div class="campaign-settings-override" style="display:none;">
                <div class="form-group">
                    <label class="control-label" for="campaign_page_${index}">Landing Page:</label>
                    <select class="form-control campaign-page" placeholder="Landing Page" id="campaign_page_${index}">
                    </select>
                </div>
                <div class="form-group">
                    <label class="control-label" for="campaign_url_${index}">URL:
                        <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Location of Gophish listener (must be reachable by targets!)"></i>
                        <span id="campaign_url_${index}_length" style="margin-left: 10px; font-size: 12px;"></span>
                    </label>
                    <div class="input-group">
                        <input type="text" class="form-control campaign-url" placeholder="http://192.168.1.1" id="campaign_url_${index}" />
                        <span class="input-group-btn">
                            <button type="button" class="btn btn-default campaign-url-template-btn" data-campaign-index="${index}" data-toggle="tooltip" title="Use URL Template">
                                <i class="fa fa-list"></i>
                            </button>
                        </span>
                    </div>
                </div>
                <div class="form-group">
                    <label class="control-label" for="campaign_urlparam_${index}">URL Parameter:
                        <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Sets the parameter used in the URL to track the target. It is recommended to change this parameter, to make Gophish stealthier. Leave blank to use default (rid)."></i>
                    </label>
                    <input type="text" class="form-control campaign-urlparam" placeholder="rid" id="campaign_urlparam_${index}" />
                </div>
                <div class="form-group">
                    <label class="control-label" for="campaign_qrsize_${index}">QR Code Size:
                        <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Size of QR code images (integer is height & width). Leave blank to not include QR code images and use normal links."></i>
                    </label>
                    <input type="number" class="form-control campaign-qrsize" placeholder="256" id="campaign_qrsize_${index}" />
                </div>
                <div class="form-group">
                    <input class="form-check-input campaign-basicauth" type="checkbox" id="campaign_basicauth_${index}">
                    <label class="form-check-label" for="campaign_basicauth_${index}"> Use HTTP Basic Access Authentication
                        <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Enables a landing page with HTTP Authentication"></i>
                    </label>
                </div>
                <div class="form-group">
                    <label class="control-label" for="campaign_launch_date_${index}">Launch Date:</label>
                    <input type="text" class="form-control campaign-launch-date" id="campaign_launch_date_${index}" />
                </div>
                <div class="form-group">
                    <label class="control-label" for="campaign_send_by_date_${index}">Send Emails By (Optional):
                        <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="If specified, Gophish will send messages evenly between the campaign launch and this date."></i>
                    </label>
                    <input type="text" class="form-control campaign-send-by-date" id="campaign_send_by_date_${index}" />
                </div>
            </div>
        </div>
    `;

    $("#campaignDetail").append(campaignDetailForm);

    // Initialize the select2 elements with proper inheritance of global defaults
    $(`#template_${index}`).select2({
        placeholder: "Select an Email Template",
        dropdownParent: $("#modal_body")
    });
    $(`#sms_template_${index}`).select2({
        placeholder: "Select an SMS Template",
        dropdownParent: $("#modal_body")
    });
    $(`#profile_${index}`).select2({
        placeholder: "Select a Sending Profile",
        dropdownParent: $("#modal_body")
    });
    $(`#sms_profile_${index}`).select2({
        placeholder: "Select an SMS Sending Profile",
        dropdownParent: $("#modal_body")
    });
    $(`#users_${index}`).select2({
        placeholder: "Select Groups",
        dropdownParent: $("#modal_body")
    });
    $(`#campaign_page_${index}`).select2({
        placeholder: "Select a Landing Page",
        dropdownParent: $("#modal_body")
    });

    // Initialize the date pickers
    $(`#campaign_launch_date_${index}`).datetimepicker({
        widgetPositioning: {
            vertical: 'bottom'
        },
        showTodayButton: true,
        defaultDate: moment(),
        format: "MMMM Do YYYY, h:mm a"
    });
    $(`#campaign_send_by_date_${index}`).datetimepicker({
        widgetPositioning: {
            vertical: 'bottom'
        },
        showTodayButton: true,
        useCurrent: false,
        format: "MMMM Do YYYY, h:mm a"
    });

    // Load options for just this campaign entry
    return loadOptionsForCampaign(index);

    // Initialize tooltips for the new campaign entry
    $(`#campaign_form_${index} [data-toggle="tooltip"]`).tooltip();

    // Update campaign settings visibility based on current shared settings
    updateCampaignSettingsVisibility();
}

// Shared data-fetch Promise — all campaigns reuse the same 6 API calls
var campaignOptionDataPromise = null;

// Loads options for a specific campaign entry (API calls fire only once; data is shared)
function loadOptionsForCampaign(index) {
    if (!campaignOptionDataPromise) {
        campaignOptionDataPromise = new Promise((resolve) => {
            let completed = 0;
            const data = {};
            const total = 6;
            const done = () => { if (++completed === total) resolve(data); };

            api.pages.get().success(pages => { data.pages = pages; done(); });
            api.templates.get().success(templates => { data.templates = templates; done(); });
            api.smsTemplates.get().success(templates => { data.smsTemplates = templates; done(); });
            api.SMTP.get().success(profiles => { data.smtp = profiles; done(); });
            api.SMS.get().success(profiles => { data.sms = profiles; done(); });
            api.groups.get().success(groups => { data.groups = groups.filter(g => !g.locked); done(); });
        });
    }

    return campaignOptionDataPromise.then(data => {
        const $page = $(`#campaign_page_${index}`);
        $page.empty().append("<option></option>");
        data.pages.forEach(p => $page.append(`<option value="${p.id}">${p.name}</option>`));

        const $tmpl = $(`#template_${index}`);
        $tmpl.empty().append("<option></option>");
        data.templates.forEach(t => $tmpl.append(`<option value="${t.id}">${t.name}</option>`));

        const $smsTmpl = $(`#sms_template_${index}`);
        $smsTmpl.empty().append("<option></option>");
        data.smsTemplates.forEach(t => $smsTmpl.append(`<option value="${t.id}">${t.name}</option>`));

        const $profile = $(`#profile_${index}`);
        $profile.empty().append("<option></option>");
        data.smtp.forEach(p => $profile.append(`<option value="${p.id}">${p.name}</option>`));

        const $smsProfile = $(`#sms_profile_${index}`);
        $smsProfile.empty().append("<option></option>");
        data.sms.forEach(p => $smsProfile.append(`<option value="${p.id}">${p.name}</option>`));

        const $group = $(`#users_${index}`);
        $group.empty();
        data.groups.forEach(g => $group.append(`<option value="${g.id}">${g.name}</option>`));
    });
}


// Helper function to safely set select2 values
function safelySetSelect2Value(selector, value, triggerChange = true) {
    if (!value) return;

    console.log(`Setting ${selector} to value:`, value);

    const $element = $(selector);
    if ($element.length === 0) {
        console.error(`Element not found: ${selector}`);
        return;
    }

    // Use Select2's native method for setting values
    if ($element.data('select2')) {
        $element.val(value).trigger('change.select2');
        console.log(`Set ${selector} using select2 method`);
    } else {
        // Fallback to standard jQuery method
        $element.val(value);
        if (triggerChange) {
            $element.trigger("change");
        }
    }
}

// Helper function to find option by name and set value
function setSelectByOptionText(selector, optionText) {
    if (!optionText) return;

    console.log(`Setting ${selector} by option text:`, optionText);

    const $element = $(selector);
    if ($element.length === 0) {
        console.error(`Element not found: ${selector}`);
        return;
    }

    const desired = optionText.trim().toLowerCase();
    const option = $element.find('option').filter(function () {
        return $(this).text().trim().toLowerCase() === desired;
    });

    if (option.length > 0) {
        // Use Select2's native method for setting values
        if ($element.data('select2')) {
            $element.val(option.val()).trigger('change.select2');
        } else {
            $element.val(option.val()).trigger("change");
        }
        console.log(`Found and set option with value: ${option.val()}`);
    } else {
        console.warn(`Option with text "${optionText}" not found in ${selector}`);
        console.log("Available options:", $element.find('option').map(function () {
            return { text: $(this).text(), value: $(this).val() };
        }).get());
    }
}

// Cached promise so the 6 API calls only fire once per page load
var availableOptionsCache = null;

// Loads the available pages, templates, profiles, and groups
function loadAvailableOptions() {
    if (availableOptionsCache) return availableOptionsCache;
    availableOptionsCache = new Promise((resolve) => {
        let completed = 0;
        const total = 6;
        const done = () => {
            completed++;
            if (completed === total) resolve();
        };

        // Load the landing pages
        api.pages.get()
            .success(function (pages) {
                $("#page").empty();
                $(".campaign-page").empty();
                $("#page").append("<option></option>");
                $(".campaign-page").append("<option></option>");
                $.each(pages, function (i, page) {
                    $("#page").append(`<option value="${page.id}">${page.name}</option>`);
                    $(".campaign-page").append(`<option value="${page.id}">${page.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading pages");
                done();
            });

        // Load the email templates
        api.templates.get()
            .success(function (templates) {
                $(".email-template").empty();
                $(".email-template").append("<option></option>");
                $.each(templates, function (i, template) {
                    $(".email-template").append(`<option value="${template.id}">${template.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading email templates");
                done();
            });

        // Load the SMS templates
        api.smsTemplates.get()
            .success(function (templates) {
                $(".sms-template").empty();
                $(".sms-template").append("<option></option>");
                $.each(templates, function (i, template) {
                    $(".sms-template").append(`<option value="${template.id}">${template.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading SMS templates");
                done();
            });

        // Load the email sending profiles
        api.SMTP.get()
            .success(function (profiles) {
                $(".email-profile").empty();
                $(".email-profile").append("<option></option>");
                $.each(profiles, function (i, profile) {
                    $(".email-profile").append(`<option value="${profile.id}">${profile.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading email profiles");
                done();
            });

        // Load the SMS sending profiles
        api.SMS.get()
            .success(function (profiles) {
                $(".sms-profile").empty();
                $(".sms-profile").append("<option></option>");
                $.each(profiles, function (i, profile) {
                    $(".sms-profile").append(`<option value="${profile.id}">${profile.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading SMS profiles");
                done();
            });

        // Load the groups
        api.groups.get()
            .success(function (groups) {
                $(".groups").empty();
                $.each(groups.filter(g => !g.locked), function (i, group) {
                    $(".groups").append(`<option value="${group.id}">${group.name}</option>`);
                });
                done();
            })
            .error(function () {
                console.error("Error loading groups");
                done();
            });
    });
    return availableOptionsCache;
}

// Launches a new campaign set
function launchCampaignSet() {
    const name = $("#name").val();
    const useSharedSettings = $("#useSharedSettings").is(":checked");
    const useSharedPage = useSharedSettings || $("#useSharedPage").is(":checked");
    const useSharedURL = useSharedSettings || $("#useSharedURL").is(":checked");
    const useSharedURLParam = useSharedSettings || $("#useSharedURLParam").is(":checked");
    const useSharedQRSize = useSharedSettings || $("#useSharedQRSize").is(":checked");
    const useSharedHTTPAuth = useSharedSettings || $("#useSharedHTTPAuth").is(":checked");
    const useSharedSchedule = useSharedSettings || $("#useSharedSchedule").is(":checked");
    
    let page_id = null;
    let url = "";
    let launch_date = "";
    let send_by_date = "";
    let urlparam = "";
    let qrsize = "";
    let basicauth = false;
    
    // Get shared settings values if any are enabled
    if (useSharedSettings || useSharedPage || useSharedURL || useSharedURLParam || 
        useSharedQRSize || useSharedHTTPAuth || useSharedSchedule) {
        
        if (useSharedPage) {
            page_id = $("#page").val();
        }
        
        if (useSharedURL) {
            url = $("#url").val();
        }
        
        if (useSharedURLParam) {
            urlparam = $("#urlparam").val();
        }
        
        if (useSharedQRSize) {
            qrsize = $("#qrsize").val();
        }
        
        if (useSharedHTTPAuth) {
            basicauth = $("#basicauth").is(":checked");
        }
        
        if (useSharedSchedule) {
            launch_date = $("#launch_date").val();
            send_by_date = $("#send_by_date").val();
        }
    }

    // Validate the campaign set
    if (name === "") {
        modalError("Campaign set name cannot be empty");
        return;
    }

    if (useSharedSettings) {
        if (page_id === "") {
            modalError("Please select a landing page");
            return;
        }
        if (url === "") {
            modalError("URL cannot be empty");
            return;
        }
    }

    // Build the campaigns array
    const campaigns = [];
    $(".campaign-detail-form").each(function (i) {
        const campaign = {};
        campaign.name = $(this).find(".campaign-name").val();
        campaign.type = $(this).find(".campaign-type").val();

        if (campaign.name === "") {
            modalError(`Campaign ${i + 1} name cannot be empty`);
            return false;
        }

        if (campaign.type === "email") {
            const templateSelect = $(this).find(".email-template");
            const smtpSelect = $(this).find(".email-profile");
            const templateId = templateSelect.val();
            const smtpId = smtpSelect.val();

            if (templateId === "") {
                modalError(`Please select an email template for campaign ${i + 1}`);
                return false;
            }
            if (smtpId === "") {
                modalError(`Please select a sending profile for campaign ${i + 1}`);
                return false;
            }

            // Create template and smtp objects with names
            campaign.template = {
                name: templateSelect.find(`option[value="${templateId}"]`).text()
            };
            campaign.smtp = {
                name: smtpSelect.find(`option[value="${smtpId}"]`).text()
            };
        } else if (campaign.type === "sms") {
            const smsTemplateSelect = $(this).find(".sms-template");
            const smsProfileSelect = $(this).find(".sms-profile");
            const smsTemplateId = smsTemplateSelect.val();
            const smsProfileId = smsProfileSelect.val();

            if (smsTemplateId === "") {
                modalError(`Please select an SMS template for campaign ${i + 1}`);
                return false;
            }
            if (smsProfileId === "") {
                modalError(`Please select an SMS sending profile for campaign ${i + 1}`);
                return false;
            }

            // Create sms_template and sms objects with names
            campaign.sms_template = {
                name: smsTemplateSelect.find(`option[value="${smsTemplateId}"]`).text()
            };
            campaign.sms = {
                name: smsProfileSelect.find(`option[value="${smsProfileId}"]`).text()
            };
        }

        // Get the groups as objects with names
        const groupSelect = $(this).find(".groups");
        const groupIds = groupSelect.val();

        if (!groupIds || groupIds.length === 0) {
            modalError(`Please select at least one group for campaign ${i + 1}`);
            return false;
        }

        // Convert group IDs to group objects with names
        campaign.groups = [];
        groupIds.forEach(function (id) {
            const groupOption = groupSelect.find(`option[value="${id}"]`);
            campaign.groups.push({
                name: groupOption.text()
            });
        });

        // Apply shared settings based on individual checkboxes
        let needsPageId = true;
        let needsURL = true;
        
        // Handle shared page
        if (useSharedSettings || useSharedPage) {
            if (page_id === "") {
                modalError("Please select a shared landing page");
                return false;
            }
            
            // Create page object with name from shared settings
            const pageSelect = $("#page");
            campaign.page = {
                name: pageSelect.find(`option[value="${page_id}"]`).text()
            };
            needsPageId = false;
        }
        
        // Handle shared URL
        if (useSharedSettings || useSharedURL) {
            if (url === "") {
                modalError("Shared URL cannot be empty");
                return false;
            }
            campaign.url = url;
            needsURL = false;
        }
        
        // Handle shared URL parameter
        if (useSharedSettings || useSharedURLParam) {
            // If URL parameter is empty, it will default to "rid" on the server side
            // But we'll set it explicitly here for clarity
            campaign.urlparam = urlparam || "rid";
        }
        
        // Handle shared QR size
        if (useSharedSettings || useSharedQRSize) {
            campaign.qrsize = qrsize || null;
        }
        
        // Handle shared HTTP auth
        if (useSharedSettings || useSharedHTTPAuth) {
            campaign.basicauth = basicauth;
        }
        
        // Handle shared schedule
        if (useSharedSettings || useSharedSchedule) {
            campaign.launch_date = launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : "";
            campaign.send_by_date = send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null;
        }
        
        // Handle campaign-specific settings (only if not using shared settings for that type)
        if (!useSharedSettings) {
            // Handle campaign-specific page if not using shared page
            if (!useSharedPage) {
                const pageSelect = $(this).find(".campaign-page");
                const pageId = pageSelect.val();
                
                if (pageId === "") {
                    modalError(`Please select a landing page for campaign ${i + 1}`);
                    return false;
                }
                
                // Create page object with name
                campaign.page = {
                    name: pageSelect.find(`option[value="${pageId}"]`).text()
                };
            }
            
            // Handle campaign-specific URL if not using shared URL
            if (!useSharedURL) {
                campaign.url = $(this).find(".campaign-url").val();
                
                if (campaign.url === "") {
                    modalError(`URL cannot be empty for campaign ${i + 1}`);
                    return false;
                }
            }
            
            // Handle campaign-specific URL parameter if not using shared URL parameter
            if (!useSharedURLParam) {
                const urlParamValue = $(this).find(".campaign-urlparam").val();
                campaign.urlparam = urlParamValue ? urlParamValue : "rid"; // Always set a default of "rid" if empty
            }
            
            // Handle campaign-specific QR size if not using shared QR size
            if (!useSharedQRSize) {
                campaign.qrsize = $(this).find(".campaign-qrsize").val() || null;
            }
            
            // Handle campaign-specific HTTP auth if not using shared HTTP auth
            if (!useSharedHTTPAuth) {
                campaign.basicauth = $(this).find(".campaign-basicauth").is(":checked");
            }
            
            // Handle campaign-specific schedule if not using shared schedule
            if (!useSharedSchedule) {
                const campaignLaunchDate = $(this).find(".campaign-launch-date").val();
                const campaignSendByDate = $(this).find(".campaign-send-by-date").val();
                campaign.launch_date = campaignLaunchDate ? moment(campaignLaunchDate, "MMMM Do YYYY, h:mm a").utc().format() : "";
                campaign.send_by_date = campaignSendByDate ? moment(campaignSendByDate, "MMMM Do YYYY, h:mm a").utc().format() : null;
            }
        }

        campaigns.push(campaign);
    });

    // If we have any validation errors, stop here
    if (campaigns.length !== $(".campaign-detail-form").length) {
        return;
    }

    // Create the campaign set
    const campaignSet = {
        name: name,
        url: url,
        urlparam: urlparam ? urlparam : "rid", // Always set a default of "rid" if empty
        qrsize: qrsize || null,
        basicauth: basicauth,
        use_shared_settings: useSharedSettings,
        use_shared_page: useSharedPage,
        use_shared_url: useSharedURL,
        use_shared_urlparam: useSharedURLParam,
        use_shared_qrsize: useSharedQRSize,
        use_shared_httpauth: useSharedHTTPAuth,
        use_shared_schedule: useSharedSchedule,
        launch_date: launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        send_by_date: send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        campaigns: campaigns
    };

    // Add page object if using shared page
    if (useSharedPage && page_id) {
        const pageSelect = $("#page");
        campaignSet.page = {
            name: pageSelect.find(`option[value="${page_id}"]`).text()
        };
    }

// Show the launch confirmation modal
    $("#launchDraftName").text(name);
    $("#launchDraftModal").data("campaignSet", campaignSet);
    $("#launchDraftModal").data("isDraft", false); // Flag to indicate this is not a draft
    $("#launchDraftModal").modal("show");
    
    // Override the launch button click handler in the modal
    $("#launchDraftModal .btn-danger").off("click").on("click", function() {
        const campaignSet = $("#launchDraftModal").data("campaignSet");
        const isDraft = $("#launchDraftModal").data("isDraft");
        
        if (isDraft) {
            // This is a draft campaign set, use the draft API
            const id = $("#launchDraftModal").data("id");
            api.draftCampaignSetId.launch(id)
                .success(function (data) {
                    $("#launchDraftModal").modal("hide");
                    $("#modal").modal("hide");
                    successFlashFade(`Campaign set launched successfully!`, 3);
                    loadCampaignSets();
                })
                .error(function (data) {
                    $("#launchDraftModal").modal("hide");
                    errorFlash(data.responseJSON.message);
                    console.error("Error launching draft campaign set:", data.responseJSON);
                });
        } else {
            // This is a new campaign set, use the regular API
            console.log("Submitting campaign set:", JSON.stringify(campaignSet, null, 2));
            api.campaignSets.post(campaignSet)
                .success(function (data) {
                    $("#launchDraftModal").modal("hide");
                    $("#modal").modal("hide");
                    successFlashFade(`Campaign set ${escapeHtml(name)} created successfully!`, 3);
                    loadCampaignSets();
                })
                .error(function (data) {
                    $("#launchDraftModal").modal("hide");
                    modalError(data.responseJSON.message);
                    console.error("Error creating campaign set:", data.responseJSON);
                });
        }
    });
}

// Saves a draft campaign set
function saveDraftCampaignSet() {
    const name = $("#name").val();
    const useSharedSettings = $("#useSharedSettings").is(":checked");
    const useSharedPage = useSharedSettings || $("#useSharedPage").is(":checked");
    const useSharedURL = useSharedSettings || $("#useSharedURL").is(":checked");
    const useSharedURLParam = useSharedSettings || $("#useSharedURLParam").is(":checked");
    const useSharedQRSize = useSharedSettings || $("#useSharedQRSize").is(":checked");
    const useSharedHTTPAuth = useSharedSettings || $("#useSharedHTTPAuth").is(":checked");
    const useSharedSchedule = useSharedSettings || $("#useSharedSchedule").is(":checked");
    
    let page_id = null;
    let url = "";
    let launch_date = "";
    let send_by_date = "";
    let urlparam = "";
    let qrsize = "";
    let basicauth = false;
    
    // Get shared settings values if any are enabled
    if (useSharedSettings || useSharedPage || useSharedURL || useSharedURLParam || 
        useSharedQRSize || useSharedHTTPAuth || useSharedSchedule) {
        
        if (useSharedPage) {
            page_id = $("#page").val();
        }
        
        if (useSharedURL) {
            url = $("#url").val();
        }
        
        if (useSharedURLParam) {
            urlparam = $("#urlparam").val();
        }
        
        if (useSharedQRSize) {
            qrsize = $("#qrsize").val();
        }
        
        if (useSharedHTTPAuth) {
            basicauth = $("#basicauth").is(":checked");
        }
        
        if (useSharedSchedule) {
            launch_date = $("#launch_date").val();
            send_by_date = $("#send_by_date").val();
        }
    }

    // Validate the campaign set
    if (name === "") {
        modalError("Campaign set name cannot be empty");
        return;
    }

    // Build the campaigns array
    const campaigns = [];
    $(".campaign-detail-form").each(function (i) {
        const campaign = {};
        campaign.name = $(this).find(".campaign-name").val();
        campaign.type = $(this).find(".campaign-type").val();

        if (campaign.name === "") {
            modalError(`Campaign ${i + 1} name cannot be empty`);
            return false;
        }

        if (campaign.type === "email") {
            const templateSelect = $(this).find(".email-template");
            const smtpSelect = $(this).find(".email-profile");
            const templateId = templateSelect.val();
            const smtpId = smtpSelect.val();

            if (templateId) {
                campaign.template = {
                    id: parseInt(templateId, 10),
                    name: templateSelect.find(`option[value="${templateId}"]`).text()
                };
            }
            if (smtpId) {
                campaign.smtp = {
                    id: parseInt(smtpId, 10),
                    name: smtpSelect.find(`option[value="${smtpId}"]`).text()
                };
            }
        } else if (campaign.type === "sms") {
            const smsTemplateSelect = $(this).find(".sms-template");
            const smsProfileSelect = $(this).find(".sms-profile");
            const smsTemplateId = smsTemplateSelect.val();
            const smsProfileId = smsProfileSelect.val();

            if (smsTemplateId) {
                campaign.sms_template = {
                    id: parseInt(smsTemplateId, 10),
                    name: smsTemplateSelect.find(`option[value="${smsTemplateId}"]`).text()
                };
            }
            if (smsProfileId) {
                campaign.sms = {
                    id: parseInt(smsProfileId, 10),
                    name: smsProfileSelect.find(`option[value="${smsProfileId}"]`).text()
                };
            }
        }

        // Get the groups as objects with names
        const groupSelect = $(this).find(".groups");
        const groupIds = groupSelect.val();

        // Convert group IDs to group objects with names and IDs
        campaign.groups = [];
        if (groupIds && groupIds.length > 0) {
            groupIds.forEach(function (id) {
                const groupOption = groupSelect.find(`option[value="${id}"]`);
                campaign.groups.push({
                    id: parseInt(id),
                    name: groupOption.text()
                });
            });
        }

        if (!useSharedSettings) {
            const pageSelect = $(this).find(".campaign-page");
            const pageId = pageSelect.val();

            if (pageId) {
                campaign.page = {
                    id: parseInt(pageId, 10),
                    name: pageSelect.find(`option[value="${pageId}"]`).text()
                };
            }

            campaign.url = $(this).find(".campaign-url").val();
            campaign.urlparam = $(this).find(".campaign-urlparam").val();
            campaign.qrsize = $(this).find(".campaign-qrsize").val() || "";
            campaign.basicauth = $(this).find(".campaign-basicauth").is(":checked");
            const campaignLaunchDate = $(this).find(".campaign-launch-date").val();
            const campaignSendByDate = $(this).find(".campaign-send-by-date").val();
            campaign.launch_date = campaignLaunchDate ? moment(campaignLaunchDate, "MMMM Do YYYY, h:mm a").utc().format() : null;
            campaign.send_by_date = campaignSendByDate ? moment(campaignSendByDate, "MMMM Do YYYY, h:mm a").utc().format() : null;
        } else {
            // Use shared page settings
            const pageSelect = $("#page");

            if (page_id) {
                campaign.page = {
                    id: parseInt(page_id, 10),
                    name: pageSelect.find(`option[value="${page_id}"]`).text()
                };
            }

            campaign.url = url;
            campaign.urlparam = urlparam || "rid";
            campaign.launch_date = launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : null;
            campaign.send_by_date = send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null;
        }

        campaigns.push(campaign);
    });

    // If we have any validation errors, stop here
    if (campaigns.length !== $(".campaign-detail-form").length) {
        return;
    }

    // Create the draft campaign set
    const draftCampaignSet = {
        name: name,
        url: url,
        urlparam: urlparam || "rid", // Always set a default of "rid" if empty
        qrsize: qrsize || "",
        basicauth: basicauth,
        use_shared_settings: useSharedSettings,
        use_shared_page: useSharedPage,
        use_shared_url: useSharedURL,
        use_shared_urlparam: useSharedURLParam,
        use_shared_qrsize: useSharedQRSize,
        use_shared_httpauth: useSharedHTTPAuth,
        use_shared_schedule: useSharedSchedule,
        launch_date: launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        send_by_date: send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        campaigns: campaigns
    };

    // Add page object if using shared page
    if (useSharedPage && page_id) {
        const pageSelect = $("#page");
        draftCampaignSet.page = {
            id: parseInt(page_id, 10),
            name: pageSelect.find(`option[value="${page_id}"]`).text()
        };
    }

    // Submit the draft campaign set
    console.log("Submitting draft campaign set:", JSON.stringify(draftCampaignSet, null, 2));
    api.draftCampaignSets.post(draftCampaignSet)
        .success(function (data) {
            $("#modal").modal("hide");
            successFlashFade(`Draft campaign set ${escapeHtml(name)} saved successfully!`, 3);
            loadCampaignSets();
        })
        .error(function (data) {
            modalError(data.responseJSON.message);
            console.error("Error saving draft campaign set:", data.responseJSON);
        });
}

// Function to update campaign settings visibility based on granular checkboxes
window.updateCampaignSettingsVisibility = function() {
    // For each campaign form, show/hide specific override fields based on granular settings
    $(".campaign-detail-form").each(function() {
        const $form = $(this);
        
        // First check if we should show the overall override section
        if ($("#useSharedSettings").is(":checked")) {
            $form.find(".campaign-settings-override").hide();
            return; // Skip the rest if using all shared settings
        } else {
            $form.find(".campaign-settings-override").show();
        }
        
        // Handle page visibility
        if ($("#useSharedPage").is(":checked")) {
            $form.find(".campaign-page").closest(".form-group").hide();
        } else {
            $form.find(".campaign-page").closest(".form-group").show();
        }
        
        // Handle URL visibility
        if ($("#useSharedURL").is(":checked")) {
            $form.find(".campaign-url").closest(".form-group").hide();
        } else {
            $form.find(".campaign-url").closest(".form-group").show();
        }
        
        // Handle URL parameter visibility
        if ($("#useSharedURLParam").is(":checked")) {
            $form.find(".campaign-urlparam").closest(".form-group").hide();
        } else {
            $form.find(".campaign-urlparam").closest(".form-group").show();
        }
        
        // Handle QR size visibility
        if ($("#useSharedQRSize").is(":checked")) {
            $form.find(".campaign-qrsize").closest(".form-group").hide();
        } else {
            $form.find(".campaign-qrsize").closest(".form-group").show();
        }
        
        // Handle HTTP auth visibility
        if ($("#useSharedHTTPAuth").is(":checked")) {
            $form.find(".campaign-basicauth").closest(".form-group").hide();
        } else {
            $form.find(".campaign-basicauth").closest(".form-group").show();
        }
        
        // Handle schedule visibility
        if ($("#useSharedSchedule").is(":checked")) {
            $form.find(".campaign-launch-date").closest(".form-group").hide();
            $form.find(".campaign-send-by-date").closest(".form-group").hide();
        } else {
            $form.find(".campaign-launch-date").closest(".form-group").show();
            $form.find(".campaign-send-by-date").closest(".form-group").show();
        }
    });
};

// Edits an existing draft campaign set
function editDraftCampaignSet(id) {
    // Clear any previous error messages
    $("#modal\\.flashes").empty();

    $("#modalLabel").text("Edit Draft Campaign Set");

    // Fully reset the tab structure to ensure it's in a clean state
    // First, remove active class from all tabs and panes
    $(".nav-tabs li").removeClass("active");
    $(".tab-pane").removeClass("active");
    
    // Show all tabs that might have been hidden
    $(".nav-tabs a[href='#generalSettings']").parent().show();
    $(".nav-tabs a[href='#campaignsTab']").parent().show();
    
    // Ensure proper tab classes are set
    $("#generalSettings").addClass("tab-pane");
    $("#campaignsTab").addClass("tab-pane");
    
    // Activate the general settings tab
    $(".nav-tabs a[href='#generalSettings']").parent().addClass("active");
    $("#generalSettings").addClass("active");
    
    // Force Bootstrap to reinitialize the tab functionality
    setTimeout(function() {
        $(".nav-tabs a[href='#generalSettings']").tab("show");
    }, 10);

    // Make sure all form fields are enabled (in case they were disabled by viewCampaignSet)
    $("#modal input, #modal select, #modal textarea").prop("disabled", false);
    $(".campaign-card-toggle, .btn-remove-campaign, #addCampaignButton").show();
    $("#saveDraftButton").show();
    $("#launchButton").show();

    // Get the draft campaign set
    api.draftCampaignSetId.get(id)
        .success(function (dcs) {
            // console.log("Editing draft campaign set:", JSON.stringify(dcs, null, 2));

            // Set basic form fields immediately
            $("#name").val(dcs.name);
            $("#url").val(dcs.url || "");
            $("#urlparam").val(dcs.urlparam || "");
            $("#qrsize").val(dcs.qrsize || "");
            $("#basicauth").prop("checked", dcs.basicauth || false);
            
            // Handle launch date - only set if it's a valid date (not 0001-01-01)
            if (dcs.launch_date && dcs.launch_date !== "0001-01-01T00:00:00Z") {
                $("#launch_date").val(moment(dcs.launch_date).format("MMMM Do YYYY, h:mm a"));
            } else {
                // Set to current date as default
                $("#launch_date").val(moment().format("MMMM Do YYYY, h:mm a"));
            }
            
            // Handle send by date - only set if it's a valid date
            if (dcs.send_by_date && dcs.send_by_date !== "0001-01-01T00:00:00Z") {
                $("#send_by_date").val(moment(dcs.send_by_date).format("MMMM Do YYYY, h:mm a"));
            } else {
                $("#send_by_date").val("");
            }

            // Set the shared settings checkbox based on the saved value
            const useSharedSettings = dcs.use_shared_settings !== undefined ? dcs.use_shared_settings : true;
            $("#useSharedSettings").prop("checked", useSharedSettings);
            
            // Set individual shared settings checkboxes and trigger change event
            $("#useSharedPage").prop("checked", dcs.use_shared_page !== undefined ? dcs.use_shared_page : true).trigger("change");
            $("#useSharedURL").prop("checked", dcs.use_shared_url !== undefined ? dcs.use_shared_url : true).trigger("change");
            $("#useSharedURLParam").prop("checked", dcs.use_shared_urlparam !== undefined ? dcs.use_shared_urlparam : true).trigger("change");
            $("#useSharedQRSize").prop("checked", dcs.use_shared_qrsize !== undefined ? dcs.use_shared_qrsize : true).trigger("change");
            $("#useSharedHTTPAuth").prop("checked", dcs.use_shared_httpauth !== undefined ? dcs.use_shared_httpauth : true).trigger("change");
            $("#useSharedSchedule").prop("checked", dcs.use_shared_schedule !== undefined ? dcs.use_shared_schedule : true).trigger("change");

            // Show/hide sections based on the shared settings value
            if (useSharedSettings) {
                $("#sharedSettingsSection").show();
                $(".campaign-settings-override").hide();
            } else {
                $("#sharedSettingsSection").hide();
                $(".campaign-settings-override").show();
            }
            
            // Show/hide individual shared settings sections
            $("#useSharedPage, #useSharedURL, #useSharedURLParam, #useSharedQRSize, #useSharedHTTPAuth, #useSharedSchedule").each(function() {
                const settingType = $(this).attr('id').replace('useShared', '');
                const isChecked = $(this).is(":checked");
                
                if (isChecked) {
                    $(`#shared${settingType}Section`).show();
                } else {
                    $(`#shared${settingType}Section`).hide();
                }
            });

            // Clear the campaigns list and detail panel
            $("#campaignList").empty();
            $("#campaignDetail").empty();
            $(".campaign-detail-placeholder").show();

            // Load available options first, then populate campaign entries
            loadAvailableOptions().then(() => {
                // console.log("Global options loaded, setting page dropdown");

                // Set page dropdown value now that options are loaded
                if (dcs.page) {
                    // console.log("Setting page value:", dcs.page);
                    if (dcs.page_id) {
                        safelySetSelect2Value("#page", dcs.page_id);
                    } else if (dcs.page.id) {
                        safelySetSelect2Value("#page", dcs.page.id);
                    } else if (dcs.page.name) {
                        setSelectByOptionText("#page", dcs.page.name);
                    }
                }

                // Add the campaigns
                const campaignPromises = [];
                if (dcs.campaigns && dcs.campaigns.length > 0) {
                    dcs.campaigns.forEach((campaign, i) => {
                        const promise = addCampaignEntry(i).then(() => {
                            // console.log(`Setting values for campaign ${i}`);

                            // Set basic campaign fields
                            $(`#campaign_name_${i}`).val(campaign.name);

                            // Update the list item text with the campaign name
                            const listItemText = campaign.name || `New Campaign ${i + 1}`;
                            $(`.campaign-list-item[data-index="${i}"] .campaign-list-item-text`).text(listItemText);
                            $(`#campaign_type_${i}`).val(campaign.type || "email").trigger("change");

                            // Set templates/profiles with a longer delay to ensure the type change has taken effect
                            // and Select2 is fully initialized
                            setTimeout(() => {
                                if (campaign.type === "email" || !campaign.type) {
                                    // Try to set template by ID first, then by name
                                    if (campaign.template_id) {
                                        safelySetSelect2Value(`#template_${i}`, campaign.template_id);
                                    } else if (campaign.template && campaign.template.id) {
                                        safelySetSelect2Value(`#template_${i}`, campaign.template.id);
                                    } else if (campaign.template && campaign.template.name) {
                                        setSelectByOptionText(`#template_${i}`, campaign.template.name);
                                    }

                                    // Try to set SMTP profile by ID first, then by name
                                    if (campaign.smtp_id) {
                                        safelySetSelect2Value(`#profile_${i}`, campaign.smtp_id);
                                    } else if (campaign.smtp && campaign.smtp.id) {
                                        safelySetSelect2Value(`#profile_${i}`, campaign.smtp.id);
                                    } else if (campaign.smtp && campaign.smtp.name) {
                                        setSelectByOptionText(`#profile_${i}`, campaign.smtp.name);
                                    }
                                } else if (campaign.type === "sms") {
                                    // Try to set SMS template by ID first, then by name
                                    if (campaign.sms_template_id) {
                                        safelySetSelect2Value(`#sms_template_${i}`, campaign.sms_template_id);
                                    } else if (campaign.sms_template && campaign.sms_template.id) {
                                        safelySetSelect2Value(`#sms_template_${i}`, campaign.sms_template.id);
                                    } else if (campaign.sms_template && campaign.sms_template.name) {
                                        setSelectByOptionText(`#sms_template_${i}`, campaign.sms_template.name);
                                    }

                                    // Try to set SMS profile by ID first, then by name
                                    if (campaign.sms_id) {
                                        safelySetSelect2Value(`#sms_profile_${i}`, campaign.sms_id);
                                    } else if (campaign.sms && campaign.sms.id) {
                                        safelySetSelect2Value(`#sms_profile_${i}`, campaign.sms.id);
                                    } else if (campaign.sms && campaign.sms.name) {
                                        setSelectByOptionText(`#sms_profile_${i}`, campaign.sms.name);
                                    }
                                }

                                // Set groups
                                if (campaign.groups && campaign.groups.length > 0) {
                                    const groupIds = campaign.groups.map(g => g.id.toString());
                                    safelySetSelect2Value(`#users_${i}`, groupIds);
                                }

                                // Set page by ID first, then by name
                                if (campaign.page_id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page_id);
                                } else if (campaign.page && campaign.page.id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page.id);
                                } else if (campaign.page && campaign.page.name) {
                                    setSelectByOptionText(`#campaign_page_${i}`, campaign.page.name);
                                }

                                // Other fields
                                $(`#campaign_url_${i}`).val(campaign.url || "");
                                $(`#campaign_urlparam_${i}`).val(campaign.urlparam || "");
                                $(`#campaign_qrsize_${i}`).val(campaign.qrsize || "");
                                $(`#campaign_basicauth_${i}`).prop("checked", campaign.basicauth || false);
                                    // Handle dates - only show if they are actually set
                                    if (campaign.launch_date && campaign.launch_date !== "0001-01-01T00:00:00Z") {
                                        $(`#campaign_launch_date_${i}`).val(moment(campaign.launch_date).format("MMMM Do YYYY, h:mm a"));
                                        $(`#campaign_launch_date_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_launch_date_${i}`).val("");
                                        $(`#campaign_launch_date_${i}`).closest('.form-group').hide();
                                    }
                                    
                                    if (campaign.send_by_date && campaign.send_by_date !== "0001-01-01T00:00:00Z") {
                                        $(`#campaign_send_by_date_${i}`).val(moment(campaign.send_by_date).format("MMMM Do YYYY, h:mm a"));
                                        $(`#campaign_send_by_date_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_send_by_date_${i}`).val("");
                                        $(`#campaign_send_by_date_${i}`).closest('.form-group').hide();
                                    }
                                    
                                    // Handle other optional fields - only show if they are actually set
                                    if (campaign.url && campaign.url.trim() !== "") {
                                        $(`#campaign_url_${i}`).val(campaign.url);
                                        $(`#campaign_url_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_url_${i}`).val("");
                                        $(`#campaign_url_${i}`).closest('.form-group').hide();
                                    }
                                    
                                    if (campaign.urlparam && campaign.urlparam.trim() !== "") {
                                        $(`#campaign_urlparam_${i}`).val(campaign.urlparam);
                                        $(`#campaign_urlparam_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_urlparam_${i}`).val("");
                                        $(`#campaign_urlparam_${i}`).closest('.form-group').hide();
                                    }
                                    
                                    if (campaign.qrsize && campaign.qrsize !== 0) {
                                        $(`#campaign_qrsize_${i}`).val(campaign.qrsize);
                                        $(`#campaign_qrsize_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_qrsize_${i}`).val("");
                                        $(`#campaign_qrsize_${i}`).closest('.form-group').hide();
                                    }
                                    
                                    if (campaign.basicauth) {
                                        $(`#campaign_basicauth_${i}`).prop("checked", true);
                                        $(`#campaign_basicauth_${i}`).closest('.form-group').show();
                                    } else {
                                        $(`#campaign_basicauth_${i}`).prop("checked", false);
                                        $(`#campaign_basicauth_${i}`).closest('.form-group').hide();
                                    }
                            }, 300); // Increased delay to ensure Select2 is fully initialized
                        });
                        campaignPromises.push(promise);
                    });
                } else {
                    addCampaignEntry(0);
                }

                // After all campaigns are loaded and populated
                Promise.all(campaignPromises).then(() => {
                    console.log("All campaigns loaded and populated");

                    // Update campaign settings visibility
                    updateCampaignSettingsVisibility();

                    // Update the buttons
                    $("#saveDraftButton").text("Update Draft");
                    $("#saveDraftButton").off("click").on("click", function () {
                        updateDraftCampaignSet(id);
                    });

                    // Change the launch button to launch the draft
                    $("#launchButton").text("Launch");
                    $("#launchButton").off("click").on("click", function () {
                        // Show the launch confirmation modal
                        prepareLaunchDraft(id);
                        $("#launchDraftModal").modal("show");
                    });

                    $("#modal").modal("show");
                });
            });
        })
        .error(function () {
            modalError("Error fetching draft campaign set");
        });
}

// Updates an existing draft campaign set
function updateDraftCampaignSet(id) {
    const name = $("#name").val();
    const useSharedSettings = $("#useSharedSettings").is(":checked");
    const useSharedPage = useSharedSettings || $("#useSharedPage").is(":checked");
    const useSharedURL = useSharedSettings || $("#useSharedURL").is(":checked");
    const useSharedURLParam = useSharedSettings || $("#useSharedURLParam").is(":checked");
    const useSharedQRSize = useSharedSettings || $("#useSharedQRSize").is(":checked");
    const useSharedHTTPAuth = useSharedSettings || $("#useSharedHTTPAuth").is(":checked");
    const useSharedSchedule = useSharedSettings || $("#useSharedSchedule").is(":checked");
    
    let page_id = null;
    let url = "";
    let urlparam = "";
    let launch_date = "";
    let send_by_date = "";
    let qrsize = "";
    let basicauth = false;
    
    // Get shared settings values if any are enabled
    if (useSharedSettings || useSharedPage || useSharedURL || useSharedURLParam || 
        useSharedQRSize || useSharedHTTPAuth || useSharedSchedule) {
        
        if (useSharedPage) {
            page_id = $("#page").val();
        }
        
        if (useSharedURL) {
            url = $("#url").val();
        }
        
        if (useSharedURLParam) {
            urlparam = $("#urlparam").val();
        }
        
        if (useSharedQRSize) {
            qrsize = $("#qrsize").val();
        }
        
        if (useSharedHTTPAuth) {
            basicauth = $("#basicauth").is(":checked");
        }
        
        if (useSharedSchedule) {
            launch_date = $("#launch_date").val();
            send_by_date = $("#send_by_date").val();
        }
    }

    // Validate the campaign set
    if (name === "") {
        modalError("Campaign set name cannot be empty");
        return;
    }

    // Build the campaigns array
    const campaigns = [];
    $(".campaign-detail-form").each(function (i) {
        const campaign = {};
        campaign.name = $(this).find(".campaign-name").val();
        campaign.type = $(this).find(".campaign-type").val();

        if (campaign.name === "") {
            modalError(`Campaign ${i + 1} name cannot be empty`);
            return false;
        }

        if (campaign.type === "email") {
            const templateSelect = $(this).find(".email-template");
            const smtpSelect = $(this).find(".email-profile");
            const templateId = templateSelect.val();
            const smtpId = smtpSelect.val();

            if (templateId) {
                campaign.template = {
                    id: parseInt(templateId, 10),
                    name: templateSelect.find(`option[value="${templateId}"]`).text()
                };
            }
            if (smtpId) {
                campaign.smtp = {
                    id: parseInt(smtpId, 10),
                    name: smtpSelect.find(`option[value="${smtpId}"]`).text()
                };
            }
        } else if (campaign.type === "sms") {
            const smsTemplateSelect = $(this).find(".sms-template");
            const smsProfileSelect = $(this).find(".sms-profile");
            const smsTemplateId = smsTemplateSelect.val();
            const smsProfileId = smsProfileSelect.val();

            if (smsTemplateId) {
                campaign.sms_template = {
                    id: parseInt(smsTemplateId, 10),
                    name: smsTemplateSelect.find(`option[value="${smsTemplateId}"]`).text()
                };
            }
            if (smsProfileId) {
                campaign.sms = {
                    id: parseInt(smsProfileId, 10),
                    name: smsProfileSelect.find(`option[value="${smsProfileId}"]`).text()
                };
            }
        }

        // Get the groups as objects with names
        const groupSelect = $(this).find(".groups");
        const groupIds = groupSelect.val();

        // Convert group IDs to group objects with names and IDs
        campaign.groups = [];
        if (groupIds && groupIds.length > 0) {
            groupIds.forEach(function (id) {
                const groupOption = groupSelect.find(`option[value="${id}"]`);
                campaign.groups.push({
                    id: parseInt(id),
                    name: groupOption.text()
                });
            });
        }

        if (!useSharedSettings) {
            const pageSelect = $(this).find(".campaign-page");
            const pageId = pageSelect.val();

            if (pageId) {
                campaign.page = {
                    id: parseInt(pageId, 10),
                    name: pageSelect.find(`option[value="${pageId}"]`).text()
                };
            }

            campaign.url = $(this).find(".campaign-url").val();
            campaign.urlparam = $(this).find(".campaign-urlparam").val();
            campaign.qrsize = $(this).find(".campaign-qrsize").val() || "";
            campaign.basicauth = $(this).find(".campaign-basicauth").is(":checked");
            const campaignLaunchDate = $(this).find(".campaign-launch-date").val();
            const campaignSendByDate = $(this).find(".campaign-send-by-date").val();
            campaign.launch_date = campaignLaunchDate ? moment(campaignLaunchDate, "MMMM Do YYYY, h:mm a").utc().format() : null;
            campaign.send_by_date = campaignSendByDate ? moment(campaignSendByDate, "MMMM Do YYYY, h:mm a").utc().format() : null;
        } else {
            // Use shared page settings
            const pageSelect = $("#page");

            campaign.page = {
                id: parseInt(page_id, 10),
                name: pageSelect.find(`option[value="${page_id}"]`).text()
            };

            campaign.url = url;
            campaign.urlparam = urlparam;
            campaign.launch_date = launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : null;
            campaign.send_by_date = send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null;
        }

        campaigns.push(campaign);
    });

    // If we have any validation errors, stop here
    if (campaigns.length !== $(".campaign-detail-form").length) {
        return;
    }

    // Create the draft campaign set
    const draftCampaignSet = {
        name: name,
        url: url,
        urlparam: urlparam || "rid", // Always set a default of "rid" if empty
        qrsize: qrsize || "",
        basicauth: basicauth,
        use_shared_settings: useSharedSettings,
        use_shared_page: useSharedPage,
        use_shared_url: useSharedURL,
        use_shared_urlparam: useSharedURLParam,
        use_shared_qrsize: useSharedQRSize,
        use_shared_httpauth: useSharedHTTPAuth,
        use_shared_schedule: useSharedSchedule,
        launch_date: launch_date ? moment(launch_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        send_by_date: send_by_date ? moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format() : null,
        campaigns: campaigns
    };

    // Add page object if using shared page
    if (useSharedPage && page_id) {
        const pageSelect = $("#page");
        draftCampaignSet.page = {
            id: parseInt(page_id, 10),
            name: pageSelect.find(`option[value="${page_id}"]`).text()
        };
    }

    // Submit the draft campaign set
    console.log("Updating draft campaign set:", JSON.stringify(draftCampaignSet, null, 2));
    api.draftCampaignSetId.put(id, draftCampaignSet)
        .success(function (data) {
            $("#modal").modal("hide");
            successFlashFade(`Draft campaign set ${escapeHtml(name)} updated successfully!`, 3);
            loadCampaignSets();
        })
        .error(function (data) {
            modalError(data.responseJSON.message);
            console.error("Error updating draft campaign set:", data.responseJSON);
        });
}

// Prepares to launch a draft campaign set
function prepareLaunchDraft(id) {
    // Store the ID for use in the launch function
    $("#launchDraftModal").data("id", id);
    $("#launchDraftModal").data("isDraft", true); // Flag to indicate this is a draft

    // Get the draft campaign set name
    api.draftCampaignSetId.get(id)
        .success(function (dcs) {
            $("#launchDraftName").text(dcs.name);
            
            // Store the draft campaign set data for validation
            $("#launchDraftModal").data("draftData", dcs);
            
            // Hide the edit modal if it's open
            if ($("#modal").hasClass("in")) {
                // Don't actually close it, just hide it visually to prevent data loss
                $("#modal").css("display", "none");
                // Store that we need to restore it later
                $("#launchDraftModal").data("restoreEditModal", true);
            } else {
                $("#launchDraftModal").data("restoreEditModal", false);
            }
        });
}

// Launches a draft campaign set or a new campaign set
function launchDraftCampaignSet() {
    const isDraft = $("#launchDraftModal").data("isDraft");
    
    if (isDraft) {
        // This is a draft campaign set
        const id = $("#launchDraftModal").data("id");
        const dcs = $("#launchDraftModal").data("draftData");
        
        if (!dcs) {
            $("#launchDraftModal").modal("hide");
            errorFlash("Error: Draft campaign set data not found");
            return;
        }

        // Perform comprehensive validation
        const validationResult = validateDraftCampaignSet(dcs);
        
        if (!validationResult.valid) {
            $("#launchDraftModal").modal("hide");
            errorFlash(validationResult.message);
            
            // Restore the edit modal if it was open
            if ($("#launchDraftModal").data("restoreEditModal")) {
                $("#modal").css("display", "block");
            }
            return;
        }
        
        // Launch the draft campaign set directly
        // The backend now properly handles all shared settings
        api.draftCampaignSetId.launch(id)
            .success(function (data) {
                // Close both modals
                $("#launchDraftModal").modal("hide");
                $("#modal").modal("hide").css("display", "");
                successFlashFade(`Campaign set launched successfully!`, 3);
                loadCampaignSets();
            })
            .error(function (data) {
                $("#launchDraftModal").modal("hide");
                errorFlash(data.responseJSON.message);
                console.error("Error launching draft campaign set:", data.responseJSON);
                
                // Restore the edit modal if it was open
                if ($("#launchDraftModal").data("restoreEditModal")) {
                    $("#modal").css("display", "block");
                }
            });
    } else {
        // This is a new campaign set
        const campaignSet = $("#launchDraftModal").data("campaignSet");
        const name = campaignSet.name;
        
        // Submit the campaign set
        console.log("Submitting new campaign set:", JSON.stringify(campaignSet, null, 2));
        api.campaignSets.post(campaignSet)
            .success(function (data) {
                $("#launchDraftModal").modal("hide");
                $("#modal").modal("hide");
                successFlashFade(`Campaign set ${escapeHtml(name)} created successfully!`, 3);
                loadCampaignSets();
            })
            .error(function (data) {
                $("#launchDraftModal").modal("hide");
                modalError(data.responseJSON.message);
                console.error("Error creating campaign set:", data.responseJSON);
            });
    }
}

// Validates a draft campaign set before launching
function validateDraftCampaignSet(dcs) {
    let valid = true;
    let message = "";
    
    // Debug log the draft campaign set
    console.log("Validating draft campaign set:", JSON.stringify(dcs, null, 2));
    
    // Check if the campaign set has a name
    if (!dcs.name || dcs.name.trim() === "") {
        valid = false;
        message = "Campaign set name cannot be empty";
        return { valid, message };
    }
    
    // Check if the campaign set has campaigns
    if (!dcs.campaigns || dcs.campaigns.length === 0) {
        valid = false;
        message = "Campaign set must have at least one campaign";
        return { valid, message };
    }
    
    // Check if shared page is specified when using shared page
    if ((dcs.use_shared_settings || dcs.use_shared_page)) {
        // Check if page is specified either as page_id or page.id
        const hasPageId = dcs.page_id && dcs.page_id > 0;
        const hasPageObject = dcs.page && dcs.page.id && dcs.page.id > 0;
        
        if (!hasPageId && !hasPageObject) {
            valid = false;
            message = "No landing page specified. Please specify a shared landing page before launching.";
            console.error("Missing landing page:", { page_id: dcs.page_id, page: dcs.page });
            return { valid, message };
        }
    }
    
    // Check if shared URL is specified when using shared URL
    if ((dcs.use_shared_settings || dcs.use_shared_url) && (!dcs.url || dcs.url.trim() === "")) {
        valid = false;
        message = "Shared URL cannot be empty. Please specify a URL before launching.";
        return { valid, message };
    }
    
    // Check each campaign
    for (let i = 0; i < dcs.campaigns.length; i++) {
        const campaign = dcs.campaigns[i];
        
        // Debug log the campaign
        console.log(`Validating campaign ${i}:`, JSON.stringify(campaign, null, 2));
        
        // Check if campaign has a name
        if (!campaign.name || campaign.name.trim() === "") {
            valid = false;
            message = `Campaign ${i+1} name cannot be empty`;
            return { valid, message };
        }
        
        // Check if campaign has a type
        if (!campaign.type) {
            valid = false;
            message = `Campaign ${i+1} type is not specified`;
            return { valid, message };
        }
        
        // Check if campaign has required fields based on type
        if (campaign.type === "email") {
            // Check template - could be in template.id or template_id
            const hasTemplateId = campaign.template_id && campaign.template_id > 0;
            const hasTemplateObject = campaign.template && campaign.template.id && campaign.template.id > 0;
            
            if (!hasTemplateId && !hasTemplateObject) {
                valid = false;
                message = `Campaign ${i+1} does not have an email template specified`;
                console.error("Missing email template:", { template_id: campaign.template_id, template: campaign.template });
                return { valid, message };
            }
            
            // Check SMTP profile - could be in smtp.id or smtp_id
            const hasSmtpId = campaign.smtp_id && campaign.smtp_id > 0;
            const hasSmtpObject = campaign.smtp && campaign.smtp.id && campaign.smtp.id > 0;
            
            if (!hasSmtpId && !hasSmtpObject) {
                valid = false;
                message = `Campaign ${i+1} does not have a sending profile specified`;
                console.error("Missing SMTP profile:", { smtp_id: campaign.smtp_id, smtp: campaign.smtp });
                return { valid, message };
            }
        } else if (campaign.type === "sms") {
            // Check SMS template - could be in sms_template.id or sms_template_id
            const hasSmsTemplateId = campaign.sms_template_id && campaign.sms_template_id > 0;
            const hasSmsTemplateObject = campaign.sms_template && campaign.sms_template.id && campaign.sms_template.id > 0;
            
            if (!hasSmsTemplateId && !hasSmsTemplateObject) {
                valid = false;
                message = `Campaign ${i+1} does not have an SMS template specified`;
                console.error("Missing SMS template:", { sms_template_id: campaign.sms_template_id, sms_template: campaign.sms_template });
                return { valid, message };
            }
            
            // Check SMS profile - could be in sms.id or sms_id
            const hasSmsId = campaign.sms_id && campaign.sms_id > 0;
            const hasSmsObject = campaign.sms && campaign.sms.id && campaign.sms.id > 0;
            
            if (!hasSmsId && !hasSmsObject) {
                valid = false;
                message = `Campaign ${i+1} does not have an SMS sending profile specified`;
                console.error("Missing SMS profile:", { sms_id: campaign.sms_id, sms: campaign.sms });
                return { valid, message };
            }
        }
        
        // Check if campaign has groups
        if (!campaign.groups || campaign.groups.length === 0) {
            valid = false;
            message = `Campaign ${i+1} does not have any groups specified`;
            return { valid, message };
        }
        
        // Check if campaign has a landing page (either shared or individual)
        if (!dcs.use_shared_settings && !dcs.use_shared_page) {
            // Check if page is specified either as page_id or page.id
            const hasPageId = campaign.page_id && campaign.page_id > 0;
            const hasPageObject = campaign.page && campaign.page.id && campaign.page.id > 0;
            
            if (!hasPageId && !hasPageObject) {
                valid = false;
                message = `Campaign ${i+1} does not have a landing page specified`;
                console.error("Missing campaign landing page:", { page_id: campaign.page_id, page: campaign.page });
                return { valid, message };
            }
        }
        
        // Check if campaign has a URL (either shared or individual)
        if (!dcs.use_shared_settings && !dcs.use_shared_url) {
            if (!campaign.url || campaign.url.trim() === "") {
                valid = false;
                message = `Campaign ${i+1} does not have a URL specified`;
                return { valid, message };
            }
        }
    }
    
    console.log("Draft campaign set validation successful");
    return { valid, message };
}

// This function has been consolidated into launchDraftCampaignSet

// View a campaign set
function viewCampaignSet(id) {
    // Clear any previous error messages
    $("#modal\\.flashes").empty();
    
    // Add modal close handler to restore tab functionality after viewing
    $("#modal").one("hidden.bs.modal", function() {
        // Reset tab structure when modal is closed to prevent tab issues in subsequent modals
        $(".nav-tabs li").removeClass("active");
        $(".tab-pane").removeClass("active");
        $(".nav-tabs a[href='#generalSettings']").parent().show();
        $(".nav-tabs a[href='#campaignsTab']").parent().show();
        // Restore any campaign summaries that might have been added
        $(".campaign-summary").remove();
        $("#overviewTabNav").hide().removeClass("active");
        $("#overviewTab").removeClass("active");
        currentSetSummary = null;
        campaignSetSummaryById = {};
    });

    // Get the campaign set
    api.campaignSetId.get(id)
        .success(function (cs) {
            $("#modalLabel").text("View Campaign Set");

            // Set basic form fields immediately
            $("#name").val(cs.name);
            $("#url").val(cs.url || "");
            $("#urlparam").val(cs.urlparam || "");
            $("#qrsize").val(cs.qrsize || "");
            $("#basicauth").prop("checked", cs.basicauth || false);
            if (cs.launch_date) {
                $("#launch_date").val(moment(cs.launch_date).format("MMMM Do YYYY, h:mm a"));
            }
            if (cs.send_by_date) {
                $("#send_by_date").val(moment(cs.send_by_date).format("MMMM Do YYYY, h:mm a"));
            }

            // Set the shared settings checkbox based on the saved value
            const useSharedSettings = cs.use_shared_settings !== undefined ? cs.use_shared_settings : true;
            $("#useSharedSettings").prop("checked", useSharedSettings);

            // Show/hide sections based on the shared settings value
            if (useSharedSettings) {
                $("#sharedSettingsSection").show();
                $(".campaign-settings-override").hide();
            } else {
                $("#sharedSettingsSection").hide();
                $(".campaign-settings-override").show();
            }

            // Clear the campaigns list and detail panel
            $("#campaignList").empty();
            $("#campaignDetail").empty();
            $(".campaign-detail-placeholder").show();

            // Load available options
            loadAvailableOptions().then(() => {
                console.log("Preparing view-only campaign set display");

                // Update the modal title to include the campaign set name
                $("#modalLabel").text(`View Campaign Set - ${cs.name}`);
                
                // Set data attributes for the modal
                $("#modal").data("viewMode", true);
                $("#modal").data("campaignSetId", cs.id);

                // Clear and reset the modal content
                // Show Overview + Campaigns, hide General Settings, default to Overview.
                $("#sharedSettingsSection").hide();
                $(".nav-tabs a[href='#generalSettings']").parent().hide();
                $("#overviewTabNav").show().addClass("active");
                $(".nav-tabs a[href='#campaignsTab']").parent().removeClass("active");
                $("#generalSettings").removeClass("active");
                $("#campaignsTab").removeClass("active");
                $("#overviewTab").addClass("active");
                $("#campaignDetail").empty();
                $("#campaignList").empty();
                
                // Hide all form controls and buttons
                $(".campaign-card-toggle, .btn-remove-campaign, #addCampaignButton").hide();
                $("#saveDraftButton").hide();
                $("#launchButton").hide();
                
                // Create the campaign list items
                if (cs.campaigns && cs.campaigns.length > 0) {
                    cs.campaigns.forEach((campaign, i) => {
                        // Create a list item for each campaign
                        const icon = campaign.type === "email" ? "fa-envelope" : "fa-mobile";
                        const sentIcon = campaign.type === "email" ? "fa-envelope-o" : "fa-mobile";
                        
                        // Create different stats based on campaign type
                        let statsHtml = '';
                        if (campaign.type === "email") {
                            statsHtml = `
                                <span class="campaign-stat campaign-stat-sent" title="Sent"><i class="fa fa-envelope-o"></i> <span class="stat-sent">0</span></span>
                                <span class="campaign-stat campaign-stat-opened" title="Opened"><i class="fa fa-envelope-open-o"></i> <span class="stat-opened">0</span></span>
                                <span class="campaign-stat campaign-stat-clicked" title="Clicked"><i class="fa fa-mouse-pointer"></i> <span class="stat-clicked">0</span></span>
                                <span class="campaign-stat campaign-stat-replied" title="Replied"><i class="fa fa-reply"></i> <span class="stat-replied">0</span></span>
                                <span class="campaign-stat campaign-stat-submitted" title="Submitted Data"><i class="fa fa-exclamation-circle"></i> <span class="stat-submitted">0</span></span>
                                <span class="campaign-stat campaign-stat-reported" title="Reported"><i class="fa fa-bullhorn"></i> <span class="stat-reported">0</span></span>
                            `;
                        } else {
                            statsHtml = `
                                <span class="campaign-stat campaign-stat-sent" title="Sent"><i class="fa fa-mobile"></i> <span class="stat-sent">0</span></span>
                                <span class="campaign-stat campaign-stat-clicked" title="Clicked"><i class="fa fa-mouse-pointer"></i> <span class="stat-clicked">0</span></span>
                                <span class="campaign-stat campaign-stat-submitted" title="Submitted Data"><i class="fa fa-exclamation-circle"></i> <span class="stat-submitted">0</span></span>
                            `;
                        }
                        
                        const campaignListItem = `
                            <a href="#" class="list-group-item campaign-list-item" data-index="${i}" data-type="${campaign.type}" data-campaign-id="${campaign.id}">
                                <span class="campaign-list-item-icon"><i class="fa ${icon}"></i></span>
                                <span class="campaign-list-item-text">${escapeHtml(campaign.name)}</span>
                                <div class="campaign-stats-container">
                                    ${statsHtml}
                                </div>
                            </a>
                        `;
                        $("#campaignList").append(campaignListItem);
                        
                            // Add results button if the campaign has an ID
                            if (campaign.id) {
                                const $listItem = $(`.campaign-list-item[data-index="${i}"]`);
                                $listItem.append(`
                                    <div style="position:absolute; right:10px; top:10px;">
                                        <button class="btn btn-sm btn-primary campaign-results-btn" 
                                                onclick="window.location='/campaigns/${campaign.id}'">
                                            <i class="fa fa-bar-chart"></i>
                                        </button>
                                    </div>
                                `);
                        }
                    });

                    // One request supplies the Overview tab AND every list badge,
                    // replacing the previous per-campaign summary calls.
                    api.campaignSetId.summary(cs.id)
                        .success(function (summary) {
                            renderCampaignSetOverview(summary);

                            campaignSetSummaryById = {};
                            (summary.campaigns || []).forEach(function (campaign) {
                                campaignSetSummaryById[campaign.id] = campaign;
                            });

                            // Fill the left-hand list badges from the same payload.
                            $(".campaign-list-item").each(function () {
                                var $item = $(this);
                                var campaign = campaignSetSummaryById[$item.data("campaign-id")];
                                if (!campaign) {
                                    return;
                                }
                                var stats = campaign.stats || {};
                                $item.find(".campaign-stats-container").show();
                                $item.find(".stat-sent").text(stats.sent || 0);
                                $item.find(".stat-clicked").text(stats.clicked || 0);
                                $item.find(".stat-submitted").text(stats.submitted_data || 0);
                                if (campaign.type === "email") {
                                    $item.find(".stat-opened").text(stats.opened || 0);
                                    $item.find(".stat-reported").text(stats.email_reported || 0);
                                    $item.find(".stat-replied").text(stats.replied || 0);
                                }
                            });

                            // The detail panel was already rendered (synchronously, from
                            // cs.campaigns.forEach's .first().trigger("click")) before this
                            // summary response arrived, so its targets count was written as
                            // "Unavailable". Now that campaignSetSummaryById is populated,
                            // re-render whichever campaign is currently selected so its
                            // targets count fills in. showCampaignSummary() removes any
                            // existing ".campaign-summary" before appending a new one, so this
                            // re-invoke replaces the stale panel rather than stacking a second.
                            const $activeItem = $(".campaign-list-item.active");
                            if ($activeItem.length) {
                                const activeIndex = $activeItem.data("index");
                                showCampaignSummary(cs.campaigns[activeIndex], activeIndex, cs.use_shared_settings, cs.id, cs.campaigns.length === 1);
                            }
                        })
                        .error(function () {
                            $("#overviewHeader").html('<div class="alert alert-warning">Could not load the campaign set overview.</div>');
                        });

                    // Add click handler to campaign list items
                    $(".campaign-list-item").on("click", function(e) {
                        e.preventDefault();
                        
                        // Get the campaign index
                        const index = $(this).data("index");
                        
                        // Update active state in list
                        $(".campaign-list-item").removeClass("active");
                        $(this).addClass("active");
                        
                        // Show the campaign summary
                        showCampaignSummary(cs.campaigns[index], index, cs.use_shared_settings, cs.id, cs.campaigns.length === 1);
                    });
                    
                    // Trigger a click on the first campaign to show its summary
                    $(".campaign-list-item").first().trigger("click");
                }

                // Show the modal
                $("#modal").modal("show");
            });
        })
        .error(function () {
            errorFlash("Error fetching campaign set");
        });
}

// Function removed - redundant with showCampaignSummary

// Complete a campaign set
function completeCampaignSet(id) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will mark the campaign set as complete. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Complete",
        confirmButtonColor: "#28a745",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignSetId.complete(id)
                    .success(function (msg) {
                        resolve();
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message);
                    });
            });
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'Campaign Set Completed!',
                'The campaign set has been marked as complete!',
                'success'
            );

            const row = campaignSetTable.row(`[data-campaign-set-id="${id}"]`);
            if (row.node()) {
                row.data()[4] = "Completed"; // update status column
                campaignSetTable.row(row).invalidate().draw(false);

                // Disable the complete button
                const $actionsCell = $(row.node()).find('td').last();
                $actionsCell.find('button.btn-success').prop('disabled', true);
            } else {
                loadCampaignSets(); // fallback
            }
        }
    });
}

// Delete a campaign set
function deleteCampaignSet(id) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign set and all its campaigns. This can't be undone!",
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
                api.campaignSetId.delete(id)
                    .success(function (msg) {
                        resolve();
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message);
                    });
            });
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'Campaign Set Deleted!',
                'The campaign set has been deleted!',
                'success'
            );
            loadCampaignSets();
        }
    });
}

// Delete a campaign from a campaign set
function deleteCampaignFromSet(campaignId, campaignSetId, isLastCampaign) {
    let title = "Delete Campaign";
    let text = "Are you sure you want to delete this campaign?";
    
    // If this is the last campaign in the set, warn that it will delete the entire set
    if (isLastCampaign) {
        title = "Delete Last Campaign";
        text = "This is the last campaign in the set. Deleting it will also delete the entire campaign set. Are you sure?";
    }
    
    Swal.fire({
        title: title,
        text: text,
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
                // Delete the campaign
                api.campaignId.delete(campaignId)
                    .success(function (msg) {
                        // If this was the last campaign, also delete the campaign set
                        if (isLastCampaign) {
                            api.campaignSetId.delete(campaignSetId)
                                .success(function (msg) {
                                    resolve({ campaignDeleted: true, setDeleted: true });
                                })
                                .error(function (data) {
                                    reject("Campaign deleted, but error deleting campaign set: " + 
                                          (data.responseJSON ? data.responseJSON.message : "Unknown error"));
                                });
                        } else {
                            resolve({ campaignDeleted: true, setDeleted: false });
                        }
                    })
                    .error(function (data) {
                        reject(data.responseJSON ? data.responseJSON.message : "Error deleting campaign");
                    });
            });
        }
    }).then(function (result) {
        if (result.value) {
            if (result.value.setDeleted) {
                Swal.fire(
                    'Deleted!',
                    'The campaign and campaign set have been deleted!',
                    'success'
                );
                // Close the modal and reload the campaign sets
                $("#modal").modal("hide");
                loadCampaignSets();
            } else {
                Swal.fire(
                    'Campaign Deleted!',
                    'The campaign has been deleted!',
                    'success'
                );
                // Reload the campaign set view to reflect the changes
                viewCampaignSet(campaignSetId);
            }
        }
    }).catch(function(error) {
        Swal.fire(
            'Error!',
            error,
            'error'
        );
    });
}

// Shows a summary of the campaign instead of the form
function showCampaignSummary(campaign, index, useSharedSettings, campaignSetId, isLastCampaign) {
    // Hide the form
    $(`#campaign_form_${index}`).hide();
    
    // Remove all existing summaries to prevent stacking
    $(".campaign-summary").remove();
    
    // Create the summary container
    const $summaryContainer = $(`<div id="campaign_summary_${index}" class="campaign-summary" style="text-align: left; margin: 15px;"></div>`);
    
    // Build the summary content directly in the container (no extra frame)
    let summaryContent = "";
    
    // Only show fields that were actually set
    summaryContent += "<h4>" + escapeHtml(campaign.name) + "</h4>";
    summaryContent += "<strong>Type:</strong> " + (campaign.type === "email" ? "Email" : "SMS") + "<br>";
    
    // Email-specific or SMS-specific fields
    if (campaign.type === "email" || !campaign.type) {
        if (campaign.template && campaign.template.name) {
            summaryContent += "<strong>Email Template:</strong> " + escapeHtml(campaign.template.name) + "<br>";
        }
        if (campaign.smtp && campaign.smtp.name) {
            summaryContent += "<strong>Sending Profile:</strong> " + escapeHtml(campaign.smtp.name) + "<br>";
        }
    } else if (campaign.type === "sms") {
        if (campaign.sms_template && campaign.sms_template.name) {
            summaryContent += "<strong>SMS Template:</strong> " + escapeHtml(campaign.sms_template.name) + "<br>";
        }
        if (campaign.sms && campaign.sms.name) {
            summaryContent += "<strong>SMS Sending Profile:</strong> " + escapeHtml(campaign.sms.name) + "<br>";
        }
    }
    
    // Targets come from the set summary already in memory. getCampaignStats
    // computes this as len(results), which is exactly what the old
    // api.campaignId.results(...).length call was measuring — so this is the
    // same number without the request.
    if (campaign.id) {
        var cached = campaignSetSummaryById[campaign.id];
        var totalTargets = cached && cached.stats ? cached.stats.total : null;
        summaryContent += "<strong>Targets:</strong> " +
            (totalTargets === null ? "Unavailable" : totalTargets + " total targets") + "<br>";
    }
    
    // Landing page
    if (campaign.page && campaign.page.name) {
        summaryContent += "<strong>Landing Page:</strong> " + escapeHtml(campaign.page.name) + "<br>";
    }
    
    // URL
    if (campaign.url && campaign.url.trim() !== "") {
        summaryContent += "<strong>URL:</strong> " + escapeHtml(campaign.url) + "<br>";
    }
    
    // URL parameter
    if (campaign.urlparam && campaign.urlparam.trim() !== "") {
        summaryContent += "<strong>URL Parameter:</strong> " + escapeHtml(campaign.urlparam) + "<br>";
    }
    
    // QR code size
    if (campaign.qrsize && campaign.qrsize !== 0) {
        summaryContent += "<strong>QR Code Size:</strong> " + campaign.qrsize + "<br>";
    }
    
    // HTTP Basic Auth
    if (campaign.basicauth) {
        summaryContent += "<strong>HTTP Basic Auth:</strong> Enabled<br>";
    }
    
    // Launch date
    if (campaign.launch_date && campaign.launch_date !== "0001-01-01T00:00:00Z") {
        summaryContent += "<strong>Launch Date:</strong> " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a') + "<br>";
    }
    
    // Send by date
    if (campaign.send_by_date && campaign.send_by_date !== "0001-01-01T00:00:00Z") {
        summaryContent += "<strong>Send By Date:</strong> " + moment(campaign.send_by_date).format('MMMM Do YYYY, h:mm:ss a') + "<br>";
    }
    
    // Status if available
    if (campaign.status) {
        summaryContent += "<strong>Status:</strong> " + campaign.status + "<br>";
    }
    
    // Results link if the campaign has an ID
    if (campaign.id) {
        summaryContent += `<div style="margin-top: 15px;"><a href="/campaigns/${campaign.id}" class="btn btn-primary"><i class="fa fa-bar-chart"></i> View Full Results</a></div>`;
    }
    
    // Add the summary to the container
    $summaryContainer.html(summaryContent);
    
    // Add the summary to the campaign detail area
    $("#campaignDetail").append($summaryContainer);
    $summaryContainer.show();
}

// Copies an existing campaign set
function copyCampaignSet(id) {
    // Clear any previous error messages
    $("#modal\\.flashes").empty();

    // Reset the modal title
    $("#modalLabel").text("Copy Campaign Set");
    
    // Get the campaign set
    api.campaignSetId.get(id)
        .success(function (cs) {
            // Reset the tabs to show both
            $(".nav-tabs li").removeClass("active");
            $(".tab-pane").removeClass("active");
            $(".nav-tabs a[href='#generalSettings']").parent().show();
            $(".nav-tabs a[href='#campaignsTab']").parent().show();
            $(".nav-tabs a[href='#generalSettings']").parent().addClass("active");
            $("#generalSettings").addClass("active");
            
            // Clear any previous summary views
            $(".campaign-summary").remove();

            // Set basic form fields with copied data
            $("#name").val("Copy of " + cs.name);
            $("#url").val(cs.url || "");
            $("#urlparam").val(cs.urlparam || "");
            $("#qrsize").val(cs.qrsize || "");
            $("#basicauth").prop("checked", cs.basicauth || false);
            
            // Set launch date to current time (not the original)
            $("#launch_date").val(moment().format("MMMM Do YYYY, h:mm a"));
            $("#send_by_date").val("");

            // Set the shared settings checkboxes
            const useSharedSettings = cs.use_shared_settings !== undefined ? cs.use_shared_settings : true;
            $("#useSharedSettings").prop("checked", useSharedSettings);
            
            $("#useSharedPage").prop("checked", cs.use_shared_page !== undefined ? cs.use_shared_page : true).trigger("change");
            $("#useSharedURL").prop("checked", cs.use_shared_url !== undefined ? cs.use_shared_url : true).trigger("change");
            $("#useSharedURLParam").prop("checked", cs.use_shared_urlparam !== undefined ? cs.use_shared_urlparam : true).trigger("change");
            $("#useSharedQRSize").prop("checked", cs.use_shared_qrsize !== undefined ? cs.use_shared_qrsize : true).trigger("change");
            $("#useSharedHTTPAuth").prop("checked", cs.use_shared_httpauth !== undefined ? cs.use_shared_httpauth : true).trigger("change");
            $("#useSharedSchedule").prop("checked", cs.use_shared_schedule !== undefined ? cs.use_shared_schedule : true).trigger("change");

            // Show/hide sections based on shared settings
            if (useSharedSettings) {
                $("#sharedSettingsSection").show();
            } else {
                $("#sharedSettingsSection").hide();
            }

            // Clear the campaigns list and detail panel
            $("#campaignList").empty();
            $("#campaignDetail").empty();
            $(".campaign-detail-placeholder").show();

            // Load available options
            loadAvailableOptions().then(() => {
                // Set page dropdown if exists
                if (cs.page) {
                    if (cs.page_id) {
                        safelySetSelect2Value("#page", cs.page_id);
                    } else if (cs.page.id) {
                        safelySetSelect2Value("#page", cs.page.id);
                    } else if (cs.page.name) {
                        setSelectByOptionText("#page", cs.page.name);
                    }
                }

                // Copy all campaigns
                const campaignPromises = [];
                if (cs.campaigns && cs.campaigns.length > 0) {
                    cs.campaigns.forEach((campaign, i) => {
                        const promise = addCampaignEntry(i).then(() => {
                            // Set campaign name
                            $(`#campaign_name_${i}`).val(campaign.name);
                            $(`.campaign-list-item[data-index="${i}"] .campaign-list-item-text`).text(campaign.name);
                            
                            // Set campaign type
                            $(`#campaign_type_${i}`).val(campaign.type || "email").trigger("change");

                            setTimeout(() => {
                                if (campaign.type === "email" || !campaign.type) {
                                    if (campaign.template && campaign.template.id) {
                                        safelySetSelect2Value(`#template_${i}`, campaign.template.id);
                                    } else if (campaign.template && campaign.template.name) {
                                        setSelectByOptionText(`#template_${i}`, campaign.template.name);
                                    }

                                    if (campaign.smtp && campaign.smtp.id) {
                                        safelySetSelect2Value(`#profile_${i}`, campaign.smtp.id);
                                    } else if (campaign.smtp && campaign.smtp.name) {
                                        setSelectByOptionText(`#profile_${i}`, campaign.smtp.name);
                                    }
                                } else if (campaign.type === "sms") {
                                    if (campaign.sms_template && campaign.sms_template.id) {
                                        safelySetSelect2Value(`#sms_template_${i}`, campaign.sms_template.id);
                                    } else if (campaign.sms_template && campaign.sms_template.name) {
                                        setSelectByOptionText(`#sms_template_${i}`, campaign.sms_template.name);
                                    }

                                    if (campaign.sms && campaign.sms.id) {
                                        safelySetSelect2Value(`#sms_profile_${i}`, campaign.sms.id);
                                    } else if (campaign.sms && campaign.sms.name) {
                                        setSelectByOptionText(`#sms_profile_${i}`, campaign.sms.name);
                                    }
                                }

                                // Set groups
                                if (campaign.groups && campaign.groups.length > 0) {
                                    const groupIds = campaign.groups.map(g => g.id.toString());
                                    safelySetSelect2Value(`#users_${i}`, groupIds);
                                }

                                // Set campaign-specific settings
                                if (campaign.page_id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page_id);
                                } else if (campaign.page && campaign.page.id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page.id);
                                } else if (campaign.page && campaign.page.name) {
                                    setSelectByOptionText(`#campaign_page_${i}`, campaign.page.name);
                                }

                                $(`#campaign_url_${i}`).val(campaign.url || "");
                                $(`#campaign_urlparam_${i}`).val(campaign.urlparam || "");
                                $(`#campaign_qrsize_${i}`).val(campaign.qrsize || "");
                                $(`#campaign_basicauth_${i}`).prop("checked", campaign.basicauth || false);
                                
                                // Reset dates to empty for copy
                                $(`#campaign_launch_date_${i}`).val("");
                                $(`#campaign_send_by_date_${i}`).val("");
                            }, 300);
                        });
                        campaignPromises.push(promise);
                    });
                } else {
                    addCampaignEntry(0);
                }

                Promise.all(campaignPromises).then(() => {
                    updateCampaignSettingsVisibility();

                    // Set up buttons for new campaign set
                    $("#saveDraftButton").text("Save as Draft");
                    $("#saveDraftButton").off("click").on("click", function () {
                        saveDraftCampaignSet();
                    });

                    $("#launchButton").text("Launch");
                    $("#launchButton").off("click").on("click", function () {
                        launchCampaignSet();
                    });

                    // Make sure all form fields are enabled
                    $("#modal input, #modal select, #modal textarea").prop("disabled", false);
                    $(".btn-remove-campaign, #addCampaignButton").show();
                    $("#saveDraftButton").show();
                    $("#launchButton").show();

                    $("#modal").modal("show");
                });
            });
        })
        .error(function () {
            errorFlash("Error fetching campaign set");
        });
}

// Copies a draft campaign set
function copyDraftCampaignSet(id) {
    // Clear any previous error messages
    $("#modal\\.flashes").empty();

    // Reset the modal title
    $("#modalLabel").text("Copy Draft Campaign Set");
    
    // Get the draft campaign set
    api.draftCampaignSetId.get(id)
        .success(function (dcs) {
            // Reset the tabs to show both
            $(".nav-tabs li").removeClass("active");
            $(".tab-pane").removeClass("active");
            $(".nav-tabs a[href='#generalSettings']").parent().show();
            $(".nav-tabs a[href='#campaignsTab']").parent().show();
            $(".nav-tabs a[href='#generalSettings']").parent().addClass("active");
            $("#generalSettings").addClass("active");
            
            // Clear any previous summary views
            $(".campaign-summary").remove();

            // Set basic form fields with copied data
            $("#name").val("Copy of " + dcs.name);
            $("#url").val(dcs.url || "");
            $("#urlparam").val(dcs.urlparam || "");
            $("#qrsize").val(dcs.qrsize || "");
            $("#basicauth").prop("checked", dcs.basicauth || false);
            
            // Set launch date to current time (not the original)
            $("#launch_date").val(moment().format("MMMM Do YYYY, h:mm a"));
            $("#send_by_date").val("");

            // Set the shared settings checkboxes
            const useSharedSettings = dcs.use_shared_settings !== undefined ? dcs.use_shared_settings : true;
            $("#useSharedSettings").prop("checked", useSharedSettings);
            
            $("#useSharedPage").prop("checked", dcs.use_shared_page !== undefined ? dcs.use_shared_page : true).trigger("change");
            $("#useSharedURL").prop("checked", dcs.use_shared_url !== undefined ? dcs.use_shared_url : true).trigger("change");
            $("#useSharedURLParam").prop("checked", dcs.use_shared_urlparam !== undefined ? dcs.use_shared_urlparam : true).trigger("change");
            $("#useSharedQRSize").prop("checked", dcs.use_shared_qrsize !== undefined ? dcs.use_shared_qrsize : true).trigger("change");
            $("#useSharedHTTPAuth").prop("checked", dcs.use_shared_httpauth !== undefined ? dcs.use_shared_httpauth : true).trigger("change");
            $("#useSharedSchedule").prop("checked", dcs.use_shared_schedule !== undefined ? dcs.use_shared_schedule : true).trigger("change");

            // Show/hide sections based on shared settings
            if (useSharedSettings) {
                $("#sharedSettingsSection").show();
            } else {
                $("#sharedSettingsSection").hide();
            }

            // Clear the campaigns list and detail panel
            $("#campaignList").empty();
            $("#campaignDetail").empty();
            $(".campaign-detail-placeholder").show();

            // Load available options
            loadAvailableOptions().then(() => {
                // Set page dropdown if exists
                if (dcs.page) {
                    if (dcs.page_id) {
                        safelySetSelect2Value("#page", dcs.page_id);
                    } else if (dcs.page.id) {
                        safelySetSelect2Value("#page", dcs.page.id);
                    } else if (dcs.page.name) {
                        setSelectByOptionText("#page", dcs.page.name);
                    }
                }

                // Copy all campaigns
                const campaignPromises = [];
                if (dcs.campaigns && dcs.campaigns.length > 0) {
                    dcs.campaigns.forEach((campaign, i) => {
                        const promise = addCampaignEntry(i).then(() => {
                            // Set campaign name
                            $(`#campaign_name_${i}`).val(campaign.name);
                            $(`.campaign-list-item[data-index="${i}"] .campaign-list-item-text`).text(campaign.name);
                            
                            // Set campaign type
                            $(`#campaign_type_${i}`).val(campaign.type || "email").trigger("change");

                            setTimeout(() => {
                                if (campaign.type === "email" || !campaign.type) {
                                    if (campaign.template_id) {
                                        safelySetSelect2Value(`#template_${i}`, campaign.template_id);
                                    } else if (campaign.template && campaign.template.id) {
                                        safelySetSelect2Value(`#template_${i}`, campaign.template.id);
                                    } else if (campaign.template && campaign.template.name) {
                                        setSelectByOptionText(`#template_${i}`, campaign.template.name);
                                    }

                                    if (campaign.smtp_id) {
                                        safelySetSelect2Value(`#profile_${i}`, campaign.smtp_id);
                                    } else if (campaign.smtp && campaign.smtp.id) {
                                        safelySetSelect2Value(`#profile_${i}`, campaign.smtp.id);
                                    } else if (campaign.smtp && campaign.smtp.name) {
                                        setSelectByOptionText(`#profile_${i}`, campaign.smtp.name);
                                    }
                                } else if (campaign.type === "sms") {
                                    if (campaign.sms_template_id) {
                                        safelySetSelect2Value(`#sms_template_${i}`, campaign.sms_template_id);
                                    } else if (campaign.sms_template && campaign.sms_template.id) {
                                        safelySetSelect2Value(`#sms_template_${i}`, campaign.sms_template.id);
                                    } else if (campaign.sms_template && campaign.sms_template.name) {
                                        setSelectByOptionText(`#sms_template_${i}`, campaign.sms_template.name);
                                    }

                                    if (campaign.sms_id) {
                                        safelySetSelect2Value(`#sms_profile_${i}`, campaign.sms_id);
                                    } else if (campaign.sms && campaign.sms.id) {
                                        safelySetSelect2Value(`#sms_profile_${i}`, campaign.sms.id);
                                    } else if (campaign.sms && campaign.sms.name) {
                                        setSelectByOptionText(`#sms_profile_${i}`, campaign.sms.name);
                                    }
                                }

                                // Set groups
                                if (campaign.groups && campaign.groups.length > 0) {
                                    const groupIds = campaign.groups.map(g => g.id.toString());
                                    safelySetSelect2Value(`#users_${i}`, groupIds);
                                }

                                // Set campaign-specific settings
                                if (campaign.page_id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page_id);
                                } else if (campaign.page && campaign.page.id) {
                                    safelySetSelect2Value(`#campaign_page_${i}`, campaign.page.id);
                                } else if (campaign.page && campaign.page.name) {
                                    setSelectByOptionText(`#campaign_page_${i}`, campaign.page.name);
                                }

                                $(`#campaign_url_${i}`).val(campaign.url || "");
                                $(`#campaign_urlparam_${i}`).val(campaign.urlparam || "");
                                $(`#campaign_qrsize_${i}`).val(campaign.qrsize || "");
                                $(`#campaign_basicauth_${i}`).prop("checked", campaign.basicauth || false);
                                
                                // Reset dates to empty for copy
                                $(`#campaign_launch_date_${i}`).val("");
                                $(`#campaign_send_by_date_${i}`).val("");
                            }, 300);
                        });
                        campaignPromises.push(promise);
                    });
                } else {
                    addCampaignEntry(0);
                }

                Promise.all(campaignPromises).then(() => {
                    updateCampaignSettingsVisibility();

                    // Set up buttons for new campaign set
                    $("#saveDraftButton").text("Save as Draft");
                    $("#saveDraftButton").off("click").on("click", function () {
                        saveDraftCampaignSet();
                    });

                    $("#launchButton").text("Launch");
                    $("#launchButton").off("click").on("click", function () {
                        launchCampaignSet();
                    });

                    // Make sure all form fields are enabled
                    $("#modal input, #modal select, #modal textarea").prop("disabled", false);
                    $(".btn-remove-campaign, #addCampaignButton").show();
                    $("#saveDraftButton").show();
                    $("#launchButton").show();

                    $("#modal").modal("show");
                });
            });
        })
        .error(function () {
            errorFlash("Error fetching draft campaign set");
        });
}

// Deletes a draft campaign set
function deleteDraftCampaignSet(id) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the draft campaign set. This can't be undone!",
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
                api.draftCampaignSetId.delete(id)
                    .success(function (msg) {
                        resolve();
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message);
                    });
            });
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'Draft Campaign Set Deleted!',
                'The draft campaign set has been deleted!',
                'success'
            );
            loadCampaignSets();
        }
    });
}
