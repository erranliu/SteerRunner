# Metadata-only preparation retry

The original public L3 entry completed its new-build L1/framework/Controlled prerequisites. Both case preparations failed before a Run/manifest existed because new-debug-release copied the Runner executable without its build sidecar. Restore the original sidecar for that exact executable, preserving the failed report and partial fixtures. No source, binary, configuration, helper or execution-scheme change is permitted.

The retry validates the original successful prerequisite records, source commit/cleanliness, identical caller and Controlled EXE SHA, original build metadata, configuration/helper/requirements digests and both specific pre-Run failures. It then uses the existing registry, coordinator, leases, worker preparation and evidence validators for two new parallel fixtures. Retain script and report; do not count the original failed batch as passed or claim complete L3. If any guard fails, stop this retry rather than bypassing it.
