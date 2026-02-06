# PV/PVC Backup and Recovery Guide

The **PV/PVC Backup** feature provided by the Thanos Rollup Hub UI is a tool designed to safely save the current state of Kubernetes resources before modifying storage settings (Attach) or upgrading the rollup stack.

If an unexpected issue occurs during operations, such as storage disconnection or resource deletion, you can use the downloaded backup file to restore the previous state.

## 📦 Backup File Contents

When you unzip the downloaded `.zip` file, you will find the following files inside a timestamped folder (formatted `YYYYMMDD-HHMMSS`):

| File/Folder Name | Description |
|---|---|
| `pv_pvc-xxxx.yaml` | **PersistentVolume (PV)** definition file. Contains information directly linked to the storage (EFS). |
| `pvc_op-geth-xx.yaml` | **PersistentVolumeClaim (PVC)** definition file. Specification for Pods requesting storage. |
| `sts_op-geth.yaml` | **StatefulSet** definition file. Deployment configuration for workloads like `op-geth`, `op-node`. |
| `storageclasses.yaml` | Storage class information for the cluster. |
| `summary.txt` | A text file summarizing PV, PVC names, and linked EFS ID information at the time of backup. |

---

## 🛠️ Recovery Procedure (Restore)

If storage fails to attach or PVCs are lost after an operation, follow these steps to restore them.

### Step 1: Unzip and Verify Files
Unzip the downloaded zip file.
```bash
unzip pvpvc-backup-xxxx.zip
cd <unzipped_directory>
```

### Step 2: Clean Up YAML Files (Important!)
Since the backed-up YAML files are dumps of the current state running in Kubernetes, **they contain system-managed fields (`uid`, `resourceVersion`, `status`, etc.)**. Applying them as-is may cause errors.

It is recommended to open `pv_*.yaml` and `pvc_*.yaml` files with a text editor and **delete** the following fields:
*   `metadata.uid`
*   `metadata.resourceVersion`
*   `metadata.creationTimestamp`
*   `metadata.managedFields` (entire block)
*   `status` (entire block at the end of the file)

### Step 3: Clean Up Existing Resources (If Necessary)
If there are problematic resources (e.g., PVCs stuck in Pending state), delete them first.
> **Warning:** To prevent actual data (EFS) deletion when deleting a PV, ensure the `ReclaimPolicy` is `Retain` or check for `persistentVolumeReclaimPolicy: Retain` in the backup file's `pv_*.yaml`.

```bash
# Example of deleting problematic resources
kubectl -n <namespace> delete pvc <pvc-name>
kubectl delete pv <pv-name>
```

### Step 4: Restore Resources
Apply the YAML files to the cluster in the following order.

**1. Restore PV (PersistentVolume)**
```bash
kubectl apply -f pv_pvc-xxxx.yaml
```

**2. Restore PVC (PersistentVolumeClaim)**
```bash
kubectl -n <namespace> apply -f pvc_op-geth-xx.yaml
```

**3. Restore StatefulSet (Workload) (Optional)**
If you need to revert Pod configurations as well, apply the StatefulSet.
```bash
kubectl -n <namespace> apply -f sts_op-geth.yaml
```

### Step 5: Verify Status
Check if all resources are created and connected correctly.

```bash
# Check if PVC is Bound
kubectl -n <namespace> get pvc

# Check if Pods are Running
kubectl -n <namespace> get pods
```

---

## ⚠️ Important Notes
*   **Verify EFS ID**: Ensure the `csi.volumeHandle` value in `pv_*.yaml` matches the actual EFS ID (`fs-xxxx`) you intend to connect.
*   **Match Namespace**: When running commands, ensure the target Namespace is correct (`-n <namespace>`).
