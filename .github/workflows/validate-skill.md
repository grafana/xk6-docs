---
on:
  workflow_run:
    workflows: ["Release Doc Bundle"]
    types: [completed]
    branches: [main]
  workflow_dispatch:

permissions:
  contents: read
  issues: read
  pull-requests: read

safe-outputs:
  create-pull-request:
    title-prefix: "[skill] "
    labels: [skill, automated]

tools:
  github:
---

# Validate and Update Agent Skill

After a new k6 doc bundle is released, validate that all lookup paths in `skills/k6-lookup-docs/` still work and that no new modules are missing.

## Steps

1. Build the k6 binary with this extension: `go install go.k6.io/xk6/cmd/xk6@latest && xk6 build --with github.com/grafana/xk6-docs=.`
2. Run the validation script: `./skills/k6-lookup-docs/scripts/validate-paths.sh ./k6`
3. If validation passes, stop — no changes needed.
4. If there are broken paths or uncovered modules:
   - For broken paths: look up the correct path using `./k6 x docs --depth 2` or `./k6 x docs search <term>`, then update the affected reference file in `skills/k6-lookup-docs/references/`.
   - For uncovered modules: create a new reference file with an "I need to..." lookup table and key gotchas (use existing references as templates), then add a row to the workflows table in `skills/k6-lookup-docs/SKILL.md` and to the Module API Quick Reference tables.
   - Never duplicate docs content. Only provide navigation paths and gotchas.
5. Run the validation script again to confirm all paths pass.
6. Open a PR with the fixes.
