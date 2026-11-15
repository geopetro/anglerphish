#!/usr/bin/env python
"""
Shared helpers and metrics for the Anglerphish Word and Excel report generators.

Both generators call preprocess() once and then compute_metrics(), so every
number in a Word report matches the same number in an Excel report.
"""

import datetime
import functools
import ipaddress
import json
import os
import re
import statistics
import sys
import urllib.request

# ---------------------------------------------------------------------------
# Event / status names (must match models/models.go)
# ---------------------------------------------------------------------------
EVENT_EMAIL_SENT = 'Email Sent'
EVENT_SMS_SENT = 'SMS Sent'
SENT_EVENTS = (EVENT_EMAIL_SENT, EVENT_SMS_SENT)
EVENT_OPENED = 'Email Opened'
EVENT_ATTACHMENT = 'Attachment Opened'
EVENT_MACRO = 'Macro Executed'
EVENT_CLICKED = 'Clicked Link'
EVENT_SUBMITTED = 'Submitted Data'
EVENT_REPLIED = 'Email Replied'
EVENT_REPORTED = 'Email Reported'
EVENT_SEND_ERROR = 'Error Sending Email'
EVENT_SMS_ERROR = 'Error Sending SMS'
SEND_ERROR_EVENTS = (EVENT_SEND_ERROR, EVENT_SMS_ERROR)
EVENT_MFA_SENT = 'MFA Code Sent'
EVENT_MFA_SEND_ERROR = 'MFA Code Send Error'
EVENT_MFA_VERIFIED = 'MFA Code Verified'
EVENT_MFA_FAILED = 'MFA Code Failed'
MFA_EVENTS = (EVENT_MFA_SENT, EVENT_MFA_SEND_ERROR, EVENT_MFA_VERIFIED, EVENT_MFA_FAILED)
EVENT_PROXY = 'Proxied request'

COMPLETE_STATUSES = ('Complete', 'Completed')
# Result statuses the app counts as a delivery error (applyResultFlags)
ERROR_STATUSES = ('Error', 'Retrying')

# People with a phish-prone % at or above this are high risk (people.js)
HIGH_RISK_PCT = 40

# Action levels for the mutually exclusive "highest action" breakdown,
# ranked the way the app ranks a result's status (models/result.go)
(LEVEL_SENT, LEVEL_OPENED, LEVEL_ATTACHMENT, LEVEL_CLICKED,
 LEVEL_MACRO, LEVEL_SUBMITTED) = 1, 2, 3, 4, 5, 6

# Form parameters the app hides when showing captured data
HIDDEN_PAYLOAD_KEYS = ('rid', 'm', 'a')

# Time-to-action buckets in minutes: (low exclusive, high inclusive, label)
TIME_BINS = [
    (float('-inf'), 3, "< 3 min"),
    (3, 15, "3–15 min"),
    (15, 60, "15–60 min"),
    (60, 240, "1–4 h"),
    (240, 1440, "4–24 h"),
    (1440, float('inf'), "> 24 h"),
]

# ---------------------------------------------------------------------------
# Optional libraries
# ---------------------------------------------------------------------------
try:
    from user_agents import parse as _parse_user_agent
    USER_AGENTS_AVAILABLE = True
except ImportError:
    USER_AGENTS_AVAILABLE = False
    sys.stderr.write("Warning: user_agents library not available. Install with: pip install user-agents\n")

try:
    import geoip2.database
    GEOIP_AVAILABLE = True
except ImportError:
    GEOIP_AVAILABLE = False

# ---------------------------------------------------------------------------
# Text helpers
# ---------------------------------------------------------------------------
_SURROGATE_RE = re.compile(r'[\ud800-\udfff]')
# Characters that are not allowed in XML 1.0 (Word and Excel files are XML)
_XML_INVALID_RE = re.compile(r'[\x00-\x08\x0b\x0c\x0e-\x1f￾￿]')


def clean_text(s):
    """Return a string that is safe to put into a .docx/.xlsx file."""
    if s is None:
        return ''
    if not isinstance(s, str):
        s = str(s)
    s = _SURROGATE_RE.sub('', s)
    return _XML_INVALID_RE.sub('', s)


def pct(part, whole, digits=0):
    """Percentage of part in whole, rounded; 0 when whole is 0."""
    if not whole:
        return 0
    value = round(part / whole * 100, digits)
    return int(value) if digits == 0 or float(value).is_integer() else value


# ---------------------------------------------------------------------------
# Dates
# ---------------------------------------------------------------------------
_TZ_RE = re.compile(r'([+-]\d{2}:?\d{2})$')


