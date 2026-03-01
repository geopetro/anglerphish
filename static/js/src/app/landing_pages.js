/*
    landing_pages.js
    Handles the creation, editing, and deletion of landing pages
    Author: Jordan Wright <github.com/jordan-wright>
*/
var pages = [];
var smsProfiles = [];
var pagesTable = null;
var selectedPages = {};  // Map of page id -> true for selected pages

// Update the selection count display and button visibility
function updatePageSelectionUI() {
    var count = Object.keys(selectedPages).length;
    $('#selectedPageCount').text(count);
    if (count > 0) {
        $('#deleteSelectedPages').show();
    } else {
        $('#deleteSelectedPages').hide();
    }
}

// Clear all selections
function clearPageSelections() {
    selectedPages = {};
    $('input.page-checkbox').prop('checked', false);
    $('#selectAllPages').prop('checked', false).prop('indeterminate', false);
    updatePageSelectionUI();
}

// Handle individual checkbox change
function handlePageCheckboxChange(pageId) {
    var checkbox = $('input.page-checkbox[data-id="' + pageId + '"]');
    if (checkbox.is(':checked')) {
        selectedPages[pageId] = true;
    } else {
        delete selectedPages[pageId];
    }
    updatePageSelectionUI();
    updateSelectAllPagesCheckbox();
}

// Update select all checkbox state
function updateSelectAllPagesCheckbox() {
    if (!pagesTable) return;
    var allCheckboxes = $(pagesTable.table().body()).find('input.page-checkbox');
    var checkedCount = allCheckboxes.filter(':checked').length;
    var totalCount = allCheckboxes.length;
    
    if (totalCount === 0 || checkedCount === 0) {
        $('#selectAllPages').prop('checked', false).prop('indeterminate', false);
    } else if (checkedCount === totalCount) {
        $('#selectAllPages').prop('checked', true).prop('indeterminate', false);
    } else {
        $('#selectAllPages').prop('checked', false).prop('indeterminate', true);
    }
}

// Handle select all checkbox
function handleSelectAllPages() {
    if (!pagesTable) return;
    var isChecked = $('#selectAllPages').is(':checked');
    var allCheckboxes = $(pagesTable.table().body()).find('input.page-checkbox');
    
    allCheckboxes.each(function() {
        $(this).prop('checked', isChecked);
        var pageId = $(this).data('id');
        if (isChecked) {
            selectedPages[pageId] = true;
        } else {
            delete selectedPages[pageId];
        }
    });
    updatePageSelectionUI();
}

// Delete selected pages
function deleteSelectedPages() {
    var ids = Object.keys(selectedPages).map(function(id) { return parseInt(id); });
    if (ids.length === 0) return;
    
    var confirmText = ids.length === 1 
        ? "Delete 1 landing page?" 
        : "Delete " + ids.length + " landing pages?";
    
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
                api.pages.bulkDelete(ids)
                    .success(function (msg) { resolve(msg) })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Landing Pages Deleted!', result.value.message, 'success');
            selectedPages = {};
            $('button:contains("OK")').on('click', function () { location.reload() })
        }
    })
}
window.deleteSelectedPages = deleteSelectedPages;

// Preview state for landing pages
var currentPagePreviewPage = null;
var currentPagePreviewDevice = 'desktop';
var loadPageImages = false;

/**
 * Set device preview mode for landing page
 * @param {string} device - The device type ('desktop', 'tablet', 'mobile')
 */
function setPagePreviewDevice(device) {
    currentPagePreviewDevice = device;
    var iframe = document.getElementById('preview_page_iframe');
    
    // Update button states
    $('#previewPageDesktopBtn, #previewPageTabletBtn, #previewPageMobileBtn').removeClass('active');
    
    switch(device) {
        case 'mobile':
            iframe.style.width = '375px';
            iframe.style.height = '667px';
            $('#previewPageMobileBtn').addClass('active');
            break;
        case 'tablet':
            iframe.style.width = '768px';
            iframe.style.height = '500px';
            $('#previewPageTabletBtn').addClass('active');
            break;
        case 'desktop':
        default:
            iframe.style.width = '100%';
            iframe.style.height = '400px';
            $('#previewPageDesktopBtn').addClass('active');
            break;
    }
}
window.setPagePreviewDevice = setPagePreviewDevice;

