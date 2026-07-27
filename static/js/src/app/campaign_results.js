var map = null
var doPoll = true;

// statuses is a helper map to point result statuses to ui classes
var statuses = {
    "Email Sent": {
        color: "#1abc9c",
        label: "label-success",
        icon: "fa-envelope",
        point: "ct-point-sent"
    },
    "Emails Sent": {
        color: "#1abc9c",
        label: "label-success",
        icon: "fa-envelope",
        point: "ct-point-sent"
    },
    "SMS Sent": {
        color: "#1abc9c",
        label: "label-success",
        icon: "fa-mobile",
        point: "ct-point-sent"
    },
    "In progress": {
        label: "label-primary"
    },
    "Queued": {
        label: "label-info"
    },
    "Completed": {
        label: "label-success"
    },
    "Email Opened": {
        color: "#f9bf3b",
        label: "label-warning",
        icon: "fa-envelope-open",
        point: "ct-point-opened"
    },
    "Clicked Link": {
        color: "#F39C12",
        label: "label-clicked",
        icon: "fa-mouse-pointer",
        point: "ct-point-clicked"
    },
    "Success": {
        color: "#f05b4f",
        label: "label-danger",
        icon: "fa-exclamation",
        point: "ct-point-clicked"
    },
    // Not a status, but is used for the campaign timeline and user timeline
    "Email Reported": {
        color: "#45d6ef",
        label: "label-info",
        icon: "fa-bullhorn",
        point: "ct-point-reported"
    },
    "Email Replied": {
        color: "#E67E22",
        label: "label-clicked",
        icon: "fa-reply",
        point: "ct-point-replied"
    },
    "Error": {
        color: "#6c7a89",
        label: "label-default",
        icon: "fa-times",
        point: "ct-point-error"
    },
    "Error Sending Email": {
        color: "#6c7a89",
        label: "label-default",
        icon: "fa-times",
        point: "ct-point-error"
    },
    "Error Sending SMS": {
        color: "#6c7a89",
        label: "label-default",
        icon: "fa-times",
        point: "ct-point-error"
    },
    "Submitted Data": {
        color: "#f05b4f",
        label: "label-danger",
        icon: "fa-exclamation",
        point: "ct-point-clicked"
    },
    "Unknown": {
        color: "#6c7a89",
        label: "label-default",
        icon: "fa-question",
        point: "ct-point-error"
    },
    "Sending": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-spinner",
        point: "ct-point-sending"
    },
    "Retrying": {
        color: "#6c7a89",
        label: "label-default",
        icon: "fa-clock-o",
        point: "ct-point-error"
    },
    "Scheduled": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-clock-o",
        point: "ct-point-sending"
    },
    "Campaign Created": {
        label: "label-success",
        icon: "fa-rocket"
    },
    "Link Created": {
        color: "#1abc9c",
        label: "label-success",
        icon: "fa-link",
        point: "ct-point-sent"
    },
    "MFA Code Sent": {
        color: "#9b59b6",
        label: "label-mfa",
        icon: "fa-mobile",
        point: "ct-point-mfa"
    },
    "MFA Code Verified": {
        color: "#27ae60",
        label: "label-success",
        icon: "fa-check-circle",
        point: "ct-point-mfa"
    },
    "MFA Code Send Error": {
        color: "#d35400",
        label: "label-warning",
        icon: "fa-exclamation-triangle",
        point: "ct-point-mfa"
    },
    "MFA Code Failed": {
        color: "#e74c3c",
        label: "label-danger",
        icon: "fa-times-circle",
        point: "ct-point-mfa"
    },
    "Email Re-queued": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-undo",
        point: "ct-point-sending"
    },
    "Failed Emails Re-queued": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-undo",
        point: "ct-point-sending"
    },
    "SMS Re-queued": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-undo",
        point: "ct-point-sending"
    },
    "Failed SMS Re-queued": {
        color: "#428bca",
        label: "label-primary",
        icon: "fa-undo",
        point: "ct-point-sending"
    }
}

var statusMapping = {
    "Email Sent": "sent",
    "SMS Sent": "sent",
    "Email Opened": "opened",
    "Clicked Link": "clicked",
    "Submitted Data": "submitted_data",
    "Email Reported": "reported",
    "Email Replied": "replied",
}

// This is an underwhelming attempt at an enum
// until I have time to refactor this appropriately.
var progressListing = [
    "Email Sent",
    "SMS Sent",
    "Email Opened",
    "Clicked Link",
    "Submitted Data"
]

var campaign = {}
var bubbles = []

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#modal").modal('hide')
    $("#resultsTable").dataTable().DataTable().clear().draw()
}

// Deletes a campaign after prompting the user
function deleteCampaign() {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete Campaign",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaign.id)
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
                'Campaign Deleted!',
                'This campaign has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.href = '/campaigns'
        })
    })
}

