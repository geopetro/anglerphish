"""
Tests for the report generators.

Run from reports/python:  python -m unittest discover -s tests
The generator smoke tests are skipped when python-docx/openpyxl are missing.
"""

import copy
import json
import os
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

import report_common as rc  # noqa: E402

ZERO = "0001-01-01T00:00:00Z"


def ev(email, message, time, details=None, **extra):
    e = {"email": email, "message": message, "time": time,
         "details": json.dumps(details) if details is not None else ""}
    e.update(extra)
    return e


def browser(ip, ua="Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36"):
    return {"browser": {"address": ip, "user-agent": ua}}


def result(rid, email="", phone="", status="Email Sent", **extra):
    r = {"id": rid, "email": email, "phone": phone, "status": status, "send_date": ZERO,
         "first_name": "Alice", "last_name": "Smith", "position": "CFO", "custom": "",
         "latitude": 0, "longitude": 0, "ip": "", "reported": False, "modified_date": ZERO}
    r.update(extra)
    return r


def sample_data():
    """Two email campaigns sharing a user (different casing), one SMS
    campaign and one generic campaign, with out-of-order events."""
    email_a = {
        "id": 1, "name": "Campaign A", "status": "Completed", "type": "email",
        "results": [
            result("r1", "Alice@Example.com", status="Submitted Data"),
            result("r2", "bob@example.com", status="Email Sent"),
            result("r3", "carol@example.com", status="Scheduled"),  # never sent
            result("r8", "dave@example.com", status="Macro Executed"),
            result("r9", "erin@example.com", status="Error"),  # delivery failed
        ],
        "timeline": [
            # deliberately out of order; the second click must not win
            ev("Alice@Example.com", "Clicked Link", "2026-01-01T10:30:00Z", browser("203.0.113.9")),
            ev("Alice@Example.com", "Email Sent", "2026-01-01T10:00:00Z"),
            ev("Alice@Example.com", "Clicked Link", "2026-01-01T10:05:00Z", browser("203.0.113.9")),
            ev("Alice@Example.com", "Submitted Data", "2026-01-01T10:06:00Z",
               dict(browser("203.0.113.9"), payload={"username": ["=HYPERLINK(\"http://x\")"],
                                                     "password": ["hunter2"]})),
            ev("bob@example.com", "Email Sent", "2026-01-01T10:00:00Z"),
            ev("bob@example.com", "Email Reported", "2026-01-01T10:02:00Z"),
            ev("bob@example.com", "Email Opened", "2026-01-01T10:01:00Z", details=None),
            {"email": "bob@example.com", "message": "Email Opened", "time": "2026-01-01T10:03:00Z",
             "details": "null"},  # details that parse to a non-object
            ev("bob@example.com", "Clicked Link", "2026-01-01T11:00:00Z", details=None,
               details_locked=True),
            ev("Alice@Example.com", "MFA Code Verified", "2026-01-01T10:07:00Z", browser("203.0.113.9")),
            ev("Alice@Example.com", "Proxied request", "2026-01-01T10:08:00Z", browser("203.0.113.9")),
            ev("dave@example.com", "Email Sent", "2026-01-01T10:00:00Z"),
            ev("dave@example.com", "Attachment Opened", "2026-01-01T10:20:00Z", browser("203.0.113.77")),
            ev("dave@example.com", "Macro Executed", "2026-01-01T10:21:00Z",
               dict(browser("203.0.113.77"), payload={"rid": ["r8"], "hostname": ["DAVE-LAPTOP"]})),
            ev("erin@example.com", "Error Sending Email", "2026-01-01T10:00:00Z", {"error": "550 mailbox unavailable"}),
        ],
    }
    email_b = {
        "id": 2, "name": "Campaign B", "status": "In progress", "type": "email",
        "results": [
            result("r4", "alice@example.com", status="Clicked Link"),
            result("r5", "bob@example.com", status="Email Sent"),
        ],
        "timeline": [
            ev("alice@example.com", "Email Sent", "2026-02-01T09:00:00+02:00"),
            ev("alice@example.com", "Clicked Link", "2026-02-01T09:10:00+02:00",
               browser("2001:db8:abcd:12::1")),
            ev("bob@example.com", "Email Sent", "2026-02-01T09:00:00+02:00"),
            ev("bob@example.com", "Email Reported", "2026-02-01T09:30:00+02:00"),
        ],
    }
    sms = {
        "id": 3, "name": "SMS [Test]: one/two", "status": "Completed", "type": "sms",
        "results": [
            result("r6", phone="+30 690 123 4567", sms_target=True, status="Clicked Link"),
            result("r7", phone="+306901112222", sms_target=True, status="SMS Sent"),
        ],
        "timeline": [
            ev("+30 690 123 4567", "SMS Sent", "2026-03-01T08:00:00Z"),
            ev("+30 690 123 4567", "Clicked Link", "2026-03-01T08:01:00Z", browser("198.51.100.7")),
            ev("+306901112222", "SMS Sent", "2026-03-01T08:00:00Z"),
        ],
    }
    generic = {
        "id": 4, "name": "Generic link", "status": "Completed", "type": "generic",
        "results": [result("LINK1", status="Clicked Link", first_name="Poster QR"),
                    result("LINK2", status="Email Sent", first_name="Flyer")],
        "timeline": [
            ev("LINK1", "Clicked Link", "2026-04-01T08:00:00Z", browser("192.0.2.44")),
            ev("LINK1", "Clicked Link", "2026-04-01T08:05:00Z", browser("192.0.2.45")),
            ev("LINK1", "Submitted Data", "2026-04-01T08:06:00Z",
               dict(browser("192.0.2.45"), payload={"email": ["visitor@corp.example"]})),
        ],
    }
    return {"campaigns": [email_a, email_b, sms, generic]}


