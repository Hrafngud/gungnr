## Next Task

Objective
Fix create-from-template port patching so docker-compose.yml is updated even when the template does not use the exact default port mappings.

Success criteria
- Template creation injects available proxy/db ports into docker-compose.yml using a more flexible matcher.
- Port patching supports common compose patterns (explicit ports, env-substituted ports) without failing.
- Job logs show which compose pattern was updated or a precise reason when no mapping is found.

Notes
- Keep task lists in planning files only. Do not move tasks back into manager.md.