// Generate a new tracking link for generic campaigns
function generateNewLink() {
    Swal.fire({
        title: "Generate New Tracking Link",
        html: '<p>Enter a name for this link (optional):</p>' +
              '<input type="text" id="linkNameInput" class="swal2-input" placeholder="e.g., QR Poster - Building A">' +
              '<p style="font-size: 12px; color: #666; margin-top: 10px;">Leave empty to auto-generate (Link 1, Link 2, etc.)</p>',
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Generate",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            var linkName = document.getElementById('linkNameInput').value.trim();
            return new Promise(function (resolve, reject) {
                api.campaignId.generateLink(campaign.id, linkName)
                    .success(function (data) {
                        resolve(data)
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value) {
            var urlParam = campaign.urlparam || 'rid';
            var newTrackingUrl = campaign.url + '?' + urlParam + '=' + result.value.id;
            var newLinkId = result.value.id;
            
            Swal.fire({
                title: 'New Link Generated!',
                html: '<p>Your new tracking URL:</p>' +
                      '<div class="input-group" style="margin-top: 10px;">' +
                      '<input type="text" class="form-control" id="newTrackingUrlInput" value="' + escapeHtml(newTrackingUrl) + '" readonly>' +
                      '<span class="input-group-btn">' +
                      '<button class="btn btn-primary" type="button" onclick="copyNewTrackingUrl()"><i class="fa fa-copy"></i> Copy</button>' +
                      '<button class="btn btn-info" type="button" id="newLinkQRBtn" onclick="downloadNewLinkQR(\'' + newLinkId + '\', \'' + escapeHtml(newTrackingUrl) + '\')"><i class="fa fa-qrcode"></i> QR</button>' +
                      '</span></div>',
                type: 'success'
            }).then(function() {
                // Full page reload after user closes the modal
                location.reload();
            });
        }
    })
}

// Copy new tracking URL to clipboard
function copyNewTrackingUrl() {
    var input = document.getElementById('newTrackingUrlInput');
    input.select();
    input.setSelectionRange(0, 99999);
    document.execCommand('copy');
    
    // Show feedback
    var btn = $(event.target).closest('button');
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-check"></i> Copied!');
    btn.removeClass('btn-primary').addClass('btn-success');
    
    setTimeout(function() {
        btn.html(originalHtml);
        btn.removeClass('btn-success').addClass('btn-primary');
    }, 2000);
}

// Copy tracking URL from results page to clipboard
function copyTrackingUrlResults() {
    var input = document.getElementById('trackingUrlDisplay');
    input.select();
    input.setSelectionRange(0, 99999);
    document.execCommand('copy');
    
    // Show feedback
    var btn = $(event.target).closest('button');
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-check"></i> Copied!');
    btn.removeClass('btn-primary').addClass('btn-success');
    
    setTimeout(function() {
        btn.html(originalHtml);
        btn.removeClass('btn-success').addClass('btn-primary');
    }, 2000);
}

// Copy a specific link URL from the table
function copyLinkUrl(linkId) {
    var input = document.getElementById('linkUrl_' + linkId);
    input.select();
    input.setSelectionRange(0, 99999);
    document.execCommand('copy');
    
    // Show feedback
    var btn = $('#copyBtn_' + linkId);
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-check"></i>');
    btn.removeClass('btn-default').addClass('btn-success');
    
    setTimeout(function() {
        btn.html(originalHtml);
        btn.removeClass('btn-success').addClass('btn-default');
    }, 2000);
}

// Download QR code for a specific link
function downloadLinkQR(linkId, trackingUrl) {
    var btn = $('#qrBtn_' + linkId);
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-spinner fa-spin"></i>');
    btn.prop('disabled', true);
    
    api.qr_code.post({
        url: trackingUrl,
        size: "256",
        storeInDb: false
    }).success(function(data) {
        if (data.qr_code_base64) {
            // Convert base64 to blob and download
            var byteCharacters = atob(data.qr_code_base64);
            var byteNumbers = new Array(byteCharacters.length);
            for (var i = 0; i < byteCharacters.length; i++) {
                byteNumbers[i] = byteCharacters.charCodeAt(i);
            }
            var byteArray = new Uint8Array(byteNumbers);
            var blob = new Blob([byteArray], {type: 'image/png'});
            
            // Create download link
            var url = window.URL.createObjectURL(blob);
            var a = document.createElement('a');
            a.href = url;
            a.download = 'qr_' + linkId + '.png';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            
            // Show success
            btn.html('<i class="fa fa-check"></i>');
            btn.removeClass('btn-default').addClass('btn-success');
        }
    }).error(function(data) {
        errorFlash("Error generating QR code: " + (data.responseJSON?.message || "Unknown error"));
        btn.html(originalHtml);
    }).always(function() {
        btn.prop('disabled', false);
        setTimeout(function() {
            btn.html(originalHtml);
            btn.removeClass('btn-success').addClass('btn-default');
        }, 2000);
    });
}

// Download QR code for newly generated link (from modal)
function downloadNewLinkQR(linkId, trackingUrl) {
    var btn = $('#newLinkQRBtn');
    var originalHtml = btn.html();
    btn.html('<i class="fa fa-spinner fa-spin"></i>');
    btn.prop('disabled', true);
    
    api.qr_code.post({
        url: trackingUrl,
        size: "256",
        storeInDb: false
    }).success(function(data) {
        if (data.qr_code_base64) {
            // Convert base64 to blob and download
            var byteCharacters = atob(data.qr_code_base64);
            var byteNumbers = new Array(byteCharacters.length);
            for (var i = 0; i < byteCharacters.length; i++) {
                byteNumbers[i] = byteCharacters.charCodeAt(i);
            }
            var byteArray = new Uint8Array(byteNumbers);
            var blob = new Blob([byteArray], {type: 'image/png'});
            
            // Create download link
            var url = window.URL.createObjectURL(blob);
            var a = document.createElement('a');
            a.href = url;
            a.download = 'qr_' + linkId + '.png';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
            
            // Show success
            btn.html('<i class="fa fa-check"></i> Done!');
            btn.removeClass('btn-info').addClass('btn-success');
        }
    }).error(function(data) {
        errorFlash("Error generating QR code: " + (data.responseJSON?.message || "Unknown error"));
        btn.html(originalHtml);
    }).always(function() {
        btn.prop('disabled', false);
        setTimeout(function() {
            btn.html(originalHtml);
            btn.removeClass('btn-success').addClass('btn-info');
        }, 2000);
    });
}

// Count events per link for generic campaigns
function countEventsPerLink(timeline, rid) {
    var clicks = 0;
    var submissions = 0;
    
    $.each(timeline, function(i, event) {
        if (event.email === rid) {
            if (event.message === "Clicked Link") {
                clicks++;
            } else if (event.message === "Submitted Data") {
                submissions++;
            }
        }
    });
    
    return { clicks: clicks, submissions: submissions };
}

// Completes a campaign after prompting the user
function completeCampaign() {
    Swal.fire({
        title: "Are you sure?",
        text: "Gophish will stop processing events for this campaign",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Complete Campaign",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.complete(campaign.id)
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
                'Campaign Completed!',
                'This campaign has been completed!',
                'success'
            );
            $('#complete_button')[0].disabled = true;
            $('#complete_button').text('Completed!')
            doPoll = false;
        }
    })
}

// Exports campaign results as a CSV file
function exportAsCSV(scope) {
    exportHTML = $("#exportButton").html()
    var csvScope = null
    var filename = campaign.name + ' - ' + capitalize(scope) + '.csv'
    switch (scope) {
        case "results":
            csvScope = campaign.results.map(result => {
                // Exclude sms_target field
                const { sms_target, ...rest } = result;
                return rest;
            });
            break;
        case "events":
            csvScope = campaign.timeline
            break;
    }
    if (!csvScope) {
        return
    }
    $("#exportButton").html('<i class="fa fa-spinner fa-spin"></i>')
    var csvString = Papa.unparse(csvScope, {
        'escapeFormulae': true
    })
    var csvData = new Blob([csvString], {
        type: 'text/csv;charset=utf-8;'
    });
    if (navigator.msSaveBlob) {
        navigator.msSaveBlob(csvData, filename);
    } else {
        var csvURL = window.URL.createObjectURL(csvData);
        var dlLink = document.createElement('a');
        dlLink.href = csvURL;
        dlLink.setAttribute('download', filename)
        document.body.appendChild(dlLink)
        dlLink.click();
        document.body.removeChild(dlLink)
    }
    $("#exportButton").html(exportHTML)
}

function replay(event_idx) {
    request = campaign.timeline[event_idx]
    details = JSON.parse(request.details)
    url = null
    form = $('<form>').attr({
        method: 'POST',
        target: '_blank',
    })
    /* Create a form object and submit it */
    $.each(Object.keys(details.payload), function (i, param) {
        if (param == "rid") {
            return true;
        }
        if (param == "__original_url") {
            url = details.payload[param];
            return true;
        }
        $('<input>').attr({
            name: param,
        }).val(details.payload[param]).appendTo(form);
    })
    /* Ensure we know where to send the user */
    // Prompt for the URL
    Swal.fire({
        title: 'Where do you want the credentials submitted to?',
        input: 'text',
        showCancelButton: true,
        inputPlaceholder: "http://example.com/login",
        inputValue: url || "",
        inputValidator: function (value) {
            return new Promise(function (resolve, reject) {
                if (value) {
                    resolve();
                } else {
                    reject('Invalid URL.');
                }
            });
        }
    }).then(function (result) {
        if (result.value) {
            url = result.value
            form.attr({
                action: url
            })
            form.appendTo('body').submit().remove()
        }
    })
}

/**
 * Returns an HTML string that displays the OS and browser that clicked the link
 * or submitted credentials.
 * 
 * @param {object} event_details - The "details" parameter for a campaign
 *  timeline event
 * 
 */
var renderDevice = function (event_details) {
    var ua = UAParser(details.browser['user-agent'])
    var detailsString = '<div class="timeline-device-details">'

    var deviceIcon = 'laptop'
    if (ua.device.type) {
        if (ua.device.type == 'tablet' || ua.device.type == 'mobile') {
            deviceIcon = ua.device.type
        }
    }

    var deviceVendor = ''
    if (ua.device.vendor) {
        deviceVendor = ua.device.vendor.toLowerCase()
        if (deviceVendor == 'microsoft') deviceVendor = 'windows'
    }

    var deviceName = 'Unknown'
    if (ua.os.name) {
        deviceName = ua.os.name
        if (deviceName == "Mac OS") {
            deviceVendor = 'apple'
        } else if (deviceName == "Windows") {
            deviceVendor = 'windows'
        }
        if (ua.device.vendor && ua.device.model) {
            deviceName = ua.device.vendor + ' ' + ua.device.model
        }
    }

    if (ua.os.version) {
        deviceName = deviceName + ' (OS Version: ' + ua.os.version + ')'
    }

    deviceString = '<div class="timeline-device-os"><span class="fa fa-stack">' +
        '<i class="fa fa-' + escapeHtml(deviceIcon) + ' fa-stack-2x"></i>' +
        '<i class="fa fa-vendor-icon fa-' + escapeHtml(deviceVendor) + ' fa-stack-1x"></i>' +
        '</span> ' + escapeHtml(deviceName) + '</div>'

    detailsString += deviceString

    var deviceBrowser = 'Unknown'
    var browserIcon = 'info-circle'
    var browserVersion = ''

    if (ua.browser && ua.browser.name) {
        deviceBrowser = ua.browser.name
        // Handle the "mobile safari" case
        deviceBrowser = deviceBrowser.replace('Mobile ', '')
        if (deviceBrowser) {
            browserIcon = deviceBrowser.toLowerCase()
            if (browserIcon == 'ie') browserIcon = 'internet-explorer'
        }
        browserVersion = '(Version: ' + ua.browser.version + ')'
    }

    var browserString = '<div class="timeline-device-browser"><span class="fa fa-stack">' +
        '<i class="fa fa-' + escapeHtml(browserIcon) + ' fa-stack-1x"></i></span> ' +
        deviceBrowser + ' ' + browserVersion + '</div>'

    detailsString += browserString
    detailsString += '</div>'
    return detailsString
}

function renderTimeline(data) {
    record = {
        "id": data[0],
        "first_name": data[2],
        "last_name": data[3],
        "email": data[4],
        "phone": data[10],  // Adjusted index as we removed the replied column
        "position": data[5],
        "status": data[6],
        "reported": data[7],
        "send_date": data[8],  // Adjusted index as we removed the replied column
        "sms_target": data[9]  // Adjusted index as we removed the replied column
    }

    // Check if this is an SMS campaign result
    const isSMS = record.sms_target || (record.email && !record.email.includes('@'));

    var contactLabel = isSMS ? "Number: " : "Email: ";

    results = '<div class="timeline col-sm-12 well well-lg">' +
        '<h6>Timeline for ' + escapeHtml(record.first_name) + ' ' + escapeHtml(record.last_name) +
        '</h6><span class="subtitle">' + contactLabel + escapeHtml(isSMS ? (record.phone || record.email) : record.email || 'N/A') +
        '<br>Result ID: ' + escapeHtml(record.id) + '</span>' +
        '<div class="timeline-graph col-sm-6">'
    $.each(campaign.timeline, function (i, event) {
        const contact = isSMS ? (record.phone || record.email) : record.email;
        // Match by contact (email/phone) OR by RId (for generic campaigns where events use RId)
        if (!event.email || event.email == contact || event.email == record.id) {
            // Add the event
            results += '<div class="timeline-entry">' +
                '    <div class="timeline-bar"></div>'
            results +=
                '    <div class="timeline-icon ' + (statuses[event.message]?.label || 'label-default') + '">' +
                '    <i class="fa ' + (statuses[event.message]?.icon || 'fa-question') + '"></i></div>' +
                '    <div class="timeline-message">' + escapeHtml(event.message) +
                '    <span class="timeline-date">' + moment.utc(event.time).local().format('MMMM Do YYYY h:mm:ss a') + '</span>'
            if (event.details) {
                details = JSON.parse(event.details)
                if (event.message == "Clicked Link" || event.message == "Submitted Data") {
                    deviceView = renderDevice(details)
                    if (deviceView) {
                        results += deviceView
                    }
                }
                if (event.message == "Submitted Data") {
                    results += '<div class="timeline-replay-button"><button onclick="replay(' + i + ')" class="btn btn-success">'
                    results += '<i class="fa fa-refresh"></i> Replay Credentials</button></div>'
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                }
                if (details.payload) {
                    results += '<div class="timeline-event-results">'
                    results += '    <table class="table table-condensed table-bordered table-striped">'
                    results += '        <thead><tr><th>Parameter</th><th>Value(s)</tr></thead><tbody>'
                    $.each(Object.keys(details.payload), function (i, param) {
                        if (param == "rid") {
                            return true;
                        }
                        results += '    <tr>'
                        results += '        <td>' + escapeHtml(param) + '</td>'
                        results += '        <td>' + escapeHtml(details.payload[param]) + '</td>'
                        results += '    </tr>'
                    })
                    results += '       </tbody></table>'
                    results += '</div>'
                }
                if (details.error) {
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                    results += '<div class="timeline-event-results">'
                    results += '<span class="label label-default">Error</span> ' + details.error
                    results += '</div>'
                }
            }
            results += '</div></div>'
        }
    })
    // Add the scheduled send event at the bottom
    if (record.status == "Scheduled" || record.status == "Retrying") {
        results += '<div class="timeline-entry">' +
            '    <div class="timeline-bar"></div>'
        results +=
            '    <div class="timeline-icon ' + (statuses[record.status]?.label || 'label-default') + '">' +
            '    <i class="fa ' + (statuses[record.status]?.icon || 'fa-question') + '"></i></div>' +
            '    <div class="timeline-message">' + "Scheduled to send at " + record.send_date + '</div>'
    }
    results += '</div></div>'
    return results
}

var renderTimelineChart = function (chartopts) {
    return Highcharts.chart('timeline_chart', {
        chart: {
            zoomType: 'x',
            type: 'line',
            height: "200px"
        },
        title: {
            text: 'Campaign Timeline'
        },
        xAxis: {
            type: 'datetime',
            dateTimeLabelFormats: {
                second: '%l:%M:%S',
                minute: '%l:%M',
                hour: '%l:%M',
                day: '%b %d, %Y',
                week: '%b %d, %Y',
                month: '%b %Y'
            }
        },
        yAxis: {
            min: 0,
            max: 2,
            visible: false,
            tickInterval: 1,
            labels: {
                enabled: false
            },
            title: {
                text: ""
            }
        },
        tooltip: {
            formatter: function () {
                // Check if this point is for an SMS target
                const isSMS = this.point.sms_target || (this.point.email && !this.point.email.includes('@'));
                const contactLabel = isSMS ? "Number: " : "Email: ";
                const contactValue = this.point.contact || 'N/A';

                return Highcharts.dateFormat('%A, %b %d %l:%M:%S %P', new Date(this.x)) +
                    '<br>Event: ' + this.point.message + '<br>' + contactLabel + '<b>' + contactValue + '</b>';
            }
        },
        legend: {
            enabled: false
        },
        plotOptions: {
            series: {
                marker: {
                    enabled: true,
                    symbol: 'circle',
                    radius: 3
                },
                cursor: 'pointer',
            },
            line: {
                states: {
                    hover: {
                        lineWidth: 1
                    }
                }
            }
        },
        credits: {
            enabled: false
        },
        series: [{
            data: chartopts['data'],
            dashStyle: "shortdash",
            color: "#cccccc",
            lineWidth: 1,
            turboThreshold: 0
        }]
    })
}

/* Renders a pie chart using the provided chartops */
var renderPieChart = function (chartopts) {
    return Highcharts.chart(chartopts['elemId'], {
        chart: {
            type: 'pie',
            events: {
                load: function () {
                    var chart = this,
                        rend = chart.renderer,
                        pie = chart.series[0],
                        left = chart.plotLeft + pie.center[0],
                        top = chart.plotTop + pie.center[1];
                    this.innerText = rend.text(chartopts['data'][0].count, left, top).
                        attr({
                            'text-anchor': 'middle',
                            'font-size': '24px',
                            'font-weight': 'bold',
                            'fill': chartopts['colors'][0],
                            'font-family': 'Helvetica,Arial,sans-serif'
                        }).add();
                },
                render: function () {
                    this.innerText.attr({
                        text: chartopts['data'][0].count
                    })
                }
            }
        },
        title: {
            text: chartopts['title']
        },
        plotOptions: {
            pie: {
                innerSize: '80%',
                dataLabels: {
                    enabled: false
                }
            }
        },
        credits: {
            enabled: false
        },
        tooltip: {
            formatter: function () {
                if (this.key == undefined) {
                    return false
                }
                return '<span style="color:' + this.color + '">\u25CF</span>' + this.point.name + ': <b>' + this.y + '%</b><br/>'
            }
        },
        series: [{
            data: chartopts['data'],
            colors: chartopts['colors'],
        }]
    })
}

/* Updates the bubbles on the map

@param {campaign.result[]} results - The campaign results to process
*/
var updateMap = function (results) {
    if (!map) {
        return
    }
    bubbles = []
    $.each(campaign.results, function (i, result) {
        // Check that it wasn't an internal IP
        if (result.latitude == 0 && result.longitude == 0) {
            return true;
        }
        newIP = true
        $.each(bubbles, function (i, bubble) {
            if (bubble.ip == result.ip) {
                bubbles[i].radius += 1
                newIP = false
                return false
            }
        })
        if (newIP) {
            bubbles.push({
                latitude: result.latitude,
                longitude: result.longitude,
                name: result.ip,
                fillKey: "point",
                radius: 2
            })
        }
    })
    map.bubbles(bubbles)
}

/**
 * Creates a status label for use in the results datatable
 * @param {string} status 
 * @param {moment(datetime)} send_date 
 */
function createStatusLabel(status, send_date) {
    var label = statuses[status]?.label || "label-default";
    var statusColumn = "<span class=\"label " + label + "\">" + status + "</span>"
    // Add the tooltip if the email is scheduled to be sent
    if (status == "Scheduled" || status == "Retrying") {
        var sendDateMessage = "Scheduled to send at " + send_date
        statusColumn = "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"top\" data-html=\"true\" title=\"" + sendDateMessage + "\">" + status + "</span>"
    }
    return statusColumn
}

/**
 * Processes campaign timeline data and calculates email series data
 * @param {Array} timeline - Campaign timeline events
 * @param {Array} results - Campaign results
 * @returns {Object} email_series_data - Processed data for charts
 */
function processTimelineData(timeline, results) {
    var email_series_data = {};
    
    // Determine campaign type and set relevant statuses
    var isSMSCampaign = results.every(r => r.sms_target);
    var relevantStatuses = isSMSCampaign
        ? ["SMS Sent", "Clicked Link", "Submitted Data"]
        : ["Email Sent", "Email Opened", "Clicked Link", "Submitted Data", "Email Reported", "Email Replied"];
        
    relevantStatuses.forEach(function (k) {
        email_series_data[k] = 0;
    });

    var processed_results = {};
    timeline.forEach(function(event) {
        // Initialize a new result if we haven't seen it before
        if (!processed_results[event.email]) {
            processed_results[event.email] = {
                "sent": false,
                "opened": false,
                "clicked": false,
                "submitted_data": false,
                "replied": false,
                "reported": false
            }
        }
        switch (event.message) {
            case "Email Sent":
            case "SMS Sent":
                processed_results[event.email]["sent"] = true;
                break;
            case "Email Opened":
                processed_results[event.email]["opened"] = true;
                break;
            case "Clicked Link":
                processed_results[event.email]["clicked"] = true;
                break;
            case "Submitted Data":
                processed_results[event.email]["submitted_data"] = true;
                break;
            case "Email Replied":
                processed_results[event.email]["replied"] = true;
                break;
            case "Email Reported":
                processed_results[event.email]["reported"] = true;
                break;
        }
    });

    Object.keys(processed_results).forEach(function(email) {
        var result = processed_results[email];
        if (result.submitted_data) {
            result.clicked = true;
        }
        if (result.clicked) {
            result.opened = true;
        }
        if (result.replied) {
            result.opened = true;
        }
        if (result.opened || result.replied) {
            result.sent = true;
        }
        if (result.sent) {
            const sent_key = isSMSCampaign ? "SMS Sent" : "Email Sent";
            email_series_data[sent_key]++;
        }
        if (result.opened) {
            email_series_data["Email Opened"]++;
        }
        if (result.clicked) {
            email_series_data["Clicked Link"]++;
        }
        if (result.submitted_data) {
            email_series_data["Submitted Data"]++;
        }
        if (result.replied) {
            email_series_data["Email Replied"]++;
        }
        if (result.reported) {
            email_series_data["Email Reported"]++;
        }
    });

    return email_series_data;
}

/* poll - Queries the API and updates the UI with the results
 *
 * Updates:
 * * Timeline Chart
 * * Email (Donut) Chart
 * * Map Bubbles
 * * Datatables
 */
function poll() {
    api.campaignId.results(campaign.id)
        .success(function (c) {
            campaign = c;
            
            const isGenericCampaign = campaign.type === "generic";

            /* Update the timeline */
            var timeline_series_data = buildTimelineData(campaign.timeline, campaign.results);

            var timeline_chart = $("#timeline_chart").highcharts()
            if (timeline_chart) {
                timeline_chart.series[0].update({
                    data: timeline_series_data
                })
            }

            /* Update the results donut chart */
            var email_series_data = processTimelineData(campaign.timeline, campaign.results);

            $.each(email_series_data, function (status, count) {
                var email_data = []
                if (!(status in statusMapping)) {
                    return true
                }
                email_data.push({
                    name: status,
                    y: Math.floor((count / campaign.results.length) * 100),
                    count: count
                })
                email_data.push({
                    name: '',
                    y: 100 - Math.floor((count / campaign.results.length) * 100)
                })
                var chart = $("#" + statusMapping[status] + "_chart").highcharts()
                if (chart) {
                    chart.series[0].update({
                        data: email_data
                    })
                }
            })

            /* Update the datatable - different handling for generic campaigns */
            if (isGenericCampaign) {
                // Update generic links table
                var genericLinksTable = $("#genericLinksTable").DataTable();
                var urlParam = campaign.urlparam || 'rid';
                
                genericLinksTable.rows().every(function (i, tableLoop, rowLoop) {
                    var row = this.row(i);
                    var rowData = row.data();
                    var linkId = rowData[0];
                    
                    $.each(campaign.results, function (j, result) {
                        if (result.id == linkId) {
                            var eventCounts = countEventsPerLink(campaign.timeline, result.id);
                            rowData[4] = eventCounts.clicks;
                            rowData[5] = eventCounts.submissions;
                            rowData[6] = getBestStatus(result, campaign.timeline);
                            rowData[7] = moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a');
                            genericLinksTable.row(i).data(rowData);
                            
                            if (row.child.isShown()) {
                                $(row.node()).find("#caret").removeClass("fa-caret-right")
                                $(row.node()).find("#caret").addClass("fa-caret-down")
                                row.child(renderGenericTimeline(row.data()));
                            }
                            return false;
                        }
                    });
                });
                genericLinksTable.draw(false);
            } else {
                // Update standard results table
                resultsTable = $("#resultsTable").DataTable()
                resultsTable.rows().every(function (i, tableLoop, rowLoop) {
                    var row = this.row(i)
                    var rowData = row.data()
                    var rid = rowData[0]
                    $.each(campaign.results, function (j, result) {
                        if (result.id == rid) {
                            rowData[8] = moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a')
                            rowData[7] = result.reported
                            // Preserve "Email Replied" status if replied is true
                            rowData[6] = getBestStatus(result, campaign.timeline);

                            rowData[9] = result.sms_target
                            resultsTable.row(i).data(rowData)
                            if (row.child.isShown()) {
                                $(row.node()).find("#caret").removeClass("fa-caret-right")
                                $(row.node()).find("#caret").addClass("fa-caret-down")
                                row.child(renderTimeline(row.data()))
                            }
                            return false
                        }
                    })
                })
                resultsTable.draw(false)
                /* Update the map information */
                updateMap(campaign.results)
            }
            
            $('[data-toggle="tooltip"]').tooltip()
            $("#refresh_message").hide()
            $("#refresh_btn").show()
        })
}

function load() {
    campaign.id = window.location.pathname.split('/').slice(-1)[0]
    var use_map = JSON.parse(localStorage.getItem('gophish.use_map'))

    // Get the campaign results
    api.campaignId.results(campaign.id)
        .success(function (c) {
            campaign = c;

            if (campaign) {
                $("title").text(c.name + " - Gophish")
                $("#loading").hide()
                $("#campaignResults").show()
                // Set the title
                $("#page-title").text("Results for " + c.name)
                if (c.status == "Completed") {
                    $('#complete_button')[0].disabled = true;
                    $('#complete_button').text('Completed!');
                    doPoll = false;
                }
                // Setup viewing the details of a result
                $("#resultsTable").on("click", ".timeline-event-details", function () {
                    // Show the parameters
                    payloadResults = $(this).parent().find(".timeline-event-results")
                    if (payloadResults.is(":visible")) {
                        $(this).find("i").removeClass("fa-caret-down")
                        $(this).find("i").addClass("fa-caret-right")
                        payloadResults.hide()
                    } else {
                        $(this).find("i").removeClass("fa-caret-right")
                        $(this).find("i").addClass("fa-caret-down")
                        payloadResults.show()
                    }
                })

                // Check if any results have sms_target set
                const hasSMSTargets = campaign.results.some(function (result) {
                    return result.sms_target;
                });

                // Check if ALL results are SMS targets (SMS-only campaign)
                const isSMSOnlyCampaign = campaign.results.length > 0 && campaign.results.every(function (result) {
                    return result.sms_target;
                });


                // Update label on table if needed
                if (hasSMSTargets) {
                    $("#resultsTable th:contains('Email')").text("Contact");
                }

                // Handle SMS-only campaign chart layout
                if (isSMSOnlyCampaign) {
                    // Hide the charts that don't apply to SMS campaigns
                    $("#opened_chart").parent().hide();
                    $("#reported_chart").parent().hide();
                    $("#replied_chart").parent().hide();
                    
                    // Move the submitted_data_chart to the first row
                    $("#submitted_data_chart").parent().detach().insertAfter($("#clicked_chart").parent());
                    
                    // Hide the entire second row
                    $("#submitted_data_chart").parent().parent().next().hide();
                    
                    // Adjust width of visible charts for balanced layout
                    $("#sent_chart").parent().removeClass("col-md-4").addClass("col-md-4");
                    $("#clicked_chart").parent().removeClass("col-md-4").addClass("col-md-4");
                    $("#submitted_data_chart").parent().removeClass("col-md-4").addClass("col-md-4");
                }
                
                // Handle Generic campaign layout
                const isGenericCampaign = campaign.type === "generic";
                if (isGenericCampaign) {
                    // Hide all charts except clicked and submitted_data
                    $("#sent_chart").parent().hide();
                    $("#opened_chart").parent().hide();
                    $("#reported_chart").parent().hide();
                    $("#replied_chart").parent().hide();
                    
                    // Move clicked and submitted to the first row and center them
                    $("#clicked_chart").parent().removeClass("col-md-4").addClass("col-md-6");
                    $("#submitted_data_chart").parent().removeClass("col-md-4").addClass("col-md-6");
                    $("#submitted_data_chart").parent().detach().insertAfter($("#clicked_chart").parent());
                    
                    // Show the generate link button
                    $("#generate_link_btn").show();
                    
                    // Hide tracking URL section for generic campaigns (we show URLs in the table)
                    $("#tracking_url_section").hide();
                    
                    // Update page title to indicate generic campaign
                    $("#page-title").text("Results for " + c.name);
                    
                    // Hide the standard results table and show generic links table
                    $("#resultsTable").hide();
                    $("#genericLinksTable").show();
                    
                    // Setup the generic links table
                    var genericLinksTable = $("#genericLinksTable").DataTable({
                        destroy: true,
                        "order": [[7, "desc"]], // Order by Created date descending
                        columnDefs: [{
                            orderable: false,
                            targets: "no-sort"
                        }, {
                            className: "details-control",
                            "targets": [1]
                        }, {
                            "visible": false,
                            "targets": [0] // Hide Link ID column (used internally)
                        },
                        {
                            // Clicks column - centered and orange colored
                            className: "text-center",
                            "render": function (data, type, row) {
                                if (type === "display") {
                                    return '<span style="color: #F39C12; font-weight: bold;">' + data + '</span>';
                                }
                                return data;
                            },
                            "targets": [4]
                        },
                        {
                            // Submissions column - centered and red colored
                            className: "text-center",
                            "render": function (data, type, row) {
                                if (type === "display") {
                                    return '<span style="color: #f05b4f; font-weight: bold;">' + data + '</span>';
                                }
                                return data;
                            },
                            "targets": [5]
                        },
                        {
                            "render": function (data, type, row) {
                                return createStatusLabel(data, row[7])
                            },
                            "targets": [6]
                        }]
                    });
                    genericLinksTable.clear();
                    
                    var urlParam = campaign.urlparam || 'rid';
                    $.each(campaign.results, function (i, result) {
                        var trackingUrl = campaign.url + '?' + urlParam + '=' + result.id;
                        var eventCounts = countEventsPerLink(campaign.timeline, result.id);
                        var linkName = result.first_name || ('Link ' + (i + 1));
                        
                        genericLinksTable.row.add([
                            result.id,
                            "<i id=\"caret\" class=\"fa fa-caret-right\"></i>",
                            escapeHtml(linkName),
                            '<div class="input-group input-group-sm" style="width:100%">' +
                                '<input type="text" class="form-control" id="linkUrl_' + result.id + '" value="' + escapeHtml(trackingUrl) + '" readonly style="font-size:12px;">' +
                                '<span class="input-group-btn">' +
                                    '<button id="copyBtn_' + result.id + '" class="btn btn-default" type="button" onclick="copyLinkUrl(\'' + result.id + '\')" title="Copy URL">' +
                                        '<i class="fa fa-copy"></i>' +
                                    '</button>' +
                                    '<button id="qrBtn_' + result.id + '" class="btn btn-default" type="button" onclick="downloadLinkQR(\'' + result.id + '\', \'' + escapeHtml(trackingUrl) + '\')" title="Download QR Code">' +
                                        '<i class="fa fa-qrcode"></i>' +
                                    '</button>' +
                                '</span>' +
                            '</div>',
                            eventCounts.clicks,
                            eventCounts.submissions,
                            getBestStatus(result, campaign.timeline),
                            moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a')
                        ]);
                    });
                    genericLinksTable.draw();
                    
                    // Setup the individual timelines for generic links table
                    $('#genericLinksTable tbody').on('click', 'td.details-control', function () {
                        var tr = $(this).closest('tr');
                        var row = genericLinksTable.row(tr);
                        if (row.child.isShown()) {
                            row.child.hide();
                            tr.removeClass('shown');
                            $(this).find("i").removeClass("fa-caret-down")
                            $(this).find("i").addClass("fa-caret-right")
                        } else {
                            $(this).find("i").removeClass("fa-caret-right")
                            $(this).find("i").addClass("fa-caret-down")
                            row.child(renderGenericTimeline(row.data())).show();
                            tr.addClass('shown');
                        }
                    });
                    
                    // Setup viewing the details of a result for generic links table
                    $("#genericLinksTable").on("click", ".timeline-event-details", function () {
                        // Show the parameters
                        payloadResults = $(this).parent().find(".timeline-event-results")
                        if (payloadResults.is(":visible")) {
                            $(this).find("i").removeClass("fa-caret-down")
                            $(this).find("i").addClass("fa-caret-right")
                            payloadResults.hide()
                        } else {
                            $(this).find("i").removeClass("fa-caret-right")
                            $(this).find("i").addClass("fa-caret-down")
                            payloadResults.show()
                        }
                    })
                }

                // Setup the results table (for non-generic campaigns)
                if (!isGenericCampaign) {
                resultsTable = $("#resultsTable").DataTable({
                    destroy: true,
                    "order": [
                        [2, "asc"]
                    ],
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }, {
                        className: "details-control",
                        "targets": [1]
                    }, {
                        "visible": false,
                        "targets": [0, 8] // Updated index since we removed a column
                    },
                    {
                        "render": function (data, type, row) {
                            var label = createStatusLabel(data, row[8]);
                            if (row[11] === "Error" || row[11] === "Retrying") {
                                label += " <button class='btn btn-xs btn-warning' style='margin-left:4px' " +
                                    "onclick='resendResult(\"" + row[0] + "\")' title='Resend'>" +
                                    "<i class='fa fa-repeat'></i></button>";
                            }
                            return label;
                        },
                        "targets": [6]
                    },
                    {
                        className: "text-center",
                        "render": function (reported, type, row) {
                            if (type == "display") {
                                if (reported) {
                                    return "<i class='fa fa-check-circle text-center text-success'></i>"
                                }
                                return "<i role='button' class='fa fa-times-circle text-center text-muted' onclick='report_mail(\"" + row[0] + "\", \"" + campaign.id + "\");'></i>"
                            }
                            return reported
                        },
                        "targets": [7]
                    },
                    {
                        className: "text-center",
                        "render": function (replied, type, row) {
                            if (type == "display") {
                                if (replied) {
                                    return "<i class='fa fa-check-circle text-center text-success'></i>"
                                }
                                return "<i class='fa fa-times-circle text-center text-muted'></i>"
                            }
                            return replied
                        },
                        "targets": [8]
                    }
                    ]
                });
                resultsTable.clear();
                var timeline_series_data = buildTimelineData(campaign.timeline, campaign.results);
                
                $.each(campaign.results, function (i, result) {
                    const isSMS = result.sms_target;
                    resultsTable.row.add([
                        result.id,
                        "<i id=\"caret\" class=\"fa fa-caret-right\"></i>",
                        escapeHtml(result.first_name) || "",
                        escapeHtml(result.last_name) || "",
                        escapeHtml(isSMS ? (result.phone || result.email) : result.email) || "",
                        escapeHtml(result.position) || "",
                        // Update status to show "Email Replied" if the user replied
                        getBestStatus(result, campaign.timeline),
                        result.reported,
                        moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a'),
                        result.sms_target,
                        escapeHtml(result.phone) || "",
                        result.status  // [11] raw DB status — used by resend button condition
                    ]);
                });
                
                var email_series_data = processTimelineData(campaign.timeline, campaign.results);
                resultsTable.draw();
                // Setup tooltips
                $('[data-toggle="tooltip"]').tooltip()
                // Setup the individual timelines
                $('#resultsTable tbody').on('click', 'td.details-control', function () {
                    var tr = $(this).closest('tr');
                    var row = resultsTable.row(tr);
                    if (row.child.isShown()) {
                        // This row is already open - close it
                        row.child.hide();
                        tr.removeClass('shown');
                        $(this).find("i").removeClass("fa-caret-down")
                        $(this).find("i").addClass("fa-caret-right")
                    } else {
                        // Open this row
                        $(this).find("i").removeClass("fa-caret-right")
                        $(this).find("i").addClass("fa-caret-down")
                        row.child(renderTimeline(row.data())).show();
                        tr.addClass('shown');
                    }
                });

                renderTimelineChart({
                    data: timeline_series_data
                })

                // Render donut charts safely
                const isSMSCampaign = campaign.results.every(r => r.sms_target);
                const relevantStatuses = isSMSCampaign
                    ? ["SMS Sent", "Clicked Link", "Submitted Data"]
                    : ["Email Sent", "Email Opened", "Clicked Link", "Submitted Data", "Email Reported", "Email Replied"];
                    
                relevantStatuses.forEach(function (status) {
                    const count = email_series_data[status] || 0;

                    const email_data = [
                        {
                            name: status,
                            y: Math.floor((count / campaign.results.length) * 100),
                            count: count
                        },
                        {
                            name: '',
                            y: 100 - Math.floor((count / campaign.results.length) * 100)
                        }
                    ];

                    const chartContainer = $("#" + statusMapping[status] + "_chart");
                    const chart = chartContainer.highcharts();

                    if (chart) {
                        chart.series[0].update({ data: email_data });
                    } else {
                        renderPieChart({
                            elemId: statusMapping[status] + '_chart',
                            title: status,
                            name: status,
                            data: email_data,
                            colors: [statuses[status].color, '#dddddd']
                        });
                    }
                });

                if (use_map) {
                    $("#resultsMapContainer").show()
                    
                    // Get theme-aware map colors
                    var currentTheme = localStorage.getItem('gophish.theme') || 'default';
                    var mapColors = {
                        'default': {
                            fill: '#ffffff',
                            hover: '#1abc9c',
                            border: '#283F50',
                            point: '#283F50'
                        },
                        'dark-teal': {
                            fill: '#1e2a35',
                            hover: '#1abc9c',
                            border: '#3a4d5c',
                            point: '#1abc9c'
                        },
                        'dark-crimson': {
                            fill: '#1e2a35',
                            hover: '#dc3545',
                            border: '#3a4d5c',
                            point: '#dc3545'
                        },
                        'goldphish': {
                            fill: '#fffdf6',
                            hover: '#f47a20',
                            border: '#e0cba0',
                            point: '#f47a20'
                        },
                        'lagocephalus': {
                            fill: '#1b2b2f',
                            hover: '#7fa8af',
                            border: '#3a545a',
                            point: '#7fa8af'
                        },
                        'light-sand': {
                            fill: '#f5f0e4',
                            hover: '#a85030',
                            border: '#c6b998',
                            point: '#a85030'
                        },
                        'matrix': {
                            fill: '#0a120a',
                            hover: '#00ff41',
                            border: '#0f4a0f',
                            point: '#00ff41'
                        }
                    };
                    var colors = mapColors[currentTheme] || mapColors['default'];
                    
                    map = new Datamap({
                        element: document.getElementById("resultsMap"),
                        responsive: true,
                        fills: {
                            defaultFill: colors.fill,
                            point: colors.point
                        },
                        geographyConfig: {
                            highlightFillColor: colors.hover,
                            borderColor: colors.border
                        },
                        bubblesConfig: {
                            borderColor: colors.border
                        }
                    });
                }
                updateMap(campaign.results)
                } // End of !isGenericCampaign block
                
                // Render timeline and charts for generic campaigns
                if (isGenericCampaign) {
                    var timeline_series_data = buildTimelineData(campaign.timeline, campaign.results);
                    var email_series_data = processTimelineData(campaign.timeline, campaign.results);
                    
                    renderTimelineChart({
                        data: timeline_series_data
                    });
                    
                    // Render only clicked and submitted charts for generic campaigns
                    ["Clicked Link", "Submitted Data"].forEach(function (status) {
                        const count = email_series_data[status] || 0;
                        const email_data = [
                            {
                                name: status,
                                y: Math.floor((count / campaign.results.length) * 100),
                                count: count
                            },
                            {
                                name: '',
                                y: 100 - Math.floor((count / campaign.results.length) * 100)
                            }
                        ];
                        renderPieChart({
                            elemId: statusMapping[status] + '_chart',
                            title: status,
                            name: status,
                            data: email_data,
                            colors: [statuses[status].color, '#dddddd']
                        });
                    });
                }
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash(" Campaign not found!")
        })
}

