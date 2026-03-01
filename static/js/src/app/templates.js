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

// Preview state for email templates
var currentEmailPreviewTemplate = null;
var currentEmailPreviewDevice = 'desktop';
var loadEmailImages = false;

// Set device preview mode for email template
function setEmailPreviewDevice(device) {
    currentEmailPreviewDevice = device;
    var iframe = document.getElementById('preview_email_iframe');
    
    // Update button states
    $('#previewDesktopBtn, #previewTabletBtn, #previewMobileBtn').removeClass('active');
    
    switch(device) {
        case 'mobile':
            iframe.style.width = '375px';
            iframe.style.height = '667px';
            $('#previewMobileBtn').addClass('active');
            break;
        case 'tablet':
            iframe.style.width = '768px';
            iframe.style.height = '500px';
            $('#previewTabletBtn').addClass('active');
            break;
        case 'desktop':
        default:
            iframe.style.width = '100%';
            iframe.style.height = '400px';
            $('#previewDesktopBtn').addClass('active');
            break;
    }
}
window.setEmailPreviewDevice = setEmailPreviewDevice;

// Toggle image loading in email preview
function toggleEmailImages() {
    loadEmailImages = $('#loadImagesToggle').is(':checked');
    if (currentEmailPreviewTemplate !== null) {
        renderEmailPreview(currentEmailPreviewTemplate);
    }
}
window.toggleEmailImages = toggleEmailImages;

// Render the email preview content
function renderEmailPreview(template) {
    var html = template.html || "";
    
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
    for (var key in sampleData) {
        var regex = new RegExp('{{\\.' + key + '}}', 'g');
        html = html.replace(regex, sampleData[key]);
    }
    
    // Apply security sanitization - remove scripts and event handlers
    html = sanitizeHTML(html);
    
    // Handle images based on toggle state
    if (!loadEmailImages) {
        html = disableExternalResources(html);
    } else {
        // Still disable links for security, but keep images
        html = disableLinksOnly(html);
    }
    
    // Get the iframe
    var iframe = document.getElementById('preview_email_iframe');
    if (!iframe) {
        return;
    }
    
    // Write the content to the iframe
    var iframeDoc = iframe.contentDocument || iframe.contentWindow.document;
    iframeDoc.open();
    iframeDoc.write(html);
    iframeDoc.close();
}

// Disable only links (keep images)
function disableLinksOnly(html) {
    var parser = new DOMParser();
    var doc = parser.parseFromString(html, 'text/html');
    
    // Handle links - disable them
    var links = doc.getElementsByTagName('a');
    for(var i = 0; i < links.length; i++) {
        links[i].setAttribute('data-original-href', links[i].href);
        links[i].href = 'javascript:void(0)';
        links[i].target = '';
    }
    
    return doc.documentElement.outerHTML;
}

// Security functions for template preview
function sanitizeHTML(html) {
    // Create a new DOMParser
    var parser = new DOMParser();
    var doc = parser.parseFromString(html, 'text/html');
    
    // Remove all script tags
    var scripts = doc.getElementsByTagName('script');
    while(scripts.length > 0) {
        scripts[0].parentNode.removeChild(scripts[0]);
    }
    
    // Remove on* attributes (onclick, onload, etc.)
    var allElements = doc.getElementsByTagName('*');
    for(var i = 0; i < allElements.length; i++) {
        var attributes = allElements[i].attributes;
        var attributesToRemove = [];
        for(var j = 0; j < attributes.length; j++) {
            var attrName = attributes[j].name;
            if(attrName.indexOf('on') === 0) {
                attributesToRemove.push(attrName);
            }
        }
        // Remove the identified attributes
        for(var j = 0; j < attributesToRemove.length; j++) {
            allElements[i].removeAttribute(attributesToRemove[j]);
        }
    }
    
    return doc.documentElement.outerHTML;
}

