# ProxyMorph Public Deployment Security Review

Date: 2026-05-29

This document records security items found before exposing ProxyMorph to the public internet. It is intentionally action-oriented so the fixes can be planned later.

## Release Recommendation

Do not expose the current service directly to the public internet without a reverse proxy and HTTPS. It is acceptable for private use, but public deployment should first complete the high-priority hardening items below.

## High Priority

1. Default admin credentials
   - Current behavior: first boot can create `admin/admin` when `PROXYMORPH_ADMIN_PASSWORD` is empty.
   - Risk: accidental public deployment with default credentials.
   - Recommended fix: require a non-empty strong admin password in production, or refuse to start when public mode is enabled and the password is empty.

2. Login rate limiting
   - Current behavior: `/api/login` has no IP or account rate limit.
   - Risk: brute-force attacks against the admin account.
   - Recommended fix: add per-IP and per-username throttling, temporary lockout after repeated failures, and generic error responses.

3. Session lifetime and cookie flags
   - Current behavior: the session token has no expiry and the cookie does not set `Secure`.
   - Risk: stolen cookies stay valid indefinitely; cookies can be sent over plain HTTP.
   - Recommended fix: include expiry in signed sessions, set `MaxAge`, support `Secure` when deployed behind HTTPS, and consider session rotation after password changes.

4. CSRF protection for admin APIs
   - Current behavior: authenticated write APIs rely on cookies and `SameSite=Lax`.
   - Risk: cross-site form or fetch-style attacks in edge cases.
   - Recommended fix: add CSRF tokens or enforce trusted `Origin`/`Referer` for non-GET admin API requests.

5. Subscription source fetching
   - Current behavior: task source URLs are fetched server-side without timeout, response size limit, or private-network blocking.
   - Risk: SSRF, hanging requests, and memory pressure from large responses.
   - Recommended fix: allow only `http`/`https`, add client timeout, limit response size, and block loopback/private/link-local destinations after DNS resolution.

## Medium Priority

1. Public subscription token lifecycle
   - Current behavior: `/sub/{token}` is intentionally public and token-based, but tokens cannot be rotated from the UI.
   - Recommended fix: add a regenerate-token action and document that subscription URLs are secrets.

2. Error detail exposure
   - Current behavior: authenticated admin APIs often return raw `err.Error()`.
   - Recommended fix: return user-safe errors from APIs and keep detailed diagnostics in logs or a dedicated admin detail view.

3. Docker deployment template
   - Current behavior: `docker-compose.yml` contains example weak secrets and publishes admin and relay ports directly.
   - Recommended fix: move secrets to `.env`, document domain + HTTPS reverse proxy deployment, and avoid exposing admin HTTP directly.

4. Managed config URL scheme
   - Current behavior: request-host based managed config URLs can be generated as `http://...`.
   - Recommended fix: require `PROXYMORPH_PUBLIC_BASE_URL=https://your-domain` for public deployment and prefer that value over request host.

## HTTPS/IP Deployment Note

Browsers only expose the modern Clipboard API in secure contexts. Plain `http://server-ip:28888` can break copy actions. For public deployment, use a domain name with a trusted TLS certificate through a reverse proxy. For direct IP access, HTTPS is usually not enough unless the certificate is trusted for that IP address.