class CommonTests(unittest.TestCase):
    def setUp(self):
        rc.configure_geolocation(online=False)
        self.data = rc.preprocess(sample_data())
        self.metrics = rc.compute_metrics(self.data)

    def test_first_action_wins_and_events_sorted(self):
        alice = self.data["campaigns"][0]["results"][0]
        self.assertEqual(alice["clicked_time"], "2026-01-01T10:05:00Z")
        deltas = rc.collect_all_deltas([alice])
        self.assertEqual(deltas["Click"], [5.0])

    def test_zero_send_date_is_missing(self):
        carol = self.data["campaigns"][0]["results"][2]
        self.assertNotIn("send_date", carol)
        self.assertEqual(rc.format_date(ZERO), "N/A")

    def test_people_counted_like_the_people_page(self):
        # alice (two casings), bob, dave and two SMS users. carol and erin were
        # never sent anything and generic links are not people.
        m = self.metrics
        self.assertEqual(m["total"], 5)
        self.assertEqual(m["never_sent"], 2)
        self.assertEqual(m["submitted"] + m["macro"] + m["clicked"] + m["attachment"] +
                         m["opened"] + m["sent_only"], m["total"])
        self.assertEqual((m["submitted"], m["macro"], m["clicked"], m["sent_only"]), (1, 1, 2, 1))

    def test_phish_prone_matches_people_page(self):
        # Deliveries: alice A+B, bob A+B, dave A, two SMS = 7. Failed: alice A
        # (submit), alice B (click), bob A (click; its details are locked but
        # it still counts), dave A (macro) and the SMS clicker = 5.
        m = self.metrics
        self.assertEqual((m["failed_deliveries"], m["deliveries"]), (5, 7))
        self.assertEqual(m["ppp"], 71.4)
        self.assertEqual(m["people_failed"], 4)
        self.assertEqual(m["high_risk"], 4)  # bob is 50%, the second SMS user 0%
        self.assertEqual(m["report_rate"], 28.6)

    def test_never_reported_uses_unique_users(self):
        # bob reported in both campaigns but is one person
        self.assertEqual(self.metrics["reported"], 1)
        self.assertEqual(self.metrics["never_reported_pct"], 80)

    def test_attachment_and_macro_are_failures(self):
        dave = self.data["campaigns"][0]["results"][3]
        self.assertTrue(dave["macro_executed"] and dave["attachment_opened"] and dave["opened"])
        self.assertTrue(rc.did_fail(dave))
        self.assertEqual(rc.action_level(dave), rc.LEVEL_MACRO)
        self.assertEqual(dave["macro_payload"], {"rid": ["r8"], "hostname": ["DAVE-LAPTOP"]})
        self.assertEqual(self.metrics["deltas"]["Run Macro"], [21.0])
        self.assertTrue(self.metrics["has_attachments"])

    def test_opening_an_attachment_alone_is_a_failure(self):
        r = {"attachment_opened": True}
        rc.apply_implications(r)
        self.assertTrue(r["opened"])
        self.assertTrue(rc.did_fail(r))
        self.assertFalse(rc.did_fail({"opened": True}))

    def test_delivery_errors_and_mfa(self):
        cm = rc.campaign_metrics(self.data["campaigns"][0])
        self.assertEqual(cm["errors"], 1)
        self.assertEqual(cm["mfa_verified"], 1)
        self.assertIn("1 verified", rc.mfa_summary(cm))
        erin = self.data["campaigns"][0]["results"][4]
        self.assertEqual(len(erin["_error_events"]), 1)
        alice = self.data["campaigns"][0]["results"][0]
        self.assertEqual(len(alice["_proxy_events"]), 1)
        labels = [row[0] for row in rc.result_timeline(alice, rc.Privacy({}))]
        self.assertIn("MFA Code Verified", labels)

    def test_routing_parameters_hidden_from_captured_data(self):
        text = rc.payload_text({"rid": ["x"], "m": ["1"], "a": ["2"], "hostname": ["PC"]})
        self.assertEqual(text, 'hostname: "PC"')

    def test_repeat_offenders_by_normalized_contact(self):
        offenders = self.metrics["repeat_offenders"]
        self.assertEqual(len(offenders), 1)
        self.assertEqual(offenders[0][2], ["Campaign A", "Campaign B"])

    def test_generic_events_matched_by_rid(self):
        link = self.data["campaigns"][3]["results"][0]
        self.assertEqual(len(link["_click_events"]), 2)
        self.assertEqual(len(link["_submission_events"]), 1)
        self.assertEqual(self.metrics["generic_clicks"], 2)

    def test_locked_events_counted(self):
        self.assertEqual(self.data["_locked_events"], 1)
        self.assertIn("1 event(s)", rc.locked_events_notice(self.data))

    def test_progress_uses_sent_count(self):
        self.assertEqual(rc.campaign_status_display(self.data["campaigns"][1]), "In progress (100% – 2/2 sent)")
        c = copy.deepcopy(self.data["campaigns"][0])
        c["status"] = "In progress"
        self.assertEqual(rc.campaign_status_display(c), "In progress (60% – 3/5 sent)")

    def test_campaign_exclusive_counts_add_up(self):
        cm = rc.campaign_metrics(self.data["campaigns"][0])
        self.assertEqual(cm["submitted"] + cm["macro_only"] + cm["clicked_only"] + cm["attachment_only"] +
                         cm["opened_only"] + cm["sent_only"], cm["total"])
        self.assertEqual(cm["sent"], 3)  # carol and erin were never sent
        self.assertEqual(cm["fail_pct"], 100)  # alice, bob and dave, of 3 sent
        self.assertEqual(rc.campaign_metrics(self.data["campaigns"][1])["fail_pct"], 50)

    def test_anonymization(self):
        self.assertEqual(rc.anonymize_email("alice@example.com"), "a***e@example.com")
        self.assertEqual(rc.anonymize_ip("203.0.113.9"), "203.0.113.***")
        self.assertNotIn("12", rc.anonymize_ip("2001:db8:abcd:12::1").split(":")[3:])
        self.assertTrue(rc.anonymize_ip("2001:db8:abcd:12::1").startswith("2001:db8:abcd:"))
        masked = rc.anonymize_phone("+30 690 123 4567")
        self.assertEqual(sum(ch.isdigit() for ch in masked), 4)

    def test_location_never_from_masked_ip_or_zero_coords(self):
        self.assertEqual(rc.get_ip_location("203.0.113.***"), "Unknown")
        self.assertIsNone(rc.result_coordinates({"latitude": 0, "longitude": 0}))
        self.assertEqual(rc.get_ip_location("10.1.2.3"), "Internal Network")

    def test_clean_text_strips_xml_invalid(self):
        self.assertEqual(rc.clean_text("a\x00b\x0bc"), "abc")