/**
 * Toggle image loading in landing page preview
 */
function togglePageImages() {
    loadPageImages = $('#loadPageImagesToggle').is(':checked');
    if (currentPagePreviewPage !== null) {
        renderPagePreview(currentPagePreviewPage);
    }
}
window.togglePageImages = togglePageImages;

/**
 * Render the landing page preview content
 * @param {object} page - The page object to render
 */
function renderPagePreview(page) {
    var html = page.html || "";
    
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
    if (!loadPageImages) {
        html = disableExternalResources(html);
    } else {
        // Still disable links for security, but keep images
        html = disableLinksOnly(html);
    }
    
    // Get the iframe element
    var iframe = document.getElementById("preview_page_iframe");
    if (!iframe) {
        return;
    }
    
    // Write the HTML to the iframe
    var doc = iframe.contentDocument || iframe.contentWindow.document;
    doc.open();
    doc.write(html);
    doc.close();
}

/**
 * Disable only links (keep images)
 * @param {string} html - The HTML content to process
 * @returns {string} - The HTML content with only links disabled
 */
function disableLinksOnly(html) {
    var doc = new DOMParser().parseFromString(html, "text/html");
    
    // Disable all links
    var links = doc.getElementsByTagName("a");
    for (var i = 0; i < links.length; i++) {
        links[i].setAttribute("data-original-href", links[i].href);
        links[i].href = "javascript:void(0)";
        links[i].target = "";
    }
    
    return doc.documentElement.outerHTML;
}

/**
 * Sanitizes HTML content by removing scripts and event handlers
 * @param {string} html - The HTML content to sanitize
 * @returns {string} - The sanitized HTML content
 */
function sanitizeHTML(html) {
    // Parse the HTML into a DOM
    var doc = new DOMParser().parseFromString(html, "text/html");
    
    // Remove all script tags
    var scripts = doc.getElementsByTagName("script");
    while (scripts.length > 0) {
        scripts[0].parentNode.removeChild(scripts[0]);
    }
    
    // Remove all event handlers (attributes starting with "on")
    var allElements = doc.getElementsByTagName("*");
    for (var i = 0; i < allElements.length; i++) {
        var attributes = allElements[i].attributes;
        var attributesToRemove = [];
        
        // First, identify all event handler attributes
        for (var j = 0; j < attributes.length; j++) {
            var attrName = attributes[j].name;
            if (attrName.indexOf("on") === 0) {
                attributesToRemove.push(attrName);
            }
        }
        
        // Then remove them
        for (var j = 0; j < attributesToRemove.length; j++) {
            allElements[i].removeAttribute(attributesToRemove[j]);
        }
    }
    
    return doc.documentElement.outerHTML;
}

/**
 * Disables external resources in HTML content by replacing them with placeholders
 * @param {string} html - The HTML content to process
 * @returns {string} - The HTML content with external resources disabled
 */
function disableExternalResources(html) {
    // Parse the HTML into a DOM
    var doc = new DOMParser().parseFromString(html, "text/html");
    
    // Replace all image sources with a placeholder
    var images = doc.getElementsByTagName("img");
    for (var i = 0; i < images.length; i++) {
        if (images[i].src) {
            // Store the original src as a data attribute
            images[i].setAttribute("data-original-src", images[i].src);
            // Replace with a placeholder SVG
            images[i].src = "data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%20width%3D%22100%22%20height%3D%22100%22%3E%3Crect%20fill%3D%22%23eee%22%20width%3D%22100%22%20height%3D%22100%22%2F%3E%3C%2Fsvg%3E";
        }
    }
    
    // Disable all links
    var links = doc.getElementsByTagName("a");
    for (var i = 0; i < links.length; i++) {
        // Store the original href as a data attribute
        links[i].setAttribute("data-original-href", links[i].href);
        // Replace with a javascript void
        links[i].href = "javascript:void(0)";
        // Remove any target attribute
        links[i].target = "";
    }
    
    return doc.documentElement.outerHTML;
}


