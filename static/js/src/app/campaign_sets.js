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

function csPct(n, of) {
    if (!of) return "";
    var p = n / of * 100;
    return (p > 0 && p < 1 ? p.toFixed(1) : Math.round(p)) + "%";
}

function csPlural(n, one, many) {
    return n + " " + (n === 1 ? one : many);
}

// One result card, shared by the set Overview and each campaign's Overview. Sent is a share
// of the targets, everything else a share of sent. A null stats object renders a placeholder.
// opts.ofTotal adds "/ total" after the number, opts.extra is appended inside the card.
function csKpiCard(stats, word, key, icon, cls, label, value, opts) {
    opts = opts || {};
    var total = stats ? stats.total || 0 : 0, sent = stats ? stats.sent || 0 : 0;
    var v = stats ? (value || 0) : null;
    var state = (key === "submitted" && v) ? " cs-kpi-alert" : (key === "reported" && v) ? " cs-kpi-good" : "";
    var pct = v === null ? "" : key === "sent" ? csPct(v, total) : csPct(v, sent);
    // The number and its share share the top line, so the label below gets the full width
    return '<div class="cs-kpi cs-kpi-' + key + state + '">' +
        '<span class="cs-kpi-ico ' + cls + '"><i class="fa ' + icon + '" aria-hidden="true"></i></span>' +
        '<div class="cs-kpi-main"><div class="cs-kpi-top">' +
        '<span class="cs-kpi-n">' + (v === null ? "–" : v) +
        (opts.ofTotal && v !== null ? '<small> / ' + total + '</small>' : '') + '</span>' +
        (pct ? '<span class="cs-kpi-pct">' + pct + '<small> of ' + (key === "sent" ? word + "s" : "sent") + '</small></span>' : '') +
        '</div><div class="cs-kpi-l">' + label + '</div></div>' +
        (opts.extra || '') +
        '</div>';
}

// The result cards for the whole set, in one of the two stat modes. Same cards and order as
// each campaign's Overview.
//
// "totals" sums across campaigns (someone in two campaigns counts twice);
// "unique" dedups by contact.
function renderOverviewKpis(summary, mode) {
    var s = mode === "unique" ? summary.unique_stats : summary.stats;
    var word = mode === "unique" ? "contact" : "target";
    var total = s.total || 0, sent = s.sent || 0;
    // Opens, attachments, macros, replies and reports only exist for email
    var hasEmail = !summary.campaigns || summary.campaigns.some(function (c) { return c.type !== "sms"; });

    function card(key, icon, cls, label, value, opts) {
        return csKpiCard(s, word, key, icon, cls, label, value, opts);
    }

    var cards = card("sent", "fa-paper-plane", "cs-stat-sent", "Sent", sent, {
            ofTotal: true,
            extra: '<div class="cs-kpi-bar"><span style="width:' + (total ? Math.min(100, sent / total * 100) : 0).toFixed(1) + '%"></span></div>'
        }) +
        (hasEmail ? card("opened", "fa-envelope-open-o", "cs-stat-opened", "Opened", s.opened) : "") +
        card("clicked", "fa-mouse-pointer", "cs-stat-clicked", "Clicked", s.clicked) +
        card("submitted", "fa-exclamation-circle", "cs-stat-submitted", "Submitted", s.submitted_data) +
        (hasEmail ?
            card("attachment", "fa-paperclip", "cs-stat-attachment-opened", "Attachment opened", s.opened_attachment) +
            card("macro", "fa-file-code-o", "cs-stat-macro-executed", "Macro executed", s.macro_executed) +
            card("replied", "fa-reply", "cs-stat-replied", "Replied", s.replied) +
            card("reported", "fa-bullhorn", "cs-stat-reported", "Reported", s.email_reported) : "");

    $("#overviewKpis").html('<div class="cs-kpis cs-kpis-campaign cs-kpis-set' + (hasEmail ? "" : " cs-kpis-3") + '">' + cards + '</div>');
}

// Renders the whole Overview tab from a campaign set summary response.
function renderCampaignSetOverview(summary) {
    currentSetSummary = summary;

    var launched = summary.launch_date && summary.launch_date !== "0001-01-01T00:00:00Z"
        ? moment(summary.launch_date).format("MMM Do YYYY, h:mm a")
        : "Not scheduled";
    var sendBy = summary.send_by_date && summary.send_by_date !== "0001-01-01T00:00:00Z"
        ? moment(summary.send_by_date).format("MMM Do YYYY, h:mm a")
        : "";

    var status = summary.status || "Unknown";
    function chip(icon, text) {
        return '<span class="cs-ov-chip"><i class="fa ' + icon + '" aria-hidden="true"></i> ' + text + '</span>';
    }

    // The set name is already in the modal title, so the header starts at the status
    $("#overviewHeader").html(
        '<div class="cs-ov-summary">' +
        '<span class="label cs-ov-status ' + (CS_STATUS_LABELS[status] || "label-default") + '">' + escapeHtml(status) + '</span>' +
        chip("fa-th-large", summary.campaign_count + ' campaign' + (summary.campaign_count === 1 ? '' : 's')) +
        chip("fa-calendar", 'Launched <strong>' + launched + '</strong>') +
        (sendBy ? chip("fa-clock-o", 'Send by <strong>' + sendBy + '</strong>') : '') +
        '</div>'
    );

    // Reset the toggle to Totals on every open.
    $("#overviewStatsToggle button").removeClass("active btn-primary").addClass("btn-default");
    $('#overviewStatsToggle button[data-mode="totals"]').addClass("active btn-primary").removeClass("btn-default");
    renderOverviewKpis(summary, "totals");

    // Initialize the stats info tooltip. This is static template markup, so there
    // is no page-load init for it; init here (idempotent) when the tab is populated.
    $("#overviewStatsInfo").tooltip();

    renderOverviewCampaigns(summary);
}

// The dashboard's stage dots (styles in main.css): filled and coloured when the stage
// happened, hollow when not, dotted when the campaign type cannot track it
var CS_STAGES = [
    { key: "sent", hex: "#1abc9c", label: "Sent", icon: "fa-envelope-o" },
    { key: "opened", hex: "#f9bf3b", label: "Opened", icon: "fa-envelope-open-o", emailOnly: true },
    { key: "clicked", hex: "#F39C12", label: "Clicked", icon: "fa-mouse-pointer" },
    { key: "submitted_data", hex: "#f05b4f", label: "Submitted", icon: "fa-exclamation-circle" },
    { key: "opened_attachment", hex: "#f0ad4e", label: "Attachment Opened", icon: "fa-paperclip", emailOnly: true },
    { key: "macro_executed", hex: "#c0392b", label: "Macro Executed", icon: "fa-file-code-o", emailOnly: true },
    { key: "replied", hex: "#E67E22", label: "Replied", icon: "fa-reply", emailOnly: true },
    { key: "email_reported", hex: "#45d6ef", label: "Reported", icon: "fa-bullhorn", emailOnly: true }
];

function csStageDots(campaign) {
    var stats = campaign.stats || {};
    var dots = "", tip = [];
    CS_STAGES.forEach(function (s) {
        if (s.emailOnly && campaign.type === "sms") {
            dots += '<span class="stage-cell"><span class="stage-dot stage-dot-na"></span></span>';
            tip.push(s.label + ": n/a");
            return;
        }
        var n = stats[s.key] || 0;
        dots += '<span class="stage-cell">' + (n > 0
            ? '<span class="stage-dot" style="background:' + s.hex + ';border-color:' + s.hex + '"></span>'
            : '<span class="stage-dot stage-dot-empty"></span>') + '</span>';
        tip.push(s.label + ": " + n);
    });
    return '<span class="stage-dots" data-toggle="tooltip" data-html="true" data-placement="top" title="' + tip.join("<br>") + '">' + dots + '</span>';
}

// Every campaign in the set on one row each, so they can be compared at a glance
function renderOverviewCampaigns(summary) {
    var campaigns = summary.campaigns || [];
    if (!campaigns.length) {
        $("#overviewCampaigns").empty();
        return;
    }
    var legend = '<span class="stage-legend">' + CS_STAGES.map(function (s) {
        return '<span class="stage-cell"><i class="fa ' + s.icon + '" title="' + s.label + '"></i></span>';
    }).join("") + '</span>';
    var rows = campaigns.map(function (c) {
        var stats = c.stats || {};
        var isSms = c.type === "sms";
        return '<tr>' +
            '<td><button type="button" class="btn btn-link cs-ov-open" data-campaign-id="' + c.id + '" title="Open on the Campaigns tab">' +
            '<i class="fa ' + (isSms ? "fa-mobile" : "fa-envelope") + '" aria-hidden="true"></i> ' + escapeHtml(c.name) + '</button></td>' +
            '<td><span class="label ' + (CS_STATUS_LABELS[c.status] || "label-default") + '">' + escapeHtml(c.status || "Unknown") + '</span></td>' +
            '<td class="cs-ov-num">' + (stats.sent || 0) + ' / ' + (stats.total || 0) + '</td>' +
            '<td>' + csStageDots(c) + '</td>' +
            '<td class="text-right"><a class="btn btn-primary btn-sm" href="/campaigns/' + c.id + '" title="View results" aria-label="View results for ' +
            escapeHtml(c.name) + '"><i class="fa fa-bar-chart" aria-hidden="true"></i></a></td>' +
            '</tr>';
    }).join("");
    $("#overviewCampaigns").html(
        '<h5 class="cs-ds-h">Campaigns</h5>' +
        '<table class="table cs-ov-table"><thead><tr>' +
        '<th>Campaign</th><th>Status</th><th class="cs-ov-num">Sent</th><th>' + legend + '</th><th></th>' +
        '</tr></thead><tbody>' + rows + '</tbody></table>');
    $('#overviewCampaigns [data-toggle="tooltip"]').tooltip({ container: "body" });
}

