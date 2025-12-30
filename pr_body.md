feat: Implement Dashboard Pagination and History Limit

Closes #11

## Changes

- **Backend**: `/api/history` now supports `limit`, `cursor`, and `since` query parameters.
- **Backend**: Use cursor-based pagination for efficiency.
- **Backend**: Enforce a 48-hour history limit to optimize performance.
- **Frontend**: Replace full-list fetch with `loadMoreHistory` action in `live.ts`.
- **Frontend**: Add "Load More History" button to `LastHeard.vue` that appends older entries.
- **Frontend**: Preserve existing entries and prevent duplicates when loading more.

## Verification

- Verified backend builds successfully (`go build`).
- Frontend logic implements robust duplicate checking and state management.
