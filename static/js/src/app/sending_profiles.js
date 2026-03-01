var profiles = []
var smsProfiles = []

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    // Get all headers from the table
    var headers = [];
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        headers.push({
            key: unescapeHtml(header[0]),
            value: unescapeHtml(header[1]),
        })
    })
    
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
        headers: []
    }
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        profile.headers.push({
            key: unescapeHtml(header[0]),
            value: unescapeHtml(header[1]),
        })
    })
    profile.name = $("#name").val()
    profile.interface_type = $("#interface_type").val()
    profile.from_address = $("#from").val()
    profile.host = $("#host").val()
    profile.username = $("#username").val()
    profile.password = $("#password").val()
    profile.ignore_cert_errors = $("#ignore_cert_errors").prop("checked")
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
    $("#headersTable").dataTable().DataTable().clear().draw()
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
    headers = $("#headersTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })

    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    var profile = {}
    if (idx != -1) {
        $("#profileModalLabel").text("Edit Sending Profile")
        profile = profiles[idx]
        $("#name").val(profile.name)
        $("#interface_type").val(profile.interface_type)
        $("#from").val(profile.from_address)
        $("#host").val(profile.host)
        $("#username").val(profile.username)
        $("#password").val(profile.password)
        $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
        $.each(profile.headers, function (i, record) {
            addCustomHeader(record.key, record.value)
        });
    } else {
        $("#profileModalLabel").text("New Sending Profile")
    }
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    var profile = {}
    profile = profiles[idx]
    $("#name").val("Copy of " + profile.name)
    $("#interface_type").val(profile.interface_type)
    $("#from").val(profile.from_address)
    $("#host").val(profile.host)
    $("#username").val(profile.username)
    $("#password").val(profile.password)
    $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
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
                        profile.interface_type,
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

function addCustomHeader(header, value) {
    // Create new data row.
    var newRow = [
        escapeHtml(header),
        escapeHtml(value),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if header already exists.
    var headersTable = headers.DataTable();
    var existingRowIndex = headersTable
        .column(0) // Email column has index of 2
        .data()
        .indexOf(escapeHtml(header));

    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        headersTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        headersTable.row.add(newRow);
    }
    headersTable.draw();
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
    // Code to deal with custom email headers
    $("#addCustomHeader").on('click', function () {
        headerKey = $("#headerKey").val();
        headerValue = $("#headerValue").val();

        if (headerKey == "" || headerValue == "") {
            return false;
        }
        addCustomHeader(headerKey, headerValue);
        // Reset user input.
        $("#headerKey").val('');
        $("#headerValue").val('');
        $("#headerKey").focus();
        return false;
    });
    // Handle Deletion
    $("#headersTable").on("click", "span>i.fa-trash-o", function () {
        headers.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    load()
    loadSMSProfiles()
    
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