// Render timeline for generic campaign links
function renderGenericTimeline(data) {
    var linkId = data[0];
    var linkName = data[2]; // Link name
    var status = data[6];
    var created = data[7];
    
    var results = '<div class="timeline col-sm-12 well well-lg">' +
        '<h6>Timeline for ' + escapeHtml(linkName) + '</h6>' +
        '<span class="subtitle">Link ID: ' + escapeHtml(linkId) + '<br>Status: ' + escapeHtml(status) + '<br>Created: ' + escapeHtml(created) + '</span>' +
        '<div class="timeline-graph col-sm-6">';
    
    $.each(campaign.timeline, function (i, event) {
        // Match events by link ID (stored in email field for generic campaigns)
        if (event.email === linkId) {
            results += '<div class="timeline-entry">' +
                '    <div class="timeline-bar"></div>';
            results +=
                '    <div class="timeline-icon ' + (statuses[event.message]?.label || 'label-default') + '">' +
                '    <i class="fa ' + (statuses[event.message]?.icon || 'fa-question') + '"></i></div>' +
                '    <div class="timeline-message">' + escapeHtml(event.message) +
                '    <span class="timeline-date">' + moment.utc(event.time).local().format('MMMM Do YYYY h:mm:ss a') + '</span>';
            
            if (event.details) {
                // Parse details and set as global for renderDevice function
                details = JSON.parse(event.details);
                if (event.message == "Clicked Link" || event.message == "Submitted Data") {
                    var deviceView = renderDevice(details);
                    if (deviceView) {
                        results += deviceView;
                    }
                }
                if (event.message == "Submitted Data") {
                    results += '<div class="timeline-replay-button"><button onclick="replay(' + i + ')" class="btn btn-success">';
                    results += '<i class="fa fa-refresh"></i> Replay Credentials</button></div>';
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>';
                }
                if (details.payload) {
                    results += '<div class="timeline-event-results">';
                    results += '    <table class="table table-condensed table-bordered table-striped">';
                    results += '        <thead><tr><th>Parameter</th><th>Value(s)</tr></thead><tbody>';
                    $.each(Object.keys(details.payload), function (j, param) {
                        if (param == "rid") {
                            return true;
                        }
                        results += '    <tr>';
                        results += '        <td>' + escapeHtml(param) + '</td>';
                        results += '        <td>' + escapeHtml(details.payload[param]) + '</td>';
                        results += '    </tr>';
                    });
                    results += '       </tbody></table>';
                    results += '</div>';
                }
            }
            results += '</div></div>';
        }
    });
    
    results += '</div></div>';
    return results;
}

