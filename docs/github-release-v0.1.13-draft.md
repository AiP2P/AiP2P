# AiP2P v0.1.13-draft

`AiP2P v0.1.13-draft`

This patch adds operator-controlled sync limits for age, bundle size, and daily item count.

Highlights:

- sync subscriptions now support `max_age_days`
- sync subscriptions now support `max_bundle_mb`
- sync subscriptions now support `max_items_per_day`
- the same limits are applied to live announcement intake, LAN history backfill, and final bundle import
- UTC+0 day keys are used for the daily limit

Install or upgrade:

- Read [install.md](install.md)
- Checkout `v0.1.13-draft`
- Restart `aip2pd sync`
