var campaigns = []
var currentCampaignFilter = 'all' // Global filter state

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
        icon: "fa-envelope",
        point: "ct-point-opened"
    },
    "Attachment Opened": {
        color: "#f0ad4e",
        label: "label-warning",
        icon: "fa-paperclip",
        point: "ct-point-attachment-opened"
    },
    "Email Reported": {
        color: "#45d6ef",
        label: "label-warning",
        icon: "fa-bullhorne",
        point: "ct-point-reported"
    },
    "Email Replied": {
        color: "#E67E22",
        label: "label-info",
        icon: "fa-reply",
        point: "ct-point-replied"
    },
    "Macro Executed": {
        color: "#c0392b",
        label: "label-danger",
        icon: "fa-file-code-o",
        point: "ct-point-macro-executed"
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
    "Campaign Created": {
        label: "label-success",
        icon: "fa-rocket"
    },
    "SMS Sent": {
        color: "#1abc9c",
        label: "label-success",
        icon: "fa-mobile",
        point: "ct-point-sent"
    }
}

var statsMapping = {
    "sent": "Email Sent",
    "opened": "Email Opened",
    "opened_attachment": "Attachment Opened",
    "macro_executed": "Macro Executed",
    "email_reported": "Email Reported",
    "clicked": "Clicked Link",
    "submitted_data": "Submitted Data",
    "replied": "Email Replied",
    "sms_sent": "SMS Sent",
}

function deleteCampaign(idx) {
    if (confirm("Delete " + campaigns[idx].name + "?")) {
        api.campaignId.delete(campaigns[idx].id)
            .success(function (data) {
                successFlash(data.message)
                location.reload()
            })
    }
}

/* Renders a donut ring for a stat card: the colour-filled arc plus the count
   (large) and percentage (small) stacked in the centre. The stat name lives in
   the card header, so the chart itself has no title. */
function renderPieChart(chartopts) {
    var count = chartopts['data'][0].count;
    var pct = Math.round(chartopts['data'][0].y) + '%';
    var color = chartopts['colors'][0];

    function drawCenter(chart) {
        var rend = chart.renderer,
            pie = chart.series[0],
            cx = chart.plotLeft + pie.center[0],
            cy = chart.plotTop + pie.center[1];
        chart.kpiCount = rend.text(count, cx, cy).attr({
            'text-anchor': 'middle',
            'font-size': '20px',
            'font-weight': 'bold',
            'fill': color,
            'font-family': 'Helvetica,Arial,sans-serif'
        }).add();
        chart.kpiPct = rend.text(pct, cx, cy + 16).attr({
            'text-anchor': 'middle',
            'font-size': '12px',
            'fill': color,
            'font-family': 'Helvetica,Arial,sans-serif'
        }).add();
    }

    return Highcharts.chart(chartopts['elemId'], {
        chart: {
            type: 'pie',
            backgroundColor: 'transparent',
            margin: [0, 0, 0, 0],
            spacing: [0, 0, 0, 0],
            events: {
                load: function () { drawCenter(this); },
                render: function () {
                    if (this.kpiCount) { this.kpiCount.attr({ text: count }); }
                    if (this.kpiPct) { this.kpiPct.attr({ text: pct }); }
                }
            }
        },
        title: { text: null },
        plotOptions: {
            pie: {
                innerSize: '78%',
                borderWidth: 0,
                dataLabels: { enabled: false },
                states: { hover: { halo: { size: 0 } } }
            }
        },
        credits: { enabled: false },
        // No tooltip: the card already shows the title, count, percent and
        // "x of N", so there is nothing extra to reveal on hover.
        tooltip: { enabled: false },
        series: [{
            data: chartopts['data'],
            colors: chartopts['colors'],
        }]
    })
}

// The order stages are presented in (funnel and donut cards).
const dashboardStatOrder = [
    "sent", "sms_sent", "opened", "clicked", "submitted_data",
    "opened_attachment", "macro_executed",
    "replied", "email_reported"
];