/**
 * Previews a landing page in a modal
 * @param {number} idx - The index of the page to preview
 */
function previewPage(idx) {
    var page = pages[idx];
    
    // Store current page for re-rendering
    currentPagePreviewPage = page;
    
    $("#previewPageModalLabel").text("Landing Page Preview - " + page.name);
    $(".modal").modal("hide");
    
    // Reset preview state - images loaded by default
    loadPageImages = true;
    $('#loadPageImagesToggle').prop('checked', true);
    currentPagePreviewDevice = 'desktop';
    
    // Render the page preview
    renderPagePreview(page);
    
    // Reset device to desktop
    setPagePreviewDevice('desktop');
    
    // Show the modal after a short delay to ensure content is loaded
    setTimeout(function() {
        $("#previewPageModal").modal({
            backdrop: "static",
            keyboard: false,
            show: true
        });
    }, 100);
}

/**
 * Saves a landing page
 * @param {number} idx - The index of the page to save, or -1 for a new page
 */
function save(idx) {
    var page = {};
    page.name = $("#name").val();
    editor = CKEDITOR.instances.html_editor;
    page.html = editor.getData();
    page.capture_credentials = $("#capture_credentials_checkbox").prop("checked");
    page.capture_passwords = $("#capture_passwords_checkbox").prop("checked");
    page.redirect_url = $("#redirect_url_input").val();
    
    // MFA settings
    page.enable_mfa = $("#enable_mfa_checkbox").prop("checked");
    page.mfa_sms_profile_id = parseInt($("#mfa_sms_profile").val()) || 0;
    page.mfa_from = $("#mfa_from").val();
    page.mfa_message = $("#mfa_message").val();
    page.mfa_code_length = parseInt($("#mfa_code_length").val()) || 6;
    page.mfa_code_type = $("#mfa_code_type").val() || "numeric";
    page.mfa_inject_page = $("#mfa_inject_page_checkbox").prop("checked");
    page.mfa_page_html = window.currentMFAPageHTML || "";

    if (idx != -1) {
        page.id = pages[idx].id;
        api.pageId.put(page)
            .success(function(data) {
                successFlash("Page edited successfully!");
                load();
                dismiss();
            });
    } else {
        // Submit the page
        api.pages.post(page)
            .success(function(data) {
                successFlash("Page added successfully!");
                load();
                dismiss();
            })
            .error(function(data) {
                modalError(data.responseJSON.message);
            });
    }
}

/**
 * Dismisses the modal and resets form fields
 */
function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#html_editor").val("");
    $("#url").val("");
    $("#redirect_url_input").val("");
    $("#modal").find("input[type='checkbox']").prop("checked", false);
    $("#capture_passwords").hide();
    $("#redirect_url").hide();
    // Reset MFA fields
    $("#mfa_settings").hide();
    $("#mfa_sms_profile").val("");
    $("#mfa_from").val("");
    $("#mfa_message").val("");
    $("#mfa_code_length").val("6");
    $("#mfa_code_type").val("numeric");
    $("#mfa_inject_page_checkbox").prop("checked", true);
    window.currentMFAPageHTML = "";
    $("#modal").modal("hide");
}

/**
 * Deletes a landing page
 * @param {number} idx - The index of the page to delete
 */
var deletePage = function(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the landing page. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(pages[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function() {
            return new Promise(function(resolve, reject) {
                api.pageId.delete(pages[idx].id)
                    .success(function(msg) {
                        resolve();
                    })
                    .error(function(data) {
                        reject(data.responseJSON.message);
                    });
            });
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire(
                "Landing Page Deleted!",
                "This landing page has been deleted!",
                "success"
            );
        }
        $('button:contains("OK")').on("click", function() {
            location.reload();
        });
    });
};

