// People page JS

var peopleTable = null;
var peopleData = [];
// peopleRequest numbers loadPeople calls so a slow, older response can't
// overwrite a newer one.
var peopleRequest = 0;
// peopleLoadedQuery is the filter query string of the rows currently shown,
// so the timeline uses the same filters as the table counts.
var peopleLoadedQuery = "";

function personName(p) {
    return ((p.first_name || "") + " " + (p.last_name || "")).trim();
}

// dateOnly returns YYYY-MM-DD for an RFC3339 string, or "" for empty or the
// Go zero time (meaning "never").
function dateOnly(s) {
    if (!s || s.indexOf("0001-01-01") === 0) {
        return "";
    }
    return s.substring(0, 10);
}

// peopleCsvField quotes a value for CSV and neutralises spreadsheet formula
// injection (leading =, +, -, @, tab, CR) since names, emails and positions
// come from imported target lists. Numbers we generate are passed through.
function peopleCsvField(value) {
    if (typeof value === "number") {
        return String(value);
    }
    var s = (value === null || value === undefined) ? "" : String(value);
    if (/^[=+\-@\t\r]/.test(s)) {
        s = "'" + s;
    }
    if (/[",\n\r]/.test(s)) {
        s = '"' + s.replace(/"/g, '""') + '"';
    }
    return s;
}

function peopleFilterQuery() {
    var params = [];
    var add = function(name, val) {
        if (val) {
            params.push(name + "=" + encodeURIComponent(val));
        }
    };
    add("from", $("#people_from").val());
    add("to", $("#people_to").val());
    add("campaign_set_id", $("#people_campaign_set").val());
    add("group_id", $("#people_group").val());
    return params.join("&");
}

function apiErrorMessage(xhr, fallback) {
    if (xhr && xhr.responseJSON && xhr.responseJSON.message) {
        return xhr.responseJSON.message;
    }
    return fallback;
}

// Risk thresholds on a person's phish-prone %. Kept in step with the Risk
// column tooltip and the "High-risk people" card in people.html.
var HIGH_RISK_PCT = 40;

function riskLevel(pct) {
    if (pct >= HIGH_RISK_PCT) return "high";
    if (pct > 0) return "medium";
    return "low";
}

var riskLabels = { high: "High risk", medium: "Medium risk", low: "Low risk" };

// Avatar colours are picked from a fixed palette by hashing the person key,
// so a person keeps the same colour across reloads.
var avatarColors = ["#c0392b", "#16a085", "#2980b9", "#8e44ad", "#d35400", "#27ae60", "#2c3e50", "#7f8c8d"];

function avatarColor(key) {
    var h = 0;
    for (var i = 0; i < key.length; i++) {
        h = (h * 31 + key.charCodeAt(i)) >>> 0;
    }
    return avatarColors[h % avatarColors.length];
}

function initials(p) {
    var a = (p.first_name || "").trim().charAt(0);
    var b = (p.last_name || "").trim().charAt(0);
    var s = (a + b) || (p.email || p.phone || "?").charAt(0);
    return s.toUpperCase();
}

// renderPeopleStats fills the summary cards from the loaded rows.
function renderPeopleStats(rows) {
    var sent = 0, failures = 0, reports = 0, high = 0;
    rows.forEach(function(r) {
        sent += r.sent || 0;
        failures += r.failures || 0;
        reports += r.reported || 0;
        if ((r.phish_prone_pct || 0) >= HIGH_RISK_PCT) high++;
    });
    var prone = sent ? Math.round(failures * 1000 / sent) / 10 : 0;
    $("#people_stat_total").text(rows.length);
    $("#people_stat_prone").text(prone.toFixed(1) + "%");
    $("#people_stat_high").text(high);
    $("#people_stat_reports").text(reports);
}

// The column renderers below build display HTML from API strings, so every
// one of those strings goes through escapeHtml and none is placed in an
// attribute. Sort and search use the raw values.
function personCell(row, type) {
    var name = personName(row);
    var contact = row.email || row.phone || "";
    if (type !== "display") {
        return [name, contact, row.phone, row.position].join(" ");
    }
    var sub = [contact, row.position].filter(function(v) { return v; }).join(" · ");
    return "<div class=\"people-person\">" +
        "<div class=\"people-avatar\" style=\"background-color:" + avatarColor(row.key || "") + "\">" +
        escapeHtml(initials(row)) + "</div>" +
        "<div><div class=\"people-person-name\">" + escapeHtml(name || contact) + "</div>" +
        "<div class=\"people-person-sub\">" + escapeHtml(sub) + "</div></div></div>";
}

function groupsCell(row, type) {
    var groups = row.groups || [];
    if (type !== "display") {
        return groups.join(" ");
    }
    if (groups.length === 0) {
        return "<span class=\"text-muted\">–</span>";
    }
    return groups.map(function(g) {
        return "<span class=\"people-chip\">" + escapeHtml(g) + "</span>";
    }).join("");
}

function proneCell(row, type) {
    var pct = row.phish_prone_pct || 0;
    if (type !== "display") {
        return pct;
    }
    var level = riskLevel(pct);
    return "<div class=\"people-prone\"><div class=\"people-prone-value\">" + pct.toFixed(1) + "%</div>" +
        "<div class=\"people-bar\"><span class=\"people-bar-" + level + "\" style=\"width:" +
        Math.min(100, Math.max(0, pct)) + "%\"></span></div></div>";
}

function riskCell(row, type) {
    var pct = row.phish_prone_pct || 0;
    if (type !== "display") {
        return pct;
    }
    var level = riskLevel(pct);
    return "<span class=\"people-risk people-risk-" + level + "\">" + riskLabels[level] + "</span>";
}

function lastFailedCell(row, type) {
    var d = dateOnly(row.last_failure);
    if (type !== "display") {
        return d;
    }
    return d ? escapeHtml(moment(d, "YYYY-MM-DD").format("MMM D, YYYY")) : "<span class=\"text-muted\">–</span>";
}

function renderPeople(rows) {
    peopleData = rows || [];
    renderPeopleStats(peopleData);
    if (peopleTable) {
        peopleTable.destroy();
        peopleTable = null;
        $("#peopleTable tbody").empty();
    }
    var num = function(field) {
        return {
            className: "people-num",
            data: function(row) {
                return row[field] || 0;
            }
        };
    };
    peopleTable = $("#peopleTable").DataTable({
        data: peopleData,
        // Table, then info and paging; the page has its own search box.
        dom: "rt<\"people-table-footer\"ip>",
        pageLength: 25,
        autoWidth: false,
        order: [[6, "desc"]],
        language: {
            info: "Showing _START_–_END_ of _TOTAL_ people",
            infoEmpty: "No people",
            infoFiltered: "(filtered from _MAX_)",
            emptyTable: "No one has been sent a campaign that matches these filters.",
            zeroRecords: "No people match your search."
        },
        columns: [
            { data: personCell },
            { data: groupsCell },
            num("sent"), num("clicked"), num("submitted_data"), num("reported"),
            { data: proneCell },
            { data: riskCell },
            { data: lastFailedCell },
            { data: null, orderable: false, searchable: false, className: "people-chevron",
                defaultContent: "<i class=\"fa fa-chevron-right\"></i>" }
        ],
        createdRow: function(tr, row) {
            if (riskLevel(row.phish_prone_pct || 0) === "high") {
                $(tr).addClass("people-row-high");
            }
        }
    });
    var term = $("#people_search").val();
    if (term) {
        peopleTable.search(term).draw();
    }
    $("#peopleTable").show();
}

function loadPeople() {
    var qs = peopleFilterQuery();
    var request = ++peopleRequest;
    $("#people_loading").show();
    query("/people/" + (qs ? "?" + qs : ""), "GET", undefined, true)
        .done(function(rows) {
            if (request !== peopleRequest) return;
            $("#people_loading").hide();
            peopleLoadedQuery = qs;
            renderPeople(rows);
        })
        .fail(function(xhr, status, error) {
            if (request !== peopleRequest) return;
            $("#people_loading").hide();
            errorFlash("Error loading people: " + escapeHtml(apiErrorMessage(xhr, error)));
        });
}

function loadPeopleFilters() {
    // Option labels are set via .text(), never as HTML.
    query("/campaign_sets/summary", "GET", undefined, true)
        .done(function(resp) {
            $.each((resp && resp.campaign_sets) || [], function(i, cs) {
                $("<option>").val(cs.id).text(cs.name).appendTo("#people_campaign_set");
            });
        });
    query("/groups/summary", "GET", undefined, true)
        .done(function(resp) {
            $.each((resp && resp.groups) || [], function(i, g) {
                $("<option>").val(g.id).text(g.name).appendTo("#people_group");
            });
        });
}

var personTable = null;

// timelineStages are the per-campaign actions, in the order of the header
// icon strip. Colours match the dashboard's recent-campaigns stage dots
// (dashboard.js stageDefs) so a stage looks the same everywhere.
var timelineStages = [
    { key: "opened", hex: "#f9bf3b", label: "Opened" },
    { key: "clicked", hex: "#F39C12", label: "Clicked" },
    { key: "submitted_data", hex: "#f05b4f", label: "Submitted data" },
    { key: "attachment_opened", hex: "#f0ad4e", label: "Attachment opened" },
    { key: "macro_executed", hex: "#c0392b", label: "Macro enabled" },
    { key: "replied", hex: "#E67E22", label: "Replied" },
    { key: "reported", hex: "#45d6ef", label: "Reported" }
];

// actionsCell renders one dot per stage: filled in the stage colour when it
// happened, hollow when not. The hover lists what happened. Labels are
// constants, so the HTML tooltip carries no user data.
function actionsCell(f) {
    var dots = $("<span>").addClass("stage-dots");
    var done = [];
    $.each(timelineStages, function(i, s) {
        var dot = $("<span>").addClass("stage-dot");
        if (f[s.key]) {
            dot.css({ background: s.hex, borderColor: s.hex });
            done.push(s.label);
        } else {
            dot.addClass("stage-dot-empty");
        }
        dots.append($("<span>").addClass("stage-cell").append(dot));
    });
    dots.attr({
        "data-toggle": "tooltip",
        "data-container": "body",
        "data-placement": "top",
        "data-html": "true",
        title: done.length ? done.join("<br>") : "No actions"
    });
    return $("<td>").addClass("person-actions").attr("data-order", done.length).append(dots);
}

// outcomeOf returns the outcome tokens for a campaign row, used for both the
// badges and the Outcome filter. Failed and Reported can both apply.
function outcomeOf(r) {
    var f = r.flags || {};
    var out = [];
    if (r.failed) out.push("failed");
    if (f.reported) out.push("reported");
    if (out.length === 0) out.push(f.opened ? "opened" : "ignored");
    return out;
}

var outcomeBadges = {
    failed: { label: "Failed", css: "label-danger", rank: 0 },
    reported: { label: "Reported", css: "people-label-reported", rank: 1 },
    opened: { label: "Opened only", css: "people-label-opened", rank: 2 },
    ignored: { label: "Ignored", css: "label-default", rank: 3 }
};

function outcomeCell(r) {
    var out = outcomeOf(r);
    var td = $("<td>").addClass("person-outcome")
        .attr("data-search", out.join(" "))
        .attr("data-order", outcomeBadges[out[0]].rank);
    $.each(out, function(i, o) {
        td.append($("<span>").addClass("label " + outcomeBadges[o].css).text(outcomeBadges[o].label));
    });
    return td;
}

// renderPersonSummary fills the modal header and cards from the person's
// table row, which was loaded with the same filters as the timeline.
function renderPersonSummary(person) {
    $("#personModalAvatar").text(initials(person)).css("background-color", avatarColor(person.key || ""));
    $("#personModalTitle").text(personName(person) || person.email || person.phone || "");
    var sub = [person.email || person.phone, person.position].filter(function(v) { return v; });
    $("#personModalSub").text(sub.join(" · "));
    // The failure cards are only red when the person has actually failed.
    $("#personStatFailures, #personStatProne").closest(".people-card")
        .toggleClass("people-card-high", (person.failures || 0) > 0);
    $("#personStatSent").text(person.sent || 0);
    $("#personStatFailures").text(person.failures || 0);
    $("#personStatProne").text((person.phish_prone_pct || 0).toFixed(1) + "%");
    $("#personStatReported").text(person.reported || 0);
    $("#personStatReportRate").text((person.report_rate_pct || 0).toFixed(1) + "% report rate");
}

function resetPersonTable() {
    if (personTable) {
        personTable.destroy();
        personTable = null;
    }
    $("#personTimeline tbody [data-toggle=\"tooltip\"]").tooltip("destroy");
    $("#personTimeline tbody").empty();
    $("#personSearch").val("");
    $("#personOutcome").val("");
    $("#personCount").text("");
}

function showTimeline(person) {
    renderPersonSummary(person);
    resetPersonTable();
    $("#personTimeline").hide();
    $("#personModalEmpty").hide();
    $("#personModalLoading").show();
    $("#personModal").modal("show");
    var qs = peopleLoadedQuery;
    var url = "/people/timeline?key=" + encodeURIComponent(person.key) + (qs ? "&" + qs : "");
    query(url, "GET", undefined, true)
        .done(function(rows) {
            $("#personModalLoading").hide();
            rows = rows || [];
            if (rows.length === 0) {
                $("#personModalEmpty").show();
                return;
            }
            // Rows are built with DOM calls (.text/.attr), never HTML strings,
            // since campaign names are user-supplied.
            var tbody = $("#personTimeline tbody");
            $.each(rows, function(i, r) {
                var f = r.flags || {};
                var tr = $("<tr>");
                $("<td>").append(
                    $("<a>").addClass("person-campaign")
                        .attr("href", "/campaigns/" + parseInt(r.campaign_id, 10))
                        .attr("data-toggle", "tooltip")
                        .attr("data-container", "body")
                        .attr("data-placement", "top")
                        .attr("title", r.campaign_name || "")
                        .text(r.campaign_name || "")
                ).appendTo(tr);
                $("<td>").addClass("person-date").text(dateOnly(r.launch_date)).appendTo(tr);
                tr.append(actionsCell(f));
                tr.append(outcomeCell(r));
                tbody.append(tr);
            });
            // Bootstrap tooltips render the title as text, not HTML.
            tbody.find("[data-toggle=\"tooltip\"]").tooltip({ container: "body" });
            // Shown before DataTables measures it, so columns size correctly.
            $("#personTimeline").show();
            personTable = $("#personTimeline").DataTable({
                dom: "rt<\"person-table-footer\"ip>",
                pageLength: 10,
                autoWidth: false,
                order: [[1, "desc"]],
                language: {
                    info: "Showing _START_–_END_ of _TOTAL_ campaigns",
                    infoEmpty: "No campaigns",
                    infoFiltered: "(filtered from _MAX_)",
                    zeroRecords: "No campaigns match."
                }
            });
            personTable.on("draw", updatePersonCount);
            updatePersonCount();
        })
        .fail(function(xhr, status, error) {
            $("#personModalLoading").hide();
            $("#personModal").modal("hide");
            errorFlash("Error loading timeline: " + escapeHtml(apiErrorMessage(xhr, error)));
        });
}

function updatePersonCount() {
    if (!personTable) return;
    var n = personTable.page.info().recordsDisplay;
    $("#personCount").text(n + (n === 1 ? " campaign" : " campaigns"));
}

// filterPersonTable applies the popup's campaign search and Outcome filter.
// The Outcome column searches its data-search tokens as whole words, so
// "opened" doesn't match a row's hidden text by accident.
function filterPersonTable() {
    if (!personTable) return;
    var outcome = $("#personOutcome").val();
    personTable.search($("#personSearch").val());
    personTable.column(3).search(outcome ? "\\b" + outcome + "\\b" : "", true, false);
    personTable.draw();
}

// exportPeopleCSV builds the CSV in the browser from all rows loaded for the
// current filters (not just the visible DataTable page).
function exportPeopleCSV() {
    var rows = peopleData || [];
    if (rows.length === 0) {
        errorFlash("No people to export");
        return;
    }
    var header = ["email", "phone", "first_name", "last_name", "position", "groups", "sent", "opened",
        "clicked", "attachment_opened", "macro_executed", "submitted_data", "replied",
        "reported", "failures", "phish_prone_pct", "report_rate_pct", "last_failure"];
    var lines = [header.join(",")];
    rows.forEach(function(r) {
        lines.push([
            peopleCsvField(r.email),
            peopleCsvField(r.phone),
            peopleCsvField(r.first_name),
            peopleCsvField(r.last_name),
            peopleCsvField(r.position),
            peopleCsvField((r.groups || []).join("; ")),
            peopleCsvField(r.sent || 0),
            peopleCsvField(r.opened || 0),
            peopleCsvField(r.clicked || 0),
            peopleCsvField(r.attachment_opened || 0),
            peopleCsvField(r.macro_executed || 0),
            peopleCsvField(r.submitted_data || 0),
            peopleCsvField(r.replied || 0),
            peopleCsvField(r.reported || 0),
            peopleCsvField(r.failures || 0),
            peopleCsvField(r.phish_prone_pct || 0),
            peopleCsvField(r.report_rate_pct || 0),
            peopleCsvField(dateOnly(r.last_failure))
        ].join(","));
    });
    var blob = new Blob([lines.join("\r\n")], { type: "text/csv;charset=utf-8;" });
    var filename = "people-" + new Date().toISOString().substring(0, 10) + ".csv";
    if (navigator.msSaveBlob) {
        navigator.msSaveBlob(blob, filename);
        return;
    }
    var url = window.URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url;
    a.setAttribute("download", filename);
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
}

$(document).ready(function() {
    // The page's static tooltips (cards, column headers) are already set up
    // by gophish.js, which runs first and passes no options, so each element
    // carries data-container="body" in the template. Without it a tooltip on
    // a <th> is inserted into the header row as an extra cell and shifts the
    // columns.
    // Hide any open tooltip when the person modal closes.
    $("#personModal").on("hide.bs.modal", function() {
        $(this).find("[data-toggle=\"tooltip\"]").tooltip("hide");
    });
    $("#personSearch").on("input", filterPersonTable);
    $("#personOutcome").on("change", filterPersonTable);
    loadPeopleFilters();
    loadPeople();
    $("#people_filters").on("submit", function(e) {
        e.preventDefault();
        loadPeople();
    });
    $("#people_export_btn").on("click", exportPeopleCSV);
    $("#people_search").on("input", function() {
        if (peopleTable) {
            peopleTable.search(this.value).draw();
        }
    });
    $("#people_clear_btn").on("click", function() {
        $("#people_from, #people_to, #people_campaign_set, #people_group, #people_search").val("");
        loadPeople();
    });
    $("#peopleTable tbody").on("click", "tr", function() {
        if (!peopleTable) return;
        var row = peopleTable.row(this).data();
        if (row) {
            showTimeline(row);
        }
    });
});
