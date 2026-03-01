#!/usr/bin/env python
"""
Main entry point for report generation.
This script processes arguments and routes to the appropriate generator.
"""

import argparse
import json
import sys
import traceback

from word_generator import generate_word_document
from excel_generator import generate_excel_document

def main():
    """Parse arguments and generate the appropriate report format"""
    parser = argparse.ArgumentParser(description='Generate campaign reports')
    parser.add_argument('--format', choices=['word', 'excel'], required=True,
                      help='Output format')
    parser.add_argument('--output', required=True,
                      help='Output file path')
    parser.add_argument('--anonymize-emails', action='store_true',
                      help='Anonymize email addresses')
    parser.add_argument('--anonymize-ips', action='store_true',
                      help='Anonymize IP addresses')
    parser.add_argument('--include-gdpr-statement', action='store_true',
                      help='Include GDPR compliance statement')
    parser.add_argument('--include-toc', action='store_true',
                      help='Include table of contents (Word only)')
    # Excel options removed as requested
    
    args = parser.parse_args()
    
    # Read JSON data from stdin with error handling
    try:
        raw_data = sys.stdin.buffer.read().decode('utf-8')
        sys.stderr.write(f"Received data of length: {len(raw_data)} bytes\n")
        if not raw_data:
            sys.stderr.write("Error: No data received from stdin\n")
            return 1
            
        try:
            data = json.loads(raw_data)
        except json.JSONDecodeError as e:
            sys.stderr.write(f"Error parsing JSON: {str(e)}\n")
            sys.stderr.write(f"Raw data preview: {raw_data[:200]}...\n")
            return 1
    except Exception as e:
        sys.stderr.write(f"Unexpected error reading input: {str(e)}\n")
        sys.stderr.write(traceback.format_exc())
        return 1
        
    # Validate data structure
    if not isinstance(data, dict):
        sys.stderr.write(f"Error: Expected dictionary but got {type(data)}\n")
        return 1
        
    if 'campaigns' not in data:
        sys.stderr.write("Error: Missing 'campaigns' key in data\n")
        sys.stderr.write(f"Available keys: {', '.join(data.keys())}\n")
        return 1
        
    if not data.get('campaigns'):
        sys.stderr.write("Error: No campaigns found in data\n")
        return 1
    
    # Build GDPR options to pass to generators
    # Generators handle anonymization internally with context-aware methods
    gdpr_options = {
        'anonymize_emails': args.anonymize_emails,
        'anonymize_ips': args.anonymize_ips,
        # Automatically include statement if any anonymization is enabled
        'include_statement': args.anonymize_emails or args.anonymize_ips 
    }
    
    # Generate the appropriate document with error handling
    try:
        sys.stderr.write(f"Generating {args.format} report to {args.output}\n")
        
        if args.format == 'word':
            success = generate_word_document(data, args.output, include_toc=args.include_toc, gdpr_options=gdpr_options)
        else:
            success = generate_excel_document(
                data,
                args.output,
                gdpr_options=gdpr_options
            )
            
        if not success:
            sys.stderr.write("Report generation failed\n")
            return 1
            
        sys.stderr.write("Report generation completed successfully\n")
        return 0
    except Exception as e:
        sys.stderr.write(f"Error generating report: {str(e)}\n")
        sys.stderr.write(traceback.format_exc())
        return 1

if __name__ == "__main__":
    try:
        exit_code = main()
        sys.exit(exit_code)
    except Exception as e:
        sys.stderr.write(f"Unhandled error in main: {str(e)}\n")
        sys.stderr.write(traceback.format_exc())
        sys.exit(1)