/**
 * Imports a site from a URL
 */
function importSite() {
    url = $("#url").val();
    if (!url) {
        modalError("No URL Specified!");
    } else {
        api.clone_site({
            url: url,
            include_resources: false
        })
        .success(function(data) {
            $("#html_editor").val(data.html);
            CKEDITOR.instances.html_editor.setMode("wysiwyg");
            $("#importSiteModal").modal("hide");
        })
        .error(function(data) {
            modalError(data.responseJSON.message);
        });
    }
}

// Global variable to store custom MFA page HTML
window.currentMFAPageHTML = "";

/**
 * Opens the MFA page editor modal
 */
function openMFAPageEditor() {
    // If we have a custom HTML, use it; otherwise fetch the default
    if (window.currentMFAPageHTML && window.currentMFAPageHTML.trim() !== "") {
        $("#mfa_page_html_editor").val(window.currentMFAPageHTML);
        $("#mfaPageEditorModal").modal("show");
    } else {
        // Fetch the default template from API
        api.mfaDefaultTemplate.get()
            .success(function(response) {
                var template = response.data || "";
                $("#mfa_page_html_editor").val(template);
                $("#mfaPageEditorModal").modal("show");
            })
            .error(function() {
                // Fallback: show empty editor
                $("#mfa_page_html_editor").val("");
                $("#mfaPageEditorModal").modal("show");
            });
    }
}

/**
 * Saves the custom MFA page HTML from the editor
 */
function saveMFAPageHTML() {
    window.currentMFAPageHTML = $("#mfa_page_html_editor").val();
    $("#mfaPageEditorModal").modal("hide");
}

/**
 * Previews the MFA page HTML in an iframe
 */
function previewMFAPage() {
    var html = $("#mfa_page_html_editor").val();
    
    if (!html || html.trim() === "") {
        html = "<html><body><p style='color:#999; text-align:center; padding:50px;'>No HTML content to preview. Enter HTML in the editor tab.</p></body></html>";
    } else {
        // Replace template variables with sample data
        html = html.replace(/\{\{\s*\.RId\s*\}\}/g, "abc123456");
        html = html.replace(/\{\{\s*if\s+\.Error\s*\}\}/g, "");
        html = html.replace(/\{\{\s*\.Error\s*\}\}/g, "");
        html = html.replace(/\{\{\s*end\s*\}\}/g, "");
        
        // Sanitize for preview
        html = sanitizeHTML(html);
    }
    
    // Get the iframe element
    var iframe = document.getElementById("mfa_preview_iframe");
    if (iframe) {
        var doc = iframe.contentDocument || iframe.contentWindow.document;
        doc.open();
        doc.write(html);
        doc.close();
    }
}

/**
 * Resets the MFA page HTML to the default template
 */
function resetMFAPageToDefault() {
    api.mfaDefaultTemplate.get()
        .success(function(response) {
            var template = response.data || "";
            $("#mfa_page_html_editor").val(template);
            window.currentMFAPageHTML = ""; // Empty means use default
        })
        .error(function() {
            // Clear to use default
            $("#mfa_page_html_editor").val("");
            window.currentMFAPageHTML = "";
        });
}

/**
 * Loads SMS profiles for the MFA dropdown
 */
function loadSMSProfiles() {
    api.SMS.get()
        .success(function(profiles) {
            smsProfiles = profiles;
            var select = $("#mfa_sms_profile");
            select.find("option:not(:first)").remove();
            $.each(profiles, function(i, profile) {
                select.append($("<option></option>")
                    .attr("value", profile.id)
                    .text(profile.name));
            });
        })
        .error(function() {
            // Silently fail - SMS profiles are optional
        });
}