// computeDashboardStats aggregates the per-campaign stats into the dashboard's
// cross-campaign totals, returning one entry per stage with its count, the
// percentage it represents, and its colour. This is the single source of truth
// for BOTH the donut view and the funnel view, so the two can never disagree.
function computeDashboardStats(campaigns) {
    const statusesToInclude = Object.keys(statsMapping);

    const emailOnlyStats = ["sent", "opened", "opened_attachment", "macro_executed", "replied", "email_reported"];
    const smsOnlyStats = ["sms_sent"];
    const globalStats = ["clicked", "submitted_data"];

    let totals = {
        all: 0,
        email: 0,
        sms: 0,
        generic: 0
    };

    let stats_series_data = {};
    statusesToInclude.forEach(key => stats_series_data[key] = 0);

    // Aggregate stats across all campaigns
    campaigns.forEach(campaign => {
        const isSMS = campaign.type === "sms";
        const isGeneric = campaign.type === "generic";
        const stats = campaign.stats || {};
        const total = stats.total || 0;

        if (isSMS) {
            totals.sms += total;
        } else if (isGeneric) {
            totals.generic += total;
        } else {
            totals.email += total;
        }
        totals.all += total;

        for (let status in stats) {
            let adjustedStatus = status.toLowerCase();

            if (isSMS && adjustedStatus === "sent") {
                adjustedStatus = "sms_sent";
            }

            // Don't count SMS stats toward email-only types
            if (isSMS && emailOnlyStats.includes(adjustedStatus)) continue;
            // Don't count generic campaign stats toward email-only or sms-only types
            if (isGeneric && (emailOnlyStats.includes(adjustedStatus) || smsOnlyStats.includes(adjustedStatus))) continue;
            if (!statusesToInclude.includes(adjustedStatus)) continue;

            stats_series_data[adjustedStatus] += stats[status];
        }
    });

    // Build an ordered list of stage results. The percent math below is
    // identical to the original per-donut calculation, so the numbers shown are
    // unchanged.
    let items = [];
    dashboardStatOrder.forEach(function (statusKey) {
        if (!(statusKey in stats_series_data)) return;
        const count = stats_series_data[statusKey];
        const label = statsMapping[statusKey];

        let denominator = 0;
        if (globalStats.includes(statusKey)) {
            denominator = totals.all;
        } else if (emailOnlyStats.includes(statusKey)) {
            denominator = totals.email;
        } else if (smsOnlyStats.includes(statusKey)) {
            denominator = totals.sms;
        } else {
            return;
        }

        // Real denominator for the "x of N" display; the percent uses a guarded
        // denominator so a type with no campaigns shows 0% instead of dividing
        // by zero.
        var realDen = denominator;
        if (denominator === 0) {
            denominator = 1; // Use 1 to avoid division by zero, count is already 0
        }

        items.push({
            key: statusKey,
            label: label,
            count: count,
            den: realDen,
            percent: (count / denominator) * 100,
            color: (statuses[label] && statuses[label].color) || "#888"
        });
    });

    return { totals: totals, items: items };
}

// Tracks rendered stats charts by container id so a re-render (e.g. toggling
// views) destroys the previous instance instead of stacking a new chart on top
// of it.
var dashboardStatsCharts = {};

function destroyStatsChart(elemId) {
    if (dashboardStatsCharts[elemId]) {
        try { dashboardStatsCharts[elemId].destroy(); } catch (e) { /* ignore */ }
        delete dashboardStatsCharts[elemId];
    }
}

function generateStatsPieCharts(campaigns) {
    const result = computeDashboardStats(campaigns);
    // Only render if there's at least one campaign of any type
    if (result.totals.all <= 0) {
        return;
    }
    result.items.forEach(function (item) {
        const elemId = item.key + '_chart';
        const stats_data = [
            { name: item.label, y: item.percent, count: item.count },
            { name: '', y: 100 - item.percent }
        ];
        $("#" + item.key + "_frac").text(item.count + " of " + item.den);
        try {
            destroyStatsChart(elemId);
            dashboardStatsCharts[elemId] = renderPieChart({
                elemId: elemId,
                name: item.key,
                data: stats_data,
                // Track colour is theme-neutral so the ring reads on light and dark.
                colors: [item.color, "rgba(127,127,127,0.25)"]
            });
        } catch (e) {
            console.error("Error rendering chart for", item.key, e);
        }
    });
}