def parse_iso(s):
    """Parse a Go/ISO timestamp into a timezone-aware datetime.

    Returns None for empty values, Go's zero time (0001-01-01) and
    anything that can't be parsed."""
    if not s or not isinstance(s, str) or s == 'N/A':
        return None
    s = s.strip()
    if s.startswith('0001-01-01'):
        return None
    tz = ''
    if s.endswith('Z'):
        s = s[:-1]
        tz = '+00:00'
    else:
        m = _TZ_RE.search(s)
        if m and 'T' in s:
            tz = m.group(1)
            s = s[:-len(tz)]
            if ':' not in tz:
                tz = tz[:3] + ':' + tz[3:]
    if '.' in s:
        date_part, frac = s.split('.', 1)
        frac = ''.join(ch for ch in frac if ch.isdigit())[:6].ljust(6, '0')
        s = f"{date_part}.{frac}"
    try:
        dt = datetime.datetime.fromisoformat(s + tz)
    except ValueError:
        return None
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=datetime.timezone.utc)
    return dt


def format_date(value):
    """Format a timestamp as 'YYYY-MM-DD HH:MM:SS UTC'.

    All report timestamps are shown in UTC so values from different sources
    can be compared directly."""
    dt = parse_iso(value) if isinstance(value, str) else value
    if isinstance(dt, datetime.datetime):
        return dt.astimezone(datetime.timezone.utc).strftime('%Y-%m-%d %H:%M:%S UTC')
    if not value or (isinstance(value, str) and value.startswith('0001-01-01')):
        return 'N/A'
    return str(value)


def generated_on():
    """Timestamp line for the report header."""
    return f"Generated on: {format_date(datetime.datetime.now(datetime.timezone.utc))}"


# ---------------------------------------------------------------------------
# Campaign / result classification
# ---------------------------------------------------------------------------
def is_sms_campaign(campaign):
    return campaign.get('type') == 'sms'


def is_generic_campaign(campaign):
    """Generic campaigns are landing-page-only links with no email/phone."""
    return campaign.get('type') == 'generic'


def is_sms_result(result):
    if result.get('sms_target'):
        return True
    return bool(result.get('phone')) and not result.get('email')


def get_contact_field(result):
    """The contact identifier (email or phone) for a result."""
    if is_sms_result(result):
        return result.get('phone') or 'Unknown'
    return result.get('email') or 'Unknown'


def contact_key(result):
    """The identity a person is grouped by, exactly as the People page does
    (personKey): the lowercased email, falling back to the phone number.
    Returns None when the result has neither (generic links)."""
    email = (result.get('email') or '').strip().lower()
    if email:
        return email
    phone = (result.get('phone') or '').strip()
    return phone or None


def full_name(result):
    return ' '.join(p for p in (result.get('first_name'), result.get('last_name')) if p).strip()


# ---------------------------------------------------------------------------
# Anonymization
# ---------------------------------------------------------------------------
REDACTED = '[REDACTED]'


def anonymize_phone(phone):
    """Mask all digits except the first two and the last two."""
    if not phone or phone == 'Unknown':
        return phone
    digit_positions = [i for i, ch in enumerate(phone) if ch.isdigit()]
    if len(digit_positions) < 5:
        return '*' * len(phone)
    keep = set(digit_positions[:2] + digit_positions[-2:])
    return ''.join(ch if (not ch.isdigit() or i in keep) else '*' for i, ch in enumerate(phone))


def anonymize_email(email):
    """Mask the local part of an email address, keeping its first and last character."""
    if not email or email == 'Unknown' or '@' not in email:
        return email
    local, domain = email.split('@', 1)
    if len(local) <= 2:
        return '*' * len(local) + '@' + domain
    return local[0] + '*' * (len(local) - 2) + local[-1] + '@' + domain


def anonymize_contact(result, contact):
    return anonymize_phone(contact) if is_sms_result(result) else anonymize_email(contact)


def anonymize_ip(ip):
    """Mask the host part of an IP: last octet for IPv4, everything after /48 for IPv6."""
    if not ip or ip in ('Unknown', 'N/A'):
        return ip
    try:
        addr = ipaddress.ip_address(ip)
    except ValueError:
        return REDACTED
    if addr.version == 4:
        return '.'.join(ip.split('.')[:3]) + '.***'
    network = ipaddress.ip_network(f"{addr}/48", strict=False)
    prefix = network.network_address.exploded.split(':')[:3]
    return ':'.join(h.lstrip('0') or '0' for h in prefix) + ':****:****:****:****:****'


class Privacy:
    """The anonymization choices for one report."""

    def __init__(self, gdpr_options=None):
        opts = gdpr_options or {}
        self.emails = bool(opts.get('anonymize_emails'))
        self.ips = bool(opts.get('anonymize_ips'))

    @property
    def enabled(self):
        return self.emails or self.ips

    def contact(self, result, contact=None):
        contact = contact if contact is not None else get_contact_field(result)
        return anonymize_contact(result, contact) if self.emails else contact

    def personal(self, value):
        """Names, positions and other free-text personal fields."""
        if self.emails and value not in (None, '', 'N/A'):
            return REDACTED
        return value

    def ip(self, ip):
        return anonymize_ip(ip) if self.ips else ip

    def location(self, ip):
        """Location for a raw IP; only the country when IPs are anonymized."""
        return get_ip_location(ip, coarse=self.ips)

    def coordinates(self, result):
        """Latitude/longitude pair, or None when unknown or anonymized."""
        if self.ips:
            return None
        return result_coordinates(result)


