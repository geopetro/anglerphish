var profiles = []
var smsProfiles = []
var dmiKeyText = ""
var dmiSavedId = null
var dmiSettingsCache = null
// Type and auth method of the profile being edited, so a change that drops its stored secret can be flagged.
var spOriginal = null

var DMI_TYPES = {
    MS365_DMI: "Microsoft 365 (Direct Injection)",
    GWS_DMI: "Google Workspace (Direct Injection)"
}

var SP_SECTIONS = {
    SMTP: { title: "SMTP settings", icon: "fa-envelope-o" },
    MS365_DMI: { title: "Microsoft 365 settings", icon: "fa-windows" },
    GWS_DMI: { title: "Google Workspace settings", icon: "fa-google" }
}

function isDMIType(t) {
    return t === "MS365_DMI" || t === "GWS_DMI"
}

function dmiAuthMethod() {
    return $("#interface_type").val() === "MS365_DMI" ? $("#ms_auth_method").val() : $("#gws_auth_method").val()
}

// Shows the field group matching the selected type and auth method.
function applyInterfaceType() {
    var t = $("#interface_type").val() || "SMTP"
    var dmi = isDMIType(t)
    $("input[name=sp_type]").each(function () {
        var on = this.value === t
        this.checked = on
        $(this).closest(".sp-type").toggleClass("is-selected", on)
    })
    var section = SP_SECTIONS[t] || SP_SECTIONS.SMTP
    $("#spConfigTitle").text(section.title)
    $("#spConfigIcon").attr("class", "fa " + section.icon)
    $(".sp-smtp-only").toggle(!dmi)
    $(".sp-dmi-only").toggle(dmi)
    $("#fromCol").attr("class", dmi ? "col-sm-12" : "col-sm-6")
    $("#fromLabel").text(dmi ? "From address" : "SMTP From")
    $("#spChangeWarning").toggle(spOriginal !== null && spOriginal.hasSecret &&
        (spOriginal.type !== t || (dmi && spOriginal.auth !== dmiAuthMethod())))
    $("#msFields").toggle(t === "MS365_DMI")
    $("#gwsFields").toggle(t === "GWS_DMI")
    var cert = t === "MS365_DMI" && $("#ms_auth_method").val() === "certificate"
    $("#msSecretGroup").toggle(!cert)
    $("#msCertGroup").toggle(cert)
    $("#msCertButtons").toggle(cert && dmiSavedId !== null)
    $("#msCertUnsaved").toggle(cert && dmiSavedId === null)
    var operator = t === "GWS_DMI" && $("#gws_auth_method").val() === "operator_key"
    $("#gwsCustomerGroup").toggle(!operator)
    $("#gwsOperatorGroup").toggle(operator)
    if (operator) {
        loadOperatorInfo()
    }
    if (!dmi) {
        $("#spConfigStatus").hide()
    }
    $("#dmiFolderNameGroup").toggle($("#dmi_folder").val() === "custom")
    $("#dmiFolderHint").text(t === "GWS_DMI" ?
        "Custom folders need the gmail.labels scope as well" :
        "Use a path such as Inbox/Phish for nested folders")
    var saved = dmiSavedId !== null
    $("#dmiTestBtn, #dmiTestMailbox").prop("disabled", !saved)
    $("#dmiTestHint").toggle(!saved)
}

function copyText(text, $btn) {
    var done = function () {
        var old = $btn.html()
        $btn.html('<i class="fa fa-check"></i> Copied')
        setTimeout(function () { $btn.html(old) }, 2000)
    }
    if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () { modalError("Could not copy") })
    } else {
        var ta = $("<textarea>").val(text).appendTo("body")
        ta[0].select()
        document.execCommand("copy")
        ta.remove()
        done()
    }
}

