var emailTemplates = []
var emailTemplateTable = null
var selectedTemplates = {}  // Map of template id -> true for selected templates

// Update the selection count display and button visibility
function updateTemplateSelectionUI() {
    var count = Object.keys(selectedTemplates).length;
    $('#selectedTemplateCount').text(count);
    if (count > 0) {
        $('#deleteSelectedTemplates').show();
    } else {
        $('#deleteSelectedTemplates').hide();
    }
}

// Clear all selections
function clearTemplateSelections() {
    selectedTemplates = {};
    $('input.template-checkbox').prop('checked', false);
    $('#selectAllTemplates').prop('checked', false).prop('indeterminate', false);
    updateTemplateSelectionUI();
}

// Handle individual checkbox change
function handleTemplateCheckboxChange(templateId) {
    var checkbox = $('input.template-checkbox[data-id="' + templateId + '"]');
    if (checkbox.is(':checked')) {
        selectedTemplates[templateId] = true;
    } else {
        delete selectedTemplates[templateId];
    }
    updateTemplateSelectionUI();
    updateSelectAllTemplatesCheckbox();
}

// Update select all checkbox state
function updateSelectAllTemplatesCheckbox() {
    if (!emailTemplateTable) return;
    var allCheckboxes = $(emailTemplateTable.table().body()).find('input.template-checkbox');
    var checkedCount = allCheckboxes.filter(':checked').length;
    var totalCount = allCheckboxes.length;
    
    if (totalCount === 0 || checkedCount === 0) {
        $('#selectAllTemplates').prop('checked', false).prop('indeterminate', false);
    } else if (checkedCount === totalCount) {
        $('#selectAllTemplates').prop('checked', true).prop('indeterminate', false);
    } else {
        $('#selectAllTemplates').prop('checked', false).prop('indeterminate', true);
    }
}

// Handle select all checkbox
function handleSelectAllTemplates() {
    if (!emailTemplateTable) return;
    var isChecked = $('#selectAllTemplates').is(':checked');
    var allCheckboxes = $(emailTemplateTable.table().body()).find('input.template-checkbox');
    
    allCheckboxes.each(function() {
        $(this).prop('checked', isChecked);
        var templateId = $(this).data('id');
        if (isChecked) {
            selectedTemplates[templateId] = true;
        } else {
            delete selectedTemplates[templateId];
        }
    });
    updateTemplateSelectionUI();
}

// Delete selected templates
function deleteSelectedTemplates() {
    var ids = Object.keys(selectedTemplates).map(function(id) { return parseInt(id); });
    if (ids.length === 0) return;
    
    var confirmText = ids.length === 1 
        ? "Delete 1 template?" 
        : "Delete " + ids.length + " templates?";
    
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
                api.templates.bulkDelete(ids)
                    .success(function (msg) { resolve(msg) })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Templates Deleted!', result.value.message, 'success');
            selectedTemplates = {};
            $('button:contains("OK")').on('click', function () { location.reload() })
        }
    })
}
window.deleteSelectedTemplates = deleteSelectedTemplates;

var icons = {
    "application/vnd.ms-excel": "fa-file-excel-o",
    "text/plain": "fa-file-text-o",
    "image/gif": "fa-file-image-o",
    "image/png": "fa-file-image-o",
    "application/pdf": "fa-file-pdf-o",
    "application/x-zip-compressed": "fa-file-archive-o",
    "application/x-gzip": "fa-file-archive-o",
    "application/vnd.openxmlformats-officedocument.presentationml.presentation": "fa-file-powerpoint-o",
    "application/vnd.openxmlformats-officedocument.wordprocessingml.document": "fa-file-word-o",
    "application/octet-stream": "fa-file-o",
    "application/x-msdownload": "fa-file-o"
}


var templateAttachments = []