/**
 * Sets up the page editing modal
 * @param {number} idx - The index of the page to edit, or -1 for a new page
 */
function edit(idx) {
    $("#modalSubmit").unbind("click").click(function() {
        save(idx);
    });
    $("#html_editor").ckeditor();
    setupAutocomplete(CKEDITOR.instances.html_editor);
    loadSMSProfiles();
    
    var page = {};
    if (idx != -1) {
        $("#modalLabel").text("Edit Landing Page");
        page = pages[idx];
        $("#name").val(page.name);
        $("#html_editor").val(page.html);
        $("#capture_credentials_checkbox").prop("checked", page.capture_credentials);
        $("#capture_passwords_checkbox").prop("checked", page.capture_passwords);
        $("#redirect_url_input").val(page.redirect_url);
        
        if (page.capture_credentials) {
            $("#capture_passwords").show();
            $("#redirect_url").show();
        }
        
        // Load MFA settings
        $("#enable_mfa_checkbox").prop("checked", page.enable_mfa);
        if (page.enable_mfa) {
            $("#mfa_settings").show();
        }
        setTimeout(function() {
            $("#mfa_sms_profile").val(page.mfa_sms_profile_id || "");
        }, 300);
        $("#mfa_from").val(page.mfa_from || "");
        $("#mfa_message").val(page.mfa_message || "");
        $("#mfa_code_length").val(page.mfa_code_length || 6);
        $("#mfa_code_type").val(page.mfa_code_type || "numeric");
        $("#mfa_inject_page_checkbox").prop("checked", page.mfa_inject_page !== false);
        window.currentMFAPageHTML = page.mfa_page_html || "";
    } else {
        $("#modalLabel").text("New Landing Page");
        $("#mfa_inject_page_checkbox").prop("checked", true);
        window.currentMFAPageHTML = "";
    }
}

/**
 * Sets up the page copying modal
 * @param {number} idx - The index of the page to copy
 */
function copy(idx) {
    $("#modalSubmit").unbind("click").click(function() {
        save(-1);
    });
    $("#html_editor").ckeditor();
    setupAutocomplete(CKEDITOR.instances.html_editor);
    loadSMSProfiles();
    
    var page = pages[idx];
    $("#name").val("Copy of " + page.name);
    $("#html_editor").val(page.html);
    $("#capture_credentials_checkbox").prop("checked", page.capture_credentials);
    $("#capture_passwords_checkbox").prop("checked", page.capture_passwords);
    $("#redirect_url_input").val(page.redirect_url);
    
    if (page.capture_credentials) {
        $("#capture_passwords").show();
        $("#redirect_url").show();
    }
    
    // Copy MFA settings
    $("#enable_mfa_checkbox").prop("checked", page.enable_mfa);
    if (page.enable_mfa) {
        $("#mfa_settings").show();
    }
    setTimeout(function() {
        $("#mfa_sms_profile").val(page.mfa_sms_profile_id || "");
    }, 300);
    $("#mfa_from").val(page.mfa_from || "");
    $("#mfa_message").val(page.mfa_message || "");
    $("#mfa_code_length").val(page.mfa_code_length || 6);
    $("#mfa_code_type").val(page.mfa_code_type || "numeric");
    $("#mfa_inject_page_checkbox").prop("checked", page.mfa_inject_page !== false);
    window.currentMFAPageHTML = page.mfa_page_html || "";
}

/**
 * Loads the landing pages from the API
 */