function renderOperatorInfo(d) {
    var $box = $("#gwsOperatorInfo").empty()
    if (!d.google_key_set) {
        var $alert = $('<div class="alert alert-warning">').text("No operator service account key is set. ")
        if (serviceAccountsTab().length) {
            $alert.append($('<a href="#service-accounts">').text("Add one under Service accounts").on("click", function (e) {
                e.preventDefault()
                $("#modal").modal("hide")
                serviceAccountsTab().tab("show")
            }))
        } else {
            $alert.append(document.createTextNode("Ask an administrator to add one under Service accounts."))
        }
        $box.append($alert)
        return
    }
    var rows = [["Client ID", d.google_client_id], ["Scope", d.scope]]
    $.each(rows, function (i, r) {
        var $in = $('<input type="text" class="form-control" readonly>').val(r[1] || "")
        var $btn = $('<button type="button" class="btn btn-default"><i class="fa fa-copy"></i> Copy</button>')
        $btn.on("click", function () { copyText($in.val(), $btn) })
        $box.append($('<div class="form-group">').append($("<label class='control-label'>").text(r[0]),
            $('<div class="input-group">').append($in, $('<span class="input-group-btn">').append($btn))))
    })
}

// The tab only exists for administrators.
function serviceAccountsTab() {
    return $('a[href="#service-accounts"]')
}

var operatorKeyText = ""

function xhrMessage(xhr, fallback) {
    return xhr && xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : fallback
}

function serviceAccountRow(label, value) {
    var $in = $('<input type="text" class="form-control" readonly>').val(value || "")
    var $btn = $('<button type="button" class="btn btn-default"><i class="fa fa-copy" aria-hidden="true"></i> Copy</button>')
    $btn.on("click", function () { copyText($in.val(), $btn) })
    return $('<div class="form-group">').append($('<label class="control-label">').text(label),
        $('<div class="input-group">').append($in, $('<span class="input-group-btn">').append($btn)))
}

function renderServiceAccounts(d) {
    operatorKeyText = ""
    dmiSettingsCache = d
    var $panel = $('<div class="panel panel-default">').append(
        $('<div class="panel-heading">').append($('<h3 class="panel-title">').text("Operator Google service account")))
    var $body = $('<div class="panel-body">').appendTo($panel)
    $body.append($("<p>").text("Google Workspace profiles that use the operator service account sign with this key. " +
        "The customer authorizes the client ID and scope below in their Admin console under domain-wide delegation."))
    if (d.error) {
        $body.append($('<div class="alert alert-danger">').text(d.error))
    }
    if (d.google_key_set) {
        $body.append(serviceAccountRow("Client ID", d.google_client_id),
            serviceAccountRow("Service account email", d.google_client_email),
            serviceAccountRow("Scope", d.scope),
            serviceAccountRow("Scope for custom folders", d.labels_scope))
    } else {
        $body.append($('<div class="alert alert-info">').text("No operator key is set."))
    }
    $body.append($('<div class="form-group">').append(
        $('<label class="control-label" for="operatorKeyFile">').text(d.google_key_set ? "Replace key (.json)" : "Upload key (.json)"),
        $('<input type="file" class="form-control" id="operatorKeyFile" accept=".json,application/json">')))
    $body.append($('<div class="text-right">').append(
        d.google_key_set ? $('<button class="btn btn-danger" type="button" id="operatorKeyClear"><i class="fa fa-trash-o" aria-hidden="true"></i> Clear key</button>') : "",
        " ",
        $('<button class="btn btn-primary" type="button" id="operatorKeySave" disabled><i class="fa fa-save" aria-hidden="true"></i> Save key</button>')))
    $("#dmiContent").empty().append($('<div class="row">').append($('<div class="col-lg-8">').append($panel)))
}

function loadServiceAccounts() {
    api.dmiSettings.get()
        .done(renderServiceAccounts)
        .fail(function (xhr) {
            errorFlash(escapeHtml(xhrMessage(xhr, "Failed to load the service accounts")))
        })
}

