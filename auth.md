# Authentication (OAuth) Strategy

Warp Panel uses GitHub OAuth for interactive logins and a signed session cookie
for API access. The API accepts either the session cookie or a bearer token with
the same signed payload.

## OAuth login flow
1) The UI opens `/auth/login` (usually via a popup window).
2) The API generates a random state value, stores it in the `warp_oauth_state`
   cookie (5 minute TTL), and redirects to GitHub OAuth.
3) GitHub redirects back to `/auth/callback` with `code` and `state`.
4) The API validates the state cookie, exchanges the code for a token, and calls
   the GitHub API (`read:user` scope) to fetch the user profile.
5) Access is allowed if the user is on the `GITHUB_ALLOWED_USERS` list or is a
   member of `GITHUB_ALLOWED_ORG`. If no allowlist is configured, any GitHub user
   is accepted.
6) The user record is upserted in the database, and a signed session cookie
   `warp_session` is set. The response redirects to `/`.

## Session model
- Cookie name: `warp_session`.
- Payload: `userId`, `login`, `avatarUrl`, `expiresAt`.
- Encoding: base64 JSON + HMAC-SHA256 signature using `SESSION_SECRET`.
- TTL: `SESSION_TTL_HOURS` (default 12 hours).
- Cookies are `HttpOnly` with `SameSite=Lax`. `Secure` is enabled when
  `APP_ENV=prod`. `COOKIE_DOMAIN` is honored when set.

## API auth behavior
- `GET /auth/me` returns the current session profile or `401` if unauthenticated.
- `POST /auth/logout` clears the session cookie.
- `/api/v1/*` routes are protected by middleware that checks:
  1) `warp_session` cookie, then
  2) `Authorization: Bearer <token>` (same signed session format).

## Callback URL handling
- `GITHUB_CALLBACK_URL` is the default redirect URI.
- If the configured callback points to `localhost` but the request host is not
  local, the API derives the callback URL from `X-Forwarded-Host`/`Host` and
  `X-Forwarded-Proto` so OAuth works behind the nginx proxy.

## Optional admin test token
- If `ADMIN_LOGIN` and `ADMIN_PASSWORD` are set, `POST /test-token` issues a
  bearer token (signed session) for scripts or diagnostics.
- This endpoint is disabled when the admin credentials are not set.

## Nginx routing
- The proxy forwards `/auth/*` and `/test-token` to the API container.
- The UI calls `/auth/login` directly and polls `/auth/me` to detect a completed
  login.