def _generator_available(module):
    try:
        __import__(module)
        return True
    except ImportError:
        return False


def _read_zip_text(path):
    with zipfile.ZipFile(path) as z:
        return "".join(z.read(n).decode("utf-8", "ignore") for n in z.namelist() if n.endswith(".xml"))


ANON = {"anonymize_emails": True, "anonymize_ips": True}
LEAKS = ["Alice@Example.com", "bob@example.com", "203.0.113.9", "198.51.100.7", "192.0.2.44",
         "DAVE-LAPTOP", "203.0.113.77",
         "hunter2", "Smith", "CFO", "690 123 4567", "visitor@corp.example", "2001:db8:abcd:12::1"]


@unittest.skipUnless(_generator_available("docx"), "python-docx not installed")
class WordTests(unittest.TestCase):
    def test_word_report_builds_and_hides_personal_data(self):
        from word_generator import generate_word_document
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "r.docx")
            self.assertTrue(generate_word_document(sample_data(), out, gdpr_options=ANON))
            text = _read_zip_text(out)
            for leak in LEAKS:
                self.assertNotIn(leak, text, leak)
            self.assertIn("203.0.113.***", text)

            out2 = os.path.join(tmp, "r2.docx")
            self.assertTrue(generate_word_document(sample_data(), out2, gdpr_options={}))
            text = _read_zip_text(out2)
            self.assertIn("hunter2", text)
            for expected in ("Attachment Opened", "Macro Executed", "DAVE-LAPTOP", "MFA Code Verified",
                             "Error Sending Email", "550 mailbox unavailable", "71.4%"):
                self.assertIn(expected, text)
            self.assertNotIn('rid: "r8"', text)


