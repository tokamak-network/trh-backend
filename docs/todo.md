# Task Todo

## Checklist

- [x] Compare `docs/PRESET_IMPLEMENTATION_KOR.md` against the current repository structure
- [x] Create an English version of the preset implementation document
- [x] Assess whether the document is sufficient for direct implementation and verification
- [x] Refine the spec into a repository-aligned executable document
- [x] Create a Korean refined implementation spec while preserving the original draft
- [x] Add backend-owned draft preset payloads to the refined English and Korean specs

## Review

- The original draft was partially corrupted and referenced outdated paths.
- The English version preserves the requested feature scope and adds a readiness assessment tied to the current codebase.
- Direct implementation from the draft alone is not safe without route, schema, DTO, and verification clarifications.
- The refined English and Korean specs now map features to real repository files, route groups, persistence points, and verification commands.
- Two product-owned values still remain open: preset payload details from `trh-sdk` and required funding thresholds per network.
- Because the pinned `trh-sdk` version does not contain preset definitions, the refined specs now include backend-owned draft preset payloads instead.
