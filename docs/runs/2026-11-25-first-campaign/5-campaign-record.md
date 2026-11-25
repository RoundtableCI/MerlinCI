Campaign record: Stop Manually Comparing Model Outputs—Automate the Regression Gate in CI/CD

## What was produced
- **Piece**: Technical article with companion GitHub Actions workflow (`.github/workflows/model-regression.yml`) and Python script (`scripts/run_model_comparison.py`), plus config examples (`config/models.yaml`, `datasets/regression.jsonl`)
- **Audience**: Developers running LLM-powered features in production who currently compare model outputs manually via spreadsheets
- **Angle**: Replace manual, non-repeatable spreadsheet-based model output comparison with a CI/CD-native regression gate that runs fan-out evaluation across pinned model versions, applies configurable evaluators (exact match, semantic similarity, LLM-as-a-judge, schema validation, latency, cost), and fails the build when quality thresholds are breached—producing an auditable comparison matrix artifact for every PR
- **Proposed channel**: to be decided

## Status
Prepared for publication, not yet published

## How to measure it
- **UTM campaign name**: `model-regression-gate-ci-cd` (filter by `utm_campaign=model-regression-gate-ci-cd`)
- **Deciding metrics from profile** (applied to this campaign):
  - Signups attributed to the campaign
  - Activation: first successful CI/CD run of the comparison workflow (pipeline executes fan-out → normalize → evaluate → gate → artifact)
  - Pipeline influence: deals where this content was touched before opportunity creation

> Likes, impressions, and number of assets are diagnostic only and must not be used to judge the campaign.

## What cannot be measured yet, and why
- **Traffic, signups, activation, pipeline influence, revenue**: Nothing has been published and no analytics source is connected. The UTM campaign name is defined but not yet live on any channel. No attribution infrastructure exists to connect article reads to signup or activation events.

## Results
No results yet. Nothing has been published and no analytics source is connected.