function load() {
    $("#pagesTable").hide();
    $("#emptyMessage").hide();
    $("#loading").show();
    
    api.pages.get()
        .success(function(ps) {
            pages = ps;
            $("#loading").hide();
            
            if (pages.length > 0) {
                $("#pagesTable").show();
                pagesTable = $("#pagesTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                pagesTable.clear();
                pageRows = [];
                
                $.each(pages, function(i, page) {
                    pageRows.push([
                        "<input type='checkbox' class='page-checkbox' data-id='" + page.id + "'>",
                        escapeHtml(page.name),
                        moment(page.modified_date).format("MMMM Do YYYY, h:mm:ss a"),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Page' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Page' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <span data-toggle='modal' data-target='#previewPageModal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Preview Page' onclick='previewPage(" + i + ")'>\
                    <i class='fa fa-eye'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Page' onclick='deletePage(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ]);
                });
                
                pagesTable.rows.add(pageRows).draw();
                $('[data-toggle="tooltip"]').tooltip();
                
                // Set up checkbox event handlers
                $('#selectAllPages').off('change').on('change', function() {
                    handleSelectAllPages();
                });
                $(document).off('change', 'input.page-checkbox').on('change', 'input.page-checkbox', function() {
                    var pageId = $(this).data('id');
                    handlePageCheckboxChange(pageId);
                });
                
                // Clear selections when loading
                clearPageSelections();
            } else {
                $("#emptyMessage").show();
            }
        })
        .error(function() {
            $("#loading").hide();
            errorFlash("Error fetching pages");
        });
}

$(document).ready(function() {
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $(".modal").on("hidden.bs.modal", function(event) {
        $(this).removeClass("fv-modal-stack");
        $("body").data("fv_open_modals", $("body").data("fv_open_modals") - 1);
    });
    
    $(".modal").on("shown.bs.modal", function(event) {
        // Keep track of the number of open modals
        if (typeof($("body").data("fv_open_modals")) == "undefined") {
            $("body").data("fv_open_modals", 0);
        }
        
        // If the z-index of this modal has been set, ignore.
        if ($(this).hasClass("fv-modal-stack")) {
            return;
        }
        
        $(this).addClass("fv-modal-stack");
        
        // Increment the number of open modals
        $("body").data("fv_open_modals", $("body").data("fv_open_modals") + 1);
        
        // Setup the appropriate z-index
        $(this).css("z-index", 1040 + (10 * $("body").data("fv_open_modals")));
        $(".modal-backdrop").not(".fv-modal-stack").css("z-index", 1039 + (10 * $("body").data("fv_open_modals")));
        $(".modal-backdrop").not("fv-modal-stack").addClass("fv-modal-stack");
    });
    
    // Fix for CKEditor in modals
    $.fn.modal.Constructor.prototype.enforceFocus = function() {
        $(document)
            .off("focusin.bs.modal") // Guard against infinite focus loop
            .on("focusin.bs.modal", $.proxy(function(e) {
                if (
                    this.$element[0] !== e.target && 
                    !this.$element.has(e.target).length &&
                    // CKEditor compatibility fix
                    !$(e.target).closest(".cke_dialog, .cke").length
                ) {
                    this.$element.trigger("focus");
                }
            }, this));
    };
    
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on("hidden.bs.modal", ".modal", function() {
        $(".modal:visible").length && $(document.body).addClass("modal-open");
    });
    
    $("#modal").on("hidden.bs.modal", function(event) {
        dismiss();
    });
    
    $("#capture_credentials_checkbox").change(function() {
        $("#capture_passwords").toggle();
        $("#redirect_url").toggle();
    });
    
    // MFA checkbox toggle
    $("#enable_mfa_checkbox").change(function() {
        if ($(this).prop("checked")) {
            $("#mfa_settings").show();
        } else {
            $("#mfa_settings").hide();
        }
    });
    
    // CKEditor link dialog customization
    CKEDITOR.on("dialogDefinition", function(ev) {
        // Take the dialog name and its definition from the event data.
        var dialogName = ev.data.name;
        var dialogDefinition = ev.data.definition;

        // Check if the definition is from the dialog window you are interested in (the "Link" dialog window).
        if (dialogName == "link") {
            dialogDefinition.minWidth = 500;
            dialogDefinition.minHeight = 100;

            // Remove the linkType field
            var infoTab = dialogDefinition.getContents("info");
            infoTab.get("linkType").hidden = true;
        }
    });

    // Load the landing pages
    load();
});
