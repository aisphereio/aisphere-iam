#!/usr/bin/env python3
from pathlib import Path

path = Path("api/iam/resource/v1/resource.proto")
content = path.read_text(encoding="utf-8")

reasons = {
    'audit: { enabled: true event: "iam.resource.move" risk: "high" }':
        'reason: "Resource moves are requested only by trusted resource owner services."',
    'audit: { enabled: true event: "iam.resource.archive" risk: "high" }':
        'reason: "Resource archival is synchronized from trusted resource owner services."',
    'audit: { enabled: true event: "iam.resource.delete" risk: "high" }':
        'reason: "Resource deletion is synchronized from trusted resource owner services."',
    'audit: { enabled: true event: "iam.resource_binding.bind" risk: "high" }':
        'reason: "Resource bindings are projected by trusted platform services."',
    'audit: { enabled: true event: "iam.resource_binding.unbind" risk: "high" }':
        'reason: "Resource binding removal is projected by trusted platform services."',
    'audit: { enabled: true event: "iam.external_resource_binding.bind" risk: "high" }':
        'reason: "External resource mappings are projected by trusted integration services."',
}

for audit, reason in reasons.items():
    if reason in content:
        continue
    needle = f"      {audit}\n"
    if needle not in content:
        raise RuntimeError(f"expected audit policy not found: {audit}")
    content = content.replace(needle, needle + f"      {reason}\n", 1)

path.write_text(content, encoding="utf-8")
print("IAM internal resource endpoint reasons added")