// Save attempts to POST to /templates/
function saveEmailTemplate(idx) {
    var template = {
        attachments: []
    }
    template.name = $("#name").val()
    template.subject = $("#subject").val()
    template.envelope_sender = $("#envelope-sender").val()
    template.html = CKEDITOR.instances["html_editor"].getData();
    // Fix the URL Scheme added by CKEditor (until we can remove it from the plugin)
    template.html = template.html.replace(/https?:\/\/{{\.URL}}/gi, "{{.URL}}")
    if ($("#use_tracker_checkbox").prop("checked")) {
        if (template.html.indexOf("{{.Tracker}}") == -1 &&
            template.html.indexOf("{{.TrackingUrl}}") == -1) {
            template.html = template.html.replace("</body>", "{{.Tracker}}</body>")
        }
    } else {
        template.html = template.html.replace(/{{\.Tracker}}/, "")
    }
    template.text = $("#text_editor").val()
    template.embed_remote_images = $("#embed_remote_images_checkbox").prop("checked")
    template.track_attachments = $("#track_attachments_checkbox").prop("checked")
    $.each(templateAttachments, function (i, a) {
        template.attachments.push({
            name: a.name,
            content: a.content,
            type: a.type,
        })
    })

    var request
    if (idx != -1) {
        template.id = emailTemplates[idx].id
        request = api.templateId.put(template)
    } else {
        request = api.templates.post(template)
    }
    request
        .success(function () {
            successFlash(idx != -1 ? "Email Template edited successfully!" : "Email Template added successfully!")
            loadEmailTemplates()
            dismiss()
        })
        .error(function (data) {
            modalError(data.responseJSON.message)
        })
}

function dismiss() {
    $("#modal\\.flashes").empty()
    templateAttachments = []
    renderAttachments()
    $("#name").val("")
    $("#subject").val("")
    $("#envelope-sender").val("")
    $("#text_editor").val("")
    $("#html_editor").val("")
    $("#use_tracker_checkbox").prop("checked", true)
    $("#embed_remote_images_checkbox").prop("checked", false)
    $("#track_attachments_checkbox").prop("checked", false)
    toggleImportPanel(false)
    $("#etUrlWarning").hide()
    $('a[href="#et-tab-html"]').tab('show')
    $("#emailModal").modal('hide')
}

