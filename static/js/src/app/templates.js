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
    // If the "Add Tracker Image" checkbox is checked, add the tracker
    if ($("#use_tracker_checkbox").prop("checked")) {
        if (template.html.indexOf("{{.Tracker}}") == -1 &&
            template.html.indexOf("{{.TrackingUrl}}") == -1) {
            template.html = template.html.replace("</body>", "{{.Tracker}}</body>")
        }
    } else {
        // Otherwise, remove the tracker completely from the HTML
        template.html = template.html.replace(/{{\.Tracker}}/, "")
    }
    template.text = $("#text_editor").val()
    // Whether to inline remote images as CID attachments at send time
    template.embed_remote_images = $("#embed_remote_images_checkbox").prop("checked")
    template.track_attachments = $("#track_attachments_checkbox").prop("checked")
    // Add the attachments
    $.each($("#attachmentsTable").DataTable().rows().data(), function (i, target) {
        template.attachments.push({
            name: unescapeHtml(target[1]),
            content: target[3],
            type: target[4],
        })
    })

    if (idx != -1) {
        template.id = emailTemplates[idx].id
        api.templateId.put(template)
            .success(function (data) {
                successFlash("Email Template edited successfully!")
                loadEmailTemplates()
                dismiss()
                $("#emailModal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the template
        api.templates.post(template)
            .success(function (data) {
                successFlash("Email Template added successfully!")
                loadEmailTemplates()
                dismiss()
                $("#emailModal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#attachmentsTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#subject").val("")
    $("#text_editor").val("")
    $("#html_editor").val("")
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

function attach(files) {
    attachmentsTable = $("#attachmentsTable").DataTable({
        destroy: true,
        "order": [
            [1, "asc"]
        ],
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }, {
            sClass: "datatable_hidden",
            targets: [3, 4]
        }]
    });
    $.each(files, function (i, file) {
        var reader = new FileReader();
        /* Make this a datatable */
        reader.onload = function (e) {
            var icon = icons[file.type] || "fa-file-o"
            // Add the record to the modal
            attachmentsTable.row.add([
                '<i class="fa ' + icon + '"></i>',
                escapeHtml(file.name),
                '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
                reader.result.split(",")[1],
                file.type || "application/octet-stream"
            ]).draw()
        }
        reader.onerror = function (e) {
            console.log(e)
        }
        reader.readAsDataURL(file)
    })
}

function editEmailTemplate(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        saveEmailTemplate(idx)
    })
    $("#attachmentUpload").unbind('click').click(function () {
        this.value = null
    })
    $("#html_editor").ckeditor()
    setupAutocomplete(CKEDITOR.instances["html_editor"])
    $("#attachmentsTable").show()
    attachmentsTable = $('#attachmentsTable').DataTable({
        destroy: true,
        "order": [
            [1, "asc"]
        ],
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }, {
            sClass: "datatable_hidden",
            targets: [3, 4]
        }]
    });
    var template = {
        attachments: []
    }
    if (idx != -1) {
        $("#templateModalLabel").text("Edit Email Template")
        template = emailTemplates[idx]
        $("#name").val(template.name)
        $("#subject").val(template.subject)
        $("#envelope-sender").val(template.envelope_sender)
        $("#html_editor").val(template.html)
        $("#text_editor").val(template.text)
        attachmentRows = []
        $.each(template.attachments, function (i, file) {
            var icon = icons[file.type] || "fa-file-o"
            // Add the record to the modal
            attachmentRows.push([
                '<i class="fa ' + icon + '"></i>',
                escapeHtml(file.name),
                '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
                file.content,
                file.type || "application/octet-stream"
            ])
        })
        attachmentsTable.rows.add(attachmentRows).draw()
        if (template.html.indexOf("{{.Tracker}}") != -1) {
            $("#use_tracker_checkbox").prop("checked", true)
        } else {
            $("#use_tracker_checkbox").prop("checked", false)
        }
        $("#embed_remote_images_checkbox").prop("checked", template.embed_remote_images === true)
        $("#track_attachments_checkbox").prop("checked", template.track_attachments === true)

    } else {
        $("#templateModalLabel").text("New Email Template")
        $("#embed_remote_images_checkbox").prop("checked", false)
        $("#track_attachments_checkbox").prop("checked", false)
    }
    // Handle Deletion
    $("#attachmentsTable").unbind('click').on("click", "span>i.fa-trash-o", function () {
        attachmentsTable.row($(this).parents('tr'))
            .remove()
            .draw();
    })
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        saveEmailTemplate(-1)
    })
    $("#attachmentUpload").unbind('click').click(function () {
        this.value = null
    })
    $("#html_editor").ckeditor()
    $("#attachmentsTable").show()
    attachmentsTable = $('#attachmentsTable').DataTable({
        destroy: true,
        "order": [
            [1, "asc"]
        ],
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }, {
            sClass: "datatable_hidden",
            targets: [3, 4]
        }]
    });
    var template = {
        attachments: []
    }
    template = emailTemplates[idx]
    $("#name").val("Copy of " + template.name)
    $("#subject").val(template.subject)
    $("#envelope-sender").val(template.envelope_sender)
    $("#html_editor").val(template.html)
    $("#text_editor").val(template.text)
    $.each(template.attachments, function (i, file) {
        var icon = icons[file.type] || "fa-file-o"
        // Add the record to the modal
        attachmentsTable.row.add([
            '<i class="fa ' + icon + '"></i>',
            escapeHtml(file.name),
            '<span class="remove-row"><i class="fa fa-trash-o"></i></span>',
            file.content,
            file.type || "application/octet-stream"
        ]).draw()
    })
    // Handle Deletion
    $("#attachmentsTable").unbind('click').on("click", "span>i.fa-trash-o", function () {
        attachmentsTable.row($(this).parents('tr'))
            .remove()
            .draw();
    })
    if (template.html.indexOf("{{.Tracker}}") != -1) {
        $("#use_tracker_checkbox").prop("checked", true)
    } else {
        $("#use_tracker_checkbox").prop("checked", false)
    }
    $("#embed_remote_images_checkbox").prop("checked", template.embed_remote_images === true)
    $("#track_attachments_checkbox").prop("checked", template.track_attachments === true)
}

function importEmail() {
    raw = $("#email_content").val()
    convert_links = $("#convert_links_checkbox").prop("checked")
    if (!raw) {
        modalError("No Content Specified!")
    } else {
        api.import_email({
                content: raw,
                convert_links: convert_links
            })
            .success(function (data) {
                $("#text_editor").val(data.text)
                $("#html_editor").val(data.html)
                $("#subject").val(data.subject)
                // If the HTML is provided, let's open that view in the editor
                if (data.html) {
                    CKEDITOR.instances["html_editor"].setMode('wysiwyg')
                    $('.nav-tabs a[href="#html"]').click()
                }
                $("#importEmailModal").modal("hide")
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
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
    $("#importEmailModal").on('hidden.bs.modal', function (event) {
        $("#email_content").val("")
    })
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
