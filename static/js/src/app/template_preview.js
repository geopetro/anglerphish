/*
 * Email template, SMS template and landing page previews, shared by the
 * Templates and Landing Pages pages and the New Campaign modal. The modal markup lives in templates/template_preview.html.
 */

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

// Shows the email preview modal for a template object
function showEmailTemplatePreview(template) {
    // Store current template for re-rendering
    currentEmailPreviewTemplate = template;
    
    // Set the modal title to include the template name
    $("#previewEmailModalLabel").text("Email Template Preview - " + template.name);
    
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
            var icon = (typeof icons !== 'undefined' && icons[attachment.type]) || 'fa-file-o';
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
window.showEmailTemplatePreview = showEmailTemplatePreview;

// Shows the SMS preview modal for a template object
function showSMSTemplatePreview(template) {
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
window.showSMSTemplatePreview = showSMSTemplatePreview;

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

// Shows the landing page preview modal for a page object
function showPagePreview(page) {
    // Store current page for re-rendering
    currentPagePreviewPage = page;
    
    $("#previewPageModalLabel").text("Landing Page Preview - " + page.name);
    
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

        // Keep the preview in front when opened over another modal
        $("#previewPageModal").css('z-index', 1060);
        $('.modal-backdrop').last().css('z-index', 1050);
    }, 100);
}
window.showPagePreview = showPagePreview;
