# Subscription provider source provenance

Cursor support is selectively adapted from
[kaitranntt/CLIProxyAPIPlus](https://github.com/kaitranntt/CLIProxyAPIPlus/tree/8f22abcbc089051073167882157556a90a753c0a)
at commit `8f22abcbc089051073167882157556a90a753c0a`.
The source areas are Cursor authentication/login, protocol encoding and decoding,
HTTP/2 streaming, model discovery and the provider executor. They are integrated
into this repository's existing service registration and execution interfaces.
The Plus application is not bundled or launched as a separate service.

The pinned source uses the MIT license with the Luis Pater and Router-For.ME
copyright notices already preserved in this repository's root LICENSE. Its
[pinned license](https://github.com/kaitranntt/CLIProxyAPIPlus/blob/8f22abcbc089051073167882157556a90a753c0a/LICENSE)
is the source of that attribution.

Local adaptations include the native Claude tool continuation path, MCP state
protocol handling, cancellation ownership, canonical subscription admission,
and sanitized bounded authentication responses. Upstream adoption does not
imply that these changes exist in the source project.

Provider authentication, upstream sessions, transport and canonical quota readers
belong here. TeamClaude owns client session identity, route selection, fallback
and dashboard display. It consumes the authenticated quota-management endpoint;
it does not duplicate provider credential readers.
