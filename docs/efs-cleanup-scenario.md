# EFS Cleanup Scenario

This document explains why we periodically clean up EFS storage, how it differs from AWS Backup retention, and what the cleanup scheduler actually does.

## 1) What we actually want to clean up

- **Only EFS storage (chain data)** is the target.
- Recovery Points (snapshots) are treated as **near‑zero cost** here and are not the primary cleanup target.

## 2) What AWS Backup “Retention” means

- AWS Backup retention controls **how long Recovery Points are kept**.
- It **does not delete the EFS file system itself**.
- So even if retention expires, the **EFS storage can still remain and cost money**.

## 3) Why we need a storage cleanup scheduler

When a restore happens:

- A **new EFS file system** is created.
- If it is not attached and left unused, **storage cost continues**.
- AWS Backup retention **does not remove that EFS**.

Therefore we need a **separate cleanup scheduler** to delete unused EFS.

## 4) Cleanup logic in plain terms

On a fixed schedule (e.g., every 14 days):

1. Find **unused** EFS file systems for a namespace.
2. Keep the **currently in‑use EFS**.
3. Delete only the ones **older than the retention window** (e.g., 30 days).

Retention window is controlled by:

- `TRH_EFS_CLEANUP_RETENTION_DAYS` (default: 14)

## 5) Key takeaway

- **Retention = Recovery Points only.**
- **Cleanup Scheduler = EFS storage cleanup.**
- They solve **different problems** and do not replace each other.