# ---------------------------------------------------------------------------
# Event details, browser and IP
# ---------------------------------------------------------------------------
def parse_details(raw):
    """Event details as a dict, whatever form they arrive in."""
    if isinstance(raw, dict):
        return raw
    if isinstance(raw, str) and raw.strip():
        try:
            parsed = json.loads(raw)
        except ValueError:
            return {}
        return parsed if isinstance(parsed, dict) else {}
    return {}


def first_value(value):
    """Form values arrive as lists (Go url.Values); return the first one."""
    if isinstance(value, list):
        return value[0] if value else ''
    return value


def payload_text(payload, redact=False, separator='\n'):
    """Render a submitted payload as 'key: "value"' lines."""
    if not isinstance(payload, dict):
        return ''
    lines = []
    for key, value in payload.items():
        if key in HIDDEN_PAYLOAD_KEYS:
            continue  # routing/marker parameters, not captured data
        shown = REDACTED if redact else first_value(value)
        lines.append(f'{key}: "{shown}"')
    return separator.join(lines)


def event_browser(details):
    browser = details.get('browser') if isinstance(details, dict) else None
    return browser if isinstance(browser, dict) else {}


INTERACTION_FLAGS = ('opened', 'attachment_opened', 'clicked', 'macro_executed',
                     'submitted_data', 'replied', 'reported')


def has_user_interaction(result):
    return any(result.get(f) for f in INTERACTION_FLAGS)


def _ua_parts(ua):
    if not ua:
        return None
    if not USER_AGENTS_AVAILABLE:
        return None
    try:
        return _parse_user_agent(ua)
    except Exception:
        return None


def describe_browser(ua):
    parsed = _ua_parts(ua)
    if parsed:
        return f"{parsed.browser.family} {parsed.browser.version_string}".strip()
    return ua[:60] if ua else 'Unknown'


def describe_os(ua):
    parsed = _ua_parts(ua)
    if parsed:
        return f"{parsed.os.family} {parsed.os.version_string}".strip()
    return 'Unknown'


def extract_user_agent(result):
    return event_browser({'browser': result.get('browser')}).get('user-agent')


def extract_browser_info(result, check_interaction=True):
    if check_interaction and not has_user_interaction(result):
        return 'N/A'
    return describe_browser(extract_user_agent(result))


def extract_os_info(result, check_interaction=True):
    if check_interaction and not has_user_interaction(result):
        return 'N/A'
    return describe_os(extract_user_agent(result))


def extract_ip_address(result, check_interaction=True):
    if check_interaction and not has_user_interaction(result):
        return 'N/A'
    return result.get('ip') or 'Unknown'


def result_coordinates(result):
    """(lat, lon) recorded by Gophish, or None when it was never set (0, 0)."""
    lat, lon = result.get('latitude'), result.get('longitude')
    if lat in (None, '') or lon in (None, ''):
        return None
    try:
        if float(lat) == 0 and float(lon) == 0:
            return None
    except (TypeError, ValueError):
        return None
    return lat, lon


# ---------------------------------------------------------------------------
# IP geolocation
# ---------------------------------------------------------------------------
_GEO = {'online': False, 'reader': None, 'online_failed': False}


def _default_geoip_paths():
    here = os.path.dirname(os.path.abspath(__file__))
    return [
        os.environ.get('ANGLERPHISH_GEOIP_DB', ''),
        'GeoLite2-City.mmdb',
        os.path.join(here, 'GeoLite2-City.mmdb'),
        os.path.join(os.path.expanduser('~'), 'GeoLite2-City.mmdb'),
        '/usr/share/GeoIP/GeoLite2-City.mmdb',
        '/usr/local/share/GeoIP/GeoLite2-City.mmdb',
    ]


def configure_geolocation(online=False):
    """Set up IP geolocation for this run.

    The local GeoLite2 database is always used when present. The online
    lookup sends target IPs to a third party, so it only runs when the
    administrator enabled it explicitly."""
    _GEO['online'] = bool(online)
    _GEO['online_failed'] = False
    _GEO['reader'] = None
    get_ip_location.cache_clear()
    if GEOIP_AVAILABLE:
        path = next((p for p in _default_geoip_paths() if p and os.path.isfile(p)), None)
        if path:
            try:
                _GEO['reader'] = geoip2.database.Reader(path)
            except Exception as e:
                sys.stderr.write(f"Warning: could not open GeoIP database {path}: {e}\n")


