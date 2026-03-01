function errorFlash(message) {
    $("#flashes").empty()
    $("#flashes").append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
        <i class=\"fa fa-exclamation-circle\"></i> " + message + "</div>")
}

function successFlash(message) {
    $("#flashes").empty()
    $("#flashes").append("<div style=\"text-align:center\" class=\"alert alert-success\">\
        <i class=\"fa fa-check-circle\"></i> " + message + "</div>")
}

// Fade message after n seconds
function errorFlashFade(message, fade) {
    $("#flashes").empty()
    $("#flashes").append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
        <i class=\"fa fa-exclamation-circle\"></i> " + message + "</div>")
    setTimeout(function(){ 
        $("#flashes").empty() 
    }, fade * 1000);
}
// Fade message after n seconds
function successFlashFade(message, fade) {  
    $("#flashes").empty()
    $("#flashes").append("<div style=\"text-align:center\" class=\"alert alert-success\">\
        <i class=\"fa fa-check-circle\"></i> " + message + "</div>")
    setTimeout(function(){ 
        $("#flashes").empty() 
    }, fade * 1000);

}

function modalError(message) {
    $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
        <i class=\"fa fa-exclamation-circle\"></i> " + message + "</div>")
}

function query(endpoint, method, data, async) {
    return $.ajax({
        url: "/api" + endpoint,
        async: async,
        method: method,
        data: JSON.stringify(data),
        dataType: "json",
        contentType: "application/json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    })
}

function escapeHtml(text) {
    return $("<div/>").text(text).html()
}
window.escapeHtml = escapeHtml

function unescapeHtml(html) {
    return $("<div/>").html(html).text()
}

/**
 * 
 * @param {string} string - The input string to capitalize
 * 
 */
var capitalize = function (string) {
    return string.charAt(0).toUpperCase() + string.slice(1);
}

/*
Define our API Endpoints
*/
var api = {
    // campaigns contains the endpoints for /campaigns
    campaigns: {
        // get() - Queries the API for GET /campaigns
        get: function () {
            return query("/campaigns/", "GET", {}, false)
        },
        // post() - Posts a campaign to POST /campaigns
        post: function (data) {
            return query("/campaigns/", "POST", data, false)
        },
        // summary() - Queries the API for GET /campaigns/summary
        summary: function () {
            return query("/campaigns/summary", "GET", {}, false)
        },
        // bulkDelete() - Deletes multiple campaigns at DELETE /campaigns
        bulkDelete: function (ids) {
            return query("/campaigns/", "DELETE", { ids: ids }, false)
        }
    },
    // campaignId contains the endpoints for /campaigns/:id
    campaignId: {
        // get() - Queries the API for GET /campaigns/:id
        get: function (id) {
            return query("/campaigns/" + id, "GET", {}, true)
        },
        // delete() - Deletes a campaign at DELETE /campaigns/:id
        delete: function (id) {
            return query("/campaigns/" + id, "DELETE", {}, false)
        },
        // results() - Queries the API for GET /campaigns/:id/results
        results: function (id) {
            return query("/campaigns/" + id + "/results", "GET", {}, true)
        },
        // complete() - Completes a campaign at POST /campaigns/:id/complete
        complete: function (id) {
            return query("/campaigns/" + id + "/complete", "GET", {}, true)
        },
        // summary() - Queries the API for GET /campaigns/summary
        summary: function (id) {
            return query("/campaigns/" + id + "/summary", "GET", {}, true)
        },
        // generateLink() - Generates a new tracking link for generic campaigns
        generateLink: function (id, linkName) {
            var data = linkName ? { name: linkName } : {};
            return query("/campaigns/" + id + "/links", "POST", data, true)
        }
    },
    // groups contains the endpoints for /groups
    groups: {
        // get() - Queries the API for GET /groups
        get: function () {
            return query("/groups/", "GET", {}, false)
        },
        // post() - Posts a group to POST /groups
        post: function (group) {
            return query("/groups/", "POST", group, false)
        },
        // summary() - Queries the API for GET /groups/summary
        summary: function () {
            return query("/groups/summary", "GET", {}, true)
        }
    },
    // groupId contains the endpoints for /groups/:id
    groupId: {
        // get() - Queries the API for GET /groups/:id
        get: function (id) {
            return query("/groups/" + id, "GET", {}, false)
        },
        // put() - Puts a group to PUT /groups/:id
        put: function (group) {
            return query("/groups/" + group.id, "PUT", group, false)
        },
        // delete() - Deletes a group at DELETE /groups/:id
        delete: function (id) {
            return query("/groups/" + id, "DELETE", {}, false)
        }
    },
    // templates contains the endpoints for /templates
    templates: {
        // get() - Queries the API for GET /templates
        get: function () {
            return query("/templates/", "GET", {}, false)
        },
        // post() - Posts a template to POST /templates
        post: function (template) {
            return query("/templates/", "POST", template, false)
        },
        // bulkDelete() - Deletes multiple templates at DELETE /templates
        bulkDelete: function (ids) {
            return query("/templates/", "DELETE", { ids: ids }, false)
        }
    },
    // templateId contains the endpoints for /templates/:id
    templateId: {
        // get() - Queries the API for GET /templates/:id
        get: function (id) {
            return query("/templates/" + id, "GET", {}, false)
        },
        // put() - Puts a template to PUT /templates/:id
        put: function (template) {
            return query("/templates/" + template.id, "PUT", template, false)
        },
        // delete() - Deletes a template at DELETE /templates/:id
        delete: function (id) {
            return query("/templates/" + id, "DELETE", {}, false)
        }
    },
    // smsTemplates contains the endpoints for /sms_templates
    smsTemplates: {
        // get() - Queries the API for GET /sms_templates
        get: function () {
            return query("/sms_templates/", "GET", {}, false)
        },
        // post() - Posts a template to POST /sms_templates
        post: function (template) {
            return query("/sms_templates/", "POST", template, false)
        },
        // bulkDelete() - Deletes multiple SMS templates at DELETE /sms_templates
        bulkDelete: function (ids) {
            return query("/sms_templates/", "DELETE", { ids: ids }, false)
        }
    },
    // smsTemplateId contains the endpoints for /sms_templates/:id
    smsTemplateId: {
        // get() - Queries the API for GET /sms_templates/:id
        get: function (id) {
            return query("/sms_templates/" + id, "GET", {}, false)
        },
        // put() - Puts a template to PUT /sms_templates/:id
        put: function (template) {
            return query("/sms_templates/" + template.id, "PUT", template, false)
        },
        // delete() - Deletes a template at DELETE /sms_templates/:id
        delete: function (id) {
            return query("/sms_templates/" + id, "DELETE", {}, false)
        }
    },
    // pages contains the endpoints for /pages
    pages: {
        // get() - Queries the API for GET /pages
        get: function () {
            return query("/pages/", "GET", {}, false)
        },
        // post() - Posts a page to POST /pages
        post: function (page) {
            return query("/pages/", "POST", page, false)
        },
        // bulkDelete() - Deletes multiple pages at DELETE /pages
        bulkDelete: function (ids) {
            return query("/pages/", "DELETE", { ids: ids }, false)
        }
    },
    // pageId contains the endpoints for /pages/:id
    pageId: {
        // get() - Queries the API for GET /pages/:id
        get: function (id) {
            return query("/pages/" + id, "GET", {}, false)
        },
        // put() - Puts a page to PUT /pages/:id
        put: function (page) {
            return query("/pages/" + page.id, "PUT", page, false)
        },
        // delete() - Deletes a page at DELETE /pages/:id
        delete: function (id) {
            return query("/pages/" + id, "DELETE", {}, false)
        }
    },
    // mfaDefaultTemplate contains the endpoint for /pages/mfa_template
    mfaDefaultTemplate: {
        // get() - Queries the API for GET /pages/mfa_template
        get: function () {
            return query("/pages/mfa_template", "GET", {}, false)
        }
    },
    // SMTP contains the endpoints for /smtp
    SMTP: {
        // get() - Queries the API for GET /smtp
        get: function () {
            return query("/smtp/", "GET", {}, false)
        },
        // post() - Posts a SMTP to POST /smtp
        post: function (smtp) {
            return query("/smtp/", "POST", smtp, false)
        }
    },
    // SMTPId contains the endpoints for /smtp/:id
    SMTPId: {
        // get() - Queries the API for GET /smtp/:id
        get: function (id) {
            return query("/smtp/" + id, "GET", {}, false)
        },
        // put() - Puts a SMTP to PUT /smtp/:id
        put: function (smtp) {
            return query("/smtp/" + smtp.id, "PUT", smtp, false)
        },
        // delete() - Deletes a SMTP at DELETE /smtp/:id
        delete: function (id) {
            return query("/smtp/" + id, "DELETE", {}, false)
        }
    },
    // IMAP contains the endpoints for /imap/
    IMAP: {
        get: function() {
            return query("/imap/", "GET", {}, !1)
        },
        post: function(e) {
            return query("/imap/", "POST", e, !1)
        },
        validate: function(e) {
            return query("/imap/validate", "POST", e, true)
        },
        getNonCampaignReports: function() {
            return query("/imap/non_campaign_reports", "GET", {}, !1)
        },
        bulkDeleteReports: function(ids) {
            return query("/imap/non_campaign_reports", "POST", { ids: ids }, !1)
        }
    },
    // IMAPId contains the endpoints for /imap/:id
    IMAPId: {
        get: function(id) {
            return query("/imap/" + id, "GET", {}, !1)
        },
        put: function(imap) {
            return query("/imap/" + imap.id, "PUT", imap, !1)
        },
        delete: function(id) {
            return query("/imap/" + id, "DELETE", {}, !1)
        }
    },
    // users contains the endpoints for /users
    users: {
        // get() - Queries the API for GET /users
        get: function () {
            return query("/users/", "GET", {}, true)
        },
        // post() - Posts a user to POST /users
        post: function (user) {
            return query("/users/", "POST", user, true)
        }
    },
    // userId contains the endpoints for /users/:id
    userId: {
        // get() - Queries the API for GET /users/:id
        get: function (id) {
            return query("/users/" + id, "GET", {}, true)
        },
        // put() - Puts a user to PUT /users/:id
        put: function (user) {
            return query("/users/" + user.id, "PUT", user, true)
        },
        // delete() - Deletes a user at DELETE /users/:id
        delete: function (id) {
            return query("/users/" + id, "DELETE", {}, true)
        }
    },
    webhooks: {
        get: function() {
            return query("/webhooks/", "GET", {}, false)
        },
        post: function(webhook) {
            return query("/webhooks/", "POST", webhook, false)
        },
    },
    webhookId: {
        get: function(id) {
            return query("/webhooks/" + id, "GET", {}, false)
        },
        put: function(webhook) {
            return query("/webhooks/" + webhook.id, "PUT", webhook, true)
        },
        delete: function(id) {
            return query("/webhooks/" + id, "DELETE", {}, false)
        },
        ping: function(id) {
            return query("/webhooks/" + id + "/validate", "POST", {}, true)
        },
    },
    qr_code: {
        // get() - Queries the API for GET /qr_code/
        get: function () {
            return query("/qr_code/", "GET", {}, false)
        },
        // post() - Posts qr code data to /qr_code/
        post: function (qr_code) {
            return query("/qr_code/", "POST", qr_code, true)
        },
        // delete() - Deletes a QR code at DELETE /qr_code/:id
        delete: function (id) {
            return query("/qr_code/" + id, "DELETE", {}, false)
        }
    },
    // qrCodeId contains the endpoints for /qr_code/:id
    qrCodeId: {
        // get() - Queries the API for GET /qr_code/:id/download
        get: function (id) {
            return query("/qr_code/" + id + "/download", "GET", {}, false)
        }
    },
    // campaignSets contains the endpoints for /campaign_sets
    campaignSets: {
        // get() - Queries the API for GET /campaign_sets
        get: function () {
            return query("/campaign_sets/", "GET", {}, false)
        },
        // post() - Posts a campaign set to POST /campaign_sets
        post: function (campaignSet) {
            return query("/campaign_sets/", "POST", campaignSet, false)
        },
        // summary() - Queries the API for GET /campaign_sets/summary
        summary: function () {
            return query("/campaign_sets/summary", "GET", {}, false)
        }
    },
    // campaignSetId contains the endpoints for /campaign_sets/:id
    campaignSetId: {
        // get() - Queries the API for GET /campaign_sets/:id
        get: function (id) {
            return query("/campaign_sets/" + id, "GET", {}, true)
        },
        // put() - Updates a campaign set at PUT /campaign_sets/:id
        put: function (id, campaignSet) {
            return query("/campaign_sets/" + id, "PUT", campaignSet, false)
        },
        // delete() - Deletes a campaign set at DELETE /campaign_sets/:id
        delete: function (id) {
            return query("/campaign_sets/" + id, "DELETE", {}, false)
        },
        // complete() - Completes a campaign set at GET /campaign_sets/:id/complete
        complete: function (id) {
            return query("/campaign_sets/" + id + "/complete", "GET", {}, true)
        }
    },
    // draftCampaignSets contains the endpoints for /draft_campaign_sets
    draftCampaignSets: {
        // get() - Queries the API for GET /draft_campaign_sets
        get: function () {
            return query("/draft_campaign_sets/", "GET", {}, false)
        },
        // post() - Posts a draft campaign set to POST /draft_campaign_sets
        post: function (draftCampaignSet) {
            return query("/draft_campaign_sets/", "POST", draftCampaignSet, false)
        }
    },
    // draftCampaignSetId contains the endpoints for /draft_campaign_sets/:id
    draftCampaignSetId: {
        // get() - Queries the API for GET /draft_campaign_sets/:id
        get: function (id) {
            return query("/draft_campaign_sets/" + id, "GET", {}, true)
        },
        // put() - Updates a draft campaign set at PUT /draft_campaign_sets/:id
        put: function (id, draftCampaignSet) {
            return query("/draft_campaign_sets/" + id, "PUT", draftCampaignSet, false)
        },
        // delete() - Deletes a draft campaign set at DELETE /draft_campaign_sets/:id
        delete: function (id) {
            return query("/draft_campaign_sets/" + id, "DELETE", {}, false)
        },
        // launch() - Launches a draft campaign set at GET /draft_campaign_sets/:id/launch
        launch: function (id) {
            return query("/draft_campaign_sets/" + id + "/launch", "GET", {}, true)
        }
    },
    // SMS contains the endpoints for /sms
    SMS: {
        // get() - Queries the API for GET /sms
        get: function () {
            return query("/sms/", "GET", {}, false)
        },
        // post() - Posts a SMS profile to POST /sms
        post: function (sms) {
            return query("/sms/", "POST", sms, false)
        }
    },
    // SMSId contains the endpoints for /sms/:id
    SMSId: {
        // get() - Queries the API for GET /sms/:id
        get: function (id) {
            return query("/sms/" + id, "GET", {}, false)
        },
        // put() - Puts a SMS profile to PUT /sms/:id
        put: function (sms) {
            return query("/sms/" + sms.id, "PUT", sms, false)
        },
        // delete() - Deletes a SMS profile at DELETE /sms/:id
        delete: function (id) {
            return query("/sms/" + id, "DELETE", {}, false)
        },
        // balance() - Gets the balance for an SMS profile at GET /sms/:id/balance
        balance: function (id) {
            return query("/sms/" + id + "/balance", "GET", {}, true)
        }
    },
    // import handles all of the "import" functions in the api
    import_email: function (req) {
        return query("/import/email", "POST", req, false)
    },
    // clone_site handles importing a site by url
    clone_site: function (req) {
        return query("/import/site", "POST", req, false)
    },
    // send_test_email sends an email to the specified email address
    send_test_email: function (req) {
        return query("/util/send_test_email", "POST", req, true)
    },
    // send_test_sms sends an SMS to the specified phone number
    send_test_sms: function (req) {
        return query("/util/send_test_sms", "POST", req, true)
    },
    reset: function () {
        return query("/reset", "POST", {}, true)
    },
    // urlTemplates contains the endpoints for /url_templates
    urlTemplates: {
        // get() - Queries the API for GET /url_templates
        get: function () {
            return query("/url_templates/", "GET", {}, false)
        },
        // post() - Posts a URL template to POST /url_templates
        post: function (template) {
            return query("/url_templates/", "POST", template, false)
        }
    },
    // urlTemplateId contains the endpoints for /url_templates/:id
    urlTemplateId: {
        // get() - Queries the API for GET /url_templates/:id
        get: function (id) {
            return query("/url_templates/" + id, "GET", {}, false)
        },
        // put() - Puts a URL template to PUT /url_templates/:id
        put: function (template) {
            return query("/url_templates/" + template.id, "PUT", template, false)
        },
        // delete() - Deletes a URL template at DELETE /url_templates/:id
        delete: function (id) {
            return query("/url_templates/" + id, "DELETE", {}, false)
        }
    }
}
window.api = api

// Global function to apply theme (supports: 'default', 'dark-teal', 'dark-crimson')
function applyTheme(theme) {
    // Remove all theme classes first
    document.body.classList.remove('dark-theme', 'crimson-theme');
    document.documentElement.classList.remove('dark-theme', 'crimson-theme');
    
    // Apply the selected theme
    if (theme === 'dark-teal') {
        document.body.classList.add('dark-theme');
        document.documentElement.classList.add('dark-theme');
    } else if (theme === 'dark-crimson') {
        document.body.classList.add('crimson-theme');
        document.documentElement.classList.add('crimson-theme');
    }
    // 'default' theme has no classes, so light theme is shown
}
window.applyTheme = applyTheme;

// Legacy function for backward compatibility
function applyDarkTheme(enabled) {
    applyTheme(enabled ? 'dark-teal' : 'default');
}
window.applyDarkTheme = applyDarkTheme;

// Register our moment.js datatables listeners
$(document).ready(function () {
    // Apply theme on page load (for all pages)
    // Migrate old boolean setting if it exists
    var oldDarkTheme = localStorage.getItem('gophish.use_dark_theme');
    var currentTheme = localStorage.getItem('gophish.theme');
    
    if (!currentTheme && oldDarkTheme !== null) {
        // Migrate: if old setting was true, use dark-teal theme
        currentTheme = JSON.parse(oldDarkTheme) ? 'dark-teal' : 'default';
        localStorage.setItem('gophish.theme', currentTheme);
        localStorage.removeItem('gophish.use_dark_theme');
    } else if (!currentTheme) {
        currentTheme = 'default';
    }
    
    applyTheme(currentTheme);

    // Setup nav highlighting
    var path = location.pathname;
    $('.nav-sidebar li').each(function () {
        var $this = $(this);
        // if the current path is like this link, make it active
        if ($this.find("a").attr('href') === path) {
            $this.addClass('active');
        }
    })
    $.fn.dataTable.moment('MMMM Do YYYY, h:mm:ss a');
    // Setup tooltips
    $('[data-toggle="tooltip"]').tooltip()
});
