function errorFlash(message) {
    $("#flashes").empty();
    $("#flashes").append(
        '<div style="text-align:center" class="alert alert-danger">\
        <i class="fa fa-exclamation-circle"></i> ' +
        message +
        "</div>"
    );
}

function successFlash(message) {
    $("#flashes").empty();
    $("#flashes").append(
        '<div style="text-align:center" class="alert alert-success">\
        <i class="fa fa-check-circle"></i> ' +
        message +
        "</div>"
    );
}

function errorFlashFade(message, timeout) {
    $("#flashes").empty();
    $("#flashes").append(
        '<div style="text-align:center" class="alert alert-danger">\
        <i class="fa fa-exclamation-circle"></i> ' +
        message +
        "</div>"
    );
    setTimeout(function() {
        $("#flashes").empty();
    }, timeout * 1000);
}

function successFlashFade(message, timeout) {
    $("#flashes").empty();
    $("#flashes").append(
        '<div style="text-align:center" class="alert alert-success">\
        <i class="fa fa-check-circle"></i> ' +
        message +
        "</div>"
    );
    setTimeout(function() {
        $("#flashes").empty();
    }, timeout * 1000);
}

function modalError(message) {
    $("#modal\\.flashes").empty().append(
        '<div style="text-align:center" class="alert alert-danger">\
        <i class="fa fa-exclamation-circle"></i> ' +
        message +
        "</div>"
    );
}

function query(endpoint, method, data, async) {
    return $.ajax({
        url: "/api" + endpoint,
        async: async,
        method: method,
        data: JSON.stringify(data),
        dataType: "json",
        contentType: "application/json",
        beforeSend: function(xhr) {
            xhr.setRequestHeader("Authorization", "Bearer " + user.api_key);
        }
    });
}

function escapeHtml(text) {
    return $("<div/>")
        .text(text)
        .html();
}

function unescapeHtml(html) {
    return $("<div/>")
        .html(html)
        .text();
}

window.escapeHtml = escapeHtml;

var capitalize = function(string) {
    return string.charAt(0).toUpperCase() + string.slice(1);
};