def is_private_ip(ip):
    try:
        addr = ipaddress.ip_address(ip)
    except (ValueError, TypeError):
        return False
    return addr.is_private or addr.is_loopback or addr.is_link_local


def _online_lookup(ip):
    if not _GEO['online'] or _GEO['online_failed']:
        return None
    try:
        with urllib.request.urlopen(f"https://freeipapi.com/api/json/{ip}", timeout=3) as resp:
            data = json.loads(resp.read().decode('utf-8'))
    except Exception as e:
        # Stop trying for the rest of this report instead of waiting on every IP
        _GEO['online_failed'] = True
        sys.stderr.write(f"Warning: online IP geolocation disabled for this report: {e}\n")
        return None
    return (data.get('cityName') or '', data.get('regionName') or '', data.get('countryName') or '')


@functools.lru_cache(maxsize=4096)
def get_ip_location(ip, coarse=False):
    """Location for an IP; with coarse=True only the country is returned."""
    if not ip or ip in ('Unknown', 'N/A'):
        return 'Unknown'
    try:
        ipaddress.ip_address(ip)
    except ValueError:
        return 'Unknown'
    if is_private_ip(ip):
        return 'Internal Network'

    city = region = country = ''
    reader = _GEO['reader']
    if reader is not None:
        try:
            resp = reader.city(ip)
            city, country = resp.city.name or '', resp.country.name or ''
        except Exception:
            pass
    if not (city or country):
        found = _online_lookup(ip)
        if found:
            city, region, country = found

    parts = (country,) if coarse else (city, region, country)
    location = ', '.join(p for p in parts if p)
    return location or 'Unknown'


# ---------------------------------------------------------------------------
# Preprocessing
#
# Flags follow the app exactly (models/campaign.go: eventRecipientFlags,
# applyResultFlags, applyRecipientImplications), so a report never disagrees
# with the dashboard, campaign results, campaign sets or People pages.
# ---------------------------------------------------------------------------
# Event message -> flags it sets (eventRecipientFlags)
EVENT_FLAGS = {
    EVENT_EMAIL_SENT: ('sent',),
    EVENT_SMS_SENT: ('sent',),
    EVENT_OPENED: ('opened',),
    EVENT_ATTACHMENT: ('attachment_opened', 'opened'),
    EVENT_MACRO: ('macro_executed',),
    EVENT_CLICKED: ('clicked',),
    EVENT_SUBMITTED: ('submitted_data',),
    EVENT_REPLIED: ('replied',),
    EVENT_REPORTED: ('reported',),
}

# Event message -> result field holding the time of its first occurrence
EVENT_TIME_FIELDS = {
    EVENT_OPENED: 'opened_time',
    EVENT_ATTACHMENT: 'attachment_time',
    EVENT_MACRO: 'macro_time',
    EVENT_CLICKED: 'clicked_time',
    EVENT_SUBMITTED: 'submitted_time',
    EVENT_REPLIED: 'replied_time',
    EVENT_REPORTED: 'reported_time',
}

FLAG_NAMES = ('sent', 'opened', 'attachment_opened', 'clicked', 'macro_executed',
              'submitted_data', 'replied', 'reported', 'error')


def apply_implications(f):
    """applyRecipientImplications: submitting implies clicking and opening;
    clicking or replying implies opening; running a macro implies opening
    the attachment, which implies opening the email. Reported stands alone."""
    if f.get('submitted_data'):
        f['clicked'] = True
        f['opened'] = True
    if f.get('clicked'):
        f['opened'] = True
    if f.get('replied'):
        f['opened'] = True
    if f.get('macro_executed'):
        f['attachment_opened'] = True
    if f.get('attachment_opened'):
        f['opened'] = True


def _event_sort_key(indexed_event):
    index, event = indexed_event
    dt = parse_iso(event.get('time'))
    return (dt is None, dt or datetime.datetime.min.replace(tzinfo=datetime.timezone.utc), index)


def _apply_event(result, event, details):
    """Record an event on a result. The first occurrence of each action wins,
    so time-to-action metrics measure the first click/submit/etc."""
    msg = event.get('message')
    ts = event.get('time')

    for flag in EVENT_FLAGS.get(msg, ()):
        result[flag] = True
    if msg in SENT_EVENTS:
        result.setdefault('send_date', ts)
    elif msg in EVENT_TIME_FIELDS:
        result.setdefault(EVENT_TIME_FIELDS[msg], ts)

    if msg == EVENT_SUBMITTED and 'payload' not in result and isinstance(details.get('payload'), dict):
        result['payload'] = details['payload']
    elif msg == EVENT_MACRO and 'macro_payload' not in result and isinstance(details.get('payload'), dict):
        result['macro_payload'] = details['payload']

    lists = {
        EVENT_CLICKED: '_click_events', EVENT_SUBMITTED: '_submission_events',
        EVENT_ATTACHMENT: '_attachment_events', EVENT_MACRO: '_macro_events',
    }
    if msg in lists:
        result[lists[msg]].append(event)
    elif msg in MFA_EVENTS:
        result['_mfa_events'].append(event)
    elif msg in SEND_ERROR_EVENTS:
        result['_error_events'].append(event)
    elif msg == EVENT_PROXY:
        result['_proxy_events'].append(event)

    # Browser/IP: keep the most recent one seen on a click, submission or
    # macro run, falling back to any event (e.g. an email open).
    browser = event_browser(details)
    if browser and (msg in (EVENT_CLICKED, EVENT_SUBMITTED, EVENT_MACRO) or 'browser' not in result):
        result['browser'] = browser
        if browser.get('address'):
            result['ip'] = browser['address']