var setRefresh

function refresh() {
    if (!doPoll) {
        return;
    }
    $("#refresh_message").show()
    $("#refresh_btn").hide()
    poll()
    clearTimeout(setRefresh)
    setRefresh = setTimeout(refresh, 60000)
};

function report_mail(rid, cid) {
    Swal.fire({
        title: "Are you sure?",
        text: "This result will be flagged as reported (RID: " + rid + ")",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Continue",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true
    }).then(function (result) {
        if (result.value) {
            api.campaignId.get(cid).success((function (c) {
                report_url = new URL(c.url)
                report_url.pathname = '/report'
                report_url.search = "?rid=" + rid
                fetch(report_url)
                    .then(response => {
                        if (!response.ok) {
                            throw new Error(`HTTP error! Status: ${response.status}`);
                        }
                        refresh();
                    })
                    .catch(error => {
                        let errorMessage = error.message;
                        if (error.message === "Failed to fetch") {
                            errorMessage = "This might be due to Mixed Content issues or network problems.";
                        }
                        Swal.fire({
                            title: 'Error',
                            text: errorMessage,
                            type: 'error',
                            confirmButtonText: 'Close'
                        });
                    });
            }));
        }
    })
}

function resendResult(rid) {
    Swal.fire({
        title: "Resend this message?",
        text: "This recipient will be re-queued and sent within the next minute.",
        type: "warning",
        showCancelButton: true,
        confirmButtonText: "Resend",
        confirmButtonColor: "#f0ad4e",
        reverseButtons: true,
        showLoaderOnConfirm: true,
        preConfirm: function() {
            return new Promise(function(resolve) {
                api.campaignId.resendResult(campaign.id, rid)
                    .success(function(msg) { resolve(msg) })
                    .error(function(data) {
                        var msg = (data.responseJSON && data.responseJSON.message) || 'An error occurred';
                        Swal.showValidationMessage(msg);
                        resolve(false);
                    })
            })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire({
                title: 'Queued!',
                text: result.value.message,
                type: 'success',
                timer: 2000,
                showConfirmButton: false
            }).then(function() {
                poll();
            });
        }
    })
}

