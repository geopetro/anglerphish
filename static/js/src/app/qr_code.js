// Formats a date for display
function formatDate(dateString) {
    const date = new Date(dateString);
    return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
}

// Creates a download button for a QR code
function createDownloadButton(qrCode) {
    return $('<button>')
        .addClass('btn btn-sm btn-primary')
        .html('<i class="fa fa-download"></i>')
        .attr('data-toggle', 'tooltip')
        .attr('title', 'Download')
        .click(function() {
            downloadQRCode(qrCode.id);
        });
}

// Creates a delete button for a QR code
function createDeleteButton(qrCode, callback) {
    return $('<button>')
        .addClass('btn btn-sm btn-danger')
        .html('<i class="fa fa-trash"></i>')
        .attr('data-toggle', 'tooltip')
        .attr('title', 'Delete')
        .click(function() {
            if (confirm('Are you sure you want to delete this QR code?')) {
                deleteQRCode(qrCode.id, callback);
            }
        });
}

// Downloads a QR code from the server
function downloadQRCode(id) {
    // Get the QR code data from the server
    api.qrCodeId.get(id)
        .success(function(data) {
            if (data.success) {
                // Convert the base64 string to a binary blob
                var byteCharacters = atob(data.qr_code_base64);
                var byteNumbers = new Array(byteCharacters.length);
                for (var i = 0; i < byteCharacters.length; i++) {
                    byteNumbers[i] = byteCharacters.charCodeAt(i);
                }
                var byteArray = new Uint8Array(byteNumbers);
                var blob = new Blob([byteArray], {type: 'image/png'});
                
                // Create a download link
                var link = document.createElement('a');
                link.href = window.URL.createObjectURL(blob);
                link.download = data.filename;
                
                // Append to the document, click, and remove
                document.body.appendChild(link);
                link.click();
                document.body.removeChild(link);
                
                // Clean up the object URL
                setTimeout(function() {
                    window.URL.revokeObjectURL(link.href);
                }, 100);
                
                // Show success message
                successFlash("QR code downloaded successfully!");
            } else {
                errorFlash(data.message || "Error downloading QR code");
            }
        })
        .error(function(data) {
            errorFlash(data.responseJSON.message || "Error downloading QR code");
        });
}

// Deletes a QR code from the server
function deleteQRCode(id, callback) {
    api.qr_code.delete(id)
        .success(function(data) {
            if (data.success) {
                successFlash("QR code deleted successfully!");
                if (callback) callback();
            } else {
                errorFlash(data.message || "Error deleting QR code");
            }
        })
        .error(function(data) {
            errorFlash(data.responseJSON.message || "Error deleting QR code");
        });
}

// Loads the QR codes from the server
function loadQRCodes() {
    $('#loading_qr_codes').show();
    $('#no_qr_codes').hide();
    $('#qrCodeTable tbody').empty();
    
    // Get the QR codes from the server
    api.qr_code.get()
        .success(function(data) {
            $('#loading_qr_codes').hide();
            
            if (data.qr_codes && data.qr_codes.length > 0) {
                // Add each QR code to the table
                $.each(data.qr_codes, function(i, qrCode) {
                    var row = $('<tr>');
                    row.append($('<td>').text(qrCode.url));
                    row.append($('<td>').text(qrCode.size));
                    row.append($('<td>').text(formatDate(qrCode.created_at)));
                    
                    // Add action buttons
                    var actions = $('<td>').addClass('text-center');
                    actions.append(createDownloadButton(qrCode));
                    actions.append(' ');
                    actions.append(createDeleteButton(qrCode, loadQRCodes));
                    row.append(actions);
                    
                    $('#qrCodeTable tbody').append(row);
                });
                
                // Initialize tooltips for the newly added buttons
                $('[data-toggle="tooltip"]').tooltip();
            } else {
                $('#no_qr_codes').show();
            }
        })
        .error(function(data) {
            $('#loading_qr_codes').hide();
            errorFlash(data.responseJSON.message || "Error loading QR codes");
        });
}

// Generate and optionally store a QR code
function generate_code() {
    var qr_code = {};
    qr_code.url = $("#url").val();
    qr_code.size = $("#size").val() || "256"; // Default to 256 if no size is provided
    
    // Check if we should store the QR code in the database
    var storeInDb = $("#storeInDb").is(':checked');

    // Clear the form fields
    function resetForm() {
        $("#url").val("")
        $("#size").val("")
        $("#url").focus()
    }

    // Validate input
    if (!qr_code.url) {
        errorFlash("Please enter a URL");
        return;
    }

    // Send the QR code data to the server using the API
    api.qr_code.post({
        url: qr_code.url,
        size: qr_code.size,
        storeInDb: storeInDb
    })
    .success(function(data) {
        if (data.success) {
            // Convert the base64 string to a binary blob
            var byteCharacters = atob(data.qr_code_base64);
            var byteNumbers = new Array(byteCharacters.length);
            for (var i = 0; i < byteCharacters.length; i++) {
                byteNumbers[i] = byteCharacters.charCodeAt(i);
            }
            var byteArray = new Uint8Array(byteNumbers);
            var blob = new Blob([byteArray], {type: 'image/png'});
            
            // Create a download link
            var link = document.createElement('a');
            link.href = window.URL.createObjectURL(blob);
            link.download = data.filename;
            
            // Append to the document, click, and remove
            document.body.appendChild(link);
            link.click();
            document.body.removeChild(link);
            
            // Clean up the object URL
            setTimeout(function() {
                window.URL.revokeObjectURL(link.href);
            }, 100);
            
            // Show success message and reset form
            successFlash("QR code downloaded successfully!");
            resetForm();
            
            // Reload the QR codes list if the code was stored
            if (storeInDb) {
                loadQRCodes();
            }
        } else {
            errorFlash(data.message || "Error generating QR code");
        }
    })
    .error(function(data) {
        errorFlash(data.responseJSON.message || "Error generating QR code");
    });
}

$(document).ready(function () {
    // Initialize tooltips
    $('[data-toggle="tooltip"]').tooltip();
    
    // Set focus on URL field
    $("#url").focus();
    
    // Enable enter key to generate QR code
    $("#url, #size").keypress(function(e) {
        if (e.which == 13) { // Enter key
            generate_code();
            return false;
        }
    });
    
    // Load the QR codes list
    loadQRCodes();
    
    // Keep CKEditor compatibility for other pages
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
})