def preprocess(data):
    """Normalize campaign data in place and attach per-result action flags.

    - Go zero times (0001-01-01) become missing values.
    - Timeline events are sorted by time and their details parsed once.
    - Flags are set exactly as the app sets them, including the result
      row's own reported/replied/error state and the implications.
    - Generic campaign events are matched to their link by RId.
    - Events whose details could not be decrypted are counted in
      data['_locked_events'] so the report can warn about missing data."""
    locked = 0
    for campaign in data.get('campaigns', []):
        results = campaign.get('results') or []
        campaign['results'] = results
        generic = is_generic_campaign(campaign)

        for r in results:
            # Go always serializes send_date; the zero value means "not sent".
            # Event-derived send times replace it below.
            r.pop('send_date', None)
            # applyResultFlags: the result row records reported/replied itself
            reported, replied = bool(r.get('reported')), bool(r.get('replied'))
            for flag in FLAG_NAMES:
                r[flag] = False
            r['reported'], r['replied'] = reported, replied
            r['error'] = r.get('status') in ERROR_STATUSES
            for name in ('_click_events', '_submission_events', '_attachment_events', '_macro_events',
                         '_mfa_events', '_error_events', '_proxy_events'):
                r[name] = []

        lookup = {}
        for r in results:
            if generic:
                # Generic link events carry the link's RId in the 'email' field
                if r.get('id'):
                    lookup[str(r['id'])] = r
                if r.get('email'):
                    lookup.setdefault(r['email'], r)
            else:
                # SMS events carry the phone number in the 'email' field
                for key in (r.get('email'), r.get('phone')):
                    if key:
                        lookup.setdefault(key, r)
                        lookup.setdefault(key.strip().lower(), r)

        timeline = campaign.get('timeline') or []
        ordered = [e for _, e in sorted(enumerate(timeline), key=_event_sort_key)]
        campaign['timeline'] = ordered

        for event in ordered:
            if event.get('details_locked'):
                locked += 1
            details = parse_details(event.get('details'))
            event['details'] = details
            contact = event.get('email') or ''
            result = lookup.get(contact) or lookup.get(contact.strip().lower())
            if result is None:
                continue
            event["_result"] = result
            _apply_event(result, event, details)

        for r in results:
            apply_implications(r)
        campaign['_sent_count'] = sum(1 for r in results if r['sent'])
    data['_locked_events'] = locked
    return data


# ---------------------------------------------------------------------------
# Metrics
# ---------------------------------------------------------------------------
def action_level(result):
    """Highest action, ranked the way the app ranks a result's status:
    sent < opened < attachment opened < clicked < macro executed < submitted."""
    if result.get('submitted_data'):
        return LEVEL_SUBMITTED
    if result.get('macro_executed'):
        return LEVEL_MACRO
    if result.get('clicked'):
        return LEVEL_CLICKED
    if result.get('attachment_opened'):
        return LEVEL_ATTACHMENT
    if result.get('opened'):
        return LEVEL_OPENED
    return LEVEL_SENT


def did_reply(result):
    return bool(result.get('replied'))


def did_report(result):
    return bool(result.get('reported'))


def did_fail(result):
    """personFailed: clicking, submitting data, opening the attachment,
    running a macro or replying. Opening the email alone is not a failure."""
    return any(result.get(f) for f in ('clicked', 'submitted_data', 'attachment_opened',
                                        'macro_executed', 'replied'))


def people_pct(part, whole):
    """peoplePct: a 0-100 percentage rounded to one decimal."""
    if not whole:
        return 0
    value = round(part * 1000 / whole) / 10
    return int(value) if float(value).is_integer() else value


# Time-to-action series: (name, result time field, counts as a failure)
DELTA_SERIES = (
    ('Click', 'clicked_time', True),
    ('Submit Data', 'submitted_time', True),
    ('Open Attachment', 'attachment_time', True),
    ('Run Macro', 'macro_time', True),
    ('Reply', 'replied_time', True),
    ('Report', 'reported_time', False),
)


