# Sources

Provider documents this project was built from (verified 2026-11-12).

- MCP server: `https://api.ibkr.com/v1/api/mcp-public` (streamable HTTP; 401 with `WWW-Authenticate: Bearer resource_metadata=...`)
- Protected resource metadata: `https://api.ibkr.com/v1/api/mcp-public/.well-known/oauth-protected-resource`
- Authorization server metadata: `https://api.ibkr.com/.well-known/oauth-authorization-server`
- IBKR connector FAQ: https://www.interactivebrokers.com/faq?id=1437187133
- Release notes: `https://ibkrguides.com/releasenotes/connector-v1.2.1.htm` (pattern: `connector-vX.Y.Z.htm`); v1.1.5 added the custom connector, v1.2.1 (2026-10-27) Autonomous Trading, saved screeners, earnings insights
- Revoking access: Client Portal > Settings > Manage Third-Party Consents
- MCP specification: https://modelcontextprotocol.io (transport: streamable HTTP; authorization)
- OAuth: RFC 7591 (dynamic client registration), RFC 7636 (PKCE), RFC 8707 (resource indicators), RFC 9207 (`iss` parameter), RFC 9728 (protected resource metadata), RFC 7009 (revocation)