function buildTimelineData(timeline, results) {
    return timeline.map(event => {
        const event_date = moment.utc(event.time).local();

        let matchingResult = null;

        if (event.email) {
            // Try to find a match based on the event.email field
            matchingResult = results.find(r => {
                // For generic campaigns, match by RId
                if (r.id === event.email) {
                    return true;
                }
                if (r.sms_target) {
                    // For SMS targets, compare with phone
                    return r.phone === event.email;
                } else {
                    // For email targets, compare with email
                    return r.email === event.email;
                }
            });
        }

        // If no match found and this might be an older SMS event with empty email
        if (!matchingResult && !event.email) {
            // Try to find a match based on time and status for SMS targets
            const eventTime = moment.utc(event.time);
            matchingResult = results.find(r => {
                if (!r.sms_target) return false;

                const resultTime = moment.utc(r.modified_date);
                const timeDiff = Math.abs(eventTime.diff(resultTime, 'seconds'));

                return timeDiff < 10 && r.status === event.message;
            });
        }

        if (event.message === "Campaign Created") {
            const firstResult = results[0];
            if (firstResult) {
                sms_target = firstResult.sms_target || false;
                contact = sms_target ? (firstResult.phone || firstResult.email || "N/A") : (firstResult.email || "N/A");
            }
        } else {
            sms_target = matchingResult?.sms_target || false;
            contact = sms_target ? matchingResult?.phone : matchingResult?.email;
        }

        return {
            contact: contact,
            message: event.message,
            x: event_date.valueOf(),
            y: 1,
            sms_target,
            marker: {
                fillColor: statuses[event.message]?.color || '#999'
            }
        };
    });
}