def collect_all_deltas(results):
    """Minutes from delivery to each first action, per action type."""
    series = {name: [] for name, _, _ in DELTA_SERIES}
    series['Failure'] = []
    for r in results:
        delivered = parse_iso(r.get('send_date'))
        if delivered is None:
            continue
        failures = []
        for name, field, is_failure in DELTA_SERIES:
            t = parse_iso(r.get(field))
            if t is None:
                continue
            minutes = (t - delivered).total_seconds() / 60
            if minutes < 0:
                continue  # action recorded before this delivery; not a valid delta
            series[name].append(minutes)
            if is_failure:
                failures.append(minutes)
        if failures:
            series['Failure'].append(min(failures))
    return series


def pct_within(values, threshold):
    return pct(sum(1 for v in values if v <= threshold), len(values))


def pct_in_bin(values, low, high):
    return pct(sum(1 for v in values if low < v <= high), len(values))


def campaign_has_attachments(campaign):
    return any(r.get('attachment_opened') for r in campaign.get('results') or [])


def campaign_metrics(campaign):
    """Counts for one campaign, matching the campaign results page:
    cumulative counts per action over all targets, plus mutually exclusive
    "highest action" counts that add up to the total. Failure and report
    rates are over the recipients actually sent the message, as on the
    People page."""
    results = campaign.get('results') or []
    total = len(results)
    m = {'total': total, 'generic': is_generic_campaign(campaign), 'sms': is_sms_campaign(campaign)}

    if m['generic']:
        timeline = campaign.get('timeline') or []
        m['clicks'] = sum(1 for e in timeline if e.get('message') == EVENT_CLICKED)
        m['submissions'] = sum(1 for e in timeline if e.get('message') == EVENT_SUBMITTED)
        m['fail_pct'] = pct(m['clicks'], total, 2)
        return m

    count = lambda flag: sum(1 for r in results if r.get(flag))  # noqa: E731
    m['sent'] = count('sent')
    m['errors'] = count('error')
    m['opened_total'] = count('opened')
    m['attachment_total'] = count('attachment_opened')
    m['clicked_total'] = count('clicked')
    m['macro_total'] = count('macro_executed')
    m['submitted'] = count('submitted_data')
    m['replied'] = count('replied')
    m['reported'] = count('reported')
    m['has_attachments'] = m['attachment_total'] > 0

    levels = [action_level(r) for r in results]
    m['macro_only'] = levels.count(LEVEL_MACRO)
    m['clicked_only'] = levels.count(LEVEL_CLICKED)
    m['attachment_only'] = levels.count(LEVEL_ATTACHMENT)
    m['opened_only'] = levels.count(LEVEL_OPENED)
    m['sent_only'] = levels.count(LEVEL_SENT)

    sent_results = [r for r in results if r.get('sent')]
    m['failed'] = len({contact_key(r) or id(r) for r in sent_results if did_fail(r)})
    reported_sent = len({contact_key(r) or id(r) for r in sent_results if did_report(r)})
    m['fail_pct'] = people_pct(m['failed'], m['sent'])
    m['report_pct'] = people_pct(reported_sent, m['sent'])

    mfa = [e.get('message') for r in results for e in r['_mfa_events']]
    m['mfa_sent'] = mfa.count(EVENT_MFA_SENT)
    m['mfa_verified'] = mfa.count(EVENT_MFA_VERIFIED)
    m['mfa_failed'] = mfa.count(EVENT_MFA_FAILED)
    m['mfa_errors'] = mfa.count(EVENT_MFA_SEND_ERROR)
    m['has_mfa'] = bool(mfa)
    return m


def campaign_status_display(campaign):
    """Status text; in-progress campaigns show how many messages went out."""
    status = campaign.get('status') or 'N/A'
    if status in COMPLETE_STATUSES or is_generic_campaign(campaign):
        return status
    total = len(campaign.get('results') or [])
    if not total:
        return status
    sent = campaign.get('_sent_count', 0)
    return f"{status} ({pct(sent, total)}% – {sent}/{total} sent)"