function disableExternalResources(html) {
    // Create a new DOMParser
    var parser = new DOMParser();
    var doc = parser.parseFromString(html, 'text/html');
    
    // Handle images - replace src with data-src
    var images = doc.getElementsByTagName('img');
    for(var i = 0; i < images.length; i++) {
        if(images[i].src) {
            images[i].setAttribute('data-original-src', images[i].src);
            images[i].src = 'data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22100%22%20height%3D%22100%22%3E%3Crect%20fill%3D%22%23eee%22%20width%3D%22100%22%20height%3D%22100%22%2F%3E%3C%2Fsvg%3E';
        }
    }
    
    // Handle links - disable them
    var links = doc.getElementsByTagName('a');
    for(var i = 0; i < links.length; i++) {
        links[i].setAttribute('data-original-href', links[i].href);
        links[i].href = 'javascript:void(0)';
        links[i].target = '';
    }
    
    return doc.documentElement.outerHTML;
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

    } else {
        $("#templateModalLabel").text("New Email Template")
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
    var template = emailTemplates[idx];
    
    // Store current template for re-rendering
    currentEmailPreviewTemplate = template;
    
    // Set the modal title to include the template name
    $("#previewEmailModalLabel").text("Email Template Preview - " + template.name);
    
    // Force any existing modals to close
    $('.modal').modal('hide');
    
    // Reset preview state - images loaded by default
    loadEmailImages = true;
    $('#loadImagesToggle').prop('checked', true);
    currentEmailPreviewDevice = 'desktop';
    
    // Display subject line with variable replacement
    var now = new Date();
    var sampleData = {
        FirstName: "John",
        LastName: "Doe",
        Email: "john.doe@example.com",
        Position: "IT Manager",
        Phone: "+15551234567",
        From: "Phishing Team",
        URL: "https://example.com?rid=abc12345",
        TrackingURL: "https://example.com/track?rid=abc12345",
        RId: "abc12345",
        BaseURL: "https://example.com",
        CurrentDateTime: now.toLocaleString('en-US', {month: 'short', day: 'numeric', year: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true}),
        CurrentDate: now.toLocaleString('en-US', {month: 'long', day: 'numeric', year: 'numeric'}),
        CurrentTime: now.toLocaleString('en-US', {hour: 'numeric', minute: '2-digit', hour12: true}),
        CurrentTime24: now.toLocaleString('en-GB', {hour: '2-digit', minute: '2-digit', hour12: false})
    };
    
    // Process and display subject line
    var subject = template.subject || "(No Subject)";
    for (var key in sampleData) {
        var regex = new RegExp('{{\\.' + key + '}}', 'g');
        subject = subject.replace(regex, sampleData[key]);
    }
    $('#preview_email_subject').text(subject);
    
    // Display attachments if any
    var attachmentsContainer = $('#preview_email_attachments_container');
    var attachmentsList = $('#preview_email_attachments');
    attachmentsList.empty();
    
    if (template.attachments && template.attachments.length > 0) {
        attachmentsContainer.show();
        $.each(template.attachments, function(i, attachment) {
            var icon = icons[attachment.type] || 'fa-file-o';
            var attachmentHtml = '<span style="display: inline-block; margin-right: 15px; margin-bottom: 5px; padding: 5px 10px; background: #fff; border: 1px solid #ddd; border-radius: 3px;">' +
                '<i class="fa ' + icon + '" style="margin-right: 5px; color: #666;"></i>' +
                escapeHtml(attachment.name) +
                '</span>';
            attachmentsList.append(attachmentHtml);
        });
    } else {
        attachmentsContainer.hide();
    }
    
    // Render the email preview
    renderEmailPreview(template);
    
    // Reset device to desktop
    setEmailPreviewDevice('desktop');
    
    // Show the modal with a slight delay to ensure everything is ready
    setTimeout(function() {
        $("#previewEmailModal").modal({
            backdrop: 'static',
            keyboard: false,
            show: true
        });
        
        // Ensure the preview modal appears in front by manually setting a high z-index
        $("#previewEmailModal").css('z-index', 1060);
        $('.modal-backdrop').last().css('z-index', 1050);
        
        // Make sure the navbar stays behind modals
        $('.navbar-fixed-top').css('z-index', 1020);
    }, 100);
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
