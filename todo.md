# Query API Implementation TODO

- [x] Add a record query method that filters by `prefix`, optional `schema`, and optional time range.
- [x] Expose `GET /query` in the REST handler with parameter validation and limits.
- [x] Advertise `net.concrnt.core.query` in `/.well-known/concrnt`.
- [x] Verify ordering and limits match CIP-3 defaults and bounds.