def compute_metrics(data):
    """Report-wide metrics, defined as on the People page:

    - People are keyed by personKey (lowercased email, else phone) and only
      campaigns actually sent to them count.
    - Phish-prone % and report rate are over (person, campaign) deliveries,
      rounded to one decimal, like the People page's overall figures.
    - High-risk people have a personal phish-prone % of 40 or more.

    Generic (anonymous link) campaigns are counted separately as events,
    because their results are links, not people."""
    m = {
        'has_email': False, 'has_sms': False, 'has_generic': False, 'in_progress': False,
        'has_attachments': False, 'has_mfa': False,
        'generic_links': 0, 'generic_clicks': 0, 'generic_submissions': 0,
        'delivery_errors': 0, 'never_sent': 0,
    }
    # person key -> campaign id -> merged flags for that delivery
    deliveries = {}
    display = {}
    user_results = []

    for c in data.get('campaigns', []):
        if c.get('status') not in COMPLETE_STATUSES:
            m['in_progress'] = True
        results = c.get('results') or []
        if is_generic_campaign(c):
            m['has_generic'] = True
            m['generic_links'] += len(results)
            for e in c.get('timeline') or []:
                if e.get('message') == EVENT_CLICKED:
                    m['generic_clicks'] += 1
                elif e.get('message') == EVENT_SUBMITTED:
                    m['generic_submissions'] += 1
            continue
        if is_sms_campaign(c):
            m['has_sms'] = True
        else:
            m['has_email'] = True

        cid = c.get('id')
        for r in results:
            m['delivery_errors'] += 1 if r.get('error') else 0
            m['has_attachments'] |= bool(r.get('attachment_opened'))
            m['has_mfa'] |= bool(r['_mfa_events'])
            key = contact_key(r)
            if key is None:
                continue
            if not r.get('sent'):
                m['never_sent'] += 1
                continue
            user_results.append(r)
            display.setdefault(key, (r, get_contact_field(r)))
            flags = deliveries.setdefault(key, {}).setdefault(cid, {'_name': c.get('name') or 'Unnamed'})
            for flag in FLAG_NAMES:
                flags[flag] = flags.get(flag, False) or bool(r.get(flag))

    pairs_sent = pairs_failed = pairs_reported = 0
    levels, failed_campaigns = [], {}
    union_counts = dict.fromkeys(('opened', 'attachment_opened', 'clicked', 'macro_executed',
                                  'submitted_data', 'replied', 'reported'), 0)
    high_risk = never_reported = 0
    for key, campaigns in deliveries.items():
        union = {}
        person_failed = person_reported = 0
        for flags in campaigns.values():
            pairs_sent += 1
            if did_fail(flags):
                person_failed += 1
                failed_campaigns.setdefault(key, []).append(flags['_name'])
            if did_report(flags):
                person_reported += 1
            for flag in FLAG_NAMES:
                union[flag] = union.get(flag, False) or flags[flag]
        pairs_failed += person_failed
        pairs_reported += person_reported
        levels.append(action_level(union))
        for flag in union_counts:
            union_counts[flag] += 1 if union.get(flag) else 0
        if people_pct(person_failed, len(campaigns)) >= HIGH_RISK_PCT:
            high_risk += 1
        if not person_reported:
            never_reported += 1

    total = len(deliveries)
    m.update({
        'total': total,
        # Highest action per person (mutually exclusive; adds up to total)
        'submitted': levels.count(LEVEL_SUBMITTED),
        'macro': levels.count(LEVEL_MACRO),
        'clicked': levels.count(LEVEL_CLICKED),
        'attachment': levels.count(LEVEL_ATTACHMENT),
        'opened': levels.count(LEVEL_OPENED),
        'sent_only': levels.count(LEVEL_SENT),
        # People who did each action at least once (cumulative)
        'ever': union_counts,
        'replied': union_counts['replied'],
        'reported': union_counts['reported'],
        'people_failed': len(failed_campaigns),
        'deliveries': pairs_sent,
        'failed_deliveries': pairs_failed,
        'ppp': people_pct(pairs_failed, pairs_sent),
        'report_rate': people_pct(pairs_reported, pairs_sent),
        'high_risk': high_risk,
        'never_reported_pct': people_pct(never_reported, total),
    })

    m['repeat_offenders'] = sorted(
        ((display[k][0], display[k][1], sorted(set(names)))
         for k, names in failed_campaigns.items() if len(set(names)) > 1),
        key=lambda row: (-len(row[2]), row[1]))

    deltas = collect_all_deltas(user_results)
    m['deltas'] = deltas
    reps = deltas['Report']
    m['report_timing'] = {
        'count': len(reps),
        'after_24h_pct': pct(sum(1 for d in reps if d > 1440), len(reps)),
        'median_min': round(statistics.median(reps)) if reps else None,
        'within_3min_pct': pct_within(reps, 3),
    }
    m['instant_failure_pct'] = pct_within(deltas['Failure'], 3)
    m['user_results'] = user_results
    return m


def interaction_stats(results, privacy):
    """Browser, OS and IP counts for results with any interaction.

    IPs are geolocated from the real address before masking, and the
    location is reduced to the country when IPs are anonymized."""
    browsers, systems, ips = {}, {}, {}
    for r in results:
        if not has_user_interaction(r):
            continue
        b = extract_browser_info(r, check_interaction=False)
        o = extract_os_info(r, check_interaction=False)
        browsers[b] = browsers.get(b, 0) + 1
        systems[o] = systems.get(o, 0) + 1
        raw_ip = extract_ip_address(r, check_interaction=False)
        shown = privacy.ip(raw_ip)
        entry = ips.setdefault(shown, {'count': 0, 'location': privacy.location(raw_ip)})
        entry['count'] += 1
    return browsers, systems, ips


