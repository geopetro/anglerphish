#!/usr/bin/env python
"""
Word document generator for Anglerphish reports.
"""

import sys

try:
    from docx import Document
    from docx.enum.text import WD_ALIGN_PARAGRAPH
    from docx.oxml import OxmlElement
    from docx.oxml.ns import qn
    from docx.shared import RGBColor
except ImportError:
    sys.stderr.write("Error: python-docx library is required. Install with: pip install python-docx\n")
    sys.exit(1)

from report_common import (
    TIME_BINS, Privacy, action_color, campaign_metrics, mfa_summary, result_timeline,
    campaign_status_display, clean_text, compute_metrics, describe_browser,
    describe_os, event_browser, extract_browser_info, extract_ip_address,
    extract_os_info, format_date, full_name, generated_on, has_user_interaction, interaction_stats, is_generic_campaign,
    is_sms_campaign, is_sms_result, locked_events_notice, payload_text,
    pct, pct_in_bin, preprocess,
)

def set_cell_background(cell, fill_hex):
    """Set the cell shading (background color). fill_hex is a string like '4472C4' (no '#')."""
    tcPr = cell._tc.get_or_add_tcPr()
    shd = OxmlElement('w:shd')
    shd.set(qn('w:val'), 'clear')
    shd.set(qn('w:color'), 'auto')
    shd.set(qn('w:fill'), fill_hex)
    tcPr.append(shd)

def set_table_borders_color(table, border_hex='FFFFFF', border_size='4', border_style='single'):
    """
    Turn all table borders (outer + inner) to the given hex color.
      • border_hex: 'RRGGBB' (no ‘#’)
      • border_size: width in eighths of a point (string)
      • border_style: e.g. 'single', 'double', etc.
    """
    tbl = table._tbl

    # 1) Grab or create <w:tblPr>
    tblPr = tbl.tblPr
    if tblPr is None:
        tblPr = OxmlElement('w:tblPr')
        tbl.insert(0, tblPr)

    # 2) Remove any existing <w:tblBorders>
    for old in list(tblPr.findall(qn('w:tblBorders'))):
        tblPr.remove(old)

    # 3) Build new borders block
    borders = OxmlElement('w:tblBorders')
    for edge in ('top','left','bottom','right','insideH','insideV'):
        elem = OxmlElement(f'w:{edge}')
        elem.set(qn('w:val'),   border_style)
        elem.set(qn('w:sz'),    border_size)
        elem.set(qn('w:color'), border_hex)
        borders.append(elem)

    # 4) Append in place
    tblPr.append(borders)

def style_table(
    table,
    header_fill='4472C4',
    row_fill='D9D9D9',
    alt_row_fill='BFBFBF',
    border_hex='FFFFFF',
    header_font_hex='FFFFFF'
):
    # 1) Set borders
    set_table_borders_color(table, border_hex=border_hex)

    # 2) Header styling
    if header_fill is not None and header_font_hex is not None:
            r, g, b = (int(header_font_hex[i:i+2], 16) for i in (0, 2, 4))
            for cell in table.rows[0].cells:
                set_cell_background(cell, header_fill)
                for p in cell.paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(r, g, b)

    # 3) Alternating row styling
    for i, row in enumerate(table.rows[1:], start=1):
        fill = row_fill if i % 2 else alt_row_fill
        for cell in row.cells:
            set_cell_background(cell, fill)