// generateFunnelChart renders the same stage data as a single horizontal bar
// chart, ordered as a funnel. Each bar is coloured by its stage and labelled
// with the count and percentage.
function generateFunnelChart(campaigns) {
    const result = computeDashboardStats(campaigns);
    if (result.totals.all <= 0) {
        return;
    }
    const categories = result.items.map(function (i) { return i.label; });
    const data = result.items.map(function (i) {
        return {
            y: i.count,
            percent: i.percent,
            color: i.color
        };
    });
    // Read the active theme's colours off the page so all chart text (labels,
    // title, and especially the data-label numbers) stays legible on both light
    // and dark themes instead of Highcharts' fixed dark defaults.
    var textColor = "#333";
    var faintColor = "rgba(127,127,127,0.25)";
    try {
        var probed = window.getComputedStyle(document.body).color;
        if (probed) { textColor = probed; }
    } catch (e) { /* ignore, keep fallback */ }

    // Background tint behind each bar, by severity: green (neutral) for sent/
    // opened/attachment/reported, orange for clicked/replied, red for the
    // compromises (submitted/macro). Coloured per stage (not merged), so the
    // scheme holds regardless of order; adjacent same-colour bars read as one band.
    var BAND_GREEN = "rgba(26,188,156,0.07)";
    var BAND_ORANGE = "rgba(243,156,18,0.09)";
    var BAND_RED = "rgba(192,57,43,0.08)";
    var BAND_BLUE = "rgba(69,214,239,0.10)";
    var bandColorOf = {
        sent: BAND_GREEN, sms_sent: BAND_GREEN, opened: BAND_GREEN,
        opened_attachment: BAND_GREEN,
        clicked: BAND_ORANGE, replied: BAND_ORANGE,
        submitted_data: BAND_RED, macro_executed: BAND_RED,
        email_reported: BAND_BLUE
    };
    var plotBands = [];
    for (var bi = 0; bi < result.items.length; bi++) {
        var bc = bandColorOf[result.items[bi].key];
        if (bc) {
            plotBands.push({ from: bi - 0.5, to: bi + 0.5, color: bc });
        }
    }

    // Icons next to each funnel stage, matching the campaign-set / donut-card
    // glyphs. Keyed by stage so the axis label (the stage name) can look them up.
    var statIcons = {
        sent: "fa-envelope-o", sms_sent: "fa-comment", opened: "fa-envelope-open-o",
        opened_attachment: "fa-paperclip", macro_executed: "fa-file-code-o",
        clicked: "fa-mouse-pointer", submitted_data: "fa-exclamation-circle",
        replied: "fa-reply", email_reported: "fa-bullhorn"
    };
    var labelMeta = {};
    result.items.forEach(function (i) {
        labelMeta[i.label] = { icon: statIcons[i.key], color: i.color };
    });

    try {
        destroyStatsChart('funnel_chart');
        dashboardStatsCharts['funnel_chart'] = Highcharts.chart('funnel_chart', {
            chart: { type: 'bar', backgroundColor: 'transparent' },
            title: { text: 'Engagement Funnel', style: { color: textColor } },
            xAxis: {
                categories: categories,
                title: { text: null },
                lineColor: faintColor,
                tickColor: faintColor,
                labels: {
                    useHTML: true,
                    style: { color: textColor },
                    formatter: function () {
                        var m = labelMeta[this.value] || {};
                        var ic = m.icon
                            ? '<i class="fa ' + m.icon + '" style="margin-right:6px;color:' + (m.color || textColor) + '"></i>'
                            : '';
                        return '<span style="color:' + textColor + '">' + ic + this.value + '</span>';
                    }
                },
                plotBands: plotBands
            },
            yAxis: {
                min: 0,
                title: { text: 'Recipients', align: 'high', style: { color: textColor } },
                allowDecimals: false,
                gridLineColor: faintColor,
                labels: { style: { color: textColor } }
            },
            legend: { enabled: false },
            credits: { enabled: false },
            tooltip: {
                formatter: function () {
                    return '<b>' + this.key + '</b><br/>' +
                        this.y + ' (' + Math.round(this.point.percent) + '%)';
                }
            },
            plotOptions: {
                bar: {
                    dataLabels: {
                        enabled: true,
                        inside: false,
                        style: { color: textColor, textOutline: 'none', fontWeight: 'normal' },
                        formatter: function () {
                            return this.y + ' (' + Math.round(this.point.percent) + '%)';
                        }
                    }
                }
            },
            series: [{
                name: 'Recipients',
                data: data,
                colorByPoint: true
            }]
        });
    } catch (e) {
        console.error("Error rendering funnel chart", e);
    }
}

