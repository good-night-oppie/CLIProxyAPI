# Sanitized production configurations

These Europa and Windows WSL snapshots use the curated v7 production schema. API keys and management secrets are replaced with `SET_LOCALLY`. No auth-directory contents or OAuth token files are included. Put each live configuration and credential directory outside the Git checkout; replace placeholders locally before running the service.

The production branch preserves the local Antigravity Claude tool-result ordering fix, refuses requests when all client tools disappear during conversion, and provides optional payload-shape diagnostics without logging request content.

Essential upstream fixes backported onto the deployed major version:

- `4337f1f0`: escape access-log request paths to prevent log injection.
- `154d5144`: strip unsupported readOnly/writeOnly schema annotations while retaining legitimate properties of those names. The tests cover every cleaner present in this v7 tree.

The latest upstream v8 baseline is preserved separately when initializing the production fork; it is not the binary selected for this deployment. The production services on both hosts execute the same Linux amd64 build. Windows hosts the services inside Ubuntu WSL.
