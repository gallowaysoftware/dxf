# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-08-27

Initial release.

### Added
- DXF R2010 (AC1024) writer with no dependencies outside the standard library.
- Entities: `LINE`, `CIRCLE`, `ARC`, `LWPOLYLINE` with bulge arcs, and `TEXT`
  with style, justification and rotation.
- Layers with AutoCAD Color Index colours, and named text styles.
- `WithUnits` for the `$INSUNITS` header variable, with `$MEASUREMENT` derived
  from it.
- Validation before any bytes are written: undefined layers, undefined text
  styles, empty names, and line breaks in string values are reported as errors.
- Deterministic output — no timestamps, no generated UUIDs — so files can be
  committed and diffed.
