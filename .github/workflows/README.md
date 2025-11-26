# GitHub Actions Workflows

## Promotion Workflows

### Issue: "GitHub Actions is not permitted to create or approve pull requests"

The promotion workflows (`promote-staging.yaml`, `promote-prod.yaml`) fail with this error because of a repository setting.

### Fix: Enable GitHub Actions to Create PRs

1. Go to repository **Settings** → **Actions** → **General**
2. Scroll to **Workflow permissions**
3. Select **"Read and write permissions"**
4. Check **"Allow GitHub Actions to create and approve pull requests"**
5. Click **Save**

### Alternative: Use Personal Access Token (PAT)

If you can't change repository settings, use a PAT:

1. Create a PAT with `repo` scope: https://github.com/settings/tokens
2. Add it as a repository secret named `GH_PAT`
3. Update workflows to use it:

```yaml
env:
  GH_TOKEN: ${{ secrets.GH_PAT }}
```

## Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `build.yaml` | Push to main/staging/prod | Build and push Docker images |
| `comprehensive-tests.yaml` | Push/PR to main/staging/prod | Run all tests |
| `promote-staging.yaml` | Manual | Create PR: main → staging |
| `promote-prod.yaml` | Manual | Create PR: staging → prod |
