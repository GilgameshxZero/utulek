#!/usr/bin/env python3
import argparse, json, os, sys
from playwright.sync_api import sync_playwright

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--home', required=True)
    ap.add_argument('--recipients', required=True)
    ap.add_argument('--text', default='')
    ap.add_argument('--files', default='')
    args = ap.parse_args()
    home = args.home
    config = json.load(open(os.path.join(home, 'config.json')))
    cookies = []
    for c in json.load(open(os.path.join(home, 'session.json'))):
        item = {k: c[k] for k in ('name', 'value', 'domain', 'path', 'httpOnly', 'secure') if k in c}
        if c.get('expires', -1) > 0:
            item['expires'] = c['expires']
        if c.get('sameSite') in ('Strict', 'Lax', 'None'):
            item['sameSite'] = c['sameSite']
        cookies.append(item)
    recipients = [r.strip() for r in args.recipients.split(',') if r.strip()]
    files = [f for f in args.files.split(',') if f]
    group_id = ''
    try:
        rows = json.load(open(os.path.join(home, 'messages.json')))
        for m in rows.values():
            participants = set(m.get('participants') or [])
            if m.get('groupId') and participants == set(recipients):
                group_id = m['groupId']
                break
    except Exception:
        pass
    with sync_playwright() as pw:
        browser = pw.chromium.launch(executable_path=config.get('browser') or '/usr/bin/chromium', headless=True, args=['--no-sandbox', '--disable-dev-shm-usage'])
        ctx = browser.new_context()
        ctx.add_cookies(cookies)
        page = ctx.new_page()
        page.goto(f"https://voice.google.com/u/{config.get('account', 0)}/messages", wait_until='domcontentloaded', timeout=60000)
        page.wait_for_timeout(5000)
        if group_id:
            page.locator(f'[data-store-key="{group_id}"]').first.click()
            page.wait_for_timeout(2000)
        else:
            page.click('[aria-label="Send new message"]')
            page.wait_for_timeout(1000)
            field = page.locator('input[placeholder="Type a name or phone number"]')
            for number in recipients:
                field.fill(number)
                page.wait_for_timeout(500)
                page.locator('[role="option"],.send-to-button').filter(has_text=number[-4:]).last.click()
                page.wait_for_timeout(500)
        if files:
            page.click('button[aria-label="Add image"]')
            page.wait_for_timeout(500)
            with page.expect_file_chooser(timeout=10000) as chooser:
                page.get_by_text('Upload', exact=True).first.click()
            chooser.value.set_files(files)
            page.wait_for_timeout(5000)
        if args.text:
            page.locator('gv-message-entry textarea').fill(args.text)
        page.wait_for_timeout(500)
        thread = ''
        source = ''
        with page.expect_response(lambda r: '/voice/v1/voiceclient/api2thread/sendsms' in r.url and r.request.method == 'POST', timeout=30000) as response:
            page.locator('gv-message-entry button[aria-label^="Send message"]').click()
        try:
            body = response.value.json()
            if isinstance(body, list) and len(body) > 2:
                thread = body[1] or ''
                source = body[2] or ''
        except Exception:
            pass
        page.wait_for_timeout(3000)
        print(json.dumps({'ok': True, 'group': bool(group_id), 'thread': thread, 'source': source}))
        browser.close()

if __name__ == '__main__':
    try:
        main()
    except Exception as e:
        print(json.dumps({'ok': False, 'error': str(e)}))
        sys.exit(1)
