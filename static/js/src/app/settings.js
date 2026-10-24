$(document).ready(function () {
    $('[data-toggle="tooltip"]').tooltip();
    
    // Copy API Key to clipboard
    $("#copyApiKey").click(function() {
        var apiKeyInput = document.getElementById("api_key");
        apiKeyInput.select();
        apiKeyInput.setSelectionRange(0, 99999); // For mobile devices
        
        try {
            // Try modern clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(apiKeyInput.value).then(function() {
                    // Change icon to checkmark temporarily
                    var btn = $("#copyApiKey");
                    btn.html('<i class="fa fa-check"></i>');
                    btn.addClass('btn-success').removeClass('btn-default');
                    setTimeout(function() {
                        btn.html('<i class="fa fa-copy"></i>');
                        btn.addClass('btn-default').removeClass('btn-success');
                    }, 2000);
                });
            } else {
                // Fallback to execCommand
                document.execCommand("copy");
                var btn = $("#copyApiKey");
                btn.html('<i class="fa fa-check"></i>');
                btn.addClass('btn-success').removeClass('btn-default');
                setTimeout(function() {
                    btn.html('<i class="fa fa-copy"></i>');
                    btn.addClass('btn-default').removeClass('btn-success');
                }, 2000);
            }
        } catch (err) {
            errorFlash("Failed to copy API key");
        }
    });

    $("#apiResetForm").submit(function (e) {
        api.reset()
            .success(function (response) {
                user.api_key = response.data
                successFlash(response.message)
                $("#api_key").val(user.api_key)
            })
            .error(function (data) {
                errorFlash(data.message)
            })
        return false
    })
    $("#settingsForm").submit(function (e) {
        $.post("/settings", $(this).serialize())
            .done(function (data) {
                successFlash(data.message)
            })
            .fail(function (data) {
                errorFlash(data.responseJSON.message)
            })
        return false
    })

    // Initialize the IMAP modal
    var imapModal = $("#newIMAPModal");
    var imapForm = $("#imapForm");
    var currentIMAPId = -1;
    
    // Handle showing the IMAP modal
    $("#showNewIMAPModal").click(function() {
        // Reset the form when creating a new configuration
        imapForm[0].reset();
        $("#imapModalLabel").text("New IMAP Configuration");
        currentIMAPId = -1;
        $('#use_tls').prop('checked', true);
        $('#ignorecerterrors').prop('checked', false);
        $('#deletecampaign').prop('checked', true);
        $('#capture_reply_body').prop('checked', true);
        $("#imapfreq").val("60");
        $("#folder").val("INBOX");
        imapModal.modal('show');
    });

    // Validate IMAP settings from the modal
    $("#validateImapButton").click(function() {
        var server = {};
        server.host = $("#imaphost").val();
        server.port = $("#imapport").val();
        server.username = $("#imapusername").val();
        server.password = $("#imappassword").val();
        server.tls = $('#use_tls').prop('checked');
        server.ignore_cert_errors = $('#ignorecerterrors').prop('checked');

        // Basic validation
        if (server.host == "") {
            errorFlash("No IMAP Host specified");
            return false;
        }
        if (server.port == "") {
            errorFlash("No IMAP Port specified");
            return false;
        }
        if (isNaN(server.port) || server.port < 1 || server.port > 65535) {
            errorFlash("Invalid IMAP Port");
            return false;
        }

        var oldHTML = $(this).html();
        // Disable inputs and change button text
        $(this).html("<i class='fa fa-circle-o-notch fa-spin'></i> Testing...");
        $(this).attr("disabled", true);
        
        api.IMAP.validate(server).done(function(data) {
            if (data.success == true) {
                Swal.fire({
                    title: "Success",
                    html: "Logged into <b>" + escapeHtml(server.host) + "</b>",
                    type: "success",
                });
            } else {
                Swal.fire({
                    title: "Failed!",
                    html: "Unable to login to <b>" + escapeHtml(server.host) + "</b>.",
                    type: "error",
                    showCancelButton: true,
                    cancelButtonText: "Close",
                    confirmButtonText: "More Info",
                    confirmButtonColor: "#428bca",
                    allowOutsideClick: false,
                }).then(function(result) {
                    if (result.value) {
                        Swal.fire({
                            title: "Error:",
                            text: data.message,
                        });
                    }
                });
            }
        })
        .fail(function() {
            Swal.fire({
                title: "Failed!",
                text: "An unexpected error occurred.",
                type: "error",
            });
        })
        .always(function() {
            // Re-enable the button and restore text
            $("#validateImapButton").attr("disabled", false);
            $("#validateImapButton").html(oldHTML);
        });
    });

    // Handle IMAP form submission
    imapForm.submit(function(e) {
        e.preventDefault();
        
        var imapSettings = {};
        imapSettings.name = $("#imapname").val();
        imapSettings.host = $("#imaphost").val();
        imapSettings.port = $("#imapport").val();
        imapSettings.username = $("#imapusername").val();
        imapSettings.password = $("#imappassword").val();
        // New IMAP configurations are enabled by default
        imapSettings.enabled = true;
        imapSettings.tls = $('#use_tls').prop('checked');

        // Advanced settings
        imapSettings.folder = $("#folder").val();
        imapSettings.imap_freq = $("#imapfreq").val();
        imapSettings.restrict_domain = $("#restrictdomain").val();
        imapSettings.ignore_cert_errors = $('#ignorecerterrors').prop('checked');
        imapSettings.delete_reported_campaign_email = $('#deletecampaign').prop('checked');
        imapSettings.tracking_type = parseInt($("#trackingtype").val());
        imapSettings.capture_reply_body = $('#capture_reply_body').prop('checked');
        
        // Basic validation
        if (imapSettings.name == "") {
            errorFlash("Please provide a name for this IMAP configuration");
            return false;
        }
        if (imapSettings.host == "") {
            errorFlash("No IMAP Host specified");
            return false;
        }
        if (imapSettings.port == "") {
            errorFlash("No IMAP Port specified");
            return false;
        }
        if (isNaN(imapSettings.port) || imapSettings.port < 1 || imapSettings.port > 65535) { 
            errorFlash("Invalid IMAP Port");
            return false;
        }
        if (imapSettings.imap_freq == "") {
            imapSettings.imap_freq = "60";
        }

        // Determine if we're creating a new config or updating an existing one
        var request;
        if (currentIMAPId == -1) {
            // Creating a new configuration
            request = api.IMAP.post(imapSettings);
        } else {
            // Updating an existing configuration
            imapSettings.id = currentIMAPId;
            request = api.IMAPId.put(imapSettings);
        }

        // Process the request
        request.done(function (data) {
            if (data.success) {
                successFlash(currentIMAPId == -1 ? 
                    "Successfully created IMAP configuration." : 
                    "Successfully updated IMAP configuration.");
                imapModal.modal('hide');
                loadIMAPSettings();
            } else {
                errorFlash("Unable to save IMAP settings: " + data.message);
            }
        })
        .fail(function (data) {
            errorFlash(data.responseJSON ? data.responseJSON.message : "An unexpected error occurred");
        });
        
        return false;
    });

    // Delete IMAP configuration
    $(document).on('click', '.delete-imap', function() {
        var imapId = $(this).attr('data-imap-id');
        var imapName = $(this).attr('data-imap-name');
        
        Swal.fire({
            title: "Are you sure?",
            text: "This will delete the IMAP configuration '" + imapName + "'",
            type: "warning",
            showCancelButton: true,
            confirmButtonText: "Delete",
            confirmButtonColor: "#d9534f",
            reverseButtons: true,
            allowOutsideClick: false,
        }).then(function(result) {
            if (result.value) {
                api.IMAPId.delete(imapId)
                .done(function(data) {
                    if (data.success) {
                        successFlash("IMAP configuration deleted successfully!");
                        loadIMAPSettings();
                    } else {
                        errorFlash(data.message || "An error occurred");
                    }
                })
                .fail(function(data) {
                    errorFlash(data.responseJSON ? data.responseJSON.message : "An unexpected error occurred");
                });
            }
        });
    });

    // Edit IMAP configuration
    $(document).on('click', '.edit-imap', function() {
        var imapId = $(this).attr('data-imap-id');
        
        // Get the specific IMAP configuration
        api.IMAPId.get(imapId)
        .done(function(imap) {
            // Set the form values
            $("#imapModalLabel").text("Edit IMAP Configuration");
            $("#imapname").val(imap.name);
            $("#imaphost").val(imap.host);
            $("#imapport").val(imap.port);
            $("#imapusername").val(imap.username);
            $("#imappassword").val(imap.password);
            $('#use_tls').prop('checked', imap.tls);
            $('#ignorecerterrors').prop('checked', imap.ignore_cert_errors);
            $("#folder").val(imap.folder || "INBOX");
            $("#restrictdomain").val(imap.restrict_domain || "");
            $('#deletecampaign').prop('checked', imap.delete_reported_campaign_email);
            $('#imapfreq').val(imap.imap_freq || "60");
            $('#trackingtype').val(imap.tracking_type || 0);
            $('#capture_reply_body').prop('checked', imap.capture_reply_body);
            
            // Set the current IMAP ID
            currentIMAPId = imap.id;
            
            // Show the modal
            imapModal.modal('show');
        })
        .fail(function() {
            errorFlash("Failed to get IMAP configuration");
        });
    });

    $("#reporttab").click(function() {
        loadIMAPSettings();
    });

    $("#advanced").click(function() {
        $("#advancedarea").toggle();
    });

    function loadIMAPSettings() {
        api.IMAP.get()
        .success(function (imaps) {
            // Clear the table
            $("#imap-configs tbody").empty();
            
            if (imaps.length == 0) {
                // No configurations yet
                $("#imap-configs tbody").append(
                    '<tr><td colspan="5" class="text-center">No IMAP configurations found. Click "Add Configuration" to create one.</td></tr>'
                );
                $("#imap-instructions").show();
            } else {
                $("#imap-instructions").hide();
                
                // Add each configuration to the table
                $.each(imaps, function(i, imap) {
                    var lastLogin = imap.last_login ? moment.utc(imap.last_login).fromNow() : 'Never';
                    var row = $('<tr/>');
                    
                    // Add cells
                    row.append($('<td/>').text(imap.name));
                    row.append($('<td/>').text(imap.host + ':' + imap.port));
                    row.append($('<td/>').text(imap.username));
                    
                    // Create status cell
                    var statusCell = $('<td/>');
                    var statusLabel = $('<span/>')
                        .addClass('label')
                        .addClass(imap.enabled ? 'label-success' : 'label-danger')
                        .text(imap.enabled ? 'Enabled' : 'Disabled')
                        .attr('id', 'status-label-' + imap.id);
                    
                    statusCell.append(statusLabel);
                    row.append(statusCell);
                    
                    row.append($('<td/>').text(lastLogin));
                    
                    // Add actions
                    var actions = $('<td/>');
                    
                    // Add toggle button in actions column
                    var toggleBtn = $('<button/>')
                        .addClass('btn btn-sm toggle-imap-status')
                        .addClass(imap.enabled ? 'btn-warning' : 'btn-success')
                        .attr('data-imap-id', imap.id)
                        .attr('data-imap-enabled', imap.enabled)
                        .attr('title', imap.enabled ? 'Disable' : 'Enable')
                        .html('<i class="fa fa-' + (imap.enabled ? 'pause' : 'play') + '"></i> ' + 
                              (imap.enabled ? 'Disable' : 'Enable'));
                    
                    actions.append(toggleBtn);
                    actions.append(' ');
                    
                    // Edit button
                    actions.append(
                        $('<button/>')
                            .addClass('btn btn-sm btn-primary edit-imap')
                            .attr('data-imap-id', imap.id)
                            .html('<i class="fa fa-pencil"></i> Edit')
                    );
                    actions.append(' ');
                    
                    // Delete button
                    actions.append(
                        $('<button/>')
                            .addClass('btn btn-sm btn-danger delete-imap')
                            .attr('data-imap-id', imap.id)
                            .attr('data-imap-name', imap.name)
                            .html('<i class="fa fa-trash-o"></i> Delete')
                    );
                    row.append(actions);
                    
                    // Add the row to the table
                    $("#imap-configs tbody").append(row);
                });
            }
        })
        .error(function () {
            errorFlash("Error fetching IMAP settings");
        });
    }

    var use_map = localStorage.getItem('gophish.use_map');
    $("#use_map").prop('checked', JSON.parse(use_map));
    $("#use_map").on('change', function () {
        localStorage.setItem('gophish.use_map', JSON.stringify(this.checked));
    });

    // Theme selector
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
        localStorage.setItem('gophish.theme', currentTheme);
    }
    
    // Set the dropdown to the current theme
    $("#theme_selector").val(currentTheme);
    
    // Handle theme changes
    $("#theme_selector").on('change', function () {
        var selectedTheme = this.value;
        localStorage.setItem('gophish.theme', selectedTheme);
        applyTheme(selectedTheme);
    });

    // Apply theme function
    function applyTheme(theme) {
        // Remove all theme classes
        document.body.classList.remove('dark-theme', 'crimson-theme', 'goldphish-theme', 'lagocephalus-theme', 'light-sand-theme', 'matrix-theme');
        document.documentElement.classList.remove('dark-theme', 'crimson-theme', 'goldphish-theme', 'lagocephalus-theme', 'light-sand-theme', 'matrix-theme');

        // Apply the selected theme
        if (theme === 'dark-teal') {
            document.body.classList.add('dark-theme');
            document.documentElement.classList.add('dark-theme');
        } else if (theme === 'dark-crimson') {
            document.body.classList.add('crimson-theme');
            document.documentElement.classList.add('crimson-theme');
        } else if (theme === 'goldphish') {
            document.body.classList.add('goldphish-theme');
            document.documentElement.classList.add('goldphish-theme');
        } else if (theme === 'lagocephalus') {
            document.body.classList.add('lagocephalus-theme');
            document.documentElement.classList.add('lagocephalus-theme');
        } else if (theme === 'light-sand') {
            document.body.classList.add('light-sand-theme');
            document.documentElement.classList.add('light-sand-theme');
        } else if (theme === 'matrix') {
            document.body.classList.add('matrix-theme');
            document.documentElement.classList.add('matrix-theme');
        }
        // 'default' theme has no classes, so light theme is shown
    }

    // Apply theme on page load
    applyTheme(currentTheme);

    // Toggle IMAP status (enabled/disabled)
    $(document).on('click', '.toggle-imap-status', function(e) {
        e.preventDefault();
        var btn = $(this);
        var imapId = btn.attr('data-imap-id');
        var currentlyEnabled = btn.attr('data-imap-enabled') === 'true';
        
        // Get the current IMAP config
        api.IMAPId.get(imapId)
        .done(function(imap) {
            // Update the enabled status - ensure it's a boolean
            imap.enabled = !currentlyEnabled;
            
            // Debug logging
            console.log("Updating IMAP configuration:", imapId);
            console.log("Current enabled status:", currentlyEnabled);
            console.log("New enabled status:", imap.enabled);
            
            // Make sure we're sending a proper boolean, not a string
            if (typeof imap.enabled !== 'boolean') {
                imap.enabled = imap.enabled === true || imap.enabled === "true";
            }
            
            // Save the updated config
            api.IMAPId.put(imap)
            .done(function(data) {
                if (data.success) {
                    // Force a reload to ensure everything is updated properly
                    loadIMAPSettings();
                    
                    successFlash(imap.enabled ? 
                        "IMAP monitoring enabled! It may take up to 30 seconds for changes to take effect." : 
                        "IMAP monitoring disabled! It may take up to 30 seconds for changes to take effect.");
                    
                    // Double-check that the change was applied correctly by getting the config again
                    setTimeout(function() {
                        api.IMAPId.get(imapId)
                        .done(function(updatedConfig) {
                            console.log("Verified config after update - enabled status:", updatedConfig.enabled);
                            if (updatedConfig.enabled !== imap.enabled) {
                                errorFlash("Warning: The server didn't save the enabled status correctly. Please try again.");
                            }
                        });
                    }, 1000);
                } else {
                    errorFlash(data.message || "An error occurred");
                }
            })
            .fail(function(data) {
                errorFlash(data.responseJSON ? data.responseJSON.message : "An unexpected error occurred");
            });
        })
        .fail(function() {
            errorFlash("Failed to get IMAP configuration");
        });
    });

    // Initialize everything
    loadIMAPSettings();

    // ── 404 Error Page Editor ──────────────────────────────────────────────
    // Load the current 404 page HTML into the textarea when the tab is shown
    $('a[href="#errorPageSettings"]').on('shown.bs.tab', function () {
        load404PageContent();
    });

    function load404PageContent() {
        api.errorPages.get404()
        .done(function (data) {
            $("#error_page_html").val(data.html);
        })
        .fail(function () {
            errorFlash("Failed to load 404 page content");
        });
    }

    // Save 404 page
    $("#save404PageBtn").click(function () {
        var html = $("#error_page_html").val().trim();
        if (html === "") {
            errorFlash("404 page HTML cannot be empty");
            return;
        }
        api.errorPages.put404(html)
        .done(function (data) {
            if (data.success) {
                successFlashFade(data.message, 5);
            } else {
                errorFlash(data.message);
            }
        })
        .fail(function (xhr) {
            var msg = xhr.responseJSON ? xhr.responseJSON.message : "Failed to save 404 page";
            errorFlash(msg);
        });
    });

    // Reset 404 page to default
    $("#reset404PageBtn").click(function () {
        Swal.fire({
            title: "Reset 404 Page?",
            text: "This will replace the current 404 page with the default content.",
            type: "warning",
            showCancelButton: true,
            confirmButtonText: "Reset",
            confirmButtonColor: "#d9534f",
            reverseButtons: true,
            allowOutsideClick: false,
        }).then(function (result) {
            if (result.value) {
                api.errorPages.reset404()
                .done(function (data) {
                    successFlashFade(data.message || "404 page reset to default", 5);
                    $("#error_page_html").val(data.html);
                })
                .fail(function (xhr) {
                    var msg = xhr.responseJSON ? xhr.responseJSON.message : "Failed to reset 404 page";
                    errorFlash(msg);
                });
            }
        });
    });

    // Preview 404 page in a modal
    $("#preview404PageBtn").click(function () {
        var html = $("#error_page_html").val();
        var previewFrame = document.getElementById("preview404Frame");
        var doc = previewFrame.contentDocument || previewFrame.contentWindow.document;
        doc.open();
        doc.write(html);
        doc.close();
        $("#preview404Modal").modal("show");
    });

    // Load on initial tab activation if the hash is already #errorPageSettings
    if (window.location.hash === "#errorPageSettings") {
        load404PageContent();
    }

    // Global Variables tab
    function loadGlobalVariables() {
        api.globalVariables.get()
        .done(function (data) {
            $("#gv_first_name").val(data.first_name || "");
            $("#gv_last_name").val(data.last_name || "");
            $("#gv_email").val(data.email || "");
            $("#gv_phone").val(data.phone || "");
            $("#gv_position").val(data.position || "");
            $("#gv_custom").val(data.custom || "");
        })
        .fail(function () {
            errorFlash("Failed to load global variables");
        });
    }

    $("a[href='#globalVariablesSettings']").on("shown.bs.tab", function () {
        loadGlobalVariables();
    });

    $("#saveGlobalVariablesBtn").click(function () {
        var vars = {
            first_name: $("#gv_first_name").val(),
            last_name:  $("#gv_last_name").val(),
            email:      $("#gv_email").val(),
            phone:      $("#gv_phone").val(),
            position:   $("#gv_position").val(),
            custom:     $("#gv_custom").val()
        };
        api.globalVariables.put(vars)
        .done(function (data) {
            if (data.success) {
                successFlashFade(data.message, 5);
            } else {
                errorFlash(data.message);
            }
        })
        .fail(function (xhr) {
            var msg = xhr.responseJSON ? xhr.responseJSON.message : "Failed to save global variables";
            errorFlash(msg);
        });
    });

    if (window.location.hash === "#globalVariablesSettings") {
        loadGlobalVariables();
    }

    // ── Encryption ─────────────────────────────────────────────────────────
    var encStatus = null;
    var encLastResults = null;   // results from the last incomplete disable
    var encDisabledNotice = false;
    var encGeneratedKey = null;  // held only while the key modal is open
    var ENC_DISCARD_PHRASE = "DISCARD ENCRYPTED DATA";
    var ENC_FIELDS = [
        ["SMTPPasswords", "SMTP passwords"],
        ["SMSConfigs", "SMS provider configs"],
        ["IMAPPasswords", "IMAP passwords"],
        ["EventDetails", "Event details (captured data)"]
    ];
    var ENC_LOCKED_REASONS = {
        missing_key: "No encryption key was provided at startup, but encrypted data exists.",
        wrong_key: "The key provided at startup does not match the encrypted data.",
        invalid_key: "The key provided at startup is not a valid encryption key."
    };

    function encMessage(xhr, fallback) {
        return (xhr && xhr.responseJSON && xhr.responseJSON.message) || fallback;
    }

    function encFieldTable(report) {
        var rows = "";
        $.each(ENC_FIELDS, function (_, f) {
            var c = (report && report[f[0]]) || {};
            rows += "<tr><td>" + f[1] + "</td><td>" + (c.Encrypted || 0) + "</td><td>" +
                (c.Plaintext || 0) + "</td><td>" + (c.Empty || 0) + "</td><td>" + (c.Total || 0) + "</td></tr>";
        });
        return '<table class="table table-striped table-condensed"><thead><tr><th>Field</th><th>Encrypted</th>' +
            '<th>Plaintext</th><th>Empty</th><th>Total</th></tr></thead><tbody>' + rows + '</tbody></table>';
    }

    function encResultsHtml(results) {
        if (!results || !results.length) {
            return "";
        }
        return "<pre>" + escapeHtml(results.join("\n")) + "</pre>";
    }

    function renderEncryption() {
        var st = encStatus;
        if (!st) {
            return;
        }
        var dis = st.migration_running ? " disabled" : "";
        var h = "";
        if (st.startup_warning) {
            h += '<div class="alert alert-warning"><i class="fa fa-exclamation-triangle"></i> ' +
                escapeHtml(st.startup_warning) + '</div>';
        }
        if (st.migration_running) {
            h += '<div class="alert alert-info"><i class="fa fa-spinner fa-spin"></i> A migration is running. Actions are disabled until it finishes.</div>';
        }
        if (encDisabledNotice && st.state === "off") {
            h += '<div class="alert alert-warning"><i class="fa fa-info-circle"></i> ' +
                'Encryption is off. Remove ANGLERPHISH_ENCRYPTION_KEY from your environment or secret manager — ' +
                'if it is still set at the next restart, encryption will turn back on with that key.</div>';
        }
        if (st.state === "off") {
            h += '<div class="alert alert-info"><p><i class="fa fa-info-circle"></i> Encryption is <strong>off</strong>. ' +
                'Turning it on encrypts SMTP passwords, SMS provider configs, IMAP passwords and captured event details at rest.</p></div>' +
                '<button class="btn btn-primary" id="encGenerateBtn"' + dis + '><i class="fa fa-key"></i> Generate key</button>';
        } else if (st.state === "on") {
            var badge = st.source === "environment" ? "environment" : "session only";
            h += '<p>Encryption is <strong>on</strong>. Key source: <span class="label ' +
                (st.source === "environment" ? "label-success" : "label-warning") + '">' + badge + '</span></p>';
            if (st.source !== "environment") {
                h += '<div class="alert alert-warning"><i class="fa fa-exclamation-triangle"></i> ' +
                    'The key is held in memory only. Set ANGLERPHISH_ENCRYPTION_KEY before the next restart or the system will start locked.</div>';
            }
            h += encFieldTable(st.report);
            if (st.plaintext_count > 0) {
                h += '<div class="alert alert-warning"><i class="fa fa-exclamation-triangle"></i> ' + st.plaintext_count +
                    ' value(s) are still stored as plaintext. Run <strong>Encrypt existing data</strong> to encrypt them.</div>';
            }
            h += '<div id="encResults"></div>' +
                '<button class="btn btn-default" id="encPreviewEncryptBtn"' + dis + '>Preview encryption</button> &nbsp;' +
                '<button class="btn btn-primary" id="encEncryptBtn"' + dis + '>Encrypt existing data</button> &nbsp;' +
                '<button class="btn btn-default" id="encPreviewDecryptBtn"' + dis + '>Preview decryption</button> &nbsp;' +
                '<button class="btn btn-danger" id="encDisableBtn"' + dis + '>Disable encryption</button>';
        } else if (st.state === "locked") {
            h += '<div class="alert alert-danger"><p><i class="fa fa-lock"></i> Encryption is <strong>locked</strong>. ' +
                escapeHtml(ENC_LOCKED_REASONS[st.locked_reason] || "The encryption key is unavailable.") + '</p>';
            if (st.plaintext_count > 0) {
                h += '<p>' + st.plaintext_count + ' value(s) are stored as plaintext.</p>';
            }
            h += '</div>' +
                '<div class="form-group"><label for="encUnlockKey" class="control-label">Encryption key</label>' +
                '<input type="password" class="form-control" id="encUnlockKey" autocomplete="off" spellcheck="false" style="max-width: 480px;" /></div>' +
                '<button class="btn btn-primary" id="encUnlockBtn"' + dis + '><i class="fa fa-unlock"></i> Unlock</button>' +
                '<hr><a href="#encDiscardSection" data-toggle="collapse" class="text-danger">Discard encrypted data</a>' +
                '<div id="encDiscardSection" class="collapse"><br /><div class="alert alert-danger">' +
                '<p><i class="fa fa-exclamation-triangle"></i> This permanently clears every encrypted value. It cannot be undone. The following will be lost:</p>' +
                encFieldTable(st.report) +
                '<p>Type <code>' + ENC_DISCARD_PHRASE + '</code> to confirm.</p>' +
                '<input type="text" class="form-control" id="encDiscardPhrase" autocomplete="off" style="max-width: 480px;" /><br />' +
                '<button class="btn btn-danger" id="encDiscardBtn" disabled><i class="fa fa-trash"></i> Discard encrypted data</button></div></div>';
        } else if (st.state === "disabling") {
            h += '<div class="alert alert-warning"><p><i class="fa fa-exclamation-triangle"></i> Encryption is <strong>being disabled</strong>, but some values could not be decrypted. Retry to finish.</p></div>' +
                encResultsHtml(encLastResults) + encFieldTable(st.report) +
                '<button class="btn btn-danger" id="encRetryDisableBtn"' + dis + '>Retry disable</button>';
        }
        $("#encryptionContent").html(h);
    }

    function loadEncryption() {
        api.encryption.status()
            .done(function (st) {
                encStatus = st;
                renderEncryption();
            })
            .fail(function (xhr) {
                errorFlash(encMessage(xhr, "Failed to load encryption status"));
            });
    }

    function encApply(st) {
        encStatus = st;
        renderEncryption();
    }

    $('a[href="#encryptionSettings"]').on("shown.bs.tab", loadEncryption);
    if (window.location.hash === "#encryptionSettings") {
        loadEncryption();
    }

    function encClearKeyModal() {
        encGeneratedKey = null;
        $("#encGeneratedKey").val("");
        $("#encConfirmKey").val("");
        $("#encEnableBtn").prop("disabled", true);
    }
    $("#encKeyModal").on("hidden.bs.modal", encClearKeyModal);

    $("#encryptionContent").on("click", "#encGenerateBtn", function () {
        api.encryption.generate()
            .done(function (data) {
                encGeneratedKey = data.key;
                $("#encGeneratedKey").val(data.key);
                $("#encConfirmKey").val("");
                $("#encEnableBtn").prop("disabled", true);
                $("#encKeyModal").modal("show");
            })
            .fail(function (xhr) {
                errorFlash(encMessage(xhr, "Failed to generate key"));
            });
    });

    $("#encConfirmKey").on("input", function () {
        $("#encEnableBtn").prop("disabled", !encGeneratedKey || $(this).val() !== encGeneratedKey);
    });

    $("#encCopyKey").click(function () {
        var input = document.getElementById("encGeneratedKey");
        input.select();
        try {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(input.value);
            } else {
                document.execCommand("copy");
            }
        } catch (err) {
            errorFlash("Failed to copy key");
        }
    });

    $("#encEnableBtn").click(function () {
        var key = encGeneratedKey;
        if (!key || $("#encConfirmKey").val() !== key) {
            return;
        }
        $("#encEnableBtn").prop("disabled", true);
        api.encryption.enable(key)
            .done(function (st) {
                encClearKeyModal();
                $("#encKeyModal").modal("hide");
                encDisabledNotice = false;
                encApply(st);
                successFlashFade("Encryption is on.", 5);
            })
            .fail(function (xhr) {
                $("#encEnableBtn").prop("disabled", false);
                errorFlash(encMessage(xhr, "Failed to turn on encryption"));
            });
    });

    function encMigrate(direction, dryRun) {
        return api.encryption.migrate(direction, dryRun)
            .done(function (data) {
                if (dryRun) {
                    $("#encResults").html(encResultsHtml(data.results));
                } else {
                    encApply(data.status);
                    $("#encResults").html(encResultsHtml(data.results));
                    successFlashFade("Existing data encrypted.", 5);
                }
            })
            .fail(function (xhr) {
                errorFlash(encMessage(xhr, "Migration failed"));
            });
    }

    $("#encryptionContent").on("click", "#encPreviewEncryptBtn", function () { encMigrate("encrypt", true); });
    $("#encryptionContent").on("click", "#encEncryptBtn", function () { encMigrate("encrypt", false); });
    $("#encryptionContent").on("click", "#encPreviewDecryptBtn", function () { encMigrate("decrypt", true); });

    $("#encryptionContent").on("click", "#encDisableBtn", function () {
        api.encryption.migrate("decrypt", true)
            .done(function (data) {
                $("#encDisablePreview").text((data.results || []).join("\n"));
                $("#encDisableModal").modal("show");
            })
            .fail(function (xhr) {
                errorFlash(encMessage(xhr, "Failed to preview decryption"));
            });
    });

    function encDisable() {
        api.encryption.disable()
            .done(function (data) {
                encLastResults = null;
                encDisabledNotice = true;
                $("#encDisableModal").modal("hide");
                encApply(data.status);
            })
            .fail(function (xhr) {
                $("#encDisableModal").modal("hide");
                var body = xhr.responseJSON || {};
                encLastResults = body.data && body.data.results ? body.data.results : null;
                errorFlash(encMessage(xhr, "Failed to disable encryption"));
                loadEncryption();
            });
    }
    $("#encDisableConfirmBtn").click(encDisable);
    $("#encryptionContent").on("click", "#encRetryDisableBtn", encDisable);

    $("#encryptionContent").on("click", "#encUnlockBtn", function () {
        var input = $("#encUnlockKey");
        api.encryption.unlock(input.val())
            .done(function (st) {
                input.val("");
                encApply(st);
                successFlashFade("Encryption unlocked.", 5);
            })
            .fail(function (xhr) {
                input.val("");
                errorFlash(encMessage(xhr, "Failed to unlock encryption"));
            });
    });

    $("#encryptionContent").on("input", "#encDiscardPhrase", function () {
        $("#encDiscardBtn").prop("disabled", $(this).val() !== ENC_DISCARD_PHRASE || encStatus.migration_running);
    });

    $("#encryptionContent").on("click", "#encDiscardBtn", function () {
        api.encryption.discard($("#encDiscardPhrase").val())
            .done(function (data) {
                encDisabledNotice = false;
                encApply(data.status);
                successFlash("Encrypted data discarded. Encryption is off. If ANGLERPHISH_ENCRYPTION_KEY is still set, remove it — or keep it knowingly: at the next restart it will become the new encryption key.");
            })
            .fail(function (xhr) {
                errorFlash(encMessage(xhr, "Failed to discard encrypted data"));
            });
    });
});
