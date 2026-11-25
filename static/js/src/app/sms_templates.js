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

function smsModalError(message) {
    $("#smsModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
        <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(message) + "</div>")
}

// Save attempts to POST or PUT to /sms_templates/
function saveSMSTemplate(idx) {
    var sms_template = {
        name: $("#smsName").val(),
        text: $("#smsText").val(),
        from: $("#smsFrom").val()
    }
    var request
    if (idx != -1) {
        sms_template.id = smsTemplates[idx].id
        request = api.smsTemplateId.put(sms_template)
    } else {
        request = api.smsTemplates.post(sms_template)
    }
    request
        .success(function () {
            successFlash(idx != -1 ? "SMS template edited successfully!" : "SMS template added successfully!")
            loadSMSTemplates()
            dismissSMS()
        })
        .error(function (data) {
            smsModalError(data.responseJSON.message)
        })
}

function dismissSMS() {
    $("#smsModal\\.flashes").empty()
    $("#smsName").val("")
    $("#smsFrom").val("")
    $("#smsText").val("")
    updateSMSPreview()
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

// mode is "new", "edit" or "copy"
function openSMSForm(template, mode, saveIdx) {
    $("#smsModalSubmit").unbind('click').click(function () {
        saveSMSTemplate(saveIdx)
    })
    var titles = { "new": "New SMS Template", "edit": "Edit SMS Template", "copy": "Copy SMS Template" }
    $("#smsTemplateModalLabel").text(titles[mode])
    $("#smsName").val(mode == "copy" ? "Copy of " + template.name : (template.name || ""))
    $("#smsFrom").val(template.from || "")
    $("#smsText").val(template.text || "")
    updateSMSPreview()
}

function editSMSTemplate(idx) {
    if (idx == -1) {
        openSMSForm({}, "new", -1)
    } else {
        openSMSForm(smsTemplates[idx], "edit", idx)
    }
}

function copySMSTemplate(idx) {
    openSMSForm(smsTemplates[idx], "copy", -1)
}

var GSM7_BASIC = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ\u001bÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?" +
    "¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
var GSM7_EXTENDED = "^{}\\[~]|€"

// Estimates SMS length the way carriers count it: GSM-7 (160, or 153 per part)
// unless any character needs UCS-2 (70, or 67 per part)
function smsSegments(text) {
    var units = 0
    var gsm = true
    for (var i = 0; i < text.length; i++) {
        var c = text.charAt(i)
        if (GSM7_BASIC.indexOf(c) != -1) {
            units += 1
        } else if (GSM7_EXTENDED.indexOf(c) != -1) {
            units += 2
        } else {
            gsm = false
            break
        }
    }
    if (!gsm) units = text.length
    var single = gsm ? 160 : 70
    var multi = gsm ? 153 : 67
    var segments = units == 0 ? 0 : (units <= single ? 1 : Math.ceil(units / multi))
    var capacity = segments <= 1 ? single : segments * multi
    return { units: units, segments: segments, encoding: gsm ? "GSM-7" : "Unicode", left: capacity - units }
}

function updateSMSCharCount() {
    var text = $("#smsText").val()
    var s = smsSegments(text)
    $("#smsCharCount").text(text.length)
    $("#smsCount").text(s.segments)
    $("#smsCountLabel").text(s.segments == 1 ? "message" : "messages")
    $("#smsEncoding").text(s.encoding)
        .toggleClass("st-unicode", s.encoding != "GSM-7")
        .attr("title", s.encoding == "GSM-7" ?
            "Standard SMS characters. 160 per message, 153 per part when split." :
            "Contains characters outside the GSM-7 set (for example emoji), so each message holds 70, or 67 per part.")
    $("#smsLeft").text(s.units ? s.left + " left in this message" : "")
}

function linkifySMS(escaped) {
    return escaped.replace(/(https?:\/\/[^\s<]+)/g, '<span class="st-link">$1</span>')
}

// Grows with the text, the CSS min and max heights bound it
function autosizeSMSText() {
    var el = document.getElementById("smsText")
    if (!el || !el.offsetParent) return
    el.style.height = "auto"
    el.style.height = (el.scrollHeight + 2) + "px"
}

function updateSMSPreview() {
    updateSMSCharCount()
    autosizeSMSText()
    var raw = $("#smsText").val()
    var text = raw.replace(/{{\s*\.QR\s*}}/g, "[QR code]")
    text = typeof fillSampleData === "function" ? fillSampleData(text) : text
    $("#stBubble").html(linkifySMS(escapeHtml(text)))
    var hasText = $.trim(raw) !== ""
    $("#stEmpty").toggle(!hasText)
    $(".st-day, #stRead").toggle(hasText)
    $("#stUrlWarning").toggle(hasText && raw.indexOf("{{.URL}}") == -1)

    var from = $.trim($("#smsFrom").val())
    var isNumber = from !== "" && /^\+?\d+$/.test(from.replace(/[\s\-()]/g, ""))
    $("#stSender").text(from || "Default sender")
    $("#stSub").text(from ? "SMS/MMS" : "From the SMS Profile")
    $("#stAvatar").text(from === "" ? "?" : (isNumber ? "#" : from.charAt(0).toUpperCase()))

    var now = new Date()
    var mins = now.getMinutes() < 10 ? "0" + now.getMinutes() : now.getMinutes()
    var clock = now.toLocaleString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true })
    $("#stStatusTime").text(now.getHours() + ":" + mins)
    $("#stDay").text("Today " + clock)
    $("#stRead").text("Read " + clock)
}

function insertSMSVariable(name) {
    var tag = "{{." + name + "}}"
    var el = document.getElementById("smsText")
    var start = el.selectionStart, end = el.selectionEnd
    el.value = el.value.slice(0, start) + tag + el.value.slice(end)
    el.selectionStart = el.selectionEnd = start + tag.length
    el.focus()
    updateSMSPreview()
}

function initSMSModal() {
    if (typeof buildVariableMenu === "function") {
        // HTML-only variables have no meaning in a text message
        buildVariableMenu($("#stVarMenu"), ["Tracker", "TrackingURL", "QR"], insertSMSVariable)
    }
    $("#smsText, #smsFrom").on("input", updateSMSPreview)
    $("#smsModal").on("shown.bs.modal", autosizeSMSText)
    updateSMSPreview()
}

// Preview an SMS template with sample data
function previewSMSTemplate(idx) {
    showSMSTemplatePreview(smsTemplates[idx]);
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
    
    initSMSModal()
    loadSMSTemplates()
})