function setupServiceAccounts() {
    if (!serviceAccountsTab().length) {
        return
    }
    serviceAccountsTab().on("shown.bs.tab", loadServiceAccounts)
    if (window.location.hash === "#service-accounts") {
        serviceAccountsTab().tab("show")
    }
    $("#dmiContent").on("change", "#operatorKeyFile", function () {
        var f = this.files && this.files[0]
        var input = this
        operatorKeyText = ""
        $("#operatorKeySave").prop("disabled", true)
        if (!f) {
            return
        }
        var reader = new FileReader()
        reader.onload = function () {
            try {
                JSON.parse(reader.result)
                operatorKeyText = reader.result
                $("#operatorKeySave").prop("disabled", false)
            } catch (e) {
                input.value = ""
                errorFlash("That file is not valid JSON")
            }
        }
        reader.readAsText(f)
    })
    $("#dmiContent").on("click", "#operatorKeySave", function () {
        if (!operatorKeyText) {
            return
        }
        var $btn = $(this).prop("disabled", true)
        api.dmiSettings.put(operatorKeyText)
            .done(function (data) {
                renderServiceAccounts(data)
                successFlashFade("Operator key saved", 5)
            })
            .fail(function (xhr) {
                $btn.prop("disabled", false)
                errorFlash(escapeHtml(xhrMessage(xhr, "Failed to save the operator key")))
            })
    })
    $("#dmiContent").on("click", "#operatorKeyClear", function () {
        Swal.fire({
            title: "Clear the operator key?",
            text: "Profiles that use the operator service account stop working until a new key is uploaded.",
            type: "warning",
            animation: false,
            showCancelButton: true,
            confirmButtonText: "Clear key",
            confirmButtonColor: "#428bca",
            reverseButtons: true,
            allowOutsideClick: false
        }).then(function (result) {
            if (!result.value) {
                return
            }
            api.dmiSettings.delete()
                .done(function (data) {
                    renderServiceAccounts(data)
                    successFlashFade("Operator key cleared", 5)
                })
                .fail(function (xhr) {
                    errorFlash(escapeHtml(xhrMessage(xhr, "Failed to clear the operator key")))
                })
        })
    })
}

function loadOperatorInfo() {
    if (dmiSettingsCache) {
        renderOperatorInfo(dmiSettingsCache)
        return
    }
    $("#gwsOperatorInfo").text("Loading...")
    api.dmiSettings.get()
        .done(function (d) {
            dmiSettingsCache = d
            renderOperatorInfo(d)
        })
        .fail(function () {
            $("#gwsOperatorInfo").empty().append($('<div class="alert alert-warning">').text(
                "The operator service account is only visible to administrators."))
        })
}

function resetDMI() {
    dmiKeyText = ""
    dmiSavedId = null
    $("#ms_auth_method").val("client_secret")
    $("#gws_auth_method").val("customer_key")
    $("#dmi_tenant_id, #dmi_client_id, #dmi_secret, #dmi_allowed_domains, #dmi_folder_name, #dmiTestMailbox").val("")
    $("#dmi_key_file").val("")
    $("#dmi_folder").val("inbox")
    $("#dmiKeyStatus").text("")
    $("#dmiTestResult").empty()
    $("#spConfigStatus").hide()
    $("#dmi_secret").attr("placeholder", "")
}

// keepSaved is false when copying, since the secret is never copied.
function fillDMI(profile, keepSaved) {
    if (!isDMIType(profile.interface_type)) {
        return
    }
    if (profile.interface_type === "MS365_DMI") {
        $("#ms_auth_method").val(profile.dmi_auth_method || "client_secret")
    } else {
        $("#gws_auth_method").val(profile.dmi_auth_method || "customer_key")
    }
    $("#dmi_tenant_id").val(profile.dmi_tenant_id)
    $("#dmi_client_id").val(profile.dmi_client_id)
    $("#dmi_allowed_domains").val(profile.dmi_allowed_domains)
    $("#dmi_folder").val(profile.dmi_folder || "inbox")
    $("#dmi_folder_name").val(profile.dmi_folder_name)
    if (keepSaved) {
        dmiSavedId = profile.id
        if (profile.dmi_secret_set) {
            $("#dmi_secret").attr("placeholder", "Saved. Leave blank to keep")
            $("#dmiKeyStatus").text("A key is saved. Choose a file to replace it.")
        }
    }
}

// Adds the DMI fields to a profile or test request object.
function readDMI(profile) {
    profile.dmi_auth_method = dmiAuthMethod()
    profile.dmi_tenant_id = $("#dmi_tenant_id").val().trim()
    profile.dmi_client_id = $("#dmi_client_id").val().trim()
    profile.dmi_allowed_domains = $("#dmi_allowed_domains").val().trim()
    profile.dmi_folder = $("#dmi_folder").val()
    profile.dmi_folder_name = $("#dmi_folder_name").val().trim()
    var secret = profile.interface_type === "GWS_DMI" ? dmiKeyText : $("#dmi_secret").val()
    if (secret && profile.dmi_auth_method !== "certificate" && profile.dmi_auth_method !== "operator_key") {
        profile.dmi_secret = secret
    }
}

