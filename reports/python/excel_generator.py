#!/usr/bin/env python
"""
Excel document generator for Anglerphish reports.
"""

import sys

try:
    import openpyxl
    from openpyxl.styles import Font, PatternFill, Alignment
    from openpyxl.utils import get_column_letter
except ImportError:
    sys.stderr.write("Error: openpyxl library is required. Install with: pip install openpyxl\n")
    sys.exit(1)

from report_common import (
    action_color, mfa_summary,
    TIME_BINS, Privacy, campaign_metrics, campaign_status_display, clean_text,
    compute_metrics, describe_browser, describe_os, event_browser,
    extract_ip_address,
    format_date, generated_on, interaction_stats, is_generic_campaign,
    is_sms_campaign, locked_events_notice, payload_text, pct,
    pct_in_bin, preprocess,
)

# Color definitions for Excel
COLORS = {
    'header_blue': '4472C4',  # Header blue
    'sent_green': '1ABC9C',   # Green for sent emails
    'light_blue': '45D6EF',   # Light blue for reporting
    'amber': 'F9BF3B',        # Amber for opened
    'orange': 'F39C12',       # Orange for clicked
    'red': 'F05B4F',          # Red for data submitted
    'dark_orange': 'E67E22',  # Dark orange for replies
    'light_grey': 'D9D9D9',   # Light grey for alternating rows
    'dark_grey': 'BFBFBF',    # Dark grey for alternating rows
    'white': 'FFFFFF',        # White for text
}

def sanitize_sheet_name(name, existing=()):
    """Make a valid, unique Excel worksheet name.

    Excel names can't contain [ ] : * ? / \\, can't start or end with an
    apostrophe and are limited to 31 characters."""
    name = (name or '').replace('[', '(').replace(']', ')')
    for char in (':', '*', '?', '/', '\\'):
        name = name.replace(char, '')
    name = clean_text(name).strip().strip("'").strip() or "Sheet"
    name = name[:31]
    taken = {n.lower() for n in existing}
    candidate, n = name, 2
    while candidate.lower() in taken:
        suffix = f" ({n})"
        candidate = name[:31 - len(suffix)] + suffix
        n += 1
    return candidate


def neutralize_formulas(workbook):
    """Store every cell as plain data, never as a formula.

    Report cells contain values that phishing targets (or anyone posting to
    a landing page) control. openpyxl turns any string starting with '='
    into a formula, which would run when the report is opened."""
    for ws in workbook.worksheets:
        for row in ws.iter_rows():
            for cell in row:
                if cell.data_type == 'f':
                    cell.data_type = 's'


def create_cell_style(fill_color=None, font_color=None, bold=False, alignment=None, border=None):
    """Create a dict of styles for Excel cells"""
    style = {}

    if fill_color:
        style['fill'] = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")

    font_opts = {}
    if font_color:
        font_opts['color'] = font_color
    if bold:
        font_opts['bold'] = True
    if font_opts:
        style['font'] = Font(**font_opts)

    if alignment:
        style['alignment'] = alignment

    if border:
        style['border'] = border

    return style

def apply_cell_style(cell, style_dict):
    """Apply a style dict to a cell"""
    for key, value in style_dict.items():
        setattr(cell, key, value)

def style_header_row(worksheet, row_num, start_col, end_col, fill_color=COLORS['header_blue'], text_color=COLORS['white']):
    """Style a header row with the specified colors"""
    for col in range(start_col, end_col + 1):
        cell = worksheet.cell(row=row_num, column=col)
        style = create_cell_style(
            fill_color=fill_color,
            font_color=text_color,
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center', wrap_text=True)
        )
        apply_cell_style(cell, style)

def style_table(worksheet, start_row, end_row, start_col, end_col,
                header_fill=COLORS['header_blue'],
                row_fill=COLORS['light_grey'],
                alt_row_fill=COLORS['dark_grey'],
                header_font=COLORS['white']):
    """Style a table region in the worksheet with alternating row colors"""
    # Style header row
    style_header_row(worksheet, start_row, start_col, end_col, header_fill, header_font)

    # Style data rows with alternating colors
    for row in range(start_row + 1, end_row + 1):
        fill_color = row_fill if (row - start_row) % 2 == 1 else alt_row_fill
        for col in range(start_col, end_col + 1):
            cell = worksheet.cell(row=row, column=col)
            style = create_cell_style(
                fill_color=fill_color,
                alignment=Alignment(vertical='center', wrap_text=True)
            )
            apply_cell_style(cell, style)

def auto_adjust_column_width(worksheet):
    """Automatically adjust column widths to fit content"""
    dims = {}
    for row in worksheet.rows:
        for cell in row:
            if cell.value:
                # Get the cell value's length, minimum width is 10
                dims[cell.column_letter] = max((dims.get(cell.column_letter, 0), len(str(cell.value)) + 2))

    # Set column widths, limiting to reasonable values
    for col, width in dims.items():
        width = min(max(width, 10), 50)  # Min 10, max 50 characters
        worksheet.column_dimensions[col].width = width