// Dashboard stats view toggle (funnel vs donuts), remembered per browser.
// Reading/writing localStorage is wrapped so a blocked or private store never
// breaks the dashboard.
var DASHBOARD_VIEW_KEY = 'gophish.dashboard.statsView';

function getDashboardView() {
    try {
        var v = localStorage.getItem(DASHBOARD_VIEW_KEY);
        if (v === 'donuts' || v === 'funnel') {
            return v;
        }
    } catch (e) { /* ignore */ }
    return 'funnel'; // default
}

function setDashboardView(view, campaigns) {
    try {
        localStorage.setItem(DASHBOARD_VIEW_KEY, view);
    } catch (e) { /* ignore */ }

    if (view === 'donuts') {
        $("#funnel_view").hide();
        $("#donuts_view").show();
        $("#view-donuts").addClass('active').siblings().removeClass('active');
        generateStatsPieCharts(campaigns);
    } else {
        $("#donuts_view").hide();
        $("#funnel_view").show();
        $("#view-funnel").addClass('active').siblings().removeClass('active');
        generateFunnelChart(campaigns);
    }
    // Highcharts needs a reflow after its container becomes visible, otherwise
    // it renders at the width it had while hidden (often zero).
    setTimeout(function () {
        $.each(Highcharts.charts, function (i, c) {
            if (c) { c.reflow(); }
        });
    }, 0);
}

// generateKPIs fills the dashboard summary tiles. Rates use the total number of
// recipients across all campaigns as the denominator (the standard phishing
// metric), so each rate reads as "did X / everyone targeted".
function generateKPIs(campaigns) {
    var t = { total: 0, sent: 0, opened: 0, clicked: 0, submitted: 0, reported: 0, replied: 0, macro: 0 };
    campaigns.forEach(function (c) {
        var s = c.stats || {};
        t.total += s.total || 0;
        t.sent += s.sent || 0;
        t.opened += s.opened || 0;
        t.clicked += s.clicked || 0;
        t.submitted += s.submitted_data || 0;
        t.reported += s.email_reported || 0;
        t.replied += s.replied || 0;
        t.macro += s.macro_executed || 0;
    });
    var N = t.total || 0;
    function pct(x) { return N > 0 ? Math.round((x / N) * 100) : 0; }
    function frac(x) { return x + "/" + N; }

    $("#kpi_delivered").text(t.sent);

    $("#kpi_open_pct").text(pct(t.opened) + "%");
    $("#kpi_open_frac").text(frac(t.opened));
    $("#kpi_click_pct").text(pct(t.clicked) + "%");
    $("#kpi_click_frac").text(frac(t.clicked));
    $("#kpi_submit_pct").text(pct(t.submitted) + "%");
    $("#kpi_submit_frac").text(frac(t.submitted));
    $("#kpi_reply_pct").text(pct(t.replied) + "%");
    $("#kpi_reply_frac").text(frac(t.replied));
    $("#kpi_report_pct").text(pct(t.reported) + "%");
    $("#kpi_report_frac").text(frac(t.reported));
}

function generateTimelineChart(campaigns) {
    var overview_data = []
    $.each(campaigns, function (i, campaign) {
        var campaign_date = moment.utc(campaign.created_date).local()
        // Add it to the chart data
        campaign.y = 0
        // Count all success indicators: clicked, submitted data, and replied
        campaign.y += (campaign.stats.clicked || 0)
        campaign.y += (campaign.stats.submitted_data || 0)
        campaign.y += (campaign.stats.replied || 0)
        // Calculate percentage and cap at 100% (since users can trigger multiple events)
        campaign.y = Math.min(100, Math.floor((campaign.y / campaign.stats.total) * 100))
        // Add the data to the overview chart
        overview_data.push({
            campaign_id: campaign.id,
            name: campaign.name,
            x: campaign_date.valueOf(),
            y: campaign.y
        })
    })
    Highcharts.chart('overview_chart', {
        chart: {
            zoomType: 'x',
            type: 'areaspline'
        },
        title: {
            text: 'Phishing Success Overview'
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
            max: 100,
            title: {
                text: "% of Success"
            }
        },
        tooltip: {
            formatter: function () {
                return Highcharts.dateFormat('%A, %b %d %l:%M:%S %P', new Date(this.x)) +
                    '<br>' + this.point.name + '<br>% Success: <b>' + this.y + '%</b>'
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
                point: {
                    events: {
                        click: function (e) {
                            window.location.href = "/campaigns/" + this.campaign_id
                        }
                    }
                }
            }
        },
        credits: {
            enabled: false
        },
        series: [{
            data: overview_data,
            color: "#f05b4f",
            fillOpacity: 0.5
        }]
    })
}