function downloadCertificate() {
    var xhr = new XMLHttpRequest()
    xhr.open("GET", "/api/smtp/" + dmiSavedId + "/dmi/certificate")
    xhr.setRequestHeader("Authorization", "Bearer " + user.api_key)
    xhr.responseType = "blob"
    xhr.onload = function () {
        if (xhr.status !== 200) {
            modalError("Could not download the certificate")
            return
        }
        var name = "certificate.cer"
        var m = /filename="([^"]+)"/.exec(xhr.getResponseHeader("Content-Disposition") || "")
        if (m) {
            name = m[1]
        }
        var a = document.createElement("a")
        a.href = URL.createObjectURL(xhr.response)
        a.download = name
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        setTimeout(function () { URL.revokeObjectURL(a.href) }, 1000)
    }
    xhr.onerror = function () { modalError("Could not download the certificate") }
    xhr.send()
}

function regenerateCertificate() {
    Swal.fire({
        title: "Regenerate certificate?",
        text: "The customer must upload the new certificate to their app registration before sending works again.",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Regenerate",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false
    }).then(function (result) {
        if (!result.value) {
            return
        }
        api.SMTPId.dmiRegenerate(dmiSavedId)
            .done(function (data) {
                $.each(profiles, function (i, p) {
                    if (p.id === data.id) {
                        profiles[i] = data
                    }
                })
                $("#modal\\.flashes").empty().append('<div style="text-align:center" class="alert alert-success">' +
                    '<i class="fa fa-check-circle"></i> Certificate regenerated. Download it and send it to the customer.</div>')
            })
            .fail(function (xhr) {
                modalError(xhr.responseJSON ? xhr.responseJSON.message : "Could not regenerate the certificate")
            })
    })
}