def generate_excel_document(data, output_path, gdpr_options=None):
    """
    Generate an Excel (.xlsx) report from Anglerphish campaign data.

    Args:
        data: Dictionary containing campaign data
        output_path: Path to save the Excel file
        gdpr_options: Dictionary with GDPR compliance options

    Returns:
        Boolean indicating success
    """
    if not isinstance(data, dict) or not data.get('campaigns'):
        sys.stderr.write("Error: Invalid or empty campaign data\n")
        return False

    privacy = Privacy(gdpr_options)
    preprocess(data)
    metrics = compute_metrics(data)
    campaigns = data['campaigns']

    # Create new Excel workbook
    workbook = openpyxl.Workbook()

    # Create worksheets: Summary, Per Campaign Unique Results, Per Campaign Results (cumulative),
    # Campaign Overview, Campaign Results, Per-Campaign Event sheets (created later), Browser Statistics, OS Statistics, IP Statistics
    summary_sheet = workbook.active
    summary_sheet.title = "Summary"
    overview_sheet = workbook.create_sheet("Campaign Information")
    scenario_sheet = workbook.create_sheet("Per Campaign Unique Results")
    cumulative_sheet = workbook.create_sheet("Per Campaign Results")
    results_sheet = workbook.create_sheet("Campaign Results")
    browser_sheet = workbook.create_sheet("Browser Statistics")
    os_sheet = workbook.create_sheet("OS Statistics")
    ip_sheet = workbook.create_sheet("IP Statistics")

    # -----------------------------------------
    # Summary Worksheet
    # -----------------------------------------
    summary_sheet['A1'] = "Anglerphish Campaign Report"
    summary_sheet['A1'].font = Font(size=16, bold=True)
    summary_sheet.merge_cells('A1:G1')
    summary_sheet['A1'].alignment = Alignment(horizontal='center')

    summary_sheet['A2'] = generated_on()
    summary_sheet['A2'].font = Font(italic=True)
    summary_sheet.merge_cells('A2:G2')
    summary_sheet['A2'].alignment = Alignment(horizontal='center')

    # Add privacy notice if anonymization is enabled
    current_row = 3
    if data.get('set_name'):
        summary_sheet['A3'] = clean_text(f"Campaign set: {data['set_name']}")
        summary_sheet.merge_cells('A3:G3')
        summary_sheet['A3'].alignment = Alignment(horizontal='center')
        current_row = 4
    notices = []
    if privacy.enabled:
        masked = []
        if privacy.emails:
            masked.append("email addresses, phone numbers, names and submitted values")
        if privacy.ips:
            masked.append("IP addresses and coordinates (locations are shown at country level)")
        notices.append(("Privacy Notice",
                        "This report has been generated with personal data obfuscated in accordance with "
                        "privacy requirements. The following have been masked: " + "; ".join(masked) + "."))
    locked_notice = locked_events_notice(data)
    if locked_notice:
        notices.append(("Missing Data", locked_notice))
    for heading, text in notices:
        summary_sheet[f'A{current_row}'] = heading
        summary_sheet[f'A{current_row}'].font = Font(bold=True, size=12)
        summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
        current_row += 1
        summary_sheet[f'A{current_row}'] = text
        summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
        summary_sheet[f'A{current_row}'].alignment = Alignment(wrap_text=True)
        summary_sheet.row_dimensions[current_row].height = 40
        notice_fill = PatternFill(start_color="DDEBF7", end_color="DDEBF7", fill_type="solid")
        for cell in summary_sheet[f'A{current_row}:G{current_row}'][0]:
            cell.fill = notice_fill
        current_row += 2

    # Introduction
    summary_sheet[f'A{current_row}'] = "Summary"
    summary_sheet[f'A{current_row}'].font = Font(bold=True, size=14)
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    current_row += 1

    summary_text = "This report provides an analysis of the phishing campaign(s) conducted using the Anglerphish platform, " + \
                  "including performance metrics and key user interactions."
    summary_sheet[f'A{current_row}'] = summary_text
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    summary_sheet[f'A{current_row}'].alignment = Alignment(wrap_text=True)
    summary_sheet.row_dimensions[current_row].height = 40
    current_row += 2

    total = metrics['total']
    in_progress = metrics['in_progress']

    # Overall Campaign Results Table
    summary_sheet[f'A{current_row}'] = "Overall Campaign Results"
    summary_sheet[f'A{current_row}'].font = Font(bold=True, size=12)
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    current_row += 1

    # Add explanatory note about deduplication
    explanation_text = "Unique users across all campaigns. Each user counted once by highest-risk action."
    summary_sheet[f'A{current_row}'] = explanation_text
    summary_sheet[f'A{current_row}'].font = Font(italic=True, size=9)
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    current_row += 1

    # Headers for stats table
    headers = ["Metric", "Count", "Percentage"]
    for col, header in enumerate(headers, 1):
        cell = summary_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    # Create the stats table
    # The first rows are mutually exclusive and add up to the total;
    # replied/reported are separate behaviours and can overlap with them.
    def summary_row(label, key_or_count):
        count = metrics[key_or_count] if isinstance(key_or_count, str) else key_or_count
        return (label, count, f"{pct(count, total)}%", action_color(label))

    # Each person counted once, by highest action (ranked as the app ranks
    # a result's status); these rows add up to the total. Replied and
    # reported are separate behaviours and can overlap with them.
    stats_data = [("People", total, "", COLORS['header_blue']),
                  summary_row("Data Submitted", 'submitted')]
    if metrics['has_attachments']:
        stats_data.append(summary_row("Macro Executed", 'macro'))
    stats_data.append(summary_row("Clicked Link", 'clicked'))
    if metrics['has_attachments']:
        stats_data.append(summary_row("Attachment Opened", 'attachment'))
    if metrics['has_email']:
        stats_data.append(summary_row("Email Opened", 'opened'))
    stats_data.append(summary_row("No Further Action", 'sent_only'))
    if metrics['has_email']:
        stats_data.append(summary_row("Email Replied", 'replied'))
        stats_data.append(summary_row("Reported", 'reported'))
    if metrics['delivery_errors']:
        stats_data.append(("Delivery Errors (recipients)", metrics['delivery_errors'], "", action_color("Delivery Errors")))
    if metrics['never_sent']:
        stats_data.append(("Never Sent (not counted)", metrics['never_sent'], "", COLORS['header_blue']))
    if metrics['has_generic']:
        stats_data.append(("Generic Links", metrics['generic_links'], "", COLORS['header_blue']))
        stats_data.append(("Generic Link Clicks (events)", metrics['generic_clicks'], "", COLORS['orange']))
        stats_data.append(("Generic Submissions (events)", metrics['generic_submissions'], "", COLORS['red']))
    row_colors = [color for _, _, _, color in stats_data]
    stats_data = [(label, value, percentage) for label, value, percentage, _ in stats_data]

    for i, (label, value, percentage) in enumerate(stats_data):
        current_row += 1
        row = current_row

        # Label cell
        cell = summary_sheet.cell(row=row, column=1)
        cell.value = label
        apply_cell_style(cell, create_cell_style(
            fill_color=row_colors[i],
            font_color=COLORS['white'],
            bold=True
        ))

        # Value cell
        cell = summary_sheet.cell(row=row, column=2)
        cell.value = value
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['light_grey']
        ))

        # Percentage cell
        cell = summary_sheet.cell(row=row, column=3)
        cell.value = percentage
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['light_grey']
        ))

    current_row += 2

    # In progress note
    if in_progress:
        note = "Note: Includes in-progress campaigns; data may change as they complete."
        summary_sheet[f'A{current_row}'] = note
        summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
        summary_sheet[f'A{current_row}'].font = Font(italic=True)
        # Add light yellow background to the note
        for cell in summary_sheet[f'A{current_row}:G{current_row}'][0]:
            cell.fill = PatternFill(start_color="FFF2CC", end_color="FFF2CC", fill_type="solid")
        current_row += 2

    # Key Takeaways section
    summary_sheet[f'A{current_row}'] = "Key Takeaways"
    summary_sheet[f'A{current_row}'].font = Font(bold=True, size=14)
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    current_row += 1

    # Phish-Prone Percentage, calculated as on the People page
    takeaways = [
        f"• Phish-Prone Percentage: {metrics['ppp']}% ({metrics['failed_deliveries']} of {metrics['deliveries']} "
        f"messages delivered led to a failure: clicking, submitting data, opening the attachment, running a "
        f"macro or replying). Calculated as on the People page.",
        f"• {metrics['people_failed']} of {total} people failed at least once.",
        f"• {metrics['high_risk']} high-risk people (a personal phish-prone % of 40 or more).",
        f"• Report rate: {metrics['report_rate']}% of messages delivered were reported.",
    ]
    for takeaway in takeaways:
        summary_sheet[f'A{current_row}'] = takeaway
        summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
        current_row += 1

    bullets = []
    if metrics['instant_failure_pct']:
        bullets.append(f"• {metrics['instant_failure_pct']}% of failures occurred in under 3 min (near-instant risk)")
    bullets.append(f"• {metrics['never_reported_pct']}% of users never reported phishing")
    timing = metrics['report_timing']
    if timing['count']:
        bullets.append(f"• {timing['after_24h_pct']}% of users who reported did so after 24 h")
        bullets.append(f"• Median time-to-report: {timing['median_min']} min")
        if timing['within_3min_pct']:
            bullets.append(f"• {timing['within_3min_pct']}% of users reported phishing within 3 min")
    elif metrics['reported']:
        bullets.append(f"• Timing data not available for {metrics['reported']} reporting users")

    # Add bullets to worksheet
    for bullet in bullets:
        summary_sheet[f'A{current_row}'] = bullet
        summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
        current_row += 1

    # Time-to-Action Distribution
    current_row += 1
    summary_sheet[f'A{current_row}'] = "Time-to-Action Distribution"
    summary_sheet[f'A{current_row}'].font = Font(bold=True, size=12)
    summary_sheet.merge_cells(f'A{current_row}:G{current_row}')
    current_row += 1

    bins = TIME_BINS
    actions = metrics['deltas']

    # Create headers for time distribution table
    headers = ["Action"] + [label for _, _, label in bins]
    for col, header in enumerate(headers, 1):
        cell = summary_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    time_dist_start_row = current_row

    # Filter out "Failure" for the table
    table_actions = {k: v for k, v in actions.items()
                     if k != 'Failure' and (metrics['has_attachments'] or k not in ('Open Attachment', 'Run Macro'))}

    # Define colors for each action type
    action_colors = {
        'Click': COLORS['orange'],
        'Submit Data': COLORS['red'],
        'Reply': COLORS['dark_orange'],
        'Report': COLORS['light_blue'],
        'Open Attachment': action_color('Attachment Opened'),
        'Run Macro': action_color('Macro Executed'),
    }

    for action_name, deltas in table_actions.items():
        current_row += 1

        # Action name with colored background
        action_cell = summary_sheet.cell(row=current_row, column=1)
        action_cell.value = action_name

        # Apply color to action cell
        series_color = action_colors.get(action_name, COLORS['header_blue'])
        apply_cell_style(action_cell, create_cell_style(
            fill_color=series_color,
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

        # Percentages for each time bin
        for col, (low, high, _) in enumerate(bins, 2):
            summary_sheet.cell(row=current_row, column=col).value = f"{pct_in_bin(deltas, low, high)}%"

    time_dist_end_row = current_row

    # Style the time distribution table (data cells only, not action column)
    for row in range(time_dist_start_row + 1, time_dist_end_row + 1):
        fill_color = COLORS['light_grey'] if (row - time_dist_start_row) % 2 == 1 else COLORS['dark_grey']
        # Skip column 1 (action names) which already has colors
        for col in range(2, len(headers) + 1):
            cell = summary_sheet.cell(row=row, column=col)
            apply_cell_style(cell, create_cell_style(
                fill_color=fill_color,
                alignment=Alignment(horizontal='center', vertical='center')
            ))

    # Create per-campaign failure table
    current_row += 2
    summary_sheet[f'A{current_row}'] = "Per Campaign Failures"
    summary_sheet[f'A{current_row}'].font = Font(bold=True, size=12)
    summary_sheet.merge_cells(f'A{current_row}:D{current_row}')
    current_row += 1

    # Headers for per-campaign failures table
    headers = ["Campaign", "Failures", "Sent", "Failure Rate (of sent)"]
    for col, header in enumerate(headers, 1):
        cell = summary_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    failures_start_row = current_row

    # Populate rows
    for campaign in campaigns:
        cm = campaign_metrics(campaign)
        failures = cm['clicks'] if cm['generic'] else cm['failed']
        current_row += 1
        summary_sheet.cell(row=current_row, column=1).value = clean_text(campaign.get('name', 'N/A'))
        summary_sheet.cell(row=current_row, column=2).value = failures
        summary_sheet.cell(row=current_row, column=3).value = cm['total'] if cm['generic'] else cm['sent']
        summary_sheet.cell(row=current_row, column=4).value = f"{cm['fail_pct']}%"

    failures_end_row = current_row

    # Style the failures table
    style_table(summary_sheet, failures_start_row, failures_end_row, 1, len(headers))

    # Auto-adjust column widths
    auto_adjust_column_width(summary_sheet)

    # -----------------------------------------
    # Per Campaign Unique Results and Per Campaign Results (cumulative)
    # -----------------------------------------
    # Columns: (header, colour, generic value, exclusive key, cumulative key).
    # Attachment columns only appear when an attachment was opened.
    has_att = metrics['has_attachments']
    action_columns = [('Data Submitted', 'submissions', 'submitted', 'submitted')]
    if has_att:
        action_columns.append(('Macro Executed', None, 'macro_only', 'macro_total'))
    action_columns.append(('Clicked Link', 'clicks', 'clicked_only', 'clicked_total'))
    if has_att:
        action_columns.append(('Attachment Opened', None, 'attachment_only', 'attachment_total'))
    action_columns += [
        ('Email Opened', None, 'opened_only', 'opened_total'),
        ('No Further Action', None, 'sent_only', None),
        ('Email Sent', None, None, 'sent'),
        ('Delivery Errors', None, 'errors', 'errors'),
        ('Email Replied', None, 'replied', 'replied'),
        ('Reported', None, 'reported', 'reported'),
    ]

    def write_campaign_sheet(sheet, title, exclusive):
        columns = [c for c in action_columns if (c[2] if exclusive else c[3]) is not None]
        headers = (['Campaign Name', 'Total Targets'] + [c[0] for c in columns] +
                   ['Fail % (of sent)', 'Report % (of sent)'])
        sheet['A1'] = title
        sheet['A1'].font = Font(size=16, bold=True)
        sheet.merge_cells('A1:I1')
        sheet['A1'].alignment = Alignment(horizontal='center')
        row = 3
        for col, header in enumerate(headers, 1):
            cell = sheet.cell(row=row, column=col)
            cell.value = header
            apply_cell_style(cell, create_cell_style(
                fill_color=action_color(header, COLORS['header_blue']),
                font_color=COLORS['white'],
                bold=True,
                alignment=Alignment(horizontal='center', vertical='center', wrap_text=True)
            ))
        start_row = row
        for campaign in campaigns:
            row += 1
            cm = campaign_metrics(campaign)
            values = [clean_text(campaign.get('name', 'Unnamed')), cm['total']]
            for header, generic_key, exclusive_key, cumulative_key in columns:
                if cm['generic']:
                    values.append(cm[generic_key] if generic_key else 'N/A')
                else:
                    values.append(cm[exclusive_key if exclusive else cumulative_key])
            values.append(f"{cm['fail_pct']}%")
            values.append('N/A' if cm['generic'] else f"{cm['report_pct']}%")
            for col, value in enumerate(values, 1):
                cell = sheet.cell(row=row, column=col)
                cell.value = value
                cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)
        for r in range(start_row + 1, row + 1):
            fill_color = COLORS['light_grey'] if (r - start_row) % 2 == 1 else COLORS['dark_grey']
            for col in range(1, len(headers) + 1):
                sheet.cell(row=r, column=col).fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")
        sheet.freeze_panes = 'B4'
        sheet.auto_filter.ref = f"A{start_row}:{get_column_letter(len(headers))}{row}"
        auto_adjust_column_width(sheet)

    write_campaign_sheet(scenario_sheet, "Per Campaign Unique Results", exclusive=True)
    write_campaign_sheet(cumulative_sheet, "Per Campaign Results (Cumulative)", exclusive=False)

    # -----------------------------------------
    # Campaign Overview Worksheet
    # -----------------------------------------
    current_row = 1
    overview_sheet['A1'] = "Campaign Overview"
    overview_sheet['A1'].font = Font(size=16, bold=True)
    overview_sheet.merge_cells('A1:C1')
    overview_sheet['A1'].alignment = Alignment(horizontal='center')
    current_row += 2

    # Headers for the campaign overview table
    # Dynamic headers based on whether Generic campaigns are present

    headers = [
        'Campaign ID', 'Campaign Name', 'Campaign Type', 'Created Date', 'Launch Date', 'Completion Date',
        'Status', 'Email Subject', 'Envelope Sender', 'Phishing URL', 'URL Parameter', 'Redirect URL',
        'Data Captured', 'Passwords Stored', 'Total Recipients/Links', 'Emails Opened',
        'Links Clicked', 'Data Submitted', 'Email Replied', 'Reported Phishing',
        'Delivery Errors', 'MFA Codes'
    ]
    if metrics['has_attachments']:
        headers += ['Attachments Opened', 'Macros Executed']

    # Create header row
    for col, header in enumerate(headers, 1):
        cell = overview_sheet.cell(row=current_row, column=col)
        cell.value = header

        # Choose color based on header
        if header == 'Emails Opened':
            color = COLORS['amber']
        elif header == 'Links Clicked':
            color = COLORS['orange']
        elif header == 'Data Submitted':
            color = COLORS['red']
        elif header == 'Email Replied':
            color = COLORS['dark_orange']
        elif header == 'Reported Phishing':
            color = COLORS['light_blue']
        elif header == 'Attachments Opened':
            color = action_color('Attachment Opened')
        elif header == 'Macros Executed':
            color = action_color('Macro Executed')
        elif header == 'Delivery Errors':
            color = action_color('Delivery Errors')
        else:
            color = COLORS['header_blue']

        apply_cell_style(cell, create_cell_style(
            fill_color=color,
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    overview_start_row = current_row

    # Populate rows with campaign data
    for campaign in campaigns:
        current_row += 1

        # Get campaign stats
        results = campaign.get('results', [])
        sent = len(results)
        is_generic = is_generic_campaign(campaign)
        is_sms = is_sms_campaign(campaign)

        # Determine campaign type display
        if is_generic:
            campaign_type = "Generic (Landing Page Only)"
        elif is_sms:
            campaign_type = "SMS"
        else:
            campaign_type = "Email"

        cm = campaign_metrics(campaign)
        if is_generic:
            clicked_c, subm_c = cm['clicks'], cm['submissions']
            opened_c = replied_c = rep_c = 0
        else:
            # Cumulative counts: opened includes clicked, clicked includes submitted
            opened_c, clicked_c = cm['opened_total'], cm['clicked_total']
            subm_c, replied_c, rep_c = cm['submitted'], cm['replied'], cm['reported']
        status_display = campaign_status_display(campaign)

        # Populate cells (adjusted for Campaign Type column at position 3)
        overview_sheet.cell(row=current_row, column=1).value = clean_text(str(campaign.get('id', 'N/A')))
        overview_sheet.cell(row=current_row, column=2).value = clean_text(campaign.get('name', 'N/A'))
        overview_sheet.cell(row=current_row, column=3).value = campaign_type
        overview_sheet.cell(row=current_row, column=4).value = format_date(campaign.get('created_date', 'N/A'))
        overview_sheet.cell(row=current_row, column=5).value = format_date(campaign.get('launch_date', 'N/A'))
        overview_sheet.cell(row=current_row, column=6).value = format_date(campaign.get('completed_date', 'N/A'))
        overview_sheet.cell(row=current_row, column=7).value = status_display
        # For Generic campaigns, Email Subject and Envelope Sender are N/A
        overview_sheet.cell(row=current_row, column=8).value = 'N/A' if is_generic else clean_text(campaign.get('template_details', {}).get('subject', 'N/A'))
        overview_sheet.cell(row=current_row, column=9).value = 'N/A' if is_generic else clean_text(campaign.get('template_details', {}).get('envelope_sender', 'N/A'))
        overview_sheet.cell(row=current_row, column=10).value = clean_text(campaign.get('phish_url', 'N/A'))
        overview_sheet.cell(row=current_row, column=11).value = clean_text(campaign.get('urlparam', 'rid'))
        overview_sheet.cell(row=current_row, column=12).value = clean_text(campaign.get('page_details', {}).get('redirect_url', 'N/A'))
        overview_sheet.cell(row=current_row, column=13).value = "Yes" if campaign.get('page_details', {}).get('capture_credentials', 0) else "No"
        overview_sheet.cell(row=current_row, column=14).value = "Yes" if campaign.get('page_details', {}).get('capture_passwords', 0) else "No"
        # For Generic campaigns, show "X links" instead of recipients
        if is_generic:
            overview_sheet.cell(row=current_row, column=15).value = f"{sent} links"
        else:
            overview_sheet.cell(row=current_row, column=15).value = str(sent)
        overview_sheet.cell(row=current_row, column=16).value = 'N/A' if is_generic else f"{opened_c} ({pct(opened_c, sent)}%)"
        # For Generic campaigns, show total click events
        if is_generic:
            overview_sheet.cell(row=current_row, column=17).value = f"{clicked_c} clicks"
        else:
            overview_sheet.cell(row=current_row, column=17).value = f"{clicked_c} ({pct(clicked_c, sent)}%)"
        if is_generic:
            overview_sheet.cell(row=current_row, column=18).value = f"{subm_c} submissions"
        else:
            overview_sheet.cell(row=current_row, column=18).value = f"{subm_c} ({pct(subm_c, sent)}%)"
        overview_sheet.cell(row=current_row, column=19).value = 'N/A' if is_generic else f"{replied_c} ({pct(replied_c, sent)}%)"
        overview_sheet.cell(row=current_row, column=20).value = 'N/A' if is_generic else f"{rep_c} ({pct(rep_c, sent)}%)"
        overview_sheet.cell(row=current_row, column=21).value = 'N/A' if is_generic else cm['errors']
        overview_sheet.cell(row=current_row, column=22).value = 'N/A' if is_generic else (mfa_summary(cm) or 'None')
        if metrics['has_attachments']:
            overview_sheet.cell(row=current_row, column=23).value = 'N/A' if is_generic else f"{cm['attachment_total']} ({pct(cm['attachment_total'], sent)}%)"
            overview_sheet.cell(row=current_row, column=24).value = 'N/A' if is_generic else f"{cm['macro_total']} ({pct(cm['macro_total'], sent)}%)"

        # Center all cells
        for col in range(1, len(headers) + 1):
            cell = overview_sheet.cell(row=current_row, column=col)
            cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)

    overview_end_row = current_row

    # Style the table with alternating row colors
    for row in range(overview_start_row + 1, overview_end_row + 1):
        fill_color = COLORS['light_grey'] if (row - overview_start_row) % 2 == 1 else COLORS['dark_grey']
        for col in range(1, len(headers) + 1):
            cell = overview_sheet.cell(row=row, column=col)
            cell.fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")

    # Freeze panes at row 3 (headers) and columns A-B
    overview_sheet.freeze_panes = 'C4'  # Freeze rows 1-3 and columns A-B

    # Enable filtering at row 3 (headers)
    overview_sheet.auto_filter.ref = f"A{overview_start_row}:{get_column_letter(len(headers))}{overview_end_row}"

    # Auto-adjust column widths
    auto_adjust_column_width(overview_sheet)

    # -----------------------------------------
    # Campaign Results Worksheet
    # -----------------------------------------
    current_row = 1
    results_sheet['A1'] = "Campaign Results"
    results_sheet['A1'].font = Font(size=16, bold=True)
    results_sheet.merge_cells('A1:J1')
    results_sheet['A1'].alignment = Alignment(horizontal='center')
    current_row += 2

    # Headers for the campaign results table
    headers = [
        'Campaign ID', 'Campaign Name', 'ID (RID)', 'Contact', 'Status', 'IP', 'Location',
        'Latitude', 'Longitude', 'Send Date', 'Reported', 'Modified Date', 'First Name',
        'Last Name', 'Position', 'Custom'
    ]

    for col, header in enumerate(headers, 1):
        cell = results_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    results_start_row = current_row

    # Collect all results
    all_results_details = []

    for campaign in campaigns:
        campaign_id = campaign.get('id', 'N/A')
        campaign_name = campaign.get('name', 'Unnamed')
        is_sms = is_sms_campaign(campaign)
        is_generic = is_generic_campaign(campaign)

        for result in campaign.get('results', []):
            contact = privacy.contact(result)
            status = result.get('status', 'N/A')

            # Adjust status display for SMS campaigns
            if is_sms and status == 'Email Sent':
                status = 'SMS Sent'

            raw_ip = extract_ip_address(result)
            if raw_ip == 'N/A':
                ip = location = latitude = longitude = 'N/A'
            else:
                ip = privacy.ip(raw_ip)
                location = privacy.location(raw_ip)
                coords = privacy.coordinates(result)
                latitude, longitude = coords if coords else ('N/A', 'N/A')

            send_date = format_date(result.get('send_date'))
            reported = "Yes" if result.get('reported') else "No"
            modified_date = format_date(result.get('modified_date'))

            if is_generic:
                # Generic results are links, not people: first_name holds the link name
                contact = clean_text(result.get('first_name') or 'Unknown Link')
                first_name = last_name = position = custom = 'N/A'
            else:
                # Recipient details come from the group; never trust the submitted
                # payload for them. All of them are personal data.
                first_name = privacy.personal(result.get('first_name') or 'N/A')
                last_name = privacy.personal(result.get('last_name') or 'N/A')
                position = privacy.personal(result.get('position') or 'N/A')
                custom = privacy.personal(result.get('custom') or 'N/A')

            # Add to results list
            all_results_details.append({
                'campaign_id': campaign_id,
                'campaign_name': campaign_name,
                'rid': result.get('id', 'N/A'),
                'contact': contact,
                'status': status,
                'ip': ip,
                'location': location,
                'latitude': latitude,
                'longitude': longitude,
                'send_date': send_date,
                'reported': reported,
                'modified_date': modified_date,
                'first_name': first_name,
                'last_name': last_name,
                'position': position,
                'custom': custom
            })

    # Add results to the worksheet
    for result_detail in all_results_details:
        current_row += 1

        results_sheet.cell(row=current_row, column=1).value = clean_text(str(result_detail['campaign_id']))
        results_sheet.cell(row=current_row, column=2).value = clean_text(result_detail['campaign_name'])
        results_sheet.cell(row=current_row, column=3).value = clean_text(str(result_detail['rid']))
        results_sheet.cell(row=current_row, column=4).value = clean_text(result_detail['contact'])

        # Status with color coding
        status_cell = results_sheet.cell(row=current_row, column=5)
        status_cell.value = clean_text(result_detail['status'])

        # Colour by status, as on the campaign results page
        status = result_detail['status']
        color = action_color(status, None)

        if color:
            status_cell.fill = PatternFill(start_color=color, end_color=color, fill_type="solid")
            status_cell.font = Font(color=COLORS['white'], bold=True)

        results_sheet.cell(row=current_row, column=6).value = clean_text(result_detail['ip'])
        results_sheet.cell(row=current_row, column=7).value = clean_text(result_detail['location'])
        results_sheet.cell(row=current_row, column=8).value = clean_text(str(result_detail['latitude']) if result_detail['latitude'] != 'N/A' else 'N/A')
        results_sheet.cell(row=current_row, column=9).value = clean_text(str(result_detail['longitude']) if result_detail['longitude'] != 'N/A' else 'N/A')
        results_sheet.cell(row=current_row, column=10).value = result_detail['send_date']
        results_sheet.cell(row=current_row, column=11).value = result_detail['reported']
        results_sheet.cell(row=current_row, column=12).value = result_detail['modified_date']
        results_sheet.cell(row=current_row, column=13).value = clean_text(result_detail['first_name'])
        results_sheet.cell(row=current_row, column=14).value = clean_text(result_detail['last_name'])
        results_sheet.cell(row=current_row, column=15).value = clean_text(result_detail['position'])
        results_sheet.cell(row=current_row, column=16).value = clean_text(result_detail['custom'])

    results_end_row = current_row

    # Apply alternating row colors but preserve status cell color
    for row in range(results_start_row + 1, results_end_row + 1):
        fill_color = COLORS['light_grey'] if (row - results_start_row) % 2 == 1 else COLORS['dark_grey']

        for col in range(1, len(headers) + 1):
            # Skip the status column (5) which already has colors
            if col != 5:
                cell = results_sheet.cell(row=row, column=col)
                cell.fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")
                cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)
            else:
                # For status column, keep the color but center the text
                cell = results_sheet.cell(row=row, column=col)
                if not cell.fill.start_color.index or cell.fill.start_color.index == '00000000':
                    cell.fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")
                cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)

    # Freeze panes at row 3 (headers) and columns A-B
    results_sheet.freeze_panes = 'C4'  # Freeze rows 1-3 and columns A-B

    # Enable filtering at row 3 (headers)
    results_sheet.auto_filter.ref = f"A{results_start_row}:{get_column_letter(len(headers))}{results_end_row}"

    # Auto-adjust column widths
    auto_adjust_column_width(results_sheet)

    # -----------------------------------------
    # Per-Campaign Detailed Events Worksheets
    # -----------------------------------------
    for campaign in campaigns:
        campaign_name = campaign.get('name', 'Unnamed')
        campaign_id = campaign.get('id', 'N/A')
        is_sms = is_sms_campaign(campaign)
        is_generic = is_generic_campaign(campaign)

        # Create sheet name (Excel has 31 char limit) and sanitize for Excel compatibility
        sheet_name = sanitize_sheet_name(f"Events-{campaign_name}", workbook.sheetnames)
        campaign_events_sheet = workbook.create_sheet(sheet_name)

        # Title and headers
        current_row = 1
        campaign_events_sheet[f'A{current_row}'] = f"Detailed Events - {campaign_name}"
        campaign_events_sheet[f'A{current_row}'].font = Font(size=16, bold=True)
        campaign_events_sheet.merge_cells(f'A{current_row}:K{current_row}')
        campaign_events_sheet[f'A{current_row}'].alignment = Alignment(horizontal='center')
        current_row += 2

        # Dynamic headers based on campaign type
        if is_generic:
            headers = [
                'RID', 'Link Name', 'IP', 'Latitude', 'Longitude',
                'Location', 'Event Type', 'Time', 'Browser', 'OS', 'Details'
            ]
        elif is_sms:
            headers = [
                'RID', 'Phone Number', 'IP', 'Latitude', 'Longitude',
                'Location', 'Event Type', 'Time', 'Browser', 'OS', 'Details'
            ]
        else:
            headers = [
                'RID', 'Email', 'IP', 'Latitude', 'Longitude',
                'Location', 'Event Type', 'Time', 'Browser', 'OS', 'Details'
            ]

        # Apply headers
        for col, header in enumerate(headers, 1):
            cell = campaign_events_sheet.cell(row=current_row, column=col)
            cell.value = header
            apply_cell_style(cell, create_cell_style(
                fill_color=COLORS['header_blue'],
                font_color=COLORS['white'],
                bold=True,
                alignment=Alignment(horizontal='center', vertical='center')
            ))

        events_start_row = current_row

        # Every recorded event for this campaign, each with its own IP,
        # browser and captured data: sends, opens, attachment opens, macro
        # runs, clicks, submissions, MFA events, replies, reports, delivery
        # errors and proxied requests. Campaign-level events are skipped.
        campaign_events = []
        for event in campaign.get('timeline', []):
            result = event.get('_result')
            msg = event.get('message') or ''
            if result is None or msg in ('Campaign Created',):
                continue
            details = event.get('details') or {}
            browser_meta = event_browser(details)
            raw_ip = browser_meta.get('address') or 'N/A'
            ua = browser_meta.get('user-agent', '')
            coords = privacy.coordinates(result) if raw_ip != 'N/A' else None
            if is_generic:
                contact = result.get('first_name') or 'Unknown Link'
            else:
                contact = privacy.contact(result)
            extra = []
            if details.get('error'):
                extra.append(f"Error: {details['error']}")
            captured = payload_text(details.get('payload'), redact=privacy.emails, separator=', ')
            if captured:
                extra.append(f"Data: {captured}")
            campaign_events.append({
                'contact': contact,
                'result_id': result.get('id', 'N/A'),
                'event_type': msg,
                'time': event.get('time'),
                'ip': privacy.ip(raw_ip),
                'raw_ip': raw_ip,
                'browser': describe_browser(ua) if ua else 'N/A',
                'os': describe_os(ua) if ua else 'N/A',
                'details': ' | '.join(extra),
                'latitude': coords[0] if coords else 'N/A',
                'longitude': coords[1] if coords else 'N/A',
            })

        # Already in time order: preprocess() sorted the timeline

        # Populate rows
        for event in campaign_events:
            current_row += 1

            campaign_events_sheet.cell(row=current_row, column=1).value = clean_text(str(event.get('result_id', 'N/A')))
            campaign_events_sheet.cell(row=current_row, column=2).value = clean_text(event['contact'])
            campaign_events_sheet.cell(row=current_row, column=3).value = clean_text(event['ip'])

            # Latitude and Longitude
            lat = event.get('latitude', 'N/A')
            lon = event.get('longitude', 'N/A')
            campaign_events_sheet.cell(row=current_row, column=4).value = clean_text(str(lat) if lat != 'N/A' else 'N/A')
            campaign_events_sheet.cell(row=current_row, column=5).value = clean_text(str(lon) if lon != 'N/A' else 'N/A')

            # Location (from the real IP; country only when IPs are anonymized)
            raw_ip = event.get('raw_ip', 'N/A')
            location = privacy.location(raw_ip) if raw_ip not in ('N/A', 'Unknown', '') else 'N/A'
            campaign_events_sheet.cell(row=current_row, column=6).value = clean_text(location)

            # Event type with color coding
            cell = campaign_events_sheet.cell(row=current_row, column=7)
            event_type = event['event_type']
            cell.value = event_type

            # Colour by event type, as on the campaign results page
            color = action_color(event_type, None)

            if color:
                cell.fill = PatternFill(start_color=color, end_color=color, fill_type="solid")
                cell.font = Font(color=COLORS['white'], bold=True)

            # Time
            campaign_events_sheet.cell(row=current_row, column=8).value = format_date(event['time'])

            # Browser & OS
            campaign_events_sheet.cell(row=current_row, column=9).value = clean_text(event['browser'])
            campaign_events_sheet.cell(row=current_row, column=10).value = clean_text(event['os'])

            # Details
            campaign_events_sheet.cell(row=current_row, column=11).value = clean_text(event['details'])

        events_end_row = current_row

        # Style the events table
        style_header_row(campaign_events_sheet, events_start_row, 1, len(headers))

        # Apply alternating row colors but preserve event type cell color
        for row in range(events_start_row + 1, events_end_row + 1):
            fill_color = COLORS['light_grey'] if (row - events_start_row) % 2 == 1 else COLORS['dark_grey']

            for col in range(1, len(headers) + 1):
                # Skip the event type column (7) which already has colors
                if col != 7:
                    cell = campaign_events_sheet.cell(row=row, column=col)
                    cell.fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")
                    cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)
                else:
                    # For event type column, keep the color but center the text
                    cell = campaign_events_sheet.cell(row=row, column=col)
                    cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)

        # Freeze panes at row 3 (headers) and column A
        campaign_events_sheet.freeze_panes = 'B4'

        # Enable filtering at row 3 (headers)
        if events_end_row > events_start_row:
            campaign_events_sheet.auto_filter.ref = f"A{events_start_row}:{get_column_letter(len(headers))}{events_end_row}"

        # Auto-adjust column widths
        auto_adjust_column_width(campaign_events_sheet)

    # -----------------------------------------
    # Browser Statistics Worksheet
    # -----------------------------------------
    current_row = 1
    browser_sheet['A1'] = "Browser Statistics"
    browser_sheet['A1'].font = Font(size=16, bold=True)
    browser_sheet.merge_cells('A1:C1')
    browser_sheet['A1'].alignment = Alignment(horizontal='center')
    current_row += 2

    # Headers for the browser statistics table
    headers = ["Browser", "Count", "Percentage"]
    for col, header in enumerate(headers, 1):
        cell = browser_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    browser_start_row = current_row

    # Browser, OS and IP counts for results with an interaction
    stats_results = [r for c in campaigns for r in c.get('results', [])]
    browser_counts, os_counts, ip_counts = interaction_stats(stats_results, privacy)

    # Calculate total for percentage
    total_browsers = sum(browser_counts.values())

    # Add browser statistics to the worksheet
    for browser, count in sorted(browser_counts.items(), key=lambda x: -x[1]):
        current_row += 1
        percentage = round((count / total_browsers) * 100) if total_browsers else 0

        browser_sheet.cell(row=current_row, column=1).value = clean_text(browser)
        browser_sheet.cell(row=current_row, column=2).value = count
        browser_sheet.cell(row=current_row, column=3).value = f"{percentage}%"

    browser_end_row = current_row

    # Style the browser statistics table
    style_table(browser_sheet, browser_start_row, browser_end_row, 1, len(headers))

    # Freeze panes at row 3 (headers)
    browser_sheet.freeze_panes = 'A4'  # Freeze rows 1-3

    # Enable filtering at row 3 (headers)
    browser_sheet.auto_filter.ref = f"A{browser_start_row}:C{browser_end_row}"

    # Auto-adjust column widths
    auto_adjust_column_width(browser_sheet)

    # -----------------------------------------
    # OS Statistics Worksheet
    # -----------------------------------------
    current_row = 1
    os_sheet['A1'] = "Operating System Statistics"
    os_sheet['A1'].font = Font(size=16, bold=True)
    os_sheet.merge_cells('A1:C1')
    os_sheet['A1'].alignment = Alignment(horizontal='center')
    current_row += 2

    # Headers for the OS statistics table
    headers = ["Operating System", "Count", "Percentage"]
    for col, header in enumerate(headers, 1):
        cell = os_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    os_start_row = current_row

    # Calculate total for percentage
    total_os = sum(os_counts.values())

    # Add OS statistics to the worksheet
    for os_name, count in sorted(os_counts.items(), key=lambda x: -x[1]):
        current_row += 1
        percentage = round((count / total_os) * 100) if total_os else 0

        os_sheet.cell(row=current_row, column=1).value = clean_text(os_name)
        os_sheet.cell(row=current_row, column=2).value = count
        os_sheet.cell(row=current_row, column=3).value = f"{percentage}%"

    os_end_row = current_row

    # Style the OS statistics table
    style_table(os_sheet, os_start_row, os_end_row, 1, len(headers))

    # Freeze panes at row 3 (headers)
    os_sheet.freeze_panes = 'A4'  # Freeze rows 1-3

    # Enable filtering at row 3 (headers)
    os_sheet.auto_filter.ref = f"A{os_start_row}:C{os_end_row}"

    # Auto-adjust column widths
    auto_adjust_column_width(os_sheet)

    # -----------------------------------------
    # IP Statistics Worksheet
    # -----------------------------------------
    current_row = 1
    ip_sheet['A1'] = "IP Address and Location Statistics"
    ip_sheet['A1'].font = Font(size=16, bold=True)
    ip_sheet.merge_cells('A1:D1')
    ip_sheet['A1'].alignment = Alignment(horizontal='center')
    current_row += 2

    # Headers for the IP statistics table
    headers = ["IP Address", "Location", "Count", "Percentage"]
    for col, header in enumerate(headers, 1):
        cell = ip_sheet.cell(row=current_row, column=col)
        cell.value = header
        apply_cell_style(cell, create_cell_style(
            fill_color=COLORS['header_blue'],
            font_color=COLORS['white'],
            bold=True,
            alignment=Alignment(horizontal='center', vertical='center')
        ))

    ip_start_row = current_row

    # IPs are masked and locations reduced to country level when anonymizing
    total_ips = sum(entry['count'] for entry in ip_counts.values())
    for ip, entry in sorted(ip_counts.items(), key=lambda x: -x[1]['count']):
        current_row += 1
        ip_sheet.cell(row=current_row, column=1).value = clean_text(ip)
        ip_sheet.cell(row=current_row, column=2).value = clean_text(entry['location'])
        ip_sheet.cell(row=current_row, column=3).value = entry['count']
        ip_sheet.cell(row=current_row, column=4).value = f"{pct(entry['count'], total_ips)}%"

    ip_end_row = current_row

    # Style the IP statistics table
    style_table(ip_sheet, ip_start_row, ip_end_row, 1, len(headers))

    # Freeze panes at row 3 (headers)
    ip_sheet.freeze_panes = 'A4'  # Freeze rows 1-3

    # Enable filtering at row 3 (headers)
    ip_sheet.auto_filter.ref = f"A{ip_start_row}:D{ip_end_row}"

    # Auto-adjust column widths
    auto_adjust_column_width(ip_sheet)

    # -----------------------------------------
    # Repeat Offenders Worksheet (users who failed in 2+ campaigns)
    # -----------------------------------------
    repeat_offenders = metrics['repeat_offenders']

    if repeat_offenders:
        repeat_sheet = workbook.create_sheet("Repeat Offenders")

        current_row = 1
        repeat_sheet['A1'] = "Repeat Offenders"
        repeat_sheet['A1'].font = Font(size=16, bold=True)
        repeat_sheet.merge_cells('A1:D1')
        repeat_sheet['A1'].alignment = Alignment(horizontal='center')
        current_row += 1

        # Add explanatory note
        note_text = "Users who failed (clicked, submitted data, opened the attachment, ran a macro or replied) in more than one campaign. These users may require additional security awareness training."
        repeat_sheet[f'A{current_row}'] = note_text
        repeat_sheet[f'A{current_row}'].font = Font(italic=True, size=9)
        repeat_sheet.merge_cells(f'A{current_row}:D{current_row}')
        repeat_sheet[f'A{current_row}'].alignment = Alignment(wrap_text=True)
        repeat_sheet.row_dimensions[current_row].height = 30
        current_row += 1

        # Headers for the repeat offenders table
        headers = ["Email/Phone", "Campaigns Failed", "Campaign Names"]
        for col, header in enumerate(headers, 1):
            cell = repeat_sheet.cell(row=current_row, column=col)
            cell.value = header
            apply_cell_style(cell, create_cell_style(
                fill_color='C55A11',  # Orange/Red for warning
                font_color=COLORS['white'],
                bold=True,
                alignment=Alignment(horizontal='center', vertical='center')
            ))

        repeat_start_row = current_row

        # Add data rows sorted by number of campaigns failed (descending)
        for offender, contact, failed_campaigns in repeat_offenders:
            current_row += 1
            repeat_sheet.cell(row=current_row, column=1).value = clean_text(privacy.contact(offender, contact))
            repeat_sheet.cell(row=current_row, column=2).value = len(failed_campaigns)
            repeat_sheet.cell(row=current_row, column=3).value = clean_text(', '.join(failed_campaigns))

            # Center align all cells
            for col in range(1, len(headers) + 1):
                cell = repeat_sheet.cell(row=current_row, column=col)
                cell.alignment = Alignment(horizontal='center', vertical='center', wrap_text=True)

        repeat_end_row = current_row

        # Style the table with alternating row colors
        for row in range(repeat_start_row + 1, repeat_end_row + 1):
            fill_color = COLORS['light_grey'] if (row - repeat_start_row) % 2 == 1 else COLORS['dark_grey']
            for col in range(1, len(headers) + 1):
                cell = repeat_sheet.cell(row=row, column=col)
                cell.fill = PatternFill(start_color=fill_color, end_color=fill_color, fill_type="solid")

        # Freeze panes at row 4 (after headers)
        repeat_sheet.freeze_panes = 'A5'

        # Enable filtering
        if repeat_end_row > repeat_start_row:
            repeat_sheet.auto_filter.ref = f"A{repeat_start_row}:C{repeat_end_row}"

        # Auto-adjust column widths
        auto_adjust_column_width(repeat_sheet)

    # Save the workbook
    neutralize_formulas(workbook)
    try:
        workbook.save(output_path)
        sys.stderr.write(f"Excel report saved to {output_path}\n")
        return True
    except Exception as e:
        sys.stderr.write(f"Error saving Excel document: {e}\n")
        return False