function getBestStatus(result, timeline) {
    const contact = result.sms_target ? result.phone : result.email;
    const rid = result.id; // RId for generic campaigns

    const relevantEvents = timeline.filter(e => {
        // Match by RId (for generic campaigns)
        if (e.email === rid) {
            return true;
        }
        if (result.sms_target) {
            return e.email === result.phone || e.email === result.email;
        }
        return e.email === result.email;
    });

    const statusPriority = {
        "Submitted Data": 6,
        "Clicked Link": 5,
        "Email Replied": 5,
        "Email Opened": 3,
        "Email Sent": 2,
        "SMS Sent": 2,
        "Error": 1
    };

    let best = null;
    let bestRank = 0;
    let bestTime = 0;

    for (const event of relevantEvents) {
        const rank = statusPriority[event.message] || 0;
        const time = new Date(event.time).getTime();
        
        if (
            rank > bestRank ||
            (rank === bestRank && time > bestTime)
        ) {
            best = event.message;
            bestRank = rank;
            bestTime = time;
        }
    }

    return best && statuses[best] ? best : result.replied ? "Email Replied" : result.status;
}


$(document).ready(function () {
    Highcharts.setOptions({
        global: {
            useUTC: false
        }
    })
    load();

    // Start the polling loop
    setRefresh = setTimeout(refresh, 60000)
})
