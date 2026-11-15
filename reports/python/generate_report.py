#!/usr/bin/env python
"""
Main entry point for report generation.
Reads campaign data as JSON on stdin and writes a .docx or .xlsx file.
"""

import argparse
import json
import sys
import traceback

from report_common import configure_geolocation


def main():
    parser = argparse.ArgumentParser(description='Generate campaign reports')
    parser.add_argument('--format', choices=['word', 'excel'], required=True, help='Output format')
    parser.add_argument('--output', required=True, help='Output file path')
    parser.add_argument('--anonymize-emails', action='store_true',
                        help='Mask email addresses, phone numbers, names and submitted values')
    parser.add_argument('--anonymize-ips', action='store_true',
                        help='Mask IP addresses and coordinates; show locations at country level')
    parser.add_argument('--online-geolocation', action='store_true',
                        help='Look up IP locations online (sends target IPs to freeipapi.com)')
    args = parser.parse_args()

    try:
        raw_data = sys.stdin.buffer.read().decode('utf-8')
    except Exception as e:
        sys.stderr.write(f"Error reading input: {e}\n")
        return 1
    if not raw_data:
        sys.stderr.write("Error: No data received from stdin\n")
        return 1
    try:
        data = json.loads(raw_data)
    except json.JSONDecodeError as e:
        # Don't echo the input: it contains personal data
        sys.stderr.write(f"Error parsing JSON input: {e}\n")
        return 1

    if not isinstance(data, dict) or not data.get('campaigns'):
        sys.stderr.write("Error: No campaigns found in data\n")
        return 1

    configure_geolocation(online=args.online_geolocation)
    gdpr_options = {
        'anonymize_emails': args.anonymize_emails,
        'anonymize_ips': args.anonymize_ips,
    }

    # Import lazily so a missing optional library for one format
    # doesn't break the other.
    if args.format == 'word':
        from word_generator import generate_word_document
        success = generate_word_document(data, args.output, gdpr_options=gdpr_options)
    else:
        from excel_generator import generate_excel_document
        success = generate_excel_document(data, args.output, gdpr_options=gdpr_options)

    if not success:
        sys.stderr.write("Report generation failed\n")
        return 1
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        sys.stderr.write(f"Error generating report: {e}\n")
        sys.stderr.write(traceback.format_exc())
        sys.exit(1)