function testDMIConnection() {
    var $out = $("#dmiTestResult")
    var mailbox = $("#dmiTestMailbox").val().trim()
    if (!mailbox) {
        $out.attr("class", "text-danger").text("Enter a mailbox to test")
        return
    }
    $out.attr("class", "text-muted").text("Testing...")
    api.SMTPId.dmiTest(dmiSavedId, mailbox)
        .done(function (data) {
            $out.attr("class", data.success ? "text-success" : "text-danger").text(data.message || (data.success ? "Connection successful" : "Test failed"))
            $("#spConfigStatus").toggle(!!data.success)
        })
        .fail(function (xhr) {
            $("#spConfigStatus").hide()
            $out.attr("class", "text-danger").text(xhr.responseJSON && xhr.responseJSON.message ? xhr.responseJSON.message : "Test failed")
        })
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    var headers = getHeaders()

    // Create the test email request with form values
    var test_email_request = {
        template: {},
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        custom: $("input[name=to_custom]").val(),
        url: '',
        smtp: {
            name: $("#name").val(),
            from_address: $("#from").val(),
            host: $("#host").val(),
            username: $("#username").val(),
            password: $("#password").val(),
            ignore_cert_errors: $("#ignore_cert_errors").prop("checked"),
            headers: headers,
        }
    }
    test_email_request.smtp.interface_type = $("#interface_type").val()
    if (isDMIType(test_email_request.smtp.interface_type)) {
        readDMI(test_email_request.smtp)
        if (dmiSavedId) {
            test_email_request.smtp.id = dmiSavedId
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
	    <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(data.responseJSON.message) + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

// Test SMS attempts to send a test SMS by POSTing to /api/util/send_test_sms
function sendTestSMS() {
    // Get the current provider
    var provider = $("#sms_provider").val();
    
    // Build provider-specific config based on form values
    var providerConfig = {};
    switch(provider) {
        case "twilio":
            providerConfig = {
                account_sid: $("#twilio_account_sid").val(),
                auth_token: $("#twilio_auth_token").val()
            }
            break;
        case "nexmo":
            providerConfig = {
                api_key: $("#nexmo_api_key").val(),
                api_secret: $("#nexmo_api_secret").val()
            }
            break;
    }
    
    var test_sms_request = {
        sms_template: {
            text: "This is a test SMS from {{.From}}." // Default test message
        },
        first_name: $("input[name=sms_to_first_name]").val(),
        last_name: $("input[name=sms_to_last_name]").val(),
        email: $("input[name=sms_to_phone]").val(), // Use email field for phone number
        position: $("input[name=sms_to_position]").val(),
        custom: $("input[name=sms_to_custom]").val(),
        url: '',
        sms: {
            name: $("#sms_name").val(),
            from: $("#sms_from").val(),
            provider: provider,
            provider_config: JSON.stringify(providerConfig)
        }
    }
    btnHtml = $("#sendTestSMSModalSubmit").html()
    $("#sendTestSMSModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    // Send the test SMS
    api.send_test_sms(test_sms_request)
        .success(function (data) {
            $("#sendTestSMSModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
	    <i class=\"fa fa-check-circle\"></i> SMS Sent!</div>")
            $("#sendTestSMSModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestSMSModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
	    <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(data.responseJSON.message) + "</div>")
            $("#sendTestSMSModalSubmit").html(btnHtml)
        })
}

// Save attempts to POST to /smtp/
function save(idx) {
    var profile = {
        headers: getHeaders()
    }
    profile.name = $("#name").val()
    profile.interface_type = $("#interface_type").val()
    profile.from_address = $("#from").val()
    profile.host = $("#host").val()
    profile.username = $("#username").val()
    profile.password = $("#password").val()
    profile.ignore_cert_errors = $("#ignore_cert_errors").prop("checked")
    if (isDMIType(profile.interface_type)) {
        readDMI(profile)
    }
    if (idx != -1) {
        profile.id = profiles[idx].id
        api.SMTPId.put(profile)
            .success(function (data) {
                successFlash("Profile edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the profile
        api.SMTP.post(profile)
            .success(function (data) {
                successFlash("Profile added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#interface_type").val("SMTP")
    $("#from").val("")
    $("#host").val("")
    $("#username").val("")
    $("#password").val("")
    $("#ignore_cert_errors").prop("checked", true)
    $("#password, #dmi_secret").attr("type", "password")
    spOriginal = null
    resetDMI()
    applyInterfaceType()
    clearHeaders()
    $("#headerKey, #headerValue").val("")
    $("#modal").modal('hide')
}

var dismissSendTestEmailModal = function () {
    $("#sendTestEmailModal\\.flashes").empty()
    $("#sendTestModalSubmit").html("<i class='fa fa-envelope'></i> Send")
}

var dismissSendTestSMSModal = function () {
    $("#sendTestSMSModal\\.flashes").empty()
    $("#sendTestSMSModalSubmit").html("<i class='fa fa-mobile'></i> Send")
}

var deleteProfile = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the sending profile. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(profiles[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.SMTPId.delete(profiles[idx].id)
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
                'Sending Profile Deleted!',
                'This sending profile has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function edit(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    var profile = {}
    resetDMI()
    clearHeaders()
    spOriginal = null
    if (idx != -1) {
        $("#profileModalLabel").text("Edit Sending Profile")
        profile = profiles[idx]
        $("#name").val(profile.name)
        $("#interface_type").val(profile.interface_type || "SMTP")
        $("#from").val(profile.from_address)
        $("#host").val(profile.host)
        $("#username").val(profile.username)
        $("#password").val(profile.password)
        $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
        fillDMI(profile, true)
        if (isDMIType(profile.interface_type)) {
            spOriginal = {
                type: profile.interface_type,
                auth: profile.dmi_auth_method,
                hasSecret: !!(profile.dmi_secret_set || profile.dmi_certificate)
            }
        }
        $.each(profile.headers, function (i, record) {
            addCustomHeader(record.key, record.value)
        });
    } else {
        $("#profileModalLabel").text("New Sending Profile")
    }
    applyInterfaceType()
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    var profile = {}
    profile = profiles[idx]
    $("#name").val("Copy of " + profile.name)
    $("#interface_type").val(profile.interface_type || "SMTP")
    $("#from").val(profile.from_address)
    $("#host").val(profile.host)
    $("#username").val(profile.username)
    $("#password").val(profile.password)
    $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
    spOriginal = null
    resetDMI()
    fillDMI(profile, false)
    clearHeaders()
    $.each(profile.headers || [], function (i, record) {
        addCustomHeader(record.key, record.value)
    })
    applyInterfaceType()
}

function load() {
    $("#profileTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.SMTP.get()
        .success(function (ss) {
            profiles = ss
            $("#loading").hide()
            if (profiles.length > 0) {
                $("#profileTable").show()
                profileTable = $("#profileTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                profileTable.clear()
                profileRows = []
                $.each(profiles, function (i, profile) {
                    profileRows.push([
                        escapeHtml(profile.name),
                        DMI_TYPES[profile.interface_type] || profile.interface_type,
                        moment(profile.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Profile' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Profile' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Profile' onclick='deleteProfile(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                profileTable.rows.add(profileRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching profiles")
        })
}

function getHeaders() {
    var list = []
    $("#headersList li").each(function () {
        list.push({ key: $(this).data("key"), value: $(this).data("value") })
    })
    return list
}

function updateHeadersEmpty() {
    $("#headersEmpty").toggle($("#headersList li").length === 0)
}

function clearHeaders() {
    $("#headersList").empty()
    updateHeadersEmpty()
}

// Adding a header that already exists replaces its value.
function addCustomHeader(header, value) {
    var $existing = $("#headersList li").filter(function () {
        return $(this).data("key") === header
    })
    var $item = $existing.length ? $existing.first().empty() : $("<li>").appendTo("#headersList")
    $item.data("key", header).data("value", value).append(
        $('<span class="sp-header-key">').text(header),
        $('<span class="sp-header-val">').text(value),
        $('<button type="button" class="btn btn-link sp-header-remove">')
            .attr("aria-label", "Remove header " + header)
            .append('<i class="fa fa-trash-o" aria-hidden="true"></i>'))
    updateHeadersEmpty()
}

function populateSMSProfiles() {
    api.SMS.get()
        .success(function (profiles) {
            var options = "<option value=''>-- Select a Sending Profile --</option>"
            $.each(profiles, function (i, profile) {
                options += "<option value='" + escapeHtml(profile.name) + "'>" + escapeHtml(profile.name) + "</option>"
            })
            $("#sms_profile").html(options)
        })
        .error(function () {
            errorFlash("Error fetching SMS sending profiles")
        })
}

$(document).ready(function () {
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
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal') // guard against infinite focus loop
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length
                    // CKEditor compatibility fix start.
                    &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                    // CKEditor compatibility fix end.
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    $("#sendTestEmailModal").on("hidden.bs.modal", function (event) {
        dismissSendTestEmailModal()
    })
    $("#sendTestSMSModal").on("hidden.bs.modal", function (event) {
        dismissSendTestSMSModal()
    })
    var addHeaderFromInputs = function () {
        var key = $("#headerKey").val().trim()
        var value = $("#headerValue").val()
        if (key === "" || value === "") {
            return false
        }
        addCustomHeader(key, value)
        $("#headerKey, #headerValue").val("")
        $("#headerKey").focus()
        return false
    }
    $("#addCustomHeader").on("click", addHeaderFromInputs)
    $("#headerKey, #headerValue").on("keydown", function (e) {
        if (e.key === "Enter") {
            e.preventDefault()
            addHeaderFromInputs()
        }
    })
    $("#headersList").on("click", ".sp-header-remove", function () {
        $(this).closest("li").remove()
        updateHeadersEmpty()
        $("#headerKey").focus()
    })
    $("input[name=sp_type]").on("change", function () {
        $("#interface_type").val(this.value).trigger("change")
    })
    $(".sp-reveal").on("click", function () {
        var $in = $($(this).data("target"))
        var show = $in.attr("type") === "password"
        $in.attr("type", show ? "text" : "password")
        $(this).attr("aria-pressed", show ? "true" : "false")
            .find(".fa").toggleClass("fa-eye", !show).toggleClass("fa-eye-slash", show)
    })
    // A passed test no longer applies once the sign-in or delivery settings change.
    $("#dmiFields, #dmiDelivery").on("input change", "input, select", function () {
        if (this.id !== "dmiTestMailbox") {
            $("#spConfigStatus").hide()
        }
    })
    $("#interface_type, #ms_auth_method, #gws_auth_method, #dmi_folder").on("change", applyInterfaceType)
    updateHeadersEmpty()
    $("#dmi_key_file").on("change", function () {
        var f = this.files && this.files[0]
        dmiKeyText = ""
        if (!f) {
            return
        }
        var reader = new FileReader()
        reader.onload = function () {
            try {
                JSON.parse(reader.result)
                dmiKeyText = reader.result
                $("#dmiKeyStatus").text("Loaded " + f.name)
            } catch (e) {
                $("#dmi_key_file").val("")
                $("#dmiKeyStatus").text("")
                modalError("That file is not valid JSON")
            }
        }
        reader.readAsText(f)
    })
    $("#dmiCertDownload").on("click", downloadCertificate)
    $("#dmiCertRegenerate").on("click", regenerateCertificate)
    $("#dmiTestBtn").on("click", testDMIConnection)
    applyInterfaceType()
    load()
    loadSMSProfiles()
    setupServiceAccounts()
    
    // Set up SMS provider change handler
    $("#sms_provider").on("change", function() {
        var provider = $(this).val();
        $(".provider-fields").hide();
        $(`#${provider}-fields`).show();
    });
    
    // Set up SMS modal submit handler
    $("#smsModalSubmit").on("click", function() {
        saveSMSProfile();
    });
    
    // Set up Send Test Email button handler
    $("button[data-target='#sendTestEmailModal']").on("click", function() {
        setupSendTestEmail();
    });
    
    // Set up Send Test SMS button handler
    $("button[data-target='#sendTestSMSModal']").on("click", function() {
        setupSendTestSMS();
    });
})

// SMS Profile Functions

// saveSMSProfile attempts to POST/PUT to /sms/
function saveSMSProfile() {
    var profile = {}
    profile.name = $("#sms_name").val()
    profile.provider = $("#sms_provider").val()
    profile.from = $("#sms_from").val()
    
    // Build provider-specific config
    var providerConfig = {}
    switch(profile.provider) {
        case "twilio":
            providerConfig = {
                account_sid: $("#twilio_account_sid").val(),
                auth_token: $("#twilio_auth_token").val()
            }
            break;
        case "nexmo":
            providerConfig = {
                api_key: $("#nexmo_api_key").val(),
                api_secret: $("#nexmo_api_secret").val()
            }
            break;
    }
    
    profile.provider_config = JSON.stringify(providerConfig)
    
    // Check if we're editing an existing profile
    if ($("#sms_id").val() != "-1") {
        profile.id = parseInt($("#sms_id").val())
        api.SMSId.put(profile)
            .success(function(data) {
                successFlash("SMS profile updated successfully!")
                loadSMSProfiles()
                dismissSMSModal()
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit a new profile
        api.SMS.post(profile)
            .success(function(data) {
                successFlash("SMS profile added successfully!")
                loadSMSProfiles()
                dismissSMSModal()
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismissSMSModal() {
    $("#smsModal\\.flashes").empty()
    $("#sms_name").val("")
    $("#sms_provider").val("twilio")
    $("#sms_from").val("")
    $("#twilio_account_sid").val("")
    $("#twilio_auth_token").val("")
    $("#nexmo_api_key").val("")
    $("#nexmo_api_secret").val("")
    $("#sms_id").val("-1")
    $(".provider-fields").hide()
    $("#twilio-fields").show()
    $("#smsModal").modal('hide')
}

function editSMSProfile(idx) {
    $("#smsModalSubmit").unbind('click').click(function() {
        saveSMSProfile()
    })
    
    // Create a hidden input to store the SMS profile ID
    if ($("#sms_id").length == 0) {
        $('<input>').attr({
            type: 'hidden',
            id: 'sms_id',
            value: '-1'
        }).appendTo('#smsModal form');
    }
    
    if (idx != -1) {
        $("#smsModalLabel").text("Edit SMS Sending Profile")
        var profile = smsProfiles[idx]
        $("#sms_id").val(profile.id)
        $("#sms_name").val(profile.name)
        $("#sms_provider").val(profile.provider)
        $("#sms_from").val(profile.from)
        
        // Parse and populate provider-specific fields
        try {
            var config = JSON.parse(profile.provider_config)
            switch(profile.provider) {
                case "twilio":
                    $("#twilio_account_sid").val(config.account_sid)
                    $("#twilio_auth_token").val(config.auth_token)
                    break;
                case "nexmo":
                    $("#nexmo_api_key").val(config.api_key)
                    $("#nexmo_api_secret").val(config.api_secret)
                    break;
            }
        } catch (e) {
            console.error("Error parsing provider config:", e)
        }
        
        // Show the appropriate provider fields
        $(".provider-fields").hide()
        $(`#${profile.provider}-fields`).show()
    } else {
        $("#smsModalLabel").text("New SMS Sending Profile")
        $("#sms_id").val("-1")
        // Default to showing Twilio fields
        $(".provider-fields").hide()
        $("#twilio-fields").show()
    }
}

function copySMSProfile(idx) {
    editSMSProfile(-1)
    var profile = smsProfiles[idx]
    $("#sms_name").val("Copy of " + profile.name)
    $("#sms_provider").val(profile.provider)
    $("#sms_from").val(profile.from)
    
    // Parse and populate provider-specific fields
    try {
        var config = JSON.parse(profile.provider_config)
        switch(profile.provider) {
            case "twilio":
                $("#twilio_account_sid").val(config.account_sid)
                $("#twilio_auth_token").val(config.auth_token)
                break;
            case "nexmo":
                $("#nexmo_api_key").val(config.api_key)
                $("#nexmo_api_secret").val(config.api_secret)
                break;
        }
    } catch (e) {
        console.error("Error parsing provider config:", e)
    }
    
    // Show the appropriate provider fields
    $(".provider-fields").hide()
    $(`#${profile.provider}-fields`).show()
}

function deleteSMSProfile(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the SMS sending profile. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(smsProfiles[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function() {
            return new Promise(function(resolve, reject) {
                api.SMSId.delete(smsProfiles[idx].id)
                    .success(function(msg) {
                        resolve()
                    })
                    .error(function(data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire(
                'SMS Profile Deleted!',
                'This SMS profile has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function() {
            loadSMSProfiles()
        })
    })
}

function loadSMSProfiles() {
    $("#smsProfileTable").hide()
    $("#smsEmptyMessage").hide()
    $("#smsLoading").show()
    api.SMS.get()
        .success(function(ss) {
            smsProfiles = ss
            $("#smsLoading").hide()
            if (smsProfiles.length > 0) {
                $("#smsProfileTable").show()
                smsProfileTable = $("#smsProfileTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                smsProfileTable.clear()
                smsProfileRows = []
                $.each(smsProfiles, function(i, profile) {
                    // Create a unique ID for the balance cell
                    var balanceCellId = "balance-" + profile.id;
                    smsProfileRows.push([
                        escapeHtml(profile.name),
                        profile.provider,
                        "<span id='" + balanceCellId + "' class='balance-cell'><i class='fa fa-spinner fa-spin'></i></span>",
                        moment(profile.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#smsModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Profile' onclick='editSMSProfile(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#smsModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Profile' onclick='copySMSProfile(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Profile' onclick='deleteSMSProfile(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                smsProfileTable.rows.add(smsProfileRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
                
                // Fetch balances asynchronously for each profile
                $.each(smsProfiles, function(i, profile) {
                    fetchSMSBalance(profile.id);
                });
            } else {
                $("#smsEmptyMessage").show()
            }
        })
        .error(function(xhr, status, error) {
            $("#smsLoading").hide()
            errorFlash("Error fetching SMS profiles")
        })
}

// Fetch the balance for a specific SMS profile
function fetchSMSBalance(profileId) {
    api.SMSId.balance(profileId)
        .success(function(data) {
            var balanceText = data.balance.toFixed(2) + " " + data.currency;
            $("#balance-" + profileId).html("<span class='label label-success'>" + balanceText + "</span>");
        })
        .error(function(data) {
            var errorMsg = data.responseJSON ? data.responseJSON.message : "Error";
            $("#balance-" + profileId).html("<span class='label label-danger' title='" + escapeHtml(errorMsg) + "'>Error</span>");
        });
}

// Setup the Send Test Email modal with the current profile
function setupSendTestEmail() {
    var name = $("#name").val();
    var from = $("#from").val();
    $("#test_email_profile_name").text(name);
    $("#test_email_from").val(from);
}

// Setup the Send Test SMS modal with the current profile
function setupSendTestSMS() {
    var name = $("#sms_name").val();
    var from = $("#sms_from").val();
    $("#test_sms_profile_name").text(name);
    $("#test_sms_from").val(from);
}