def generate_word_document(data, output_path, gdpr_options=None):
    """Generate a Word (.docx) report from Anglerphish campaign data."""
    if not isinstance(data, dict) or not data.get('campaigns'):
        sys.stderr.write("Error: Invalid or empty campaign data\n")
        return False

    privacy = Privacy(gdpr_options)
    preprocess(data)
    metrics = compute_metrics(data)

    # Create document
    doc = Document()
    title = doc.add_heading("Anglerphish Campaign Report", 0)
    title.alignment = WD_ALIGN_PARAGRAPH.CENTER
    if data.get('set_name'):
        set_para = doc.add_paragraph(f"Campaign set: {clean_text(data['set_name'])}")
        set_para.alignment = WD_ALIGN_PARAGRAPH.CENTER
    date_para = doc.add_paragraph(generated_on())
    date_para.alignment = WD_ALIGN_PARAGRAPH.CENTER

    if privacy.enabled:
        doc.add_heading("Privacy Notice", level=2)
        masked = []
        if privacy.emails:
            masked.append("email addresses, phone numbers, names and submitted values")
        if privacy.ips:
            masked.append("IP addresses (locations are shown at country level)")
        privacy_para = doc.add_paragraph(
            "This report has been generated with personal data obfuscated in accordance with privacy "
            "requirements. The following have been masked: " + "; ".join(masked) + "."
        )
        privacy_para.style = 'Intense Quote'
        doc.add_paragraph()

    locked_notice = locked_events_notice(data)
    if locked_notice:
        doc.add_paragraph(locked_notice, style='Intense Quote')

    # Executive Summary
    doc.add_heading("Executive Summary", level=1)
    doc.add_paragraph(
        "This report provides an analysis of the phishing campaign(s) conducted using the Anglerphish platform, "
        "including performance metrics and key user interactions."
    )

    total = metrics['total']
    reported = metrics['reported']
    has_email_campaigns = metrics['has_email']
    has_attachments = metrics['has_attachments']
    has_generic_campaigns = metrics['has_generic']
    in_progress = metrics['in_progress']

    doc.add_heading("Overall Campaign Results", level=2)

    # Add explanatory note about deduplication
    doc.add_paragraph(
        "The following table shows unique users across all campaigns. Each user is counted once, "
        "categorized by their highest-risk action (e.g., if a user clicked in Campaign A but submitted "
        "data in Campaign B, they are counted under 'Data Submitted')."
    )

    # Mutually exclusive rows (each person counted once, by highest action,
    # ranked as the app ranks a result's status); they add up to the total.
    # Replied/reported are separate behaviours and can overlap with them.
    def overall_row(label, count):
        return (label, count, pct(count, total), action_color(label))

    row_data = [overall_row("Data Submitted", metrics['submitted'])]
    if has_attachments:
        row_data.append(overall_row("Macro Executed", metrics['macro']))
    row_data.append(overall_row("Clicked Link", metrics['clicked']))
    if has_attachments:
        row_data.append(overall_row("Attachment Opened", metrics['attachment']))
    if has_email_campaigns:
        row_data.append(overall_row("Email Opened", metrics['opened']))
    row_data.append(overall_row("No Further Action", metrics['sent_only']))
    if has_email_campaigns:
        row_data.append(overall_row("Email Replied", metrics['replied']))
        row_data.append(overall_row("Reported", reported))
    row_data.append(("Total", total, '', 'D9D9D9'))  # Gray - no percentage for total

    notes = []
    if metrics['never_sent']:
        notes.append(f"{metrics['never_sent']} recipient(s) who were never sent a message are not counted.")
    if metrics['delivery_errors']:
        notes.append(f"{metrics['delivery_errors']} recipient(s) had a delivery error.")
    if notes:
        doc.add_paragraph(' '.join(notes))

    # Create table with appropriate number of rows
    stats_tbl = doc.add_table(rows=len(row_data) + 1, cols=3, style='Table Grid')

    # Header row
    hdr = stats_tbl.rows[0].cells
    hdr[0].text = 'Category'
    hdr[1].text = 'Users'
    hdr[2].text = 'Percentage'

    # Style header row with blue background and white text
    for cell in hdr:
        set_cell_background(cell, '4472C4')
        for p in cell.paragraphs:
            for run in p.runs:
                run.font.color.rgb = RGBColor(255, 255, 255)
                run.bold = True

    # Add data and style the cells (using the dynamically created row_data from above)
    for i, (label, count, percentage, color) in enumerate(row_data, start=1):
        row = stats_tbl.rows[i].cells

        # Category column
        row[0].text = label
        set_cell_background(row[0], color)
        for p in row[0].paragraphs:
            for run in p.runs:
                run.font.color.rgb = RGBColor(255, 255, 255)
                run.bold = True

        # Users column
        row[1].text = str(count)
        set_cell_background(row[1], 'E7E6E6')  # Light gray

        # Percentage column
        row[2].text = f"{percentage}%" if percentage != '' else ''
        set_cell_background(row[2], 'E7E6E6')  # Light gray

    # Set table borders to white
    set_table_borders_color(stats_tbl, border_hex='FFFFFF')

    # Add Generic Campaign Results section if there are generic campaigns
    if has_generic_campaigns:
        doc.add_paragraph()  # spacing
        doc.add_heading("Generic Campaign Results (Event Counts)", level=2)
        doc.add_paragraph(
            "Generic campaigns count every event (not unique users) since links can be shared "
            "and accessed by multiple anonymous users. The following shows total engagement events."
        )

        generic_row_data = [
            ("Total Links", metrics['generic_links'], '', '1ABC9C'),  # Green
            ("Total Clicks", metrics['generic_clicks'], '', 'F39C12'),  # Orange
            ("Total Submissions", metrics['generic_submissions'], '', 'F05B4F'),  # Red
        ]

        generic_tbl = doc.add_table(rows=len(generic_row_data) + 1, cols=2, style='Table Grid')

        # Header row
        hdr = generic_tbl.rows[0].cells
        hdr[0].text = 'Metric'
        hdr[1].text = 'Count'

        for cell in hdr:
            set_cell_background(cell, '4472C4')
            for p in cell.paragraphs:
                for run in p.runs:
                    run.font.color.rgb = RGBColor(255, 255, 255)
                    run.bold = True

        for i, (label, count, _, color) in enumerate(generic_row_data, start=1):
            row = generic_tbl.rows[i].cells
            row[0].text = label
            set_cell_background(row[0], color)
            for p in row[0].paragraphs:
                for run in p.runs:
                    run.font.color.rgb = RGBColor(255, 255, 255)
                    run.bold = True

            row[1].text = str(count)
            set_cell_background(row[1], 'E7E6E6')

        set_table_borders_color(generic_tbl, border_hex='FFFFFF')

    if in_progress:
        doc.add_paragraph(
            "Note: Includes in-progress campaigns; data may change as they complete.",
            style='Intense Quote'
        )

    # Add per-scenario results table
    doc.add_paragraph()  # spacing
    scenario_tbl = doc.add_table(rows=1, cols=6, style='Table Grid')

    # Header row with specific colors
    hdr = scenario_tbl.rows[0].cells
    hdr[0].text = 'Scenario'
    hdr[1].text = 'Targets\nFailed'
    hdr[2].text = 'Reported\nPhishing'
    hdr[3].text = 'Sent'
    hdr[4].text = 'Fail %\n(of sent)'
    hdr[5].text = 'Report %\n(of sent)'

    # Header colors: Scenario (blue), Targets Failed (red), Reported Phishing (cyan), rest (blue)
    header_colors = ['4472C4', 'C55A11', '00B0F0', '4472C4', '4472C4', '4472C4']

    for idx, (cell, color) in enumerate(zip(hdr, header_colors)):
        set_cell_background(cell, color)
        for p in cell.paragraphs:
            for run in p.runs:
                run.font.color.rgb = RGBColor(255, 255, 255)
                run.bold = True

    # Add data rows for each campaign/scenario
    for campaign in data['campaigns']:
        name = campaign.get('name', 'N/A')
        cm = campaign_metrics(campaign)
        total_targets = cm['total']
        is_generic_camp = cm['generic']
        if is_generic_camp:
            # Generic links count every click event (links can be shared)
            targets_failed = cm['clicks']
            reported_count = 0
            report_pct = 0
        else:
            targets_failed = cm['failed']
            reported_count = cm['reported']
            report_pct = cm['report_pct']
            total_targets = cm['sent']
        fail_pct = cm['fail_pct']

        # Add row
        row = scenario_tbl.add_row().cells
        row[0].text = clean_text(name)
        # For Generic campaigns, show "Total Clicks" label context
        row[1].text = str(targets_failed) + (" clicks" if is_generic_camp else "")
        row[2].text = str(reported_count) if not is_generic_camp else "N/A"
        row[3].text = str(total_targets) + (" links" if is_generic_camp else "")
        row[4].text = f"{fail_pct}%"
        row[5].text = f"{report_pct}%" if not is_generic_camp else "N/A"

        # Alternate row coloring - light gray and white
        row_num = len(scenario_tbl.rows) - 1  # Current row number (excluding header)
        fill_color = 'E7E6E6' if row_num % 2 else 'FFFFFF'
        for cell in row:
            set_cell_background(cell, fill_color)

    # Set table borders
    set_table_borders_color(scenario_tbl, border_hex='FFFFFF')

    # Add "Repeat Offenders" table - users who failed in more than one campaign
    repeat_offenders = metrics['repeat_offenders']

    if repeat_offenders:
        doc.add_paragraph()  # spacing
        doc.add_heading("Repeat Offenders", level=2)
        doc.add_paragraph(
            "The following users failed (clicked, submitted data, opened the attachment, ran a macro or replied) in more than one campaign. "
            "These users may require additional security awareness training."
        )

        # Create table with Email/Phone, # of Campaigns Failed, Campaign Names
        repeat_tbl = doc.add_table(rows=1, cols=3, style='Table Grid')

        # Header row
        hdr = repeat_tbl.rows[0].cells
        hdr[0].text = 'Email/Phone'
        hdr[1].text = 'Campaigns Failed'
        hdr[2].text = 'Campaign Names'

        # Style header row
        for cell in hdr:
            set_cell_background(cell, 'C55A11')  # Orange/Red for warning
            for p in cell.paragraphs:
                for run in p.runs:
                    run.font.color.rgb = RGBColor(255, 255, 255)
                    run.bold = True

        # Add data rows sorted by number of campaigns failed (descending)
        for offender, contact, campaigns in repeat_offenders:
            row = repeat_tbl.add_row().cells
            row[0].text = clean_text(privacy.contact(offender, contact))
            row[1].text = str(len(campaigns))
            row[2].text = clean_text(', '.join(sorted(campaigns)))

            # Alternate row coloring
            row_num = len(repeat_tbl.rows) - 1
            fill_color = 'E7E6E6' if row_num % 2 else 'FFFFFF'
            for cell in row:
                set_cell_background(cell, fill_color)

        # Set table borders
        set_table_borders_color(repeat_tbl, border_hex='FFFFFF')

    doc.add_heading("Key Takeaways", level=1)
    # — Phish-Prone Percentage, calculated as on the People page —
    takeaways = [
        f"Phish-Prone Percentage: {metrics['ppp']}% ({metrics['failed_deliveries']} of "
        f"{metrics['deliveries']} messages delivered led to a failure: clicking, submitting data, "
        f"opening the attachment, running a macro or replying). Calculated as on the People page.",
        f"{metrics['people_failed']} of {total} people failed at least once.",
        f"{metrics['high_risk']} high-risk people (a personal phish-prone % of 40 or more).",
        f"Report rate: {metrics['report_rate']}% of messages delivered were reported.",
    ]
    for text in takeaways:
        para = doc.add_paragraph(text, style='List Bullet')
        para.alignment = WD_ALIGN_PARAGRAPH.LEFT

    # — Execution Summary: speed metrics bullets —
    bullets = []
    if metrics['instant_failure_pct']:
        bullets.append(f"{metrics['instant_failure_pct']}% of failures occurred in under 3 min (near-instant risk)")
    bullets.append(f"{metrics['never_reported_pct']}% of users never reported phishing")
    timing = metrics['report_timing']
    if timing['count']:
        bullets.append(f"{timing['after_24h_pct']}% of users who reported did so after 24 h")
        bullets.append(f"Median time-to-report: {timing['median_min']} min")
        if timing['within_3min_pct']:
            bullets.append(f"{timing['within_3min_pct']}% of users reported phishing within 3 min")
    elif reported:
        bullets.append(f"Timing data not available for {reported} reporting users")

    # now emit each bullet with the List Bullet style
    for text in bullets:
        para = doc.add_paragraph(text, style='List Bullet')
        para.alignment = WD_ALIGN_PARAGRAPH.LEFT

    doc.add_paragraph()  # spacing


    # — Results Summary per Campaign —
    doc.add_heading("Campaign Results", level=1)

    campaigns = data['campaigns']
    for campaign_index, c in enumerate(campaigns):
        results = c.get('results', [])
        sent = len(results)
        cm = campaign_metrics(c)

        # Mutually exclusive counts (highest action only): they add up to `sent`

        # — Campaign heading —
        doc.add_heading(clean_text(f"Campaign: {c.get('name', 'Unnamed')}"), level=2)
        status_display = campaign_status_display(c)

        # — Campaign Info Table (metadata only) —
        doc.add_heading("Campaign Information", level=3)

        # Build info fields based on campaign type
        is_sms = is_sms_campaign(c)
        is_generic = is_generic_campaign(c)

        # Determine campaign type display
        if is_generic:
            campaign_type_display = "Generic (Landing Page Only)"
        elif is_sms:
            campaign_type_display = "SMS"
        else:
            campaign_type_display = "Email"

        info_fields = [
            ("Campaign ID", str(c.get('id', 'N/A'))),
            ("Campaign Type", campaign_type_display),
            ("Created Date", format_date(c.get('created_date', 'N/A'))),
            ("Launch Date", format_date(c.get('launch_date', 'N/A'))),
            ("Completion Date", format_date(c.get('completed_date', 'N/A'))),
            ("Status", status_display),
        ]

        # Add type-specific fields
        if is_sms:
            # SMS campaign fields
            info_fields.extend([
                ("Sender ID", c.get('sms', {}).get('from', 'N/A')),
                ("SMS Provider", c.get('sms', {}).get('provider', 'N/A')),
            ])
        elif not is_generic:
            # Email campaign fields (skip for Generic campaigns)
            info_fields.extend([
                ("Email Subject", c.get('template_details', {}).get('subject', 'N/A')),
                ("Envelope Sender", c.get('template_details', {}).get('envelope_sender', 'N/A')),
            ])

        # Common fields for both types
        info_fields.extend([
            ("Phishing URL", c.get('phish_url', 'N/A')),
            ("URL Parameter", c.get('urlparam', 'rid')),
            ("Redirect URL", c.get('page_details', {}).get('redirect_url', 'N/A')),
            ("Data Captured", "Yes" if c.get('page_details', {}).get('capture_credentials', 0) else "No"),
            ("Passwords Stored", "Yes" if c.get('page_details', {}).get('capture_passwords', 0) else "No"),
        ])

        info_table = doc.add_table(rows=len(info_fields), cols=2, style='Table Grid')

        for i, (label, val) in enumerate(info_fields):
            info_table.rows[i].cells[0].text = clean_text(label)
            info_table.rows[i].cells[1].text = clean_text(val)

            # Color the left column cell with blue
            left_cell = info_table.rows[i].cells[0]
            set_cell_background(left_cell, '4472C4')

            # Make the text white and bold
            for p in left_cell.paragraphs:
                for run in p.runs:
                    run.font.color.rgb = RGBColor(255, 255, 255)
                    run.bold = True

            # Set the right column to grey
            right_cell = info_table.rows[i].cells[1]
            set_cell_background(right_cell, 'D9D9D9')

        # Set table borders to white
        set_table_borders_color(info_table, border_hex='FFFFFF')

        doc.add_paragraph()  # spacing

        # — Campaign Results Table (matching Overall Campaign Results format) —
        # For Generic campaigns, use event-based counts from timeline instead of unique users
        if is_generic:
            doc.add_heading("Campaign Event Summary", level=3)
            doc.add_paragraph(
                "Generic campaigns count every event (not unique users) since links can be shared "
                "and accessed by multiple anonymous visitors."
            )

            # Count events from timeline for this specific campaign
            generic_clicks = sum(1 for e in c.get('timeline', []) if e.get('message') == 'Clicked Link')
            generic_submissions = sum(1 for e in c.get('timeline', []) if e.get('message') == 'Submitted Data')

            # Create simple 2-column table for Generic campaigns (Metric | Count)
            generic_event_tbl = doc.add_table(rows=4, cols=2, style='Table Grid')

            # Header row
            hdr = generic_event_tbl.rows[0].cells
            hdr[0].text = 'Metric'
            hdr[1].text = 'Count'

            for cell in hdr:
                set_cell_background(cell, '4472C4')
                for p in cell.paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

            # Data rows
            generic_metrics = [
                ("Total Links", sent, '1ABC9C'),
                ("Total Clicks", generic_clicks, 'F39C12'),
                ("Total Submissions", generic_submissions, 'F05B4F'),
            ]

            for i, (label, count, color) in enumerate(generic_metrics, start=1):
                row = generic_event_tbl.rows[i].cells
                row[0].text = label
                set_cell_background(row[0], color)
                for p in row[0].paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

                row[1].text = str(count)
                set_cell_background(row[1], 'E7E6E6')

            set_table_borders_color(generic_event_tbl, border_hex='FFFFFF')
            doc.add_paragraph()  # spacing

            # Skip the standard campaign_row_data table for generic
            campaign_row_data = None
        else:
            doc.add_heading("Campaign Unique Results Summary", level=3)

            # Each recipient counted once, by highest action; adds up to the total
            def campaign_row(label, count):
                return (label, count, pct(count, sent), action_color(label))

            campaign_row_data = [campaign_row("Data Submitted", cm['submitted'])]
            if cm['has_attachments']:
                campaign_row_data.append(campaign_row("Macro Executed", cm['macro_only']))
            campaign_row_data.append(campaign_row("Clicked Link", cm['clicked_only']))
            if cm['has_attachments']:
                campaign_row_data.append(campaign_row("Attachment Opened", cm['attachment_only']))
            if not is_sms:
                campaign_row_data.append(campaign_row("Email Opened", cm['opened_only']))
            campaign_row_data.append(campaign_row("No Further Action", cm['sent_only']))
            if not is_sms:
                campaign_row_data.append(campaign_row("Email Replied", cm['replied']))
                campaign_row_data.append(campaign_row("Reported", cm['reported']))
            campaign_row_data.append(("Total", sent, '', 'D9D9D9'))  # Gray - no percentage for total

        # Only create results table for non-Generic campaigns
        if campaign_row_data is not None:
            # Create table with appropriate number of rows
            results_tbl = doc.add_table(rows=len(campaign_row_data) + 1, cols=3, style='Table Grid')

            # Header row
            hdr = results_tbl.rows[0].cells
            hdr[0].text = 'Category'
            hdr[1].text = 'Users'
            hdr[2].text = 'Percentage'

            # Style header row with blue background and white text
            for cell in hdr:
                set_cell_background(cell, '4472C4')
                for p in cell.paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

            # Add data and style the cells (using the dynamically created campaign_row_data)
            for i, (label, count, percentage, color) in enumerate(campaign_row_data, start=1):
                row = results_tbl.rows[i].cells

                # Category column
                row[0].text = label
                set_cell_background(row[0], color)
                for p in row[0].paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

                # Users column
                row[1].text = str(count)
                set_cell_background(row[1], 'E7E6E6')  # Light gray

                # Percentage column
                row[2].text = f"{percentage}%" if percentage != '' else ''
                set_cell_background(row[2], 'E7E6E6')  # Light gray

            # Set table borders to white
            set_table_borders_color(results_tbl, border_hex='FFFFFF')

            doc.add_paragraph()  # spacing

        # — Campaign Results Summary Table (Cumulative/Hierarchical counts) —
        # Skip this table for Generic campaigns (already shown event counts above)
        if not is_generic:
            doc.add_heading("Campaign Results Summary", level=3)

            # Cumulative counts, as on the campaign results page: each action
            # includes recipients who went further (percentages of all targets)
            def cumulative_row(label, count):
                return (label, count, pct(count, sent), action_color(label))

            cumulative_row_data = [cumulative_row("Data Submitted", cm['submitted'])]
            if cm['has_attachments']:
                cumulative_row_data.append(cumulative_row("Macro Executed", cm['macro_total']))
            cumulative_row_data.append(cumulative_row("Clicked Link", cm['clicked_total']))
            if cm['has_attachments']:
                cumulative_row_data.append(cumulative_row("Attachment Opened", cm['attachment_total']))
            if not is_sms:
                cumulative_row_data.append(cumulative_row("Email Opened", cm['opened_total']))
            cumulative_row_data.append(cumulative_row("SMS Sent" if is_sms else "Email Sent", cm['sent']))
            if cm['errors']:
                cumulative_row_data.append(cumulative_row("Delivery Errors", cm['errors']))
            if not is_sms:
                cumulative_row_data.append(cumulative_row("Email Replied", cm['replied']))
                cumulative_row_data.append(cumulative_row("Reported", cm['reported']))

            # Create table with appropriate number of rows
            cumulative_tbl = doc.add_table(rows=len(cumulative_row_data) + 1, cols=3, style='Table Grid')

            # Header row
            hdr = cumulative_tbl.rows[0].cells
            hdr[0].text = 'Category'
            hdr[1].text = 'Users'
            hdr[2].text = 'Percentage'

            # Style header row with blue background and white text
            for cell in hdr:
                set_cell_background(cell, '4472C4')
                for p in cell.paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

            # Add data and style the cells
            for i, (label, count, percentage, color) in enumerate(cumulative_row_data, start=1):
                row = cumulative_tbl.rows[i].cells

                # Category column
                row[0].text = label
                set_cell_background(row[0], color)
                for p in row[0].paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

                # Users column
                row[1].text = str(count)
                set_cell_background(row[1], 'E7E6E6')  # Light gray

                # Percentage column
                row[2].text = f"{percentage}%" if percentage != '' else ''
                set_cell_background(row[2], 'E7E6E6')  # Light gray

            # Set table borders to white
            set_table_borders_color(cumulative_tbl, border_hex='FFFFFF')

            mfa_text = mfa_summary(cm)
            if mfa_text:
                doc.add_paragraph(mfa_text)

            doc.add_paragraph()  # spacing

        # — User Interaction Summary Table —
        if results:
            # Dynamic heading based on campaign type
            if is_generic:
                doc.add_heading("Link Summary", level=3)
            else:
                doc.add_heading("User Interaction Summary", level=3)

            # Dynamic table structure based on campaign type
            if is_generic:
                # Generic campaigns: Link Name, Total Clicks, Total Submissions (counts per link from timeline)
                num_cols = 3
                tbl = doc.add_table(rows=1, cols=num_cols, style='Table Grid')
                hdr = tbl.rows[0].cells
                hdr[0].text = 'Link Name'
                hdr[1].text = 'Total Clicks'
                hdr[2].text = 'Total Submissions'

                header_colors = [
                    '4472C4',  # Link Name
                    'F39C12',  # Total Clicks
                    'F05B4F',  # Total Submissions
                ]

                for idx, cell in enumerate(hdr):
                    fill = header_colors[idx]
                    set_cell_background(cell, fill)
                    for p in cell.paragraphs:
                        for run in p.runs:
                            run.font.color.rgb = RGBColor(255, 255, 255)
                            run.bold = True

                # Populate rows for Generic campaigns (per-link counts)
                for r in results:
                    row_cells = tbl.add_row().cells
                    # Click/submission events were matched to each link (by RId) in preprocess()
                    row_cells[0].text = clean_text(r.get('first_name') or 'Unknown Link')
                    row_cells[1].text = str(len(r['_click_events']))
                    row_cells[2].text = str(len(r['_submission_events']))

                style_table(tbl,
                        header_fill=None,
                        row_fill='D9D9D9',
                        alt_row_fill='BFBFBF',
                        border_hex='FFFFFF',
                        header_font_hex='FFFFFF')

                doc.add_paragraph()  # spacing

            else:
                # One column per action this campaign type can record, in the
                # app's order; attachment columns only when attachments were opened
                if is_sms:
                    flag_columns = [('Clicked', 'clicked', 'Clicked Link'),
                                    ('Data Submitted', 'submitted_data', 'Data Submitted')]
                else:
                    flag_columns = [('Opened', 'opened', 'Email Opened')]
                    if cm['has_attachments']:
                        flag_columns.append(('Attachment Opened', 'attachment_opened', 'Attachment Opened'))
                    flag_columns.append(('Clicked', 'clicked', 'Clicked Link'))
                    if cm['has_attachments']:
                        flag_columns.append(('Macro Executed', 'macro_executed', 'Macro Executed'))
                    flag_columns += [('Data Submitted', 'submitted_data', 'Data Submitted'),
                                     ('Replied', 'replied', 'Email Replied'),
                                     ('Reported', 'reported', 'Reported')]
                headers = [('Phone Number' if is_sms else 'Email Address', '4472C4')]
                headers += [(title, action_color(label)) for title, _, label in flag_columns]
                headers += [('OS', '4472C4'), ('Browser', '4472C4')]
                tbl = doc.add_table(rows=1, cols=len(headers), style='Table Grid')
                hdr = tbl.rows[0].cells
                for idx, (title, _) in enumerate(headers):
                    hdr[idx].text = title
                header_colors = [color for _, color in headers]

            # Style header row (only for non-generic campaigns, generic already handled above)
            if not is_generic:
                for idx, cell in enumerate(hdr):
                    fill = header_colors[idx]
                    set_cell_background(cell, fill)
                    for p in cell.paragraphs:
                        for run in p.runs:
                            run.font.color.rgb = RGBColor(255, 255, 255)
                            run.bold = True

            # populate rows (only for non-generic campaigns, generic already handled above)
            if not is_generic:
                for r in results:
                    row_cells = tbl.add_row().cells

                    contact = privacy.contact(r)
                    row_cells[0].text = clean_text(contact)
                    for idx, (_, flag, _) in enumerate(flag_columns, start=1):
                        cell = row_cells[idx]
                        cell.text = ""
                        run = cell.paragraphs[0].add_run('✓' if r.get(flag) else '✘')
                        if r.get(flag):
                            run.bold = True
                    row_cells[len(flag_columns) + 1].text = clean_text(extract_os_info(r))
                    row_cells[len(flag_columns) + 2].text = clean_text(extract_browser_info(r))

            style_table(tbl,
                    header_fill=None,
                    row_fill='D9D9D9',
                    alt_row_fill='BFBFBF',
                    border_hex='FFFFFF',
                    header_font_hex='FFFFFF')

            doc.add_paragraph()  # add a blank line for spacing

        # Dynamic heading based on campaign type
        if is_generic:
            doc.add_heading("Per Link Details", level=3)
        else:
            doc.add_heading("Per User Details", level=3)

        for r in results:
            # Only include users/links with any action, MFA event or delivery error
            if not (has_user_interaction(r) or r['_mfa_events'] or r['_error_events']):
                continue

            # Get contact/link field dynamically and apply anonymization
            if is_generic:
                contact = r.get('first_name') or 'Unknown Link'
                contact_label = "Link Name"
                sent_label = "Link Created"
                heading = contact
            else:
                contact = privacy.contact(r)
                contact_label = "Phone Number" if is_sms_result(r) else "Email"
                sent_label = "SMS Sent" if is_sms_result(r) else "Email Sent"
                # Real names are personal data too; only show them when not anonymizing
                heading = contact if privacy.emails else (full_name(r) or contact)
            doc.add_heading(clean_text(heading), level=4)

            send_date = format_date(r.get('send_date'))

            # Geolocate from the real IP, then mask what is displayed
            raw_ip = extract_ip_address(r)
            if raw_ip == 'N/A':
                ip = location = 'N/A'
            else:
                ip = privacy.ip(raw_ip)
                location = privacy.location(raw_ip)
            browser = extract_browser_info(r)
            os_name = extract_os_info(r)

            # Create user summary info table
            summary_tbl = doc.add_table(rows=6, cols=2, style='Table Grid')
            summary_data = [
                (contact_label, clean_text(contact)),
                (sent_label, send_date),
                ("IP Address", ip),
                ("Location", location),
                ("Browser", browser),
                ("Operating System", os_name),
            ]

            for i, (label, value) in enumerate(summary_data):
                summary_tbl.rows[i].cells[0].text = label
                summary_tbl.rows[i].cells[1].text = clean_text(value)

                # Style left column
                left_cell = summary_tbl.rows[i].cells[0]
                set_cell_background(left_cell, '4472C4')
                for p in left_cell.paragraphs:
                    for run in p.runs:
                        run.font.color.rgb = RGBColor(255, 255, 255)
                        run.bold = True

                # Style right column
                right_cell = summary_tbl.rows[i].cells[1]
                set_cell_background(right_cell, 'E7E6E6')

            set_table_borders_color(summary_tbl, border_hex='FFFFFF')
            doc.add_paragraph()  # spacing

            # User Actions Timeline Table
            doc.add_heading("User Activity Timeline" if not is_generic else "Link Activity Timeline", level=5)

            # Collect all actions with timestamps
            actions_list = []

            # For Generic campaigns, show ALL events from the stored event lists
            if is_generic:
                # Every click/submission on this link, as matched in preprocess()
                for event in r['_click_events']:
                    browser_meta = event_browser(event['details'])
                    ip_info = privacy.ip(browser_meta.get('address', ''))
                    ua = browser_meta.get('user-agent', '')
                    detail_text = f"IP: {ip_info}" if ip_info else "Link clicked"
                    if ua:
                        detail_text += f" | {describe_browser(ua)} on {describe_os(ua)}"
                    actions_list.append(("Link Clicked", format_date(event.get('time')), detail_text, 'F39C12'))

                for event in r['_submission_events']:
                    details = event['details']
                    ip_info = privacy.ip(event_browser(details).get('address', ''))
                    submitted = payload_text(details.get('payload'), redact=privacy.emails)
                    parts = [p for p in (f"IP: {ip_info}" if ip_info else '', submitted) if p]
                    detail = '\n'.join(parts) if parts else "Data was submitted"
                    actions_list.append(("Data Submitted", format_date(event.get('time')), detail, 'F05B4F'))

                # Timestamps are uniform 'YYYY-MM-DD HH:MM:SS UTC' strings, so they sort correctly
                actions_list.sort(key=lambda x: x[1])
            else:
                # Every recorded action: opens, attachment opens, macro runs,
                # clicks, submissions, MFA events, replies, reports, errors
                actions_list = [
                    (label, format_date(when), detail or '', action_color(label))
                    for label, when, detail in result_timeline(r, privacy, sms=is_sms)
                    if label not in ('Email Sent', 'SMS Sent')
                ]

            # Create timeline table
            if actions_list:
                timeline_tbl = doc.add_table(rows=len(actions_list) + 1, cols=3, style='Table Grid')

                # Header row
                hdr = timeline_tbl.rows[0].cells
                hdr[0].text = 'Action'
                hdr[1].text = 'Timestamp'
                hdr[2].text = 'Details'

                for cell in hdr:
                    set_cell_background(cell, '4472C4')
                    for p in cell.paragraphs:
                        for run in p.runs:
                            run.font.color.rgb = RGBColor(255, 255, 255)
                            run.bold = True

                # Data rows
                for i, (action, timestamp, detail, color) in enumerate(actions_list, start=1):
                    row = timeline_tbl.rows[i].cells

                    # Action column with color coding
                    row[0].text = action
                    set_cell_background(row[0], color)
                    for p in row[0].paragraphs:
                        for run in p.runs:
                            run.font.color.rgb = RGBColor(255, 255, 255)
                            run.bold = True

                    # Timestamp column
                    row[1].text = timestamp
                    set_cell_background(row[1], 'E7E6E6')

                    # Details column
                    row[2].text = clean_text(detail)
                    set_cell_background(row[2], 'E7E6E6')

                set_table_borders_color(timeline_tbl, border_hex='FFFFFF')

            doc.add_paragraph()  # spacing between users

        # Add page break after each campaign (except the last one)
        if campaign_index < len(campaigns) - 1:
            doc.add_page_break()

    # Add page break before Other Statistics section
    doc.add_page_break()
    doc.add_heading("Other Statistics", level=1)
    # — Time-to-Action Distribution —
    bins = TIME_BINS
    actions = metrics['deltas']

    doc.add_heading("Time-to-Action Distribution", level=2)
    tbl = doc.add_table(rows=1, cols=len(bins)+1, style='Table Grid')
    hdr = tbl.rows[0].cells
    hdr[0].text = "Action"
    for idx,(_,_,label) in enumerate(bins, start=1):
        hdr[idx].text = label

    # Style header row with blue background and white bold text
    for cell in hdr:
        set_cell_background(cell, '4472C4')
        for p in cell.paragraphs:
            for run in p.runs:
                run.font.color.rgb = RGBColor(255, 255, 255)
                run.bold = True

    # Define action colors matching dashboard.js
    action_colors = {
        'Click': 'F39C12',        # Orange (for Clicked Link)
        'Submit Data': 'F05B4F',  # Red (for Submitted Data - from dashboard)
        'Reply': 'E67E22',        # Dark orange (for Email Reply)
        'Report': '45D6EF',       # Light blue (for Report)
        'Open Attachment': action_color('Attachment Opened'),
        'Run Macro': action_color('Macro Executed'),
    }

    # If you don't want "Failure" in the table, filter it out here:
    table_actions = {k: v for k, v in actions.items()
                     if k != 'Failure' and (has_attachments or k not in ('Open Attachment', 'Run Macro'))}

    for action_name, deltas in table_actions.items():
        row = tbl.add_row().cells

        # Set action name with color coding
        row[0].text = action_name
        series_color = action_colors.get(action_name, '4472C4')  # Default to blue if not found
        set_cell_background(row[0], series_color)
        for p in row[0].paragraphs:
            for run in p.runs:
                run.font.color.rgb = RGBColor(255, 255, 255)
                run.bold = True

        # Set percentage values with light gray background
        for idx,(low, high, _) in enumerate(bins, start=1):
            row[idx].text = f"{pct_in_bin(deltas, low, high)}%"
            set_cell_background(row[idx], 'E7E6E6')  # Light gray

    # Set table borders to white
    set_table_borders_color(tbl, border_hex='FFFFFF')

    doc.add_paragraph()  # spacing before next section

    # Browser, OS and IP stats - only for results with an interaction.
    # Generic link results are included: their clicks come from real visitors.
    stats_results = [r for c in campaigns for r in c.get('results', [])]
    browser_counts, os_counts, ip_counts = interaction_stats(stats_results, privacy)

    doc.add_heading("Browser Statistics", level=2)
    tbl = doc.add_table(rows=1, cols=2, style='Table Grid')
    header_cells = tbl.rows[0].cells
    header_cells[0].text, header_cells[1].text = 'Browser', 'Count'
    for b, count in sorted(browser_counts.items(), key=lambda x: -x[1]):
        row = tbl.add_row().cells
        row[0].text, row[1].text = clean_text(b), str(count)
    style_table(tbl)

    doc.add_heading("Operating System Statistics", level=2)
    tbl = doc.add_table(rows=1, cols=2, style='Table Grid')
    header_cells = tbl.rows[0].cells
    header_cells[0].text, header_cells[1].text = 'OS', 'Count'
    for o, count in sorted(os_counts.items(), key=lambda x: -x[1]):
        row = tbl.add_row().cells
        row[0].text, row[1].text = clean_text(o), str(count)
    style_table(tbl)

    # IPs are masked and locations reduced to country level when anonymizing
    doc.add_heading("IP Address and Location Statistics", level=2)
    tbl = doc.add_table(rows=1, cols=3, style='Table Grid')
    header_cells = tbl.rows[0].cells
    header_cells[0].text = 'IP Address'
    header_cells[1].text = 'Location'
    header_cells[2].text = 'Count'
    for ip, entry in sorted(ip_counts.items(), key=lambda x: -x[1]['count']):
        row = tbl.add_row().cells
        row[0].text, row[1].text, row[2].text = clean_text(ip), clean_text(entry['location']), str(entry['count'])
    style_table(tbl)

    # Save
    try:
        doc.save(output_path)
        sys.stderr.write(f"Report saved to {output_path}\n")
        return True
    except Exception as e:
        sys.stderr.write(f"Error saving document: {e}\n")
        return False

# End of script
