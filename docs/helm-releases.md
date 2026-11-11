# Helm releases: ApplyHelmChart and Meshery Operator deployment

`utils/kubernetes` applies Helm charts for the ecosystem. Meshery Server installs Meshery
Operator through `models/controllers` -> `applyOperatorHelmChart` -> `(*Client).ApplyHelmChart`
with `Action: INSTALL` and `UpgradeIfInstalled: true` (release `meshery-operator`, namespace
`meshery`). Model dependents go through `InstallHelmChartOnce` instead (see its Go doc); this page
covers `ApplyHelmChart`.

## Release state decides the action

With `INSTALL` and `UpgradeIfInstalled`, the **latest revision** of the release (Helm's
`Releases.Last`, which reads the full history in the namespace the action configuration is bound
to) decides what happens. Implementation: `planInstallOrUpgrade` and `recoverAbandonedRelease` in
`utils/kubernetes/helm-release-state.go`.

| Latest revision | Result |
| --- | --- |
| none | install |
| `deployed`, `failed`, `superseded` | upgrade (Helm upgrades from the last deployed revision, or from the failed one when none was ever deployed) |
| `uninstalled` (history kept) | install with `Replace` |
| `pending-install`, `pending-upgrade`, `pending-rollback`, started less than `PendingReleaseTimeout` ago | `ErrHelmReleasePending`; nothing changes |
| the same, started longer ago | the revision is marked `failed` (description says it was abandoned), then the release is upgraded |
| the same, longer ago, but `DryRun` | `ErrHelmReleasePending`; a dry run never writes the release record |
| `uninstalling`, or a status MeshKit does not know | `ErrHelmReleaseState`; nothing changes |

`PendingReleaseTimeout` defaults to `DefaultPendingReleaseTimeout` (10 minutes, twice Helm's
default operation timeout). The start of a pending operation is the revision's
`Info.LastDeployed`; a time in the future (clock skew) counts as just started.

### Why "mark failed, then upgrade" for an abandoned pending revision

A pending revision whose operation died (Meshery Server restarted mid-install, for example) blocks
the release forever: Helm's install refuses the name and Helm's upgrade refuses a pending release.
The alternatives considered:

- **Uninstall, then install.** Deletes every resource of the release (and, for a chart that ships
  CRDs as templates, every custom resource of those kinds) and takes the operator down. Rejected.
- **Roll back to the last deployed revision, then upgrade.** Two operations and two revisions to
  reach the requested chart, and impossible for a pending first install, which has no deployed
  revision to roll back to.
- **Mark the revision failed, then upgrade (chosen).** This is exactly what Helm records when an
  operation fails, and Helm's upgrade already knows how to continue from a failed revision, with
  or without an earlier deployed one. It deletes nothing, takes one operation, and leaves the
  abandoned revision in `helm history` with a description saying why.

An `uninstalling` release is not recovered: completing the uninstall deletes the release's
resources, which an install must not do on its own. The error tells the operator to finish it
with `helm uninstall`.

### Concurrency

Calls for one release (cluster host, namespace, name) are serialized within the process by the
same per-release lock `InstallHelmChartOnce` uses, and a chart source is downloaded under its own
lock. Across processes (several Meshery Server replicas, the Helm CLI), Helm's release record is
the only coordination, which is why a recently started pending revision is reported rather than
recovered.

### Other behavior

- `UPGRADE` runs against `cfg.ReleaseName` (the chart name was used before, which upgraded the
  wrong release whenever the two differed).
- `UPGRADE` keeps `MaxHistory` revisions, `DefaultHelmMaxHistory` (10, the Helm CLI default) when
  unset; a negative value keeps all of them. Without a limit, every redeploy added a Secret to the
  release namespace forever.
- `UNINSTALL` with a `ReleaseName` does not fetch the chart, so it works when the chart repository
  is unreachable.
- A downloaded chart is written to a temporary file and renamed into place, because an existing
  file at the download path is reused as is: a truncated download used to be reused forever.

## Errors

Each failure is reported once, with the code for what failed, so callers can map it to a cause
and remedy that fit:

| Code symbol (`utils/kubernetes`) | Raised when |
| --- | --- |
| `ErrHelmChartFetchCode` | the chart could not be located, downloaded or loaded |
| `ErrHelmClusterUnreachableCode` | `IsReachable` failed (API server unreachable, credentials rejected) |
| `ErrHelmReleaseStorageCode` | the action configuration could not be built, or the release record could not be read or written |
| `ErrHelmReleasePendingCode` | see the table above |
| `ErrHelmReleaseStateCode` | see the table above |
| `ErrApplyHelmChartCode` | Helm's install, upgrade or uninstall itself failed |

`models/controllers` turns them into Meshery Operator errors (`classifyOperatorHelmError`):

| Helm error | Operator error |
| --- | --- |
| `ErrHelmReleasePending`, `ErrHelmReleaseState` | `ErrOperatorHelmReleaseState` |
| `ErrHelmClusterUnreachable`, `ErrHelmReleaseStorage` | `ErrOperatorClusterAccess` |
| `ErrHelmChartFetch` | `ErrOperatorChartFetch` |
| anything else, on deploy / undeploy | `ErrDeployController` / `ErrUndeployController` |

`Deploy` refused while the operator Deployment is still rolling out returns
`ErrOperatorDeploymentInProgress`. Every one of these errors is a `NewV3` error whose causes
and remediations describe that failure only; the wrapped error contributes its long description
(the instance text), never its own probable cause and remediation. Before this,
`ErrDeployController` blamed kubeconfig file permissions for every failure, including Helm
release-state conflicts.

## Tests

`utils/kubernetes/helm-release-state_test.go` runs Helm's real actions against Helm's in-memory
storage driver (`driver.NewMemory`) and a fake Kubernetes client, for every release state above,
and checks the resulting history. `models/controllers/error_test.go` covers the classification
and the operator's `Deploy`/`Undeploy` error paths without a cluster.
