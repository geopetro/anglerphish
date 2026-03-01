// SMSTemplates is the list of SMS templates
var smsTemplates = []
var smsTemplateTable = null
var selectedSMSTemplates = {}  // Map of template id -> true for selected templates

// Update the selection count display and button visibility
function updateSMSTemplateSelectionUI() {
    var count = Object.keys(selectedSMSTemplates).length;
    $('#selectedSMSTemplateCount').text(count);
    if (count > 0) {
        $('#deleteSelectedSMSTemplates').show();
    } else {
        $('#deleteSelectedSMSTemplates').hide();
    }
}

// Clear all selections
function clearSMSTemplateSelections() {
    selectedSMSTemplates = {};
    $('input.sms-template-checkbox').prop('checked', false);
    $('#selectAllSMSTemplates').prop('checked', false).prop('indeterminate', false);
    updateSMSTemplateSelectionUI();
}

// Handle individual checkbox change
function handleSMSTemplateCheckboxChange(templateId) {
    var checkbox = $('input.sms-template-checkbox[data-id="' + templateId + '"]');
    if (checkbox.is(':checked')) {
        selectedSMSTemplates[templateId] = true;
    } else {
        delete selectedSMSTemplates[templateId];
    }
    updateSMSTemplateSelectionUI();
    updateSelectAllSMSTemplatesCheckbox();
}

// Update select all checkbox state
function updateSelectAllSMSTemplatesCheckbox() {
    if (!smsTemplateTable) return;
    var allCheckboxes = $(smsTemplateTable.table().body()).find('input.sms-template-checkbox');
    var checkedCount = allCheckboxes.filter(':checked').length;
    var totalCount = allCheckboxes.length;
    
    if (totalCount === 0 || checkedCount === 0) {
        $('#selectAllSMSTemplates').prop('checked', false).prop('indeterminate', false);
    } else if (checkedCount === totalCount) {
        $('#selectAllSMSTemplates').prop('checked', true).prop('indeterminate', false);
    } else {
        $('#selectAllSMSTemplates').prop('checked', false).prop('indeterminate', true);
    }
}

// Handle select all checkbox
function handleSelectAllSMSTemplates() {
    if (!smsTemplateTable) return;
    var isChecked = $('#selectAllSMSTemplates').is(':checked');
    var allCheckboxes = $(smsTemplateTable.table().body()).find('input.sms-template-checkbox');
    
    allCheckboxes.each(function() {
        $(this).prop('checked', isChecked);
        var templateId = $(this).data('id');
        if (isChecked) {
            selectedSMSTemplates[templateId] = true;
        } else {
            delete selectedSMSTemplates[templateId];
        }
    });
    updateSMSTemplateSelectionUI();
}

// Delete selected SMS templates
function deleteSelectedSMSTemplates() {
    var ids = Object.keys(selectedSMSTemplates).map(function(id) { return parseInt(id); });
    if (ids.length === 0) return;
    
    var confirmText = ids.length === 1 
        ? "Delete 1 SMS template?" 
        : "Delete " + ids.length + " SMS templates?";
    
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
                api.smsTemplates.bulkDelete(ids)
                    .success(function (msg) { resolve(msg) })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('SMS Templates Deleted!', result.value.message, 'success');
            selectedSMSTemplates = {};
            $('button:contains("OK")').on('click', function () { location.reload() })
        }
    })
}
window.deleteSelectedSMSTemplates = deleteSelectedSMSTemplates;

