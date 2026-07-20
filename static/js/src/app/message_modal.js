// Shared viewer for message content.
//
// SECURITY: message HTML is attacker-controlled. It is rendered only inside an
// iframe with sandbox="" — an EMPTY sandbox attribute. Never add allow-scripts
// or allow-same-origin: together they let the framed document reach its own
// frame element and remove the sandbox, which is equivalent to no sandbox at
// all. A CSP inside the srcdoc is a second, independent layer so that a mistake
// in one does not expose the admin session.

var currentMessage = null;
var imagesEnabled = false;

function escapeHtml(value) {
    return String(value == null ? '' : value)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}

// Strip href so a triaging admin cannot click through to the live phishing
// site. The destination stays visible via the title attribute.
//
// DOMParser is used rather than a regex: it handles malformed and deliberately
// crafted markup that a pattern would let slip through, and it normalizes the
// document on the way out. Parsing with DOMParser does NOT execute scripts —
// the resulting document is inert and never attached to this page.
function neutralizeLinks(html) {
    var doc = new DOMParser().parseFromString(html, 'text/html');
    var anchors = doc.getElementsByTagName('a');
    for (var i = 0; i < anchors.length; i++) {
        var url = anchors[i].getAttribute('href') || '';
        anchors[i].removeAttribute('href');
        anchors[i].setAttribute('title', url);
        anchors[i].setAttribute('data-blocked-href', url);
    }
    return doc.body ? doc.body.innerHTML : '';
}

function buildSrcdoc(html) {
    var imgSrc = imagesEnabled ? "img-src http: https: data:;" : "img-src 'none';";
    var csp = "default-src 'none'; style-src 'unsafe-inline'; " + imgSrc;
    return '<!doctype html><html><head>' +
        '<meta http-equiv="Content-Security-Policy" content="' + csp + '">' +
        '</head><body style="font-family:sans-serif;margin:8px;">' +
        neutralizeLinks(html || '<em>No HTML content.</em>') +
        '</body></html>';
}

function renderMessage(msg) {
    currentMessage = msg;

    $('#messageModalSubject').text(msg.subject || '(no subject)');
    $('#messageModalFrom').text(msg.from || '');
    $('#messageModalDate').text(msg.date || '');

    $('#messageTabText').text(msg.text || '(No plain text part.)');

    var headerRows = '';
    var headers = msg.headers || [];
    for (var i = 0; i < headers.length; i++) {
        headerRows += '<tr><td class="message-header-name">' + escapeHtml(headers[i].name) +
            '</td><td>' + escapeHtml(headers[i].value) + '</td></tr>';
    }
    $('#messageTabHeaders').html(headerRows || '<tr><td colspan="2">No headers.</td></tr>');

    if (msg.html) {
        $('#messageHtmlUnavailable').hide();
        $('#messageHtmlFrame').show();
        $('#messageImageBar').show();
        document.getElementById('messageHtmlFrame').srcdoc = buildSrcdoc(msg.html);
    } else {
        $('#messageHtmlFrame').hide();
        $('#messageImageBar').hide();
        $('#messageHtmlUnavailable').show();
    }

    // Plain text is the default tab so routine triage never renders
    // attacker HTML at all.
    $('#messageModalTabs a[href="#messageTabTextPane"]').tab('show');
    $('#messageModalLoading').hide();
    $('#messageModalBody').show();
}

function toggleMessageImages() {
    imagesEnabled = !imagesEnabled;
    $('#messageImageToggle').text(imagesEnabled ? 'Block images' : 'Load images');
    $('#messageImageWarning').toggle(!imagesEnabled);
    if (currentMessage && currentMessage.html) {
        document.getElementById('messageHtmlFrame').srcdoc = buildSrcdoc(currentMessage.html);
    }
}
window.toggleMessageImages = toggleMessageImages;

function showMessageModal(source) {
    imagesEnabled = false;
    currentMessage = null;
    $('#messageImageToggle').text('Load images');
    $('#messageImageWarning').show();
    $('#messageModalBody').hide();
    $('#messageModalError').hide();
    $('#messageModalLoading').show();
    $('#messageModal').modal('show');

    if (source.message) {
        renderMessage(source.message);
        return;
    }

    $.ajax({
        url: source.url,
        method: 'GET',
        dataType: 'json',
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    }).done(function (response) {
        renderMessage(response);
    }).fail(function (xhr) {
        var message = 'Could not load this message.';
        if (xhr.responseJSON && xhr.responseJSON.message) {
            message = xhr.responseJSON.message;
        }
        $('#messageModalLoading').hide();
        $('#messageModalError').text(message).show();
    });
}
window.showMessageModal = showMessageModal;