var deleteTemplate = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the template. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(emailTemplates[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.templateId.delete(emailTemplates[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if(result.value) {
            Swal.fire(
                'Template Deleted!',
                'This template has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function deleteTemplate(idx) {
    if (confirm("Delete " + emailTemplates[idx].name + "?")) {
        api.templateId.delete(emailTemplates[idx].id)
            .success(function (data) {
                successFlash(data.message)
                loadEmailTemplates()
            })
    }
}

function formatFileSize(bytes) {
    if (bytes < 1024) return bytes + " B"
    if (bytes < 1024 * 1024) return Math.round(bytes / 1024) + " KB"
    return (bytes / (1024 * 1024)).toFixed(1) + " MB"
}

function renderAttachments() {
    var list = $("#etFileList").empty()
    $.each(templateAttachments, function (i, a) {
        var icon = icons[a.type] || "fa-file-o"
        var bytes = Math.floor((a.content || "").length * 3 / 4)
        list.append(
            '<li><i class="fa ' + icon + '" aria-hidden="true"></i>' +
            '<span class="et-file-name" title="' + escapeHtml(a.name) + '">' + escapeHtml(a.name) + '</span>' +
            '<span class="et-file-size">' + formatFileSize(bytes) + '</span>' +
            '<button type="button" class="btn btn-link et-file-remove" data-idx="' + i + '" aria-label="Remove ' + escapeHtml(a.name) + '">' +
            '<i class="fa fa-trash-o" aria-hidden="true"></i></button></li>')
    })
    $("#etAttachCount").text(templateAttachments.length)
}

function attach(files) {
    $.each(files, function (i, file) {
        var reader = new FileReader();
        reader.onload = function () {
            templateAttachments.push({
                name: file.name,
                content: reader.result.split(",")[1],
                type: file.type || "application/octet-stream"
            })
            renderAttachments()
        }
        reader.onerror = function (e) {
            console.log(e)
        }
        reader.readAsDataURL(file)
    })
}

function initHtmlEditor() {
    if (CKEDITOR.instances["html_editor"]) return
    // Sized to fill its pane, so the drag handle would only push it out of the modal
    $("#html_editor").ckeditor({ resize_enabled: false })
    var editor = CKEDITOR.instances["html_editor"]
    setupAutocomplete(editor)
    editor.on("instanceReady", resizeHtmlEditor)
    editor.on("change", updateContentWarnings)
    editor.on("dataReady", updateContentWarnings)
}

function resizeHtmlEditor() {
    var editor = CKEDITOR.instances["html_editor"]
    var pane = $("#et-tab-html")
    if (!editor || !editor.container || !pane.is(":visible")) return
    editor.resize("100%", pane.height())
}

function updateContentWarnings() {
    var editor = CKEDITOR.instances["html_editor"]
    var html = editor ? editor.getData() : ""
    var text = $("#text_editor").val()
    var hasContent = $.trim(html) !== "" || $.trim(text) !== ""
    var hasUrl = html.indexOf("{{.URL}}") != -1 || text.indexOf("{{.URL}}") != -1
    $("#etUrlWarning").toggle(hasContent && !hasUrl)
}

// Fills the modal from a template. mode is "new", "edit" or "copy".
function openTemplateForm(template, mode, saveIdx) {
    $("#modalSubmit").unbind('click').click(function () {
        saveEmailTemplate(saveIdx)
    })
    initHtmlEditor()
    var titles = { "new": "New Email Template", "edit": "Edit Email Template", "copy": "Copy Email Template" }
    $("#templateModalLabel").text(titles[mode])
    $("#name").val(mode == "copy" ? "Copy of " + template.name : (template.name || ""))
    $("#subject").val(template.subject || "")
    $("#envelope-sender").val(template.envelope_sender || "")
    $("#html_editor").val(template.html || "")
    $("#text_editor").val(template.text || "")
    templateAttachments = $.map(template.attachments || [], function (file) {
        return {
            name: file.name,
            content: file.content,
            type: file.type || "application/octet-stream"
        }
    })
    renderAttachments()
    $("#use_tracker_checkbox").prop("checked", mode == "new" || (template.html || "").indexOf("{{.Tracker}}") != -1)
    $("#embed_remote_images_checkbox").prop("checked", template.embed_remote_images === true)
    $("#track_attachments_checkbox").prop("checked", template.track_attachments === true)
    updateContentWarnings()
}

function editEmailTemplate(idx) {
    if (idx == -1) {
        openTemplateForm({ attachments: [] }, "new", -1)
    } else {
        openTemplateForm(emailTemplates[idx], "edit", idx)
    }
}

function copy(idx) {
    openTemplateForm(emailTemplates[idx], "copy", -1)
}

function toggleImportPanel(show) {
    $("#etImportPanel").toggle(show)
    $("#etImportToggle").attr("aria-expanded", show ? "true" : "false").toggleClass("active", show)
    if (show) {
        $("#email_content").focus()
    } else {
        $("#email_content").val("")
    }
}

function importEmail() {
    var raw = $("#email_content").val()
    var convert_links = $("#convert_links_checkbox").prop("checked")
    if (!raw) {
        modalError("No Content Specified!")
        return
    }
    api.import_email({
            content: raw,
            convert_links: convert_links
        })
        .success(function (data) {
            $("#modal\\.flashes").empty()
            $("#text_editor").val(data.text)
            $("#html_editor").val(data.html)
            $("#subject").val(data.subject)
            if (data.html) {
                CKEDITOR.instances["html_editor"].setMode('wysiwyg')
                $('a[href="#et-tab-html"]').tab('show')
            } else {
                $('a[href="#et-tab-text"]').tab('show')
            }
            toggleImportPanel(false)
            updateContentWarnings()
        })
        .error(function (data) {
            modalError(data.responseJSON.message)
        })
}

var previewQrImage = '<span style="display:inline-block;width:120px;height:120px;line-height:120px;border:1px dashed #999;' +
    'text-align:center;font:12px sans-serif;color:#666">QR code</span>'

function previewSampleData() {
    var now = new Date()
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
        QR: previewQrImage,
        RId: "abc12345",
        BaseURL: "https://example.com",
        CurrentDateTime: now.toLocaleString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true }),
        CurrentDate: now.toLocaleString('en-US', { month: 'long', day: 'numeric', year: 'numeric' }),
        CurrentTime: now.toLocaleString('en-US', { hour: 'numeric', minute: '2-digit', hour12: true }),
        CurrentTime24: now.toLocaleString('en-GB', { hour: '2-digit', minute: '2-digit', hour12: false })
    }
}

function fillSampleData(s) {
    $.each(previewSampleData(), function (key, value) {
        s = s.replace(new RegExp("{{\\s*\\." + key + "\\s*}}", "g"), value)
    })
    return s
}

function renderPreviewSubject() {
    var subject = fillSampleData($("#subject").val()).replace(/{{\s*\.QR\s*}}/g, "[QR code]")
    $("#etPreviewSubject").text($.trim(subject) || "(No subject)")
}

function renderInlinePreview() {
    var editor = CKEDITOR.instances["html_editor"]
    var html = editor ? editor.getData() : ""
    if ($.trim(html) === "") {
        html = "<pre style='white-space:pre-wrap;font-family:inherit'>" + escapeHtml($("#text_editor").val()) + "</pre>"
    }
    html = disableLinksOnly(sanitizeHTML(fillSampleData(html)))
    renderPreviewSubject()
    var doc = document.getElementById("etPreviewFrame").contentDocument
    doc.open()
    doc.write(html)
    doc.close()
}

function insertTemplateVariable(name) {
    var tag = "{{." + name + "}}"
    var active = $(".et-tabs li.active a").attr("href")
    if (active == "#et-tab-text") {
        var el = document.getElementById("text_editor")
        var start = el.selectionStart, end = el.selectionEnd
        el.value = el.value.slice(0, start) + tag + el.value.slice(end)
        el.selectionStart = el.selectionEnd = start + tag.length
        el.focus()
    } else {
        CKEDITOR.instances["html_editor"].insertText(tag)
    }
    updateContentWarnings()
}

// Fills a dropdown with the template variables, grouped, skipping names in exclude
function buildVariableMenu(menu, exclude, onPick) {
    if (typeof TEMPLATE_TAGS === "undefined") return
    var lastGroup = null
    $.each(TEMPLATE_TAGS, function (i, t) {
        if (exclude.indexOf(t.name) != -1) return
        if (t.group && t.group != lastGroup) {
            if (lastGroup !== null) menu.append('<li role="separator" class="divider"></li>')
            menu.append('<li class="dropdown-header">' + escapeHtml(t.group) + '</li>')
            lastGroup = t.group
        }
        menu.append('<li><a href="#" data-var="' + t.name + '"><code>{{.' + t.name + '}}</code><small>' +
            escapeHtml(t.description) + '</small></a></li>')
    })
    menu.on("click", "a[data-var]", function (e) {
        e.preventDefault()
        onPick($(this).data("var"))
    })
}

function initEmailModal() {
    buildVariableMenu($("#etVarMenu"), [], insertTemplateVariable)
    $('.et-tabs a[data-toggle="tab"]').on("shown.bs.tab", function (e) {
        var target = $(e.target).attr("href")
        $("#etInsertVar button").prop("disabled", target == "#et-tab-preview")
        if (target == "#et-tab-html") resizeHtmlEditor()
        if (target == "#et-tab-preview") renderInlinePreview()
    })
    $("#etImportToggle").on("click", function () {
        toggleImportPanel(!$("#etImportPanel").is(":visible"))
    })
    $("#etImportCancel").on("click", function () {
        toggleImportPanel(false)
    })
    $("#text_editor").on("input", updateContentWarnings)
    $("#subject").on("input", renderPreviewSubject)
    $("#attachmentUpload").on("change", function () {
        attach(this.files)
        this.value = null
    })
    $("#etDropZone")
        .on("dragover dragenter", function (e) {
            e.preventDefault()
            $(this).addClass("et-drop-over")
        })
        .on("dragleave dragend drop", function () {
            $(this).removeClass("et-drop-over")
        })
        .on("drop", function (e) {
            e.preventDefault()
            attach(e.originalEvent.dataTransfer.files)
        })
    $("#etFileList").on("click", ".et-file-remove", function () {
        templateAttachments.splice($(this).data("idx"), 1)
        renderAttachments()
    })
    $("#emailModal").on("shown.bs.modal", resizeHtmlEditor)
    $(window).on("resize", resizeHtmlEditor)
}

// Preview an email template with sample data
function previewEmailTemplate(idx) {
    // Force any existing modals to close
    $('.modal').modal('hide');
    showEmailTemplatePreview(emailTemplates[idx]);
}

function loadEmailTemplates() {
    $("#emailTemplateTable").hide()
    $("#emailEmptyMessage").hide()
    $("#emailLoading").show()
    api.templates.get()
        .success(function (ts) {
            emailTemplates = ts
            $("#emailLoading").hide()
            if (emailTemplates.length > 0) {
                $("#emailTemplateTable").show()
                emailTemplateTable = $("#emailTemplateTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                emailTemplateTable.clear()
                emailTemplateRows = []
                $.each(emailTemplates, function (i, template) {
                    emailTemplateRows.push([
                        "<input type='checkbox' class='template-checkbox' data-id='" + template.id + "'>",
                        escapeHtml(template.name),
                        moment(template.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#emailModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Template' onclick='editEmailTemplate(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#emailModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Template' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <span data-toggle='modal' data-target='#previewEmailModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Preview Template' onclick='previewEmailTemplate(" + i + ")'>\
                    <i class='fa fa-eye'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Template' onclick='deleteTemplate(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                emailTemplateTable.rows.add(emailTemplateRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
                
                // Set up checkbox event handlers
                $('#selectAllTemplates').off('change').on('change', function() {
                    handleSelectAllTemplates();
                });
                $(document).off('change', 'input.template-checkbox').on('change', 'input.template-checkbox', function() {
                    var templateId = $(this).data('id');
                    handleTemplateCheckboxChange(templateId);
                });
                
                // Clear selections when loading
                clearTemplateSelections();
            } else {
                $("#emailEmptyMessage").show()
            }
        })
        .error(function () {
            $("#emailLoading").hide()
            errorFlash("Error fetching email templates")
        })
}

$(document).ready(function () {
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    // Store the original navbar z-index
    var originalNavbarZIndex = $('.navbar-fixed-top').css('z-index') || 1030;
    
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
    $('#emailModal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    initEmailModal()
    CKEDITOR.on('dialogDefinition', function (ev) {
        // Take the dialog name and its definition from the event data.
        var dialogName = ev.data.name;
        var dialogDefinition = ev.data.definition;

        // Check if the definition is from the dialog window you are interested in (the "Link" dialog window).
        if (dialogName == 'link') {
            dialogDefinition.minWidth = 500
            dialogDefinition.minHeight = 100

            // Remove the linkType field
            var infoTab = dialogDefinition.getContents('info');
            infoTab.get('linkType').hidden = true;
        }
    });
    loadEmailTemplates()
})
