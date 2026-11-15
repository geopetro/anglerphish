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
        ["EventDetails", "Event details (captured data)"],
        ["OIDCSecrets", "SSO client secret"]
    ];
    var ENC_LOCKED_REASONS = {
        missing_key: "No encryption key was provided at startup, but encrypted data exists.",
        wrong_key: "The key provided at startup does not match the encrypted data.",
        invalid_key: "The key provided at startup is not a valid encryption key."
    };

    function encMessage(xhr, fallback) {
        return (xhr && xhr.responseJSON && xhr.responseJSON.message) || fallback;
    }

    function encCounts(report) {
        var enc = 0, plain = 0;
        $.each(ENC_FIELDS, function (_, f) {
            var c = (report && report[f[0]]) || {};
            enc += Number(c.Encrypted) || 0;
            plain += Number(c.Plaintext) || 0;
        });
        return { encrypted: enc, plaintext: plain, total: enc + plain };
    }

    function encFieldTable(report) {
        var rows = "";
        $.each(ENC_FIELDS, function (_, f) {
            var c = (report && report[f[0]]) || {};
            rows += "<tr><td>" + f[1] + "</td><td>" + (Number(c.Encrypted) || 0) + "</td><td>" +
                (Number(c.Plaintext) || 0) + "</td></tr>";
        });
        return '<table class="table table-condensed enc-table"><thead><tr><th>Field</th><th>Encrypted</th>' +
            '<th>Not encrypted</th></tr></thead><tbody>' + rows + '</tbody></table>';
    }

    // One row of the encryption panel. tone colours the icon only
    // (success, warning, danger, info or muted).
    function encRow(tone, icon, title, text, side, extra) {
        return '<div class="enc-row">' +
            '<div class="enc-row-main">' +
            (icon ? '<span class="enc-icon enc-' + tone + '"><i class="fa ' + icon + '"></i></span>' : '') +
            '<div class="enc-row-text"><div class="enc-row-title">' + title + '</div>' +
            (text ? '<div class="text-muted">' + text + '</div>' : '') + '</div>' +
            (side ? '<div class="enc-row-side">' + side + '</div>' : '') +
            '</div>' + (extra || '') + '</div>';
    }

    function encDetailsRow(report) {
        return encRow("muted", "fa-table", "Stored data", "Encrypted and unencrypted values per field.",
            '<a href="#encDetails" data-toggle="collapse" class="btn btn-default btn-sm">Show</a>',
            '<div id="encDetails" class="collapse enc-row-extra">' + encFieldTable(report) + '</div>');
    }

    function encResultsHtml(results) {
        if (!results || !results.length) {
            return "";
        }
        return "<pre>" + escapeHtml(results.join("\n")) + "</pre>";
    }

    // Keeps the navbar badge (rendered server-side on page load) in step with
    // status changes made on this page, so unlocking doesn't need a reload.
    // Mirrors templates/base.html: locked wins, then session key.
    function syncEncryptionNavbar(st) {
        var badge = "";
        if (st.state === "locked") {
            badge = '<a class="btn btn-danger btn-sm" href="/settings#encryptionSettings" ' +
                'title="Encrypted data found but no valid encryption key is loaded. Sending with encrypted credentials is blocked. Unlock in Settings.">' +
                '<i class="fa fa-lock"></i> Encryption locked</a>';
        } else if (st.source === "session") {
            badge = '<a class="btn btn-warning btn-sm" href="/settings#encryptionSettings" ' +
                'title="Encryption key is active for this session only. Add it to your environment or secret manager before the next restart.">' +
                '<i class="fa fa-key"></i> Session key</a>';
        }
        var $item = $("#encryption-navbar-item");
        if (!badge) {
            $item.remove();
            return;
        }
        if (!$item.length) {
            $item = $('<li id="encryption-navbar-item"></li>');
            $(".navbar-right").prepend($item);
        }
        $item.html('<div class="encryption-navbar-badge">' + badge + '</div>');
    }

    function renderEncryption() {
        var st = encStatus;
        if (!st) {
            return;
        }
        syncEncryptionNavbar(st);
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
                'Encryption is off. Remove ANGLERPHISH_ENCRYPTION_KEY from your environment, or encryption turns back on with that key at the next restart.</div>';
        }
        var n = encCounts(st.report);
        var rows = "";
        if (st.state === "off") {
            rows += encRow("info", "fa-unlock-alt", "Encryption is off",
                "Protect passwords, provider credentials and captured data in the database.",
                '<button class="btn btn-primary" id="encGenerateBtn"' + dis + '><i class="fa fa-key"></i> Generate key</button>');
        } else if (st.state === "on") {
            var pct = n.total ? Math.floor(n.encrypted * 100 / n.total) : 100;
            var session = st.source !== "environment";
            rows += encRow((session || n.plaintext > 0) ? "warning" : "success", "fa-lock", "Encryption is on",
                session ? "Key held for this session only." : "Key loaded from the environment.",
                '<span class="enc-percent">' + pct + '%</span>',
                '<div class="enc-row-extra"><div class="progress enc-progress"><div class="progress-bar progress-bar-' +
                (pct === 100 ? 'success' : 'warning') + '" style="width: ' + pct + '%;"></div></div>' +
                '<div class="text-muted">' + n.encrypted + ' of ' + n.total + ' stored values encrypted</div></div>');
            if (session) {
                rows += encRow("warning", "fa-key", "Save the key",
                    "Add it to your environment as <code>ANGLERPHISH_ENCRYPTION_KEY</code>. Otherwise it is lost on restart and Anglerphish starts locked.", "");
            }
            if (n.plaintext > 0) {
                rows += encRow("info", "fa-database", n.plaintext + " value(s) not encrypted yet",
                    "Only new data is encrypted automatically.",
                    '<button class="btn btn-default" id="encPreviewEncryptBtn"' + dis + '>Preview</button> ' +
                    '<button class="btn btn-primary" id="encEncryptBtn"' + dis + '>Encrypt existing data</button>');
            }
            rows += '<div id="encResults"></div>';
            rows += encDetailsRow(st.report);
            rows += encRow("muted", "fa-power-off", "Turn off encryption", "Decrypts all stored data and unloads the key.",
                '<button class="btn btn-default" id="encPreviewDecryptBtn"' + dis + '>Preview</button> ' +
                '<button class="btn btn-danger" id="encDisableBtn"' + dis + '>Disable encryption</button>');
        } else if (st.state === "locked") {
            rows += encRow("danger", "fa-exclamation-circle", "Encryption is locked",
                escapeHtml(ENC_LOCKED_REASONS[st.locked_reason] || "The encryption key is unavailable.") +
                (st.plaintext_count > 0 ? ' ' + Number(st.plaintext_count) + ' value(s) were saved unencrypted meanwhile.' : ''), "",
                '<div class="enc-row-extra"><div class="enc-inline">' +
                '<input type="password" class="form-control" id="encUnlockKey" autocomplete="off" spellcheck="false" placeholder="Paste the encryption key" />' +
                '<button class="btn btn-primary" id="encUnlockBtn"' + dis + '>Unlock</button></div></div>');
            rows += encDetailsRow(st.report);
            rows += encRow("muted", "fa-trash", "Key lost?", "Discarding deletes all encrypted data for good.",
                '<a href="#encDiscardSection" data-toggle="collapse" class="btn btn-default">Discard&hellip;</a>',
                '<div id="encDiscardSection" class="collapse enc-row-extra">' +
                '<p>Type <code>' + ENC_DISCARD_PHRASE + '</code> to confirm.</p>' +
                '<div class="enc-inline"><input type="text" class="form-control" id="encDiscardPhrase" autocomplete="off" />' +
                '<button class="btn btn-danger" id="encDiscardBtn" disabled>Discard encrypted data</button></div></div>');
        } else if (st.state === "disabling") {
            rows += encRow("warning", "fa-exclamation-triangle", "Encryption is being disabled",
                "Some values could not be decrypted yet.",
                '<button class="btn btn-danger" id="encRetryDisableBtn"' + dis + '>Retry disable</button>',
                encLastResults && encLastResults.length ? '<div class="enc-row-extra">' + encResultsHtml(encLastResults) + '</div>' : '');
            rows += encDetailsRow(st.report);
        }
        h += '<div class="enc-panel">' + rows + '</div>';
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

    // ── SSO ────────────────────────────────────────────────────────────────
    var ssoStatus = null;       // status string from the last load/save
    var ssoPreset = "generic";  // preset whose defaults the fields may still hold
    var SSO_PRESETS = {
        entra:    { username_from_email: "local_part", allow_unverified_email: true },
        keycloak: { username_from_email: "local_part", allow_unverified_email: false },
        okta:     { username_from_email: "local_part", allow_unverified_email: false },
        generic:  { username_from_email: "local_part", allow_unverified_email: false }
    };
    var SSO_ISSUER_HINTS = {
        entra: "https://login.microsoftonline.com/<tenant-id>/v2.0",
        keycloak: "https://<host>/realms/<realm>",
        okta: "https://<your-org>.okta.com/oauth2/default",
        generic: "https://idp.example.com"
    };
    var SSO_GROUP_HINTS = {
        entra: " Use the group's Object ID (GUID), not its name.",
        keycloak: " Group name, the leading / is optional.",
        okta: "",
        generic: ""
    };
    var SSO_TIPS = {
        ssoEnabledTip: "While SSO is on, only the admin account can sign in with a password. Users must already exist in Anglerphish.",
        ssoClientIdTip: "The client (application) ID registered with your identity provider.",
        ssoClientSecretTip: "Write-only: it is never shown again. Leave empty to keep the stored secret.",
        ssoRedirectTip: "Register this URL with your identity provider.",
        ssoGroupsClaimTip: "The token claim that lists the user's groups. Leave empty to use groups, which most providers send.",
        ssoUsernameMapTip: "How the email from your provider maps to an existing Anglerphish username."
    };

    // ssoTip returns a tooltip icon; its text is set after render with ssoSetTip.
    function ssoTip(id) {
        return ' <i class="fa fa-question-circle" id="' + id + '" data-toggle="tooltip" data-placement="right"></i>';
    }

    function ssoSetTip(id, text) {
        $("#" + id).attr("title", text).tooltip("fixTitle");
    }

    function ssoStatusLine(st, locked) {
        if (st === "enabled") {
            return '<span class="label label-success">Enabled</span>';
        }
        if (st === "unavailable") {
            return '<span class="label label-danger">Unavailable</span> <span class="text-danger">' +
                (locked ? "Client secret unavailable while encryption is locked" : "Saved settings are invalid, SSO is off") + '</span>';
        }
        return '<span class="label label-default">Disabled</span>';
    }

    function ssoApplyHints(provider) {
        $("#ssoIssuer").attr("placeholder", SSO_ISSUER_HINTS[provider] || SSO_ISSUER_HINTS.generic);
        ssoSetTip("ssoIssuerTip", "The issuer URL from your identity provider's OIDC settings. The example in the field shows the format for " +
            $("#ssoProvider option:selected").text() + ".");
        ssoSetTip("ssoGroupTip", "Users must be in this group to sign in." + (SSO_GROUP_HINTS[provider] || ""));
        ssoSetTip("ssoUnverifiedTip", "Skip the email_verified check. " + (provider === "entra" ?
            "Microsoft Entra ID never sends the email_verified claim, so this must be on for Entra." :
            "Only needed for providers that don't send the email_verified claim."));
    }

    // A field is only overwritten when it is empty or still holds the
    // previous preset's default, so values the admin typed are kept.
    function ssoApplyPreset(next) {
        var prev = SSO_PRESETS[ssoPreset] || SSO_PRESETS.generic;
        var def = SSO_PRESETS[next] || SSO_PRESETS.generic;
        var $map = $("#ssoUsernameMap");
        if ($map.val() === "" || $map.val() === prev.username_from_email) {
            $map.val(def.username_from_email);
        }
        var $unv = $("#ssoAllowUnverified");
        if ($unv.prop("checked") === prev.allow_unverified_email) {
            $unv.prop("checked", def.allow_unverified_email);
        }
        ssoPreset = next;
        ssoApplyHints(next);
    }

    function renderSSO(d) {
        ssoStatus = d.status;
        ssoPreset = SSO_PRESETS[d.provider] ? d.provider : "generic";
        var redirect = d.redirect_url || (window.location.origin + "/auth/oidc/callback");
        var secretPh = d.secret_set ? "•••• set — leave empty to keep" : "";
        var opts = "";
        $.each([["entra", "Microsoft Entra ID"], ["keycloak", "Keycloak"], ["okta", "Okta"], ["generic", "Generic"]], function (_, o) {
            opts += '<option value="' + o[0] + '"' + (o[0] === ssoPreset ? " selected" : "") + ">" + o[1] + "</option>";
        });
        var fullEmail = d.username_from_email === "full_email";
        var h = "";
        if (d.encryption_locked) {
            h += '<div class="alert alert-warning"><i class="fa fa-exclamation-triangle"></i> ' +
                'Encryption is locked. A new client secret saved now is stored unencrypted until you unlock. ' +
                '<a href="/settings#encryptionSettings" id="ssoEncLink">DB Encryption settings</a></div>';
        } else if (!d.encryption_enabled) {
            h += '<div class="alert alert-warning"><i class="fa fa-exclamation-triangle"></i> ' +
                'Database encryption is off. The client secret is stored unencrypted. ' +
                '<a href="/settings#encryptionSettings" id="ssoEncLink">DB Encryption settings</a></div>';
        }
        h += '<div class="row"><div class="col-md-12">' +
            '<div class="sso-enable-row">' +
            '<div class="checkbox checkbox-primary"><input type="checkbox" id="ssoEnabled"' + (d.enabled ? " checked" : "") + '><label for="ssoEnabled"><strong>Enable SSO</strong>' + ssoTip("ssoEnabledTip") + '</label></div>' +
            '<div class="sso-status">' + ssoStatusLine(d.status, d.encryption_locked) + '</div>' +
            '</div>' +

            '<div class="row">' +
            '<div class="col-lg-6">' +
            '<div class="panel panel-default">' +
            '<div class="panel-heading"><h3 class="panel-title">Identity provider</h3></div>' +
            '<div class="panel-body">' +
            '<div class="form-group"><label class="control-label" for="ssoProvider">Provider</label>' +
            '<select class="form-control" id="ssoProvider">' + opts + '</select></div>' +
            '<div class="form-group"><label class="control-label" for="ssoIssuer">Issuer URL</label>' + ssoTip("ssoIssuerTip") +
            '<input type="text" class="form-control" id="ssoIssuer" autocomplete="off" spellcheck="false"></div>' +
            '<div class="form-group"><label class="control-label" for="ssoClientId">Client ID</label>' + ssoTip("ssoClientIdTip") +
            '<input type="text" class="form-control" id="ssoClientId" autocomplete="off" spellcheck="false"></div>' +
            '<div class="form-group"><label class="control-label" for="ssoClientSecret">Client secret</label>' + ssoTip("ssoClientSecretTip") +
            '<div class="input-group"><input type="password" class="form-control" id="ssoClientSecret" autocomplete="new-password">' +
            '<span class="input-group-btn"><button class="btn btn-default" type="button" id="ssoSecretToggle" title="Show what you typed"><i class="fa fa-eye"></i></button></span></div></div>' +
            '<div class="form-group sso-last"><label class="control-label" for="ssoRedirectUrl">Redirect URL</label>' + ssoTip("ssoRedirectTip") +
            '<div class="input-group"><input type="text" class="form-control" id="ssoRedirectUrl" autocomplete="off" spellcheck="false">' +
            '<span class="input-group-btn"><button class="btn btn-default" type="button" id="ssoCopyRedirect"><i class="fa fa-copy"></i> Copy</button></span></div></div>' +
            '</div>' +
            '<div class="panel-footer">' +
            '<button class="btn btn-default btn-sm" type="button" id="ssoTestBtn"><i class="fa fa-plug"></i> Test connection</button> ' +
            '<span id="ssoTestResult" class="text-muted">Checks that the issuer URL is reachable and speaks OIDC.</span>' +
            '</div></div></div>' +

            '<div class="col-lg-6">' +
            '<div class="panel panel-default">' +
            '<div class="panel-heading"><h3 class="panel-title">User access</h3></div>' +
            '<div class="panel-body">' +
            '<div class="form-group"><label class="control-label" for="ssoRequiredGroup">Required group</label>' + ssoTip("ssoGroupTip") +
            '<input type="text" class="form-control" id="ssoRequiredGroup" autocomplete="off" spellcheck="false"></div>' +
            '<div class="form-group"><label class="control-label" for="ssoGroupsClaim">Groups claim</label>' + ssoTip("ssoGroupsClaimTip") +
            '<input type="text" class="form-control" id="ssoGroupsClaim" placeholder="groups" autocomplete="off" spellcheck="false"></div>' +
            '<div class="form-group"><label class="control-label" for="ssoUsernameMap">Username mapping</label>' + ssoTip("ssoUsernameMapTip") +
            '<select class="form-control" id="ssoUsernameMap">' +
            '<option value="local_part"' + (fullEmail ? "" : " selected") + '>Part before the @ (jane.doe@example.com becomes jane.doe)</option>' +
            '<option value="full_email"' + (fullEmail ? " selected" : "") + '>Full email address (jane.doe@example.com)</option>' +
            '</select></div>' +
            '<div class="checkbox checkbox-primary sso-last"><input type="checkbox" id="ssoAllowUnverified"' + (d.allow_unverified_email ? " checked" : "") + '><label for="ssoAllowUnverified">Allow unverified email' + ssoTip("ssoUnverifiedTip") + '</label></div>' +
            '</div></div></div>' +
            '</div>' +

            '<div class="text-right">' +
            '<button class="btn btn-default" type="button" id="ssoCancelBtn">Cancel</button> ' +
            '<button class="btn btn-primary" type="button" id="ssoSaveBtn"><i class="fa fa-save"></i> Save changes</button>' +
            '</div>' +
            '</div></div>';
        $("#ssoContent").html(h);
        $("#ssoIssuer").val(d.issuer || "");
        $("#ssoClientId").val(d.client_id || "");
        $("#ssoClientSecret").attr("placeholder", secretPh);
        $("#ssoRedirectUrl").val(redirect);
        $("#ssoRequiredGroup").val(d.required_group || "");
        $("#ssoGroupsClaim").val(d.groups_claim || "");
        $.each(SSO_TIPS, ssoSetTip);
        ssoApplyHints(ssoPreset);
    }

    function loadSSO() {
        api.sso.get()
            .done(renderSSO)
            .fail(function (xhr) {
                errorFlash(escapeHtml(encMessage(xhr, "Failed to load SSO settings")));
            });
    }

    $('a[href="#ssoSettings"]').on("shown.bs.tab", loadSSO);
    if (window.location.hash === "#ssoSettings") {
        loadSSO();
    }

    $("#ssoContent").on("change", "#ssoProvider", function () {
        ssoApplyPreset($(this).val());
    });

    $("#ssoContent").on("click", "#ssoSecretToggle", function () {
        var $in = $("#ssoClientSecret");
        var show = $in.attr("type") === "password";
        $in.attr("type", show ? "text" : "password");
        $(this).find("i").toggleClass("fa-eye", !show).toggleClass("fa-eye-slash", show);
    });

    // Cancel discards unsaved edits by reloading the stored settings.
    $("#ssoContent").on("click", "#ssoCancelBtn", loadSSO);

    $("#ssoContent").on("click", "#ssoEncLink", function (e) {
        e.preventDefault();
        $('a[href="#encryptionSettings"]').tab("show");
    });

    $("#ssoContent").on("click", "#ssoCopyRedirect", function () {
        var input = document.getElementById("ssoRedirectUrl");
        var $btn = $("#ssoCopyRedirect");
        input.select();
        function copied() {
            $btn.html('<i class="fa fa-check"></i> Copied');
            $btn.addClass("btn-success").removeClass("btn-default");
            setTimeout(function () {
                $btn.html('<i class="fa fa-copy"></i> Copy');
                $btn.addClass("btn-default").removeClass("btn-success");
            }, 2000);
        }
        try {
            if (navigator.clipboard && navigator.clipboard.writeText) {
                navigator.clipboard.writeText(input.value).then(copied, function () {
                    errorFlash("Could not copy");
                });
            } else {
                document.execCommand("copy");
                copied();
            }
        } catch (err) {
            errorFlash("Failed to copy URL");
        }
    });

    $("#ssoContent").on("click", "#ssoTestBtn", function () {
        var $out = $("#ssoTestResult");
        $out.attr("class", "text-muted").text("Testing...");
        api.sso.test($("#ssoIssuer").val().trim())
            .done(function (data) {
                $out.attr("class", "text-success").text(data.message || "Issuer found.");
            })
            .fail(function (xhr) {
                $out.attr("class", "text-danger").text(encMessage(xhr, "Test failed"));
            });
    });

    function ssoSave() {
        var payload = {
            enabled: $("#ssoEnabled").prop("checked"),
            provider: $("#ssoProvider").val(),
            issuer: $("#ssoIssuer").val().trim(),
            client_id: $("#ssoClientId").val().trim(),
            client_secret: $("#ssoClientSecret").val(),
            redirect_url: $("#ssoRedirectUrl").val().trim(),
            required_group: $("#ssoRequiredGroup").val().trim(),
            groups_claim: $("#ssoGroupsClaim").val().trim(),
            username_from_email: $("#ssoUsernameMap").val(),
            allow_unverified_email: $("#ssoAllowUnverified").prop("checked")
        };
        var $btns = $("#ssoSaveBtn, #ssoEnableConfirmBtn").prop("disabled", true);
        api.sso.put(payload)
            .always(function () {
                $btns.prop("disabled", false);
            })
            .done(function (data) {
                renderSSO(data);
                successFlash("SSO settings saved");
            })
            .fail(function (xhr) {
                var msg = encMessage(xhr, "Failed to save SSO settings");
                errorFlash(escapeHtml(msg.replace(/^invalid sso settings: /, "")));
            });
    }

    $("#ssoContent").on("click", "#ssoSaveBtn", function () {
        if ($("#ssoEnabled").prop("checked") && ssoStatus !== "enabled") {
            $("#ssoEnableModal").modal("show");
            return;
        }
        ssoSave();
    });

    $("#ssoEnableConfirmBtn").click(function () {
        $("#ssoEnableModal").modal("hide");
        ssoSave();
    });

    // ---------------------------------------------------------------
    // Report Generation settings (admin only)
    // ---------------------------------------------------------------
    function renderReportSettings(d) {
        var h = '<div class="row"><div class="col-lg-8">' +
            '<div class="panel panel-default">' +
            '<div class="panel-heading"><h3 class="panel-title">IP geolocation</h3></div>' +
            '<div class="panel-body">' +
            '<p>Reports show where a click or submission came from. Locations are looked up in a local ' +
            'GeoLite2 database when one is available, so no data leaves this server.</p>' +
            '<div class="form-group"><label class="control-label" for="rsGeoipPath">GeoLite2 database path</label>' +
            '<input type="text" class="form-control" id="rsGeoipPath" autocomplete="off" spellcheck="false" ' +
            'placeholder="e.g. /usr/share/GeoIP/GeoLite2-City.mmdb">' +
            '<p class="help-block">A GeoLite2-City.mmdb file on this server. Leave empty to look in the usual places ' +
            '(the working directory, your home directory and /usr/share/GeoIP).</p></div>' +
            '<div class="checkbox checkbox-primary"><input type="checkbox" id="rsOnlineGeo"' + (d.online_geolocation ? " checked" : "") + '>' +
            '<label for="rsOnlineGeo"><strong>Look up unknown IP locations online</strong></label></div>' +
            '<div id="rsOnlineGeoWarning" class="alert alert-warning"' + (d.online_geolocation ? "" : ' style="display:none"') + '>' +
            '<i class="fa fa-exclamation-triangle"></i> Target IP addresses will be sent to <strong>freeipapi.com</strong>, ' +
            'a third-party service, whenever a report is generated. Check that this is allowed by your engagement ' +
            'and privacy rules before turning it on.</div>' +
            '</div></div>' +

            '<div class="panel panel-default">' +
            '<div class="panel-heading"><h3 class="panel-title">Generation</h3></div>' +
            '<div class="panel-body">' +
            '<div class="form-group"><label class="control-label" for="rsTimeout">Time limit per report (minutes)</label>' +
            '<input type="number" class="form-control" id="rsTimeout" min="1" max="' + d.max_job_timeout_minutes + '">' +
            '<p class="help-block">A report that takes longer fails, so it can\'t block the queue.</p></div>' +
            '<div class="form-group"><label class="control-label" for="rsRetention">Keep reports for (days)</label>' +
            '<input type="number" class="form-control" id="rsRetention" min="1" max="' + d.max_retention_days + '">' +
            '<p class="help-block">Older reports and their files are deleted automatically. Applies to reports generated after you save.</p></div>' +
            '</div></div>' +

            '<div class="text-right">' +
            '<button class="btn btn-default" type="button" id="rsCancelBtn">Cancel</button> ' +
            '<button class="btn btn-primary" type="button" id="rsSaveBtn"><i class="fa fa-save"></i> Save changes</button>' +
            '</div>' +
            '</div></div>';
        $("#reportSettingsContent").html(h);
        $("#rsGeoipPath").val(d.geoip_db_path || "");
        $("#rsTimeout").val(d.job_timeout_minutes);
        $("#rsRetention").val(d.retention_days);
    }

    function loadReportSettings() {
        api.reportSettings.get()
            .done(renderReportSettings)
            .fail(function (xhr) {
                errorFlash(escapeHtml(encMessage(xhr, "Failed to load report settings")));
            });
    }

    $('a[href="#reportGenerationSettings"]').on("shown.bs.tab", loadReportSettings);
    if (window.location.hash === "#reportGenerationSettings") {
        loadReportSettings();
    }

    $("#reportSettingsContent").on("change", "#rsOnlineGeo", function () {
        $("#rsOnlineGeoWarning").toggle($(this).prop("checked"));
    });

    // Cancel discards unsaved edits by reloading the stored settings.
    $("#reportSettingsContent").on("click", "#rsCancelBtn", loadReportSettings);

    $("#reportSettingsContent").on("click", "#rsSaveBtn", function () {
        var payload = {
            online_geolocation: $("#rsOnlineGeo").prop("checked"),
            geoip_db_path: $("#rsGeoipPath").val().trim(),
            job_timeout_minutes: parseInt($("#rsTimeout").val(), 10) || 0,
            retention_days: parseInt($("#rsRetention").val(), 10) || 0
        };
        var $btn = $("#rsSaveBtn").prop("disabled", true);
        api.reportSettings.put(payload)
            .always(function () {
                $btn.prop("disabled", false);
            })
            .done(function (data) {
                renderReportSettings(data);
                successFlash("Report settings saved");
            })
            .fail(function (xhr) {
                var msg = encMessage(xhr, "Failed to save report settings");
                errorFlash(escapeHtml(msg.replace(/^invalid report settings: /, "")));
            });
    });
});
