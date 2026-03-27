# Lessons

- When translating implementation specs, verify repository path and API shape alignment before treating the document as executable guidance.
- If a source spec contains corrupted or truncated sections, record that explicitly in the translated version instead of silently guessing missing requirements.
- When a repository stores request DTOs as persisted JSON and later rehydrates them across services, implementation specs must call out backward-compatibility risks explicitly.
- If an upstream dependency referenced by a spec does not actually expose the claimed feature in the pinned version, move the source of truth into the local service spec and state that assumption explicitly.