var api = {
    // campaigns contains the endpoints for /api/campaigns
    campaigns: {
        // get() - Queries the API for campaigns
        get: function() {
            return query("/campaigns/", "GET", {}, false);
        },
        // post() - Creates a new campaign
        post: function(campaign) {
            return query("/campaigns/", "POST", campaign, false);
        },
        // summary() - Queries the API for campaign summary information
        summary: function() {
            return query("/campaigns/summary", "GET", {}, false);
        }
    },
    // campaignId contains the endpoints for /api/campaigns/:id
    campaignId: {
        // get() - Queries the API for the campaign with the specified ID
        get: function(id) {
            return query("/campaigns/" + id, "GET", {}, true);
        },
        // delete() - Deletes the campaign with the specified ID
        delete: function(id) {
            return query("/campaigns/" + id, "DELETE", {}, false);
        },
        // results() - Queries the API for campaign results
        results: function(id) {
            return query("/campaigns/" + id + "/results", "GET", {}, true);
        },
        // complete() - Marks a campaign as complete
        complete: function(id) {
            return query("/campaigns/" + id + "/complete", "GET", {}, true);
        },
        // summary() - Queries the API for campaign summary information
        summary: function(id) {
            return query("/campaigns/" + id + "/summary", "GET", {}, true);
        }
    },
    // groups contains the endpoints for /api/groups/
    groups: {
        // get() - Queries the API for groups
        get: function() {
            return query("/groups/", "GET", {}, false);
        },
        // post() - Creates a new group
        post: function(group) {
            return query("/groups/", "POST", group, false);
        },
        // summary() - Queries the API for group summary information
        summary: function() {
            return query("/groups/summary", "GET", {}, true);
        }
    },
    // groupId contains the endpoints for /api/groups/:id
    groupId: {
        // get() - Queries the API for the group with the specified ID
        get: function(id) {
            return query("/groups/" + id, "GET", {}, false);
        },
        // put() - Edits a group
        put: function(group) {
            return query("/groups/" + group.id, "PUT", group, false);
        },
        // delete() - Deletes a group
        delete: function(id) {
            return query("/groups/" + id, "DELETE", {}, false);
        }
    },
    // templates contains the endpoints for /api/templates/
    templates: {
        // get() - Queries the API for templates
        get: function() {
            return query("/templates/", "GET", {}, false);
        },
        // post() - Creates a new template
        post: function(template) {
            return query("/templates/", "POST", template, false);
        }
    },
    // templateId contains the endpoints for /api/templates/:id
    templateId: {
        // get() - Queries the API for the template with the specified ID
        get: function(id) {
            return query("/templates/" + id, "GET", {}, false);
        },
        // put() - Edits a template
        put: function(template) {
            return query("/templates/" + template.id, "PUT", template, false);
        },
        // delete() - Deletes a template
        delete: function(id) {
            return query("/templates/" + id, "DELETE", {}, false);
        }
    },
    // pages contains the endpoints for /api/pages/
    pages: {
        // get() - Queries the API for pages
        get: function() {
            return query("/pages/", "GET", {}, false);
        },
        // post() - Creates a new page
        post: function(page) {
            return query("/pages/", "POST", page, false);
        }
    },
    // pageId contains the endpoints for /api/pages/:id
    pageId: {
        // get() - Queries the API for the page with the specified ID
        get: function(id) {
            return query("/pages/" + id, "GET", {}, false);
        },
        // put() - Edits a page
        put: function(page) {
            return query("/pages/" + page.id, "PUT", page, false);
        },
        // delete() - Deletes a page
        delete: function(id) {
            return query("/pages/" + id, "DELETE", {}, false);
        }
    },
    // SMTP contains the endpoints for /api/smtp/
    SMTP: {
        // get() - Queries the API for SMTP settings
        get: function() {
            return query("/smtp/", "GET", {}, false);
        },
        // post() - Creates a new SMTP setting
        post: function(smtp) {
            return query("/smtp/", "POST", smtp, false);
        }
    },
    // SMTPId contains the endpoints for /api/smtp/:id
    SMTPId: {
        // get() - Queries the API for the SMTP setting with the specified ID
        get: function(id) {
            return query("/smtp/" + id, "GET", {}, false);
        },
        // put() - Edits a SMTP setting
        put: function(smtp) {
            return query("/smtp/" + smtp.id, "PUT", smtp, false);
        },
        // delete() - Deletes a SMTP setting
        delete: function(id) {
            return query("/smtp/" + id, "DELETE", {}, false);
        }
    },
    // IMAP contains the endpoints for /api/imap/
    IMAP: {
        // get() - Queries the API for IMAP settings
        get: function() {
            return query("/imap/", "GET", {}, false);
        },
        // post() - Creates a new IMAP setting
        post: function(imap) {
            return query("/imap/", "POST", imap, false);
        },
        // validate() - Validates an IMAP setting
        validate: function(imap) {
            return query("/imap/validate", "POST", imap, true);
        }
    },
    // users contains the endpoints for /api/users/
    users: {
        // get() - Queries the API for users
        get: function() {
            return query("/users/", "GET", {}, true);
        },
        // post() - Creates a new user
        post: function(user) {
            return query("/users/", "POST", user, true);
        }
    },
    // userId contains the endpoints for /api/users/:id
    userId: {
        // get() - Queries the API for the user with the specified ID
        get: function(id) {
            return query("/users/" + id, "GET", {}, true);
        },
        // put() - Edits a user
        put: function(user) {
            return query("/users/" + user.id, "PUT", user, true);
        },
        // delete() - Deletes a user
        delete: function(id) {
            return query("/users/" + id, "DELETE", {}, true);
        }
    },
    // webhooks contains the endpoints for /api/webhooks/
    webhooks: {
        // get() - Queries the API for webhooks
        get: function() {
            return query("/webhooks/", "GET", {}, false);
        },
        // post() - Creates a new webhook
        post: function(webhook) {
            return query("/webhooks/", "POST", webhook, false);
        }
    },
    // webhookId contains the endpoints for /api/webhooks/:id
    webhookId: {
        // get() - Queries the API for the webhook with the specified ID
        get: function(id) {
            return query("/webhooks/" + id, "GET", {}, false);
        },
        // put() - Edits a webhook
        put: function(webhook) {
            return query("/webhooks/" + webhook.id, "PUT", webhook, true);
        },
        // delete() - Deletes a webhook
        delete: function(id) {
            return query("/webhooks/" + id, "DELETE", {}, false);
        },
        // ping() - Sends a test request to the webhook
        ping: function(id) {
            return query("/webhooks/" + id + "/validate", "POST", {}, true);
        }
    },
    // qr_code contains the endpoints for /api/qr_code/
    qr_code: {
        // get() - Queries the API for QR codes
        get: function() {
            return query("/qr_code/", "GET", {}, true);
        },
        // post() - Creates a new QR code
        post: function(qr_code) {
            return query("/qr_code/", "POST", qr_code, true);
        },
        // delete() - Deletes a QR code
        delete: function(id) {
            return query("/qr_code/" + id, "DELETE", {}, false);
        }
    },
    // qrCodeId contains the endpoints for /api/qr_code/:id
    qrCodeId: {
        // get() - Queries the API for the QR code with the specified ID
        get: function(id) {
            return query("/qr_code/" + id + "/download", "GET", {}, false);
        }
    },
    // import_email contains the endpoint for /api/import/email
    import_email: function(req) {
        return query("/import/email", "POST", req, false);
    },
    // clone_site contains the endpoint for /api/import/site
    clone_site: function(req) {
        return query("/import/site", "POST", req, false);
    },
    // send_test_email contains the endpoint for /api/util/send_test_email
    send_test_email: function(req) {
        return query("/util/send_test_email", "POST", req, true);
    },
    // reset contains the endpoint for /api/reset
    reset: function() {
        return query("/reset", "POST", {}, true);
    }
};

window.api = api;

$(document).ready(function() {
    var path = location.pathname;
    $(".nav-sidebar li").each(function() {
        var $this = $(this);
        if ($this.find("a").attr("href") === path) {
            $this.addClass("active");
        }
    });
    $.fn.dataTable.moment("MMMM Do YYYY, h:mm:ss a");
    $('[data-toggle="tooltip"]').tooltip();
});