// A campaign name in the Overview list opens that campaign on the Campaigns tab
$(document).on("click", ".cs-ov-open", function () {
    var id = $(this).data("campaign-id");
    $('a[href="#campaignsTab"]').tab("show");
    $('.campaign-list-item[data-campaign-id="' + id + '"]').trigger("click");
});

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
        // Locked, not only dimmed: the controls cannot be clicked, typed in or tabbed to
        $fieldCol.find("input, select, textarea, button").prop("disabled", !isChecked);
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
                applyURLTemplate(currentUrlFieldTarget, template.url);
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
    // Categories keep the order the server sends
    Object.keys(grouped).forEach(function(category) {
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
        $("#send_order").val("sequential");
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
        // Taken before the new campaign becomes the selected one. With none selected yet,
        // the last campaign is the natural one to start from.
        const activeIndex = $(".campaign-list-item.active").data("index");
        // .data() gives null, not undefined, when nothing is selected
        const $source = activeIndex != null ? $(`#campaign_form_${activeIndex}`) : $(".campaign-detail-form").last();
        const optionsLoaded = addCampaignEntry(campaignCount);

        // Select the newly added campaign
        $(`.campaign-list-item[data-index="${campaignCount}"]`).trigger("click");

        if ($source.length && optionsLoaded && optionsLoaded.then) {
            optionsLoaded.then(() => csCopyCampaignSetup($source, campaignCount));
        }
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
        // Schedule is three rows under one toggle, so its whole group dims together
        var $row   = $btn.closest(".cs-setting-group");
        if (!$row.length) $row = $btn.closest(".inline-setting-row");

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
        
        // The shared field stays in place and is locked by syncPillsFromCheckboxes, so the
        // toggle beside it never moves
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
        
        // Per-campaign settings stay in place, locked, so the toggles never move
        syncPillsFromCheckboxes();
        
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

        // The sections above were shown as a whole, so hide the ones left with no fields
        csUpdateOverrideHeads();
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
                                "<div class='pull-right cs-row-actions'><button class='btn btn-info' onclick='viewCampaignSet(" + campaignSet.id + ")'><i class='fa fa-eye'></i></button><button class='btn btn-warning' onclick='copyCampaignSet(" + campaignSet.id + ")'><i class='fa fa-copy'></i></button><button class='btn btn-success' onclick='completeCampaignSet(" + campaignSet.id + ")' " + completeButtonDisabled + "><i class='fa fa-check'></i></button><button class='btn btn-danger' onclick='deleteCampaignSet(" + campaignSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
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
                                "<div class='pull-right cs-row-actions'><button class='btn btn-success' data-toggle='modal' data-target='#launchDraftModal' onclick='prepareLaunchDraft(" + draftSet.id + ")'><i class='fa fa-rocket'></i></button><button class='btn btn-warning' onclick='copyDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-copy'></i></button><button class='btn btn-primary' onclick='editDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-pencil'></i></button><button class='btn btn-danger' onclick='deleteDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
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
                                "<div class='pull-right cs-row-actions'><button class='btn btn-info' onclick='viewCampaignSet(" + campaignSet.id + ")'><i class='fa fa-eye'></i></button><button class='btn btn-success' onclick='completeCampaignSet(" + campaignSet.id + ")' " + completeButtonDisabled + "><i class='fa fa-check'></i></button><button class='btn btn-danger' onclick='deleteCampaignSet(" + campaignSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
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
                                "<div class='pull-right cs-row-actions'><button class='btn btn-success' data-toggle='modal' data-target='#launchDraftModal' onclick='prepareLaunchDraft(" + draftSet.id + ")'><i class='fa fa-rocket'></i></button><button class='btn btn-primary' onclick='editDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-pencil'></i></button><button class='btn btn-danger' onclick='deleteDraftCampaignSet(" + draftSet.id + ")'><i class='fa fa-trash-o'></i></button></div>"
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
            <section class="cs-cf-section">
                <button type="button" class="cs-cf-head" aria-expanded="true">
                    <i class="fa fa-cog cs-cf-ico" aria-hidden="true"></i><span class="cs-cf-title">General</span>
                    <span class="cs-cf-hint">Name, Type</span><i class="fa fa-chevron-up cs-cf-chev" aria-hidden="true"></i>
                </button>
                <div class="cs-cf-body cs-cf-row">
                    <div class="form-group cs-cf-grow">
                        <label class="control-label" for="campaign_name_${index}">Name <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <input type="text" class="form-control campaign-name" placeholder="Campaign name" id="campaign_name_${index}" />
                    </div>

                    <!-- Type: hidden select (drives all logic) + btn-group identical to campaigns.html -->
                    <select class="campaign-type" id="campaign_type_${index}" style="display:none;">
                        <option value="email">Email</option>
                        <option value="sms">SMS</option>
                    </select>
                    <div class="form-group">
                        <label class="control-label">Type <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <div>
                            <div class="btn-group cs-cf-type" role="group">
                                <button type="button" class="btn btn-primary cs-type-btn active" data-type="email">
                                    <i class="fa fa-envelope"></i> Email
                                </button>
                                <button type="button" class="btn btn-default cs-type-btn" data-type="sms">
                                    <i class="fa fa-mobile"></i> SMS
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </section>

            <section class="cs-cf-section">
                <button type="button" class="cs-cf-head" aria-expanded="true">
                    <i class="fa fa-envelope-o cs-cf-ico" aria-hidden="true"></i><span class="cs-cf-title">Content</span>
                    <span class="cs-cf-hint">Template, Sending Profile</span><i class="fa fa-chevron-up cs-cf-chev" aria-hidden="true"></i>
                </button>
                <div class="cs-cf-body cs-cf-cols">
                    <div class="form-group email-template-group">
                        <label class="control-label" for="template_${index}">Email Template <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <select class="form-control email-template" placeholder="Template" id="template_${index}">
                        </select>
                    </div>

                    <div class="form-group sms-template-group" style="display:none;">
                        <label class="control-label" for="sms_template_${index}">SMS Template <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <select class="form-control sms-template" placeholder="SMS Template" id="sms_template_${index}">
                        </select>
                    </div>

                    <div class="form-group email-profile-group">
                        <label class="control-label" for="profile_${index}">Sending Profile <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <select class="form-control email-profile" placeholder="Sending Profile" id="profile_${index}">
                        </select>
                    </div>

                    <div class="form-group sms-profile-group" style="display:none;">
                        <label class="control-label" for="sms_profile_${index}">SMS Sending Profile <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <select class="form-control sms-profile" placeholder="SMS Sending Profile" id="sms_profile_${index}">
                        </select>
                    </div>
                </div>
            </section>

            <section class="cs-cf-section">
                <button type="button" class="cs-cf-head" aria-expanded="true">
                    <i class="fa fa-users cs-cf-ico" aria-hidden="true"></i><span class="cs-cf-title">Targeting</span>
                    <span class="cs-cf-hint">Groups, Send order</span><i class="fa fa-chevron-up cs-cf-chev" aria-hidden="true"></i>
                </button>
                <div class="cs-cf-body">
                    <div class="form-group">
                        <label class="control-label" for="users_${index}">Groups <span class="cs-cf-req" aria-hidden="true">*</span></label>
                        <select class="form-control groups" id="users_${index}" multiple="multiple">
                        </select>
                        <p class="cs-groups-count" id="groups_count_${index}"></p>
                    </div>

                    <div class="form-group cs-cf-check">
                        <input class="form-check-input campaign-randomize-send-order" type="checkbox" id="campaign_randomize_send_order_${index}">
                        <label class="form-check-label" for="campaign_randomize_send_order_${index}"> Randomize send order
                            <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Sends messages to recipients in a random order instead of the order they appear in the target groups"></i>
                        </label>
                    </div>
                </div>
            </section>

            <!-- Only shown when something in it is set per campaign in Setup -->
            <section class="cs-cf-section campaign-settings-override" style="display:none;">
                <button type="button" class="cs-cf-head" aria-expanded="true">
                    <i class="fa fa-link cs-cf-ico" aria-hidden="true"></i><span class="cs-cf-title">URL &amp; Tracking</span>
                    <span class="cs-cf-hint">Set for this campaign</span><i class="fa fa-chevron-up cs-cf-chev" aria-hidden="true"></i>
                </button>
                <div class="cs-cf-body">
                    <div class="form-group">
                        <label class="control-label" for="campaign_page_${index}">Landing Page</label>
                        <select class="form-control campaign-page" placeholder="Landing Page" id="campaign_page_${index}">
                        </select>
                    </div>
                    <div class="cs-cf-cols cs-cf-cols-3">
                        <div class="form-group">
                            <label class="control-label" for="campaign_url_${index}">URL
                                <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Location of Gophish listener (must be reachable by targets!)"></i>
                                <span id="campaign_url_${index}_length" style="margin-left: 10px; font-size: 12px;"></span>
                            </label>
                            <div class="input-group">
                                <input type="text" class="form-control campaign-url" placeholder="http://192.168.1.1" id="campaign_url_${index}" />
                                <span class="input-group-btn">
                                    <button type="button" class="btn btn-primary campaign-url-template-btn" data-campaign-index="${index}" data-toggle="tooltip" title="Use URL Template">
                                        <i class="fa fa-list"></i>
                                    </button>
                                </span>
                            </div>
                        </div>
                        <div class="form-group">
                            <label class="control-label" for="campaign_urlparam_${index}">URL Parameter
                                <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Sets the parameter used in the URL to track the target. It is recommended to change this parameter, to make Gophish stealthier. Leave blank to use default (rid)."></i>
                            </label>
                            <input type="text" class="form-control campaign-urlparam" placeholder="rid" id="campaign_urlparam_${index}" />
                        </div>
                        <div class="form-group">
                            <label class="control-label" for="campaign_qrsize_${index}">QR Code Size
                                <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Size of QR code images (integer is height & width). Leave blank to not include QR code images and use normal links."></i>
                            </label>
                            <input type="number" class="form-control campaign-qrsize" placeholder="256" id="campaign_qrsize_${index}" />
                        </div>
                    </div>
                    <div class="form-group cs-cf-check">
                        <input class="form-check-input campaign-basicauth" type="checkbox" id="campaign_basicauth_${index}">
                        <label class="form-check-label" for="campaign_basicauth_${index}"> Use HTTP Basic Access Authentication
                            <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="Enables a landing page with HTTP Authentication"></i>
                        </label>
                    </div>
                </div>
            </section>

            <section class="cs-cf-section campaign-settings-override" style="display:none;">
                <button type="button" class="cs-cf-head" aria-expanded="true">
                    <i class="fa fa-calendar cs-cf-ico" aria-hidden="true"></i><span class="cs-cf-title">Schedule</span>
                    <span class="cs-cf-hint">Set for this campaign</span><i class="fa fa-chevron-up cs-cf-chev" aria-hidden="true"></i>
                </button>
                <div class="cs-cf-body cs-cf-cols">
                    <div class="form-group">
                        <label class="control-label" for="campaign_launch_date_${index}">Launch Date</label>
                        <input type="text" class="form-control campaign-launch-date" id="campaign_launch_date_${index}" />
                    </div>
                    <div class="form-group">
                        <label class="control-label" for="campaign_send_by_date_${index}">Send Emails By <span class="cs-set-optional">(Optional)</span>
                            <i class="fa fa-question-circle" data-toggle="tooltip" data-placement="right" title="If specified, Gophish will send messages evenly between the campaign launch and this date."></i>
                        </label>
                        <input type="text" class="form-control campaign-send-by-date" id="campaign_send_by_date_${index}" />
                    </div>
                </div>
            </section>
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

    $(`#campaign_form_${index} [data-toggle="tooltip"]`).tooltip({ container: "body" });

    // Load options for just this campaign entry
    return loadOptionsForCampaign(index);
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
        data.groups.forEach(g => $group.append(`<option value="${g.id}" data-targets="${(g.targets || []).length}">${g.name}</option>`));
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
        campaign.randomize_send_order = $(this).find(".campaign-randomize-send-order").is(":checked");

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
        send_order: $("#send_order").val() || "sequential",
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
        campaign.randomize_send_order = $(this).find(".campaign-randomize-send-order").is(":checked");

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
        send_order: $("#send_order").val() || "sequential",
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

            // Restore the send order (defaults to sequential)
            $("#send_order").val(dcs.send_order || "sequential");

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
            
            // Per-campaign settings stay in place, locked, so the toggles never move
            syncPillsFromCheckboxes();

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
                                $(`#campaign_randomize_send_order_${i}`).prop("checked", campaign.randomize_send_order || false);
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
        campaign.randomize_send_order = $(this).find(".campaign-randomize-send-order").is(":checked");

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
        send_order: $("#send_order").val() || "sequential",
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
        // Left set, the edit and copy flows would otherwise treat Delete as a launched-set delete
        $("#modal").data("viewMode", false).removeClass("cs-viewing");
        $("#campaignDetail").removeData("csCampaign");
        $(".cs-li-search, #csListEmpty").remove();
        csFullOpen = false;
        $("#csPvFull").hide();
        $("#csPreviewFullMeta, #csPreviewFullBody").empty();
        csPreviewSeq++;
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
            $("#send_order").val(cs.send_order || "sequential");

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
                $("#modal").data("viewMode", true).addClass("cs-viewing");
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
                        // Status and type identify a campaign, the numbers live in the detail panel
                        const typeLabel = campaign.type === "sms" ? "SMS" : "Email";
                        const status = campaign.status || "";

                        const campaignListItem = `
                            <a href="#" class="list-group-item campaign-list-item" data-index="${i}" data-type="${campaign.type}" data-campaign-id="${campaign.id}" title="${escapeHtml(campaign.name)}">
                                <span class="campaign-list-item-icon"><i class="fa ${icon}" aria-hidden="true"></i></span>
                                <span class="campaign-list-item-text">${escapeHtml(campaign.name)}</span>
                                <div class="cs-li-meta">
                                    ${status ? `<span class="label cs-li-status ${CS_STATUS_LABELS[status] || "label-default"}">${escapeHtml(status)}</span>` : ""}
                                    <span class="cs-li-type">${typeLabel}</span>
                                </div>
                            </a>
                        `;
                        $("#campaignList").append(campaignListItem);
                    });

                    $(".cs-li-search, #csListEmpty").remove();
                    if (cs.campaigns.length >= CS_SEARCH_THRESHOLD) {
                        $("#campaignList").before(
                            '<div class="cs-li-search"><i class="fa fa-search" aria-hidden="true"></i>' +
                            '<input type="search" class="form-control input-sm" id="csCampaignSearch" placeholder="Search campaigns" aria-label="Search campaigns"></div>');
                        $("#campaignList").after('<p class="cs-li-empty" id="csListEmpty" style="display:none;">No campaigns match.</p>');
                    }

                    // One request supplies the Overview tab AND every list badge,
                    // replacing the previous per-campaign summary calls.
                    api.campaignSetId.summary(cs.id)
                        .success(function (summary) {
                            renderCampaignSetOverview(summary);

                            campaignSetSummaryById = {};
                            (summary.campaigns || []).forEach(function (campaign) {
                                campaignSetSummaryById[campaign.id] = campaign;
                            });

                            // The detail panel rendered before this response, with its numbers
                            // still empty. Re-render the selected campaign so they fill in.
                            // showCampaignSummary() replaces the old panel rather than stacking.
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
// Same colours as the Campaigns list
var CS_STATUS_LABELS = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger"
};

// The detail tab and preview kind are kept while switching campaigns
var csDetailTab = "overview";
var csPreviewMode = "message";
// Bumped on every render so a slow response for a previous campaign is dropped
var csPreviewSeq = 0;
var csPreviewCache = {};
// Whether the full-screen preview is open, so renders also fill it
var csFullOpen = false;
// Shows the search box once the list is long enough to need one
var CS_SEARCH_THRESHOLD = 6;

function csValidDate(d) {
    return d && d !== "0001-01-01T00:00:00Z";
}

// The api.* helpers are synchronous, which would freeze the modal while a preview loads
function csFetch(kind, id) {
    var key = kind + ":" + id;
    if (csPreviewCache[key]) return csPreviewCache[key];
    var paths = { template: "/templates/", sms: "/sms_templates/", page: "/pages/" };
    var request = $.ajax({
        url: "/api" + paths[kind] + id,
        method: "GET",
        dataType: "json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader("Authorization", "Bearer " + user.api_key);
        }
    });
    // Cache before attaching fail, or a request that fails at once stays cached
    csPreviewCache[key] = request;
    request.fail(function () {
        if (csPreviewCache[key] === request) delete csPreviewCache[key];
    });
    return request;
}