// Save attempts to POST or PUT to /sms_templates/
function saveSMSTemplate(idx) {
    var sms_template = {
        text: $("#smsText").val(),
        from: $("#smsFrom").val()
    }
    sms_template.name = $("#smsName").val()
    if (idx != -1) {
        sms_template.id = smsTemplates[idx].id
        api.smsTemplateId.put(sms_template)
            .success(function (data) {
                successFlash("SMS template edited successfully!")
                loadSMSTemplates()
                dismissSMS()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the template
        api.smsTemplates.post(sms_template)
            .success(function (data) {
                successFlash("SMS template added successfully!")
                loadSMSTemplates()
                dismissSMS()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismissSMS() {
    $("#smsModal\\.flashes").empty()
    $("#smsName").val("")
    $("#smsFrom").val("")
    $("#smsText").val("")
    $("#smsModal").modal('hide')
}

function deleteSMSTemplate(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the SMS template. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(smsTemplates[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.smsTemplateId.delete(smsTemplates[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire(
                'SMS Template Deleted!',
                'This SMS template has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function editSMSTemplate(idx) {
    $("#smsModalSubmit").unbind('click').click(function () {
        saveSMSTemplate(idx)
    })
    var sms_template = {}
    if (idx != -1) {
        $("#smsTemplateModalLabel").text("Edit SMS Template")
        sms_template = smsTemplates[idx]
        $("#smsName").val(sms_template.name)
        $("#smsFrom").val(sms_template.from || "")
        $("#smsText").val(sms_template.text)
    } else {
        $("#smsTemplateModalLabel").text("New SMS Template")
    }
    
    // Update character count
    updateSMSCharCount();
    
    // Add event listener for text changes
    $("#smsText").on('input', function() {
        updateSMSCharCount();
    });
}

function updateSMSCharCount() {
    var text = $("#smsText").val();
    var charCount = text.length;
    var smsCount = Math.ceil(charCount / 160);
    
    $("#smsCharCount").text(charCount);
    $("#smsCount").text(smsCount);
}

function copySMSTemplate(idx) {
    $("#smsModalSubmit").unbind('click').click(function () {
        saveSMSTemplate(-1)
    })
    var sms_template = smsTemplates[idx]
    $("#smsName").val("Copy of " + sms_template.name)
    $("#smsFrom").val(sms_template.from || "")
    $("#smsText").val(sms_template.text)
    
    // Update character count
    updateSMSCharCount();
}

// Preview an SMS template with sample data
function previewSMSTemplate(idx) {
    var template = smsTemplates[idx];
    
    // Set the modal title to include the template name
    $("#previewSMSModalLabel").text("SMS Template Preview - " + template.name);
    
    // Don't force existing modals to close as it breaks modal stacking
    // $('.modal').modal('hide');
    
    // Create sample data for the preview
    var now = new Date();
    var sampleData = {
        // Recipient fields
        FirstName: "John",
        LastName: "Doe",
        Email: "john.doe@example.com",
        Position: "IT Manager",
        Phone: "+15551234567",
        
        // Context fields
        From: "Phishing Team",
        URL: "https://example.com?rid=abc12345",
        TrackingURL: "https://example.com/track?rid=abc12345",
        Tracker: "[TRACKING IMAGE]",
        RId: "abc12345",
        BaseURL: "https://example.com",
        QR: "[QR CODE IMAGE]",
        
        // DateTime fields (dynamic)
        CurrentDateTime: now.toLocaleString('en-US', {month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true}),
        CurrentDate: now.toLocaleString('en-US', {month: 'long', day: 'numeric', year: 'numeric'}),
        CurrentTime: now.toLocaleString('en-US', {hour: 'numeric', minute: '2-digit', hour12: true}),
        CurrentTime24: now.toLocaleString('en-GB', {hour: '2-digit', minute: '2-digit', hour12: false})
    };
    
    // Replace template variables with sample data
    var text = template.text || "";
    for (var key in sampleData) {
        var regex = new RegExp('{{\\.' + key + '}}', 'g');
        text = text.replace(regex, sampleData[key]);
    }
    
    // Display the preview in the div
    var previewDiv = $("#preview_sms_content");
    if (!previewDiv.length) {
        alert("Preview div not found. Make sure the modal HTML is properly included in templates.html");
        return;
    }
    
    previewDiv.text(text);
    
    // Update the sender info in the phone UI
    var senderName = template.from || "Unknown Sender";
    var senderInitial = senderName.charAt(0).toUpperCase();
    
    // If it's a phone number, format it nicely
    if (/^\+?\d+$/.test(senderName.replace(/[\s\-\(\)]/g, ''))) {
        // It's a phone number
        $("#sms-sender-name").text(senderName);
        $("#sms-sender-avatar").text("#");
    } else {
        // It's a name/alphanumeric sender ID
        $("#sms-sender-name").text(senderName);
        $("#sms-sender-avatar").text(senderInitial);
    }
    
    // Update the status bar time (iOS style)
    var hours = now.getHours();
    var minutes = now.getMinutes();
    minutes = minutes < 10 ? '0' + minutes : minutes;
    var statusTimeString = hours + ':' + minutes;
    $("#sms-status-time").text(statusTimeString);
    
    // Update the message time
    var ampm = hours >= 12 ? 'PM' : 'AM';
    var displayHours = hours % 12;
    displayHours = displayHours ? displayHours : 12; // the hour '0' should be '12'
    var timeString = displayHours + ':' + minutes + ' ' + ampm;
    $("#sms-time").text('Read ' + timeString);
    
    // Update the date
    var dateOptions = { weekday: 'long', hour: 'numeric', minute: '2-digit', hour12: true };
    var dateString = now.toLocaleString('en-US', dateOptions);
    $("#sms-date").text('Today ' + timeString);
    
    // Show the modal with a slight delay to ensure everything is ready
    setTimeout(function() {
        $("#previewSMSModal").modal({
            backdrop: 'static',
            keyboard: false,
            show: true
        });
        
        // Ensure the preview modal appears in front by manually setting a high z-index
        // Make sure it's higher than the navbar (which has z-index: 1030 by default)
        $("#previewSMSModal").css('z-index', 1060);
        $('.modal-backdrop').last().css('z-index', 1050);
        
        // Make sure the navbar stays behind modals
        $('.navbar-fixed-top').css('z-index', 1020);
    }, 100);
}

function loadSMSTemplates() {
    $("#smsTemplateTable").hide()
    $("#smsEmptyMessage").hide()
    $("#smsLoading").show()
    api.smsTemplates.get()
        .success(function (ss) {
            smsTemplates = ss
            $("#smsLoading").hide()
            if (smsTemplates.length > 0) {
                $("#smsTemplateTable").show()
                smsTemplateTable = $("#smsTemplateTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                smsTemplateTable.clear()
                smsTemplateRows = []
                $.each(smsTemplates, function (i, template) {
                    smsTemplateRows.push([
                        "<input type='checkbox' class='sms-template-checkbox' data-id='" + template.id + "'>",
                        escapeHtml(template.name),
                        template.text.length + " chars",
                        moment(template.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#smsModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Template' onclick='editSMSTemplate(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#smsModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Template' onclick='copySMSTemplate(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <span data-toggle='modal' data-target='#previewSMSModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Preview Template' onclick='previewSMSTemplate(" + i + ")'>\
                    <i class='fa fa-eye'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Template' onclick='deleteSMSTemplate(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                smsTemplateTable.rows.add(smsTemplateRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
                
                // Set up checkbox event handlers
                $('#selectAllSMSTemplates').off('change').on('change', function() {
                    handleSelectAllSMSTemplates();
                });
                $(document).off('change', 'input.sms-template-checkbox').on('change', 'input.sms-template-checkbox', function() {
                    var templateId = $(this).data('id');
                    handleSMSTemplateCheckboxChange(templateId);
                });
                
                // Clear selections when loading
                clearSMSTemplateSelections();
            } else {
                $("#smsEmptyMessage").show()
            }
        })
        .error(function () {
            $("#smsLoading").hide()
            errorFlash("Error fetching SMS templates")
        })
}

$(document).ready(function () {
    // Store the original navbar z-index
    var originalNavbarZIndex = $('.navbar-fixed-top').css('z-index') || 1030;
    
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        
        // Safely decrement the counter
        var currentCount = parseInt($('body').data('fv_open_modals') || 0);
        var newCount = Math.max(0, currentCount - 1);
        $('body').data('fv_open_modals', newCount);
        
        // If no modals are open, reset everything
        if (newCount === 0) {
            // Reset the navbar to its original z-index
            $('.navbar-fixed-top').css('z-index', originalNavbarZIndex);
            // Clear any leftover backdrops
            $('.modal-backdrop').remove();
        }
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
        var currentCount = parseInt($('body').data('fv_open_modals') || 0);
        $('body').data('fv_open_modals', currentCount + 1);
        
        // Make sure navbar stays behind
        $('.navbar-fixed-top').css('z-index', 1020);
        
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * (currentCount + 1)));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * (currentCount + 1)));
        $('.modal-backdrop').not('.fv-modal-stack').addClass('fv-modal-stack');
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
    $('#smsModal').on('hidden.bs.modal', function (event) {
        dismissSMS()
    });
    
    loadSMSTemplates()
})
