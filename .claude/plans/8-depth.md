# Depth config for subtopic listing

## Goal
Add configurable `depth` to `docs.yaml` controlling how many levels of subtopics
are printed. Default: 2. Applied everywhere subtopics are listed (TOC and section
subtopics footer).

## Steps
1. Add `Depth int` to `docsConfig` with yaml tag, default to 2 in `loadConfig`
2. Add `Depth int` to `docsEnv`
3. Create `printTree(w, idx, items, parentSlug, indent, depth)` — single recursive function
4. Refactor `printTOC` and `printSubtopics`/`printSection` to use `printTree`
5. Wire config → docsEnv → showDocs/printTOC/printSection
6. Update golden test files
7. Lint