function csSampleData() {
    var now = new Date();
    return {
        FirstName: "John",
        LastName: "Doe",
        Email: "john.doe@example.com",
        Phone: "+15551234567",
        Position: "IT Manager",
        Custom: "Custom value",
        From: "Phishing Team",
        URL: "https://example.com?rid=abc12345",
        TrackingURL: "https://example.com/track?rid=abc12345",
        Tracker: "",
        QR: "[QR code]",
        RId: "abc12345",
        BaseURL: "https://example.com",
        CurrentDateTime: now.toLocaleString("en-US", { month: "short", day: "numeric", year: "numeric", hour: "numeric", minute: "2-digit", hour12: true }),
        CurrentDate: now.toLocaleString("en-US", { month: "long", day: "numeric", year: "numeric" }),
        CurrentTime: now.toLocaleString("en-US", { hour: "numeric", minute: "2-digit", hour12: true }),
        CurrentTime24: now.toLocaleString("en-GB", { hour: "2-digit", minute: "2-digit", hour12: false })
    };
}

function csFillSample(s, url) {
    var data = csSampleData();
    if (url) data.URL = url;
    $.each(data, function (key, value) {
        s = s.replace(new RegExp("{{\\s*\\." + key + "\\s*}}", "g"), value);
    });
    return s;
}

// The phished link as a target would see it, with a sample rid
function csSampleUrl(campaign) {
    var base = $.trim(campaign.url || "");
    if (!base) return "";
    var param = $.trim(campaign.urlparam || "") || "rid";
    return base + (base.indexOf("?") === -1 ? "?" : "&") + encodeURIComponent(param) + "=abc12345";
}

// Previews are pictures of the message: nothing inside can be clicked, focused or typed in
function csWriteFrame(frame, html) {
    frame.setAttribute("inert", "");
    frame.setAttribute("tabindex", "-1");
    frame.setAttribute("aria-hidden", "true");
    frame.setAttribute("scrolling", "no");
    var doc = frame.contentDocument || frame.contentWindow.document;
    doc.open();
    doc.write(html);
    doc.close();
    // The frame is sized to its content, so its own scrollbars would only get in the way
    var style = doc.createElement("style");
    style.textContent = "html,body{overflow:hidden !important}";
    (doc.head || doc.documentElement).appendChild(style);
    // Images and fonts change the size once they load
    if (frame.contentWindow) frame.contentWindow.addEventListener("load", csFitFrames);
    csFitFrames();
    setTimeout(csFitFrames, 300);
}