def event_detail_text(event, privacy):
    """One line describing an event for timelines: IP, browser and any
    captured values (redacted when anonymizing)."""
    details = event.get('details') or {}
    msg = event.get('message')
    parts = []
    browser = event_browser(details)
    if browser.get('address'):
        parts.append(f"IP: {privacy.ip(browser['address'])}")
    if browser.get('user-agent'):
        ua = browser['user-agent']
        parts.append(f"{describe_browser(ua)} on {describe_os(ua)}")
    if msg in SEND_ERROR_EVENTS or msg == EVENT_MFA_SEND_ERROR:
        if details.get('error'):
            parts.append(f"Error: {details['error']}")
    captured = payload_text(details.get('payload'), redact=privacy.emails, separator=', ')
    if captured:
        parts.append(f"Data: {captured}")
    return ' | '.join(parts)


# Timeline rows per result: (event message, label, result event-list field)
TIMELINE_EVENT_LISTS = (
    (EVENT_ATTACHMENT, 'Attachment Opened', '_attachment_events'),
    (EVENT_MACRO, 'Macro Executed', '_macro_events'),
)


def result_timeline(result, privacy, sms=False):
    """Every recorded action for one result as (label, time, details) rows,
    sorted by time. Repeated clicks and submissions are listed once (first),
    matching the per-action timestamps used in the metrics; attachment
    opens, macro runs, MFA events and delivery errors are listed in full."""
    rows = []
    sent_label = 'SMS Sent' if sms else 'Email Sent'
    if result.get('send_date'):
        rows.append((sent_label, result['send_date'], ''))
    if result.get('opened_time'):
        rows.append(('Email Opened', result['opened_time'], 'User viewed the email'))
    for msg, label, field in TIMELINE_EVENT_LISTS:
        for e in result.get(field, []):
            rows.append((label, e.get('time'), event_detail_text(e, privacy)))
    if result.get('clicked_time'):
        first = result['_click_events'][0] if result.get('_click_events') else {}
        rows.append(('Link Clicked', result['clicked_time'],
                     event_detail_text(first, privacy) or 'User clicked the phishing link'))
    if result.get('submitted_time'):
        first = result['_submission_events'][0] if result.get('_submission_events') else {}
        rows.append(('Data Submitted', result['submitted_time'],
                     event_detail_text(first, privacy) or 'Data was submitted'))
    for e in result.get('_mfa_events', []):
        rows.append((e.get('message'), e.get('time'), event_detail_text(e, privacy)))
    if result.get('replied_time'):
        rows.append(('Email Replied', result['replied_time'], 'User replied to the phishing email'))
    if result.get('reported_time'):
        rows.append(('Phish Reported', result['reported_time'], 'User reported the phishing attempt'))
    for e in result.get('_error_events', []):
        rows.append((e.get('message'), e.get('time'), event_detail_text(e, privacy)))
    rows.sort(key=lambda row: format_date(row[1]))
    return rows


def locked_events_notice(data):
    count = data.get('_locked_events', 0)
    if not count:
        return None
    return (f"{count} event(s) could not be decrypted (the encryption key was unavailable). "
            "Their submitted data, IP and browser details are missing from this report.")


# Colours the app uses for each action (campaign_results.js), as hex without '#'
ACTION_COLORS = {
    'Email Sent': '1ABC9C', 'SMS Sent': '1ABC9C', 'No Further Action': '1ABC9C',
    'Email Opened': 'F9BF3B',
    'Attachment Opened': 'F0AD4E',
    'Clicked Link': 'F39C12', 'Link Clicked': 'F39C12',
    'Macro Executed': 'C0392B',
    'Data Submitted': 'F05B4F', 'Submitted Data': 'F05B4F',
    'Email Replied': 'E67E22',
    'Email Reported': '45D6EF', 'Phish Reported': '45D6EF', 'Reported': '45D6EF',
    'Error Sending Email': '6C7A89', 'Error Sending SMS': '6C7A89', 'Delivery Errors': '6C7A89',
    'MFA Code Sent': '9B59B6', 'MFA Code Verified': '27AE60',
    'MFA Code Send Error': 'D35400', 'MFA Code Failed': 'E74C3C',
    'Proxied request': '6C7A89',
}


def action_color(label, default='4472C4'):
    return ACTION_COLORS.get(label, default)


def mfa_summary(cm):
    """One sentence summarizing a campaign's MFA events, or None."""
    if not cm.get('has_mfa'):
        return None
    return (f"MFA codes: {cm['mfa_sent']} sent, {cm['mfa_verified']} verified (captured), "
            f"{cm['mfa_failed']} failed, {cm['mfa_errors']} could not be sent.")
