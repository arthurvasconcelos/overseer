---
title: prs
weight: 12
---

List open pull requests (GitHub) and merge requests (GitLab) across all configured instances.

```bash
overseer prs
overseer prs --format json
```

Fetches from all configured `integrations.github[]` and `integrations.gitlab[]` instances in parallel.

## JSON output

```json
[
  {
    "source": "github",
    "instance": "personal",
    "items": [
      {
        "number": 42,
        "title": "Add dark mode",
        "repo": "you/myproject",
        "url": "https://github.com/you/myproject/pull/42",
        "draft": false
      }
    ]
  },
  {
    "source": "gitlab",
    "instance": "work",
    "items": [
      {
        "iid": 5,
        "title": "Fix auth middleware",
        "project": "team/api",
        "url": "https://gitlab.company.com/team/api/-/merge_requests/5",
        "draft": false,
        "status": "can_be_merged"
      }
    ]
  }
]
```
