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

/* Renders a pie chart using the provided chartops */
function renderPieChart(chartopts) {
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
                        'font-size': '16px',
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

function generateStatsPieCharts(campaigns) {

    const statusesToInclude = Object.keys(statsMapping);

    const emailOnlyStats = ["sent", "opened", "replied", "email_reported"];
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

    // Render donut charts
    // Only render if there's at least one campaign of any type
    if (totals.all > 0) {
        for (let statusKey in stats_series_data) {
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
                continue;
            }

            // If there are no campaigns of this specific type, use a small denominator to show 0%
            // This ensures the donut still renders at 0% rather than being hidden
            if (denominator === 0) {
                denominator = 1; // Use 1 to avoid division by zero, count is already 0
            }

            const percent = (count / denominator) * 100;
            const chartElemId = statusKey + '_chart';

            const stats_data = [
                { name: label, y: percent, count: count },
                { name: '', y: 100 - percent }
            ];

            try {
                renderPieChart({
                    elemId: chartElemId,
                    title: label,
                    name: statusKey,
                    data: stats_data,
                    colors: [statuses[label].color || "#888", "#dddddd"]
                });
            } catch (e) {
                console.error("Error rendering chart for", statusKey, e);
            }
        }
    }
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
                        },
                        {
                            className: "color-sent",
                            targets: [2]
                        },
                        {
                            className: "color-opened",
                            targets: [3]
                        },
                        {
                            className: "color-clicked",
                            targets: [4]
                        },
                        {
                            className: "color-success",
                            targets: [5]
                        },
                        {
                            className: "color-replied",
                            targets: [6]
                        },
                        {
                            className: "color-reported",
                            targets: [7]
                        }
                    ],
                    order: [
                        [1, "desc"]
                    ]
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
                        
                        var quickStats;
                        if (isGenericCampaign) {
                            // Generic campaign stats
                            quickStats = launchDate + "\n" + 
                                "Number of links: " + (campaign.stats.total || 0) + "\n" + 
                                "Clicks: " + (campaign.stats.clicked || 0) + "\n" + 
                                "Submitted Credentials: " + (campaign.stats.submitted_data || 0);
                        } else {
                            var sentLabel = isSMSCampaign ? "SMS sent: " : "Emails sent: ";
                            
                            quickStats = launchDate + "\n" + 
                                "Number of recipients: " + (campaign.stats.total || 0) + "\n" + 
                                sentLabel + (campaign.stats.sent || 0);

                            // Email-only fields
                            if (!isSMSCampaign) {
                                quickStats += "\n" + 
                                    "Emails opened: " + (campaign.stats.opened || 0) + "\n" + 
                                    "Emails replied: " + (campaign.stats.replied || 0) + "\n" + 
                                    "Reported: " + (campaign.stats.email_reported || 0);
                            }

                            // Shared fields (email + SMS)
                            quickStats += "\n" + 
                                "Clicks: " + (campaign.stats.clicked || 0) + "\n" + 
                                "Submitted Credentials: " + (campaign.stats.submitted_data || 0) + "\n" + 
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
                    
                    // Add it to the list
                    campaignRows.push([
                        campaignTypeIcon + escapeHtml(campaign.name),
                        campaign_date,
                        sentValue,
                        openedValue,
                        campaign.stats.clicked,
                        campaign.stats.submitted_data,
                        repliedValue,
                        reportedValue,
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
                
                // Build the charts
                generateStatsPieCharts(campaigns)
                generateTimelineChart(campaigns)
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching campaigns")
        })
})
