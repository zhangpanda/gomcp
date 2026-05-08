# Traffic Archive

Weekly snapshots of this repository's GitHub Traffic statistics.

## Why this exists

GitHub's Traffic tab (`Insights → Traffic`) only keeps the **last 14 days**
of data — anything older is discarded. This directory preserves weekly
snapshots so that long-term trends (monthly, quarterly, yearly) remain
analyzable.

## How it works

A GitHub Actions workflow (`.github/workflows/traffic-archive.yml`) runs
every Monday at 02:00 UTC (10:00 Beijing). It fetches four endpoints from
the GitHub Traffic API and commits the raw JSON into `docs/traffic/data/`.

You can also trigger it manually from the **Actions** tab
("Traffic archive" → "Run workflow").

## File layout

```
docs/traffic/
├── README.md                         # this file
└── data/
    ├── 2026-05-11-views.json             # 14-day views  (as of snapshot date)
    ├── 2026-05-11-clones.json            # 14-day clones
    ├── 2026-05-11-popular-paths.json     # top referred internal paths
    ├── 2026-05-11-popular-referrers.json # top referring sites
    ├── 2026-05-18-...
    └── ...
```

Each snapshot contains the 14-day window as of the snapshot date. Running
weekly means consecutive snapshots overlap by 7 days — this overlap is
intentional and provides redundancy if a run is missed.

## Data format

### `*-views.json` and `*-clones.json`

```json
{
  "count": 154,
  "uniques": 28,
  "views": [
    { "timestamp": "2026-04-25T00:00:00Z", "count": 12, "uniques": 3 },
    ...
  ]
}
```

For clones, the inner array is named `"clones"` instead of `"views"`.

### `*-popular-paths.json`

```json
[
  { "path": "/zhangpanda/gomcp", "title": "Overview", "count": 49, "uniques": 18 },
  ...
]
```

### `*-popular-referrers.json`

```json
[
  { "referrer": "github.com", "count": 96, "uniques": 7 },
  ...
]
```

See the [GitHub REST API docs](https://docs.github.com/en/rest/metrics/traffic)
for field definitions.

## Permissions

The Traffic API requires **push access** to the repository. The workflow
uses the default `GITHUB_TOKEN` with `contents: write`, which includes
push access and is normally sufficient.

### For this repo (user-owned: `zhangpanda/gomcp`)

No extra setup needed. The default `GITHUB_TOKEN` works.

### Fallback: Personal Access Token

If a workflow run fails on the `Fetch traffic data` step with
`Resource not accessible by integration` or `HTTP 403`, switch to a
Personal Access Token:

1. Go to **GitHub → Settings → Developer settings → Personal access
   tokens → Fine-grained tokens** (recommended) or **Tokens (classic)**.
2. Create a token with:
   - **Fine-grained:** Repository access = `zhangpanda/gomcp`,
     Permissions = `Administration: Read-only`, `Metadata: Read-only`,
     `Contents: Read & write` (to commit).
   - **Classic:** `repo` scope.
3. Add it to this repository's secrets:
   **Settings → Secrets and variables → Actions → New repository secret**
   with the name `TRAFFIC_TOKEN`.

The workflow auto-detects `TRAFFIC_TOKEN` and uses it in preference to
`GITHUB_TOKEN`.

## Analysing the archive

Quick totals across all snapshots:

```bash
jq -s 'map({count, uniques}) | {
  total_views: (map(.count) | add),
  total_uniques_noisy: (map(.uniques) | add)
}' docs/traffic/data/*-views.json
```

> `total_uniques_noisy` double-counts visitors who appear in multiple
> windows. For a more accurate unique count, dedupe by the daily
> `timestamp` entries inside each file.

Daily view timeseries (deduplicated):

```bash
jq -s '[.[].views[]] | unique_by(.timestamp) | sort_by(.timestamp)' \
  docs/traffic/data/*-views.json
```

## Retention

Archives are kept indefinitely. If the directory grows too large, consider
moving old snapshots to a separate branch (`traffic-archive`) or another
repository.
