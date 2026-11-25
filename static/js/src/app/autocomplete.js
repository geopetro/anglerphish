var TEMPLATE_TAGS = [{
    id: 1,
    group: 'Recipient',
    name: 'FirstName',
    description: 'The recipient\'s first name.'
},
{
    id: 2,
    group: 'Recipient',
    name: 'LastName',
    description: 'The recipient\'s last name.'
},
{
    id: 3,
    group: 'Recipient',
    name: 'Email',
    description: 'The recipient\'s email address.'
},
{
    id: 4,
    group: 'Recipient',
    name: 'Phone',
    description: 'The recipient\'s phone number.'
},
{
    id: 5,
    group: 'Recipient',
    name: 'Position',
    description: 'The recipient\'s position.'
},
{
    id: 6,
    group: 'Recipient',
    name: 'Custom',
    description: 'The recipient\'s custom field.'
},
{
    id: 7,
    group: 'Recipient',
    name: 'RId',
    description: 'The unique ID for the recipient.'
},
{
    id: 8,
    group: 'Links and tracking',
    name: 'URL',
    description: 'The phishing URL for this recipient.'
},
{
    id: 9,
    group: 'Links and tracking',
    name: 'BaseURL',
    description: 'The base URL with the path and rid parameter stripped. Useful for making links to static files.'
},
{
    id: 10,
    group: 'Links and tracking',
    name: 'Tracker',
    description: 'An HTML tag that adds a hidden tracking image (recommended instead of TrackingURL).'
},
{
    id: 11,
    group: 'Links and tracking',
    name: 'TrackingURL',
    description: 'The URL to track emails being opened.'
},
{
    id: 12,
    group: 'Links and tracking',
    name: 'QR',
    description: 'An image tag with a QR code that points to the phishing URL.'
},
{
    id: 13,
    group: 'Sender',
    name: 'From',
    description: 'The address emails are sent from.'
},
{
    id: 14,
    group: 'Date and time',
    name: 'CurrentDateTime',
    description: 'Current date and time, e.g. Nov 23, 2025 7:39 PM.'
},
{
    id: 15,
    group: 'Date and time',
    name: 'CurrentDate',
    description: 'Current date, e.g. November 23, 2025.'
},
{
    id: 16,
    group: 'Date and time',
    name: 'CurrentTime',
    description: 'Current time (12-hour), e.g. 7:39 PM.'
},
{
    id: 17,
    group: 'Date and time',
    name: 'CurrentTime24',
    description: 'Current time (24-hour), e.g. 19:39.'
}
];

var textTestCallback = function (range) {
if (!range.collapsed) {
    return null;
}

return CKEDITOR.plugins.textMatch.match(range, matchCallback);
}

var matchCallback = function (text, offset) {
var pattern = /\{{2}\.?([A-Za-z0-9]|\})*$/,
    match = text.slice(0, offset)
    .match(pattern);

if (!match) {
    return null;
}

return {
    start: match.index,
    end: offset
};
}

/**
* 
* @param {regex} matchInfo - The matched text object
* @param {function} callback - The callback to execute with the matched data
*/
var dataCallback = function (matchInfo, callback) {
var data = TEMPLATE_TAGS.filter(function (item) {
    var itemName = '{{.' + item.name.toLowerCase() + '}}';
    return itemName.indexOf(matchInfo.query.toLowerCase()) == 0;
});

callback(data);
}

/**
* 
* @param {CKEditor} editor - The CKEditor instance.
* 
* Installs the autocomplete plugin to the CKEditor.
*/
var setupAutocomplete = function (editor) {
editor.on('instanceReady', function (evt) {
    var itemTemplate = '<li data-id="{id}">' +
        '<div><strong class="item-title">{name}</strong></div>' +
        '<div><i>{description}</i></div>' +
        '</li>',
        outputTemplate = '[[.{name}]]';

    var autocomplete = new CKEDITOR.plugins.autocomplete(evt.editor, {
        textTestCallback: textTestCallback,
        dataCallback: dataCallback,
        itemTemplate: itemTemplate,
        outputTemplate: outputTemplate
    });

    // We have to use brackets for the output template tag and 
    // then manually replace them due to the way CKEditor's 
    // templating works.
    autocomplete.getHtmlToInsert = function (item) {
        var parsedTemplate = this.outputTemplate.output(item);
        parsedTemplate = parsedTemplate.replace("[[", "{{").replace("]]", "}}")
        return parsedTemplate
    }
});
}