$(document).ready(function () {
    Highcharts.setOptions({
        global: {
            useUTC: false
        }
    })
    api.campaigns.summary()
        .success(function (data) {
            $("#loading").hide()
            campaigns = data.campaigns
            if (campaigns.length > 0) {
                $("#dashboard").show()
                // Create the overview chart data
                campaignTable = $("#campaignTable").DataTable({
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }],
                    order: [
                        [1, "desc"]
                    ]
                });

                // DataTables keeps only the current page's rows in the DOM, so
                // tooltips must be (re)initialised on every draw (pagination/sort/
                // search) — otherwise later-page rows fall back to the native
                // title tooltip. Idempotent: Bootstrap keeps existing instances.
                campaignTable.on('draw', function () {
                    $('#campaignTable').find('[data-toggle="tooltip"]').tooltip();
                });

                campaignRows = []
                $.each(campaigns, function (i, campaign) {
                    var campaign_date = moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a')
                    var statusObj = statuses[campaign.status] || {};
                    var label = statusObj.label || "label-default";
                    //section for tooltips on the status of a campaign to show some quick stats
                    var launchDate;
                    
                    // Determine campaign types
                    var isSMSCampaign = campaign.type === "sms";
                    var isGenericCampaign = campaign.type === "generic";
                    
                    if (moment(campaign.launch_date).isAfter(moment())) {
                        launchDate = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + campaign.stats.total
                    } else {
                        launchDate = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        
                        // Status tooltip is kept to schedule/size/delivery only;
                        // the per-stage engagement counts live in the dots tooltip.
                        var quickStats;
                        if (isGenericCampaign) {
                            quickStats = launchDate + "<br>" +
                                "Number of links: " + (campaign.stats.total || 0);
                        } else {
                            quickStats = launchDate + "<br>" +
                                "Number of recipients: " + (campaign.stats.total || 0) + "<br>" +
                                "Errors: " + (campaign.stats.error || 0);
                        }
                    }
                    // Determine campaign type icon
                    var campaignTypeIcon;
                    if (isGenericCampaign) {
                        campaignTypeIcon = '<i class="fa fa-link" title="Generic Link Campaign" style="margin-right: 5px;"></i>';
                    } else if (isSMSCampaign) {
                        campaignTypeIcon = '<i class="fa fa-mobile" title="SMS Campaign" style="font-size: 1.2em; margin-right: 5px;"></i>';
                    } else {
                        campaignTypeIcon = '<i class="fa fa-envelope" title="Email Campaign" style="margin-right: 5px;"></i>';
                    }
                    
                    // For generic and SMS campaigns, show "-" for email-specific columns
                    var sentValue = isGenericCampaign ? "-" : (campaign.stats.sent || 0);
                    var openedValue = (isGenericCampaign || isSMSCampaign) ? "-" : campaign.stats.opened;
                    var repliedValue = (isGenericCampaign || isSMSCampaign) ? "-" : (campaign.stats.replied || 0);
                    var reportedValue = (isGenericCampaign || isSMSCampaign) ? "-" : campaign.stats.email_reported;
                    var attachmentValue = (isGenericCampaign || isSMSCampaign) ? "-" : (campaign.stats.opened_attachment || 0);
                    var macroValue = (isGenericCampaign || isSMSCampaign) ? "-" : (campaign.stats.macro_executed || 0);

                    // One dot per stage (order follows stageDefs below): filled +
                    // coloured when the stage happened, hollow when not, dotted
                    // when N/A for the campaign type. Exact counts are in the
                    // cell's hover title.
                    var stageDefs = [
                        { v: sentValue, hex: "#1abc9c", label: "Sent" },
                        { v: openedValue, hex: "#f9bf3b", label: "Opened" },
                        { v: campaign.stats.clicked, hex: "#F39C12", label: "Clicked" },
                        { v: campaign.stats.submitted_data, hex: "#f05b4f", label: "Submitted" },
                        { v: attachmentValue, hex: "#f0ad4e", label: "Attachment Opened" },
                        { v: macroValue, hex: "#c0392b", label: "Macro Executed" },
                        { v: repliedValue, hex: "#E67E22", label: "Replied" },
                        { v: reportedValue, hex: "#45d6ef", label: "Reported" }
                    ];
                    var dots = "", tip = [];
                    $.each(stageDefs, function (di, s) {
                        var dot;
                        if (s.v === "-") {
                            dot = '<span class="stage-dot stage-dot-na"></span>';
                            tip.push(s.label + ": n/a");
                        } else {
                            var n = s.v || 0;
                            if (n > 0) {
                                dot = '<span class="stage-dot" style="background:' + s.hex + ';border-color:' + s.hex + '"></span>';
                            } else {
                                dot = '<span class="stage-dot stage-dot-empty"></span>';
                            }
                            tip.push(s.label + ": " + n);
                        }
                        // Each dot sits in a fixed-width cell so it lines up under
                        // the matching header icon.
                        dots += '<span class="stage-cell">' + dot + '</span>';
                    });
                    var stagesCell = '<span class="stage-dots" data-toggle="tooltip" data-html="true" data-placement="top" title="' + tip.join("<br>") + '">' + dots + '</span>';

                    // Add it to the list
                    campaignRows.push([
                        campaignTypeIcon + escapeHtml(campaign.name),
                        campaign_date,
                        stagesCell,
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        "<div class='pull-right'><a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>\
                    <i class='fa fa-bar-chart'></i>\
                    </a>\
                    <button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                $('[data-toggle="tooltip"]').tooltip()
                campaignTable.rows.add(campaignRows).draw()
                
                // Add campaign type as a data attribute to each row for filtering
                campaignTable.rows().every(function(rowIdx) {
                    var campaign = campaigns[rowIdx];
                    $(this.node()).attr('data-campaign-type', campaign.type || 'email');
                });
                
                // Register persistent filter function
                $.fn.dataTable.ext.search.push(
                    function(settings, data, dataIndex) {
                        // If showing all campaigns, don't filter
                        if (currentCampaignFilter === 'all') {
                            return true;
                        }
                        
                        // Get the campaign type for this row
                        var rowNode = campaignTable.row(dataIndex).node();
                        var campaignType = $(rowNode).attr('data-campaign-type') || 'email';
                        
                        // Apply filter based on current filter state
                        if (currentCampaignFilter === 'email') {
                            return campaignType !== 'sms' && campaignType !== 'generic';
                        } else if (currentCampaignFilter === 'sms') {
                            return campaignType === 'sms';
                        } else if (currentCampaignFilter === 'generic') {
                            return campaignType === 'generic';
                        }
                        
                        return true;
                    }
                );
                
                // Setup filter buttons with persistent filtering
                $("#filter-all").on("click", function() {
                    $(this).addClass('active').siblings().removeClass('active');
                    currentCampaignFilter = 'all';
                    campaignTable.draw();
                });
                
                $("#filter-email").on("click", function() {
                    $(this).addClass('active').siblings().removeClass('active');
                    currentCampaignFilter = 'email';
                    campaignTable.draw();
                });
                
                $("#filter-sms").on("click", function() {
                    $(this).addClass('active').siblings().removeClass('active');
                    currentCampaignFilter = 'sms';
                    campaignTable.draw();
                });
                
                $("#filter-generic").on("click", function() {
                    $(this).addClass('active').siblings().removeClass('active');
                    currentCampaignFilter = 'generic';
                    campaignTable.draw();
                });
                
                // Build the summary KPIs and charts. The stats charts are drawn
                // by the view toggle below (it renders only the active view).
                generateKPIs(campaigns)
                generateTimelineChart(campaigns)

                $("#view-funnel").on("click", function () {
                    setDashboardView('funnel', campaigns);
                });
                $("#view-donuts").on("click", function () {
                    setDashboardView('donuts', campaigns);
                });
                // Render the remembered (or default) view.
                setDashboardView(getDashboardView(), campaigns);
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching campaigns")
        })
})