function csContentHeight(frame) {
    try {
        var doc = frame.contentDocument;
        return Math.max(doc.documentElement.scrollHeight, doc.body ? doc.body.scrollHeight : 0);
    } catch (e) {
        return 0;
    }
}

function csSafeHtml(html, url) {
    return disableLinksOnly(sanitizeHTML(csFillSample(html || "", url)));
}

function csPreviewMessage(text) {
    return '<div class="cs-pv-message"><i class="fa fa-info-circle" aria-hidden="true"></i> ' + text + '</div>';
}

function csLoading(what) {
    return '<div class="cs-pv-loading"><i class="fa fa-spinner fa-spin" aria-hidden="true"></i> Loading ' + what + '</div>';
}

// Landing pages are laid out at desktop width. Emails use their own width, at least the panel's.
var CS_PAGE_WIDTH = 1280;

// Each frame is drawn at full size and scaled down to the panel width, so it never
// scrolls itself. Only the box around it scrolls, like a long screenshot.
function csFitFrames() {
    $(".cs-pv-canvas").each(function () {
        var canvas = this;
        var frame = $(canvas).find("iframe")[0];
        var width = canvas.clientWidth;
        if (!frame || !width || !frame.contentDocument) return;
        var desktop = $(canvas).data("layout") === "desktop";

        frame.style.transform = "none";
        frame.style.width = (desktop ? CS_PAGE_WIDTH : width) + "px";
        var layoutWidth = desktop ? CS_PAGE_WIDTH :
            Math.max(width, frame.contentDocument.documentElement.scrollWidth);
        frame.style.width = layoutWidth + "px";
        // Shrink first, or scrollHeight reports the old frame height instead of the content
        frame.style.height = "1px";
        var scale = Math.min(1, width / layoutWidth);
        var fill = desktop ? $(canvas).parent().height() / scale : 0;
        var h = Math.max(csContentHeight(frame), fill);

        frame.style.height = h + "px";
        frame.style.transform = scale < 1 ? "scale(" + scale + ")" : "none";
        canvas.style.height = Math.ceil(h * scale) + "px";
    });
}

// Metadata as labelled columns side by side, each pair is [label, html value]
function csMetaCols(cols) {
    return '<div class="cs-pv-meta-cols">' + cols.map(function (c) {
        return '<div class="cs-pv-meta-col"><span class="cs-pv-k">' + c[0] + '</span><span class="cs-pv-v">' + c[1] + '</span></div>';
    }).join("") + '</div>';
}

// Each renderer fills two places: the metadata card and the rendered preview below it
function csRenderEmail(v, campaign, seq) {
    if (!campaign.template || !campaign.template.id) {
        v.meta.empty();
        v.body.html(csPreviewMessage("The email template for this campaign was deleted."));
        return;
    }
    v.meta.empty();
    v.body.html(csLoading("email"));
    csFetch("template", campaign.template.id).done(function (t) {
        if (seq !== csPreviewSeq) return;
        var url = csSampleUrl(campaign);
        var from = $.trim(t.envelope_sender || "") ||
            (campaign.smtp && campaign.smtp.name ? "From address of " + campaign.smtp.name : "From address of the sending profile");
        // Just a count here, the Attachment stop on the path lists the files
        var files = t.attachments || [];
        var fileCount = '<span title="' + escapeHtml(files.map(function (a) { return a.name; }).join(", ")) + '">' +
            '<i class="fa fa-paperclip" aria-hidden="true"></i> ' + files.length + (files.length === 1 ? " file" : " files") + '</span>';
        v.meta.html(
            '<div class="cs-pv-subject">' + escapeHtml(csFillSample(t.subject || "") || "(No subject)") + '</div>' +
            csMetaCols([
                ["From", escapeHtml(csFillSample(from))],
                ["To", "John Doe &lt;john.doe@example.com&gt;"]
            ].concat(files.length ? [["Attachments", fileCount]] : [])));
        v.body.html('<div class="cs-pv-scroll cs-pv-paper"><div class="cs-pv-canvas"><iframe sandbox="allow-same-origin" title="Email preview"></iframe></div></div>');
        var html = $.trim(t.html || "") ? t.html :
            "<pre style='white-space:pre-wrap;font-family:sans-serif'>" + escapeHtml(t.text || "") + "</pre>";
        csWriteFrame(v.body.find("iframe")[0], csSafeHtml(html, url));
    }).fail(function () {
        if (seq !== csPreviewSeq) return;
        v.body.html(csPreviewMessage("Could not load the email template."));
    });
}

function csRenderSms(v, campaign, seq) {
    v.meta.empty();
    if (!campaign.sms_template || !campaign.sms_template.id) {
        v.body.html(csPreviewMessage("The SMS template for this campaign was deleted."));
        return;
    }
    v.body.html(csLoading("message"));
    csFetch("sms", campaign.sms_template.id).done(function (t) {
        if (seq !== csPreviewSeq) return;
        var from = $.trim(t.from || "");
        v.meta.html(csMetaCols([
            ["From", escapeHtml(from || "Sender set by the SMS profile")],
            ["To", "+15551234567"]
        ]));
        var text = escapeHtml(csFillSample(t.text || "", csSampleUrl(campaign)))
            .replace(/(https?:\/\/[^\s<]+)/g, '<span class="st-link">$1</span>');
        var clock = new Date().toLocaleString("en-US", { hour: "numeric", minute: "2-digit", hour12: true });
        // Just the conversation, the phone around it only cost space here
        v.body.html(
            '<div class="cs-pv-sms"><div class="st-thread">' +
            '<div class="st-day"><span>Today ' + clock + '</span></div>' +
            '<div class="st-bubble">' + text + '</div>' +
            '<div class="st-read">Read ' + clock + '</div>' +
            '</div></div>');
    }).fail(function () {
        if (seq !== csPreviewSeq) return;
        v.body.html(csPreviewMessage("Could not load the SMS template."));
    });
}

function csRenderPage(v, campaign, seq) {
    var url = csSampleUrl(campaign);
    v.meta.html(csMetaCols([
        ["Page", escapeHtml(campaign.page && campaign.page.name ? campaign.page.name : "None")],
        ["Link", escapeHtml(url || "No URL set")]
    ]));
    if (!campaign.page || !campaign.page.id) {
        v.body.html(csPreviewMessage(campaign.page && campaign.page.name === "[Deleted]" ?
            "The landing page for this campaign was deleted." : "This campaign has no landing page."));
        return;
    }
    v.body.html(csLoading("landing page"));
    csFetch("page", campaign.page.id).done(function (p) {
        if (seq !== csPreviewSeq) return;
        v.body.html(
            '<div class="cs-pv-browser">' +
            '<div class="cs-pv-browser-bar"><span class="cs-pv-dots"><i></i><i></i><i></i></span>' +
            '<span class="cs-pv-address"><i class="fa fa-lock" aria-hidden="true"></i> ' + escapeHtml(url || "No URL set") + '</span></div>' +
            '<div class="cs-pv-scroll cs-pv-browser-body"><div class="cs-pv-canvas" data-layout="desktop"><iframe sandbox="allow-same-origin" title="Landing page preview"></iframe></div></div>' +
            '</div>');
        csWriteFrame(v.body.find("iframe")[0], csSafeHtml(p.html, url));
    }).fail(function () {
        if (seq !== csPreviewSeq) return;
        v.body.html(csPreviewMessage("Could not load the landing page."));
    });
}

// The full-screen overlay mirrors the inline preview, so both render through here
function csPreviewViews() {
    var views = [{ meta: $("#csPreviewMeta"), body: $("#csPreviewBody") }];
    if (csFullOpen) views.push({ meta: $("#csPreviewFullMeta"), body: $("#csPreviewFullBody") });
    return views;
}

function csRenderPreview(campaign) {
    // SMS has no attachment stop to show
    if (csPreviewMode === "attachment" && campaign.type === "sms") csPreviewMode = "message";
    var seq = ++csPreviewSeq;
    $(".cs-flow-step[data-mode]").each(function () {
        var on = $(this).data("mode") === csPreviewMode;
        $(this).toggleClass("active", on).attr("aria-pressed", on ? "true" : "false");
    });
    csPreviewViews().forEach(function (v) {
        if (csPreviewMode === "page") {
            csRenderPage(v, campaign, seq);
        } else if (csPreviewMode === "attachment") {
            csRenderAttachments(v, campaign, seq);
        } else if (campaign.type === "sms") {
            csRenderSms(v, campaign, seq);
        } else {
            csRenderEmail(v, campaign, seq);
        }
    });
}

function csShowDetailTab(tab) {
    if (tab !== "overview" && tab !== "preview") tab = "overview";
    csDetailTab = tab;
    $(".cs-dt-tab").each(function () {
        var on = $(this).data("tab") === tab;
        $(this).toggleClass("active", on).attr("aria-selected", on ? "true" : "false").attr("tabindex", on ? "0" : "-1");
    });
    $(".cs-dt-pane").each(function () {
        $(this).toggleClass("active", $(this).data("tab") === tab);
    });
    // Frames measure 0 while their pane is hidden
    if (tab === "preview") csFitFrames();
}