@unittest.skipUnless(_generator_available("openpyxl"), "openpyxl not installed")
class ExcelTests(unittest.TestCase):
    def test_excel_report_builds_and_hides_personal_data(self):
        from excel_generator import generate_excel_document
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "r.xlsx")
            self.assertTrue(generate_excel_document(sample_data(), out, gdpr_options=ANON))
            text = _read_zip_text(out)
            for leak in LEAKS:
                self.assertNotIn(leak, text, leak)

    def test_excel_lists_every_action(self):
        import openpyxl
        from excel_generator import generate_excel_document
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "r.xlsx")
            self.assertTrue(generate_excel_document(sample_data(), out, gdpr_options={}))
            wb = openpyxl.load_workbook(out)
            events = [row[6] for row in wb["Events-Campaign A"].iter_rows(min_row=4, values_only=True)]
            for expected in ("Attachment Opened", "Macro Executed", "MFA Code Verified",
                             "Proxied request", "Error Sending Email", "Clicked Link"):
                self.assertIn(expected, events)
            self.assertEqual(events.count("Clicked Link"), 3)  # every click, not just the first
            unique = [c.value for c in wb["Per Campaign Unique Results"][3]]
            self.assertIn("Macro Executed", unique)
            self.assertIn("Attachment Opened", unique)
            summary = "".join(str(c) for row in wb["Summary"].iter_rows(values_only=True) for c in row if c)
            self.assertIn("71.4%", summary)

    def test_excel_never_writes_formulas(self):
        import openpyxl
        from excel_generator import generate_excel_document
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "r.xlsx")
            data = sample_data()
            data["campaigns"][0]["name"] = "=1+1"
            data["campaigns"][0]["results"][0]["first_name"] = "=cmd|'/c calc'!A1"
            self.assertTrue(generate_excel_document(data, out, gdpr_options={}))
            with zipfile.ZipFile(out) as z:
                for name in z.namelist():
                    if name.startswith("xl/worksheets/"):
                        self.assertNotIn("<f>", z.read(name).decode("utf-8"), name)
            wb = openpyxl.load_workbook(out)
            names = wb.sheetnames
            self.assertEqual(len(names), len(set(n.lower() for n in names)))
            self.assertTrue(all(len(n) <= 31 for n in names))

    def test_excel_has_no_fake_send_rows(self):
        import openpyxl
        from excel_generator import generate_excel_document
        with tempfile.TemporaryDirectory() as tmp:
            out = os.path.join(tmp, "r.xlsx")
            self.assertTrue(generate_excel_document(sample_data(), out, gdpr_options={}))
            wb = openpyxl.load_workbook(out)
            for ws in wb.worksheets:
                for row in ws.iter_rows(values_only=True):
                    self.assertFalse(any(isinstance(v, str) and v.startswith("0001-01-01") for v in row))


if __name__ == "__main__":
    unittest.main()