function csHost(url) {
    var m = /^[a-z]+:\/\/([^/?#]+)/i.exec(url || "");
    return m ? m[1] : "";
}

function csCount(n, word) {
    return '<b>' + n + '</b> ' + word;
}

var CS_MACRO_FILE = /\.(docm|dotm|xlsm|xltm|xlam|xlsb|pptm|potm|ppsm|ppam)$/i;

function csFlowStep(attrs, icon, key, value, sub, extraClass) {
    var tag = attrs ? "button" : "div";
    return '<' + tag + (attrs ? ' type="button" ' + attrs : '') + ' class="cs-flow-step' + (extraClass || '') + '">' +
        '<span class="cs-flow-ico"><i class="fa ' + icon + '" aria-hidden="true"></i></span>' +
        '<span class="cs-flow-txt"><span class="cs-flow-k">' + key + '</span>' +
        '<span class="cs-flow-v">' + value + '</span>' +
        '<span class="cs-flow-s">' + sub + '</span></span>' +
        '</' + tag + '>';
}

function csFlowLink(icon, label, extraClass) {
    return '<div class="cs-flow-link' + (extraClass || '') + '"><span class="cs-flow-line"></span>' +
        '<span class="cs-flow-label"><i class="fa ' + icon + '" aria-hidden="true"></i> ' + label + '</span></div>';
}

// The journey a target takes. The link route is always there. When the email carries
// attachments a second lane branches off: opening the file and, for macro-enabled
// documents, running the macro. Stops with something to preview are buttons, and the
// connectors carry how many targets made it that far.
function csFlowHtml(campaign, stats) {
    var isSms = campaign.type === "sms";
    var hasPage = campaign.page && campaign.page.id;
    var n = function (k) { return stats ? (stats[k] || 0) : "–"; };
    var templateName = isSms
        ? (campaign.sms_template && campaign.sms_template.name) || ""
        : (campaign.template && campaign.template.name) || "";
    var messageStats = isSms
        ? csCount(n("sent"), "sent")
        : csCount(n("sent"), "sent") + ', ' + csCount(n("opened"), "opened");

    var linkLane = '<div class="cs-flow-lane">' +
        csFlowLink(isSms ? "fa-hand-pointer-o" : "fa-mouse-pointer", csCount(n("clicked"), isSms ? "tapped" : "clicked")) +
        csFlowStep('data-mode="page"' + (hasPage ? '' : ' disabled'), "fa-globe", "Landing page",
            // The capture icon follows the page name
            escapeHtml(hasPage ? campaign.page.name : "None") + '<span class="cs-flow-fill-cap"></span>',
            '<span class="cs-flow-fill-page">' + escapeHtml(csHost(campaign.url)) + '</span>') +
        csFlowLink("fa-exclamation-circle", csCount(n("submitted_data"), "submitted"), " cs-flow-fill-submit") +
        csFlowStep(null, "fa-share", "Redirect", '<span class="cs-flow-fill-redirect-host"></span>', "After submitting", " cs-flow-end cs-flow-fill-redirect") +
        '</div>';

    // Filled in once the template is loaded. The lane, the macro stop and the redirect stay
    // hidden until classes on .cs-flow reveal them.
    var attachmentLane = isSms ? '' : '<div class="cs-flow-lane cs-flow-lane-att">' +
        csFlowLink("fa-paperclip", csCount(n("opened_attachment"), "opened")) +
        csFlowStep('data-mode="attachment"', "fa-paperclip", "Attachment",
            '<span class="cs-flow-fill-att-name"></span>', '<span class="cs-flow-fill-att-sub"></span>') +
        csFlowLink("fa-file-code-o", csCount(n("macro_executed"), "ran macro"), " cs-flow-fill-macro") +
        csFlowStep(null, "fa-file-code-o", "Macro", "Executed", "After enabling content", " cs-flow-end cs-flow-fill-macro") +
        '</div>';

    return '<div class="cs-flow" role="group" aria-label="Target journey">' +
        csFlowStep('data-mode="message"', isSms ? "fa-mobile" : "fa-envelope", isSms ? "SMS" : "Email",
            '<span class="cs-flow-fill-message">' + escapeHtml(templateName) + '</span>', messageStats, " cs-flow-start") +
        '<div class="cs-flow-lanes">' + linkLane + attachmentLane + '</div>' +
        '</div>';
}

// Fills in what only the fetched template and page know: the subject, the attachments and
// what the page captures
function csFillFlow(campaign) {
    var isSms = campaign.type === "sms";
    var msg = isSms ? campaign.sms_template : campaign.template;
    if (msg && msg.id) {
        csFetch(isSms ? "sms" : "template", msg.id).done(function (t) {
            var line = isSms ? $.trim(t.from || "") : $.trim(csFillSample(t.subject || ""));
            if (line) $(".cs-flow-fill-message").text(line).attr("title", line);
            var files = isSms ? [] : (t.attachments || []);
            if (!files.length) return;
            $(".cs-flow-fill-att-name").text(files[0].name).attr("title", files.map(function (f) { return f.name; }).join(", "));
            $(".cs-flow-fill-att-sub").text(files.length > 1 ? "+" + (files.length - 1) + " more" :
                (t.track_attachments ? "Opens tracked" : "Opens not tracked"));
            var macro = files.some(function (f) { return CS_MACRO_FILE.test(f.name || ""); });
            $(".cs-flow").addClass("cs-flow-split").toggleClass("cs-flow-has-macro", macro);
        });
    }
    if (campaign.page && campaign.page.id) {
        csFetch("page", campaign.page.id).done(function (p) {
            // An icon beside the label, the sentence never fitted next to the host
            if (p.capture_credentials) {
                var what = p.capture_passwords ? "Captures credentials" : "Captures form data";
                $(".cs-flow-fill-cap").html('<i class="fa ' + (p.capture_passwords ? "fa-key" : "fa-pencil-square-o") +
                    ' cs-flow-cap" title="' + what + '" aria-label="' + what + '" role="img"></i>');
            }
            var redirect = $.trim(p.redirect_url || "");
            if (p.capture_credentials && redirect) {
                $(".cs-flow-fill-redirect-host").text(csHost(redirect) || redirect).attr("title", redirect);
                $(".cs-flow").addClass("cs-flow-has-redirect");
            }
        });
    }
}

function csFileSize(base64) {
    var bytes = Math.floor((base64 || "").length * 3 / 4);
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

function csFileIcon(name) {
    var ext = (/\.([a-z0-9]+)$/i.exec(name || "") || [])[1] || "";
    ext = ext.toLowerCase();
    if (/^(doc|docx|docm|dotm|rtf)$/.test(ext)) return "fa-file-word-o";
    if (/^(xls|xlsx|xlsm|xltm|xlam|xlsb|csv)$/.test(ext)) return "fa-file-excel-o";
    if (/^(ppt|pptx|pptm|potm|ppsm|ppam)$/.test(ext)) return "fa-file-powerpoint-o";
    if (ext === "pdf") return "fa-file-pdf-o";
    if (/^(zip|rar|7z|gz|iso|img)$/.test(ext)) return "fa-file-archive-o";
    if (/^(png|jpe?g|gif|svg|webp)$/.test(ext)) return "fa-file-image-o";
    if (/^(html?|js|hta|lnk|exe|bat|ps1|vbs)$/.test(ext)) return "fa-file-code-o";
    return "fa-file-o";
}

// Office files cannot be rendered here, so the attachment stop lists what the email carries
function csRenderAttachments(v, campaign, seq) {
    v.meta.empty();
    if (!campaign.template || !campaign.template.id) {
        v.body.html(csPreviewMessage("The email template for this campaign was deleted."));
        return;
    }
    v.body.html(csLoading("attachments"));
    csFetch("template", campaign.template.id).done(function (t) {
        if (seq !== csPreviewSeq) return;
        var files = t.attachments || [];
        // The attachment stop is hidden for this email, so show the message instead
        if (!files.length) {
            csPreviewMode = "message";
            csRenderPreview(campaign);
            return;
        }
        v.meta.html(csMetaCols([
            ["Files", String(files.length)],
            ["Open tracking", t.track_attachments ? "On" : "Off"]
        ]));
        v.body.html('<div class="cs-pv-files">' + files.map(function (f) {
            var macro = CS_MACRO_FILE.test(f.name || "");
            return '<div class="cs-pv-file">' +
                '<i class="fa ' + csFileIcon(f.name) + '" aria-hidden="true"></i>' +
                '<span class="cs-pv-file-name">' + escapeHtml(f.name || "Unnamed file") + '</span>' +
                (macro ? '<span class="cs-pv-file-tag">Macro-enabled</span>' : '') +
                '<span class="cs-pv-file-size">' + csFileSize(f.content) + '</span>' +
                '</div>';
        }).join("") + '</div>');
    }).fail(function () {
        if (seq !== csPreviewSeq) return;
        v.body.html(csPreviewMessage("Could not load the attachments."));
    });
}

function showCampaignSummary(campaign, index, useSharedSettings, campaignSetId, isLastCampaign) {
    $(`#campaign_form_${index}`).hide();
    $(".campaign-summary").remove();

    var isSms = campaign.type === "sms";
    var cached = campaign.id ? campaignSetSummaryById[campaign.id] : null;
    var stats = cached && cached.stats ? cached.stats : null;
    var status = campaign.status || (cached && cached.status) || "";
    var url = $.trim(campaign.url || "");

    // Every result as a card, same cards as the set Overview. Email gets the full eight.
    var st = stats || {};
    var tile = function (key, icon, cls, label, value) {
        return csKpiCard(stats, "target", key, icon, cls, label, value);
    };
    var tiles = tile("sent", "fa-paper-plane", "cs-stat-sent", "Sent", st.sent) +
        (isSms ? "" : tile("opened", "fa-envelope-open-o", "cs-stat-opened", "Opened", st.opened)) +
        tile("clicked", "fa-mouse-pointer", "cs-stat-clicked", "Clicked", st.clicked) +
        tile("submitted", "fa-exclamation-circle", "cs-stat-submitted", "Submitted", st.submitted_data) +
        (isSms ? "" :
            tile("attachment", "fa-paperclip", "cs-stat-attachment-opened", "Attachment opened", st.opened_attachment) +
            tile("macro", "fa-file-code-o", "cs-stat-macro-executed", "Macro executed", st.macro_executed) +
            tile("replied", "fa-reply", "cs-stat-replied", "Replied", st.replied) +
            tile("reported", "fa-bullhorn", "cs-stat-reported", "Reported", st.email_reported));

    // Configuration: what the campaign sends on the left, where it links and when it ran on the right
    function cfgRow(icon, label, value) {
        return value ? '<div class="cs-cfg-row"><i class="fa ' + icon + '" aria-hidden="true"></i>' +
            '<span class="cs-cfg-k">' + label + '</span><span class="cs-cfg-v">' + value + '</span></div>' : "";
    }
    var fmt = function (d) { return moment(d).format("MMM Do YYYY, h:mm a"); };
    var setupRows =
        cfgRow(isSms ? "fa-mobile" : "fa-envelope-o", "Type", isSms ? "SMS" : "Email") +
        cfgRow("fa-file-text-o", isSms ? "SMS template" : "Email template", isSms
            ? (campaign.sms_template && campaign.sms_template.name ? escapeHtml(campaign.sms_template.name) : "")
            : (campaign.template && campaign.template.name ? escapeHtml(campaign.template.name) : "")) +
        cfgRow("fa-server", isSms ? "SMS profile" : "Sending profile", isSms
            ? (campaign.sms && campaign.sms.name ? escapeHtml(campaign.sms.name) : "")
            : (campaign.smtp && campaign.smtp.name ? escapeHtml(campaign.smtp.name) : "")) +
        cfgRow("fa-window-maximize", "Landing page", campaign.page && campaign.page.name ? escapeHtml(campaign.page.name) : "") +
        cfgRow("fa-qrcode", "QR code size", campaign.qrsize ? escapeHtml(String(campaign.qrsize)) + " px" : "") +
        cfgRow("fa-lock", "HTTP basic auth", campaign.basicauth ? "Enabled" : "");
    var urlCard =
        '<section class="cs-cfg-card"><h6><i class="fa fa-link" aria-hidden="true"></i> Landing page URL</h6>' +
        (url
            // A read-only field, so the full URL can be scrolled through by dragging or with the arrow keys
            ? '<div class="cs-cfg-url"><input type="text" readonly value="' + escapeHtml(url) + '" title="' + escapeHtml(url) + '" aria-label="Landing page URL" spellcheck="false">' +
              '<button type="button" class="btn btn-link cs-ds-copy" data-url="' + escapeHtml(url) + '" title="Copy URL" aria-label="Copy URL">' +
              '<i class="fa fa-clone" aria-hidden="true"></i></button></div>' +
              (campaign.urlparam ? '<p class="cs-cfg-sub cs-cfg-param"><i class="fa fa-tag" aria-hidden="true"></i> URL parameter <code>' + escapeHtml(campaign.urlparam) + '</code></p>' : '')
            : '<p class="cs-cfg-sub">No URL set</p>') +
        '</section>';
    var whenCard =
        '<section class="cs-cfg-card"><h6><i class="fa fa-calendar" aria-hidden="true"></i> Launched</h6>' +
        '<p class="cs-cfg-when">' + (csValidDate(campaign.launch_date) ? fmt(campaign.launch_date) : "Not launched yet") + '</p>' +
        (csValidDate(campaign.send_by_date) ? '<p class="cs-cfg-sub">Send by <b>' + fmt(campaign.send_by_date) + '</b></p>' : '') +
        '</section>';
    var details = '<div class="cs-cfg">' +
        '<section class="cs-cfg-card"><h6><i class="fa fa-cog" aria-hidden="true"></i> Setup</h6><div class="cs-cfg-rows">' + setupRows + '</div></section>' +
        '<div class="cs-cfg-side">' + urlCard + whenCard + '</div>' +
        '</div>';

    var overview =
        '<div class="cs-ds-status-head"><h5 class="cs-ds-h">Status</h5>' +
        (stats ? '<span class="cs-target-pill"><i class="fa fa-users" aria-hidden="true"></i> ' + csPlural(stats.total, "target", "targets") + '</span>' : '') +
        // Errors are about delivery, not engagement, so they sit with the target count
        (st.error ? '<span class="cs-target-pill cs-error-pill" title="Sending failed for ' + csPlural(st.error, "target", "targets") + '">' +
            '<i class="fa fa-exclamation-triangle" aria-hidden="true"></i> ' + csPlural(st.error, "error", "errors") + '</span>' : '') +
        '</div>' +
        '<div class="cs-kpis cs-kpis-campaign' + (isSms ? " cs-kpis-3" : "") + '">' + tiles + '</div>' +
        '<h5 class="cs-ds-h">Configuration</h5>' + details;

    var preview =
        '<div class="cs-pv-head">' + csFlowHtml(campaign, stats) +
        '<button type="button" class="btn btn-default btn-sm cs-pv-expand" title="Full screen" aria-label="Full screen preview"><i class="fa fa-expand" aria-hidden="true"></i></button>' +
        '</div>' +
        '<div class="cs-pv-meta" id="csPreviewMeta"></div>' +
        '<div class="cs-pv-body" id="csPreviewBody"></div>';

    var tabs = [["overview", "Overview"], ["preview", "Preview"]];
    var html =
        '<div class="campaign-summary cs-ds" id="campaign_summary_' + index + '">' +
        '<div class="cs-ds-head">' +
        '<div class="cs-ds-title"><h4>' + escapeHtml(campaign.name) + '</h4>' +
        (status ? '<span class="label cs-ov-status ' + (CS_STATUS_LABELS[status] || "label-default") + '">' + escapeHtml(status) + '</span>' : '') +
        '</div>' +
        (campaign.id ? '<a href="/campaigns/' + campaign.id + '" class="btn btn-primary btn-sm"><i class="fa fa-bar-chart" aria-hidden="true"></i> View results</a>' : '') +
        '</div>' +
        '<div class="cs-dt-tabs" role="tablist" aria-label="Campaign details">' +
        tabs.map(function (t) {
            return '<button type="button" class="cs-dt-tab" role="tab" id="csTab-' + t[0] + '" aria-controls="csPane-' + t[0] + '" data-tab="' + t[0] + '">' + t[1] + '</button>';
        }).join("") +
        '</div>' +
        '<div class="cs-dt-pane" role="tabpanel" id="csPane-overview" aria-labelledby="csTab-overview" data-tab="overview">' + overview + '</div>' +
        '<div class="cs-dt-pane cs-dt-pane-preview" role="tabpanel" id="csPane-preview" aria-labelledby="csTab-preview" data-tab="preview">' + preview + '</div>' +
        '</div>';

    $("#campaignDetail").append(html);
    $(".campaign-detail-placeholder").hide();
    $("#campaignDetail").data("csCampaign", campaign);
    if (csFullOpen) {
        $("#csPvFullFlow").html(csFlowHtml(campaign, stats));
        $("#csPvFullTitle").text(campaign.name);
    }
    csShowDetailTab(csDetailTab);
    csFillFlow(campaign);
    csRenderPreview(campaign);
}

function csOpenFullPreview() {
    var campaign = $("#campaignDetail").data("csCampaign");
    if (!campaign) return;
    if (!$("#csPvFull").length) {
        // Inside #modal (not its transformed dialog), so it covers the window and Bootstrap
        // keeps focus where it is
        $("#modal").append(
            '<div class="cs-pv-full" id="csPvFull" role="dialog" aria-modal="true" aria-labelledby="csPvFullTitle" style="display:none;">' +
            '<div class="cs-pv-full-head">' +
            '<h4 id="csPvFullTitle"></h4>' +
            '<button type="button" class="btn btn-default btn-sm cs-pv-full-close"><i class="fa fa-compress" aria-hidden="true"></i> Exit full screen</button>' +
            '</div>' +
            '<div class="cs-pv-full-flow" id="csPvFullFlow"></div>' +
            '<div class="cs-pv-meta" id="csPreviewFullMeta"></div>' +
            '<div class="cs-pv-body" id="csPreviewFullBody"></div>' +
            '</div>');
        // Bound on the overlay itself so it runs before Bootstrap's handler on #modal,
        // letting Esc leave full screen without closing the modal
        $("#csPvFull").on("keydown", function (e) {
            if (e.key === "Escape") {
                e.stopPropagation();
                csCloseFullPreview();
            }
        });
    }
    var cached = campaign.id ? campaignSetSummaryById[campaign.id] : null;
    $("#csPvFullFlow").html(csFlowHtml(campaign, cached && cached.stats ? cached.stats : null));
    $("#csPvFullTitle").text(campaign.name);
    csFullOpen = true;
    $("#csPvFull").show();
    $("#csPvFull .cs-pv-full-close").focus();
    csFillFlow(campaign);
    csRenderPreview(campaign);
}

function csCloseFullPreview() {
    csFullOpen = false;
    $("#csPvFull").hide();
    $("#csPreviewFullMeta, #csPreviewFullBody, #csPvFullFlow").empty();
    $("#campaignDetail .cs-pv-expand").focus();
    csFitFrames();
}

// Narrows the campaign list by name
function csFilterCampaigns(query) {
    var q = $.trim(query).toLowerCase();
    var shown = 0;
    $("#campaignList .campaign-list-item").each(function () {
        var match = q === "" || $(this).find(".campaign-list-item-text").text().toLowerCase().indexOf(q) !== -1;
        $(this).toggle(match);
        if (match) shown++;
    });
    $("#csListEmpty").toggle(shown === 0);
}

$(document).on("click", ".cs-dt-tab", function () {
    csShowDetailTab($(this).data("tab"));
});

// Arrow keys move between the detail tabs, as in any tab list
$(document).on("keydown", ".cs-dt-tab", function (e) {
    var $tabs = $(".cs-dt-tab");
    var i = $tabs.index(this);
    var next = e.key === "ArrowRight" ? i + 1 : e.key === "ArrowLeft" ? i - 1 : null;
    if (next === null) return;
    e.preventDefault();
    var $t = $tabs.eq((next + $tabs.length) % $tabs.length);
    csShowDetailTab($t.data("tab"));
    $t.focus();
});

$(document).on("click", ".cs-flow-step[data-mode]", function () {
    var campaign = $("#campaignDetail").data("csCampaign");
    csPreviewMode = $(this).data("mode");
    if (campaign) csRenderPreview(campaign);
});

$(document).on("click", ".cs-pv-expand", csOpenFullPreview);
$(document).on("click", ".cs-pv-full-close", csCloseFullPreview);

$(document).on("input", "#csCampaignSearch", function () {
    csFilterCampaigns($(this).val());
});

$(document).on("click", ".cs-ds-copy", function () {
    var $btn = $(this);
    var done = function () {
        $btn.find(".fa").removeClass("fa-clone").addClass("fa-check");
        setTimeout(function () { $btn.find(".fa").removeClass("fa-check").addClass("fa-clone"); }, 1500);
    };
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText($btn.data("url")).then(done);
    }
});

// Frames measure 0 while their tab is hidden, so size them again once it shows
$(document).on("shown.bs.tab", 'a[href="#campaignsTab"]', csFitFrames);
$(window).on("resize", csFitFrames);

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
            $("#send_order").val(cs.send_order || "sequential");

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
                                $(`#campaign_randomize_send_order_${i}`).prop("checked", campaign.randomize_send_order || false);
                                
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
            $("#send_order").val(dcs.send_order || "sequential");

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
                                $(`#campaign_randomize_send_order_${i}`).prop("checked", campaign.randomize_send_order || false);
                                
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

// ── Creating or editing a set: three steps over the existing panes ──
// Setup is the General Settings pane, Campaigns the Campaigns pane, and Review a read-only
// summary. The panes, saving, drafts and launch are unchanged. Viewing a launched set keeps
// its own tabs, so all of this is skipped while the modal has .cs-viewing.

var CS_STEP_PANES = { 1: "#generalSettings", 2: "#campaignsTab", 3: "#reviewTab" };
var csCurrentStep = 1;

function csWizardActive() {
    return !$("#modal").hasClass("cs-viewing");
}

function csGoStep(n) {
    $('#csMainTabs a[href="' + CS_STEP_PANES[n] + '"]').tab("show");
}

// Follows whichever pane is showing, so the existing code that switches panes keeps the
// steps in sync
function csSyncStep(pane) {
    var n = pane === "#campaignsTab" ? 2 : pane === "#reviewTab" ? 3 : 1;
    csCurrentStep = n;
    $("#modal").removeClass("cs-step-1 cs-step-2 cs-step-3").addClass("cs-step-" + n);
    $("#csSteps .cs-step").each(function () {
        var s = $(this).data("step");
        $(this).toggleClass("active", s === n).toggleClass("done", s < n);
        // The line after a passed step fills in
        $(this).closest("li").toggleClass("cs-step-passed", s < n);
        if (s === n) $(this).attr("aria-current", "step"); else $(this).removeAttr("aria-current");
    });
    if (!csWizardActive()) return;
    if (n === 1) csAutoOpenAdvanced();
    if (n === 2) csUpdateReadiness();
    if (n === 3) csRenderReview();
}

// Reads the shared switches the same way launchCampaignSet does
function csSharedFlags() {
    var all = $("#useSharedSettings").is(":checked");
    return {
        page: all || $("#useSharedPage").is(":checked"),
        url: all || $("#useSharedURL").is(":checked"),
        urlparam: all || $("#useSharedURLParam").is(":checked"),
        qrsize: all || $("#useSharedQRSize").is(":checked"),
        basicauth: all || $("#useSharedHTTPAuth").is(":checked"),
        schedule: all || $("#useSharedSchedule").is(":checked")
    };
}

// The Advanced settings, only those in use: a value when shared, or set per campaign
function csAdvancedReviewRows(shared) {
    var rows = "";
    var add = function (label, isShared, value) {
        if (!isShared) rows += csReviewRow(label, "Set per campaign", false);
        else if (value) rows += csReviewRow(label, value, false);
    };
    add("URL parameter", shared.urlparam, escapeHtml(csFieldValue($("#urlparam"))));
    add("QR code size", shared.qrsize, csFieldValue($("#qrsize")) ? escapeHtml(csFieldValue($("#qrsize"))) + " px" : "");
    add("HTTP auth", shared.basicauth, $("#basicauth").is(":checked") ? "Enabled" : "");
    return rows;
}

function csFieldValue($el) {
    var v = $el.val();
    if ($.isArray(v)) return v.length ? v : "";
    return v ? $.trim(String(v)) : "";
}

// What launch would reject for the set itself
function csSetIssues(shared) {
    var issues = [];
    if (!csFieldValue($("#name"))) issues.push({ text: "Name", field: "#name" });
    if (shared.page && !csFieldValue($("#page"))) issues.push({ text: "Landing page", field: "#page" });
    if (shared.url && !csURLReady(csFieldValue($("#url")))) issues.push({ text: "URL", field: "#url" });
    return issues;
}

// What launch would reject for one campaign
function csCampaignIssues($form, shared) {
    var issues = [];
    var sms = $form.find(".campaign-type").val() === "sms";
    var need = function (sel, text) {
        if (!csFieldValue($form.find(sel))) issues.push({ text: text, field: sel });
    };
    need(".campaign-name", "Name");
    need(sms ? ".sms-template" : ".email-template", sms ? "SMS template" : "Email template");
    need(sms ? ".sms-profile" : ".email-profile", sms ? "SMS profile" : "Sending profile");
    need(".groups", "Groups");
    if (!shared.page) need(".campaign-page", "Landing page");
    if (!shared.url && !csURLReady(csFieldValue($form.find(".campaign-url")))) issues.push({ text: "URL", field: ".campaign-url" });
    return issues;
}

// A URL still holding a template placeholder such as [DOMAIN] would be refused at launch
function csURLReady(url) {
    return !!url && !hasURLPlaceholder(url);
}

function csFormIndex(form) {
    return String(form.id || "").replace("campaign_form_", "");
}

// Ready or Incomplete on each campaign in the Campaigns step
function csUpdateReadiness() {
    if (!csWizardActive()) return;
    csUpdateGroupCounts();
    csUpdateOverrideHeads();
    var shared = csSharedFlags();
    $("#campaignDetail .campaign-detail-form").each(function () {
        var issues = csCampaignIssues($(this), shared);
        var $item = $('#campaignList .campaign-list-item[data-index="' + csFormIndex(this) + '"]');
        var $pill = $item.find(".cs-ready");
        if (!$pill.length) $pill = $('<span class="label cs-ready"></span>').appendTo($item);
        $pill.toggleClass("cs-ready-ok", !issues.length).toggleClass("cs-ready-todo", issues.length > 0)
            .text(issues.length ? "Incomplete" : "Ready")
            .attr("title", issues.length ? "Missing: " + issues.map(function (i) { return i.text; }).join(", ") : "Ready to launch");
    });
}

var csReadinessTimer = null;
function csQueueReadiness() {
    clearTimeout(csReadinessTimer);
    csReadinessTimer = setTimeout(csUpdateReadiness, 150);
}

// "Email template" reads "email template" mid-sentence, but SMS and URL keep their capitals
function csLowerFirst(text) {
    return /^[A-Z]{2}/.test(text) ? text : text.charAt(0).toLowerCase() + text.slice(1);
}

function csSelectedText($select) {
    return $select.find("option:selected").map(function () { return $(this).text(); }).get().join(", ");
}

function csReviewRow(label, value, missing) {
    return '<div class="cs-rv-row' + (missing ? ' cs-rv-missing' : '') + '"><span class="cs-rv-k">' + label + '</span>' +
        '<span class="cs-rv-v">' + (missing ? '<i class="fa fa-exclamation-circle" aria-hidden="true"></i> Missing' : value) + '</span></div>';
}

function csFixButton(attrs) {
    return '<button type="button" class="btn btn-default btn-xs cs-rv-fix" ' + attrs + '>Fix</button>';
}

// The summary on the Review step, with every problem launch would stop on at once
function csRenderReview() {
    var shared = csSharedFlags();
    var setIssues = csSetIssues(shared);
    var missing = function (list, field) { return list.some(function (i) { return i.field === field; }); };
    var total = setIssues.length;

    var schedule = shared.schedule
        ? escapeHtml($("#launch_date").val() || "Now") +
          ($("#send_by_date").val() ? ", sending until " + escapeHtml($("#send_by_date").val()) : "") +
          ", " + escapeHtml(csSelectedText($("#send_order")))
        : "Set per campaign";

    var setCard =
        '<section class="cs-rv-card"><header><h6><i class="fa fa-cog" aria-hidden="true"></i> Setup</h6>' +
        (setIssues.length ? csFixButton('data-step="1" data-field="' + setIssues[0].field + '"') : '') + '</header>' +
        csReviewRow("Name", escapeHtml($("#name").val()), missing(setIssues, "#name")) +
        csReviewRow("Landing page", shared.page ? escapeHtml(csSelectedText($("#page"))) : "Set per campaign", missing(setIssues, "#page")) +
        csReviewRow("URL", shared.url ? escapeHtml($("#url").val()) : "Set per campaign", missing(setIssues, "#url")) +
        csAdvancedReviewRows(shared) +
        csReviewRow("Schedule", schedule, false) +
        '</section>';

    // One card per campaign: what is missing and a Fix link, or Ready with what it sends
    var incomplete = 0;
    var forms = $("#campaignDetail .campaign-detail-form");
    var campaignCards = forms.map(function () {
        var $f = $(this);
        var idx = csFormIndex(this);
        var sms = $f.find(".campaign-type").val() === "sms";
        var issues = csCampaignIssues($f, shared);
        total += issues.length;
        if (issues.length) incomplete++;
        var name = $.trim($f.find(".campaign-name").val()) ||
            $('#campaignList .campaign-list-item[data-index="' + idx + '"] .campaign-list-item-text').text() ||
            "Campaign " + (parseInt(idx, 10) + 1);
        var status, detail;
        if (issues.length) {
            var missing = issues.map(function (i) { return csLowerFirst(i.text); }).join(", ");
            status = '<div class="cs-rv-cstatus cs-rv-warn"><i class="fa fa-exclamation-circle" aria-hidden="true"></i> ' +
                issues.length + (issues.length === 1 ? " missing field" : " missing fields") + '</div>';
            detail = escapeHtml(missing.charAt(0).toUpperCase() + missing.slice(1));
        } else {
            status = '<div class="cs-rv-cstatus cs-rv-ok"><i class="fa fa-check-circle" aria-hidden="true"></i> Ready</div>';
            detail = [csSelectedText($f.find(sms ? ".sms-template" : ".email-template")),
                csSelectedText($f.find(sms ? ".sms-profile" : ".email-profile")),
                csSelectedText($f.find(".groups"))].filter(function (p) { return p; }).map(escapeHtml).join('<span class="cs-rv-sep"></span>');
        }
        return '<div class="cs-rv-ccard' + (issues.length ? ' cs-rv-todo' : '') + '">' +
            '<div class="cs-rv-chead"><i class="fa ' + (sms ? "fa-mobile" : "fa-envelope-o") + ' cs-rv-type" aria-hidden="true"></i>' +
            '<span class="cs-rv-name" title="' + escapeHtml(name) + '">' + escapeHtml(name) + '</span></div>' +
            status + '<div class="cs-rv-detail">' + detail + '</div>' +
            (issues.length ? '<button type="button" class="btn btn-default btn-xs cs-rv-fix cs-rv-cfix" data-step="2" data-index="' + idx +
                '" data-field="' + issues[0].field + '">Fix</button>' : '') +
            '</div>';
    }).get().join("");

    var campaignsSummary = !forms.length ? "" : incomplete
        ? '<span class="cs-rv-csum cs-rv-warn"><i class="fa fa-exclamation-triangle" aria-hidden="true"></i> ' + incomplete + " incomplete</span>"
        : '<span class="cs-rv-csum cs-rv-ok"><i class="fa fa-check-circle" aria-hidden="true"></i> All ready</span>';
    var banner = total
        ? '<div class="cs-rv-banner cs-rv-banner-todo"><i class="fa fa-exclamation-triangle" aria-hidden="true"></i> ' +
          total + (total === 1 ? " thing" : " things") + ' to fix before launching</div>'
        : '<div class="cs-rv-banner cs-rv-banner-ok"><i class="fa fa-check-circle" aria-hidden="true"></i> Everything is set. Ready to launch.</div>';

    $("#csReview").html(banner + setCard +
        '<section class="cs-rv-campaigns"><header><h6>Campaigns (' + forms.length + ')</h6>' + campaignsSummary + '</header>' +
        (campaignCards ? '<div class="cs-rv-grid">' + campaignCards + '</div>' : '<p class="cs-rv-empty">No campaigns yet.</p>') + '</section>');
}

// Fix: jump to the step and the field that needs attention
$(document).on("click", ".cs-rv-fix", function () {
    var step = $(this).data("step"), field = $(this).data("field"), idx = $(this).data("index");
    csGoStep(step);
    setTimeout(function () {
        var $target;
        if (step === 2) {
            $('#campaignList .campaign-list-item[data-index="' + idx + '"]').trigger("click");
            $target = $("#campaign_form_" + idx).find(field);
        } else {
            $target = $(field);
        }
        if (!$target || !$target.length) return;
        // A folded section would hide the field, so unfold it first
        csSetSectionOpen($target.closest(".cs-cf-section"), true);
        if ($target.hasClass("select2-hidden-accessible")) $target.select2("open"); else $target.focus();
    }, 200);
});

$(document).on("shown.bs.tab", '#csMainTabs a[data-toggle="tab"]', function (e) {
    csSyncStep($(e.target).attr("href"));
});
$(document).on("click", "#csSteps .cs-step", function () {
    csGoStep($(this).data("step"));
});
// Launch shows on every step. Pressed before the Review step, it goes to Review first, so the
// set is always checked before it launches. Capture phase, so it runs before the launch
// handlers the existing code binds on the button itself.
document.addEventListener("click", function (e) {
    var btn = e.target.closest && e.target.closest("#launchButton");
    if (!btn || !csWizardActive() || csCurrentStep === 3) return;
    e.preventDefault();
    e.stopPropagation();
    csGoStep(3);
}, true);
// Opening for new, edit or copy starts wherever the existing code put the panes
$(document).on("shown.bs.modal", "#modal", function () {
    var active = $("#modal .tab-content > .tab-pane.active").attr("id");
    csSyncStep(active ? "#" + active : "#generalSettings");
    if (csWizardActive()) csAutoOpenAdvanced(true);
});
$(document).on("hidden.bs.modal", "#modal", function () {
    $("#reviewTab").removeClass("active");
    $("#csMainTabs .cs-review-nav").removeClass("active");
});
$(document).on("input change", "#campaignDetail :input, #generalSettings :input", csQueueReadiness);
$(document).on("click", ".shared-toggle-btn, #setAllShared, #setAllPerCampaign, .cs-type-btn", csQueueReadiness);

// ── Form contents: the Advanced fold, suggested names, target counts, copied setup ──

function csSetAdvancedOpen(open) {
    $("#csAdvanced").toggle(open);
    $("#csAdvancedToggle").attr("aria-expanded", open ? "true" : "false")
        .find(".fa").toggleClass("fa-caret-right", !open).toggleClass("fa-caret-down", open);
}

// Opens the fold when one of its settings is in use, so nothing set is ever hidden. Only a
// fresh open closes it, so a fold opened by hand stays open between steps.
function csAutoOpenAdvanced(fresh) {
    var inUse = csFieldValue($("#urlparam")) || csFieldValue($("#qrsize")) || $("#basicauth").is(":checked") ||
        !$("#useSharedURLParam").is(":checked") || !$("#useSharedQRSize").is(":checked") || !$("#useSharedHTTPAuth").is(":checked");
    if (inUse) csSetAdvancedOpen(true);
    else if (fresh) csSetAdvancedOpen(false);
}

// "52 targets in 2 groups" under each campaign's Groups
function csUpdateGroupCounts() {
    $("#campaignDetail .campaign-detail-form").each(function () {
        var $selected = $(this).find(".groups option:selected");
        var known = $selected.filter(function () { return $(this).attr("data-targets") !== undefined; });
        var total = 0;
        known.each(function () { total += parseInt($(this).attr("data-targets"), 10) || 0; });
        var text = "";
        if ($selected.length && known.length === $selected.length) {
            text = total + (total === 1 ? " target" : " targets") +
                ($selected.length > 1 ? " in " + $selected.length + " groups" : "");
        }
        $("#groups_count_" + csFormIndex(this)).text(text);
    });
}

// The URL & Tracking and Schedule sections only show when something in them is set per campaign
function csUpdateOverrideHeads() {
    $("#campaignDetail .campaign-settings-override").each(function () {
        var anyField = $(this).find(".form-group").filter(function () { return this.style.display !== "none"; }).length > 0;
        $(this).toggle(anyField);
    });
}

// A new campaign starts with the selected one's type, sending profile and groups
function csCopyCampaignSetup($from, index) {
    var $to = $("#campaign_form_" + index);
    if (!$to.length) return;
    if ($from.find(".campaign-type").val() === "sms") $to.find('.cs-type-btn[data-type="sms"]').trigger("click");
    [".email-profile", ".sms-profile", ".groups"].forEach(function (sel) {
        var v = $from.find(sel).val();
        if (v && v.length) $to.find(sel).val(v).trigger("change");
    });
    $to.find(".campaign-randomize-send-order").prop("checked", $from.find(".campaign-randomize-send-order").is(":checked"));
    csQueueReadiness();
}

// Picking a template fills an empty name with the template's name. A name typed by hand
// is never replaced.
$(document).on("change", ".campaign-detail-form .email-template, .campaign-detail-form .sms-template", function () {
    var $form = $(this).closest(".campaign-detail-form");
    var $name = $form.find(".campaign-name");
    var current = $.trim($name.val());
    if (current && current !== $name.data("csSuggested")) return;
    var picked = $.trim($(this).find("option:selected").text());
    if (!picked) return;
    $name.val(picked).data("csSuggested", picked).trigger("input");
});

$(document).on("click", "#csAdvancedToggle", function () {
    csSetAdvancedOpen(!$("#csAdvanced").is(":visible"));
});

// Campaign form sections fold from their header
function csSetSectionOpen($section, open) {
    if (!$section.length) return;
    $section.toggleClass("cs-cf-collapsed", !open);
    $section.find("> .cs-cf-head").attr("aria-expanded", open ? "true" : "false");
}

$(document).on("click", ".cs-cf-head", function () {
    var $section = $(this).closest(".cs-cf-section");
    csSetSectionOpen($section, $section.hasClass("cs-cf-collapsed"));
});
